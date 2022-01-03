// Copyright 2021 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cmd

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/internal/file/archive"

	"github.com/open-policy-agent/opa/util"

	"github.com/open-policy-agent/opa/util/test"
)

func TestDoInspect(t *testing.T) {
	files := [][2]string{
		{"/.manifest", `{"revision": "rev", "roots": ["foo", "bar", "fuz", "baz", "a", "x"]}`},
		{"/data.json", `{"x": {"y": true}, "a": {"b": {"z": true}}}}`},
		{"/example/foo.rego", `package foo`},
	}

	buf := archive.MustWriteTarGz(files)

	test.WithTempFS(nil, func(rootDir string) {
		bundleFile := filepath.Join(rootDir, "bundle.tar.gz")

		bf, err := os.Create(bundleFile)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		_, err = bf.Write(buf.Bytes())
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		var out bytes.Buffer
		params := newInspectCommandParams()
		err = params.outputFormat.Set(evalJSONOutput)
		if err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}

		err = doInspect(params, bundleFile, &out)
		if err != nil {
			t.Fatalf("Unexpected error %v", err)
		}

		res := `{
    "manifest": {"revision": "rev", "roots": ["foo", "bar", "fuz", "baz", "a", "x"]},
    "signatures_config": {},
    "namespaces": {"data": ["/data.json"], "data.foo": ["/example/foo.rego"]}
  }`

		exp := util.MustUnmarshalJSON([]byte(res))
		result := util.MustUnmarshalJSON(out.Bytes())
		if !reflect.DeepEqual(exp, result) {
			t.Fatalf("expected inspect output to be %v, got %v", exp, result)
		}
	})
}

func TestDoInspectPretty(t *testing.T) {

	root := fmt.Sprintf("metadata/%v/features", strings.Repeat("foobar", 20))

	manifest := fmt.Sprintf(`{"revision": "%s",
"roots": ["foo", "bar", "fuz", "http", "a", "x", "%s"],
"metadata": {"hello": "%s"},
"wasm": [{"entrypoint": "http/example/authz", "module": "/policy.wasm"}, {"entrypoint": "http/example/foo/allow", "module": "/example/policy.wasm"}]}`, strings.Repeat("foobar", 10), root, strings.Repeat("world", 100))

	files := [][2]string{
		{"/.manifest", manifest},
		{"/data.json", `{"x": {"y": true}, "a": {"b": {"z": true}}}}`},
		{"/http/example/authz/foo.rego", `package http.example.authz`},
		{"/http/example/authz/data.json", `{"faz": "baz"}`},
		{"/example/foo.rego", `package foo`},
		{"/a/b/y/foo.rego", `package a.b.y`},
		{"/a/xxxxxxxxxxxxxxxxxxxxxx/yyyyyyyyyyyyyyyyyyyy/foo.rego", `package a.b.y`},
		{"/example/policy.wasm", `modules-compiled-as-wasm-binary`},
		{"/http/example/policy.wasm", `modules-compiled-as-wasm-binary`},
		{"/policy.wasm", `modules-compiled-as-wasm-binary`},
	}

	buf := archive.MustWriteTarGz(files)

	test.WithTempFS(nil, func(rootDir string) {
		bundleFile := filepath.Join(rootDir, "bundle.tar.gz")

		bf, err := os.Create(bundleFile)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		_, err = bf.Write(buf.Bytes())
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		var out bytes.Buffer
		err = doInspect(newInspectCommandParams(), bundleFile, &out)
		if err != nil {
			t.Fatalf("Unexpected error %v", err)
		}

		output := strings.TrimSpace(out.String())
		expected := strings.TrimSpace(`
 MANIFEST:
+----------+-------------------------------------------------------+
|  FIELD   |                         VALUE                         |
+----------+-------------------------------------------------------+
| Revision | foobarfoobarfoobarfoobarfoobarfoobarfoobarfoobarfo... |
| Roots    | a                                                     |
|          | bar                                                   |
|          | foo                                                   |
|          | fuz                                                   |
|          | http                                                  |
|          | metadata/...oobarfoobarfoobarfoobarfoobar/features    |
|          | x                                                     |
| Metadata | {"hello":"worldworldworldworldworldworldworldworld... |
+----------+-------------------------------------------------------+
NAMESPACES:
+-----------------------------+----------------------------------------------------+
|          NAMESPACE          |                        FILE                        |
+-----------------------------+----------------------------------------------------+
| data                        | /data.json                                         |
| data.a.b.y                  | /a/b/y/foo.rego                                    |
|                             | /a/...xxxxxxxxxxxxxx/yyyyyyyyyyyyyyyyyyyy/foo.rego |
| data.foo                    | /example/foo.rego                                  |
| data.http.example.authz     | /http/example/authz/foo.rego                       |
|                             | /http/example/authz/data.json                      |
|                             | /policy.wasm                                       |
| data.http.example.foo.allow | /example/policy.wasm                               |
+-----------------------------+----------------------------------------------------+
`)

		if output != expected {
			t.Fatalf("Unexpected output. Expected:\n\n%v\n\nGot:\n\n%v", expected, output)
		}

	})
}

func TestDoInspectPrettyManifestOnlySingleRoot(t *testing.T) {

	root := fmt.Sprintf("metadata/%v/features", strings.Repeat("foobar", 6))

	manifest := fmt.Sprintf(`{"roots": ["%s"],
"metadata": {"hello": "world"}}`, root)

	files := [][2]string{
		{"/.manifest", manifest},
	}

	buf := archive.MustWriteTarGz(files)

	test.WithTempFS(nil, func(rootDir string) {
		bundleFile := filepath.Join(rootDir, "bundle.tar.gz")

		bf, err := os.Create(bundleFile)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		_, err = bf.Write(buf.Bytes())
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		var out bytes.Buffer
		err = doInspect(newInspectCommandParams(), bundleFile, &out)
		if err != nil {
			t.Fatalf("Unexpected error %v", err)
		}

		output := strings.TrimSpace(out.String())
		expected := strings.TrimSpace(`
MANIFEST:
+----------+----------------------------------------------------+
|  FIELD   |                       VALUE                        |
+----------+----------------------------------------------------+
| Roots    | metadata/...oobarfoobarfoobarfoobarfoobar/features |
| Metadata | {"hello":"world"}                                  |
+----------+----------------------------------------------------+
`)

		if output != expected {
			t.Fatalf("Unexpected output. Expected:\n\n%v\n\nGot:\n\n%v", expected, output)
		}

	})
}

func TestInspectMultiBundleError(t *testing.T) {
	params := newInspectCommandParams()
	err := validateInspectParams(&params, []string{"foo", "bar"})
	if err == nil {
		t.Fatal("Expected error but got nil")
	}

	exp := "specify exactly one OPA bundle or path"
	if err.Error() != exp {
		t.Fatalf("Expected error %v but got %v", exp, err.Error())
	}
}
