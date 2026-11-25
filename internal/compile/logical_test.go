// Copyright 2026 The OPA Authors
// SPDX-License-Identifier: Apache-2.0

package compile

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/rego"
)

func TestFlattenLogical(t *testing.T) {
	inPopts := ast.ParserOptions{FutureKeywords: []string{"not", "and", "or"}}

	tests := []struct {
		note  string
		in    string
		exp   []string
		expV2 bool // parse exp as rego v2: it keeps a not body
	}{
		{
			note: "no logical expressions",
			in:   `input.a = 1; input.b = 2`,
			exp:  []string{`input.a = 1; input.b = 2`},
		},
		{
			note: "not expression",
			in:   `not input.a = 1`,
			exp:  []string{`not input.a = 1`},
		},
		{
			note: "not body",
			in:   "not {\n input.a = 1\n input.b = 2\n}",
			exp:  []string{`not input.a = 1`, `not input.b = 2`},
		},
		{
			note: "or",
			in:   `input.a = 1 or input.b = 2`,
			exp:  []string{`input.a = 1`, `input.b = 2`},
		},
		{
			note: "or of not",
			in:   `not input.a = 1 or input.b = 2`,
			exp:  []string{`not input.a = 1`, `input.b = 2`},
		},
		{
			note: "or in conjunction",
			in:   `input.c = 3; input.a = 1 or input.b = 2`,
			exp:  []string{`input.c = 3; input.a = 1`, `input.c = 3; input.b = 2`},
		},
		{
			note: "and",
			in:   `input.a = 1 and input.b = 2`,
			exp:  []string{`input.a = 1; input.b = 2`},
		},
		{
			note: "not or",
			in:   `not (input.a = 1 or input.b = 2)`,
			exp:  []string{`not input.a = 1; not input.b = 2`},
		},
		{
			note: "not and",
			in:   `not (input.a = 1 and input.b = 2)`,
			exp:  []string{`not input.a = 1`, `not input.b = 2`},
		},
		{
			note: "not body over not body",
			in:   "not {\n not {\n  input.a = 1\n  input.b = 2\n }\n}",
			exp:  []string{`input.a = 1; input.b = 2`},
		},
		{
			note: "not body over not body over or",
			in:   "not {\n not {\n  input.a = 1 or input.b = 2\n }\n}",
			exp:  []string{`input.a = 1`, `input.b = 2`},
		},
		{
			note:  "not body sharing a variable through refs",
			in:    "not {\n input.items[i].a = 1\n input.items[i].b = 2\n}",
			exp:   []string{"not {input.items[i].a = 1; input.items[i].b = 2}"},
			expV2: true,
		},
		{
			note:  "not body sharing a bound variable",
			in:    "not {\n x = input.items[_]\n x.a = 1\n x.b = 2\n}",
			exp:   []string{"not {x = input.items[_]; x.a = 1; x.b = 2}"},
			expV2: true,
		},
		{
			note:  "with is kept for the checks",
			in:    `(input.a = 1 or input.b = 2) with input.x as 1`,
			exp:   []string{`(input.a = 1 or input.b = 2) with input.x as 1`},
			expV2: true,
		},
		{
			note:  "not body sharing a variable",
			in:    "not {\n input.a[i] = 1\n input.b[i] = 2\n}",
			exp:   []string{"not {input.a[i] = 1; input.b[i] = 2}"},
			expV2: true,
		},
		{
			note:  "or operand sharing a variable, negated",
			in:    "input.c = 3; not (input.d = 4 or {input.a[i] = 1; input.b[i] = 2})",
			exp:   []string{"input.c = 3; not (input.d = 4 or {input.a[i] = 1; input.b[i] = 2})"},
			expV2: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			in := ast.MustParseBodyWithOpts(tc.in, inPopts)
			exp := make([]ast.Body, len(tc.exp))
			for i := range tc.exp {
				// expectations are rego v1: `not` is a plain negated expression
				expPopts := ast.ParserOptions{}
				if tc.expV2 {
					expPopts = inPopts
				}
				exp[i] = ast.MustParseBodyWithOpts(tc.exp[i], expPopts)
			}

			act, err := FlattenLogical([]ast.Body{in}, nil)
			if err != nil {
				t.Fatal(err)
			}

			if len(act) != len(exp) {
				t.Fatalf("expected %d queries, got %d: %v", len(exp), len(act), act)
			}
			for i := range act {
				if act[i].String() != exp[i].String() { // not Equal: that compares positions, too
					t.Errorf("query %d: expected %v, got %v", i, exp[i], act[i])
				}
			}
		})
	}
}

// A local that the flattening can't replace must fail the checks: translation
// needs a field operand.
func TestCheckComparisonWithoutField(t *testing.T) {
	pq := &rego.PartialQueries{Queries: []ast.Body{ast.MustParseBody(`__local0__1 = lower(input.doc.name); gt(__local0__1, 0)`)}}
	cons, _ := NewConstraints("ucast", "all")
	errs := Check(pq, NewConstraintSet(cons), nil).ASTErrors()
	if !slices.ContainsFunc(errs, func(e *ast.Error) bool { return strings.Contains(e.Message, "no field operand") }) {
		t.Fatalf("expected a no field operand error, got %v", errs)
	}
}

func TestFlattenLogicalTooBig(t *testing.T) {
	popts := ast.ParserOptions{FutureKeywords: []string{"not", "and", "or"}}
	ors := func(n int) string {
		var exprs []string
		for i := range n {
			exprs = append(exprs, fmt.Sprintf("(input.a%d = 1 or input.b%d = 2)", i, i))
		}
		return strings.Join(exprs, "; ")
	}
	// not ((a0 and b0) or (a1 and b1) ...) is (not a0 or not b0) and (not a1 or not b1) ...
	negatedAnds := func(n int) string {
		return "not (" + strings.ReplaceAll(strings.ReplaceAll(ors(n), " or ", " and "), "; ", " or ") + ")"
	}
	tests := []struct {
		note string
		in   string
		exp  int // disjuncts; 0 for the error
	}{
		{note: "ten ors", in: ors(10), exp: 1024},
		{note: "eleven ors", in: ors(11)},
		{note: "negated, ten", in: negatedAnds(10), exp: 1024},
		{note: "negated, eleven", in: negatedAnds(11)},
	}
	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			act, err := FlattenLogical([]ast.Body{ast.MustParseBodyWithOpts(tc.in, popts)}, nil)
			switch {
			case tc.exp == 0 && (err == nil || !strings.Contains(err.Message, "more than 1024 alternatives")):
				t.Fatalf("expected the too many alternatives error, got %v and %d disjuncts", err, len(act))
			case tc.exp != 0 && (err != nil || len(act) != tc.exp):
				t.Fatalf("expected %d disjuncts, got %d, %v", tc.exp, len(act), err)
			}
		})
	}
}

// Partial evaluation's own queries don't count against the limit, only what one
// of them expands into.
func TestFlattenLogicalManyQueries(t *testing.T) {
	queries := make([]ast.Body, 2000)
	for i := range queries {
		queries[i] = ast.MustParseBody(fmt.Sprintf("input.a = %d", i))
	}
	act, err := FlattenLogical(queries, nil)
	if err != nil || len(act) != len(queries) {
		t.Fatalf("expected %d queries, got %d, %v", len(queries), len(act), err)
	}
}

func TestFlattenLogicalSupport(t *testing.T) {
	popts := ast.ParserOptions{FutureKeywords: []string{"not", "and", "or"}}
	tests := []struct {
		note    string
		support string
		exp     []string // rego v1
	}{
		{
			note:    "one body",
			support: "__not1__ if input.a < 2",
			exp:     []string{"not lt(input.a, 2)"},
		},
		{
			note:    "two bodies",
			support: "__not1__ if input.a < 2\n__not1__ if startswith(input.b, \"x\")",
			exp:     []string{`not lt(input.a, 2); not startswith(input.b, "x")`},
		},
		{
			note:    "body with two expressions",
			support: "__not1__ if { input.a < 2; input.b = 1 }",
			exp:     []string{"not lt(input.a, 2)", "not input.b = 1"},
		},
		{
			note:    "a value other than true",
			support: "__not1__ := 5 if input.a < 2",
		},
		{
			note:    "a default",
			support: "default __not1__ := false\n__not1__ if input.a < 2",
		},
	}
	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			support := ast.MustParseModuleWithOpts("package partial\n"+tc.support, ast.ParserOptions{RegoVersion: ast.RegoV1})
			act, err := FlattenLogical([]ast.Body{ast.MustParseBodyWithOpts("not data.partial.__not1__", popts)}, []*ast.Module{support})
			if err != nil {
				t.Fatal(err)
			}
			if tc.exp == nil { // left for the checks to reject
				if len(act) != 1 || act[0].String() != "not data.partial.__not1__" {
					t.Fatalf("expected the negation to be kept, got %v", act)
				}
				return
			}
			if len(act) != len(tc.exp) {
				t.Fatalf("expected %d queries, got %v", len(tc.exp), act)
			}
			for i := range act {
				if exp := ast.MustParseBody(tc.exp[i]); act[i].String() != exp.String() {
					t.Errorf("query %d: expected %v, got %v", i, exp, act[i])
				}
			}
		})
	}
}
