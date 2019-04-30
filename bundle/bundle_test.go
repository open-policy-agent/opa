// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package bundle

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/ast"
)

func TestRead(t *testing.T) {

	files := [][2]string{
		{"/a/b/c/data.json", "[1,2,3]"},
		{"/a/b/d/data.json", "true"},
		{"/example/example.rego", `package example`},
	}

	buf := writeTarGz(files)
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
				},
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
		t.Fatal("Exp:", exp, "\n\nGot:", bundle)
	}
}

func TestReadWithManifest(t *testing.T) {
	files := [][2]string{
		{"/.manifest", `{"revision": "quickbrownfaux"}`},
	}
	buf := writeTarGz(files)
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
	buf := writeTarGz(files)
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
			err: "manifest roots do not permit 'package foo' in /x.rego",
		},
		{
			note: "err: overlapped",
			files: [][2]string{
				{"/.manifest", `{"revision": "abcd", "roots": ["a/b", "a"]}`},
			},
			err: "manifest has overlapped roots: a/b and a",
		},
		{
			note: "err: package outside scope",
			files: [][2]string{
				{"/.manifest", `{"revision": "abcd", "roots": ["a", "b", "c/d"]}`},
				{"/a.rego", `package b.c`},
				{"/x.rego", `package c.e`},
			},
			err: "manifest roots do not permit 'package c.e' in /x.rego",
		},
		{
			note: "err: data outside scope",
			files: [][2]string{
				{"/.manifest", `{"revision": "abcd", "roots": ["a", "b", "c/d"]}`},
				{"/data.json", `{"a": 1}`},
				{"/c/e/data.json", `"bad bad bad"`},
			},
			err: "manifest roots do not permit data at path c/e",
		},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			buf := writeTarGz(tc.files)
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
	}
	for _, test := range tests {
		buf := writeTarGz(test.files)
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

func writeTarGz(files [][2]string) *bytes.Buffer {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	defer gw.Close()
	tw := tar.NewWriter(gw)
	defer tw.Close()
	for _, file := range files {
		if err := writeFile(tw, file[0], []byte(file[1])); err != nil {
			panic(err)
		}
	}
	return &buf
}
