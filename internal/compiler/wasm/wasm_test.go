// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package wasm

import (
	"testing"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/internal/planner"
)

func TestCompilerHelloWorld(t *testing.T) {

	policy, err := planner.New().
		WithQueries([]ast.Body{ast.MustParseBody(`input.foo = 1`)}).
		Plan()

	if err != nil {
		t.Fatal(err)
	}

	c := New().WithPolicy(policy)
	_, err = c.Compile()
	if err != nil {
		t.Fatal(err)
	}
}
