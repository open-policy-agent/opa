// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.
package topdown

import (
	"context"
	"testing"

	"github.com/open-policy-agent/opa/ast"
)

func TestOPARuntime(t *testing.T) {

	ctx := context.Background()
	q := NewQuery(ast.MustParseBody("opa.runtime(x)")) // no runtime info
	rs, err := q.Run(ctx)
	if err != nil {
		t.Fatal(err)
	} else if len(rs) != 1 {
		t.Fatal("Expected result set to contain exactly one result")
	}

	term := rs[0][ast.Var("x")]
	exp := ast.ObjectTerm()

	if ast.Compare(term, exp) != 0 {
		t.Fatalf("Expected %v but got %v", exp, term)
	}

	q = NewQuery(ast.MustParseBody("opa.runtime(x)")).WithRuntime(ast.MustParseTerm(`{"config": {"a": 1}}`))
	rs, err = q.Run(ctx)
	if err != nil {
		t.Fatal(err)
	} else if len(rs) != 1 {
		t.Fatal("Expected result set to contain exactly one result")
	}

	term = rs[0][ast.Var("x")]
	exp = ast.MustParseTerm(`{"config": {"a": 1}}`)

	if ast.Compare(term, exp) != 0 {
		t.Fatalf("Expected %v but got %v", exp, term)
	}

}
