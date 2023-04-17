package compile

import (
	"context"
	"fmt"
	"strings"
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

	numPolicies := []int{1000, 2500, 5000, 7500, 10000}

	for _, n := range numPolicies {
		testcase := generateDynamicPolicyBenchmarkData(n)
		b.Run(fmt.Sprintf("%d", n), func(b *testing.B) {
			test.WithTempFS(testcase, func(root string) {
				b.ResetTimer()

				compiler := New().
					WithPaths(root)

				err := compiler.Build(context.Background())
				if err != nil {
					b.Fatal("unexpected error", err)
				}
			})
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
		files[fmt.Sprintf("policy%d.rego", i)] = generateDynamicMockPolicy(i)
	}

	return files
}

func generateDynamicMockPolicy(N int) string {
	return fmt.Sprintf(`package policies["%d"]["%d"].policy%d
denies[x] {
	input.attribute == "%d"
	x := "policy%d"
}`, N, N, N, N, N)
}

func BenchmarkLargePartialRulePolicy(b *testing.B) {
	// This benchmarks the compiler against very large partial rule sets.
	// See: https://github.com/open-policy-agent/opa/issues/5756
	numPolicies := []int{1000, 2500, 5000, 7500}

	for _, n := range numPolicies {
		testcase := generateLargePartialRuleBenchmarkData(n)
		b.ResetTimer()
		b.Run(fmt.Sprintf("%d", n), func(b *testing.B) {
			test.WithTempFS(testcase, func(root string) {
				b.ResetTimer()

				compiler := New().
					WithPaths(root)

				err := compiler.Build(context.Background())
				if err != nil {
					b.Fatal("unexpected error", err)
				}
			})
		})
	}
}

func generateLargePartialRuleBenchmarkData(N int) map[string]string {
	var policy strings.Builder
	policy.Grow((140 * N) + 100) // Each rule takes around 130 characters.

	policy.WriteString(`package example.large.partial.rules.policy["dynamic_part"].main`)
	policy.WriteString("\n\n")
	for i := 0; i < N; i++ {
		policy.WriteString(generateLargePartialRuleMockRule(i))
		policy.WriteString("\n\n")
	}
	policy.WriteString(`number_denies = x {
		x := count(deny)
	}`)

	files := map[string]string{
		"main.rego": policy.String(),
	}
	return files
}

func generateLargePartialRuleMockRule(N int) string {
	return fmt.Sprintf(`deny[[resource, errormsg]] {
		resource := "example.%d"
		i := %d
		i %% 2 != 0
		errormsg := "denied because %d is an odd number."
}`, N, N, N)
}
