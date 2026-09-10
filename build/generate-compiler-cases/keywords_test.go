// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cases

import (
	"slices"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/test/compilecases"
)

func TestDirectiveImports(t *testing.T) {
	tests := []struct {
		note    string
		version string
		module  string
		want    [][]string
	}{
		{
			// The compiler resolves the import away, and `or` is not a keyword
			// without it, so the printed form does not parse at all.
			note:   "an infix keyword the compiled form still uses",
			module: "package test\n\nimport future.keywords.or\np {\n\t{ input := 1 } or { data := 2 }\n}\n",
			want:   [][]string{{"future.keywords.or"}},
		},
		{
			// The case the derivation exists for: `not` is a keyword either way, so
			// the printed form parses whether or not it is active — to a different
			// AST when it is not. See TestFutureNotParsesEitherWay below.
			note:   "a keyword that changes meaning rather than syntax",
			module: "package test\n\nimport future.keywords.not\np {\n\tnot q\n}\n\nq {\n\ttrue\n}\n",
			want:   [][]string{{"future.keywords.not"}},
		},
		{
			// Reported even though the compiled form does not use it. Working out
			// that it is redundant would mean modelling what the compiler does to
			// each keyword, and an extra keyword costs a consumer nothing while a
			// missing one leaves a fixture nobody can parse.
			note:   "an unused import is still declared",
			module: "package test\n\nimport future.keywords.every\np {\n\ttrue\n}\n",
			want:   [][]string{{"future.keywords.every"}},
		},
		{
			note:   "a wildcard import is carried as written",
			module: "package test\n\nimport future.keywords\np {\n\ttrue\n}\n",
			want:   [][]string{{"future.keywords"}},
		},
		{
			note:   "both the wildcard and a named keyword are reported",
			module: "package test\n\nimport future.keywords\nimport future.keywords.every\np {\n\ttrue\n}\n",
			want:   [][]string{{"future.keywords", "future.keywords.every"}},
		},
		{
			// A dialect selector rather than a keyword activation, and carried the
			// same way: under rego_version v0 the compiler strips it as it strips a
			// keyword import, and want_modules then only parses as v1. Declaring the
			// import is what tells a consumer so.
			note:    "rego.v1 is reported like any other directive",
			version: "v1",
			module:  "package test\n\nimport rego.v1\n\np if {\n\ttrue\n}\n",
			want:    [][]string{{"rego.v1"}},
		},
		{
			note:   "no imports at all",
			module: "package test\n\np {\n\ttrue\n}\n",
			want:   [][]string{nil},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			version := tc.version
			if version == "" {
				version = "v0"
			}
			c := compilecases.TestCase{Note: tc.note, Modules: []string{tc.module}, RegoVersion: version}

			popts, err := parserOptions(c)
			if err != nil {
				t.Fatal(err)
			}

			compiler, err := compileCase(c, popts)
			if err != nil {
				t.Fatal(err)
			}
			if compiler.Failed() {
				t.Fatalf("unexpected compile errors: %v", compiler.Errors)
			}

			got, err := directiveImports(c, popts)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.EqualFunc(got, tc.want, slices.Equal) {
				t.Errorf("expected %v, got %v", tc.want, got)
			}

			// Whatever it reports has to be enough to print the compiled module.
			wantOpts, err := wantParserOptions(c, got, 0)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := formatModule(compiler.Modules[compilecases.ModuleName(0)], wantOpts); err != nil {
				t.Errorf("the reported imports do not suffice: %v", err)
			}
		})
	}
}

// TestFutureNotParsesEitherWay is why the keywords are read from the case's imports
// rather than discovered from a parse failure.
//
// `not` is a keyword in every version, so a compiled body using the *future* `not`
// prints as text that parses whether or not the keyword is active — as a Not node
// when it is, and as a negated expression when it is not. Nothing fails, so nothing
// would prompt a search for the keyword, and the case would fall back to want_ast
// for no reason. Reading the import gets it right without guessing.
func TestFutureNotParsesEitherWay(t *testing.T) {
	v0 := ast.ParserOptions{RegoVersion: ast.RegoV0}
	withNot := ast.ParserOptions{RegoVersion: ast.RegoV0, FutureKeywords: []string{"not"}}

	m, err := ast.ParseModuleWithOpts("t.rego", "package test\n\nimport future.keywords.not\np {\n\tnot q\n}\n\nq {\n\ttrue\n}\n", v0)
	if err != nil {
		t.Fatal(err)
	}

	c := ast.NewCompiler().SetErrorLimit(0)
	c.Compile(map[string]*ast.Module{"t.rego": m})
	if c.Failed() {
		t.Fatalf("unexpected compile errors: %v", c.Errors)
	}

	compiled := c.Modules["t.rego"]
	compiled.Comments = nil

	text, err := formatModule(compiled, withNot)
	if err != nil {
		t.Fatalf("expected the compiled module to be printable with the keyword active: %v", err)
	}

	// Parses without the keyword, which is the trap: no error to react to.
	plain, err := ast.ParseModuleWithOpts("t.rego", text, v0)
	if err != nil {
		t.Fatalf("expected the text to parse without the keyword, got: %v", err)
	}
	plain.Comments = nil

	if compiled.Equal(plain) {
		t.Error("expected the future `not` to parse to a different AST without the keyword; " +
			"if this ever stops being true, the derivation is no longer load-bearing for this case")
	}
}
