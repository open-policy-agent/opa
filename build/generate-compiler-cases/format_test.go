// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cases

import (
	"errors"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/test/compilecases"
	"github.com/open-policy-agent/opa/v1/test/compilecases/testdata"
)

func TestFormatModule(t *testing.T) {
	tests := []struct {
		note     string
		version  ast.RegoVersion
		keywords []string // future keywords the compiled form needs, which it no longer imports
		module   string
		want     string
	}{
		{
			note:    "one expression per line",
			version: ast.RegoV1,
			module:  "package t\n\np if {\n\tx := 1\n\tx == 1\n}\n",
			want: `package t

p = true if {
	__local0__ = 1
	__local0__ = 1
}
`,
		},
		{
			note:    "an else chain breaks at every branch",
			version: ast.RegoV1,
			module:  "package t\n\np = 1 if {\n\tinput.x\n} else = 2\n",
			want: `package t

p = 1 if {
	input.x
} else = 2 if {
	true
}
`,
		},
		{
			note: "a default rule has no body to break, and keeps its one-line form",
			// The rule that follows it does break, so the two shapes sit together.
			version: ast.RegoV1,
			module:  "package t\n\ndefault p := 1\n\np := 2 if {\n\tinput.x\n}\n",
			want: `package t

default p := 1

p := 2 if {
	input.x
}
`,
		},
		{
			// No `if`, and `p[x]` rather than `p contains x`: the layout comes from
			// here but every token comes from OPA's printer, so the v0 spellings
			// survive without this file knowing about them.
			note:    "a v0 rule keeps v0 syntax",
			version: ast.RegoV0,
			module:  "package t\n\np[x] { x = 1 }\n",
			want: `package t

p[x] {
	x = 1
}
`,
		},
		{
			note:    "a v1 partial set keeps contains",
			version: ast.RegoV1,
			module:  "package t\n\np contains x if {\n\tx := 1\n}\n",
			want: `package t

p contains __local0__ if {
	__local0__ = 1
}
`,
		},
		{
			note:    "a nested every body is broken too",
			version: ast.RegoV1,
			module:  "package t\n\np if {\n\tevery x in [1] {\n\t\tx > 0\n\t}\n}\n",
			want: `package t

p = true if {
	__local2__ = [1]
	every __local0__, __local1__ in __local2__ {
		gt(__local1__, 0)
	}
}
`,
		},
		{
			// The operand bodies of a logical group are broken like any other body, and
			// the closing brace lines up with the line that opened it.
			note:     "an explicit group body is broken",
			version:  ast.RegoV1,
			keywords: []string{"and"},
			module: "package t\n\nimport future.keywords.and\n\n" +
				"p if {\n\ttrue and {\n\t\tx := 2\n\t\tx > 1\n\t}\n}\n",
			want: `package t

p = true if {
	true and {
		__local0__ = 2
		gt(__local0__, 1)
	}
}
`,
		},
		{
			// Both sides of a chain, with the operand that prints inline left alone.
			note:     "a chained group breaks each braced operand",
			version:  ast.RegoV1,
			keywords: []string{"and"},
			module: "package t\n\nimport future.keywords.and\n\n" +
				"p if {\n\t{ x := 1; x > 0 } and true and { y := 2; y > 0 }\n}\n",
			want: `package t

p = true if {
	{
		__local0__ = 1
		gt(__local0__, 0)
	} and true and {
		__local1__ = 2
		gt(__local1__, 0)
	}
}
`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			m, err := ast.ParseModuleWithOpts("t.rego", tc.module, ast.ParserOptions{RegoVersion: tc.version})
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

			got, err := formatModule(compiled, ast.ParserOptions{
				RegoVersion: tc.version,
				// The compiler resolves a directive import away and the printer does not
				// put it back, so the reparse has to be told what the module relied on.
				FutureKeywords: tc.keywords,
			})
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Errorf("expected:\n%s\ngot:\n%s", tc.want, got)
			}
		})
	}
}

// TestFormatModuleIsLayoutOnly is the property that matters: breaking the bodies
// must not change what the text parses to. formatModule refuses rather than
// returning text it cannot verify, so the property is that it refuses on exactly
// the modules OPA's own one-line printer fails to round-trip — no more and no
// fewer, across the whole corpus.
//
// Fewer would mean the layout broke a fixture. More would mean formatModule is
// papering over a printer bug, which is worth knowing rather than benefiting from
// silently.
func TestFormatModuleIsLayoutOnly(t *testing.T) {
	set, err := compilecases.LoadFS(testdata.FS, ".")
	if err != nil {
		t.Fatal(err)
	}
	if len(set.Cases) == 0 {
		t.Fatal("expected the committed corpus to hold cases")
	}

	var both, neither int

	for _, tc := range set.Sorted().Cases {
		// Only the cases that assert a compiled form. A case asserting diagnostics has
		// no printed form to check, and its modules are left partway through the
		// pipeline — where comparing a rule head whose key is set against one whose is
		// not panics inside Module.Equal. See tmp/external_compilers/opa_bugs.md.
		if !tc.Transform() && len(tc.WantStages) == 0 {
			continue
		}

		plain, err := roundTrips(tc, func(m *ast.Module) string { return m.String() })
		if err != nil {
			t.Fatalf("%s: %v", tc.Note, err)
		}

		formatted, err := formats(tc)
		if err != nil {
			t.Fatalf("%s: %v", tc.Note, err)
		}

		switch {
		case plain && formatted:
			both++
		case plain:
			t.Errorf("%s: String() round-trips but formatModule refuses", tc.Note)
		case formatted:
			t.Errorf("%s: formatModule accepts what String() cannot round-trip; it is hiding a printer bug", tc.Note)
		default:
			neither++
		}
	}

	if both == 0 {
		t.Fatal("expected some modules to round-trip")
	}
	t.Logf("%d modules round-trip through both printers, %d through neither", both, neither)
}

// formats compiles a case and reports whether formatModule accepts every module,
// which is to say whether its own check passed.
func formats(tc compilecases.TestCase) (bool, error) {
	popts, err := parserOptions(tc)
	if err != nil {
		return false, err
	}

	for _, compiled := range compileFor(tc, popts) {
		if _, ferr := formatModule(compiled, popts); ferr != nil {
			return false, nil
		}
	}

	return true, nil
}

// roundTrips compiles a case and reports whether every module survives being
// printed by print and parsed back.
func roundTrips(tc compilecases.TestCase, print func(*ast.Module) string) (bool, error) {
	popts, err := parserOptions(tc)
	if err != nil {
		return false, err
	}

	for _, compiled := range compileFor(tc, popts) {
		reparsed, perr := ast.ParseModuleWithOpts("formatted.rego", print(compiled), popts)
		if perr != nil {
			return false, nil
		}
		if !compiled.Equal(reparsed) {
			return false, nil
		}
	}

	return true, nil
}

// compileFor compiles a case and returns its compiled modules in order.
func compileFor(tc compilecases.TestCase, popts ast.ParserOptions) []*ast.Module {
	modules := map[string]*ast.Module{}
	for i, src := range tc.Modules {
		name := compilecases.ModuleName(i)
		modules[name] = ast.MustParseModuleWithOpts(src, popts)
	}

	c := ast.NewCompiler().
		SetErrorLimit(0).
		WithStrict(tc.StrictMode()).
		WithEnablePrintStatements(tc.PrintStatements)
	c.Compile(modules)

	out := make([]*ast.Module, 0, len(tc.Modules))
	for i := range tc.Modules {
		out = append(out, c.Modules[compilecases.ModuleName(i)])
	}
	return out
}

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
