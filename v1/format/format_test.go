// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package format

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/ast/location"
)

func TestFormatNilLocation(t *testing.T) {
	tests := []struct {
		note        string
		regoVersion ast.RegoVersion
		rule        string
		exp         string
	}{
		{
			note:        "v0",
			regoVersion: ast.RegoV0,
			rule:        `r = y { y = "foo" }`,
			exp: `r = y {
	y = "foo"
}`,
		},
		{
			note:        "v1",
			regoVersion: ast.RegoV1,
			rule:        `r = y if { y = "foo" }`,
			exp: `r := y if y = "foo"
`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			rule := ast.MustParseRuleWithOpts(tc.rule, ast.ParserOptions{RegoVersion: tc.regoVersion})
			rule.Head.Location = nil

			bs, err := AstWithOpts(rule, Opts{RegoVersion: tc.regoVersion})
			if err != nil {
				t.Fatal(err)
			}

			if string(bs) != tc.exp {
				t.Fatalf("Expected:\n\n%q\n\nbut got:\n\n%q", tc.exp, string(bs))
			}
		})
	}
}

func TestFormatNilLocationEmptyBody(t *testing.T) {
	b := ast.NewBody()
	x, err := Ast(b)
	if len(x) != 0 || err != nil {
		t.Fatalf("Expected empty result but got: %q, err: %v", string(x), err)
	}
}

func TestFormatNilLocationFunctionArgs(t *testing.T) {
	b := ast.NewBody()
	s := ast.StringTerm(" ")
	s.SetLocation(location.NewLocation([]byte("foo"), "p.rego", 2, 2))
	b.Append(ast.Split.Expr(ast.NewTerm(ast.Var("__local1__")), s, ast.NewTerm(ast.Var("__local2__"))))
	exp := "split(__local1__, \" \", __local2__)\n"
	bs, err := Ast(b)
	if err != nil {
		t.Fatal(err)
	}
	if string(bs) != exp {
		t.Fatalf("Expected %q but got %q", exp, string(bs))
	}
}

func TestFormatSourceError(t *testing.T) {
	rego := "testfiles/v0/test.rego.error"
	contents, err := os.ReadFile(rego)
	if err != nil {
		t.Fatalf("Failed to read rego source: %v", err)
	}

	_, err = Source(rego, contents)
	if err == nil {
		t.Fatal("Expected parsing error, not nil")
	}

	exp := "1 error occurred: testfiles/v0/test.rego.error:27: rego_parse_error: unexpected eof token"

	if !strings.HasPrefix(err.Error(), exp) {
		t.Fatalf("Expected error message '%s', got '%s'", exp, err.Error())
	}
}

func TestFormatV0Source(t *testing.T) {
	regoFiles, err := filepath.Glob("testfiles/v0/*.rego")
	if err != nil {
		panic(err)
	}

	for _, rego := range regoFiles {
		t.Run(rego, func(t *testing.T) {
			contents, err := os.ReadFile(rego)
			if err != nil {
				t.Fatalf("Failed to read rego source: %v", err)
			}

			expected, err := os.ReadFile(rego + ".formatted")
			if err != nil {
				t.Fatalf("Failed to read expected rego source: %v", err)
			}

			popts := ast.ParserOptions{
				RegoVersion: ast.RegoV0,
			}
			opts := Opts{
				RegoVersion:   ast.RegoV0,
				ParserOptions: &popts,
			}

			formatted, err := SourceWithOpts(rego, contents, opts)
			if err != nil {
				t.Fatalf("Failed to format file: %v", err)
			}

			if ln, at := differsAt(formatted, expected); ln != 0 {
				t.Fatalf("Expected formatted bytes to equal expected bytes but differed near line %d / byte %d (got: %q, expected: %q):\n%s", ln, at, formatted[at], expected[at], prefixWithLineNumbers(formatted))
			}

			if _, err := ast.ParseModuleWithOpts(rego+".tmp", string(formatted), popts); err != nil {
				t.Fatalf("Failed to parse formatted bytes: %v", err)
			}

			formatted, err = SourceWithOpts(rego, formatted, opts)
			if err != nil {
				t.Fatalf("Failed to double format file")
			}

			if ln, at := differsAt(formatted, expected); ln != 0 {
				t.Fatalf("Expected roundtripped bytes to equal expected bytes but differed near line %d / byte %d:\n%s", ln, at, prefixWithLineNumbers(formatted))
			}

		})
	}
}

func TestFormatV1Source(t *testing.T) {
	regoFiles, err := filepath.Glob("testfiles/v1/*.rego")
	if err != nil {
		panic(err)
	}

	for _, rego := range regoFiles {
		t.Run(rego, func(t *testing.T) {
			contents, err := os.ReadFile(rego)
			if err != nil {
				t.Fatalf("Failed to read rego source: %v", err)
			}

			expected, err := os.ReadFile(rego + ".formatted")
			if err != nil {
				t.Fatalf("Failed to read expected rego source: %v", err)
			}

			popts := ast.ParserOptions{
				RegoVersion: ast.RegoV1,
			}
			opts := Opts{
				RegoVersion:   ast.RegoV1,
				ParserOptions: &popts,
			}

			formatted, err := SourceWithOpts(rego, contents, opts)
			if err != nil {
				t.Fatalf("Failed to format file: %v", err)
			}

			if ln, at := differsAt(formatted, expected); ln != 0 {
				t.Fatalf("Expected formatted bytes to equal expected bytes but differed near line %d / byte %d (got: %q, expected: %q):\n%s", ln, at, formatted[at], expected[at], prefixWithLineNumbers(formatted))
			}

			if _, err := ast.ParseModuleWithOpts(rego+".tmp", string(formatted), popts); err != nil {
				t.Fatalf("Failed to parse formatted bytes: %v", err)
			}

			formatted, err = SourceWithOpts(rego, formatted, opts)
			if err != nil {
				t.Fatalf("Failed to double format file")
			}

			if ln, at := differsAt(formatted, expected); ln != 0 {
				t.Fatalf("Expected roundtripped bytes to equal expected bytes but differed near line %d / byte %d:\n%s", ln, at, prefixWithLineNumbers(formatted))
			}

		})
	}
}

func TestFormatV0SourceToRegoV1(t *testing.T) {
	regoFiles, err := filepath.Glob("testfiles/v0_to_v1/*.rego")
	if err != nil {
		panic(err)
	}

	for _, rego := range regoFiles {
		t.Run(rego, func(t *testing.T) {
			contents, err := os.ReadFile(rego)
			if err != nil {
				t.Fatalf("Failed to read rego source: %v", err)
			}

			errorExpected := false
			expected, err := os.ReadFile(rego + ".formatted")
			if err != nil {
				if os.IsNotExist(err) {
					errorExpected = true
					expected, err = os.ReadFile(rego + ".error")
					if err != nil {
						t.Fatalf("Failed to read expected error source: %v", err)
					}
				}
				if !errorExpected {
					t.Fatalf("Failed to read expected rego source: %v", err)
				}
			}

			sourceOpts := Opts{
				RegoVersion: ast.RegoV0CompatV1, // Target syntax is v0 compat v1
				ParserOptions: &ast.ParserOptions{
					RegoVersion: ast.RegoV0, // Original syntax is v0
				},
			}
			targetOpts := Opts{
				RegoVersion: ast.RegoV0CompatV1, // Target syntax is v0 compat v1
			}

			if errorExpected {
				formatted, err := SourceWithOpts(rego, contents, sourceOpts)

				if err == nil {
					t.Fatalf("Expected error, got: %s", formatted)
				}
				if err.Error() != string(expected) {
					t.Fatalf("Expected error:\n\n'%s'\n\ngot:\n\n'%s'", expected, err.Error())
				}
			} else {
				formatted, err := SourceWithOpts(rego, contents, sourceOpts)

				if err != nil {
					t.Fatalf("Failed to format file: %v", err)
				}

				if ln, at := differsAt(formatted, expected); ln != 0 {
					t.Fatalf("Expected formatted bytes to equal expected bytes but differed near line %d / byte %d (got: %q, expected: %q):\n%s", ln, at, formatted[at], expected[at], prefixWithLineNumbers(formatted))
				}

				if _, err := ast.ParseModule(rego+".tmp", string(formatted)); err != nil {
					t.Fatalf("Failed to parse formatted bytes: %v", err)
				}

				formatted, err = SourceWithOpts(rego, formatted, targetOpts)
				if err != nil {
					t.Fatalf("Failed to double format file")
				}

				if ln, at := differsAt(formatted, expected); ln != 0 {
					t.Fatalf("Expected roundtripped bytes to equal expected bytes but differed near line %d / byte %d:\n%s", ln, at, prefixWithLineNumbers(formatted))
				}

				// rego-v1 formatted code is still compliant with v0, and should not be changed if formatted as such
				formatted, err = SourceWithOpts(rego, formatted, targetOpts)
				if err != nil {
					t.Fatalf("Failed to double format file as v0")
				}

				if ln, at := differsAt(formatted, expected); ln != 0 {
					t.Fatalf("Expected roundtripped bytes to equal expected bytes but differed near line %d / byte %d:\n%s", ln, at, prefixWithLineNumbers(formatted))
				}
			}
		})
	}
}

func TestFormatAST(t *testing.T) {
	cases := []struct {
		note        string
		regoVersion ast.RegoVersion
		toFmt       interface{}
		expected    string
	}{
		{
			note:     "var",
			toFmt:    ast.Var(`foo`),
			expected: "foo",
		},
		{
			note: "string",
			toFmt: &ast.Term{
				Value:    ast.String("foo"),
				Location: &ast.Location{Text: []byte(`"foo"`)},
			},
			expected: `"foo"`,
		},
		{
			note:     "var wildcard",
			toFmt:    ast.Var(`$12`),
			expected: "_",
		},
		{
			note: "string with wildcard prefix",
			toFmt: &ast.Term{
				Value:    ast.String("$01"),
				Location: &ast.Location{Text: []byte(`"$01"`)},
			},
			expected: `"$01"`,
		},
		{
			note:     "ref var only",
			toFmt:    ast.MustParseRef(`data.foo`),
			expected: "data.foo",
		},
		{
			note:     "ref multi vars",
			toFmt:    ast.MustParseRef(`data.foo.bar.baz`),
			expected: "data.foo.bar.baz",
		},
		{
			note:     "ref with string",
			toFmt:    ast.MustParseRef(`data["foo"]`),
			expected: `data.foo`,
		},
		{
			note:     "ref multi string",
			toFmt:    ast.MustParseRef(`data["foo"]["bar"]["baz"]`),
			expected: `data.foo.bar.baz`,
		},
		{
			note:     "ref with string needs brackets",
			toFmt:    ast.MustParseRef(`data["foo my-var\nbar"]`),
			expected: `data["foo my-var\nbar"]`,
		},
		{
			note:     "ref multi string needs brackets",
			toFmt:    ast.MustParseRef(`data["foo my-var"]["bar"]["almost.baz"]`),
			expected: `data["foo my-var"].bar["almost.baz"]`,
		},
		{
			note:     "ref var wildcard",
			toFmt:    ast.MustParseRef(`data.foo[_]`),
			expected: "data.foo[_]",
		},
		{
			note:     "ref var wildcard",
			toFmt:    ast.MustParseRef(`foo[_]`),
			expected: "foo[_]",
		},
		{
			note:     "ref string with wildcard prefix",
			toFmt:    ast.MustParseRef(`foo["$01"]`),
			expected: `foo["$01"]`,
		},
		{
			note:     "nested ref var wildcard",
			toFmt:    ast.MustParseRef(`foo[bar[baz[_]]]`),
			expected: "foo[bar[baz[_]]]",
		},
		{
			note:     "ref mixed",
			toFmt:    ast.MustParseRef(`foo["bar"].baz[_]["bar-2"].qux`),
			expected: `foo.bar.baz[_]["bar-2"].qux`,
		},
		{
			note:     "ref empty",
			toFmt:    ast.Ref{},
			expected: ``,
		},
		{
			note:     "ref nil",
			toFmt:    ast.Ref(nil),
			expected: ``,
		},
		{
			note:     "ref operator",
			toFmt:    ast.MustParseRef(`foo[count(foo) - 1]`),
			expected: `foo[count(foo) - 1]`,
		},
		{
			note:     "x in xs",
			toFmt:    ast.Member.Call(ast.VarTerm("x"), ast.VarTerm("xs")),
			expected: `x in xs`,
		},
		{
			note:     "x, y in xs",
			toFmt:    ast.MemberWithKey.Call(ast.VarTerm("x"), ast.VarTerm("y"), ast.VarTerm("xs")),
			expected: `(x, y in xs)`,
		},
		{
			note: "some x in xs",
			toFmt: ast.NewExpr(&ast.SomeDecl{Symbols: []*ast.Term{
				ast.Member.Call(ast.VarTerm("x"), ast.VarTerm("xs")),
			}}),
			expected: `some x in xs`,
		},
		{
			note: "some x, y in xs",
			toFmt: ast.NewExpr(&ast.SomeDecl{Symbols: []*ast.Term{
				ast.MemberWithKey.Call(ast.VarTerm("x"), ast.VarTerm("y"), ast.VarTerm("xs")),
			}}),
			expected: `some x, y in xs`,
		},
		{
			note:        "v0, every adds import if missing",
			regoVersion: ast.RegoV0,
			toFmt: ast.MustParseModuleWithOpts(`package test
			p {
				every k, v in [1, 2] { k != v }
			}`,
				ast.ParserOptions{
					RegoVersion:    ast.RegoV0,
					FutureKeywords: []string{"every"},
				}),
			expected: `package test

import future.keywords.every

p {
	every k, v in [1, 2] { k != v }
}`,
		},
		{
			note:        "v1, every doesn't add import if missing",
			regoVersion: ast.RegoV1,
			toFmt: ast.MustParseModuleWithOpts(`package test
			p if {
				every k, v in [1, 2] { k != v }
			}`,
				ast.ParserOptions{RegoVersion: ast.RegoV1}),
			expected: `package test

p if {
	every k, v in [1, 2] { k != v }
}`,
		},
		{
			note:        "v0: every does not add import if all future KWs are there",
			regoVersion: ast.RegoV0,
			toFmt: ast.MustParseModuleWithOpts(`package test
			import future.keywords
			p {
				every k, v in [1, 2] { k != v }
			}`,
				ast.ParserOptions{
					FutureKeywords: []string{"every"},
					RegoVersion:    ast.RegoV0,
				}),
			expected: `package test

import future.keywords

p if {
	every k, v in [1, 2] { k != v }
}`,
		},
		{
			note:        "v0: every does not add import if already present",
			regoVersion: ast.RegoV0,
			toFmt: ast.MustParseModuleWithOpts(`package test
			import future.keywords
			p {
				every k, v in [1, 2] { k != v }
			}`,
				ast.ParserOptions{
					FutureKeywords: []string{"every"},
					RegoVersion:    ast.RegoV0,
				}),
			expected: `package test

import future.keywords

p if {
	every k, v in [1, 2] { k != v }
}`,
		},
		{
			note: "body shared wildcard",
			toFmt: ast.Body{
				&ast.Expr{
					Index: 0,
					Terms: []*ast.Term{
						ast.RefTerm(ast.VarTerm("eq")),
						ast.RefTerm(ast.VarTerm("input"), ast.StringTerm("arr"), ast.VarTerm("$01"), ast.StringTerm("some key"), ast.VarTerm("$02")),
						ast.VarTerm("bar"),
					},
				},
				&ast.Expr{
					Index: 1,
					Location: &ast.Location{
						Row: 2,
						Col: 1,
					},
					Terms: []*ast.Term{
						ast.RefTerm(ast.VarTerm("eq")),
						ast.RefTerm(ast.VarTerm("input"), ast.StringTerm("arr"), ast.VarTerm("$01"), ast.StringTerm("bar")),
						ast.VarTerm("qux"),
					},
				},
				&ast.Expr{
					Index: 1,
					Location: &ast.Location{
						Row: 2,
						Col: 1,
					},
					Terms: []*ast.Term{
						ast.RefTerm(ast.VarTerm("eq")),
						ast.RefTerm(ast.VarTerm("foo"), ast.VarTerm("$03"), ast.VarTerm("$01"), ast.StringTerm("bar")),
						ast.RefTerm(ast.VarTerm("bar"), ast.VarTerm("$03"), ast.VarTerm("$04"), ast.VarTerm("$01"), ast.StringTerm("bar")),
					},
				},
			},
			expected: `input.arr[_01]["some key"][_] = bar
input.arr[_01].bar = qux
foo[_03][_01].bar = bar[_03][_][_01].bar
`,
		},
		{
			note: "body shared wildcard - ref head",
			toFmt: ast.Body{
				&ast.Expr{
					Index: 0,
					Terms: ast.VarTerm("$x"),
				},
				&ast.Expr{
					Index: 1,
					Terms: ast.RefTerm(ast.VarTerm("$x"), ast.VarTerm("y")),
				},
			},
			expected: `_x
_x[y]`,
		},
		{
			note: "body shared wildcard - nested ref",
			toFmt: ast.Body{
				&ast.Expr{
					Index: 0,
					Terms: ast.VarTerm("$x"),
				},
				&ast.Expr{
					Index: 1,
					Terms: ast.RefTerm(ast.VarTerm("a"), ast.RefTerm(ast.VarTerm("$x"), ast.VarTerm("y"))),
				},
			},
			expected: `_x
a[_x[y]]`,
		},
		{
			note: "body shared wildcard - nested ref array",
			toFmt: ast.Body{
				&ast.Expr{
					Index: 0,
					Terms: ast.VarTerm("$x"),
				},
				&ast.Expr{
					Index: 1,
					Terms: ast.RefTerm(ast.VarTerm("a"), ast.RefTerm(ast.VarTerm("$x"), ast.VarTerm("y"), ast.ArrayTerm(ast.VarTerm("z"), ast.VarTerm("w")))),
				},
			},
			expected: `_x
a[_x[y][[z, w]]]`,
		},
		{
			note: "expr with wildcard that has a default location",
			toFmt: func() *ast.Expr {
				expr := ast.MustParseExpr(`["foo", _] = split(input.foo, ":")`)
				ast.WalkTerms(expr, func(term *ast.Term) bool {
					v, ok := term.Value.(ast.Var)
					if ok && v.IsWildcard() {
						term.Location = defaultLocation(term)
						return true
					}
					term.Location.File = "foo.rego"
					term.Location.Row = 2
					return false
				})
				return expr
			}(),
			expected: `["foo", _] = split(input.foo, ":")`,
		},
		{
			note: "expr all terms having empty-file locations",
			toFmt: ast.MustParseExpr(`[
					"foo",
					_
					] = split(input.foo, ":")`),
			expected: `
[
	"foo",
	_,
] = split(input.foo, ":")`,
		},
		{
			note: "expr where all terms having empty-file locations, and one is a default location",
			toFmt: func() *ast.Expr {
				expr := ast.MustParseExpr(`
["foo", __local1__] = split(input.foo, ":")`)
				ast.WalkTerms(expr, func(term *ast.Term) bool {
					if ast.VarTerm("__local1__").Equal(term) {
						term.Location = defaultLocation(term)
						return true
					}
					return false
				})
				return expr
			}(),
			expected: `["foo", __local1__] = split(input.foo, ":")`,
		},
		{
			note: "expr where generated var has an AST location not matching its source location",
			toFmt: func() *ast.Expr {
				e := ast.MustParseExpr(`__local0__ = concat(",", [__local1__])`)
				ast.WalkTerms(e, func(t *ast.Term) bool {
					t.Location.File = "t.rego"
					return false
				})
				// mangling that may happen in PE
				return ast.Concat.Expr(
					e.Operand(1).Value.(ast.Call)[1],
					e.Operand(1).Value.(ast.Call)[2],
					e.Operand(0),
				).SetLocation(e.Location)
			}(),
			expected: `concat(",", [__local1__], __local0__)`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			bs, err := AstWithOpts(tc.toFmt, Opts{
				RegoVersion: tc.regoVersion,
				ParserOptions: &ast.ParserOptions{
					RegoVersion: tc.regoVersion,
				},
			})
			if err != nil {
				t.Fatalf("Unexpected error: %s", err)
			}
			expected := strings.TrimSpace(tc.expected)
			actual := strings.TrimSpace(string(bs))
			if actual != expected {
				t.Fatalf("Expected:\n\n%q\n\nGot:\n\n%q\n\n", expected, actual)
			}
		})

		// consistency check: disregarding source locations, it shouldn't panic
		t.Run("no_loc/"+tc.note, func(t *testing.T) {
			_, err := AstWithOpts(tc.toFmt, Opts{IgnoreLocations: true})
			if err != nil {
				t.Fatalf("Unexpected error: %s", err)
			}
			if err != nil {
				t.Fatalf("Unexpected error: %s", err)
			}
		})
	}
}

func TestFormatDeepCopy(t *testing.T) {

	original := ast.Body{
		&ast.Expr{
			Index: 0,
			Terms: ast.VarTerm("$x"),
		},
		&ast.Expr{
			Index: 1,
			Terms: ast.RefTerm(ast.VarTerm("$x"), ast.VarTerm("y")),
		},
	}

	cpy := original.Copy()

	_, err := Ast(original)
	if err != nil {
		t.Fatal(err)
	}

	if !cpy.Equal(original) {
		t.Fatal("expected original to be unmodified")
	}

}

func differsAt(a, b []byte) (int, int) {
	if bytes.Equal(a, b) {
		return 0, 0
	}
	minLen := len(a)
	if minLen > len(b) {
		minLen = len(b)
	}
	ln := 1
	for i := range minLen {
		if a[i] == '\n' {
			ln++
		}
		if a[i] != b[i] {
			return ln, i
		}
	}
	return ln, minLen - 1
}

func prefixWithLineNumbers(bs []byte) []byte {
	raw := string(bs)
	lines := strings.Split(raw, "\n")
	format := fmt.Sprintf("%%%dd %%s", len(strconv.Itoa(len(lines)+1)))
	for i, line := range lines {
		lines[i] = fmt.Sprintf(format, i+1, line)
	}
	return []byte(strings.Join(lines, "\n"))
}

func TestSource_DefaultRegoVersion(t *testing.T) {
	tests := []struct {
		note         string
		module       string
		expFormatted string
		expErrs      []string
	}{
		{
			note: "v0", // from default rego-version
			module: `package test

p[x]            {
	x = "a"
}`,

			expErrs: []string{
				"test.rego:3: rego_parse_error: `if` keyword is required before rule body",
				"test.rego:3: rego_parse_error: `contains` keyword is required for partial set rules",
			},
		},
		{
			note: "v1",
			module: `package test

p    contains    x    if      {
	x = "a"
}`,
			expFormatted: `package test

p contains x if {
	x = "a"
}
`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			formatted, err := Source("test.rego", []byte(tc.module))
			if len(tc.expErrs) > 0 {
				if err == nil {
					t.Fatalf("expected errors but got nil")
				}

				for _, expErr := range tc.expErrs {
					if !strings.Contains(err.Error(), expErr) {
						t.Fatalf("expected error:\n\n%q\n\nbut got:\n\n%q", expErr, err)
					}
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				formattedStr := string(formatted)
				if formattedStr != tc.expFormatted {
					t.Fatalf("expected %q but got %q", tc.expFormatted, formattedStr)
				}
			}
		})
	}
}

func TestSourceWithOpts_DefaultRegoVersion(t *testing.T) {
	tests := []struct {
		note          string
		toRegoVersion ast.RegoVersion
		module        string
		expFormatted  string
		expErrs       []string
	}{
		{
			note:          "v0 -> v0", // from default rego-version
			toRegoVersion: ast.RegoV0,
			module: `package test

p[x]            {
	x = "a"
}`,
			expErrs: []string{
				"test.rego:3: rego_parse_error: `if` keyword is required before rule body",
				"test.rego:3: rego_parse_error: `contains` keyword is required for partial set rules",
			},
		},
		{
			note:          "v0 -> v1", // from default rego-version
			toRegoVersion: ast.RegoV1,
			module: `package test

p[x]            {
	x = "a"
}`,
			expErrs: []string{
				"test.rego:3: rego_parse_error: `if` keyword is required before rule body",
				"test.rego:3: rego_parse_error: `contains` keyword is required for partial set rules",
			},
		},
		{
			note:          "v1 -> v1", // from non-default rego-version
			toRegoVersion: ast.RegoV1,
			module: `package test

p    contains    x    if      {
	x = "a"
}`,
			expFormatted: `package test

p contains x if {
	x = "a"
}
`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			formatted, err := SourceWithOpts("test.rego", []byte(tc.module), Opts{RegoVersion: tc.toRegoVersion})
			if len(tc.expErrs) > 0 {
				if err == nil {
					t.Fatalf("expected errors but got nil")
				}

				for _, expErr := range tc.expErrs {
					if !strings.Contains(err.Error(), expErr) {
						t.Fatalf("expected error:\n\n%q\n\nbut got:\n\n%q", expErr, err)
					}
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				formattedStr := string(formatted)
				if formattedStr != tc.expFormatted {
					t.Fatalf("expected %q but got %q", tc.expFormatted, formattedStr)
				}
			}
		})
	}
}

// 382	   3064960 ns/op	 4573131 B/op	   26266 allocs/op // no optimizations
// 685	   1737719 ns/op	 1972193 B/op	   14160 allocs/op // pre-allocate partitionComments
// 708	   1674343 ns/op	 1916700 B/op	   11556 allocs/op // static memberRef & memberWithKeyRef
// 746	   1594546 ns/op	 1882652 B/op	   10644 allocs/op // various minor fixes
// 1250	    853508 ns/op	  441730 B/op	    8895 allocs/op // partitionComments early return if unchanged
// 1396	    812859 ns/op	  362651 B/op	    8811 allocs/op // partitionComments reuse backing array
func BenchmarkFormatLargePolicy(b *testing.B) {
	contents, err := os.ReadFile("testdata/bench.rego")
	if err != nil {
		b.Fatalf("Failed to read rego source: %v", err)
	}
	module := ast.MustParseModule(string(contents))

	b.ResetTimer()

	for range b.N {
		_, err := AstWithOpts(module, Opts{RegoVersion: ast.RegoV1})
		if err != nil {
			b.Fatal(err)
		}
	}
}
