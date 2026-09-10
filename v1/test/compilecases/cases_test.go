// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package compilecases

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestWantParserOptions(t *testing.T) {
	tests := []struct {
		note        string
		regoVersion string
		imports     [][]string
		module      int // which entry's imports to read; defaults to 0
		want        WantOptions
		wantErr     string
	}{
		{
			note: "no imports leaves the case's own version",
			want: WantOptions{},
		},
		{
			note:        "a v0 case stays v0",
			regoVersion: "v0",
			want:        WantOptions{RegoVersion: "v0"},
		},
		{
			note:        "a named keyword",
			regoVersion: "v0",
			imports:     [][]string{{"future.keywords.or"}},
			want:        WantOptions{RegoVersion: "v0", FutureKeywords: []string{"or"}},
		},
		{
			note:        "several named keywords",
			regoVersion: "v0",
			imports:     [][]string{{"future.keywords.and", "future.keywords.or"}},
			want:        WantOptions{RegoVersion: "v0", FutureKeywords: []string{"and", "or"}},
		},
		{
			note:        "the wildcard",
			regoVersion: "v0",
			imports:     [][]string{{"future.keywords"}},
			want:        WantOptions{RegoVersion: "v0", AllFutureKeywords: true},
		},
		{
			// The point of carrying rego.v1: the compiler strips it, and the printed
			// form of a v0 module that had it only parses as v1. Not v0-compat-v1 —
			// that mode requires the import the printed form no longer carries.
			note:        "rego.v1 overrides a v0 case's version",
			regoVersion: "v0",
			imports:     [][]string{{"rego.v1"}},
			want:        WantOptions{RegoVersion: "v1"},
		},
		{
			note:        "rego.v1 alongside a keyword",
			regoVersion: "v0",
			imports:     [][]string{{"future.keywords.in", "rego.v1"}},
			want:        WantOptions{RegoVersion: "v1", FutureKeywords: []string{"in"}},
		},
		{
			note:    "an unknown directive",
			imports: [][]string{{"future.something"}},
			wantErr: `unrecognised 'want[0].imports' entry "future.something"`,
		},
		{
			note:    "an unknown rego directive",
			imports: [][]string{{"rego.v2"}},
			wantErr: `unrecognised 'want[0].imports' entry "rego.v2"`,
		},
		{
			note:    "a nested keyword path",
			imports: [][]string{{"future.keywords.a.b"}},
			wantErr: `unrecognised 'want[0].imports' entry "future.keywords.a.b"`,
		},
		{
			note:    "an empty keyword",
			imports: [][]string{{"future.keywords."}},
			wantErr: `unrecognised 'want[0].imports' entry "future.keywords."`,
		},
		{
			note:    "an ordinary data import is not a directive",
			imports: [][]string{{"data.foo"}},
			wantErr: `unrecognised 'want[0].imports' entry "data.foo"`,
		},
		{
			// The reason the field is per module: one module's activation must not
			// reach its neighbour, or a neighbour using `not` as ordinary negation
			// would have it read as a Not node.
			note:        "imports do not carry across modules",
			regoVersion: "v0",
			imports:     [][]string{{"future.keywords.not"}, nil},
			module:      1,
			want:        WantOptions{RegoVersion: "v0"},
		},
		{
			note:        "the second module's own imports are read",
			regoVersion: "v0",
			imports:     [][]string{nil, {"rego.v1"}},
			module:      1,
			want:        WantOptions{RegoVersion: "v1"},
		},
		{
			note:        "a module beyond the declared lists has nothing in effect",
			regoVersion: "v0",
			imports:     [][]string{{"future.keywords.or"}},
			module:      5,
			want:        WantOptions{RegoVersion: "v0"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			c := TestCase{RegoVersion: tc.regoVersion}
			for _, imports := range tc.imports {
				c.Want = append(c.Want, Want{Module: "package t\n", Imports: imports})
			}

			got, err := c.WantParserOptions(tc.module)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("expected an error containing %q, got %+v", tc.wantErr, got)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("expected an error containing %q, got %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}

			if got.RegoVersion != tc.want.RegoVersion {
				t.Errorf("expected rego version %q, got %q", tc.want.RegoVersion, got.RegoVersion)
			}
			if !slices.Equal(got.FutureKeywords, tc.want.FutureKeywords) {
				t.Errorf("expected keywords %v, got %v", tc.want.FutureKeywords, got.FutureKeywords)
			}
			if got.AllFutureKeywords != tc.want.AllFutureKeywords {
				t.Errorf("expected all-future-keywords %v, got %v", tc.want.AllFutureKeywords, got.AllFutureKeywords)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	module := "package test\n\np if {\n\ttrue\n}\n"

	tests := []struct {
		note    string
		tc      TestCase
		wantErr string
	}{
		{
			note: "a failure case",
			tc:   TestCase{Note: "a", Modules: []string{module}, WantErrors: []Error{{Code: "x", Row: 1, Message: "m"}}},
		},
		{
			note: "a transformation case",
			tc:   TestCase{Note: "a", Modules: []string{module}, Want: []Want{{Module: module}}},
		},
		{
			note:    "no assertion at all",
			tc:      TestCase{Note: "a", Modules: []string{module}},
			wantErr: "expected 'want_errors', or 'want'",
		},
		{
			note: "an entry with both spellings",
			tc: TestCase{Note: "a", Modules: []string{module},
				Want: []Want{{Module: module, AST: "{}"}}},
			wantErr: "'want[0]' has both 'module' and 'ast'",
		},
		{
			note: "an entry with neither",
			tc: TestCase{Note: "a", Modules: []string{module},
				Want: []Want{{Imports: []string{"rego.v1"}}}},
			wantErr: "'want[0]' has neither 'module' nor 'ast'",
		},
		{
			note: "imports on an ast entry",
			tc: TestCase{Note: "a", Modules: []string{module},
				Want: []Want{{AST: "{}", Imports: []string{"rego.v1"}}}},
			wantErr: "'want[0].imports' only applies to an entry asserting 'module'",
		},
		{
			note: "want of the wrong length",
			tc: TestCase{Note: "a", Modules: []string{module, module},
				Want: []Want{{Module: module}}},
			wantErr: "'want' has 1 entries for 2 modules",
		},
		{
			// The AST form for one module and Rego for the other, which is what the
			// entries are for: an unprintable module does not drag its neighbour in.
			note: "a mixed case is well formed",
			tc: TestCase{Note: "a", Modules: []string{module, module},
				Want: []Want{{Module: module}, {AST: "{}"}}},
		},
		{
			note: "an unreadable import",
			tc: TestCase{Note: "a", Modules: []string{module},
				Want: []Want{{Module: module, Imports: []string{"nonsense"}}}},
			wantErr: "unrecognised 'want[0].imports' entry",
		},
		{
			note:    "an unknown strictness",
			tc:      TestCase{Note: "a", Modules: []string{module}, Strict: "sometimes", Want: []Want{{Module: module}}},
			wantErr: "unknown 'strict'",
		},
		{
			note: "exhaustive without want_errors",
			tc: TestCase{Note: "a", Modules: []string{module},
				Want: []Want{{Module: module}}, Exhaustive: true},
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
				t.Fatalf("expected an error containing %q, got none", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("expected an error containing %q, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestLoadRejectsAnUnknownField(t *testing.T) {
	dir := t.TempDir()
	corpus := "cases:\n  - note: a\n    modules: [package test]\n    no_such_field: true\n"

	if err := os.WriteFile(filepath.Join(dir, "cases.yaml"), []byte(corpus), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected loading to be rejected")
	}
	if want := `unknown field "no_such_field"`; !strings.Contains(err.Error(), want) {
		t.Errorf("expected an error containing %q, got %v", want, err)
	}
}
