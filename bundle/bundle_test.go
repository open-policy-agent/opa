// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package bundle

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
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
	bundle, err := Read(buf)
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
	bundle, err := Read(buf)
	if err != nil {
		t.Fatal(err)
	}
	if bundle.Manifest.Revision != "quickbrownfaux" {
		t.Fatalf("Unexpected manifest.revision value: %v", bundle.Manifest.Revision)
	}
}

func TestReadErrorBadGzip(t *testing.T) {
	buf := bytes.NewBufferString("bad gzip bytes")
	_, err := Read(buf)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestReadErrorBadTar(t *testing.T) {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	gw.Write([]byte("bad tar bytes"))
	gw.Close()
	_, err := Read(&buf)
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
	}
	for _, test := range tests {
		buf := writeTarGz(test.files)
		_, err := Read(buf)
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

	bundle2, err := Read(&buf)
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
