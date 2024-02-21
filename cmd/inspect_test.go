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
		{"/data.json", `{"x": {"y": true}, "a": {"b": {"z": true}}}`},
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
    "capabilities": {},
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
		{"/data.json", `{"x": {"y": true}, "a": {"b": {"z": true}}}`},
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
+----------+----------------------------------------------------+
|  FIELD   |                       VALUE                        |
+----------+----------------------------------------------------+
| Revision | foobarfoobarfoobarfoobarfoobarfoobarfoobarfooba... |
| Roots    | a                                                  |
|          | bar                                                |
|          | foo                                                |
|          | fuz                                                |
|          | http                                               |
|          | metadata/...oobarfoobarfoobarfoobarfoobar/features |
|          | x                                                  |
| Metadata | {"hello":"worldworldworldworldworldworldworldwo... |
+----------+----------------------------------------------------+
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

func TestDoInspectPrettyWithAnnotations(t *testing.T) {

	files := map[string]string{
		"x.rego": `# METADATA
# title: pkg-title
# description: pkg-descr
# organizations:
# - pkg-org
# related_resources:
# - https://pkg
# - ref: https://pkg
#   description: rr-pkg-note
# authors:
# - pkg-author
# schemas:
# - input: {"type": "boolean"}
# custom:
#  pkg: pkg-custom
package test

# METADATA
# scope: document
# title: doc-title
# description: doc-descr
# organizations:
# - doc-org
# related_resources:
# - https://doc
# - ref: https://doc
#   description: rr-doc-note
# authors:
# - doc-author
# schemas:
# - input: {"type": "integer"}
# custom:
#  doc: doc-custom

# METADATA
# title: rule-title
# description: rule-title
# organizations:
# - rule-org
# related_resources:
# - https://rule
# - ref: https://rule
#   description: rr-rule-note
# authors:
# - rule-author
# schemas:
# - input: {"type": "string"}
# custom:
#  rule: rule-custom
p = 1`,
	}

	test.WithTempFS(files, func(rootDir string) {
		ps := newInspectCommandParams()
		ps.listAnnotations = true
		var out bytes.Buffer
		err := doInspect(ps, rootDir, &out)
		if err != nil {
			t.Fatalf("Unexpected error %v", err)
		}

		bs := out.Bytes()
		idx := bytes.Index(bs, []byte(`ANNOTATIONS`)) // skip NAMESPACE box
		output := strings.TrimSpace(string(bs[idx:]))
		expected := strings.TrimSpace(fmt.Sprintf(`
ANNOTATIONS:
pkg-title
=========

pkg-descr

Package:  test
Location: %[1]s/x.rego:16
Scope: package

Organizations:
 pkg-org

Authors:
 pkg-author

Schemas:
 input: {"type":"boolean"}

Related Resources:
 https://pkg
 https://pkg rr-pkg-note

Custom:
 pkg: "pkg-custom"

doc-title
=========

doc-descr

Package:  test
Rule:     p
Location: %[1]s/x.rego:50
Scope: document

Organizations:
 doc-org

Authors:
 doc-author

Schemas:
 input: {"type":"integer"}

Related Resources:
 https://doc
 https://doc rr-doc-note

Custom:
 doc: "doc-custom"

rule-title
==========

rule-title

Package:  test
Rule:     p
Location: %[1]s/x.rego:50
Scope: rule

Organizations:
 rule-org

Authors:
 rule-author

Schemas:
 input: {"type":"string"}

Related Resources:
 https://rule
 https://rule rr-rule-note

Custom:
 rule: "rule-custom"`, rootDir))

		if output != expected {
			t.Fatalf("Unexpected output. Expected:\n\n%q\n\nGot:\n\n%q", expected, output)
		}

	})
}

func TestDoInspectTarballPrettyWithAnnotations(t *testing.T) {

	files := [][2]string{
		{"x.rego", `# METADATA
# title: pkg-title
# description: pkg-descr
# organizations:
# - pkg-org
# related_resources:
# - https://pkg
# - ref: https://pkg
#   description: rr-pkg-note
# authors:
# - pkg-author
# schemas:
# - input: {"type": "boolean"}
# custom:
#  pkg: pkg-custom
package test

# METADATA
# scope: document
# title: doc-title
# description: doc-descr
# organizations:
# - doc-org
# related_resources:
# - https://doc
# - ref: https://doc
#   description: rr-doc-note
# authors:
# - doc-author
# schemas:
# - input: {"type": "integer"}
# custom:
#  doc: doc-custom

# METADATA
# title: rule-title
# description: rule-title
# organizations:
# - rule-org
# related_resources:
# - https://rule
# - ref: https://rule
#   description: rr-rule-note
# authors:
# - rule-author
# schemas:
# - input: {"type": "string"}
# custom:
#  rule: rule-custom
p = 1`},
		{".manifest", `
{
	"revision": "",
	"roots": [
		""
	],
	"wasm": [
		{
			"entrypoint": "test/a",
			"module": "/policy.wasm"
		},
		{
			"entrypoint": "test/b",
			"module": "/policy.wasm",
			"annotations": [
				{
					"scope": "rule",
					"title": "WASM RULE B",
					"entrypoint": true
				}
			]
		}
	]
}`},
		{"policy.wasm", ""},
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

		ps := newInspectCommandParams()
		ps.listAnnotations = true
		var out bytes.Buffer

		err = doInspect(ps, bundleFile, &out)
		if err != nil {
			t.Fatalf("Unexpected error %v", err)
		}

		bs := out.Bytes()
		idx := bytes.Index(bs, []byte(`ANNOTATIONS`)) // skip NAMESPACE box
		output := strings.TrimSpace(string(bs[idx:]))
		expected := strings.TrimSpace(`
ANNOTATIONS:
pkg-title
=========

pkg-descr

Package:  test
Location: /x.rego:16
Scope: package

Organizations:
 pkg-org

Authors:
 pkg-author

Schemas:
 input: {"type":"boolean"}

Related Resources:
 https://pkg
 https://pkg rr-pkg-note

Custom:
 pkg: "pkg-custom"

WASM RULE B
===========

Location: /policy.wasm:0
Scope: rule
Entrypoint: true

doc-title
=========

doc-descr

Package:  test
Rule:     p
Location: /x.rego:50
Scope: document

Organizations:
 doc-org

Authors:
 doc-author

Schemas:
 input: {"type":"integer"}

Related Resources:
 https://doc
 https://doc rr-doc-note

Custom:
 doc: "doc-custom"

rule-title
==========

rule-title

Package:  test
Rule:     p
Location: /x.rego:50
Scope: rule

Organizations:
 rule-org

Authors:
 rule-author

Schemas:
 input: {"type":"string"}

Related Resources:
 https://rule
 https://rule rr-rule-note

Custom:
 rule: "rule-custom"`)

		if output != expected {
			t.Fatalf("Unexpected output. Expected:\n\n%q\n\nGot:\n\n%q", expected, output)
		}

	})
}

func TestDoInspectV1Compatible(t *testing.T) {
	tests := []struct {
		note         string
		v1Compatible bool
		module       string
		expErrs      []string
	}{
		{
			note: "v0.x, keywords not used",
			module: `package test
p[v] { 
	v := input.x 
}`,
		},
		{
			note: "v0.x, no keywords imported, but used",
			module: `package test
p contains v if { 
	v := input.x 
}`,
			expErrs: []string{
				"rego_parse_error: var cannot be used for rule name",
			},
		},
		{
			note: "v0.x, keywords imported",
			module: `package test
import future.keywords
p contains v if { 
	v := input.x 
}`,
		},
		{
			note: "v0.x, rego.v1 imported",
			module: `package test
import rego.v1
p contains v if { 
	v := input.x 
}`,
		},
		{
			note:         "v1.0, keywords not used",
			v1Compatible: true,
			module: `package test
p[v] { 
	v := input.x 
}`,
			expErrs: []string{
				"rego_parse_error: `if` keyword is required before rule body",
				"rego_parse_error: `contains` keyword is required for partial set rules",
			},
		},
		{
			note:         "v1.0, no keywords imported",
			v1Compatible: true,
			module: `package test
p contains v if { 
	v := input.x 
}`,
		},
		{
			note:         "v1.0, keywords imported",
			v1Compatible: true,
			module: `package test
import future.keywords
p contains v if { 
	v := input.x 
}`,
		},
		{
			note:         "v1.0, rego.v1 imported",
			v1Compatible: true,
			module: `package test
import rego.v1
p contains v if { 
	v := input.x 
}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			test.WithTempFS(nil, func(rootDir string) {
				buf := archive.MustWriteTarGz([][2]string{{"/policy.rego", tc.module}})

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
				params.v1Compatible = tc.v1Compatible
				err = params.outputFormat.Set(evalJSONOutput)
				if err != nil {
					t.Fatalf("Unexpected error: %s", err)
				}

				err = doInspect(params, bundleFile, &out)

				if len(tc.expErrs) > 0 {
					if err == nil {
						t.Fatalf("Expected error but got nil")
					}

					for _, expErr := range tc.expErrs {
						if !strings.Contains(err.Error(), expErr) {
							t.Fatalf("Expected error:\n\n%v\n\nbut got:\n\n%v", expErr, err.Error())
						}
					}
				} else {
					if err != nil {
						t.Fatalf("Unexpected error %v", err)
					}
				}
			})
		})
	}
}

func TestCallToUnknownBuiltInFunction(t *testing.T) {
	files := [][2]string{
		{"/policy.rego", `package test
			p {
				foo.bar(42)
				contains("foo", "o")
			}
		`},
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

		bs := out.Bytes()
		output := strings.TrimSpace(string(bs))
		// Note: unknown foo.bar() built-in doesn't appear in the output, but also didn't cause an error.
		expected := strings.TrimSpace(`{
  "manifest": {
    "revision": "",
    "roots": [
      ""
    ]
  },
  "signatures_config": {},
  "namespaces": {
    "data.test": [
      "/policy.rego"
    ]
  },
  "capabilities": {
    "builtins": [
      {
        "name": "contains",
        "decl": {
          "args": [
            {
              "type": "string"
            },
            {
              "type": "string"
            }
          ],
          "result": {
            "type": "boolean"
          },
          "type": "function"
        }
      }
    ]
  }
}`)

		if output != expected {
			t.Fatalf("Unexpected output. Expected:\n\n%s\n\nGot:\n\n%s", expected, output)
		}
	})
}

func TestCallToUnknownRegoFunction(t *testing.T) {
	files := [][2]string{
		{"/policy.rego", `package test
import data.x.y

p {
	y(1) == true
}
		`},
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

		bs := out.Bytes()
		output := strings.TrimSpace(string(bs))
		// Note: unknown data.x.y() function doesn't appear in the output, but also didn't cause an error.
		expected := strings.TrimSpace(`{
  "manifest": {
    "revision": "",
    "roots": [
      ""
    ]
  },
  "signatures_config": {},
  "namespaces": {
    "data.test": [
      "/policy.rego"
    ]
  },
  "capabilities": {
    "builtins": [
      {
        "name": "eq",
        "decl": {
          "args": [
            {
              "type": "any"
            },
            {
              "type": "any"
            }
          ],
          "result": {
            "type": "boolean"
          },
          "type": "function"
        },
        "infix": "="
      },
      {
        "name": "equal",
        "decl": {
          "args": [
            {
              "type": "any"
            },
            {
              "type": "any"
            }
          ],
          "result": {
            "type": "boolean"
          },
          "type": "function"
        },
        "infix": "=="
      }
    ]
  }
}`)

		if output != expected {
			t.Fatalf("Unexpected output. Expected:\n\n%s\n\nGot:\n\n%s", expected, output)
		}
	})
}
