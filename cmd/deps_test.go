// Copyright 2024 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/internal/file/archive"
	"github.com/open-policy-agent/opa/v1/util/test"
)

func TestDeps_DefaultRegoVersion(t *testing.T) {
	tests := []struct {
		note    string
		module  string
		query   string
		expErrs []string
	}{
		{
			note: "v0 module",
			module: `package test
a[x] {
	x := 42
}`,
			query: `data.test.p`,
			expErrs: []string{
				"test.rego:2: rego_parse_error: `if` keyword is required before rule body",
				"test.rego:2: rego_parse_error: `contains` keyword is required for partial set rules",
			},
		},
		{
			note: "v1 module",
			module: `package test
a contains x if {
	x := 42
}`,
			query: `data.test.a`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			files := map[string]string{
				"test.rego": tc.module,
			}

			test.WithTempFS(files, func(rootPath string) {
				params := newDepsCommandParams()
				_ = params.outputFormat.Set(depsFormatPretty)

				for f := range files {
					_ = params.dataPaths.Set(filepath.Join(rootPath, f))
				}

				err := deps([]string{tc.query}, params, io.Discard)

				if len(tc.expErrs) > 0 {
					if err == nil {
						t.Fatalf("Expected error but got nil")
					}
					for _, expErr := range tc.expErrs {
						if !strings.Contains(err.Error(), expErr) {
							t.Fatalf("Expected error:\n\n%s\n\ngot:\n\n%s", expErr, err.Error())
						}
					}
				} else if err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}
			})
		})
	}
}

func TestDepsCompatibleFlags(t *testing.T) {
	tests := []struct {
		note         string
		v0Compatible bool
		v1Compatible bool
		module       string
		query        string
		expErrs      []string
	}{
		{
			note:         "v0, no keywords",
			v0Compatible: true,
			module: `package test
p[3] {
	input.x = 1
}`,
			query: `data.test.p`,
		},
		{
			note:         "v0, keywords not imported, but used",
			v0Compatible: true,
			module: `package test
p contains 3 if {
	input.x = 1
}`,
			query: `data.test.p`,
			expErrs: []string{
				"rego_parse_error: var cannot be used for rule name",
				"rego_parse_error: number cannot be used for rule name",
			},
		},
		{
			note:         "v0, keywords imported",
			v0Compatible: true,
			module: `package test
import future.keywords
p contains 3 if {
	input.x = 1
}`,
			query: `data.test.p`,
		},
		{
			note:         "v0, rego.v1 imported",
			v0Compatible: true,
			module: `package test
import rego.v1
p contains 3 if {
	input.x = 1
}`,
			query: `data.test.p`,
		},
		{
			note:         "v1, no keywords",
			v1Compatible: true,
			module: `package test
p[3] {
	input.x = 1
}`,
			query: `data.test.p`,
			expErrs: []string{
				"rego_parse_error: `if` keyword is required before rule body",
				"rego_parse_error: `contains` keyword is required for partial set rules",
			},
		},
		{
			note:         "v1, no keyword imports",
			v1Compatible: true,
			module: `package test
p contains 3 if {
	input.x = 1
}`,
			query: `data.test.p`,
		},
		{
			note:         "v1, keywords imported",
			v1Compatible: true,
			module: `package test
import future.keywords
p contains 3 if {
	input.x = 1
}`,
			query: `data.test.p`,
		},
		{
			note:         "v1, rego.v1 imported",
			v1Compatible: true,
			module: `package test
import rego.v1
p contains 3 if {
	input.x = 1
}`,
			query: `data.test.p`,
		},
		// v0 takes precedence over v1
		{
			note:         "v0+v1, no keywords",
			v0Compatible: true,
			v1Compatible: true,
			module: `package test
p[3] {
	input.x = 1
}`,
			query: `data.test.p`,
		},
		{
			note:         "v0+v1, keywords not imported, but used",
			v0Compatible: true,
			v1Compatible: true,
			module: `package test
p contains 3 if {
	input.x = 1
}`,
			query: `data.test.p`,
			expErrs: []string{
				"rego_parse_error: var cannot be used for rule name",
				"rego_parse_error: number cannot be used for rule name",
			},
		},
		{
			note:         "v0+v1, keywords imported",
			v0Compatible: true,
			v1Compatible: true,
			module: `package test
import future.keywords
p contains 3 if {
	input.x = 1
}`,
			query: `data.test.p`,
		},
		{
			note:         "v0+v1, rego.v1 imported",
			v0Compatible: true,
			v1Compatible: true,
			module: `package test
import rego.v1
p contains 3 if {
	input.x = 1
}`,
			query: `data.test.p`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			files := map[string]string{
				"test.rego": tc.module,
			}

			test.WithTempFS(files, func(rootPath string) {
				params := newDepsCommandParams()
				params.v0Compatible = tc.v0Compatible
				params.v1Compatible = tc.v1Compatible
				_ = params.outputFormat.Set(depsFormatPretty)

				for f := range files {
					_ = params.dataPaths.Set(filepath.Join(rootPath, f))
				}

				err := deps([]string{tc.query}, params, io.Discard)

				if len(tc.expErrs) > 0 {
					if err == nil {
						t.Fatalf("Expected error but got nil")
					}
					for _, expErr := range tc.expErrs {
						if !strings.Contains(err.Error(), expErr) {
							t.Fatalf("Expected error:\n\n%s\n\ngot:\n\n%s", expErr, err.Error())
						}
					}
				} else if err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}
			})
		})
	}
}

func TestDepsV1WithBundleRegoVersion(t *testing.T) {
	tests := []struct {
		note    string
		files   map[string]string
		query   string
		expErrs []string
	}{
		{
			note: "v0.x bundle, no keywords",
			files: map[string]string{
				".manifest": `{"rego_version": 0}`,
				"policy.rego": `package test
p[3] {
	input.x = 1
}`,
			},
			query: `data.test.p`,
		},
		{
			note: "v0.x bundle, keywords not imported, but used",
			files: map[string]string{
				".manifest": `{"rego_version": 0}`,
				"policy.rego": `package test
p contains 3 if {
	input.x = 1
}`,
			},
			query: `data.test.p`,
			expErrs: []string{
				"rego_parse_error: var cannot be used for rule name",
				"rego_parse_error: number cannot be used for rule name",
			},
		},
		{
			note: "v0.x bundle, keywords imported",
			files: map[string]string{
				".manifest": `{"rego_version": 0}`,
				"policy.rego": `package test
import future.keywords
p contains 3 if {
	input.x = 1
}`,
			},
			query: `data.test.p`,
		},
		{
			note: "v0.x bundle, rego.v1 imported",
			files: map[string]string{
				".manifest": `{"rego_version": 0}`,
				"policy.rego": `package test
import rego.v1
p contains 3 if {
	input.x = 1
}`,
			},
			query: `data.test.p`,
		},
		{
			note: "v0 bundle, v1 per-file override",
			files: map[string]string{
				".manifest": `{
	"rego_version": 0,
	"file_rego_versions": {
		"/policy2.rego": 1
	}
}`,
				"policy1.rego": `package test
p[3] {
	input.x = 1
}`,
				"policy2.rego": `package test
p contains 4 if {
	input.x = 1
}`,
			},
		},
		{
			note: "v0 bundle, v1 per-file override (glob)",
			files: map[string]string{
				".manifest": `{
	"rego_version": 0,
	"file_rego_versions": {
		"/bar/*.rego": 1
	}
}`,
				"foo/policy1.rego": `package test
p[3] {
	input.x = 1
}`,
				"bar/policy2.rego": `package test
p contains 4 if {
	input.x = 1
}`,
			},
		},
		{
			note: "v0 bundle, v1 per-file override, incompliant",
			files: map[string]string{
				".manifest": `{
	"rego_version": 0,
	"file_rego_versions": {
		"/policy2.rego": 1
	}
}`,
				"policy1.rego": `package test
p[3] {
	input.x = 1
}`,
				"policy2.rego": `package test
p[4] {
	input.x = 1
}`,
			},
			expErrs: []string{
				"rego_parse_error: `if` keyword is required before rule body",
				"rego_parse_error: `contains` keyword is required for partial set rules",
			},
		},
		{
			note: "v1.0 bundle, no keywords",
			files: map[string]string{
				".manifest": `{"rego_version": 1}`,
				"policy.rego": `package test
p[3] {
	input.x = 1
}`,
			},
			query: `data.test.p`,
			expErrs: []string{
				"rego_parse_error: `if` keyword is required before rule body",
				"rego_parse_error: `contains` keyword is required for partial set rules",
			},
		},
		{
			note: "v1.0 bundle, no keyword imports",
			files: map[string]string{
				".manifest": `{"rego_version": 1}`,
				"policy.rego": `package test
p contains 3 if {
	input.x = 1
}`,
			},
			query: `data.test.p`,
		},
		{
			note: "v1.0 bundle, keywords imported",
			files: map[string]string{
				".manifest": `{"rego_version": 1}`,
				"policy.rego": `package test
import future.keywords
p contains 3 if {
	input.x = 1
}`,
			},
			query: `data.test.p`,
		},
		{
			note: "v1.0 bundle, rego.v1 imported",
			files: map[string]string{
				".manifest": `{"rego_version": 1}`,
				"policy.rego": `package test
import rego.v1
p contains 3 if {
	input.x = 1
}`,
			},
			query: `data.test.p`,
		},
		{
			note: "v1 bundle, v0 per-file override",
			files: map[string]string{
				".manifest": `{
	"rego_version": 1,
	"file_rego_versions": {
		"/policy1.rego": 0
	}
}`,
				"policy1.rego": `package test
p[3] {
	input.x = 1
}`,
				"policy2.rego": `package test
p contains 4 if {
	input.x = 1
}`,
			},
		},
		{
			note: "v1 bundle, v0 per-file override (glob)",
			files: map[string]string{
				".manifest": `{
	"rego_version": 1,
	"file_rego_versions": {
		"/foo/*.rego": 0
	}
}`,
				"foo/policy1.rego": `package test
p[3] {
	input.x = 1
}`,
				"bar/policy2.rego": `package test
p contains 4 if {
	input.x = 1
}`,
			},
		},
		{
			note: "v1 bundle, v0 per-file override, incompliant",
			files: map[string]string{
				".manifest": `{
	"rego_version": 1,
	"file_rego_versions": {
		"/policy1.rego": 0
	}
}`,
				"policy1.rego": `package test
p contains 3 if {
	input.x = 1
}`,
				"policy2.rego": `package test
p contains 4 if {
	input.x = 1
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

						params := newDepsCommandParams()
						if err := params.bundlePaths.Set(p); err != nil {
							t.Fatalf("Unexpected error: %s", err)
						}

						params.v1Compatible = v1CompatibleFlag.used
						_ = params.outputFormat.Set(depsFormatPretty)

						err := deps([]string{tc.query}, params, io.Discard)

						if len(tc.expErrs) > 0 {
							if err == nil {
								t.Fatalf("Expected error but got nil")
							}
							for _, expErr := range tc.expErrs {
								if !strings.Contains(err.Error(), expErr) {
									t.Fatalf("Expected error:\n\n%s\n\ngot:\n\n%s", expErr, err.Error())
								}
							}
						} else if err != nil {
							t.Fatalf("Unexpected error: %v", err)
						}
					})
				})
			}
		}
	}
}
