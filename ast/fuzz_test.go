// Copyright 2021 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// nolint
package ast

import (
	"testing"

	"github.com/open-policy-agent/opa/test/cases"
)

var testcases = cases.MustLoad("../test/cases/testdata").Sorted().Cases

func FuzzCompileModules(f *testing.F) {
	for _, tc := range testcases {
		for _, mod := range tc.Modules {
			f.Add(mod)
		}
	}
	f.Fuzz(func(t *testing.T, input string) {
		t.Parallel()
		CompileModules(map[string]string{"": input})
	})
}

func FuzzCompileModulesWithPrintAndAllFutureKWs(f *testing.F) {
	for _, tc := range testcases {
		for _, mod := range tc.Modules {
			f.Add(mod)
		}
	}
	f.Fuzz(func(t *testing.T, input string) {
		t.Parallel()
		CompileModulesWithOpt(map[string]string{"": input}, CompileOpts{
			EnablePrintStatements: true,
			ParserOptions:         ParserOptions{AllFutureKeywords: true},
		})
	})
}
