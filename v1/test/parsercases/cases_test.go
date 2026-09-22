// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package parsercases

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		note    string
		tc      TestCase
		wantErr string
	}{
		{
			note: "success case",
			tc:   TestCase{Note: "a", Module: "package test", WantAST: "{}"},
		},
		{
			note: "success case with equivalent module",
			tc:   TestCase{Note: "a", Module: "package test", WantAST: "{}", WantEquivalent: "package test"},
		},
		{
			note: "failure case",
			tc:   TestCase{Note: "a", Module: "package test", WantErrors: []Error{{Code: "rego_parse_error"}}},
		},
		{
			note:    "missing note",
			tc:      TestCase{Module: "package test", WantAST: "{}"},
			wantErr: "missing 'note'",
		},
		{
			note:    "missing module",
			tc:      TestCase{Note: "a", WantAST: "{}"},
			wantErr: "missing 'module'",
		},
		{
			note:    "unknown rego version",
			tc:      TestCase{Note: "a", Module: "package test", RegoVersion: "v2", WantAST: "{}"},
			wantErr: "unknown 'rego_version'",
		},
		{
			note:    "neither want_ast nor want_errors",
			tc:      TestCase{Note: "a", Module: "package test"},
			wantErr: "expected 'want_ast' or 'want_errors'",
		},
		{
			note:    "want_ast and want_errors",
			tc:      TestCase{Note: "a", Module: "package test", WantAST: "{}", WantErrors: []Error{{Code: "rego_parse_error"}}},
			wantErr: "'want_ast' is not expected",
		},
		{
			note:    "want_equivalent and want_errors",
			tc:      TestCase{Note: "a", Module: "package test", WantEquivalent: "package test", WantErrors: []Error{{Code: "rego_parse_error"}}},
			wantErr: "'want_equivalent' is not expected",
		},
		{
			note:    "locations and want_errors",
			tc:      TestCase{Note: "a", Module: "package test", Locations: true, WantErrors: []Error{{Code: "rego_parse_error"}}},
			wantErr: "'locations' has no effect",
		},
		{
			note:    "want_equivalent and locations",
			tc:      TestCase{Note: "a", Module: "package test", WantAST: "{}", WantEquivalent: "package test", Locations: true},
			wantErr: "mutually exclusive",
		},
		{
			note: "a policy that merely mentions comments is not a stale fixture",
			tc: TestCase{Note: "a", Module: "package test", WantAST: `{
  "rules": [
    {
      "head": {
        "value": {
          "type": "string",
          "value": "comments"
        }
      }
    }
  ]
}
`},
		},
		{
			note:    "a fixture that is not JSON",
			tc:      TestCase{Note: "a", Module: "package test", WantAST: "{not json"},
			wantErr: "'want_ast' is not valid JSON",
		},
		{
			note:    "a fixture malformed deeper in",
			tc:      TestCase{Note: "a", Module: "package test", WantAST: `{"rules": [{"head": }]}`},
			wantErr: "'want_ast' is not valid JSON",
		},
		{
			note: "a fixture recording comments is stale",
			tc: TestCase{Note: "a", Module: "package test", WantAST: `{
  "package": {},
  "comments": [
    {}
  ]
}
`},
			wantErr: "records comments",
		},
		{
			note:    "trailing whitespace in the module",
			tc:      TestCase{Note: "a", Module: "package test\n\np := [1, \n", WantAST: "{}"},
			wantErr: `"module" line 3 has trailing whitespace`,
		},
		{
			note:    "trailing whitespace in want_equivalent",
			tc:      TestCase{Note: "a", Module: "package test", WantAST: "{}", WantEquivalent: "package test \n"},
			wantErr: `"want_equivalent" line 1 has trailing whitespace`,
		},
		{
			note:    "exhaustive without want_errors",
			tc:      TestCase{Note: "a", Module: "package test", WantAST: "{}", Exhaustive: true},
			wantErr: "'exhaustive' only applies",
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			err := tc.tc.Validate()

			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
				return
			}

			if err == nil {
				t.Fatalf("expected error containing %q, got none", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestLoadRejectsDuplicateNotes(t *testing.T) {
	dir := t.TempDir()
	corpus := `
cases:
  - note: a
    module: package test
    want_ast: "{}"
  - note: a
    module: package test
    want_ast: "{}"
`
	if err := os.WriteFile(filepath.Join(dir, "test-cases.yaml"), []byte(corpus), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := Load(dir)
	if err == nil || !strings.Contains(err.Error(), "note is already used by") {
		t.Fatalf("expected duplicate note error, got %v", err)
	}
}

// TestLoadAcceptsANoteReusedAcrossRegoVersions checks that a note may be reused under a
// different rego_version. The same construct parsed as v0 and as v1 is two cases with one
// note, so most of the corpus does this.
func TestLoadAcceptsANoteReusedAcrossRegoVersions(t *testing.T) {
	dir := t.TempDir()
	corpus := `
cases:
  - note: a
    module: package test
    want_ast: "{}"
  - note: a
    rego_version: v0
    module: package test
    want_ast: "{}"
  - note: a
    rego_version: v0-compat-v1
    module: package test
    want_ast: "{}"
`
	if err := os.WriteFile(filepath.Join(dir, "test-cases.yaml"), []byte(corpus), 0o600); err != nil {
		t.Fatal(err)
	}

	set, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(set.Cases) != 3 {
		t.Fatalf("expected 3 cases, got %d", len(set.Cases))
	}
}

// TestLoadRejectsANoteReusedWithinOneRegoVersion covers the version being spelled two ways:
// an absent rego_version is v1, so these two collide.
func TestLoadRejectsANoteReusedWithinOneRegoVersion(t *testing.T) {
	dir := t.TempDir()
	corpus := `
cases:
  - note: a
    module: package test
    want_ast: "{}"
  - note: a
    rego_version: v1
    module: package test
    want_ast: "{}"
`
	if err := os.WriteFile(filepath.Join(dir, "test-cases.yaml"), []byte(corpus), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := Load(dir)
	if err == nil || !strings.Contains(err.Error(), "for the same rego version") {
		t.Fatalf("expected duplicate note error, got %v", err)
	}
}
