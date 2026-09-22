// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cases

import (
	"errors"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
)

// TestFormatModuleRejectsWhatItCannotVerify exercises the check itself. `else :=`
// is the case OPA's printer gets wrong today: the assignment is dropped from the
// else head, so the text reparses to an AST whose Head.Assign is false. Without
// the check the fixture would look right in a diff and assert the wrong thing.
func TestFormatModuleRejectsWhatItCannotVerify(t *testing.T) {
	m, err := ast.ParseModule("t.rego", "package t\n\np := 1 if {\n\tinput.x\n} else := 2\n")
	if err != nil {
		t.Fatal(err)
	}

	c := ast.NewCompiler().SetErrorLimit(0)
	c.Compile(map[string]*ast.Module{"t.rego": m})
	if c.Failed() {
		t.Fatalf("unexpected compile errors: %v", c.Errors)
	}

	got, err := formatModule(c.Modules["t.rego"], ast.ParserOptions{RegoVersion: ast.RegoV1})
	if err == nil {
		t.Fatalf("expected the check to reject this, got:\n%s", got)
	}
	if want := "parses to a different AST"; !strings.Contains(err.Error(), want) {
		t.Errorf("expected an error containing %q, got %v", want, err)
	}
	if got != "" {
		t.Errorf("expected no text alongside the error, got %q", got)
	}
}

// TestFormatModuleRejectsUnparseableOutput covers the other branch. The compiler
// resolves `import future.keywords.and` away and the printer does not put it back,
// so the text only parses with the keyword already active.
func TestFormatModuleRejectsUnparseableOutput(t *testing.T) {
	popts := ast.ParserOptions{RegoVersion: ast.RegoV0}

	m, err := ast.ParseModuleWithOpts("t.rego",
		"package test\n\nimport future.keywords.and\np {\n\t{ input := 1 } and { data := 2 }\n}\n", popts)
	if err != nil {
		t.Fatal(err)
	}

	c := ast.NewCompiler().SetErrorLimit(0)
	c.Compile(map[string]*ast.Module{"t.rego": m})
	if c.Failed() {
		t.Fatalf("unexpected compile errors: %v", c.Errors)
	}

	if _, err := formatModule(c.Modules["t.rego"], popts); err == nil {
		t.Error("expected the check to reject this")
	} else if want := "needs `import future.keywords.and`"; !strings.Contains(err.Error(), want) {
		t.Errorf("expected an error containing %q, got %v", want, err)
	}
}

// TestParseFailureReasonNamesTheMissingImport pins the diagnosis, because the
// parse error it replaces points somewhere else entirely: without `or` active,
// `{ x = 1 } or { y = 2 }` reads as an unterminated set, and the message says so
// rather than saying the import is missing.
func TestParseFailureReasonNamesTheMissingImport(t *testing.T) {
	tests := []struct {
		note    string
		module  string
		keyword string
	}{
		{
			note:    "or",
			module:  "package test\n\nimport future.keywords.or\np {\n\t{ input := 1 } or { data := 2 }\n}\n",
			keyword: "or",
		},
		{
			note:    "and",
			module:  "package test\n\nimport future.keywords.and\np {\n\t{ input := 1 } and { data := 2 }\n}\n",
			keyword: "and",
		},
		{
			note:    "not",
			module:  "package test\n\nimport future.keywords.not\np {\n\tnot { input := 1; data := 2 }\n}\n",
			keyword: "not",
		},
		{
			note:    "every",
			module:  "package test\n\nimport future.keywords.every\np {\n\tevery x in [1, 2] { x > 1 }\n}\n",
			keyword: "every",
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			popts := ast.ParserOptions{RegoVersion: ast.RegoV0}

			m, err := ast.ParseModuleWithOpts("t.rego", tc.module, popts)
			if err != nil {
				t.Fatal(err)
			}

			c := ast.NewCompiler().SetErrorLimit(0)
			c.Compile(map[string]*ast.Module{"t.rego": m})
			if c.Failed() {
				t.Fatalf("unexpected compile errors: %v", c.Errors)
			}

			_, ferr := formatModule(c.Modules["t.rego"], popts)
			if ferr == nil {
				t.Fatal("expected the module to be unprintable")
			}

			np, ok := errors.AsType[notPrintableError](ferr)
			if !ok {
				t.Fatalf("expected a notPrintableError, got %T", ferr)
			}

			want := "`import future.keywords." + tc.keyword + "`"
			if !strings.Contains(np.Reason(), want) {
				t.Errorf("expected the reason to name %s, got: %s", want, np.Reason())
			}
		})
	}
}
