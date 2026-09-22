// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cases

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/open-policy-agent/opa/v1/ast"
)

const planSchemaPath = "../../v1/ir/plan.schema.json"

// TestWithIR checks that planning runs to completion, not that any individual plan is
// correct: no plan is committed, so there is nothing to compare one against. What is
// asserted is that every plan conforms to the published schema, that it has a plan per
// entrypoint, and that planning twice gives the same answer.
func TestWithIR(t *testing.T) {
	schema := compilePlanSchema(t)

	sets, err := LoadCompilerTestCases(WithIR())
	if err != nil {
		t.Fatal(err)
	}

	again, err := LoadCompilerTestCases(WithIR())
	if err != nil {
		t.Fatal(err)
	}

	var planned, explained, queries int

	for i, set := range sets {
		for j, tc := range set.Cases {
			assertsForm := tc.Transform() || queryCompiles(tc.TestCase)

			if !assertsForm {
				// A case asserting diagnostics has no compiled form to plan, and says
				// nothing about why.
				if tc.WantIR != nil || tc.IRError != "" {
					t.Errorf("%s: expected no plan and no reason for a case asserting diagnostics", tc.Note)
				}
				continue
			}

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

			// A query case is planned for its query, which has no ref to be named after.
			if tc.QueryCase() {
				queries++
				if want := []string{QueryEntryPoint}; !slices.Equal(tc.EntryPoints, want) {
					t.Errorf("%s: expected the entrypoints %v, got %v", tc.Note, want, tc.EntryPoints)
				}
			}

			names := make([]string, len(tc.WantIR.Plans.Plans))
			for k, plan := range tc.WantIR.Plans.Plans {
				names[k] = plan.Name
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

	if planned == 0 || queries == 0 || explained == 0 {
		t.Fatalf("expected planning to be exercised on every kind of case, planned %d of which %d queries, %d explained",
			planned, queries, explained)
	}

	t.Logf("%d cases planned, %d of them queries, %d could not be planned", planned, queries, explained)
}

// TestLoadCompilerTestCasesGeneratesNothingByDefault pins that the plan is generated on
// request only, so a consumer reading the committed corpus never pays for it.
func TestLoadCompilerTestCasesGeneratesNothingByDefault(t *testing.T) {
	sets, err := LoadCompilerTestCases()
	if err != nil {
		t.Fatal(err)
	}

	for _, set := range sets {
		for _, tc := range set.Cases {
			if tc.WantIR != nil || tc.IRError != "" || len(tc.EntryPoints) > 0 {
				t.Errorf("%s: expected no IR without WithIR", tc.Note)
			}
		}
	}
}

func TestCapabilitiesFilter(t *testing.T) {
	capabilities := &ast.Capabilities{Builtins: []*ast.Builtin{ast.BuiltinMap["plus"]}}

	unfiltered, err := LoadCompilerTestCases(WithIR())
	if err != nil {
		t.Fatal(err)
	}

	sets, err := LoadCompilerTestCasesFiltered([]Filters{CapabilitiesFilter(capabilities)}, WithIR())
	if err != nil {
		t.Fatal(err)
	}

	var rejected, kept int

	for i, set := range sets {
		for j, tc := range set.Cases {
			ref := unfiltered[i].Cases[j]
			if tc.Note != ref.Note {
				t.Fatalf("expected filtering to preserve order, got %q where %q was", tc.Note, ref.Note)
			}

			if !tc.Ignore {
				kept++
				continue
			}
			rejected++

			// A rejected case keeps everything the corpus committed; only the plan it
			// cannot execute goes.
			if tc.WantIR != nil {
				t.Errorf("%s: expected the plan to be dropped", tc.Note)
			}
			if len(tc.Want) != len(ref.Want) || len(tc.WantErrors) != len(ref.WantErrors) {
				t.Errorf("%s: expected the assertions of the case to survive filtering", tc.Note)
			}
		}
	}

	if rejected == 0 || kept == 0 {
		t.Errorf("expected a one-builtin capability set to reject some plans and keep others, rejected %d of %d",
			rejected, rejected+kept)
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
