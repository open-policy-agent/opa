// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cases

import (
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/test/compilecases"
)

func TestLoadCompilerTestCases(t *testing.T) {
	sets, err := LoadCompilerTestCases()
	if err != nil {
		t.Fatal(err)
	}

	if len(sets) == 0 {
		t.Fatal("expected at least one set")
	}

	notes := map[string]string{}
	for _, set := range sets {
		for _, tc := range set.Cases {
			if tc.Ignore {
				t.Errorf("%s: expected an unfiltered load to ignore nothing", tc.Note)
			}
			if len(tc.WantErrors) == 0 && !tc.Compiles {
				t.Errorf("%s: expected want_errors or compiles", tc.Note)
			}
			if other, ok := notes[tc.Note]; ok {
				t.Errorf("%s: note is already used by %s", tc.Note, other)
			}
			notes[tc.Note] = tc.Filename
		}
	}
}

func TestRegoVersionFilter(t *testing.T) {
	unfiltered, err := LoadCompilerTestCases()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		note      string
		supported []ast.RegoVersion
		rejects   func(version string) bool
	}{
		{
			note:      "v1 only",
			supported: []ast.RegoVersion{ast.RegoV1},
			rejects:   func(v string) bool { return v != "" && v != "v1" },
		},
		{
			note:      "v0 only",
			supported: []ast.RegoVersion{ast.RegoV0},
			rejects:   func(v string) bool { return v != "v0" },
		},
		{
			note:      "both",
			supported: []ast.RegoVersion{ast.RegoV0, ast.RegoV1},
			rejects:   func(string) bool { return false },
		},
		{
			note:      "none filters nothing",
			supported: nil,
			rejects:   func(string) bool { return false },
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			sets, err := LoadCompilerTestCasesFiltered([]Filters{RegoVersionFilter(tc.supported...)})
			if err != nil {
				t.Fatal(err)
			}

			if len(sets) != len(unfiltered) {
				t.Fatalf("expected filtering to keep every set, got %d of %d", len(sets), len(unfiltered))
			}

			var ignored, kept int

			for i, set := range sets {
				if len(set.Cases) != len(unfiltered[i].Cases) {
					t.Fatalf("expected filtering to keep every case, got %d of %d",
						len(set.Cases), len(unfiltered[i].Cases))
				}

				for j, c := range set.Cases {
					ref := unfiltered[i].Cases[j]
					if c.Note != ref.Note {
						t.Fatalf("expected filtering to preserve order, got %q where %q was", c.Note, ref.Note)
					}

					if want := tc.rejects(c.RegoVersion); want != c.Ignore {
						t.Errorf("%s: rego_version %q, ignore is %v, expected %v",
							c.Note, c.RegoVersion, c.Ignore, want)
					}

					if c.Ignore {
						ignored++
					} else {
						kept++
					}

					// The case is marked, never removed or emptied: the corpus
					// stays addressable, and what a consumer does with an ignored
					// case is its own business.
					if len(c.WantErrors) != len(ref.WantErrors) {
						t.Errorf("%s: expected the diagnostics of the case to survive filtering", c.Note)
					}
				}
			}

			if len(tc.supported) == 1 && (ignored == 0 || kept == 0) {
				t.Errorf("expected %s to reject some cases and keep others, rejected %d of %d",
					tc.note, ignored, ignored+kept)
			}
			if len(tc.supported) != 1 && ignored != 0 {
				t.Errorf("expected %s to reject nothing, rejected %d", tc.note, ignored)
			}
		})
	}
}

func TestStrictModeFilter(t *testing.T) {
	unfiltered, err := LoadCompilerTestCases()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		note      string
		supported []string
		rejects   func(strict string) bool
	}{
		{
			note:      "a switchable strict mode runs everything",
			supported: nil,
			rejects:   func(string) bool { return false },
		},
		{
			note:      "no strict mode rejects the cases that need it on",
			supported: []string{compilecases.StrictDisabled},
			rejects:   func(s string) bool { return s == compilecases.StrictEnabled },
		},
		{
			note:      "unconditional strict rejects the cases that need it off",
			supported: []string{compilecases.StrictEnabled},
			rejects:   func(s string) bool { return s == compilecases.StrictDisabled },
		},
		{
			note:      "both named is the same as neither",
			supported: []string{compilecases.StrictEnabled, compilecases.StrictDisabled},
			rejects:   func(string) bool { return false },
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			sets, err := LoadCompilerTestCasesFiltered([]Filters{StrictModeFilter(tc.supported...)})
			if err != nil {
				t.Fatal(err)
			}

			if len(sets) != len(unfiltered) {
				t.Fatalf("expected filtering to keep every set, got %d of %d", len(sets), len(unfiltered))
			}

			var unset int

			for i, set := range sets {
				for j, c := range set.Cases {
					ref := unfiltered[i].Cases[j]
					if c.Note != ref.Note {
						t.Fatalf("expected filtering to preserve order, got %q where %q was", c.Note, ref.Note)
					}

					if want := tc.rejects(c.Strict); want != c.Ignore {
						t.Errorf("%s: strict %q, ignore is %v, expected %v", c.Note, c.Strict, c.Ignore, want)
					}

					// A case that says nothing about strict mode is runnable by
					// every consumer, whatever it declares.
					if c.Strict == "" {
						unset++
						if c.Ignore {
							t.Errorf("%s: leaves strict unset but was rejected", c.Note)
						}
					}
				}
			}

			if unset == 0 {
				t.Fatal("expected some cases to leave strict unset")
			}
		})
	}
}

// TestStrictModeFilterRejectsSomething guards against the filter appearing to work
// only because no committed case pins a strict setting.
func TestStrictModeFilterRejectsSomething(t *testing.T) {
	sets, err := LoadCompilerTestCasesFiltered([]Filters{StrictModeFilter(compilecases.StrictEnabled)})
	if err != nil {
		t.Fatal(err)
	}

	var rejected int
	for _, set := range sets {
		for _, c := range set.Cases {
			if c.Ignore {
				rejected++
			}
		}
	}
	if rejected == 0 {
		t.Error("no committed case pins strict: disabled, so the filter is untested")
	}
}
