// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cases

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/v1/test/parsercases"
)

func TestGenerateRejectsTrailingWhitespace(t *testing.T) {
	// The module is quoted so the trailing space survives being authored; a block
	// scalar the generator would be willing to write back cannot hold one.
	corpus := "---\ncases:\n" +
		"  - note: errors/trailing space\n" +
		"    module: \"package test\\n\\n[foo, bar, \\n\"\n"

	root, dir := newCorpus(t)
	path := filepath.Join(dir, "test-cases.yaml")
	if err := os.WriteFile(path, []byte(corpus), 0o600); err != nil {
		t.Fatal(err)
	}

	err := Generate(root)
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
			wantErr: "asserts 'want_errors', but the policy parses",
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
			wantErr: "asserts 'want_ast', but the policy no longer parses",
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			root, dir := newCorpus(t)
			if err := os.WriteFile(filepath.Join(dir, "test-cases.yaml"), []byte(tc.corpus), 0o600); err != nil {
				t.Fatal(err)
			}

			err := Generate(root)
			if err == nil {
				t.Fatalf("expected an error containing %q, got none", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected an error containing %q, got %v", tc.wantErr, err)
			}
		})
	}
}

// TestGenerateCompletesPinnedDiagnostic covers the case that names the diagnostic it
// asserts because OPA reports it after another one: the message is the author's, the
// position and the code are filled in, and a message no parse reports fails generation.
func TestGenerateCompletesPinnedDiagnostic(t *testing.T) {
	tests := []struct {
		note    string
		corpus  string
		wantErr string
		want    parsercases.Error
	}{
		{
			note: "a message OPA reports second is completed",
			corpus: `---
cases:
  - note: errors/pinned
    module: |
      package test

      p if {
      	$"{}"
      }
    want_errors:
      - message: invalid template-string expression
`,
			// The parser reports `unexpected } token` first, which is what a case that
			// names nothing would have recorded.
			want: parsercases.Error{Code: "rego_parse_error", Row: 4, Col: 5, Message: "invalid template-string expression"},
		},
		{
			note: "a message no parse reports fails generation",
			corpus: `---
cases:
  - note: errors/not reported
    module: |
      package test

      p if {
      	$"{}"
      }
    want_errors:
      - message: something else entirely
`,
			wantErr: `names "something else entirely", which the parser does not report`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			root, dir := newCorpus(t)
			path := filepath.Join(dir, "test-cases.yaml")
			if err := os.WriteFile(path, []byte(tc.corpus), 0o600); err != nil {
				t.Fatal(err)
			}

			err := Generate(root)

			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("expected an error containing %q, got %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}

			set, err := parsercases.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			if len(set.Cases) != 1 || len(set.Cases[0].WantErrors) != 1 {
				t.Fatalf("expected one case with one diagnostic, got %v", set.Cases)
			}
			if got := set.Cases[0].WantErrors[0]; got != tc.want {
				t.Errorf("expected %v, got %v", tc.want, got)
			}
		})
	}
}

// TestGenerateCompletesRepeatedMessages covers a message the parser reports more than
// once: naming it twice asks for both positions, and naming it more often than it is
// reported is an error rather than a fixture with the same position twice.
func TestGenerateCompletesRepeatedMessages(t *testing.T) {
	// Both operands of the `and` are rejected, so one message arrives at two positions.
	const module = `
    module: |
      package test

      import future.keywords.and

      p if {
      	print("x") and print("x")
      }
`
	const message = "      - message: 'operand of `and` cannot consist only of calls to `print` " +
		"(hint: `print` produces no value and always succeeds, so the operand can never fail; " +
		"move it out of the operand, or add an expression that can fail)'\n"

	tests := []struct {
		note    string
		corpus  string
		want    []int // the positions the completed diagnostics carry
		wantErr string
	}{
		{
			note:   "named twice, one position each",
			corpus: "---\ncases:\n  - note: errors/twice" + module + "    want_errors:\n" + message + message,
			want:   []int{2, 17},
		},
		{
			note:    "named more often than reported",
			corpus:  "---\ncases:\n  - note: errors/thrice" + module + "    want_errors:\n" + message + message + message,
			wantErr: "more often than the parser reports it",
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			root, dir := newCorpus(t)
			if err := os.WriteFile(filepath.Join(dir, "test-cases.yaml"), []byte(tc.corpus), 0o600); err != nil {
				t.Fatal(err)
			}

			err := Generate(root)

			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("expected an error containing %q, got %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}

			set, err := parsercases.Load(root)
			if err != nil {
				t.Fatal(err)
			}

			var cols []int
			for _, e := range set.Cases[0].WantErrors {
				cols = append(cols, e.Col)
			}
			if !slices.Equal(cols, tc.want) {
				t.Errorf("expected the diagnostics at columns %v, got %v", tc.want, cols)
			}
		})
	}
}

// TestGenerateFillsEveryDiagnosticWhenExhaustive covers what `exhaustive` asks of the
// generator: the case asserts that the recorded set is the whole one, so recording the
// first diagnostic alone would write a fixture the runner rejects for the rest.
func TestGenerateFillsEveryDiagnosticWhenExhaustive(t *testing.T) {
	// `$"{}"` reports two diagnostics: `unexpected } token` and, after it, `invalid
	// template-string expression`.
	const module = `
    module: |
      package test

      p if {
      	$"{}"
      }
`

	tests := []struct {
		note    string
		corpus  string
		want    []parsercases.Error
		wantErr string
	}{
		{
			note:   "every diagnostic, sorted",
			corpus: "---\ncases:\n  - note: errors/exhaustive" + module + "    exhaustive: true\n",
			want: []parsercases.Error{
				{Code: "rego_parse_error", Row: 4, Col: 5, Message: "invalid template-string expression"},
				{Code: "rego_parse_error", Row: 4, Col: 5, Message: "unexpected } token"},
			},
		},
		{
			note: "an exhaustive case that names only one of them fails",
			corpus: "---\ncases:\n  - note: errors/partial" + module +
				"    want_errors:\n      - message: invalid template-string expression\n    exhaustive: true\n",
			wantErr: `also reports "unexpected } token"`,
		},
		{
			note:   "without exhaustive, the first alone",
			corpus: "---\ncases:\n  - note: errors/first" + module,
			want: []parsercases.Error{
				{Code: "rego_parse_error", Row: 4, Col: 5, Message: "unexpected } token"},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			root, dir := newCorpus(t)
			if err := os.WriteFile(filepath.Join(dir, "test-cases.yaml"), []byte(tc.corpus), 0o600); err != nil {
				t.Fatal(err)
			}

			err := Generate(root)

			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("expected an error containing %q, got %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}

			set, err := parsercases.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			if got := set.Cases[0].WantErrors; !slices.Equal(got, tc.want) {
				t.Errorf("expected %v, got %v", tc.want, got)
			}
		})
	}
}

// newCorpus returns a temporary corpus root and the version directory inside it that case
// files go in. A case's rego version comes from that directory, so the generator rejects a
// file sitting at the root.
func newCorpus(t *testing.T) (root, versionDir string) {
	t.Helper()

	root = t.TempDir()
	versionDir = filepath.Join(root, "v1")
	if err := os.Mkdir(versionDir, 0o755); err != nil {
		t.Fatal(err)
	}
	return root, versionDir
}
