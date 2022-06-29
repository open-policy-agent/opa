// Copyright 2021 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

//go:build go1.18
// +build go1.18

package ast

import "testing"

func FuzzParseStatementsAndCompileModules(f *testing.F) {
	f.Fuzz(func(t *testing.T, input string) {
		t.Parallel() // seed corpus tests can run in parallel
		_, _, err := ParseStatements("", input)
		if err == nil {
			// CompileModules is expected to error, but it shouldn't panic
			CompileModules(map[string]string{"": input}) //nolint
		}
	})
}
