// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cover

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/rego"
	"github.com/open-policy-agent/opa/v1/topdown"
)

func TestCover(t *testing.T) {

	cover := New()

	module := `package test

import data.deadbeef # expect not reported

foo if {
	bar
	p
	not baz
}

bar if {
	a := 1
	b := 2
	a != b
}

baz if {     # expect no exit
	true
	false # expect eval but fail
	true  # expect not covered
}

p if {
	some bar # should not be included in coverage report
	bar = 1
	bar + 1 == 2
}
`

	parsedModule, err := ast.ParseModuleWithOpts("test.rego", module, ast.ParserOptions{AllFutureKeywords: true})
	if err != nil {
		t.Fatal(err)
	}

	eval := rego.New(
		rego.ParsedModule(parsedModule),
		rego.Query("data.test.foo"),
		rego.QueryTracer(cover),
	)

	ctx := t.Context()
	_, err = eval.Eval(ctx)

	if err != nil {
		t.Fatal(err)
	}

	report := cover.Report(map[string]*ast.Module{
		"test.rego": parsedModule,
	})

	fr, ok := report.Files["test.rego"]
	if !ok {
		t.Fatal("Expected file report for test.rego")
	}

	expectedCovered := []Position{
		{Row: 5},                     // foo head
		{Row: 6}, {Row: 7}, {Row: 8}, // foo body
		{Row: 11},                       // bar head
		{Row: 12}, {Row: 13}, {Row: 14}, // bar body
		{Row: 18}, {Row: 19}, // baz body hits
		{Row: 23},            // p head
		{Row: 25}, {Row: 26}, // p body
	}

	expectedNotCovered := []Position{
		{Row: 17}, // baz head
		{Row: 20}, // baz body miss
	}

	for _, exp := range expectedCovered {
		if !fr.IsCovered(exp.Row) {
			t.Errorf("Expected %v to be covered", exp)
		}
	}

	for _, exp := range expectedNotCovered {
		if !fr.IsNotCovered(exp.Row) {
			t.Errorf("Expected %v to NOT be covered", exp)
		}
	}

	if len(expectedCovered) != fr.locCovered() {
		t.Errorf(
			"Expected %d loc to be covered, got %d instead",
			len(expectedCovered),
			fr.locCovered())
	}

	if len(expectedNotCovered) != fr.locNotCovered() {
		t.Errorf(
			"Expected %d loc to not be covered, got %d instead",
			len(expectedNotCovered),
			fr.locNotCovered())
	}

	expectedCoveragePercentage := 100.0 * float64(len(expectedCovered)) / float64(len(expectedCovered)+len(expectedNotCovered))
	if expectedCoveragePercentage != fr.Coverage {
		t.Errorf("Expected coverage %v != %v", expectedCoveragePercentage, fr.Coverage)
	}

	// there's just one file, hence the overall coverage is equal to the
	// one of the only file report we have
	if expectedCoveragePercentage != report.Coverage {
		t.Errorf("Expected report coverage %f != %f",
			expectedCoveragePercentage,
			report.Coverage)
	}

	if t.Failed() {
		bs, err := json.MarshalIndent(fr, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		fmt.Println(string(bs))
	}
}

func TestCoverRangeCases(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		module           string
		query            string
		input            any
		reportKey        string // defaults to "test.rego" if empty
		covered          []Range
		notCovered       []Range
		indexExcluded    []Range
		notIndexExcluded []Range
	}{
		"rule head and value expression on same row counted once": {
			module: `package test

# Both a rule and an expression, but should not be counted twice
foo := 1

allow if { true }
`,
			query: "data.test.allow",
			covered: []Range{
				{Start: Position{Row: 6, Col: 1}, End: Position{Row: 6, Col: 6}}, // allow head
			},
			notCovered: []Range{
				{Start: Position{Row: 4, Col: 1}, End: Position{Row: 4, Col: 9}}, // foo := 1 head
			},
		},
		"inline rule head not covered": {
			module: `package test

foo if false

test_foo if {
	not foo
}
`,
			query: "data.test.test_foo",
			covered: []Range{
				{Start: Position{Row: 3, Col: 8}, End: Position{Row: 3, Col: 13}}, // false expr
			},
			notCovered: []Range{
				{Start: Position{Row: 3, Col: 1}, End: Position{Row: 3, Col: 4}}, // foo head
			},
		},
		"index-excluded rule body is not covered": {
			module: `package test

allow if {
	input.action == "read"
}

allow if {
	input.action in {"delete", "update"}
}
`,
			query: "data.test.allow",
			input: map[string]any{"action": "write"},
			notCovered: []Range{
				{Start: Position{Row: 3, Col: 1}, End: Position{Row: 3, Col: 6}},
				{Start: Position{Row: 4, Col: 2}, End: Position{Row: 4, Col: 24}},
				{Start: Position{Row: 7, Col: 1}, End: Position{Row: 7, Col: 6}},
				{Start: Position{Row: 8, Col: 2}, End: Position{Row: 8, Col: 38}},
			},
			indexExcluded: []Range{
				{Start: Position{Row: 3, Col: 1}, End: Position{Row: 3, Col: 6}},
				{Start: Position{Row: 4, Col: 2}, End: Position{Row: 4, Col: 24}},
				{Start: Position{Row: 7, Col: 1}, End: Position{Row: 7, Col: 6}},
				{Start: Position{Row: 8, Col: 2}, End: Position{Row: 8, Col: 38}},
			},
		},
		"index exclusions inside a with scope": {
			// with pushes a new eval frame and exclusions there must still be reported
			module: `package test

allow if {
	input.action == "read"
}

allow if {
	input.action in {"delete", "update"}
}

test_allow_write if {
	allow with input as {"action": "write"}
}
`,
			query: "data.test.test_allow_write",
			covered: []Range{
				{Start: Position{Row: 12, Col: 2}, End: Position{Row: 12, Col: 41}},
			},
			notCovered: []Range{
				{Start: Position{Row: 3, Col: 1}, End: Position{Row: 3, Col: 6}},
				{Start: Position{Row: 4, Col: 2}, End: Position{Row: 4, Col: 24}},
				{Start: Position{Row: 7, Col: 1}, End: Position{Row: 7, Col: 6}},
				{Start: Position{Row: 8, Col: 2}, End: Position{Row: 8, Col: 38}},
			},
			indexExcluded: []Range{
				{Start: Position{Row: 3, Col: 1}, End: Position{Row: 3, Col: 6}},
				{Start: Position{Row: 4, Col: 2}, End: Position{Row: 4, Col: 24}},
				{Start: Position{Row: 7, Col: 1}, End: Position{Row: 7, Col: 6}},
				{Start: Position{Row: 8, Col: 2}, End: Position{Row: 8, Col: 38}},
			},
		},
		"index-excluded rule body is not covered, bundle key mismatch": {
			// Lookup must key on loc.File, not the modules map key, since bundle
			// modules key differently from their parse-time Location.File.
			module: `package test

allow if {
	input.action == "read"
}

allow if {
	input.action in {"delete", "update"}
}
`,
			query:     "data.test.allow",
			input:     map[string]any{"action": "write"},
			reportKey: "bundle/test.rego",
			indexExcluded: []Range{
				{Start: Position{Row: 3, Col: 1}, End: Position{Row: 3, Col: 6}},
				{Start: Position{Row: 4, Col: 2}, End: Position{Row: 4, Col: 24}},
				{Start: Position{Row: 7, Col: 1}, End: Position{Row: 7, Col: 6}},
				{Start: Position{Row: 8, Col: 2}, End: Position{Row: 8, Col: 38}},
			},
		},
		"index-excluded root rule: else branch also excluded": {
			module: `package test

allow if {
	input.action == "read"
} else if {
	input.action == "admin"
}

allow if {
	input.action in {"delete", "update"}
}
`,
			query: "data.test.allow",
			input: map[string]any{"action": "write"},
			notCovered: []Range{
				{Start: Position{Row: 3, Col: 1}, End: Position{Row: 3, Col: 6}},    // allow head (root)
				{Start: Position{Row: 4, Col: 2}, End: Position{Row: 4, Col: 24}},   // input.action == "read"
				{Start: Position{Row: 6, Col: 2}, End: Position{Row: 6, Col: 24}},   // input.action == "admin" (else body)
				{Start: Position{Row: 9, Col: 1}, End: Position{Row: 9, Col: 6}},    // allow head (second)
				{Start: Position{Row: 10, Col: 2}, End: Position{Row: 10, Col: 38}}, // input.action in ...
			},
			indexExcluded: []Range{
				{Start: Position{Row: 3, Col: 1}, End: Position{Row: 3, Col: 6}},    // allow head (root)
				{Start: Position{Row: 4, Col: 2}, End: Position{Row: 4, Col: 24}},   // input.action == "read"
				{Start: Position{Row: 6, Col: 2}, End: Position{Row: 6, Col: 24}},   // input.action == "admin" (else body)
				{Start: Position{Row: 9, Col: 1}, End: Position{Row: 9, Col: 6}},    // allow head (second)
				{Start: Position{Row: 10, Col: 2}, End: Position{Row: 10, Col: 38}}, // input.action in ...
			},
		},
		"else rule promoted to root by indexer: short-circuited else body is not falsely index-excluded": {
			// Lookup can promote the else-rule into the "root" slot; input.foo is
			// merely short-circuited by 1 == 2, not index-excluded.
			module: `package test

allow if {
	input.undef
} else if {
	1 == 2
	input.foo
}
`,
			query: "data.test.allow",
			input: map[string]any{"foo": true},
			covered: []Range{
				{Start: Position{Row: 6, Col: 2}, End: Position{Row: 6, Col: 8}}, // 1 == 2
			},
			notCovered: []Range{
				{Start: Position{Row: 3, Col: 1}, End: Position{Row: 3, Col: 6}},  // allow head (primary)
				{Start: Position{Row: 4, Col: 2}, End: Position{Row: 4, Col: 13}}, // input.undef
				{Start: Position{Row: 5, Col: 3}, End: Position{Row: 5, Col: 7}},  // else head
				{Start: Position{Row: 7, Col: 2}, End: Position{Row: 7, Col: 11}}, // input.foo (short-circuited, not index-excluded)
			},
			indexExcluded: []Range{
				{Start: Position{Row: 3, Col: 1}, End: Position{Row: 3, Col: 6}},  // allow head (primary)
				{Start: Position{Row: 4, Col: 2}, End: Position{Row: 4, Col: 13}}, // input.undef
			},
			notIndexExcluded: []Range{
				{Start: Position{Row: 5, Col: 3}, End: Position{Row: 5, Col: 7}},  // else head: reached, just never exits
				{Start: Position{Row: 7, Col: 2}, End: Position{Row: 7, Col: 11}}, // input.foo: short-circuited, NOT index-excluded
			},
		},
		"negated expression rule is not covered without indexer exclusion": {
			module: `package test

allow if {
	not input.blocked
}

other if {
	true
}
`,
			query: "data.test.other",
			covered: []Range{
				{Start: Position{Row: 7, Col: 1}, End: Position{Row: 7, Col: 6}},
				{Start: Position{Row: 8, Col: 2}, End: Position{Row: 8, Col: 6}},
			},
			notCovered: []Range{
				{Start: Position{Row: 3, Col: 1}, End: Position{Row: 3, Col: 6}},
				{Start: Position{Row: 4, Col: 2}, End: Position{Row: 4, Col: 19}},
			},
		},
		"semicolon-separated expressions short-circuit": {
			module: `package test

foo if {
	true; true; false; false
}

test_foo if {
	not foo
}
`,
			query: "data.test.test_foo",
			// Row 4: `\ttrue; true; false; false`
			covered: []Range{
				{Start: Position{Row: 4, Col: 2}, End: Position{Row: 4, Col: 6}},   // true
				{Start: Position{Row: 4, Col: 8}, End: Position{Row: 4, Col: 12}},  // true
				{Start: Position{Row: 4, Col: 14}, End: Position{Row: 4, Col: 19}}, // false (caused failure)
			},
			notCovered: []Range{
				{Start: Position{Row: 4, Col: 21}, End: Position{Row: 4, Col: 26}}, // false (never evaluated)
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cover := New()

			parsedModule, err := ast.ParseModule("test.rego", tc.module)
			if err != nil {
				t.Fatalf("failed to parse module: %v", err)
			}

			args := []func(*rego.Rego){
				rego.ParsedModule(parsedModule),
				rego.Query(tc.query),
				rego.QueryTracer(cover),
			}
			if tc.input != nil {
				args = append(args, rego.Input(tc.input))
			}

			eval := rego.New(args...)
			_, err = eval.Eval(t.Context())
			if err != nil {
				t.Fatalf("failed to evaluate: %v", err)
			}

			reportKey := tc.reportKey
			if reportKey == "" {
				reportKey = "test.rego"
			}

			report := cover.Report(map[string]*ast.Module{reportKey: parsedModule})
			fr, ok := report.Files[reportKey]
			if !ok {
				t.Fatalf("expected file report for %q", reportKey)
			}

			for _, r := range tc.covered {
				if !fr.isRangeCovered(r) {
					t.Errorf("expected range %v to be covered", r)
				}
			}

			for _, r := range tc.notCovered {
				if !fr.isRangeNotCovered(r) {
					t.Errorf("expected range %v to be not covered", r)
				}
			}

			for _, r := range tc.indexExcluded {
				if notCoveredKind(t, fr, r) != KindIndexExcluded {
					t.Errorf("expected range %v to be indexer excluded", r)
				}
				// index-excluded ranges must also appear in not_covered (backward compat)
				if !fr.isRangeNotCovered(r) {
					t.Errorf("expected index-excluded range %v to also be in not_covered", r)
				}
			}

			for _, r := range tc.notIndexExcluded {
				if kind := notCoveredKind(t, fr, r); kind != "" {
					t.Errorf("expected range %v to NOT be indexer excluded, got kind %q", r, kind)
				}
			}
		})
	}
}

func TestCoverQueryTracerInterface(t *testing.T) {
	ct := topdown.QueryTracer(New())
	conf := ct.Config()
	expected := topdown.TraceConfig{
		PlugLocalVars: false,
		ReportOps:     []topdown.Op{topdown.IndexExcludedOp},
	}

	if !reflect.DeepEqual(expected, conf) {
		t.Fatalf("Expected config: %+v, got %+v", expected, conf)
	}
}

// notCoveredKind returns the Kind of the not-covered range in fr containing
// r, or "" if r is not found among fr.NotCovered.
func notCoveredKind(t *testing.T, fr *FileReport, r Range) Kind {
	t.Helper()
	for _, candidate := range fr.NotCovered {
		if candidate.contains(r) {
			return candidate.Kind
		}
	}
	return ""
}
