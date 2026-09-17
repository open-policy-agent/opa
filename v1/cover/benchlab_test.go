// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Curated subset of this package's benchmarks for the nightly benchlab run.
// See build/bench-nightly/set.json.

package cover

import (
	"fmt"
	"testing"

	"github.com/open-policy-agent/opa/v1/rego"
)

// The same three shapes are curated in v1/profiler, so the cost of the two
// tracers stays directly comparable.
func BenchmarkBenchlabCoverBigLocalVar(b *testing.B) {
	for _, c := range []struct{ vars, iterations int }{
		{1, 1},
		{10, 100},
		{10, 1000},
	} {
		b.Run(fmt.Sprintf("%dVars%dIterations", c.vars, c.iterations), func(b *testing.B) {
			ctx := b.Context()
			pq, err := rego.New(
				rego.Module("test.rego", generateModule(c.vars, c.iterations)),
				rego.Query("data.test.p"),
			).PrepareForEval(ctx)
			if err != nil {
				b.Fatal(err)
			}

			cover := New()
			b.ResetTimer()

			for b.Loop() {
				if _, err := pq.Eval(ctx, rego.EvalQueryTracer(cover)); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
