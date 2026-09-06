// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cases

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/test/parsercases/testdata"
)

const planSchemaPath = "../../v1/ir/plan.schema.json"

func TestLoadParserTestCases(t *testing.T) {
	sets, err := LoadParserTestCases()
	if err != nil {
		t.Fatal(err)
	}

	if len(sets) == 0 {
		t.Fatal("expected at least one set of test cases")
	}

	for _, set := range sets {
		for _, tc := range set.Cases {
			if tc.WantIR != nil {
				t.Errorf("%s: want_ir is generated on request, it must not be committed", tc.Note)
			}
		}
	}
}

// TestLoadParserTestCasesWithIR checks that IR generation runs to completion,
// not that any individual plan is correct: no plan is committed, so there is
// nothing to compare one against. What is asserted is that every plan produced
// conforms to the published schema, and that planning twice gives the same
// answer.
func TestLoadParserTestCasesWithIR(t *testing.T) {
	schema := compilePlanSchema(t)

	sets, err := LoadParserTestCases(WithIR())
	if err != nil {
		t.Fatal(err)
	}

	again, err := LoadParserTestCases(WithIR())
	if err != nil {
		t.Fatal(err)
	}

	planned := 0
	authored := false
	explained := 0

	for i, set := range sets {
		for j, tc := range set.Cases {
			if tc.Failure() {
				continue
			}

			// A success case either plans, or says why it did not.
			switch {
			case tc.WantIR == nil && tc.IRError == "":
				t.Errorf("%s: no plan and no reason for it", tc.Note)
			case tc.WantIR != nil && tc.IRError != "":
				t.Errorf("%s: planned, but also reported %q", tc.Note, tc.IRError)
			}

			if tc.WantIR == nil {
				explained++
				continue
			}
			planned++

			if len(tc.EntryPoints) == 0 {
				t.Errorf("%s: expected the entrypoints the plan was generated for", tc.Note)
			}

			// The case authoring entrypoints must be planned for those alone,
			// not for every ground rule ref in its module.
			if tc.Note == "rules/authored-entrypoints" {
				authored = true
				if want := []string{"test/helper"}; !slices.Equal(tc.EntryPoints, want) {
					t.Errorf("%s: expected the authored entrypoints %v, got %v", tc.Note, want, tc.EntryPoints)
				}
			}

			names := make([]string, len(tc.WantIR.Plans.Plans))
			for i, plan := range tc.WantIR.Plans.Plans {
				names[i] = plan.Name
			}

			if !slices.Equal(names, tc.EntryPoints) {
				t.Errorf("%s: expected a plan per entrypoint %v, got %v", tc.Note, tc.EntryPoints, names)
			}

			bs, err := json.Marshal(tc.WantIR)
			if err != nil {
				t.Fatalf("%s: marshal plan: %v", tc.Note, err)
			}

			var doc any
			if err := json.Unmarshal(bs, &doc); err != nil {
				t.Fatalf("%s: unmarshal plan: %v", tc.Note, err)
			}

			if err := schema.Validate(doc); err != nil {
				t.Errorf("%s: plan does not validate:\n%s\n\nplan JSON:\n%s",
					tc.Note, strings.TrimSpace(err.Error()), string(bs))
			}

			other, err := json.Marshal(again[i].Cases[j].WantIR)
			if err != nil {
				t.Fatalf("%s: marshal plan: %v", tc.Note, err)
			}

			if !bytes.Equal(bs, other) {
				t.Errorf("%s: planning is not deterministic", tc.Note)
			}
		}
	}

	if planned == 0 {
		t.Fatal("expected at least one case to produce a plan")
	}

	if explained == 0 {
		t.Fatal("expected at least one case to report why it produced no plan")
	}

	if !authored {
		t.Fatal("expected a case authoring entrypoints to produce a plan")
	}
}

func TestLoadParserTestCasesWithASTLocations(t *testing.T) {
	sets, err := LoadParserTestCases(WithASTLocations())
	if err != nil {
		t.Fatal(err)
	}

	for _, set := range sets {
		for _, tc := range set.Cases {
			if tc.Failure() {
				continue
			}
			if !strings.Contains(tc.WantAST, `"location"`) {
				t.Errorf("%s: expected want_ast to carry locations", tc.Note)
			}
			if tc.WantEquivalent != "" {
				t.Errorf("%s: expected want_equivalent to be dropped, positions differ from the module's", tc.Note)
			}
		}
	}
}

func TestCapabilitiesFilter(t *testing.T) {
	capabilities := &ast.Capabilities{Builtins: []*ast.Builtin{ast.BuiltinMap["plus"]}}

	unfiltered, err := LoadParserTestCases(WithIR())
	if err != nil {
		t.Fatal(err)
	}

	sets, err := LoadParserTestCasesFiltered([]Filters{CapabilitiesFilter(capabilities)}, WithIR())
	if err != nil {
		t.Fatal(err)
	}

	if len(sets) != len(unfiltered) {
		t.Fatalf("expected filtering to keep every set, got %d of %d", len(sets), len(unfiltered))
	}

	var ignored, kept int

	for i, set := range sets {
		if len(set.Cases) != len(unfiltered[i].Cases) {
			t.Fatalf("expected filtering to keep every case, got %d of %d", len(set.Cases), len(unfiltered[i].Cases))
		}

		for j, tc := range set.Cases {
			// The unfiltered load is the reference: it still has the plan the
			// filter made its decision on.
			ref := unfiltered[i].Cases[j]
			if tc.Note != ref.Note {
				t.Fatalf("expected filtering to preserve order, got %q where %q was", tc.Note, ref.Note)
			}

			uses := ref.WantIR != nil && len(ref.WantIR.Static.BuiltinFuncs) > 0
			if uses != tc.Ignore {
				t.Errorf("%s: ignore is %v, but the plan %s a builtin outside the capabilities",
					tc.Note, tc.Ignore, map[bool]string{true: "uses", false: "does not use"}[uses])
			}

			if !tc.Ignore {
				kept++
				continue
			}
			ignored++

			// The plan is what the consumer cannot run, so it is dropped. The
			// parse assertion is still theirs to run, so it stays.
			if tc.WantIR != nil {
				t.Errorf("%s: expected the plan of an ignored case to be dropped", tc.Note)
			}
			if tc.WantAST != ref.WantAST {
				t.Errorf("%s: expected the parse assertion of an ignored case to survive", tc.Note)
			}
		}
	}

	if ignored == 0 || kept == 0 {
		t.Fatalf("expected the filter to reject some cases and keep others, rejected %d of %d", ignored, ignored+kept)
	}
}

// TestGeneratedFixturesDoNotDrift regenerates the corpus into a scratch copy and
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
			t.Errorf("%s is stale; run `make generate` to update", path)
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func compilePlanSchema(t *testing.T) *jsonschema.Schema {
	t.Helper()

	bs, err := os.ReadFile(filepath.FromSlash(planSchemaPath))
	if err != nil {
		t.Fatalf("read %s: %v", planSchemaPath, err)
	}

	var doc any
	if err := json.Unmarshal(bs, &doc); err != nil {
		t.Fatalf("unmarshal schema: %v", err)
	}

	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("plan.schema.json", doc); err != nil {
		t.Fatalf("add schema resource: %v", err)
	}

	schema, err := compiler.Compile("plan.schema.json")
	if err != nil {
		t.Fatalf("compile schema: %v", err)
	}

	return schema
}
