package topdown_test

import (
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/topdown"
)

func BenchmarkBiunifyArrays(b *testing.B) {
	q := topdown.NewQuery(ast.MustParseBody("[1,x,3] = [y,5,6]"))

	ctx := b.Context()

	for b.Loop() {
		if _, err := q.Run(ctx); err != nil {
			b.Fatalf("Query failed: %v", err)
		}
	}
}

func BenchmarkEvalNot(b *testing.B) {
	q := topdown.NewQuery(ast.MustParseBody("x = 2; not x = 1; not x = 3; not x = 4"))

	ctx := b.Context()

	for b.Loop() {
		if _, err := q.Run(ctx); err != nil {
			b.Fatalf("Query failed: %v", err)
		}
	}
}

func BenchmarkEvalBuiltinContext(b *testing.B) {
	q := topdown.NewQuery(ast.MustParseBody(`regex.match("^a", "abc"); concat(",", ["a", "b"], x); glob.match("a*", [], "abc")`))

	ctx := b.Context()

	for b.Loop() {
		if _, err := q.Run(ctx); err != nil {
			b.Fatalf("Query failed: %v", err)
		}
	}
}
