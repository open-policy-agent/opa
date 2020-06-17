// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cover

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/rego"
)

func BenchmarkCoverBigLocalVar(b *testing.B) {
	iterations := []int{1, 100, 1000}
	vars := []int{1, 10}

	for _, iterationCount := range iterations {
		for _, varCount := range vars {
			name := fmt.Sprintf("%dVars%dIterations", varCount, iterationCount)
			b.Run(name, func(b *testing.B) {
				cover := New()
				module := generateModule(varCount, iterationCount)

				_, err := ast.ParseModule("test.rego", module)
				if err != nil {
					b.Fatal(err)
				}

				ctx := context.Background()

				pq, err := rego.New(
					rego.Module("test.rego", module),
					rego.Query("data.test.p"),
				).PrepareForEval(ctx)

				if err != nil {
					b.Fatal(err)
				}

				b.ResetTimer()

				for i := 0; i < b.N; i++ {
					b.StartTimer()
					_, err = pq.Eval(ctx, rego.EvalQueryTracer(cover))
					b.StopTimer()

					if err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}

func generateModule(numVars int, dataSize int) string {
	sb := strings.Builder{}
	sb.WriteString(`package test

p {
	x := a
	v := x[i]
`)
	for i := 0; i < numVars; i++ {
		sb.WriteString(fmt.Sprintf("\tv%d := x[i+%d]\n", i, i))
	}
	sb.WriteString("\tfalse\n}\n")
	sb.WriteString("\na := [\n")
	for i := 0; i < dataSize; i++ {
		sb.WriteString(fmt.Sprintf("\t%d,\n", i))
	}
	sb.WriteString("]\n")
	return sb.String()
}
