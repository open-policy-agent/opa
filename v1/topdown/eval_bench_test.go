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
