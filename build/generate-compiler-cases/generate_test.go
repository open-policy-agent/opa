// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cases

import (
	"bytes"
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

// TestGenerateReproducesCommittedFixtures strips want_errors from the corpus and
// regenerates it. The committed diagnostics were reviewed by hand, so this checks
// the generator against expectations it did not write, which the drift test below
// cannot: that one would pass just as well if the generator filled nothing.
func TestGenerateReproducesCommittedFixtures(t *testing.T) {
	committed, err := compilecases.LoadFS(testdata.FS, ".")
	if err != nil {
		t.Fatal(err)
	}
	// Only a failure case has want_errors to strip; a transformation case asserts
	// what its modules compile to instead.
	var failures int
	for _, tc := range committed.Cases {
		if tc.Failure() {
			failures++
		}
	}
	if failures == 0 {
		t.Fatal("expected the committed corpus to hold failure cases")
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

	if stripped != failures {
		t.Fatalf("expected to strip want_errors from all %d failure cases, stripped %d", failures, stripped)
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

func TestGenerateRejectsATransformThatReportsDiagnostics(t *testing.T) {
	corpus := `---
cases:
  - note: safety/does not compile
    want:
      - module: |
          package test

          p = true if { true }
    modules:
      - |
        package test

        p if {
        	x == 2
        }
`

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "test-cases.yaml"), []byte(corpus), 0o600); err != nil {
		t.Fatal(err)
	}

	err := Generate(dir)
	if err == nil {
		t.Fatal("expected generation to be rejected")
	}
	if want := "they report 1 diagnostic(s)"; !strings.Contains(err.Error(), want) {
		t.Fatalf("expected an error containing %q, got %v", want, err)
	}
}

func TestGenerateRejectsTransformThatDoesNotCompile(t *testing.T) {
	corpus := `---
cases:
  - note: transforms/does not compile
    modules:
      - |
        package test

        p if {
        	x == 2
        }
    want:
      - module: |
          package test

          p = true if { true }
`

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "test-cases.yaml"), []byte(corpus), 0o600); err != nil {
		t.Fatal(err)
	}

	err := Generate(dir)
	if err == nil {
		t.Fatal("expected generation to be rejected")
	}
	if want := "they report 1 diagnostic(s)"; !strings.Contains(err.Error(), want) {
		t.Fatalf("expected an error containing %q, got %v", want, err)
	}
}

// TestGenerateFallsBackToWantAST covers the choice the generator makes for the
// author. `else :=` is a compiled form OPA's printer cannot write as Rego that
// parses back to it, so the case gets a marshalled AST instead — and the decision
// is taken by attempting the round-trip, not by anything in the case.
func TestGenerateFallsBackToWantAST(t *testing.T) {
	corpus := `---
cases:
  - note: transforms/else assign
    modules:
      - |
        package test

        p := 1 if {
        	input.x
        } else := 2
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

	tc := set.Cases[0]
	if len(tc.Want) != 1 {
		t.Fatalf("expected one entry, got %d", len(tc.Want))
	}
	if tc.Want[0].Module != "" {
		t.Errorf("expected no module, got %q", tc.Want[0].Module)
	}
	if !strings.Contains(tc.Want[0].AST, `"assign": true`) {
		t.Errorf("expected the AST to record the assignment the printer drops, got:\n%s", tc.Want[0].AST)
	}
}

// TestGenerateRejectsAnUnknownField pins that a field the schema does not declare
// fails generation, rather than loading as though it were not there.
func TestGenerateRejectsAnUnknownField(t *testing.T) {
	corpus := `---
cases:
  - note: transforms/unknown field
    modules:
      - |
        package test

        p if {
        	true
        }
    no_such_field: true
`

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "test-cases.yaml"), []byte(corpus), 0o600); err != nil {
		t.Fatal(err)
	}

	err := Generate(dir)
	if err == nil {
		t.Fatal("expected generation to be rejected")
	}
	if want := `unknown field "no_such_field"`; !strings.Contains(err.Error(), want) {
		t.Fatalf("expected an error containing %q, got %v", want, err)
	}
}

// TestGenerateDropsStagesMatchingTheFullPipeline is the difference gate. A stage
// whose form is the endpoint's again asserts nothing the endpoint does not, and
// keeping it would pin OPA's stage decomposition for no gain.
func TestGenerateDropsStagesMatchingTheFullPipeline(t *testing.T) {
	corpus := `---
cases:
  - note: transforms/nothing left to do
    modules:
      - |
        package test

        q := 1

        p if {
        	[q]
        }
    want_stages:
      RewriteDynamicTerms: []
      BuildRequiredCapabilities: []
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

	if got := set.Cases[0].SortedStages(); len(got) != 0 {
		t.Fatalf("expected every stage to be dropped, got %v", got)
	}
	if bs, err := os.ReadFile(filepath.Join(dir, "test-cases.yaml")); err != nil {
		t.Fatal(err)
	} else if strings.Contains(string(bs), "want_stages") {
		t.Errorf("expected want_stages to be removed from the file, got:\n%s", bs)
	}
}

// TestGenerateRejectsAnUnreachableStage keeps a defective case loud. A case whose
// diagnostics are raised before the stage it pins has no form to record there, and
// finding that out at load time would turn a corpus defect into a silent skip.
func TestGenerateRejectsAnUnreachableStage(t *testing.T) {
	corpus := `---
cases:
  - note: safety/unsafe var, stage pinned after the check
    modules:
      - |
        package test

        p if {
        	x == 2
        }
    want_errors:
      - code: rego_unsafe_var_error
        row: 3
        col: 4
        message: var x is unsafe
    want_stages:
      CheckTypes: []
`

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "test-cases.yaml"), []byte(corpus), 0o600); err != nil {
		t.Fatal(err)
	}

	err := Generate(dir)
	if err == nil {
		t.Fatal("expected generation to be rejected")
	}
	if want := "compiling up to CheckTypes reports"; !strings.Contains(err.Error(), want) {
		t.Fatalf("expected an error containing %q, got %v", want, err)
	}
}

// TestGenerateRejectsAnUnknownStage guards the vocabulary. WithOnlyStagesUpTo runs
// the whole pipeline when it does not recognise the stage, so a typo would
// otherwise record the full-pipeline form under a name that means nothing.
func TestGenerateRejectsAnUnknownStage(t *testing.T) {
	corpus := `---
cases:
  - note: transforms/typo
    modules:
      - |
        package test

        p if {
        	true
        }
    want_stages:
      RewriteDynamicTerm: []
`

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "test-cases.yaml"), []byte(corpus), 0o600); err != nil {
		t.Fatal(err)
	}

	err := Generate(dir)
	if err == nil {
		t.Fatal("expected generation to be rejected")
	}
	if want := `"RewriteDynamicTerm", which is not a compiler stage`; !strings.Contains(err.Error(), want) {
		t.Fatalf("expected an error containing %q, got %v", want, err)
	}
}

// TestCompileCaseToStageRejectsAStageTheCompilerDoesNotHave guards the one path
// where a stale name does real damage. WithOnlyStagesUpTo runs the whole pipeline
// when it does not recognise its argument, so without this check a renamed stage
// would compile to the endpoint, match the full-pipeline want, and be dropped by the
// difference gate — deleting the assertion instead of failing.
func TestCompileCaseToStageRejectsAStageTheCompilerDoesNotHave(t *testing.T) {
	tc := compilecases.TestCase{
		Note:    "transforms/stale stage",
		Modules: []string{"package test\n\np if {\n\ttrue\n}\n"},
	}

	popts, err := parserOptions(tc)
	if err != nil {
		t.Fatal(err)
	}

	// A name compilecases.Stages could plausibly still carry after OPA renamed it.
	if _, err := compileCaseToStage(tc, popts, "RewriteEqualsOp"); err == nil {
		t.Fatal("expected a stage the compiler does not have to be rejected")
	} else if want := `"RewriteEqualsOp" is not one of the compiler's stages`; !strings.Contains(err.Error(), want) {
		t.Fatalf("expected an error containing %q, got %v", want, err)
	}

	// The real one still works, so the guard is not rejecting everything.
	if _, err := compileCaseToStage(tc, popts, "RewriteEquals"); err != nil {
		t.Fatalf("expected a real stage to be accepted, got %v", err)
	}
}
