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

// Benchmark comparing comprehension performance with and without binding size hints
// This simulates "before" vs "after" optimization by measuring allocation patterns
func BenchmarkComprehensionOptimizationComparison(b *testing.B) {
	// Test case with typical comprehension usage patterns
	testCases := []struct {
		name   string
		module string
		query  string
	}{
		{
			name: "SimpleArrayComp",
			module: `package test
import rego.v1

data_array := [1, 2, 3, 4, 5, 6, 7, 8, 9, 10]
result := [y | x := data_array[_]; y := x * 2]`,
			query: "data.test.result",
		},
		{
			name: "SimpleSetComp",
			module: `package test
import rego.v1

data_array := [1, 2, 3, 4, 5, 6, 7, 8, 9, 10]
result := {y | x := data_array[_]; y := x * 2}`,
			query: "data.test.result",
		},
		{
			name: "SimpleObjectComp",
			module: `package test
import rego.v1

data_array := [1, 2, 3, 4, 5, 6, 7, 8, 9, 10]
result := {x: y | x := data_array[_]; y := x * 2}`,
			query: "data.test.result",
		},
		{
			name: "MultiExpressionComp",
			module: `package test
import rego.v1

data_array := [1, 2, 3, 4, 5]
result := [z |
	x := data_array[_]
	y := x * 2
	z := y + 1
	z < 10
]`,
			query: "data.test.result",
		},
		{
			name: "NestedComprehension",
			module: `package test
import rego.v1

outer := [1, 2, 3]
inner := [4, 5, 6]
result := [sum |
	x := outer[_]
	y := inner[_]
	sum := x + y
]`,
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

			// Warmup
			for range 3 {
				q := NewQuery(ast.MustParseBody(tc.query)).
					WithCompiler(compiler).
					WithStore(store)
				_, err := q.Run(ctx)
				if err != nil {
					b.Fatalf("Warmup query failed: %v", err)
				}
			}

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

// Benchmark high-frequency comprehension scenarios (mimicking Regal-style workloads)
func BenchmarkComprehensionHighFrequency(b *testing.B) {
	// Simulate scenarios where comprehensions are evaluated many times
	// (similar to linting operations where rules are checked repeatedly)
	module := `package test
import rego.v1

violations contains msg if {
	some file in input.files
	some line in file.lines
	line.length > 80
	msg := sprintf("Line too long in %s", [file.name])
}

short_lines := {line |
	some file in input.files
	some line in file.lines
	line.length <= 80
}

file_stats := {file.name: len |
	some file in input.files
	len := count(file.lines)
}`

	// Mock input data
	inputData := map[string]any{
		"files": []map[string]any{
			{
				"name": "file1.rego",
				"lines": []map[string]any{
					{"length": 50},
					{"length": 90},
					{"length": 70},
				},
			},
			{
				"name": "file2.rego",
				"lines": []map[string]any{
					{"length": 60},
					{"length": 85},
				},
			},
		},
	}

	mod := ast.MustParseModule(module)
	compiler := ast.NewCompiler()
	compiler.Compile(map[string]*ast.Module{"test": mod})
	if compiler.Failed() {
		b.Fatalf("Compilation failed: %v", compiler.Errors)
	}

	store := inmem.NewFromObject(inputData)
	ctx := context.Background()

	testCases := []struct {
		name  string
		query string
	}{
		{"Violations", "data.test.violations"},
		{"ShortLines", "data.test.short_lines"},
		{"FileStats", "data.test.file_stats"},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			var m1, m2 runtime.MemStats
			runtime.GC()
			runtime.ReadMemStats(&m1)

			b.ResetTimer()

			for b.Loop() {
				q := NewQuery(ast.MustParseBody(tc.query)).
					WithCompiler(compiler).
					WithStore(store).
					WithInput(ast.NewTerm(ast.MustInterfaceToValue(inputData)))

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
