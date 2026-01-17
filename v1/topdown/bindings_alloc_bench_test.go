// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"fmt"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/storage"
	inmem "github.com/open-policy-agent/opa/v1/storage/inmem/test"
)

// BenchmarkBindingsAllocation benchmarks memory allocation for bindings with different sizes.
// This directly tests the optimization from issue #7266.
func BenchmarkBindingsAllocation(b *testing.B) {
	tests := []struct {
		name     string
		bindings int
	}{
		{"1_binding", 1},
		{"2_bindings", 2},
		{"3_bindings", 3},
		{"5_bindings", 5},
		{"10_bindings", 10},
		{"16_bindings", 16},
		{"20_bindings", 20},
		{"50_bindings", 50},
	}

	for _, tt := range tests {
		b.Run(tt.name+"_without_hint", func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				bi := newBindings(0, nil)
				for j := range tt.bindings {
					key := ast.VarTerm(fmt.Sprintf("x%d", j))
					val := ast.IntNumberTerm(j)
					bi.bind(key, val, nil, &undo{})
				}
			}
		})

		b.Run(tt.name+"_with_hint", func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				bi := newBindingsWithSize(0, nil, tt.bindings)
				for j := range tt.bindings {
					key := ast.VarTerm(fmt.Sprintf("x%d", j))
					val := ast.IntNumberTerm(j)
					bi.bind(key, val, nil, &undo{})
				}
			}
		})
	}
}

// BenchmarkCustomFunctionInHotPath reproduces the scenario from issue #7266.
// This tests calling custom functions thousands of times in a hot path (like inside walk()).
func BenchmarkCustomFunctionInHotPath(b *testing.B) {
	ctx := b.Context()

	// Create a simple custom function with 2 arguments
	module := `package test

	is_ref(x, y) if {
		x == y
	}

	result if {
		is_ref(1, 1)
	}
	`

	compiler := ast.MustCompileModules(map[string]string{
		"test.rego": module,
	})

	store := inmem.NewFromObject(map[string]any{})

	query := ast.MustParseBody(`data.test.result`)

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		err := storage.Txn(ctx, store, storage.TransactionParams{}, func(txn storage.Transaction) error {
			q := NewQuery(query).
				WithCompiler(compiler).
				WithStore(store).
				WithTransaction(txn)

			_, err := q.Run(ctx)
			return err
		})

		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkFunctionArgumentCounts benchmarks functions with varying argument counts.
// This demonstrates the memory waste when small functions allocate 16-slot arrays.
func BenchmarkFunctionArgumentCounts(b *testing.B) {
	argCounts := []int{1, 2, 3, 5, 10, 15, 20}

	for _, argCount := range argCounts {
		b.Run(fmt.Sprintf("%d_args", argCount), func(b *testing.B) {
			ctx := b.Context()

			// Create function with N arguments
			args := make([]string, argCount)
			checks := make([]string, argCount)
			for i := range argCount {
				args[i] = fmt.Sprintf("x%d", i)
				checks[i] = fmt.Sprintf("%s == %d", args[i], i)
			}

			module := fmt.Sprintf(`package test

			f(%s) if {
				%s
			}
			`, strings.Join(args, ", "), strings.Join(checks, "\n\t\t\t\t"))

			compiler := ast.MustCompileModules(map[string]string{
				"test.rego": module,
			})

			store := inmem.NewFromObject(map[string]any{})

			// Create call with matching arguments
			callArgs := make([]string, argCount)
			for i := range argCount {
				callArgs[i] = fmt.Sprintf("%d", i)
			}
			query := ast.MustParseBody(fmt.Sprintf(`test.f(%s)`, strings.Join(callArgs, ", ")))

			b.ReportAllocs()
			b.ResetTimer()

			for b.Loop() {
				err := storage.Txn(ctx, store, storage.TransactionParams{}, func(txn storage.Transaction) error {
					q := NewQuery(query).
						WithCompiler(compiler).
						WithStore(store).
						WithTransaction(txn)

					_, err := q.Run(ctx)
					return err
				})

				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkWalkWithCustomFunction simulates the common pattern of using walk() with custom predicates.
// This is the exact scenario described in issue #7266.
func BenchmarkWalkWithCustomFunction(b *testing.B) {
	sizes := []int{10, 100, 1000}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("walk_%d_items", size), func(b *testing.B) {
			ctx := b.Context()

			// Create test data with nested objects
			items := make([]string, size)
			for i := range size {
				items[i] = fmt.Sprintf(`{"id": %d, "value": "item_%d"}`, i, i)
			}

			module := fmt.Sprintf(`package test

			is_ref(x) if {
				is_object(x)
				x.id
			}

			count_refs[count] if {
				arr := [%s]
				refs := [x | walk(arr, [_, x]); is_ref(x)]
				count := count(refs)
			}
			`, strings.Join(items, ", "))

			compiler := ast.MustCompileModules(map[string]string{
				"test.rego": module,
			})

			store := inmem.NewFromObject(map[string]any{})

			query := ast.MustParseBody(`data.test.count_refs[x]`)

			b.ReportAllocs()
			b.ResetTimer()

			for b.Loop() {
				err := storage.Txn(ctx, store, storage.TransactionParams{}, func(txn storage.Transaction) error {
					q := NewQuery(query).
						WithCompiler(compiler).
						WithStore(store).
						WithTransaction(txn)

					_, err := q.Run(ctx)
					return err
				})

				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkBindingsArrayHashmapTransition benchmarks the transition from array to map mode.
func BenchmarkBindingsArrayHashmapTransition(b *testing.B) {
	b.Run("without_hint_transition_at_17", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			bh := newBindingsArrayHashmap()
			// Add 17 bindings to force transition to map
			for j := 0; j < 17; j++ {
				key := ast.VarTerm(fmt.Sprintf("x%d", j))
				val := value{v: ast.IntNumberTerm(j)}
				bh.Put(key, val)
			}
		}
	})

	b.Run("with_hint_starts_with_map", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			bh := newBindingsArrayHashmapWithSize(17)
			// Add 17 bindings directly to map (no transition)
			for j := range 17 {
				key := ast.VarTerm(fmt.Sprintf("x%d", j))
				val := value{v: ast.IntNumberTerm(j)}
				bh.Put(key, val)
			}
		}
	})
}
