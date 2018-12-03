// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cover

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/rego"
)

func TestCover(t *testing.T) {

	cover := New()

	module := `package test

import data.deadbeef # expect not reported

foo {
	bar
	not baz
}

bar {
	a := 1
	b := 2
	a != b
}

baz {     # expect no exit
	true
	false # expect eval but fail
	true  # expect not covered
}`

	parsedModule, err := ast.ParseModule("test.rego", module)
	if err != nil {
		t.Fatal(err)
	}

	eval := rego.New(
		rego.Module("test.rego", module),
		rego.Query("data.test.foo"),
		rego.Tracer(cover),
	)

	ctx := context.Background()
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
		{5},      // foo head
		{6}, {7}, // foo body
		{10},             // bar head
		{11}, {12}, {13}, // bar body
		{17}, {18}, // baz body hits
	}

	expectedNotCovered := []Position{
		{16}, // baz head
		{19}, // baz body miss
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

	expectedCoveragePercentage := round(100.0*float64(len(expectedCovered))/float64(len(expectedCovered)+len(expectedNotCovered)), 2)
	if expectedCoveragePercentage != fr.Coverage {
		t.Errorf("Expected coverage %f != %f", expectedCoveragePercentage, fr.Coverage)
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
