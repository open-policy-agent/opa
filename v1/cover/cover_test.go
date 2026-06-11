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

func TestCoverNoDuplicates(t *testing.T) {

	cover := New()

	module := `package test

# Both a rule and an expression, but should not be counted twice
foo := 1

allow if { true }
`

	parsedModule, err := ast.ParseModuleWithOpts("test.rego", module, ast.ParserOptions{AllFutureKeywords: true})
	if err != nil {
		t.Fatal(err)
	}

	eval := rego.New(
		rego.ParsedModule(parsedModule),
		rego.Query("data.test.allow"),
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
		{Row: 6}, // allow
	}

	expectedNotCovered := []Position{
		{Row: 4}, // foo
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
		t.Errorf("Expected coverage %f != %f", expectedCoveragePercentage, fr.Coverage)
	}

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

func TestCoverInlineRuleHeadNotCovered(t *testing.T) {
	t.Parallel()

	cover := New()

	module := `package test

foo if false

test_foo if {
	not foo
}
`

	parsedModule, err := ast.ParseModule("test.rego", module)
	if err != nil {
		t.Fatalf("failed to parse module: %v", err)
	}

	eval := rego.New(
		rego.ParsedModule(parsedModule),
		rego.Query("data.test.test_foo"),
		rego.QueryTracer(cover),
	)

	ctx := t.Context()
	_, err = eval.Eval(ctx)
	if err != nil {
		t.Fatalf("failed to evaluate: %v", err)
	}

	report := cover.Report(map[string]*ast.Module{"test.rego": parsedModule})

	fr, ok := report.Files["test.rego"]
	if !ok {
		t.Fatal("Expected file report for test.rego")
	}

	fooHead := Range{Start: Position{Row: 3, Col: 1}, End: Position{Row: 3, Col: 4}}
	if !fr.isRangeNotCovered(fooHead) {
		t.Errorf("expected foo head %v to be not covered", fooHead)
	}

	falseExpr := Range{Start: Position{Row: 3, Col: 8}, End: Position{Row: 3, Col: 13}}
	if !fr.isRangeCovered(falseExpr) {
		t.Errorf("expected false body %v to be covered", falseExpr)
	}
}

func TestFileReportIsRangeCovered(t *testing.T) {
	t.Parallel()

	fr := &FileReport{
		Covered: []Range{
			{Start: Position{Row: 3, Col: 8}, End: Position{Row: 3, Col: 13}},
		},
		NotCovered: []Range{
			{Start: Position{Row: 3, Col: 1}, End: Position{Row: 3, Col: 4}},
		},
	}

	covered := Range{Start: Position{Row: 3, Col: 8}, End: Position{Row: 3, Col: 13}}
	if !fr.isRangeCovered(covered) {
		t.Errorf("expected %v to be covered", covered)
	}

	subCovered := Range{Start: Position{Row: 3, Col: 9}, End: Position{Row: 3, Col: 11}}
	if !fr.isRangeCovered(subCovered) {
		t.Errorf("expected sub-range %v to be covered", subCovered)
	}

	notCovered := Range{Start: Position{Row: 3, Col: 1}, End: Position{Row: 3, Col: 4}}
	if !fr.isRangeNotCovered(notCovered) {
		t.Errorf("expected %v to be not covered", notCovered)
	}

	absent := Range{Start: Position{Row: 5, Col: 1}, End: Position{Row: 5, Col: 4}}
	if fr.isRangeCovered(absent) {
		t.Errorf("expected %v to not be covered", absent)
	}
	if fr.isRangeNotCovered(absent) {
		t.Errorf("expected %v to not be in not_covered", absent)
	}
}

func TestCoverTraceConfig(t *testing.T) {
	ct := topdown.QueryTracer(New())
	conf := ct.Config()

	expected := topdown.TraceConfig{
		PlugLocalVars: false,
	}

	if !reflect.DeepEqual(expected, conf) {
		t.Fatalf("Expected config: %+v, got %+v", expected, conf)
	}
}
