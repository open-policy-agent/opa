// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"testing"

	"github.com/open-policy-agent/opa/v1/test/compilecases"
	"github.com/open-policy-agent/opa/v1/test/conformance"
)

// TestCompileCases runs the compiler diagnostic corpus in v1/test/compilecases.
func TestCompileCases(t *testing.T) {
	for _, dir := range []string{"v0", "v1"} {
		for _, tc := range compilecases.MustLoad("../test/compilecases/testdata/" + dir).Sorted().Cases {
			t.Run(dir+"/"+tc.Note, func(t *testing.T) {
				runCompileCase(t, tc)
			})
		}
	}
}

func runCompileCase(t *testing.T, tc compilecases.TestCase) {
	t.Helper()

	if len(tc.WantErrors) == 0 {
		t.Fatalf("%s: expected at least one entry in 'want_errors'", tc.Filename)
	}

	regoVersion, err := caseRegoVersion(tc.RegoVersion)
	if err != nil {
		t.Fatalf("%s: %v", tc.Filename, err)
	}

	popts := ParserOptions{RegoVersion: regoVersion}
	if tc.ExperimentalKeywords {
		popts.Capabilities = CapabilitiesForThisVersion(CapabilitiesExperimentalKeywords(true))
	}

	modules := make(map[string]*Module, len(tc.Modules))
	for i, module := range tc.Modules {
		name := compilecases.ModuleName(i)
		parsed, err := ParseModuleWithOpts(name, module, popts)
		if err != nil {
			// Parse errors are reported by their own stage, and are covered by
			// the parser corpus in v1/test/parsercases.
			t.Fatalf("unexpected parse error: %v", err)
		}
		modules[name] = parsed
	}

	// Without lifting the limit the compiler stops at CompileErrorLimitDefault
	// and appends a "too many errors" diagnostic of its own, so a case with more
	// than ten would be matched against a truncated set. The generator lifts it
	// for the same reason.
	c := NewCompiler().
		SetErrorLimit(0).
		WithStrict(tc.Strict).
		WithEnablePrintStatements(tc.PrintStatements)

	c.Compile(modules)

	if !c.Failed() {
		t.Fatal("expected compilation to fail, but it succeeded")
	}

	got := make([]conformance.Error, 0, len(c.Errors))
	for _, e := range c.Errors {
		got = append(got, caseError(e))
	}

	assertCaseErrors(t, tc.Filename, tc.WantErrors, got, tc.Exhaustive)
}
