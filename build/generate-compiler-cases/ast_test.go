// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cases

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/open-policy-agent/opa/build/internal/corpusgen"
	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/test/compilecases"
)

func TestWithAST(t *testing.T) {
	plain, err := LoadCompilerTestCases()
	if err != nil {
		t.Fatal(err)
	}

	marshalled, err := LoadCompilerTestCases(WithAST())
	if err != nil {
		t.Fatal(err)
	}

	var entries, stages, queries, committed, unmarshallable, unreadable int

	for i, set := range marshalled {
		for j, tc := range set.Cases {
			ref := plain[i].Cases[j]
			if tc.Note != ref.Note {
				t.Fatalf("expected the option to preserve order, got %q where %q was", tc.Note, ref.Note)
			}

			// A compiled form OPA cannot marshal; see the note on errNotMarshallable.
			if tc.ASTError != "" {
				unmarshallable++
				continue
			}

			if tc.QueryCase() {
				if ref.Query.Want == "" && ref.Query.WantAST == "" {
					if tc.Query.WantAST != "" {
						t.Errorf("%s: the query reports diagnostics, so it has no compiled form", tc.Note)
					}
					continue
				}

				queries++
				checkAST(t, tc.Note, "query.want_ast", tc.Query.WantAST)
				if tc.Query.Want != ref.Query.Want {
					t.Errorf("%s: expected query.want to survive, got %q", tc.Note, tc.Query.Want)
				}
				if ref.Query.WantAST != "" && tc.Query.WantAST != ref.Query.WantAST {
					t.Errorf("%s: the generated query AST differs from the committed one", tc.Note)
				}
				continue
			}

			for k := range tc.Want {
				entries++
				if ref.Want[k].AST != "" {
					committed++
				}
				unreadable += checkEntry(t, tc.TestCase, "want", k, tc.Want[k], ref.Want[k])
			}

			for _, stage := range tc.SortedStages() {
				for k := range tc.WantStages[stage] {
					stages++
					unreadable += checkEntry(t, tc.TestCase, "want_stages."+stage, k,
						tc.WantStages[stage][k], ref.WantStages[stage][k])
				}
			}
		}
	}

	if entries == 0 || stages == 0 || queries == 0 || committed == 0 {
		t.Errorf("expected the option to be exercised on every kind of expectation, filled %d want, %d want_stages, %d query, %d already committed",
			entries, stages, queries, committed)
	}

	t.Logf("%d case(s) OPA cannot marshal, %d entr(ies) it cannot read back", unmarshallable, unreadable)
}

// checkEntry holds one filled entry to what the option promises: the AST is there and
// is the same assertion the Rego spelling makes, and nothing the case committed moved.
// It returns 1 where the AST is one OPA cannot read back, which is a defect in OPA
// rather than in the entry.
func checkEntry(t *testing.T, tc compilecases.TestCase, field string, i int, got, want compilecases.Want) int {
	t.Helper()

	at := tc.Note + ": " + field
	checkAST(t, at, "ast", got.AST)

	if got.Module != want.Module {
		t.Errorf("%s[%d]: expected module to survive, got %q", at, i, got.Module)
	}
	if want.AST != "" && got.AST != want.AST {
		t.Errorf("%s[%d]: the generated AST differs from the committed one", at, i)
	}
	if got.Module == "" {
		return 0
	}

	popts, err := wantOptions(tc, field, i)
	if err != nil {
		t.Fatalf("%s[%d]: %v", at, i, err)
	}

	mod, err := ast.ParseModuleWithOpts(compilecases.ModuleName(i), got.Module, popts)
	if err != nil {
		t.Fatalf("%s[%d]: %v", at, i, err)
	}

	// The two spellings are one assertion, which is what lets a consumer with both a
	// printer and an AST cross-check them. Not byte equality: the marshalled form also
	// carries the compiler's own `generated` markers, which Rego cannot spell.
	var unmarshalled ast.Module
	if err := json.Unmarshal([]byte(got.AST), &unmarshalled); err != nil {
		t.Logf("%s[%d]: the AST does not read back: %v", at, i, err)
		return 1
	}
	if !unmarshalled.Equal(mod) {
		t.Errorf("%s[%d]: the AST is not what the module spells out", at, i)
	}

	return 0
}

// wantOptions is how the entry's module is read, whether it belongs to want or to a
// stage.
func wantOptions(tc compilecases.TestCase, field string, i int) (ast.ParserOptions, error) {
	opts, err := tc.WantParserOptions(i)
	if stage, ok := stageOf(field); ok {
		opts, err = tc.WantStageParserOptions(stage, i)
	}
	if err != nil {
		return ast.ParserOptions{}, err
	}

	version, err := corpusgen.RegoVersion(opts.RegoVersion)
	if err != nil {
		return ast.ParserOptions{}, err
	}

	popts := ast.ParserOptions{
		RegoVersion:       version,
		FutureKeywords:    opts.FutureKeywords,
		AllFutureKeywords: opts.AllFutureKeywords,
		ProcessAnnotation: true,
	}
	if tc.ExperimentalKeywords {
		popts.Capabilities = ast.CapabilitiesForThisVersion(ast.CapabilitiesExperimentalKeywords(true))
	}

	return popts, nil
}

func stageOf(field string) (string, bool) {
	const prefix = "want_stages."
	if len(field) <= len(prefix) || field[:len(prefix)] != prefix {
		return "", false
	}
	return field[len(prefix):], true
}

func checkAST(t *testing.T, note, field, marshalled string) {
	t.Helper()

	if marshalled == "" {
		t.Errorf("%s: expected %s to be filled in", note, field)
		return
	}

	var doc any
	if err := json.Unmarshal([]byte(marshalled), &doc); err != nil {
		t.Errorf("%s: %s is not JSON: %v", note, field, err)
	}
}

// TestWithASTIsDeterministic guards the generated artifact the way the committed ones
// are guarded by regenerating and diffing.
func TestWithASTIsDeterministic(t *testing.T) {
	first, err := LoadCompilerTestCases(WithAST())
	if err != nil {
		t.Fatal(err)
	}

	second, err := LoadCompilerTestCases(WithAST())
	if err != nil {
		t.Fatal(err)
	}

	a, err := json.Marshal(first)
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(second)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(a, b) {
		t.Error("expected two loads to generate the same ASTs")
	}
}
