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

	"github.com/open-policy-agent/opa/bundle"
	"github.com/open-policy-agent/opa/util/test"
)

func TestDepsV1Compatible(t *testing.T) {
	tests := []struct {
		note         string
		v1Compatible bool
		module       string
		query        string
		expErrs      []string
	}{
		{
			note: "v0.x, no keywords",
			module: `package test
p[3] {
	input.x = 1
}`,
			query: `data.test.p`,
		},
		{
			note: "v0.x, keywords not imported, but used",
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
			note: "v0.x, keywords imported",
			module: `package test
import future.keywords
p contains 3 if {
	input.x = 1
}`,
			query: `data.test.p`,
		},
		{
			note: "v0.x, rego.v1 imported",
			module: `package test
import rego.v1
p contains 3 if {
	input.x = 1
}`,
			query: `data.test.p`,
		},
		{
			note:         "v1.0, no keywords",
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
			note:         "v1.0, no keyword imports",
			v1Compatible: true,
			module: `package test
p contains 3 if {
	input.x = 1
}`,
			query: `data.test.p`,
		},
		{
			note:         "v1.0, keywords imported",
			v1Compatible: true,
			module: `package test
import future.keywords
p contains 3 if {
	input.x = 1
}`,
			query: `data.test.p`,
		},
		{
			note:         "v1.0, rego.v1 imported",
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
				} else {
					if err != nil {
						t.Fatalf("Unexpected error: %v", err)
					}
				}
			})
		})
	}
}

func TestDepsV1WithBundleRegoVersion(t *testing.T) {
	tests := []struct {
		note              string
		bundleRegoVersion int
		module            string
		query             string
		expErrs           []string
	}{
		{
			note:              "v0.x bundle, no keywords",
			bundleRegoVersion: 0,
			module: `package test
p[3] {
	input.x = 1
}`,
			query: `data.test.p`,
		},
		{
			note:              "v0.x bundle, keywords not imported, but used",
			bundleRegoVersion: 0,
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
			note:              "v0.x bundle, keywords imported",
			bundleRegoVersion: 0,
			module: `package test
import future.keywords
p contains 3 if {
	input.x = 1
}`,
			query: `data.test.p`,
		},
		{
			note:              "v0.x bundle, rego.v1 imported",
			bundleRegoVersion: 0,
			module: `package test
import rego.v1
p contains 3 if {
	input.x = 1
}`,
			query: `data.test.p`,
		},
		{
			note:              "v1.0 bundle, no keywords",
			bundleRegoVersion: 1,
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
			note:              "v1.0 bundle, no keyword imports",
			bundleRegoVersion: 1,
			module: `package test
p contains 3 if {
	input.x = 1
}`,
			query: `data.test.p`,
		},
		{
			note:              "v1.0 bundle, keywords imported",
			bundleRegoVersion: 1,
			module: `package test
import future.keywords
p contains 3 if {
	input.x = 1
}`,
			query: `data.test.p`,
		},
		{
			note:              "v1.0 bundle, rego.v1 imported",
			bundleRegoVersion: 1,
			module: `package test
import rego.v1
p contains 3 if {
	input.x = 1
}`,
			query: `data.test.p`,
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
						files["test.rego"] = tc.module
						files[".manifest"] = fmt.Sprintf(`{"rego_version": %d}`, tc.bundleRegoVersion)
					}

					test.WithTempFS(files, func(root string) {
						p := root
						if bundleType.tar {
							p = filepath.Join(root, "bundle.tar.gz")
							b := bundle.Bundle{
								Manifest: bundle.Manifest{RegoVersion: &tc.bundleRegoVersion},
								Data:     map[string]interface{}{},
								Modules: []bundle.ModuleFile{
									{
										Path: "test.rego",
										Raw:  []byte(tc.module),
									},
								},
							}
							p = filepath.Join(root, "bundle.tar.gz")
							f, err := os.OpenFile(p, os.O_WRONLY, os.ModePerm)
							if err != nil {
								t.Fatalf("Unexpected error: %s", err)
							}
							err = bundle.Write(f, b)
							if err != nil {
								t.Fatalf("Unexpected error: %s", err)
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
						} else {
							if err != nil {
								t.Fatalf("Unexpected error: %v", err)
							}
						}
					})
				})
			}
		}
	}
}
