// Copyright 2024 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"strings"
	"testing"
)

func TestCompile_DefaultRegoVersion(t *testing.T) {
	tests := []struct {
		note    string
		modules map[string]*Module
		expErrs []string
	}{
		{
			note: "no module rego-version, no v1 violations",
			modules: map[string]*Module{
				"test": {
					Package: MustParsePackage(`package test`),
					Imports: MustParseImports(`import data.foo
						import data.bar`),
				},
			},
		},
		{
			note: "no module rego-version, v1 violations", // default is v0, no errors expected
			modules: map[string]*Module{
				"test": {
					Package: MustParsePackage(`package test`),
					Imports: MustParseImports(`import data.foo
						import data.bar as foo`),
				},
			},
		},
		{
			note: "v0 module, v1 violations",
			modules: map[string]*Module{
				"test": MustParseModuleWithOpts(`package test
						import data.foo
						import data.bar as foo`,
					ParserOptions{RegoVersion: RegoV0}),
			},
		},
		{
			note: "v1 module, v1 violations",
			modules: map[string]*Module{
				"test": MustParseModuleWithOpts(`package test
						import data.foo
						import data.bar as foo`,
					ParserOptions{RegoVersion: RegoV1}),
			},
			expErrs: []string{
				"3:7: rego_compile_error: import must not shadow import data.foo",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			compiler := NewCompiler()

			compiler.Compile(tc.modules)

			if len(tc.expErrs) > 0 {
				assertErrors(t, compiler.Errors, tc.expErrs)
			} else if len(compiler.Errors) > 0 {
				t.Fatalf("Unexpected errors: %v", compiler.Errors)
			}
		})
	}
}

func assertErrors(t *testing.T, actual Errors, expected []string) {
	t.Helper()
	if len(expected) != len(actual) {
		t.Fatalf("Expected %d errors, got %d:\n\n%s\n", len(expected), len(actual), actual.Error())
	}
	incorrectErrs := false
	for _, e := range expected {
		found := false
		for _, actual := range actual {
			if strings.Contains(actual.Error(), e) {
				found = true
				break
			}
		}
		if !found {
			incorrectErrs = true
		}
	}
	if incorrectErrs {
		t.Fatalf("Expected errors:\n\n%s\n\nGot:\n\n%s\n", expected, actual.Error())
	}
}
