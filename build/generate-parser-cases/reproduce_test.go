// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cases

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"go.yaml.in/yaml/v3"

	"github.com/open-policy-agent/opa/build/internal/corpusgen"
	"github.com/open-policy-agent/opa/v1/test/parsercases"
	"github.com/open-policy-agent/opa/v1/test/parsercases/testdata"
)

// TestGenerateReproducesCommittedDiagnostics reduces every committed want_errors entry to
// the message a case could have authored, and checks the generator fills the rest back in
// exactly as committed.
//
// want_errors is filled once and never overwritten, so regenerating the committed corpus —
// what TestGeneratedFixturesDoNotDrift does — never re-enters the code that fills it:
// completeDiagnostics, coversAll and positionless go unreached there. Reducing every entry
// to its message puts all 960 failure cases through the path a hand-authored message takes.
func TestGenerateReproducesCommittedDiagnostics(t *testing.T) {
	failures := len(committedFailures(t))

	dir := t.TempDir()
	if err := os.CopyFS(dir, testdata.FS); err != nil {
		t.Fatal(err)
	}

	reduced := rewriteCases(t, dir, func(c *yaml.Node) bool {
		errs := corpusgen.MapValue(c, "want_errors")
		if errs == nil {
			return false
		}

		for _, e := range errs.Content {
			corpusgen.DeleteMapValue(e, "code")
			corpusgen.DeleteMapValue(e, "row")
			corpusgen.DeleteMapValue(e, "col")
		}
		return true
	})

	if reduced != failures {
		t.Fatalf("expected to reduce all %d failure cases, reduced %d", failures, reduced)
	}

	if err := Generate(dir); err != nil {
		t.Fatal(err)
	}

	assertMatchesCommitted(t, dir)
}

// TestGenerateSeedsCommittedDiagnostics strips want_errors outright, which is what a newly
// authored case looks like, and checks the generator seeds it back as committed.
//
// This is the firstDiagnostic and allDiagnostics path, also unreached by regeneration.
func TestGenerateSeedsCommittedDiagnostics(t *testing.T) {
	// Cases a bare seed does not reproduce, because a seed records the first diagnostic and
	// these record something else. Two reasons, both deliberate:
	//
	//   - the case names several messages, where a seed records only the first
	//   - the case names a message OPA reports after another one, or drops the col
	//
	// Named rather than counted so that a case arriving here is read as one of those choices
	// or as a regression, not as a number to bump.
	notSeedable := []string{
		"v1/imports/import/bad variable term (module)",
		"v1/imports/invalid-import-path",
		"v1/rules/rule-body-missing-if",
		"v1/rules/rule/no output (module)",
		"v1/templatestrings/template-string-error/empty template expression (body)",
		"v1/templatestrings/template-string-error/empty template expression (module)",
	}

	failures := len(committedFailures(t))

	dir := t.TempDir()
	if err := os.CopyFS(dir, testdata.FS); err != nil {
		t.Fatal(err)
	}

	stripped := rewriteCases(t, dir, func(c *yaml.Node) bool {
		return corpusgen.DeleteMapValue(c, "want_errors")
	})

	if stripped != failures {
		t.Fatalf("expected to strip want_errors from all %d failure cases, stripped %d", failures, stripped)
	}

	if err := Generate(dir); err != nil {
		t.Fatal(err)
	}

	assertSeeded(t, dir, notSeedable)
}

// caseKey identifies a case the way the corpus does: a note is unique within one version
// directory, not across the corpus, so both are needed.
func caseKey(tc *parsercases.TestCase) string {
	return tc.RegoVersion + "/" + tc.Note
}

// committedFailures returns the diagnostics every committed failure case asserts.
func committedFailures(t *testing.T) map[string][]parsercases.Error {
	t.Helper()

	set, err := parsercases.LoadFS(testdata.FS, ".")
	if err != nil {
		t.Fatal(err)
	}

	out := map[string][]parsercases.Error{}
	for i := range set.Cases {
		if tc := &set.Cases[i]; tc.Failure() {
			out[caseKey(tc)] = tc.WantErrors
		}
	}

	if len(out) == 0 {
		t.Fatal("expected the committed corpus to hold failure cases")
	}
	return out
}

// rewriteCases applies edit to every case in the corpus under dir, returning how many it
// reported having changed.
func rewriteCases(t *testing.T, dir string, edit func(*yaml.Node) bool) int {
	t.Helper()

	var edited int

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
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
			if edit(c) {
				edited++
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

	return edited
}

// assertSeeded checks that every case in dir matches the committed corpus, except those
// named in notSeedable, each of which must differ.
func assertSeeded(t *testing.T, dir string, notSeedable []string) {
	t.Helper()

	committed := committedFailures(t)

	set, err := parsercases.Load(dir)
	if err != nil {
		t.Fatal(err)
	}

	seeded := map[string][]parsercases.Error{}
	for i := range set.Cases {
		tc := &set.Cases[i]
		seeded[caseKey(tc)] = tc.WantErrors
	}

	differs := map[string]bool{}

	for note, want := range committed {
		got, ok := seeded[note]
		if !ok {
			t.Errorf("%s: no seeded case", note)
			continue
		}

		if !slices.Equal(want, got) {
			differs[note] = true
		}
	}

	for _, note := range notSeedable {
		if !differs[note] {
			t.Errorf("%s is listed as not seedable, but seeding reproduced it; drop it from the list", note)
		}
		delete(differs, note)
	}

	for note := range differs {
		t.Errorf("%s was not reproduced by seeding: either it is a deliberate authoring choice and belongs in the list here, or the generator has regressed\n  want %v\n  got  %v",
			note, committed[note], seeded[note])
	}
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
			t.Errorf("%s does not match the committed fixture", path)
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
