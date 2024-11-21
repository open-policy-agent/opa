package ast

import (
	"fmt"
	"testing"
)

func BenchmarkRewriteDynamics(b *testing.B) {

	// The choice of query to use is somewhat arbitrary. This query is
	// representative of the ones that result from partial evaluation on IAM
	// data models (e.g., a triple glob match on subject/action/resource.)
	body := MustParseBody(`
		glob.match("a:*", [":"], input.abcdef.x12345);
		glob.match("a:*", [":"], input.abcdef.y12345);
		glob.match("a:*", [":"], input.abcdef.z12345)
	`)
	sizes := []int{1, 10, 100, 1000, 10000, 100000}
	queries := makeQueriesForRewriteDynamicsBenchmark(sizes, body)

	for i := range sizes {
		b.Run(fmt.Sprint(sizes[i]), func(b *testing.B) {
			factory := newEqualityFactory(newLocalVarGenerator("q", nil))
			b.ResetTimer()
			for n := 0; n < b.N; n++ {
				for _, body := range queries[i] {
					rewriteDynamics(factory, body)
				}
			}
		})
	}

}

func makeQueriesForRewriteDynamicsBenchmark(sizes []int, body Body) [][]Body {

	queries := make([][]Body, len(sizes))

	for i := range queries {
		queries[i] = make([]Body, sizes[i])
		for j := 0; j < sizes[i]; j++ {
			queries[i][j] = body.Copy()
		}
	}

	return queries
}
