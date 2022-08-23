package topdown

import (
	"context"
	"fmt"
	"testing"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/storage/inmem"
)

func BenchmarkBulkStartsWithNaive(b *testing.B) {
	data := generateBulkStartsWithInput()
	ctx := context.Background()
	store := inmem.NewFromObject(data)

	compiler := ast.MustCompileModules(map[string]string{
		"test.rego": `
package test

result {
  startswith(data.strings[_], data.prefixes[_])
}
`,
	})

	query, err := compiler.QueryCompiler().Compile(ast.MustParseBody("data.test.result"))
	if err != nil {
		b.Fatal(err)
	}

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

func BenchmarkBulkStartsWithOptimized(b *testing.B) {
	data := generateBulkStartsWithInput()
	ctx := context.Background()
	store := inmem.NewFromObject(data)

	compiler := ast.MustCompileModules(map[string]string{
		"test.rego": `
package test

result {
  strings.any_prefix_match(data.strings, data.prefixes)
}
`,
	})

	query, err := compiler.QueryCompiler().Compile(ast.MustParseBody("data.test.result"))
	if err != nil {
		b.Fatal(err)
	}

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

func generateBulkStartsWithInput() map[string]interface{} {
	var strs, prefixes []string
	for i := 0; i < 1000; i++ {
		strs = append(strs, fmt.Sprintf("aabbccddeeffgghhiijjkkllmmnnoopp_%d", i))
	}
	for i := 0; i < 100; i++ {
		prefixes = append(prefixes, fmt.Sprintf("aabbccddeeffgghhiijjkkllmmnnoorr_%d", i))
	}
	return map[string]interface{}{
		"strings":  strs,
		"prefixes": prefixes,
	}
}
