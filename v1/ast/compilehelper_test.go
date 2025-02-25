// Copyright 2024 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"strings"
	"testing"
)

func TestCompileModules_DefaultRegoVersion(t *testing.T) {
	tests := []struct {
		note    string
		modules map[string]string
		expErrs []string
	}{
		// NOT default rego-version
		{
			note: "v0 module, no v1 compile-time violations",
			modules: map[string]string{
				"test.rego": `package test
					p[x] { 
						x = "a" 
					}`,
			},
			expErrs: []string{
				"test.rego:2: rego_parse_error: `if` keyword is required before rule body",
				"test.rego:2: rego_parse_error: `contains` keyword is required for partial set rules",
			},
		},
		{
			note: "v0 module, v1 compile-time violations",
			modules: map[string]string{
				"test.rego": `package test
					import data.foo
					import data.bar as foo

					p[x] { 
						x = "a" 
					}`,
			},
			expErrs: []string{
				"test.rego:5: rego_parse_error: `if` keyword is required before rule body",
				"test.rego:5: rego_parse_error: `contains` keyword is required for partial set rules",
			},
		},

		// cross-rego-version
		{
			note: "rego.v1 import, no v1 compile-time violations",
			modules: map[string]string{
				"test.rego": `package test
					import rego.v1

					p contains x if { 
						x = "a" 
					}`,
			},
		},
		{
			note: "rego.v1 import, v1 compile-time violations",
			modules: map[string]string{
				"test.rego": `package test
					import rego.v1

					import data.foo
					import data.bar as foo

					p contains x if { 
						x = "a" 
					}`,
			},
			expErrs: []string{
				"test.rego:5: rego_compile_error: import must not shadow import data.foo",
			},
		},

		// default rego-version
		{
			note: "v1 module, no v1 compile-time violations",
			modules: map[string]string{
				"test.rego": `package test
					p contains x if { 
						x = "a" 
					}`,
			},
		},
		{
			note: "v1 module, v1 compile-time violations",
			modules: map[string]string{
				"test.rego": `package test
					import data.foo
					import data.bar as foo

					p contains x if { 
						x = "a" 
					}`,
			},
			expErrs: []string{
				"test.rego:3: rego_compile_error: import must not shadow import data.foo",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			_, err := CompileModules(tc.modules)

			if len(tc.expErrs) > 0 {
				for _, expErr := range tc.expErrs {
					if err := err.Error(); !strings.Contains(err, expErr) {
						t.Fatalf("Expected error to contain:\n\n%s\n\nbut got:\n\n%s", expErr, err)
					}
				}
			} else if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
		})
	}
}

func TestCompileModulesWithOpt_DefaultRegoVersion(t *testing.T) {
	tests := []struct {
		note    string
		modules map[string]string
		expErrs []string
	}{
		// NOT default rego-version
		{
			note: "v0 module, no v1 compile-time violations",
			modules: map[string]string{
				"test.rego": `package test
					p[x] { 
						x = "a" 
					}`,
			},
			expErrs: []string{
				"test.rego:2: rego_parse_error: `if` keyword is required before rule body",
				"test.rego:2: rego_parse_error: `contains` keyword is required for partial set rules",
			},
		},
		{
			note: "v0 module, v1 compile-time violations",
			modules: map[string]string{
				"test.rego": `package test
					import data.foo
					import data.bar as foo

					p[x] { 
						x = "a" 
					}`,
			},
			expErrs: []string{
				"test.rego:5: rego_parse_error: `if` keyword is required before rule body",
				"test.rego:5: rego_parse_error: `contains` keyword is required for partial set rules",
			},
		},

		// cross-rego-version
		{
			note: "rego.v1 import, no v1 compile-time violations",
			modules: map[string]string{
				"test.rego": `package test
					import rego.v1

					p contains x if { 
						x = "a" 
					}`,
			},
		},
		{
			note: "rego.v1 import, v1 compile-time violations",
			modules: map[string]string{
				"test.rego": `package test
					import rego.v1

					import data.foo
					import data.bar as foo

					p contains x if { 
						x = "a" 
					}`,
			},
			expErrs: []string{
				"test.rego:5: rego_compile_error: import must not shadow import data.foo",
			},
		},

		// default rego-version
		{
			note: "v1 module, no v1 compile-time violations",
			modules: map[string]string{
				"test.rego": `package test
					p contains x if { 
						x = "a" 
					}`,
			},
		},
		{
			note: "v1 module, v1 compile-time violations",
			modules: map[string]string{
				"test.rego": `package test
					import data.foo
					import data.bar as foo

					p contains x if { 
						x = "a" 
					}`,
			},
			expErrs: []string{
				"test.rego:3: rego_compile_error: import must not shadow import data.foo",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			_, err := CompileModulesWithOpt(tc.modules, CompileOpts{EnablePrintStatements: true})

			if len(tc.expErrs) > 0 {
				for _, expErr := range tc.expErrs {
					if err := err.Error(); !strings.Contains(err, expErr) {
						t.Fatalf("Expected error to contain:\n\n%s\n\nbut got:\n\n%s", expErr, err)
					}
				}
			} else if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
		})
	}
}
