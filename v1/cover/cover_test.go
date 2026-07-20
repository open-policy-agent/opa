// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cover

import (
	"encoding/json"
	"fmt"
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
		module     string
		query      string
		covered    []Range
		notCovered []Range
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

			eval := rego.New(
				rego.ParsedModule(parsedModule),
				rego.Query(tc.query),
				rego.QueryTracer(cover),
			)
			_, err = eval.Eval(t.Context())
			if err != nil {
				t.Fatalf("failed to evaluate: %v", err)
			}

			report := cover.Report(map[string]*ast.Module{"test.rego": parsedModule})
			fr, ok := report.Files["test.rego"]
			if !ok {
				t.Fatal("expected file report for test.rego")
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
		})
	}
}

func TestCoverQueryTracerInterface(t *testing.T) {
	ct := topdown.QueryTracer(New())
	conf := ct.Config()
	expected := topdown.TraceConfig{PlugLocalVars: false}

	if expected != conf {
		t.Fatalf("Expected config: %+v, got %+v", expected, conf)
	}
}
