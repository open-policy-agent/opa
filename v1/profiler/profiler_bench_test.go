// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package profiler

import (
	"fmt"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/rego"
)

func BenchmarkProfilerBigLocalVar(b *testing.B) {
	iterations := []int{1, 100, 1000}
	vars := []int{1, 10}

	for _, iterationCount := range iterations {
		for _, varCount := range vars {
			name := fmt.Sprintf("%dVars%dIterations", varCount, iterationCount)
			b.Run(name, func(b *testing.B) {
				profiler := New()
				module := generateModule(varCount, iterationCount)

				if _, err := ast.ParseModule("test.rego", module); err != nil {
					b.Fatal(err)
				}

				ctx := b.Context()

				pq, err := rego.New(
					rego.Module("test.rego", module),
					rego.Query("data.test.p"),
				).PrepareForEval(ctx)

				if err != nil {
					b.Fatal(err)
				}

				b.ResetTimer()

				for b.Loop() {
					if _, err = pq.Eval(ctx, rego.EvalQueryTracer(profiler)); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}

func generateModule(numVars int, dataSize int) string {
	sb := &strings.Builder{}
	sb.WriteString(`package test

p if {
	x := a
	v := x[i]
`)
	for i := range numVars {
		fmt.Fprintf(sb, "\tv%d := x[i+%d]\n", i, i)
	}
	sb.WriteString("\tfalse\n}\n")
	sb.WriteString("\na := [\n")
	for i := range dataSize {
		fmt.Fprintf(sb, "\t%d,\n", i)
	}
	sb.WriteString("]\n")
	return sb.String()
}
