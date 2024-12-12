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
	"github.com/open-policy-agent/opa/v1/util"

	"github.com/open-policy-agent/opa/v1/util/test"
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
    "capabilities": {"features": ["rego_v1"]},
    "manifest": {"revision": "rev", "roots": ["foo", "bar", "fuz", "baz", "a", "x"]},
    "signatures_config": {},
    "namespaces": {"data": ["/data.json"], "data.foo": ["/example/foo.rego"]}
  }`

		exp := util.MustUnmarshalJSON([]byte(res))
		result := util.MustUnmarshalJSON(out.Bytes())
		if !reflect.DeepEqual(exp, result) {
			t.Fatalf("expected inspect output to be:\n\n%v\n\ngot:\n\n%v", exp, result)
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

func TestDoInspect_V0Compatible(t *testing.T) {
	tests := []struct {
		note         string
		v0Compatible bool
		module       string
		expErrs      []string
	}{
		{
			note:         "v0, keywords not used",
			v0Compatible: true,
			module: `package test
p[v] { 
	v := input.x 
}`,
		},
		{
			note:         "v0, no keywords imported, but used",
			v0Compatible: true,
			module: `package test
p contains v if { 
	v := input.x 
}`,
			expErrs: []string{
				"rego_parse_error: var cannot be used for rule name",
			},
		},
		{
			note: "v0, keywords imported",
			module: `package test
import future.keywords
p contains v if { 
	v := input.x 
}`,
		},
		{
			note: "v0, rego.v1 imported",
			module: `package test
import rego.v1
p contains v if { 
	v := input.x 
}`,
		},
		{
			note: "v1, keywords not used",
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
			note: "v1, no keywords imported",
			module: `package test
p contains v if { 
	v := input.x 
}`,
		},
		{
			note: "v1, keywords imported",
			module: `package test
import future.keywords
p contains v if { 
	v := input.x 
}`,
		},
		{
			note: "v1, rego.v1 imported",
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
				params.v0Compatible = tc.v0Compatible
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

func TestDoInspectWithBundleRegoVersion(t *testing.T) {
	tests := []struct {
		note              string
		bundleRegoVersion int
		files             map[string]string
		expErrs           []string
	}{
		{
			note:              "v0.x bundle, keywords not used",
			bundleRegoVersion: 0,
			files: map[string]string{
				".manifest": `{"rego_version": 0}`,
				"policy.rego": `package test
p[v] { 
	v := input.x 
}`,
			},
		},
		{
			note:              "v0.x bundle, no keywords imported, but used",
			bundleRegoVersion: 0,
			files: map[string]string{
				".manifest": `{"rego_version": 0}`,
				"policy.rego": `package test
p contains v if { 
	v := input.x 
}`,
			},
			expErrs: []string{
				"rego_parse_error: var cannot be used for rule name",
			},
		},
		{
			note:              "v0.x bundle, keywords imported",
			bundleRegoVersion: 0,
			files: map[string]string{
				".manifest": `{"rego_version": 0}`,
				"policy.rego": `package test
import future.keywords
p contains v if { 
	v := input.x 
}`,
			},
		},
		{
			note:              "v0.x bundle, rego.v1 imported",
			bundleRegoVersion: 0,
			files: map[string]string{
				".manifest": `{"rego_version": 0}`,
				"policy.rego": `package test
import rego.v1
p contains v if { 
	v := input.x 
}`,
			},
		},
		{
			note:              "v0 bundle, v1 per-file override",
			bundleRegoVersion: 0,
			files: map[string]string{
				".manifest": `{
	"rego_version": 0,
	"file_rego_versions": {
		"/policy2.rego": 1
	}
}`,
				"policy1.rego": `package test
p[1] {
	v := input.x
}`,
				"policy2.rego": `package test
p contains 2 if {
	v := input.x
}`,
			},
		},
		{
			note:              "v0 bundle, v1 per-file override (glob)",
			bundleRegoVersion: 0,
			files: map[string]string{
				".manifest": `{
	"rego_version": 0,
	"file_rego_versions": {
		"/bar/*.rego": 1
	}
}`,
				"foo/policy1.rego": `package test
p[1] {
	v := input.x
}`,
				"bar/policy2.rego": `package test
p contains 2 if {
	v := input.x
}`,
			},
		},
		{
			note:              "v0 bundle, v1 per-file override, incompatible",
			bundleRegoVersion: 0,
			files: map[string]string{
				".manifest": `{
	"rego_version": 0,
	"file_rego_versions": {
		"/policy2.rego": 1
	}
}`,
				"policy1.rego": `package test
p[1] {
	v := input.x
}`,
				"policy2.rego": `package test
p[2] {
	v := input.x
}`,
			},
			expErrs: []string{
				"rego_parse_error: `if` keyword is required before rule body",
				"rego_parse_error: `contains` keyword is required for partial set rules",
			},
		},
		{
			note:              "v1.0 bundle, keywords not used",
			bundleRegoVersion: 1,
			files: map[string]string{
				".manifest": `{"rego_version": 1}`,
				"policy.rego": `package test
p[v] { 
	v := input.x 
}`,
			},
			expErrs: []string{
				"rego_parse_error: `if` keyword is required before rule body",
				"rego_parse_error: `contains` keyword is required for partial set rules",
			},
		},
		{
			note:              "v1.0 bundle, no keywords imported",
			bundleRegoVersion: 1,
			files: map[string]string{
				".manifest": `{"rego_version": 1}`,
				"policy.rego": `package test
p contains v if { 
	v := input.x 
}`,
			},
		},
		{
			note:              "v1.0 bundle, keywords imported",
			bundleRegoVersion: 1,
			files: map[string]string{
				".manifest": `{"rego_version": 1}`,
				"policy.rego": `package test
import future.keywords
p contains v if { 
	v := input.x 
}`,
			},
		},
		{
			note:              "v1.0 bundle, rego.v1 imported",
			bundleRegoVersion: 1,
			files: map[string]string{
				".manifest": `{"rego_version": 1}`,
				"policy.rego": `package test
import rego.v1
p contains v if { 
	v := input.x 
}`,
			},
		},
		{
			note:              "v1 bundle, v0 per-file override",
			bundleRegoVersion: 1,
			files: map[string]string{
				".manifest": `{
	"rego_version": 1,
	"file_rego_versions": {
		"/policy1.rego": 0
	}
}`,
				"policy1.rego": `package test
p[1] {
	v := input.x
}`,
				"policy2.rego": `package test
p contains 2 if {
	v := input.x
}`,
			},
		},
		{
			note:              "v1 bundle, v0 per-file override",
			bundleRegoVersion: 1,
			files: map[string]string{
				".manifest": `{
	"rego_version": 1,
	"file_rego_versions": {
		"/foo/*.rego": 0
	}
}`,
				"foo/policy1.rego": `package test
p[1] {
	v := input.x
}`,
				"bar/policy2.rego": `package test
p contains 2 if {
	v := input.x
}`,
			},
		},
		{
			note:              "v1 bundle, v0 per-file override, incompatible",
			bundleRegoVersion: 1,
			files: map[string]string{
				".manifest": `{
	"rego_version": 1,
	"file_rego_versions": {
		"/policy1.rego": 0
	}
}`,
				"policy1.rego": `package test
p contains 1 if {
	v := input.x
}`,
				"policy2.rego": `package test
p contains 2 if {
	v := input.x
}`,
			},
			expErrs: []string{
				"rego_parse_error: var cannot be used for rule name",
				"rego_parse_error: number cannot be used for rule name",
			},
		},
	}

	bundleTypeCases := []struct {
		note string
		tar  bool
	}{
		{
			"bundle dir", false,
		},
		{
			"bundle tar", true,
		},
	}

	v1CompatibleFlagCases := []struct {
		note string
		used bool
	}{
		{
			"no --v1-compatible", false,
		},
		{
			"--v1-compatible", true,
		},
	}

	for _, bundleType := range bundleTypeCases {
		for _, v1CompatibleFlag := range v1CompatibleFlagCases {
			for _, tc := range tests {
				t.Run(fmt.Sprintf("%s, %s, %s", bundleType.note, v1CompatibleFlag.note, tc.note), func(t *testing.T) {
					files := map[string]string{}
					if bundleType.tar {
						files["bundle.tar.gz"] = ""
					} else {
						for k, v := range tc.files {
							files[k] = v
						}
					}

					test.WithTempFS(files, func(root string) {
						p := root
						if bundleType.tar {
							p = filepath.Join(root, "bundle.tar.gz")
							files := make([][2]string, 0, len(tc.files))
							for k, v := range tc.files {
								files = append(files, [2]string{k, v})
							}
							buf := archive.MustWriteTarGz(files)
							bf, err := os.Create(p)
							if err != nil {
								t.Fatalf("Unexpected error: %v", err)
							}
							_, err = bf.Write(buf.Bytes())
							if err != nil {
								t.Fatalf("Unexpected error: %v", err)
							}
						}

						var out bytes.Buffer
						params := newInspectCommandParams()
						params.v1Compatible = v1CompatibleFlag.used
						err := params.outputFormat.Set(evalPrettyOutput)
						if err != nil {
							t.Fatalf("Unexpected error: %s", err)
						}

						err = doInspect(params, p, &out)

						if len(tc.expErrs) > 0 {
							if err == nil {
								t.Fatalf("Expected error but got output: %s", out.String())
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

							expOut := fmt.Sprintf(`MANIFEST:
+--------------+-------+
|    FIELD     | VALUE |
+--------------+-------+
| Rego Version | %d     |
+--------------+-------+`,
								tc.bundleRegoVersion)
							if !strings.Contains(out.String(), expOut) {
								t.Fatalf("Expected output to contain:\n\n%s\n\nbut got:\n\n%s", expOut, out.String())
							}
						}
					})
				})
			}
		}
	}
}

func TestUnknownRefs(t *testing.T) {
	tests := []struct {
		note     string
		files    [][2]string
		expected string
	}{
		{
			note: "unknown built-in func call",
			files: [][2]string{
				{
					"/policy.rego", `package test
p if {
	foo.bar(42)
	contains("foo", "o")
}`,
				},
			},
			// Note: unknown foo.bar() built-in doesn't appear in the output, but also didn't cause an error.
			expected: `{
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
    ],
    "features": [
      "rego_v1"
    ]
  }
}`,
		},
		{
			// Happy path
			note: "known ref replaced inside 'with' stmt",
			files: [][2]string{
				{"/policy.rego", `package test

foo.bar(_) := false

p if {
	foo.bar(42)
}

mock(_) := true

test_p if {
	p with data.test.foo.bar as mock
}`},
			},
			expected: `{
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
    "features": [
      "rego_v1"
    ]
  }
}`,
		},
		{
			note: "unknown ref replaced inside 'with' stmt",
			files: [][2]string{
				{"/policy.rego", `package test

p if {
	data.foo.bar(42)
}

mock(_) := true

test_p if {
	p with data.foo.bar as mock
}`},
			},
			expected: `{
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
    "features": [
      "rego_v1"
    ]
  }
}`,
		},
		{
			note: "unknown built-in (var) replaced inside 'with' stmt",
			files: [][2]string{
				{"/policy.rego", `package test

p if {
	foo(42)
}

mock(_) := true

test_p if {
	p with foo as mock
}`},
			},
			expected: `{
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
    "features": [
      "rego_v1"
    ]
  }
}`,
		},
		{
			note: "unknown built-in (ref) replaced inside 'with' stmt",
			files: [][2]string{
				{"/policy.rego", `package test

p if {
	foo.bar(42)
}

mock(_) := true

test_p if {
	p with foo.bar as mock
}`},
			},
			expected: `{
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
    "features": [
      "rego_v1"
    ]
  }
}`,
		},
		{
			note: "call replaced by unknown data ref inside 'with' stmt",
			files: [][2]string{
				{"/policy.rego", `package test

p if {
	foo(42)
}

foo(_) := false

test_p if {
	p with foo as data.bar
}`},
			},
			expected: `{
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
      }
    ],
    "features": [
      "rego_v1"
    ]
  }
}`,
		},
		{
			note: "call replaced by unknown built-in (var) inside 'with' stmt",
			files: [][2]string{
				{"/policy.rego", `package test

p if {
	foo(42)
}

foo(_) := false

test_p if {
	# bar is unknown built-in
	p with foo as bar
}`},
			},
			expected: `{
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
    "features": [
      "rego_v1"
    ]
  }
}`,
		},
		{
			note: "call replaced by unknown built-in (ref) inside 'with' stmt",
			files: [][2]string{
				{"/policy.rego", `package test

p if {
	foo(42)
}

foo(_) := false

test_p if {
	# bar.baz is unknown built-in
	p with foo as bar.baz
}`},
			},
			expected: `{
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
      }
    ],
    "features": [
      "rego_v1"
    ]
  }
}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			buf := archive.MustWriteTarGz(tc.files)

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
				if output != tc.expected {
					t.Fatalf("Unexpected output. Expected:\n\n%s\n\nGot:\n\n%s", tc.expected, output)
				}
			})
		})
	}
}

func TestCallToUnknownRegoFunction(t *testing.T) {
	files := [][2]string{
		{"/policy.rego", `package test
import data.x.y

p if {
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
    ],
    "features": [
      "rego_v1"
    ]
  }
}`)

		if output != expected {
			t.Fatalf("Unexpected output. Expected:\n\n%s\n\nGot:\n\n%s", expected, output)
		}
	})
}

func TestDoInspectSingleFileWithAnnotations(t *testing.T) {
	files := map[string]string{
		"/a/xxxxxxxxxxxxxxxxxxxxxx/yyyyyyyyyyyyyyyyyyyy/foo.rego": `# METADATA
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
		fileName := fmt.Sprintf("%s/a/xxxxxxxxxxxxxxxxxxxxxx/yyyyyyyyyyyyyyyyyyyy/foo.rego", rootDir)
		ps := newInspectCommandParams()
		ps.listAnnotations = true
		var out bytes.Buffer
		err := doInspect(ps, fileName, &out)
		if err != nil {
			t.Fatalf("Unexpected error %v", err)
		}

		shortFileName := truncateFileName(fileName)
		output := strings.TrimSpace(out.String())
		expected := strings.TrimSpace(fmt.Sprintf(`
NAMESPACES:
+-----------+----------------------------------------------------+
| NAMESPACE |                        FILE                        |
+-----------+----------------------------------------------------+
| data.test | %[1]s |
+-----------+----------------------------------------------------+
ANNOTATIONS:
pkg-title
=========

pkg-descr

Package:  test
Location: %[2]s:16
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
Location: %[2]s:50
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
Location: %[2]s:50
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
 rule: "rule-custom"`, shortFileName, fileName))

		if output != expected {
			t.Fatalf("Unexpected output. Expected:\n\n%q\n\nGot:\n\n%q", expected, output)
		}
	})
}

func TestDoInspectSingleFile(t *testing.T) {
	files := map[string]string{
		"/a/xxxxxxxxxxxxxxxxxxxxxx/yyyyyyyyyyyyyyyyyyyy/foo.rego": `# METADATA
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
		fileName := fmt.Sprintf("%s/a/xxxxxxxxxxxxxxxxxxxxxx/yyyyyyyyyyyyyyyyyyyy/foo.rego", rootDir)
		ps := newInspectCommandParams()
		var out bytes.Buffer
		err := doInspect(ps, fileName, &out)
		if err != nil {
			t.Fatalf("Unexpected error %v", err)
		}

		shortFileName := truncateFileName(fileName)
		output := strings.TrimSpace(out.String())
		expected := strings.TrimSpace(fmt.Sprintf(`
NAMESPACES:
+-----------+----------------------------------------------------+
| NAMESPACE |                        FILE                        |
+-----------+----------------------------------------------------+
| data.test | %s |
+-----------+----------------------------------------------------+
`, shortFileName))

		if output != expected {
			t.Fatalf("Unexpected output. Expected:\n\n%q\n\nGot:\n\n%q", expected, output)
		}
	})
}
