// Copyright 2022 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/storage"
	inmem "github.com/open-policy-agent/opa/v1/storage/inmem/test"
	"github.com/open-policy-agent/opa/v1/util"
)

func genNxMSetBenchmarkData(n, m int) ast.Value {
	setOfSets := ast.NewSet()
	for i := range n {
		v := ast.NewSet()
		for j := range m {
			v.Add(ast.StringTerm(fmtInts(i, j, ',')))
		}
		setOfSets.Add(ast.NewTerm(v))
	}
	return setOfSets
}

func BenchmarkSetIntersection(b *testing.B) {
	sizes := []int{10, 100, 1000}

	for _, n := range sizes {
		for _, m := range sizes {
			b.Run(fmtInts(n, m, 'x'), func(b *testing.B) {
				ops := []*ast.Term{ast.NewTerm(genNxMSetBenchmarkData(n, m))}

				for b.Loop() {
					if err := builtinSetIntersection(BuiltinContext{}, ops, noOpIter); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}

func BenchmarkSetIntersectionSlow(b *testing.B) {
	sizes := []int{10, 50, 100}

	for _, n := range sizes {
		for _, m := range sizes {
			b.Run(fmtInts(n, m, 'x'), func(b *testing.B) {
				store := inmem.NewFromObject(map[string]any{"sets": genNxMSetBenchmarkData(n, m)})

				module := `package test

				combined contains z if {
					data.sets[m][z]
					every ss in data.sets {
						ss[z]
					}
				}`

				query := ast.MustParseBody("data.test.combined")
				compiler := ast.MustCompileModules(map[string]string{
					"test.rego": module,
				})

				b.ResetTimer()
				ctx := b.Context()

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

func BenchmarkSetUnion(b *testing.B) {
	sizes := []int{10, 100, 250}

	for _, n := range sizes {
		for _, m := range sizes {
			b.Run(fmtInts(n, m, 'x'), func(b *testing.B) {
				ops := []*ast.Term{ast.NewTerm(genNxMSetBenchmarkData(n, m))}

				for b.Loop() {
					if err := builtinSetUnion(BuiltinContext{}, ops, noOpIter); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}

func BenchmarkSetUnionSlow(b *testing.B) {
	// This benchmarks the suggested means to implement union
	// without using the builtin, to give us an idea of whether or not
	// the builtin is actually making things any faster.
	sizes := []int{10, 100, 250}

	for _, n := range sizes {
		for _, m := range sizes {
			b.Run(fmtInts(n, m, 'x'), func(b *testing.B) {
				ctx := b.Context()
				store := inmem.NewFromObject(map[string]any{"sets": genNxMSetBenchmarkData(n, m)})

				// Code is lifted from here:
				// https://github.com/open-policy-agent/opa/issues/4979#issue-1332019382

				module := `package test

				combined := {t | s := data.sets[_]; s[t]}`

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

func fmtInts(n, m int, sep byte) string {
	return util.ByteSliceToString(util.AppendInt(append(util.AppendInt(make([]byte, 0, 32), n), sep), m))
}
