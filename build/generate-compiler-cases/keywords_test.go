// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cases

import (
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
)

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
