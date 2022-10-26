package compile

import (
	"context"
	"fmt"
	"testing"

	"github.com/open-policy-agent/opa/util/test"
)

// type compileBenchTestData struct {
// 	filename string
// 	module   string
// }

func BenchmarkCompileDynamicPolicy(b *testing.B) {
	// This benchmarks the compiler against increasingly large numbers of dynamically-selected policies.
	// See: https://github.com/open-policy-agent/opa/issues/5216
	//ctx := context.Background()

	numPolicies := []int{1000, 2500, 5000, 7500, 10000}
	testcases := map[int]map[string]string{}

	for _, n := range numPolicies {
		testcases[n] = generateDynamicPolicyBenchmarkData(n)
	}

	b.ResetTimer()

	for _, n := range numPolicies {
		b.Run(fmt.Sprintf("%d", n), func(b *testing.B) {
			test.WithTempFS(testcases[n], func(root string) {
				b.ResetTimer()

				compiler := New().
					WithPaths(root)

				err := compiler.Build(context.Background())
				if err != nil {
					b.Fatal("unexpected error", err)
				}
			})

			// store := inmem.NewFromObject(map[string]interface{}{"objs": generateMockPolicy(n)})
			// module := `package test

			// 	combined := {k: true | s := data.objs[_]; s[k]}`

			// query := ast.MustParseBody("data.test.combined")
			// compiler := ast.MustCompileModules(map[string]string{
			// 	"test.rego": module,
			// })

			// b.ResetTimer()

			// for i := 0; i < b.N; i++ {

			// 	err := storage.Txn(ctx, store, storage.TransactionParams{}, func(txn storage.Transaction) error {

			// 		q := NewQuery(query).
			// 			WithCompiler(compiler).
			// 			WithStore(store).
			// 			WithTransaction(txn)

			// 		_, err := q.Run(ctx)
			// 		if err != nil {
			// 			return err
			// 		}

			// 		return nil
			// 	})

			// 	if err != nil {
			// 		b.Fatal(err)
			// 	}
			// }
		})
	}
}

func generateDynamicPolicyBenchmarkData(N int) map[string]string {
	files := map[string]string{
		"main.rego": `
			package main

			denies[x] {
				x := data.policies[input.type][input.subtype][_].denies[_]
			}
			any_denies {
				denies[_]
			}
			allow {
				not any_denies
			}`,
	}

	for i := 0; i < N; i++ {
		files[fmt.Sprintf("policy%d.rego", i)] = generateMockPolicy(i)
	}

	return files
}

func generateMockPolicy(N int) string {
	return fmt.Sprintf(`package policies["%d"]["%d"].policy%d
denies[x] {
	input.attribute == "%d"
	x := "policy%d"
}`, N, N, N, N, N)
}
