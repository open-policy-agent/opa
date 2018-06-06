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
		{17}, // baz body hits
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

	if t.Failed() {
		bs, err := json.MarshalIndent(fr, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		fmt.Println(string(bs))
	}

}
