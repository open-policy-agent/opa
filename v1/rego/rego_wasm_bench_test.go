//go:build opa_wasm
// +build opa_wasm

package rego

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/storage/inmem"
)

// benchmarkWithWASMTargets runs the same benchmark with both topdown and WASM targets
// NOTE: Memory benchmarks may be inaccurate for WASM due to cgo memory allocation in wasmtime-go
func benchmarkWithWASMTargets(b *testing.B, fn func(b *testing.B, target string)) {
	b.Run("topdown", func(b *testing.B) {
		fn(b, "rego")
	})
	b.Run("wasm", func(b *testing.B) {
		fn(b, "wasm")
	})
}

// BenchmarkTrivialPolicyTargets tests minimal policy performance across targets
func BenchmarkTrivialPolicyTargets(b *testing.B) {
	benchmarkWithWASMTargets(b, func(b *testing.B, target string) {
		ctx := context.Background()
		r := New(
			ParsedQuery(ast.MustParseBody("data.p.r = x")),
			ParsedModule(ast.MustParseModule(`package p
			r := 1`)),
			GenerateJSON(noOpGenerateJSON),
			Target(target),
		)

		pq, err := r.PrepareForEval(ctx)
		if err != nil {
			if target == "wasm" && strings.Contains(err.Error(), "not found") {
				b.Skip("WASM engine not available")
			}
			b.Fatal(err)
		}

		b.ResetTimer()
		for range b.N {
			if _, err := pq.Eval(ctx); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkArrayIterationTargets tests array iteration performance across targets
func BenchmarkArrayIterationTargets(b *testing.B) {
	benchmarkWithWASMTargets(b, func(b *testing.B, target string) {
		ctx := context.Background()

		at := make([]*ast.Term, 512)
		for i := range 511 {
			at[i] = ast.StringTerm("a")
		}
		at[511] = ast.StringTerm("v")

		input := ast.NewObject(ast.Item(ast.StringTerm("foo"), ast.ArrayTerm(at...)))
		module := ast.MustParseModule(`package test

		default r := false

		r if input.foo[_] == "v"`)

		r := New(
			Query("data.test.r = x"),
			ParsedModule(module),
			Target(target),
		)

		pq, err := r.PrepareForEval(ctx)
		if err != nil {
			if target == "wasm" && strings.Contains(err.Error(), "not found") {
				b.Skip("WASM engine not available")
			}
			b.Fatal(err)
		}

		b.ResetTimer()
		for range b.N {
			res, err := pq.Eval(ctx, EvalParsedInput(input))
			if err != nil {
				b.Fatal(err)
			}

			if res == nil {
				b.Fatal("expected result")
			}

			if res[0].Bindings["x"].(bool) != true {
				b.Fatalf("expected true, got %v", res[0].Bindings["x"])
			}
		}
	})
}

// BenchmarkSimpleAuthzTargets compares simple authorization between topdown and WASM
func BenchmarkSimpleAuthzTargets(b *testing.B) {
	benchmarkWithWASMTargets(b, func(b *testing.B, target string) {
		ctx := context.Background()

		module := ast.MustParseModule(`package authz
		default allow = false
		allow if input.user.role == "admin"
		allow if input.user.id == input.resource.owner`)

		input := map[string]interface{}{
			"user": map[string]interface{}{
				"id":   "user123",
				"role": "user",
			},
			"resource": map[string]interface{}{
				"id":    "doc456",
				"owner": "user123",
			},
		}

		r := New(
			Query("data.authz.allow"),
			ParsedModule(module),
			Input(input),
			Target(target),
		)

		pq, err := r.PrepareForEval(ctx)
		if err != nil {
			if target == "wasm" && strings.Contains(err.Error(), "not found") {
				b.Skip("WASM engine not available")
			}
			b.Fatal(err)
		}

		b.ResetTimer()
		for range b.N {
			rs, err := pq.Eval(ctx)
			if err != nil {
				b.Fatal(err)
			}
			if len(rs) == 0 || rs[0].Expressions[0].Value != true {
				b.Fatal("unexpected result")
			}
		}
	})
}

// BenchmarkBuiltinPerformanceTargets tests performance of builtin functions in WASM vs topdown
func BenchmarkBuiltinPerformanceTargets(b *testing.B) {
	benchmarkWithWASMTargets(b, func(b *testing.B, target string) {
		ctx := context.Background()

		// Test sprintf builtin which requires wasmtime for WASM
		module := ast.MustParseModule(`package test
		result := sprintf("user:%s action:%s resource:%s", [input.user, input.action, input.resource])`)

		input := map[string]interface{}{
			"user":     "alice",
			"action":   "read",
			"resource": "document",
		}

		r := New(
			Query("data.test.result"),
			ParsedModule(module),
			Input(input),
			Target(target),
		)

		pq, err := r.PrepareForEval(ctx)
		if err != nil {
			if target == "wasm" && strings.Contains(err.Error(), "not found") {
				b.Skip("WASM engine not available")
			}
			b.Fatal(err)
		}

		b.ResetTimer()
		for range b.N {
			rs, err := pq.Eval(ctx)
			if err != nil {
				b.Fatal(err)
			}
			if len(rs) == 0 {
				b.Fatal("no results")
			}
		}
	})
}

// BenchmarkDataSizesTargets tests performance with different data sizes
func BenchmarkDataSizesTargets(b *testing.B) {
	sizes := []int{10, 100, 1000}

	for _, n := range sizes {
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			benchmarkWithWASMTargets(b, func(b *testing.B, target string) {
				ctx := context.Background()

				// Generate test data
				data := make(map[string]interface{})
				users := make([]map[string]interface{}, n)
				for i := 0; i < n; i++ {
					users[i] = map[string]interface{}{
						"id":   fmt.Sprintf("user%d", i),
						"role": "user",
					}
				}
				data["users"] = users

				module := ast.MustParseModule(`package authz
				allow if {
					some i
					data.users[i].id == input.user_id
				}`)

				input := map[string]interface{}{
					"user_id": fmt.Sprintf("user%d", n/2), // Look for user in middle
				}

				store := inmem.NewFromObject(data)
				r := New(
					Query("data.authz.allow"),
					ParsedModule(module),
					Input(input),
					Store(store),
					Target(target),
				)

				pq, err := r.PrepareForEval(ctx)
				if err != nil {
					if target == "wasm" && strings.Contains(err.Error(), "not found") {
						b.Skip("WASM engine not available")
					}
					b.Fatal(err)
				}

				b.ResetTimer()
				for range b.N {
					rs, err := pq.Eval(ctx)
					if err != nil {
						b.Fatal(err)
					}
					if len(rs) == 0 || rs[0].Expressions[0].Value != true {
						b.Fatal("unexpected result")
					}
				}
			})
		})
	}
}

// BenchmarkPolicyComplexityTargets tests performance with different policy complexities
func BenchmarkPolicyComplexityTargets(b *testing.B) {
	sizes := []int{10, 100, 1000}

	for _, n := range sizes {
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			benchmarkWithWASMTargets(b, func(b *testing.B, target string) {
				ctx := context.Background()

				// Generate policy with n permission rules
				var rules []string
				for i := 0; i < n; i++ {
					rules = append(rules, fmt.Sprintf("allow if input.permissions[_] == \"perm_%d\"", i))
				}

				policyStr := fmt.Sprintf(`package authz
				default allow = false
				%s`, strings.Join(rules, "\n"))
				module := ast.MustParseModule(policyStr)

				// Generate input with matching permission
				input := map[string]interface{}{
					"permissions": []string{fmt.Sprintf("perm_%d", n/2)}, // Permission in middle
				}

				r := New(
					Query("data.authz.allow"),
					ParsedModule(module),
					Input(input),
					Target(target),
				)

				pq, err := r.PrepareForEval(ctx)
				if err != nil {
					if target == "wasm" && strings.Contains(err.Error(), "not found") {
						b.Skip("WASM engine not available")
					}
					b.Fatal(err)
				}

				b.ResetTimer()
				for range b.N {
					rs, err := pq.Eval(ctx)
					if err != nil {
						b.Fatal(err)
					}
					if len(rs) == 0 || rs[0].Expressions[0].Value != true {
						b.Fatal("unexpected result")
					}
				}
			})
		})
	}
}
