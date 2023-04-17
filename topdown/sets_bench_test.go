// Copyright 2022 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"context"
	"fmt"
	"testing"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/storage"
	inmem "github.com/open-policy-agent/opa/storage/inmem/test"
)

func genNxMSetBenchmarkData(n, m int) ast.Value {
	setOfSets := ast.NewSet()
	for i := 0; i < n; i++ {
		v := ast.NewSet()
		for j := 0; j < m; j++ {
			v.Add(ast.StringTerm(fmt.Sprintf("%d,%d", i, j)))
		}
		setOfSets.Add(ast.NewTerm(v))
	}
	return setOfSets
}

func BenchmarkSetIntersection(b *testing.B) {
	ctx := context.Background()

	sizes := []int{10, 100, 1000}

	for _, n := range sizes {
		for _, m := range sizes {
			b.Run(fmt.Sprintf("%dx%d", n, m), func(b *testing.B) {
				store := inmem.NewFromObject(map[string]interface{}{"sets": genNxMSetBenchmarkData(n, m)})

				module := `package test

				combined := intersection({s | s := data.sets[_]})`

				query := ast.MustParseBody("data.test.combined")
				compiler := ast.MustCompileModules(map[string]string{
					"test.rego": module,
				})

				b.ResetTimer()

				for i := 0; i < b.N; i++ {

					err := storage.Txn(ctx, store, storage.TransactionParams{}, func(txn storage.Transaction) error {

						q := NewQuery(query).
							WithCompiler(compiler).
							WithStore(store).
							WithTransaction(txn)

						_, err := q.Run(ctx)
						if err != nil {
							return err
						}

						return nil
					})

					if err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}

func BenchmarkSetIntersectionSlow(b *testing.B) {
	ctx := context.Background()

	sizes := []int{10, 50, 100}

	for _, n := range sizes {
		for _, m := range sizes {
			b.Run(fmt.Sprintf("%dx%d", n, m), func(b *testing.B) {
				store := inmem.NewFromObject(map[string]interface{}{"sets": genNxMSetBenchmarkData(n, m)})

				module := `package test
				import future.keywords.every
				import future.keywords.in

				combined[z] {
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

				for i := 0; i < b.N; i++ {

					err := storage.Txn(ctx, store, storage.TransactionParams{}, func(txn storage.Transaction) error {

						q := NewQuery(query).
							WithCompiler(compiler).
							WithStore(store).
							WithTransaction(txn)

						_, err := q.Run(ctx)
						if err != nil {
							return err
						}

						return nil
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
	ctx := context.Background()

	sizes := []int{10, 100, 250}

	for _, n := range sizes {
		for _, m := range sizes {
			b.Run(fmt.Sprintf("%dx%d", n, m), func(b *testing.B) {
				store := inmem.NewFromObject(map[string]interface{}{"sets": genNxMSetBenchmarkData(n, m)})

				// Code is lifted from here:
				// https://github.com/open-policy-agent/opa/issues/4979#issue-1332019382

				module := `package test

				combined := union({s | s := data.sets[_]})`

				query := ast.MustParseBody("data.test.combined")
				compiler := ast.MustCompileModules(map[string]string{
					"test.rego": module,
				})

				b.ResetTimer()

				for i := 0; i < b.N; i++ {

					err := storage.Txn(ctx, store, storage.TransactionParams{}, func(txn storage.Transaction) error {

						q := NewQuery(query).
							WithCompiler(compiler).
							WithStore(store).
							WithTransaction(txn)

						_, err := q.Run(ctx)
						if err != nil {
							return err
						}

						return nil
					})

					if err != nil {
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
	ctx := context.Background()

	sizes := []int{10, 100, 250}

	for _, n := range sizes {
		for _, m := range sizes {
			b.Run(fmt.Sprintf("%dx%d", n, m), func(b *testing.B) {
				store := inmem.NewFromObject(map[string]interface{}{"sets": genNxMSetBenchmarkData(n, m)})

				// Code is lifted from here:
				// https://github.com/open-policy-agent/opa/issues/4979#issue-1332019382

				module := `package test

				combined := {t | s := data.sets[_]; s[t]}`

				query := ast.MustParseBody("data.test.combined")
				compiler := ast.MustCompileModules(map[string]string{
					"test.rego": module,
				})

				b.ResetTimer()

				for i := 0; i < b.N; i++ {

					err := storage.Txn(ctx, store, storage.TransactionParams{}, func(txn storage.Transaction) error {

						q := NewQuery(query).
							WithCompiler(compiler).
							WithStore(store).
							WithTransaction(txn)

						_, err := q.Run(ctx)
						if err != nil {
							return err
						}

						return nil
					})

					if err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}
