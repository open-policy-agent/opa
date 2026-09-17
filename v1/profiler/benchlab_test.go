// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Curated subset of this package's benchmarks for the nightly benchlab run.
// See build/bench-nightly/set.json.

package profiler

import (
	"fmt"
	"testing"

	"github.com/open-policy-agent/opa/v1/rego"
)

// Mirrors BenchmarkBenchlabCoverBigLocalVar in v1/cover; keep the shapes in
// step so the two tracers stay comparable.
func BenchmarkBenchlabProfilerBigLocalVar(b *testing.B) {
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

			profiler := New()
			b.ResetTimer()

			for b.Loop() {
				if _, err := pq.Eval(ctx, rego.EvalQueryTracer(profiler)); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
