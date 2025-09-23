// Copyright 2025 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

//go:build opa_wasm
// +build opa_wasm

package rego

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/storage/inmem"
)

func isWASMNotAvailable(err error, target string) bool {
	return target == "wasm" && err != nil && strings.Contains(err.Error(), "not found")
}

// NOTE: Memory benchmarks may be inaccurate for WASM due to cgo allocation through wasmtime-go
func benchmarkWithWASMTargets(b *testing.B, fn func(b *testing.B, target string)) {
	b.Helper()
	b.Run("topdown", func(b *testing.B) {
		fn(b, "rego")
	})
	b.Run("wasm", func(b *testing.B) {
		fn(b, "wasm")
	})
}

func BenchmarkTrivialPolicyTargets(b *testing.B) {
	benchmarkWithWASMTargets(b, func(b *testing.B, target string) {
		ctx := b.Context()
		r := New(
			ParsedQuery(ast.MustParseBody("data.p.r = x")),
			ParsedModule(ast.MustParseModule(`package p
			r := 1`)),
			GenerateJSON(noOpGenerateJSON),
			Target(target),
		)

		pq, err := r.PrepareForEval(ctx)
		if err != nil {
			if isWASMNotAvailable(err, target) {
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

func BenchmarkArrayIterationTargets(b *testing.B) {
	benchmarkWithWASMTargets(b, func(b *testing.B, target string) {
		ctx := b.Context()

		const arraySize = 512
		at := make([]*ast.Term, arraySize)
		for i := range arraySize - 1 {
			at[i] = ast.StringTerm("a")
		}
		at[arraySize-1] = ast.StringTerm("v")

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
			if isWASMNotAvailable(err, target) {
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

			val, ok := res[0].Bindings["x"].(bool)
			if !ok || !val {
				b.Fatalf("expected true, got %v", res[0].Bindings["x"])
			}
		}
	})
}

func BenchmarkSimpleAuthzTargets(b *testing.B) {
	benchmarkWithWASMTargets(b, func(b *testing.B, target string) {
		ctx := b.Context()

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
			if isWASMNotAvailable(err, target) {
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

func BenchmarkBuiltinPerformanceTargets(b *testing.B) {
	benchmarkWithWASMTargets(b, func(b *testing.B, target string) {
		ctx := b.Context()

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
			if isWASMNotAvailable(err, target) {
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

func BenchmarkDataSizesTargets(b *testing.B) {
	sizes := []int{10, 100, 1000}

	for _, n := range sizes {
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			benchmarkWithWASMTargets(b, func(b *testing.B, target string) {
				ctx := b.Context()

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
					"user_id": fmt.Sprintf("user%d", n/2),
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
					if isWASMNotAvailable(err, target) {
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

func BenchmarkPolicyComplexityTargets(b *testing.B) {
	sizes := []int{10, 100, 1000}

	for _, n := range sizes {
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			benchmarkWithWASMTargets(b, func(b *testing.B, target string) {
				ctx := b.Context()

				rules := make([]string, 0, n)
				for i := 0; i < n; i++ {
					rules = append(rules, fmt.Sprintf("allow if input.permissions[_] == \"perm_%d\"", i))
				}

				policyStr := fmt.Sprintf(`package authz
				default allow = false
				%s`, strings.Join(rules, "\n"))
				module := ast.MustParseModule(policyStr)

				input := map[string]interface{}{
					"permissions": []string{fmt.Sprintf("perm_%d", n/2)},
				}

				r := New(
					Query("data.authz.allow"),
					ParsedModule(module),
					Input(input),
					Target(target),
				)

				pq, err := r.PrepareForEval(ctx)
				if err != nil {
					if isWASMNotAvailable(err, target) {
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

func BenchmarkWASMCompilationTargets(b *testing.B) {
	benchmarkWithWASMTargets(b, func(b *testing.B, target string) {
		module := ast.MustParseModule(`package test
		import rego.v1
		default allow := false
		allow if input.user == "admin"
		allow if input.role in ["editor", "viewer"]`)

		b.ResetTimer()
		for range b.N {
			r := New(
				Query("data.test.allow"),
				ParsedModule(module),
				Target(target),
			)

			ctx := b.Context()
			_, err := r.PrepareForEval(ctx)
			if err != nil {
				if isWASMNotAvailable(err, target) {
					b.Skip("WASM engine not available")
				}
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkWASMColdStartTargets(b *testing.B) {
	benchmarkWithWASMTargets(b, func(b *testing.B, target string) {
		module := ast.MustParseModule(`package cold
		allow if input.user.role == "admin"`)

		input := map[string]interface{}{
			"user": map[string]interface{}{
				"role": "admin",
			},
		}

		b.ResetTimer()
		for range b.N {
			ctx := b.Context()
			r := New(
				Query("data.cold.allow"),
				ParsedModule(module),
				Input(input),
				Target(target),
			)

			pq, err := r.PrepareForEval(ctx)
			if err != nil {
				if isWASMNotAvailable(err, target) {
					b.Skip("WASM engine not available")
				}
				b.Fatal(err)
			}

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

func BenchmarkMemoryAllocationTargets(b *testing.B) {
	sizes := []int{100, 1000, 10000}

	for _, n := range sizes {
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			benchmarkWithWASMTargets(b, func(b *testing.B, target string) {
				ctx := b.Context()

				// Generate large data structure to stress memory
				data := make(map[string]interface{})
				items := make([]interface{}, n)
				for i := 0; i < n; i++ {
					items[i] = map[string]interface{}{
						"id":    fmt.Sprintf("item_%d", i),
						"value": i,
						"tags":  []string{"tag1", "tag2", "tag3"},
					}
				}
				data["items"] = items

				module := ast.MustParseModule(`package memory
				import rego.v1

				processed contains item if {
					some i
					item := data.items[i]
					item.value > 50
				}

				count_processed := count(processed)`)

				store := inmem.NewFromObject(data)
				r := New(
					Query("data.memory.count_processed"),
					ParsedModule(module),
					Store(store),
					Target(target),
				)

				pq, err := r.PrepareForEval(ctx)
				if err != nil {
					if isWASMNotAvailable(err, target) {
						b.Skip("WASM engine not available")
					}
					b.Fatal(err)
				}

				b.ReportAllocs()
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
		})
	}
}

func BenchmarkBundleSizeTargets(b *testing.B) {
	policies := []struct {
		name string
		size int
	}{
		{"small", 10},
		{"medium", 100},
		{"large", 1000},
	}

	for _, p := range policies {
		b.Run(p.name, func(b *testing.B) {
			// Generate policy with multiple rules
			var rules []string
			for i := 0; i < p.size; i++ {
				rules = append(rules, fmt.Sprintf(`
				rule_%d contains msg if {
					input.data[_].type == "type_%d"
					msg := "matched rule %d"
				}`, i, i, i))
			}

			module := ast.MustParseModule(fmt.Sprintf(`package bundle
			import rego.v1
			%s`, strings.Join(rules, "\n")))

			// Measure compilation for both targets
			b.Run("topdown", func(b *testing.B) {
				ctx := b.Context()
				b.ResetTimer()

				for range b.N {
					r := New(
						Query("data.bundle"),
						ParsedModule(module),
						Target("rego"),
					)

					_, err := r.PrepareForEval(ctx)
					if err != nil {
						b.Fatal(err)
					}
				}
			})

			b.Run("wasm", func(b *testing.B) {
				ctx := b.Context()
				b.ResetTimer()

				for range b.N {
					r := New(
						Query("data.bundle"),
						ParsedModule(module),
						Target("wasm"),
					)

					_, err := r.PrepareForEval(ctx)
					if err != nil {
						if isWASMNotAvailable(err, "wasm") {
							b.Skip("WASM engine not available")
						}
						b.Fatal(err)
					}
				}
			})
		})
	}
}