// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package format

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/ast/location"
)

func TestFormatNilLocation(t *testing.T) {
	rule := ast.MustParseRule(`r = y { y = "foo" }`)
	rule.Head.Location = nil

	bs, err := Ast(rule)
	if err != nil {
		t.Fatal(err)
	}

	exp := strings.Trim(`
r = y {
	y = "foo"
}`, " \n")

	if string(bs) != exp {
		t.Fatalf("Expected %q but got %q", exp, string(bs))
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
	rego := "testfiles/test.rego.error"
	contents, err := ioutil.ReadFile(rego)
	if err != nil {
		t.Fatalf("Failed to read rego source: %v", err)
	}

	_, err = Source(rego, contents)
	if err == nil {
		t.Fatal("Expected parsing error, not nil")
	}

	exp := "1 error occurred: testfiles/test.rego.error:27: rego_parse_error: unexpected eof token"

	if !strings.HasPrefix(err.Error(), exp) {
		t.Fatalf("Expected error message '%s', got '%s'", exp, err.Error())
	}
}

func TestFormatSource(t *testing.T) {
	regoFiles, err := filepath.Glob("testfiles/*.rego")
	if err != nil {
		panic(err)
	}

	for _, rego := range regoFiles {
		t.Run(rego, func(t *testing.T) {
			contents, err := ioutil.ReadFile(rego)
			if err != nil {
				t.Fatalf("Failed to read rego source: %v", err)
			}

			expected, err := ioutil.ReadFile(rego + ".formatted")
			if err != nil {
				t.Fatalf("Failed to read expected rego source: %v", err)
			}

			formatted, err := Source(rego, contents)
			if err != nil {
				t.Fatalf("Failed to format file: %v", err)
			}

			if ln, at := differsAt(formatted, expected); ln != 0 {
				t.Fatalf("Expected formatted bytes to equal expected bytes but differed near line %d / byte %d (got: %q, expected: %q):\n%s", ln, at, formatted[at], expected[at], prefixWithLineNumbers(formatted))
			}

			if _, err := ast.ParseModule(rego+".tmp", string(formatted)); err != nil {
				t.Fatalf("Failed to parse formatted bytes: %v", err)
			}

			formatted, err = Source(rego, formatted)
			if err != nil {
				t.Fatalf("Failed to double format file")
			}

			if ln, at := differsAt(formatted, expected); ln != 0 {
				t.Fatalf("Expected roundtripped bytes to equal expected bytes but differed near line %d / byte %d:\n%s", ln, at, prefixWithLineNumbers(formatted))
			}

		})
	}
}

func TestFormatAST(t *testing.T) {
	cases := []struct {
		note     string
		toFmt    interface{}
		expected string
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
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			bs, err := Ast(tc.toFmt)
			if err != nil {
				t.Fatalf("Unexpected error: %s", err)
			}
			expected := strings.TrimSpace(tc.expected)
			actual := strings.TrimSpace(string(bs))
			if actual != expected {
				t.Fatalf("Expected:\n\n%s\n\nGot:\n\n%s\n\n", expected, actual)
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
	for i := 0; i < minLen; i++ {
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
	format := fmt.Sprintf("%%%dd %%s", len(fmt.Sprint(len(lines)+1)))
	for i, line := range lines {
		lines[i] = fmt.Sprintf(format, i+1, line)
	}
	return []byte(strings.Join(lines, "\n"))
}
