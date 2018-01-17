// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"testing"

	"github.com/open-policy-agent/opa/ast"
)

func TestBindingsZeroValues(t *testing.T) {
	var unifier *bindings

	// Plugging
	result := unifier.Plug(term("x"))
	exp := term("x")
	if !result.Equal(exp) {
		t.Fatalf("Expected %v but got %v", exp, result)
	}

	// String
	if unifier.String() != "()" {
		t.Fatalf("Expected empty binding list but got: %v", unifier.String())
	}
}

func term(s string) *ast.Term {
	return ast.MustParseTerm(s)
}
