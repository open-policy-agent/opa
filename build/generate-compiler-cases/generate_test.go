// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cases

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"

	"github.com/open-policy-agent/opa/build/internal/corpusgen"
	"github.com/open-policy-agent/opa/v1/test/compilecases"
	"github.com/open-policy-agent/opa/v1/test/compilecases/testdata"
)

func TestGenerateFillsWantErrors(t *testing.T) {
	// The second case is authored with a diagnostic that does not match what the
	// compiler reports, to pin that generation leaves it that way: a message that
	// changes has to fail the runner, not be rewritten underneath it.
	corpus := `---
cases:
  - note: safety/filled-in
    modules:
      - |
        package test

        p if {
        	x == 2
        }
    exhaustive: true
  - note: safety/left-alone
    modules:
      - |
        package test

        p if {
        	x == 2
        }
    want_errors:
      - code: rego_unsafe_var_error
        row: 99
        message: something else entirely
`

	dir := t.TempDir()
	path := filepath.Join(dir, "test-safety.yaml")
	if err := os.WriteFile(path, []byte(corpus), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := Generate(dir); err != nil {
		t.Fatal(err)
	}

	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	set, err := compilecases.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(set.Cases) != 2 {
		t.Fatalf("expected 2 cases, got %d", len(set.Cases))
	}

	filled, left := set.Cases[0], set.Cases[1]

	want := compilecases.Error{
		Code:    "rego_unsafe_var_error",
		Row:     4,
		Col:     2,
		Message: "var x is unsafe",
	}
	if len(filled.WantErrors) != 1 || filled.WantErrors[0] != want {
		t.Errorf("expected %v, got %v", want, filled.WantErrors)
	}

	// want_errors is inserted ahead of exhaustive, which the case already carried.
	if i, j := bytes.Index(first, []byte("want_errors")), bytes.Index(first, []byte("exhaustive")); i > j {
		t.Errorf("expected want_errors before exhaustive:\n%s", first)
	}

	if len(left.WantErrors) != 1 || left.WantErrors[0].Row != 99 {
		t.Errorf("expected the authored diagnostic to survive, got %v", left.WantErrors)
	}

	// Generation is idempotent, including over the case it just filled in.
	if err := Generate(dir); err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Errorf("second generation changed the file:\n%s", string(second))
	}
}

// TestGenerateReproducesCommittedFixtures strips want_errors from the corpus and
// regenerates it. The committed diagnostics were reviewed by hand, so this checks
// the generator against expectations it did not write, which the drift test below
// cannot: that one would pass just as well if the generator filled nothing.
func TestGenerateReproducesCommittedFixtures(t *testing.T) {
	committed, err := compilecases.LoadFS(testdata.FS, ".")
	if err != nil {
		t.Fatal(err)
	}
	if len(committed.Cases) == 0 {
		t.Fatal("expected the committed corpus to hold cases")
	}

	dir := t.TempDir()

	if err := os.CopyFS(dir, testdata.FS); err != nil {
		t.Fatal(err)
	}

	stripped := 0
	err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}

		bs, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		var doc yaml.Node
		if err := yaml.Unmarshal(bs, &doc); err != nil {
			return err
		}

		for _, c := range corpusgen.MapValue(doc.Content[0], "cases").Content {
			if corpusgen.DeleteMapValue(c, "want_errors") {
				stripped++
			}
		}

		out, err := corpusgen.Encode(doc.Content[0])
		if err != nil {
			return err
		}

		return os.WriteFile(path, out, 0o600)
	})
	if err != nil {
		t.Fatal(err)
	}

	if stripped != len(committed.Cases) {
		t.Fatalf("expected to strip want_errors from all %d cases, stripped %d", len(committed.Cases), stripped)
	}

	if err := Generate(dir); err != nil {
		t.Fatal(err)
	}

	assertMatchesCommitted(t, dir)
}

// TestGeneratedFixturesDoNotDrift regenerates the committed corpus in place and
// checks that nothing moved, the way TestSchemaDoesNotDrift does for the IR plan
// schema.
func TestGeneratedFixturesDoNotDrift(t *testing.T) {
	dir := t.TempDir()

	if err := os.CopyFS(dir, testdata.FS); err != nil {
		t.Fatal(err)
	}

	if err := Generate(dir); err != nil {
		t.Fatal(err)
	}

	assertMatchesCommitted(t, dir)
}

func assertMatchesCommitted(t *testing.T, dir string) {
	t.Helper()

	err := fs.WalkDir(testdata.FS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}

		want, err := testdata.FS.ReadFile(path)
		if err != nil {
			return err
		}

		got, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(path)))
		if err != nil {
			return err
		}

		if !bytes.Equal(got, want) {
			t.Errorf("%s does not match the committed fixture:\nwant:\n%s\ngot:\n%s", path, want, got)
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestGenerateAttributesErrorsToTheirModule(t *testing.T) {
	corpus := `---
cases:
  - note: safety/across-modules
    modules:
      - |
        package a

        p if {
        	x == 2
        }
      - |
        package b

        q if {
        	y == 3
        }
    exhaustive: true
`

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "test-cases.yaml"), []byte(corpus), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := Generate(dir); err != nil {
		t.Fatal(err)
	}

	set, err := compilecases.Load(dir)
	if err != nil {
		t.Fatal(err)
	}

	want := []compilecases.Error{
		// The first module is the default, so it is not named.
		{Code: "rego_unsafe_var_error", Row: 4, Col: 2, Message: "var x is unsafe"},
		{Module: "test-1.rego", Code: "rego_unsafe_var_error", Row: 4, Col: 2, Message: "var y is unsafe"},
	}

	got := set.Cases[0].WantErrors
	if len(got) != len(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("expected %v, got %v", want[i], got[i])
		}
	}
}

// TestGenerateRecordsMoreThanTheErrorLimit pins the SetErrorLimit(0) in
// compileCase: at the compiler's default the twelfth diagnostic would be
// "too many errors" and the rest would be missing.
func TestGenerateRecordsMoreThanTheErrorLimit(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("---\ncases:\n  - note: safety/many\n    modules:\n      - |\n        package test\n")
	for i := range 12 {
		fmt.Fprintf(&sb, "\n        p%d if {\n        	x%d == 2\n        }\n", i, i)
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "test-cases.yaml"), []byte(sb.String()), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := Generate(dir); err != nil {
		t.Fatal(err)
	}

	set, err := compilecases.Load(dir)
	if err != nil {
		t.Fatal(err)
	}

	got := set.Cases[0].WantErrors
	if len(got) != 12 {
		t.Fatalf("expected 12 diagnostics, got %d: %v", len(got), got)
	}
	for _, e := range got {
		if e.Code != "rego_unsafe_var_error" {
			t.Errorf("expected only unsafe-var diagnostics, got %v", e)
		}
	}
}

func TestGenerateRejectsContradictoryCases(t *testing.T) {
	tests := []struct {
		note    string
		corpus  string
		wantErr string
	}{
		{
			note: "want_errors on modules that compile",
			corpus: `---
cases:
  - note: a
    modules:
      - |
        package test

        p := 1
    want_errors:
      - code: rego_compile_error
        row: 3
        message: nope
`,
			wantErr: "asserts 'want_errors', but the modules compile",
		},
		{
			note: "no assertion at all",
			corpus: `---
cases:
  - note: a
    modules:
      - |
        package test

        p := 1
`,
			wantErr: "the modules compile and the case asserts nothing",
		},
		{
			note: "a module that does not parse",
			corpus: `---
cases:
  - note: a
    modules:
      - |
        package test

        p := 03
`,
			wantErr: "test-0.rego does not parse",
		},
		{
			note:    "trailing whitespace in a module",
			corpus:  "---\ncases:\n  - note: a\n    modules:\n      - \"package test\\n\\np if {\\n\\tx == 2 \\n}\\n\"\n",
			wantErr: `"test-0.rego" line 4 has trailing whitespace`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "test-cases.yaml")
			if err := os.WriteFile(path, []byte(tc.corpus), 0o600); err != nil {
				t.Fatal(err)
			}

			err := Generate(dir)
			if err == nil {
				t.Fatalf("expected an error containing %q, got none", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected an error containing %q, got %v", tc.wantErr, err)
			}

			// The file is left as authored: nothing is rewritten on the way to failing.
			bs, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(bs) != tc.corpus {
				t.Errorf("expected the file to be untouched, got:\n%s", string(bs))
			}
		})
	}
}
