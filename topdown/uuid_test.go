// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"context"
	"errors"
	"math/rand"
	"testing"

	"github.com/open-policy-agent/opa/ast"
)

func TestUUIDRFC4122SeedingAndCaching(t *testing.T) {

	query := `uuid.rfc4122("x",x); uuid.rfc4122("y", y); uuid.rfc4122("x",x2)`

	q := NewQuery(ast.MustParseBody(query)).WithSeed(rand.New(rand.NewSource(0))).WithCompiler(ast.NewCompiler())

	ctx := context.Background()

	qrs, err := q.Run(ctx)
	if err != nil {
		t.Fatal(err)
	} else if len(qrs) != 1 {
		t.Fatal("expected exactly one result but got:", qrs)
	}

	exp := ast.MustParseTerm(`
		{
			{
				x: "0194fdc2-fa2f-4cc0-81d3-ff12045b73c8",
				x2: "0194fdc2-fa2f-4cc0-81d3-ff12045b73c8",
				y: "6e4ff95f-f662-45ee-a82a-bdf44a2d0b75",
			}
		}
	`)

	result := queryResultSetToTerm(qrs)

	if !result.Equal(exp) {
		t.Fatalf("expected %v but got %v", exp, result)
	}

}

type fakeSeedErrorReader struct{}

func (fakeSeedErrorReader) Read([]byte) (int, error) {
	return 0, errors.New("xxx")
}

func TestUUIDRFC4122SeedError(t *testing.T) {

	query := `uuid.rfc4122("x",x)`

	q := NewQuery(ast.MustParseBody(query)).WithSeed(fakeSeedErrorReader{}).WithCompiler(ast.NewCompiler()).WithStrictBuiltinErrors(true)

	_, err := q.Run(context.Background())

	if topdownErr, ok := err.(*Error); !ok || topdownErr.Code != BuiltinErr {
		t.Fatal("unexpected error (or lack of error):", err)
	}

}

func TestUUIDRFC4122SavingDuringPartialEval(t *testing.T) {

	query := `foo = "x"; uuid.rfc4122(foo,x)`

	q := NewQuery(ast.MustParseBody(query)).WithSeed(rand.New(rand.NewSource(0))).WithCompiler(ast.NewCompiler())

	queries, modules, err := q.PartialRun(context.Background())
	if err != nil {
		t.Fatal(err)
	} else if len(modules) > 0 {
		t.Fatal("expected no support")
	}

	exp := ast.MustParseBody(`uuid.rfc4122("x", x); foo = "x"`)

	if len(queries) != 1 || !queries[0].Equal(exp) {
		t.Fatalf("expected %v but got: %v", exp, queries)
	}
}

func queryResultSetToTerm(qrs QueryResultSet) *ast.Term {
	s := ast.NewSet()
	for i := range qrs {
		bindings := ast.NewObject()
		for k := range qrs[i] {
			bindings.Insert(ast.NewTerm(k), qrs[i][k])
		}
		s.Add(ast.NewTerm(bindings))
	}
	return ast.NewTerm(s)
}
