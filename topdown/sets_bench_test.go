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

func BenchmarkSetUnion(b *testing.B) {
	ctx := context.Background()

	sizes := []int{10, 100, 250}

	for _, n := range sizes {
		for _, m := range sizes {
			b.Run(fmt.Sprintf("%dx%d", n, m), func(b *testing.B) {
				store := inmem.NewFromObject(map[string]interface{}{"nsets": n, "nsize": m})

				// Code is lifted from here:
				// https://github.com/open-policy-agent/opa/issues/4979#issue-1332019382

				module := `package test

				nums := numbers.range(0, data.nsets)
				sizes := numbers.range(0, data.nsize)

				sets[n] = x {
					nums[n]
					x := {sprintf("%d,%d", [n, i]) | sizes[i]}
				}
				combined := union({s | s := sets[_]})`

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
				store := inmem.NewFromObject(map[string]interface{}{"nsets": n, "nsize": m})

				// Code is lifted from here:
				// https://github.com/open-policy-agent/opa/issues/4979#issue-1332019382

				module := `package test

				nums := numbers.range(0, data.nsets)
				sizes := numbers.range(0, data.nsize)

				sets[n] = x {
					nums[n]
					x := {sprintf("%d,%d", [n, i]) | sizes[i]}
				}
				combined := {t | s := sets[_]; s[t]}`

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
