// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"

	astJSON "github.com/open-policy-agent/opa/v1/ast/json"

	"github.com/open-policy-agent/opa/v1/test/compilecases"
	"github.com/open-policy-agent/opa/v1/test/conformance"
)

// TestCompileCases runs the compiler diagnostic corpus in v1/test/compilecases.
func TestCompileCases(t *testing.T) {
	// The AST marshalling options are global state. Compiler fixtures never pin
	// positions, so they are set once here rather than partitioned on as the
	// parser corpus has to do.
	defer astJSON.SetOptions(astJSON.GetOptions())
	astJSON.SetOptions(conformance.MarshalOptions(false, false))

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
		WithStrict(tc.StrictMode()).
		WithEnablePrintStatements(tc.PrintStatements)

	c.Compile(modules)

	got := make([]conformance.Error, 0, len(c.Errors))
	for _, e := range c.Errors {
		got = append(got, caseError(e))
	}

	// An absent want_errors is itself an assertion: nothing may be reported. That
	// is what compiles states explicitly, and what a transformation case relies on.
	if tc.Failure() {
		if len(got) == 0 {
			t.Fatalf("%s: expected compilation to fail, but it succeeded", tc.Filename)
		}
		assertCaseErrors(t, tc.Filename, tc.WantErrors, got, tc.Exhaustive)
	} else if len(got) > 0 {
		t.Fatalf("%s: expected the modules to compile, got:%s", tc.Filename, indented(got))
	}

	for i, want := range tc.Want {
		name := compilecases.ModuleName(i)
		got := c.Modules[name]

		if want.AST != "" {
			// Comments are not part of the assertion: the module is in the case
			// already, and holding an implementation to the shape OPA marshals a
			// comment in is the reason the parser corpus drops them too.
			got.Comments = nil

			bs, err := json.Marshal(got)
			if err != nil {
				t.Fatalf("%s: marshalling the compiled %s: %v", tc.Filename, name, err)
			}

			formatted, err := conformance.FormatAST(bs)
			if err != nil {
				t.Fatalf("%s: formatting the compiled %s: %v", tc.Filename, name, err)
			}

			if formatted != want.AST {
				t.Fatalf("%s: %s does not compile to want[%d].ast (-want, +got):\n%s",
					tc.Filename, name, i, cmp.Diff(want.AST, formatted))
			}
			continue
		}

		// The entry's own directive imports are put in effect. The compiler resolves
		// those away, so the expected module depends on them with no import left to
		// say so — and they do not carry across, so one module's
		// `future.keywords.not` must not reach its neighbour.
		declared, err := tc.WantParserOptions(i)
		if err != nil {
			t.Fatalf("%s: %v", tc.Filename, err)
		}

		wantVersion, err := caseRegoVersion(declared.RegoVersion)
		if err != nil {
			t.Fatalf("%s: %v", tc.Filename, err)
		}

		wantOpts := popts
		wantOpts.RegoVersion = wantVersion
		wantOpts.FutureKeywords = declared.FutureKeywords
		wantOpts.AllFutureKeywords = declared.AllFutureKeywords

		exp, err := ParseModuleWithOpts(name, want.Module, wantOpts)
		if err != nil {
			t.Fatalf("%s: want[%d].module does not parse: %v", tc.Filename, i, err)
		}

		if !got.Equal(exp) {
			t.Fatalf("%s: %s does not compile to want[%d].module\n--- want\n%v\n--- got\n%v",
				tc.Filename, name, i, exp, got)
		}
	}
}
