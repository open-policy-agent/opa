// Copyright 2022 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"context"
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
	ctx := context.Background()

	sizes := []int{10, 100, 250}

	for _, n := range sizes {
		for _, m := range sizes {
			b.Run(fmt.Sprintf("%dx%d", n, m), func(b *testing.B) {
				store := inmem.NewFromObject(map[string]interface{}{"objs": genNxMObjectBenchmarkData(n, m)})
				module := `package test

				combined := object.union_n(data.objs)`

				query := ast.MustParseBody("data.test.combined")
				compiler := ast.MustCompileModules(map[string]string{
					"test.rego": module,
				})

				b.ResetTimer()

				for range b.N {

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

func BenchmarkObjectUnionNSlow(b *testing.B) {
	// This benchmarks the suggested means to implement union
	// without using the builtin, to give us an idea of whether or not
	// the builtin is actually making things any faster.
	ctx := context.Background()

	sizes := []int{10, 100, 250}

	for _, n := range sizes {
		for _, m := range sizes {
			b.Run(fmt.Sprintf("%dx%d", n, m), func(b *testing.B) {
				store := inmem.NewFromObject(map[string]interface{}{"objs": genNxMObjectBenchmarkData(n, m)})
				module := `package test

				combined := {k: true | s := data.objs[_]; s[k]}`

				query := ast.MustParseBody("data.test.combined")
				compiler := ast.MustCompileModules(map[string]string{
					"test.rego": module,
				})

				b.ResetTimer()

				for range b.N {

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
