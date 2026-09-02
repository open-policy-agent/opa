// Copyright 2026 The OPA Authors. All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.
package oracle

import (
	"errors"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
)

// cursorMarker marks the position to query for in a test module. It is removed
// from the module before parsing, and its byte offset becomes the query position.
const cursorMarker = "‸"

func logicalParserOpts() ast.ParserOptions {
	return ast.ParserOptions{
		FutureKeywords: []string{"and", "or", "not"},
	}
}

func cursorOffset(t *testing.T, module string) (string, int) {
	t.Helper()

	pos := strings.Index(module, cursorMarker)
	if pos < 0 {
		t.Fatalf("module is missing the %q cursor marker", cursorMarker)
	}

	return strings.Replace(module, cursorMarker, "", 1), pos
}

func TestOracleFindDefinitionLogical(t *testing.T) {
	t.Parallel()

	cases := []struct {
		note    string
		opts    *ast.ParserOptions // defaults to logicalParserOpts()
		modules map[string]string  // buffer.rego holds the cursor marker
		exp     *ast.Location
	}{
		{
			note: "rule ref in implicit lhs operand",
			modules: map[string]string{
				"buffer.rego": `package test

p if {
	‸q or r
}

q := 1

r := 2`,
			},
			exp: &ast.Location{File: "buffer.rego", Row: 7, Col: 1, Text: []byte("q := 1")},
		},
		{
			note: "rule ref in implicit rhs operand",
			modules: map[string]string{
				"buffer.rego": `package test

p if {
	q or ‸r
}

q := 1

r := 2`,
			},
			exp: &ast.Location{File: "buffer.rego", Row: 9, Col: 1, Text: []byte("r := 2")},
		},
		{
			note: "rule ref in explicit operand body",
			modules: map[string]string{
				"buffer.rego": `package test

p if {
	{‸q} and {r}
}

q := 1

r := 2`,
			},
			exp: &ast.Location{File: "buffer.rego", Row: 7, Col: 1, Text: []byte("q := 1")},
		},
		{
			note: "outer var referenced from operand body",
			modules: map[string]string{
				"buffer.rego": `package test

p if {
	x := 1
	{‸x > 0} and {x < 5}
}`,
			},
			exp: &ast.Location{File: "buffer.rego", Row: 4, Col: 2, Text: []byte("x")},
		},
		{
			note: "operand-local assignment",
			modules: map[string]string{
				"buffer.rego": `package test

p if {
	{y := 1; ‸y > 0} and q
}

q := 2`,
			},
			exp: &ast.Location{File: "buffer.rego", Row: 4, Col: 3, Text: []byte("y")},
		},
		{
			note: "assignment in enclosing operand body",
			modules: map[string]string{
				"buffer.rego": `package test

p if {
	{y := 1; {‸y > 0} and true} or false
}`,
			},
			exp: &ast.Location{File: "buffer.rego", Row: 4, Col: 3, Text: []byte("y")},
		},
		{
			note: "assignment in sibling operand body is not visible",
			modules: map[string]string{
				"buffer.rego": `package test

p if {
	{x := 1; x > 0} or {x := 2; ‸x > 1}
}`,
			},
			exp: &ast.Location{File: "buffer.rego", Row: 4, Col: 22, Text: []byte("x")},
		},
		{
			note: "some declaration in operand body",
			modules: map[string]string{
				"buffer.rego": `package test

p if {
	{some x in [1]; ‸x > 0} and true
}`,
			},
			exp: &ast.Location{File: "buffer.rego", Row: 4, Col: 8, Text: []byte("x")},
		},
		{
			note: "some domain in operand body",
			modules: map[string]string{
				"buffer.rego": `package test

p if {
	{some x in ‸d; x > 0} and true
}

d := [1]`,
			},
			exp: &ast.Location{File: "buffer.rego", Row: 7, Col: 1, Text: []byte("d := [1]")},
		},
		{
			note: "function argument referenced from operand body",
			modules: map[string]string{
				"buffer.rego": `package test

f(a) if {
	{‸a > 1} or {a < 0}
}`,
			},
			exp: &ast.Location{File: "buffer.rego", Row: 3, Col: 3, Text: []byte("a")},
		},
		{
			note: "imported ref in operand resolves in other module",
			modules: map[string]string{
				"buffer.rego": `package test

import data.lib

p if {
	‸lib.q or lib.r
}`,
				"lib.rego": `package lib

q := 1

r := 2`,
			},
			exp: &ast.Location{File: "lib.rego", Row: 3, Col: 1, Text: []byte("q := 1")},
		},
		{
			note: "ref in nested parenthesized groups",
			modules: map[string]string{
				"buffer.rego": `package test

p if {
	((‸q or r) and s) or t
}

q := 1

r := 2

s := 3

t := 4`,
			},
			exp: &ast.Location{File: "buffer.rego", Row: 7, Col: 1, Text: []byte("q := 1")},
		},
		{
			note: "ref in negated group",
			modules: map[string]string{
				"buffer.rego": `package test

p if {
	not (q or ‸r)
}

q := 1

r := 2`,
			},
			exp: &ast.Location{File: "buffer.rego", Row: 9, Col: 1, Text: []byte("r := 2")},
		},
		{
			note: "ref in or inside every body",
			modules: map[string]string{
				"buffer.rego": `package test

p if {
	every v in [1] {
		‸q or v
	}
}

q := 1`,
			},
			exp: &ast.Location{File: "buffer.rego", Row: 9, Col: 1, Text: []byte("q := 1")},
		},
		{
			note: "ref in or inside comprehension body",
			modules: map[string]string{
				"buffer.rego": `package test

p := [x |
	x := 1
	‸q or r
]

q := 1

r := 2`,
			},
			exp: &ast.Location{File: "buffer.rego", Row: 8, Col: 1, Text: []byte("q := 1")},
		},
		{
			note: "ref in operand body with with-modifier",
			modules: map[string]string{
				"buffer.rego": `package test

p if {
	{q with input as 1} and ‸r
}

q := input

r := 2`,
			},
			exp: &ast.Location{File: "buffer.rego", Row: 9, Col: 1, Text: []byte("r := 2")},
		},
		{
			note: "ref in group with with-modifier",
			modules: map[string]string{
				"buffer.rego": `package test

p if {
	(‸q and r) with input as 1
}

q := input

r := 2`,
			},
			exp: &ast.Location{File: "buffer.rego", Row: 7, Col: 1, Text: []byte("q := input")},
		},
		{
			note: "keywords activated by future import",
			opts: &ast.ParserOptions{},
			modules: map[string]string{
				"buffer.rego": `package test

import future.keywords.and

p if {
	‸q and r
}

q := 1

r := 2`,
			},
			exp: &ast.Location{File: "buffer.rego", Row: 9, Col: 1, Text: []byte("q := 1")},
		},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			t.Parallel()

			opts := logicalParserOpts()
			if tc.opts != nil {
				opts = *tc.opts
			}

			buffer, pos := cursorOffset(t, tc.modules["buffer.rego"])

			modules := map[string]*ast.Module{}
			for k, v := range tc.modules {
				if k == "buffer.rego" {
					v = buffer
				}

				var err error
				modules[k], err = ast.ParseModuleWithOpts(k, v, opts)
				if err != nil {
					t.Fatal(err)
				}
			}

			result, err := New().FindDefinition(DefinitionQuery{
				Modules:       modules,
				Buffer:        []byte(buffer),
				Filename:      "buffer.rego",
				Pos:           pos,
				ParserOptions: opts,
			})
			if err != nil {
				t.Fatal(err)
			}

			if !tc.exp.Equal(result.Result) {
				t.Fatalf("expected %v %q but got %v %q", tc.exp, tc.exp.Text, result.Result, result.Result.Text)
			}
		})
	}
}

func TestOracleFindDefinitionLogicalErrors(t *testing.T) {
	t.Parallel()

	cases := []struct {
		note   string
		opts   *ast.ParserOptions // defaults to logicalParserOpts()
		buffer string
		exp    error
	}{
		{
			note: "no good match - and keyword",
			buffer: `package test

p if {
	q ‸and r
}

q := 1

r := 2`,
			exp: ErrNoDefinitionFound,
		},
		{
			note: "no good match - whitespace between operands",
			buffer: `package test

p if {
	q and‸ r
}

q := 1

r := 2`,
			exp: ErrNoDefinitionFound,
		},
		{
			note: "buffer parse error - keywords not in capabilities",
			opts: &ast.ParserOptions{},
			buffer: `package test

p if {
	‸q and r
}

q := 1

r := 2`,
			exp: errors.New("rego_parse_error"),
		},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			t.Parallel()

			opts := logicalParserOpts()
			if tc.opts != nil {
				opts = *tc.opts
			}

			buffer, pos := cursorOffset(t, tc.buffer)

			result, err := New().FindDefinition(DefinitionQuery{
				Buffer:        []byte(buffer),
				Filename:      "buffer.rego",
				Pos:           pos,
				ParserOptions: opts,
			})
			if err == nil || result != nil {
				t.Fatal("expected error but got:", err, "result:", result)
			}

			if !strings.Contains(err.Error(), tc.exp.Error()) {
				t.Fatalf("expected %v but got %v", tc.exp, err)
			}
		})
	}
}

func TestFindContainingNodeStackLogical(t *testing.T) {
	t.Parallel()

	module, pos := cursorOffset(t, `package test

p if {
	x := 1
	{‸x > 0} and {x < 5}
}`)

	parsed := ast.MustParseModuleWithOpts(module, logicalParserOpts())

	and, ok := parsed.Rules[0].Body[1].Terms.(*ast.LogicalAnd)
	if !ok {
		t.Fatalf("expected *ast.LogicalAnd but got %T", parsed.Rules[0].Body[1].Terms)
	}

	exp := []*ast.Location{
		parsed.Rules[0].Loc(),
		parsed.Rules[0].Body.Loc(),
		parsed.Rules[0].Body[1].Loc(),
		and.Loc(),
		and.Lhs.Loc(),
		and.Lhs[0].Loc(),
		and.Lhs[0].Terms.([]*ast.Term)[1].Loc(),
	}

	assertNodeStack(t, findContainingNodeStack(parsed, pos), exp)
}

// Nodes carrying bodies must be traversed even when they have no location, so
// that programmatically constructed ASTs can still be searched.
func TestFindContainingNodeStackMissingLocation(t *testing.T) {
	t.Parallel()

	cases := []struct {
		note   string
		module string
		unset  func(*ast.Expr)
	}{
		{
			note: "and",
			module: `package test

p if {
	{‸q} and {r}
}`,
			unset: func(expr *ast.Expr) { expr.Terms.(*ast.LogicalAnd).Location = nil },
		},
		{
			note: "or",
			module: `package test

p if {
	{‸q} or {r}
}`,
			unset: func(expr *ast.Expr) { expr.Terms.(*ast.LogicalOr).Location = nil },
		},
		{
			note: "not",
			module: `package test

p if {
	not {‸q}
}`,
			unset: func(expr *ast.Expr) { expr.Terms.(*ast.Not).Location = nil },
		},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			t.Parallel()

			module, pos := cursorOffset(t, tc.module)
			parsed := ast.MustParseModuleWithOpts(module, logicalParserOpts())
			tc.unset(parsed.Rules[0].Body[0])

			stack := findContainingNodeStack(parsed, pos)
			if len(stack) == 0 {
				t.Fatal("expected non-empty node stack")
			}

			top, ok := stack[len(stack)-1].(*ast.Term)
			if !ok {
				t.Fatalf("expected *ast.Term at top of stack but got %T: %v", stack[len(stack)-1], stack)
			}

			if !ast.Var("q").Equal(top.Value) {
				t.Fatalf("expected var q at top of stack but got %v", top)
			}
		})
	}
}

func assertNodeStack(t *testing.T, result []ast.Node, exp []*ast.Location) {
	t.Helper()

	if len(result) != len(exp) {
		t.Fatalf("expected %d nodes on the stack but got %d: %v", len(exp), len(result), result)
	}

	for i := range result {
		if result[i].Loc() != exp[i] {
			t.Fatalf("expected exact location pointers but found difference on i = %d (%T): %v", i, result[i], result[i].Loc())
		}
	}
}
