// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"context"
	"runtime"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/storage/inmem"
)

// Benchmark comprehensions with different variable counts
func BenchmarkComprehensionBindings(b *testing.B) {
	testCases := []struct {
		name   string
		module string
		query  string
	}{
		{
			name: "ArrayComp_1Var",
			module: `package test
arr := [1, 2, 3, 4, 5]
result := [x | x := arr[_]]`,
			query: "data.test.result",
		},
		{
			name: "ArrayComp_2Vars",
			module: `package test
arr := [1, 2, 3, 4, 5]
result := [y | x := arr[_]; y := x * 2]`,
			query: "data.test.result",
		},
		{
			name: "ArrayComp_3Vars",
			module: `package test
arr := [1, 2, 3, 4, 5]
result := [z | x := arr[_]; y := x * 2; z := y + 1]`,
			query: "data.test.result",
		},
		{
			name: "SetComp_1Var",
			module: `package test
arr := [1, 2, 3, 4, 5]
result := {x | x := arr[_]}`,
			query: "data.test.result",
		},
		{
			name: "SetComp_2Vars",
			module: `package test
arr := [1, 2, 3, 4, 5]
result := {y | x := arr[_]; y := x * 2}`,
			query: "data.test.result",
		},
		{
			name: "ObjectComp_2Vars",
			module: `package test
arr := [1, 2, 3, 4, 5]
result := {x: y | x := arr[_]; y := x * 2}`,
			query: "data.test.result",
		},
		{
			name: "ObjectComp_3Vars",
			module: `package test
arr := [1, 2, 3, 4, 5]
result := {x: z | x := arr[_]; y := x * 2; z := y + 1}`,
			query: "data.test.result",
		},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			mod := ast.MustParseModule(tc.module)
			compiler := ast.NewCompiler()
			compiler.Compile(map[string]*ast.Module{"test": mod})
			if compiler.Failed() {
				b.Fatalf("Compilation failed: %v", compiler.Errors)
			}

			store := inmem.New()
			ctx := context.Background()

			var m1, m2 runtime.MemStats
			runtime.GC()
			runtime.ReadMemStats(&m1)

			b.ResetTimer()

			for b.Loop() {
				q := NewQuery(ast.MustParseBody(tc.query)).
					WithCompiler(compiler).
					WithStore(store)

				_, err := q.Run(ctx)
				if err != nil {
					b.Fatalf("Query failed: %v", err)
				}
			}

			b.StopTimer()

			runtime.GC()
			runtime.ReadMemStats(&m2)

			allocPerOp := (m2.TotalAlloc - m1.TotalAlloc) / uint64(b.N)
			mallocsPerOp := (m2.Mallocs - m1.Mallocs) / uint64(b.N)

			b.ReportMetric(float64(allocPerOp), "B/op")
			b.ReportMetric(float64(mallocsPerOp), "allocs/op")
		})
	}
}

// Benchmark large comprehension iterations
func BenchmarkComprehensionLargeIteration(b *testing.B) {
	testCases := []struct {
		name   string
		module string
		query  string
	}{
		{
			name: "ArrayComp_20Elements",
			module: `package test
import rego.v1

numbers := [1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20]
result := [y | x := numbers[_]; y := x * 2]`,
			query: "data.test.result",
		},
		{
			name: "SetComp_20Elements",
			module: `package test
import rego.v1

numbers := [1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20]
result := {y | x := numbers[_]; y := x * 2}`,
			query: "data.test.result",
		},
		{
			name: "NestedComp",
			module: `package test
arr := [1, 2, 3, 4, 5]
result := [z | x := arr[_]; y := [a | a := arr[_]; a > x][_]; z := x + y]`,
			query: "data.test.result",
		},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			mod := ast.MustParseModule(tc.module)
			compiler := ast.NewCompiler()
			compiler.Compile(map[string]*ast.Module{"test": mod})
			if compiler.Failed() {
				b.Fatalf("Compilation failed: %v", compiler.Errors)
			}

			store := inmem.New()
			ctx := context.Background()

			var m1, m2 runtime.MemStats
			runtime.GC()
			runtime.ReadMemStats(&m1)

			b.ResetTimer()

			for b.Loop() {
				q := NewQuery(ast.MustParseBody(tc.query)).
					WithCompiler(compiler).
					WithStore(store)

				_, err := q.Run(ctx)
				if err != nil {
					b.Fatalf("Query failed: %v", err)
				}
			}

			b.StopTimer()

			runtime.GC()
			runtime.ReadMemStats(&m2)

			allocPerOp := (m2.TotalAlloc - m1.TotalAlloc) / uint64(b.N)
			mallocsPerOp := (m2.Mallocs - m1.Mallocs) / uint64(b.N)

			b.ReportMetric(float64(allocPerOp), "B/op")
			b.ReportMetric(float64(mallocsPerOp), "allocs/op")
		})
	}
}

// Test to verify EstimateBindingCount returns body length
func TestComprehensionBindingEstimate(t *testing.T) {
	testCases := []struct {
		name     string
		module   string
		expected int // Expected is body length
	}{
		{
			name: "ArrayComp_1Expr",
			module: `package test
result := [x | x := [1, 2, 3][_]]`,
			expected: 1, // 1 expression in body
		},
		{
			name: "ArrayComp_2Exprs",
			module: `package test
result := [y | x := [1, 2, 3][_]; y := x * 2]`,
			expected: 2, // 2 expressions in body
		},
		{
			name: "ArrayComp_3Exprs",
			module: `package test
result := [z | x := [1, 2, 3][_]; y := x * 2; z := y + 1]`,
			expected: 3, // 3 expressions in body
		},
		{
			name: "SetComp_2Exprs",
			module: `package test
result := {y | x := [1, 2, 3][_]; y := x * 2}`,
			expected: 2,
		},
		{
			name: "ObjectComp_3Exprs",
			module: `package test
result := {x: z | x := [1, 2, 3][_]; y := x * 2; z := y + 1}`,
			expected: 3,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mod := ast.MustParseModule(tc.module)

			// Find the comprehension in the module
			var count int
			ast.WalkRules(mod, func(r *ast.Rule) bool {
				ast.WalkTerms(r, func(term *ast.Term) bool {
					switch c := term.Value.(type) {
					case *ast.ArrayComprehension:
						count = ast.EstimateBodyBindingCount(c.Body)
						return true
					case *ast.SetComprehension:
						count = ast.EstimateBodyBindingCount(c.Body)
						return true
					case *ast.ObjectComprehension:
						count = ast.EstimateBodyBindingCount(c.Body)
						return true
					}
					return false
				})
				return false
			})

			if count != tc.expected {
				t.Errorf("Expected %d bindings, got %d", tc.expected, count)
			}
		})
	}
}
