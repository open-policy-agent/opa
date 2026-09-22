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
			wantErr: "unknown rego version",
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

const oneCase = `
cases:
  - note: a
    module: package test
    want_ast: "{}"
`

func TestLoadRejectsDuplicateNotes(t *testing.T) {
	root, dir := newCorpus(t)
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

	_, err := Load(root)
	if err == nil || !strings.Contains(err.Error(), "note is already used by") {
		t.Fatalf("expected duplicate note error, got %v", err)
	}
}

// TestLoadRejectsDuplicateNotesAcrossFiles is the same rule between two files of one
// version, where the single-file check above cannot reach.
func TestLoadRejectsDuplicateNotesAcrossFiles(t *testing.T) {
	root, dir := newCorpus(t)

	for _, name := range []string{"test-one.yaml", "test-two.yaml"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(oneCase), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	_, err := Load(root)
	if err == nil || !strings.Contains(err.Error(), "note is already used by") {
		t.Fatalf("expected duplicate note error, got %v", err)
	}
}

// TestLoadAcceptsANoteReusedInAnotherVersion checks that a note may be reused under a
// different version directory. The same construct parsed as v0 and as v1 is two cases with
// one note, which most of the corpus relies on.
func TestLoadAcceptsANoteReusedInAnotherVersion(t *testing.T) {
	root := t.TempDir()

	for _, version := range []string{"v0", "v1", "v0-compat-v1"} {
		dir := filepath.Join(root, version)
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "test-cases.yaml"), []byte(oneCase), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	set, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(set.Cases) != 3 {
		t.Fatalf("expected 3 cases, got %d", len(set.Cases))
	}

	for _, tc := range set.Cases {
		if tc.RegoVersion == "" {
			t.Errorf("%s was not stamped with the version of the directory it came from", tc.Filename)
		}
	}
}

// TestLoadRejectsAnAuthoredRegoVersion checks that the directory stays the only statement
// of a case's version. The field is tagged out of the schema, so an authored one is an
// unknown field rather than a second source of truth.
func TestLoadRejectsAnAuthoredRegoVersion(t *testing.T) {
	root, dir := newCorpus(t)
	corpus := `
cases:
  - note: a
    rego_version: v0
    module: package test
    want_ast: "{}"
`
	if err := os.WriteFile(filepath.Join(dir, "test-cases.yaml"), []byte(corpus), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := Load(root)
	if err == nil || !strings.Contains(err.Error(), "rego_version") {
		t.Fatalf("expected the field to be rejected, got %v", err)
	}
}

func TestLoadRejectsACaseFileOutsideAVersionDirectory(t *testing.T) {
	tests := []struct {
		note    string
		path    string
		wantErr string
	}{
		{
			note:    "at the corpus root",
			path:    "test-cases.yaml",
			wantErr: "belongs under a rego version directory",
		},
		{
			note:    "under a directory that is not a version",
			path:    filepath.Join("terms", "test-cases.yaml"),
			wantErr: `under "terms", which is not a rego version directory`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, tc.path)
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(oneCase), 0o600); err != nil {
				t.Fatal(err)
			}

			_, err := Load(root)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected an error containing %q, got %v", tc.wantErr, err)
			}
		})
	}
}

// newCorpus returns a temporary corpus root and the version directory inside it that case
// files go in. A case's rego version comes from that directory, so the loader rejects a
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
