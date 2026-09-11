// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"encoding/json"
	"slices"
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

	c := compileCaseModules(t, tc, popts, "")

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

	assertCaseWant(t, tc, popts, c, "")

	// want_stages is additive: the full-pipeline assertions above stand on their
	// own, and a consumer without OPA's stages ignores everything below.
	for _, stage := range tc.SortedStages() {
		sc := compileCaseModules(t, tc, popts, StageID(stage))

		// The field asserts a form, which there is only one of if the pipeline got
		// that far cleanly. A case whose diagnostics are raised before the stage it
		// pins has nothing to say here.
		if len(sc.Errors) > 0 {
			reported := make([]conformance.Error, 0, len(sc.Errors))
			for _, e := range sc.Errors {
				reported = append(reported, caseError(e))
			}
			t.Fatalf("%s: compiling up to %s reports:%s", tc.Filename, stage, indented(reported))
		}

		assertCaseWant(t, tc, popts, sc, stage)
	}
}

// compileCaseModules parses the case's modules and compiles them, stopping after
// stage when one is named.
//
// The parse is repeated per compilation rather than shared: the stages rewrite the
// modules in place, so a second run over the same ASTs would start from the first
// run's output.
//
// The stage is checked against AllStages() first, because WithOnlyStagesUpTo runs
// the whole pipeline when it does not recognise its argument — so a name the corpus
// carries but the compiler no longer has would silently be asserted against the
// full-pipeline form.
func compileCaseModules(t *testing.T, tc compilecases.TestCase, popts ParserOptions, stage StageID) *Compiler {
	t.Helper()

	if stage != "" && !slices.Contains(AllStages(), stage) {
		t.Fatalf("%s: %q is not one of the compiler's stages", tc.Filename, stage)
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

	if stage != "" {
		c = c.WithOnlyStagesUpTo(stage)
	}

	c.Compile(modules)

	return c
}

// assertCaseWant compares the compiled modules against the case's expectations —
// the full-pipeline want where stage is empty, the one pinned to stage otherwise.
func assertCaseWant(t *testing.T, tc compilecases.TestCase, popts ParserOptions, c *Compiler, stage string) {
	t.Helper()

	want, field := tc.Want, "want"
	if stage != "" {
		want, field = tc.WantStages[stage], "want_stages."+stage
	}

	for i, want := range want {
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
				t.Fatalf("%s: %s does not compile to %s[%d].ast (-want, +got):\n%s",
					tc.Filename, name, field, i, cmp.Diff(want.AST, formatted))
			}
			continue
		}

		// The entry's own directive imports are put in effect. The compiler resolves
		// those away, so the expected module depends on them with no import left to
		// say so — and they do not carry across, so one module's
		// `future.keywords.not` must not reach its neighbour.
		declared, err := tc.WantParserOptions(i)
		if stage != "" {
			declared, err = tc.WantStageParserOptions(stage, i)
		}
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

		// Annotations are built from METADATA comments by the compiler and compared by
		// Module.Compare, so the expected module has to be read with them processed.
		wantOpts.ProcessAnnotation = true

		exp, err := ParseModuleWithOpts(name, want.Module, wantOpts)
		if err != nil {
			t.Fatalf("%s: %s[%d].module does not parse: %v", tc.Filename, field, i, err)
		}

		if !got.Equal(exp) {
			t.Fatalf("%s: %s does not compile to %s[%d].module\n--- want\n%v\n--- got\n%v",
				tc.Filename, name, field, i, exp, got)
		}
	}
}

// TestCorpusStagesMatchCompiler keeps compilecases.Stages agreeing with the
// compiler's own list.
//
// This is not the check that catches a stage rename: the fixtures are. Updating
// Stages is what makes a committed want_stages key unknown, which Validate rejects
// in both the runner and the generator, and both also check the name against
// AllStages() before compiling. What this test buys is that the two lists cannot
// disagree — so those checks cannot contradict each other, and an author can pin any
// stage the compiler really has.
func TestCorpusStagesMatchCompiler(t *testing.T) {
	got := make([]string, 0, len(AllStages()))
	for _, s := range AllStages() {
		got = append(got, string(s))
	}

	if diff := cmp.Diff(compilecases.Stages, got); diff != "" {
		t.Fatalf("compilecases.Stages is out of step with ast.AllStages() (-corpus, +compiler):\n%s", diff)
	}
}
