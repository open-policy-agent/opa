// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package bundle

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/internal/file/archive"
)

func TestRead(t *testing.T) {

	files := [][2]string{
		{"/a/b/c/data.json", "[1,2,3]"},
		{"/a/b/d/data.json", "true"},
		{"/a/b/y/data.yaml", `foo: 1`},
		{"/example/example.rego", `package example`},
		{"/data.json", `{"x": {"y": true}, "a": {"b": {"z": true}}}}`},
	}

	buf := archive.MustWriteTarGz(files)
	bundle, err := NewReader(buf).Read()
	if err != nil {
		t.Fatal(err)
	}

	module := `package example`

	exp := Bundle{
		Data: map[string]interface{}{
			"a": map[string]interface{}{
				"b": map[string]interface{}{
					"c": []interface{}{json.Number("1"), json.Number("2"), json.Number("3")},
					"d": true,
					"y": map[string]interface{}{
						"foo": json.Number("1"),
					},
					"z": true,
				},
			},
			"x": map[string]interface{}{
				"y": true,
			},
		},
		Modules: []ModuleFile{
			{
				Path:   "/example/example.rego",
				Parsed: ast.MustParseModule(module),
				Raw:    []byte(module),
			},
		},
	}

	if !exp.Equal(bundle) {
		t.Fatal("\nExp:", exp, "\n\nGot:", bundle)
	}
}

func TestReadWithManifest(t *testing.T) {
	files := [][2]string{
		{"/.manifest", `{"revision": "quickbrownfaux"}`},
	}
	buf := archive.MustWriteTarGz(files)
	bundle, err := NewReader(buf).Read()
	if err != nil {
		t.Fatal(err)
	}
	if bundle.Manifest.Revision != "quickbrownfaux" {
		t.Fatalf("Unexpected manifest.revision value: %v", bundle.Manifest.Revision)
	}
}

func TestReadWithManifestInData(t *testing.T) {
	files := [][2]string{
		{"/.manifest", `{"revision": "quickbrownfaux"}`},
	}
	buf := archive.MustWriteTarGz(files)
	bundle, err := NewReader(buf).IncludeManifestInData(true).Read()
	if err != nil {
		t.Fatal(err)
	}

	system := bundle.Data["system"].(map[string]interface{})
	b := system["bundle"].(map[string]interface{})
	m := b["manifest"].(map[string]interface{})

	if m["revision"] != "quickbrownfaux" {
		t.Fatalf("Unexpected manifest.revision value: %v. Expected: %v", m["revision"], "quickbrownfaux")
	}
}

func TestReadRootValidation(t *testing.T) {
	cases := []struct {
		note  string
		files [][2]string
		err   string
	}{
		{
			note: "default: full extent",
			files: [][2]string{
				{"/.manifest", `{"revision": "abcd"}`},
				{"/data.json", `{"a": 1}`},
				{"/x.rego", `package foo`},
			},
			err: "",
		},
		{
			note: "explicit: full extent",
			files: [][2]string{
				{"/.manifest", `{"revision": "abcd", "roots": [""]}`},
				{"/data.json", `{"a": 1}`},
				{"/x.rego", `package foo`},
			},
			err: "",
		},
		{
			note: "implicit: prefixed",
			files: [][2]string{
				{"/.manifest", `{"revision": "abcd", "roots": ["a/b", "foo"]}`},
				{"/data.json", `{"a": {"b": 1}}`},
				{"/x.rego", `package foo.bar`},
			},
			err: "",
		},
		{
			note: "err: empty",
			files: [][2]string{
				{"/.manifest", `{"revision": "abcd", "roots": []}`},
				{"/x.rego", `package foo`},
			},
			err: "manifest roots [] do not permit 'package foo' in module '/x.rego'",
		},
		{
			note: "err: overlapped",
			files: [][2]string{
				{"/.manifest", `{"revision": "abcd", "roots": ["a/b", "a"]}`},
			},
			err: "manifest has overlapped roots: a/b and a",
		},
		{
			note: "edge: overlapped partial segment",
			files: [][2]string{
				{"/.manifest", `{"revision": "abcd", "roots": ["a", "another_root"]}`},
			},
			err: "",
		},
		{
			note: "err: package outside scope",
			files: [][2]string{
				{"/.manifest", `{"revision": "abcd", "roots": ["a", "b", "c/d"]}`},
				{"/a.rego", `package b.c`},
				{"/x.rego", `package c.e`},
			},
			err: "manifest roots [a b c/d] do not permit 'package c.e' in module '/x.rego'",
		},
		{
			note: "err: data outside scope",
			files: [][2]string{
				{"/.manifest", `{"revision": "abcd", "roots": ["a", "b", "c/d"]}`},
				{"/data.json", `{"a": 1}`},
				{"/c/e/data.json", `"bad bad bad"`},
			},
			err: "manifest roots [a b c/d] do not permit data at path '/c/e'",
		},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			buf := archive.MustWriteTarGz(tc.files)
			_, err := NewReader(buf).IncludeManifestInData(true).Read()
			if tc.err == "" && err != nil {
				t.Fatal("Unexpected error occurred:", err)
			} else if tc.err != "" && err == nil {
				t.Fatal("Expected error but got success")
			} else if tc.err != "" && err != nil {
				if !strings.Contains(err.Error(), tc.err) {
					t.Fatalf("Expected error to contain %q but got: %v", tc.err, err)
				}
			}
		})
	}

}

func TestReadErrorBadGzip(t *testing.T) {
	buf := bytes.NewBufferString("bad gzip bytes")
	_, err := NewReader(buf).Read()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestReadErrorBadTar(t *testing.T) {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	gw.Write([]byte("bad tar bytes"))
	gw.Close()
	_, err := NewReader(&buf).Read()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestReadErrorBadContents(t *testing.T) {
	tests := []struct {
		files [][2]string
	}{
		{[][2]string{{"/test.rego", "lkafjasdkljf"}}},
		{[][2]string{{"/data.json", "lskjafkljsdf"}}},
		{[][2]string{{"/data.json", "[1,2,3]"}}},
		{[][2]string{
			{"/a/b/data.json", "[1,2,3]"},
			{"a/b/c/data.json", "true"},
		}},
		{[][2]string{{"/test.rego", ""}}},
		{[][2]string{
			{"/a/b/data.json", `{"c": "foo"}`},
			{"/data.json", `{"a": {"b": {"c": [123]}}}`},
		}},
	}
	for _, test := range tests {
		buf := archive.MustWriteTarGz(test.files)
		_, err := NewReader(buf).Read()
		if err == nil {
			t.Fatal("expected error")
		}
	}

}

func TestRoundtrip(t *testing.T) {

	bundle := Bundle{
		Data: map[string]interface{}{
			"foo": map[string]interface{}{
				"bar": []interface{}{json.Number("1"), json.Number("2"), json.Number("3")},
				"baz": true,
				"qux": "hello",
			},
		},
		Modules: []ModuleFile{
			{
				Path:   "/foo/corge/corge.rego",
				Parsed: ast.MustParseModule(`package foo.corge`),
				Raw:    []byte(`package foo.corge`),
			},
		},
		Manifest: Manifest{
			Revision: "quickbrownfaux",
		},
	}

	var buf bytes.Buffer

	if err := Write(&buf, bundle); err != nil {
		t.Fatal("Unexpected error:", err)
	}

	bundle2, err := NewReader(&buf).Read()
	if err != nil {
		t.Fatal("Unexpected error:", err)
	}

	if !bundle2.Equal(bundle) {
		t.Fatal("Exp:", bundle, "\n\nGot:", bundle2)
	}

}

func TestRootPathsOverlap(t *testing.T) {
	cases := []struct {
		note     string
		rootA    string
		rootB    string
		expected bool
	}{
		{"both empty", "", "", true},
		{"a empty", "", "foo/bar", true},
		{"b empty", "foo/bar", "", true},
		{"no overlap", "a/b/c", "x/y", false},
		{"partial segment overlap a", "a/b", "a/banana", false},
		{"partial segment overlap b", "a/banana", "a/b", false},
		{"overlap a", "a/b", "a/b/c", true},
		{"overlap b", "a/b/c", "a/b", true},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			actual := RootPathsOverlap(tc.rootA, tc.rootB)
			if actual != tc.expected {
				t.Errorf("Expected %t, got %t", tc.expected, actual)
			}
		})
	}
}

func TestParsedModules(t *testing.T) {
	cases := []struct {
		note            string
		bundle          Bundle
		name            string
		expectedModules []string
	}{
		{
			note: "base",
			bundle: Bundle{
				Modules: []ModuleFile{
					{
						Path:   "/foo/policy.rego",
						Parsed: ast.MustParseModule(`package foo`),
						Raw:    []byte(`package foo`),
					},
				},
			},
			name: "test-bundle",
			expectedModules: []string{
				"test-bundle/foo/policy.rego",
			},
		},
		{
			note: "filepath name",
			bundle: Bundle{
				Modules: []ModuleFile{
					{
						Path:   "/foo/policy.rego",
						Parsed: ast.MustParseModule(`package foo`),
						Raw:    []byte(`package foo`),
					},
				},
			},
			name: "/some/system/path",
			expectedModules: []string{
				"/some/system/path/foo/policy.rego",
			},
		},
		{
			note: "file url name",
			bundle: Bundle{
				Modules: []ModuleFile{
					{
						Path:   "/foo/policy.rego",
						Parsed: ast.MustParseModule(`package foo`),
						Raw:    []byte(`package foo`),
					},
				},
			},
			name: "file:///some/system/path",
			expectedModules: []string{
				"/some/system/path/foo/policy.rego",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			parsedMods := tc.bundle.ParsedModules(tc.name)

			for _, exp := range tc.expectedModules {
				mod, ok := parsedMods[exp]
				if !ok {
					t.Fatalf("Missing expected module %s, got: %+v", exp, parsedMods)
				}
				if mod == nil {
					t.Fatalf("Expected module to be non-nil")
				}
			}
		})
	}
}
