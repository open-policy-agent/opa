package topdown

import (
	"context"
	"fmt"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/storage"
	"github.com/open-policy-agent/opa/v1/storage/inmem"
)

func BenchmarkBulkStartsWithNaive(b *testing.B) {
	data := generateBulkStartsWithInput()
	ctx := context.Background()
	store := inmem.NewFromObject(data)

	compiler := ast.MustCompileModules(map[string]string{
		"test.rego": `
package test

result if {
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

result if {
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

func BenchmarkSplit(b *testing.B) {
	bctx := BuiltinContext{}
	operands := []*ast.Term{
		ast.StringTerm("a.b.c.d.e"),
		ast.StringTerm("."),
	}

	exp := eqIter(ast.ArrayTerm(
		ast.StringTerm("a"),
		ast.StringTerm("b"),
		ast.StringTerm("c"),
		ast.StringTerm("d"),
		ast.StringTerm("e"),
	))

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if err := builtinSplit(bctx, operands, exp); err != nil {
			b.Fatal(err)
		}
	}
}

func eqIter(a *ast.Term) func(*ast.Term) error {
	return func(b *ast.Term) error {
		if !a.Equal(b) {
			return fmt.Errorf("expected %v equal to %v", a, b)
		}
		return nil
	}
}
