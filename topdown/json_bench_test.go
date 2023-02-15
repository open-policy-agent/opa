// Copyright 2023 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"context"
	"fmt"
	"strconv"
	"testing"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/storage"
	inmem "github.com/open-policy-agent/opa/storage/inmem/test"
)

func BenchmarkJSONFilterShallow(b *testing.B) {
	ctx := context.Background()

	sizes := []int{10, 100, 1000, 10000}

	for _, n := range sizes {
		source := genTestObject(n)
		for _, m := range sizes {
			if m > n {
				continue // skip tests where too many paths would be removed. (error case)
			}
			testName := fmt.Sprintf("object-%d-%d", n, m)
			// Build dataset right before use:
			paths := make([]*ast.Term, 0, m)
			// paths to remove
			for i := 0; i < m; i++ {
				paths = append(paths, ast.StringTerm("/"+strconv.FormatInt(int64(i), 10)))
			}

			b.ResetTimer()
			b.Run(testName, func(b *testing.B) {
				runJSONFilterBenchmarkTest(ctx, b, source, paths)
			})
		}
	}
}

func BenchmarkJSONFilterNested(b *testing.B) {
	ctx := context.Background()

	sizes := []int{10, 100, 1000, 10000}

	for _, n := range sizes {
		source := genNestedTestObject(n, 3)
		for _, m := range sizes {
			if m > n {
				continue // skip tests where too many paths would be removed. (error case)
			}
			testName := fmt.Sprintf("object-%d-%d", n, m)
			// Build dataset right before use:
			paths := make([]*ast.Term, 0, m)
			// paths to remove
			for i := 0; i < m; i++ {
				idx := strconv.FormatInt(int64(i), 10)
				paths = append(paths, ast.StringTerm("/"+idx+"/"+idx+"/"+idx))
			}

			b.ResetTimer()
			b.Run(testName, func(b *testing.B) {
				runJSONFilterBenchmarkTest(ctx, b, source, paths)
			})
		}
	}
}

func runJSONFilterBenchmarkTest(ctx context.Context, b *testing.B, source ast.Value, paths []*ast.Term) {
	store := inmem.NewFromObject(map[string]interface{}{
		"source": source,
		"paths":  ast.NewArray(paths...),
	})

	module := `package test

			result := json.filter(data.source, data.paths)`

	query := ast.MustParseBody("data.test.result")
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
}

func genTestObject(width int) ast.Value {
	out := ast.NewObject()
	for i := 0; i < width; i++ {
		out.Insert(ast.IntNumberTerm(i), ast.IntNumberTerm(i))
	}
	return out
}

func genNestedTestObject(width, levels int) ast.Value {
	if levels == 1 {
		return genTestObject(width)
	} else if levels > 1 {
		out := ast.NewObject()
		childValue := genNestedTestObject(width, levels-1)
		for i := 0; i < width; i++ {
			out.Insert(ast.IntNumberTerm(i), ast.NewTerm(childValue))
		}
	}
	return ast.NewObject()
}
