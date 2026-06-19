// Copyright 2022 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"fmt"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/storage"
	inmem "github.com/open-policy-agent/opa/v1/storage/inmem/test"
)

func genNxMObjectBenchmarkData(n, m int) ast.Value {
	objList := make([]*ast.Term, n)
	for i := range n {
		v := ast.NewObject()
		for j := range m {
			v.Insert(ast.StringTerm(fmt.Sprintf("%d,%d", i, j)), ast.BooleanTerm(true))
		}
		objList[i] = ast.NewTerm(v)
	}
	return ast.NewArray(objList...)
}

func BenchmarkObjectUnionN(b *testing.B) {
	sizes := []int{10, 100, 250}

	for _, n := range sizes {
		for _, m := range sizes {
			b.Run(fmt.Sprintf("%dx%d", n, m), func(b *testing.B) {
				store := inmem.NewFromObject(map[string]any{"objs": genNxMObjectBenchmarkData(n, m)})
				compiler := ast.MustCompileModules(map[string]string{
					"test.rego": "package test\n\ncombined := object.union_n(data.objs)",
				})
				ctx := b.Context()

				b.ResetTimer()

				err := storage.Txn(ctx, store, storage.TransactionParams{}, func(txn storage.Transaction) error {
					q := NewQuery(ast.MustParseBody("data.test.combined")).
						WithCompiler(compiler).
						WithStore(store).
						WithTransaction(txn)

					for b.Loop() {
						if _, err := q.Run(ctx); err != nil {
							b.Fatal(err)
						}
					}

					return nil
				})

				if err != nil {
					b.Fatal(err)
				}
			})
		}
	}
}

func BenchmarkObjectUnionNSlow(b *testing.B) {
	// This benchmarks the suggested means to implement union
	// without using the builtin, to give us an idea of whether or not
	// the builtin is actually making things any faster.
	ctx := b.Context()

	sizes := []int{10, 100, 250}

	for _, n := range sizes {
		for _, m := range sizes {
			b.Run(fmt.Sprintf("%dx%d", n, m), func(b *testing.B) {
				store := inmem.NewFromObject(map[string]any{"objs": genNxMObjectBenchmarkData(n, m)})
				module := `package test

				combined := {k: true | s := data.objs[_]; s[k]}`

				query := ast.MustParseBody("data.test.combined")
				compiler := ast.MustCompileModules(map[string]string{
					"test.rego": module,
				})

				b.ResetTimer()

				for b.Loop() {
					err := storage.Txn(ctx, store, storage.TransactionParams{}, func(txn storage.Transaction) error {
						_, err := NewQuery(query).
							WithCompiler(compiler).
							WithStore(store).
							WithTransaction(txn).
							Run(ctx)

						return err
					})

					if err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}

// empty_array-16                    102861856	        11.5 ns/op	       0 B/op	       0 allocs/op
// single_object-16                   43886583	        25.9 ns/op	       0 B/op	       0 allocs/op
// merge_empty-16                      6329768	       189.9 ns/op	     392 B/op	       8 allocs/op
// merge_equal-16                      5543514	       216.9 ns/op	     400 B/op	       8 allocs/op
// merge_non-overlapping-16            4018898	       296.0 ns/op	     456 B/op	      10 allocs/op
// merge_overlapping-16                4234546	       282.7 ns/op	     576 B/op	      10 allocs/op
// merge_nested-16                     2450734	       489.9 ns/op	     816 B/op	      18 allocs/op
// merge_nested_with_conflict-16       2441014	       492.6 ns/op	     920 B/op	      17 allocs/op
// merge_nested_1_equal_branch-16      1553234	       773.5 ns/op	    1272 B/op	      24 allocs/op
// merge_nested_no_equal_branch-16     1490298	       804.1 ns/op	    1344 B/op	      27 allocs/op
func BenchmarkObjectUnionNCallOnly(b *testing.B) {
	cases := []struct {
		name string
		objs []string
		want string
	}{
		{"empty array", []string{}, `{}`},
		{"single object", []string{
			`{"a": 1}`,
		}, `{"a": 1}`},
		{"merge empty", []string{
			`{"a": 1}`,
			`{}`,
		}, `{"a": 1}`},
		{"merge equal", []string{
			`{"a": 1}`,
			`{"a": 1}`,
		}, `{"a": 1}`},
		{"merge non-overlapping", []string{
			`{"a": 1}`,
			`{"b": 2}`,
		}, `{"a": 1, "b": 2}`},
		{"merge overlapping", []string{
			`{"a": 1}`,
			`{"a": 2}`,
		}, `{"a": 2}`},
		{"merge nested", []string{
			`{"a": {"b": 1}}`,
			`{"a": {"c": 2}}`,
		}, `{"a": {"b": 1, "c": 2}}`},
		{"merge nested with conflict", []string{
			`{"a": {"b": 1}}`,
			`{"a": {"b": 2}}`,
		}, `{"a": {"b": 2}}`},
		{"merge nested 1 equal branch", []string{
			`{"a": {"b": 1}, "b": {"c": 1}}`,
			`{"a": {"b": 1}, "b": {"c": 2}}`,
		}, `{"a": {"b": 1}, "b": {"c": 2}}`},
		{"merge nested no equal branch", []string{
			`{"a": {"b": 1}, "b": {"c": 1}}`,
			`{"a": {"b": 2}, "b": {"d": 2}}`,
		}, `{"a": {"b": 2}, "b": {"c": 1, "d": 2}}`},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			arr := make([]*ast.Term, len(tc.objs))
			for i, o := range tc.objs {
				arr[i] = ast.MustParseTerm(o)
			}

			exp := ast.MustParseTerm(tc.want)
			ops := []*ast.Term{ast.ArrayTerm(arr...)}

			for b.Loop() {
				if err := builtinObjectUnionN(BuiltinContext{}, ops, eqIter(exp)); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// 72.64 ns/op	      56 B/op	       2 allocs/op
// 45.49 ns/op	       0 B/op	       0 allocs/op
func BenchmarkObjectGetFound(b *testing.B) {
	obj := ast.MustParseTerm(`{"a": {"b": {"c": {"d": 1}}}}`)
	arr := ast.ArrayTerm(ast.InternedTerm("a"), ast.InternedTerm("b"), ast.InternedTerm("c"), ast.InternedTerm("d"))
	def := ast.NullTerm()

	ops := []*ast.Term{obj, arr, def}
	exp := eqIter(ast.InternedTerm(1))
	bcx := BuiltinContext{}

	for b.Loop() {
		if err := builtinObjectGet(bcx, ops, exp); err != nil {
			b.Fatal(err)
		}
	}
}

// 48.15 ns/op	      32 B/op	       1 allocs/op
// 36.74 ns/op	       0 B/op	       0 allocs/op
func BenchmarkObjectGetNotFound(b *testing.B) {
	obj := ast.MustParseTerm(`{"a": {"b": {"c": {"d": 1}}}}`)
	arr := ast.ArrayTerm(ast.InternedTerm("a"), ast.InternedTerm("b"), ast.InternedTerm("c"), ast.InternedTerm("e"))
	def := ast.NullTerm()

	ops := []*ast.Term{obj, arr, def}
	exp := eqIter(def)
	bcx := BuiltinContext{}

	for b.Loop() {
		if err := builtinObjectGet(bcx, ops, exp); err != nil {
			b.Fatal(err)
		}
	}
}
