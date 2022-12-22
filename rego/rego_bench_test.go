package rego_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/open-policy-agent/opa/ast"
	pr "github.com/open-policy-agent/opa/internal/presentation"
	"github.com/open-policy-agent/opa/internal/runtime"
	"github.com/open-policy-agent/opa/rego"
	inmem "github.com/open-policy-agent/opa/storage/inmem/test"
	"github.com/open-policy-agent/opa/util/test"
)

func BenchmarkPartialObjectRuleCrossModule(b *testing.B) {
	ctx := context.Background()
	sizes := []int{10, 100, 1000}

	for _, n := range sizes {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			store := inmem.NewFromObject(map[string]interface{}{})
			mods := test.PartialObjectBenchmarkCrossModule(n)
			query := "data.test.foo"

			input := make(map[string]interface{})
			for idx := 0; idx <= 3; idx++ {
				input[fmt.Sprintf("test_input_%d", idx)] = "test_input_10"
			}
			inputAST, err := ast.InterfaceToValue(input)
			if err != nil {
				b.Fatal(err)
			}

			compiler := ast.MustCompileModules(map[string]string{
				"test/foo.rego": mods[0],
				"test/bar.rego": mods[1],
				"test/baz.rego": mods[2],
			})
			info, err := runtime.Term(runtime.Params{})
			if err != nil {
				b.Fatal(err)
			}

			pq, err := rego.New(
				rego.Query(query),
				rego.Compiler(compiler),
				rego.Store(store),
				rego.Runtime(info),
			).PrepareForEval(ctx)

			if err != nil {
				b.Fatal(err)
			}

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				var result pr.Output

				result.Result, err = pq.Eval(
					ctx,
					rego.EvalParsedInput(inputAST),
					rego.EvalRuleIndexing(true),
					rego.EvalEarlyExit(true),
				)

				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
