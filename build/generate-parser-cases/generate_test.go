// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cases

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/v1/test/parsercases"
)

func TestGenerateFillsWantErrors(t *testing.T) {
	// The second case is authored with a diagnostic that does not match what the
	// parser reports, to pin that generation leaves it that way: a message that
	// changes has to fail the runner, not be rewritten underneath it.
	corpus := `---
cases:
  - note: errors/filled-in
    module: |
      package test

      p := 03
  - note: errors/left-alone
    module: |
      package test

      p := 03
    want_errors:
      - code: rego_parse_error
        row: 99
        message: something else entirely
`

	dir := t.TempDir()
	path := filepath.Join(dir, "test-errors.yaml")
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

	set, err := parsercases.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(set.Cases) != 2 {
		t.Fatalf("expected 2 cases, got %d", len(set.Cases))
	}

	filled, left := set.Cases[0], set.Cases[1]

	want := parsercases.Error{
		Code:    "rego_parse_error",
		Row:     3,
		Col:     6,
		Message: "unexpected number token: expected number without leading zero",
	}
	if len(filled.WantErrors) != 1 || filled.WantErrors[0] != want {
		t.Errorf("expected %v, got %v", want, filled.WantErrors)
	}

	if len(left.WantErrors) != 1 || left.WantErrors[0].Row != 99 {
		t.Errorf("expected the authored diagnostic to survive, got %v", left.WantErrors)
	}

	// Neither case gets a want_ast: the module does not parse.
	for _, tc := range set.Cases {
		if tc.WantAST != "" {
			t.Errorf("%s: expected no want_ast on a failure case", tc.Note)
		}
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

func TestGenerateRejectsTrailingWhitespace(t *testing.T) {
	// The module is quoted so the trailing space survives being authored; a block
	// scalar the generator would be willing to write back cannot hold one.
	corpus := "---\ncases:\n" +
		"  - note: errors/trailing space\n" +
		"    module: \"package test\\n\\n[foo, bar, \\n\"\n"

	dir := t.TempDir()
	path := filepath.Join(dir, "test-cases.yaml")
	if err := os.WriteFile(path, []byte(corpus), 0o600); err != nil {
		t.Fatal(err)
	}

	err := Generate(dir)
	if err == nil {
		t.Fatal("expected generation to be rejected")
	}
	if want := `"module" line 3 has trailing whitespace`; !strings.Contains(err.Error(), want) {
		t.Fatalf("expected an error containing %q, got %v", want, err)
	}

	// The file is left as authored: nothing is rewritten on the way to failing.
	bs, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(bs) != corpus {
		t.Errorf("expected the file to be untouched, got:\n%s", string(bs))
	}
}

func TestGenerateRejectsContradictoryCases(t *testing.T) {
	tests := []struct {
		note    string
		corpus  string
		wantErr string
	}{
		{
			note: "want_errors on a module that parses",
			corpus: `---
cases:
  - note: a
    module: |
      package test

      p := 1
    want_errors:
      - code: rego_parse_error
        row: 3
        message: nope
`,
			wantErr: "asserts 'want_errors', but the module parses",
		},
		{
			note: "want_ast on a module that does not parse",
			corpus: `---
cases:
  - note: a
    module: |
      package test

      p := 03
    want_ast: |
      {}
`,
			wantErr: "asserts 'want_ast', but the module no longer parses",
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "test-cases.yaml"), []byte(tc.corpus), 0o600); err != nil {
				t.Fatal(err)
			}

			err := Generate(dir)
			if err == nil {
				t.Fatalf("expected an error containing %q, got none", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected an error containing %q, got %v", tc.wantErr, err)
			}
		})
	}
}
