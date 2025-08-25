// Copyright 2024 The OPA Authors. All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

//go:build opa_wasm
// +build opa_wasm

package topdown

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/rego"
	"github.com/open-policy-agent/opa/storage/inmem"
)

// benchmarkWithTargets runs the same benchmark with both topdown and WASM targets
func benchmarkWithTargets(b *testing.B, fn func(b *testing.B, target string)) {
	b.Run("topdown", func(b *testing.B) {
		fn(b, "rego")
	})
	b.Run("wasm", func(b *testing.B) {
		fn(b, "wasm")
	})
}

// BenchmarkSimpleAuthzTargets compares simple authorization between topdown and WASM
func BenchmarkSimpleAuthzTargets(b *testing.B) {
	benchmarkWithTargets(b, func(b *testing.B, target string) {
		ctx := context.Background()

		policy := `package authz
		default allow = false
		allow { input.user.role == "admin" }
		allow { input.user.id == input.resource.owner }`

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

		r := rego.New(
			rego.Query("data.authz.allow"),
			rego.Module("authz", policy),
			rego.Input(input),
			rego.Target(target),
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
// NOTE: Memory benchmarks may be inaccurate for WASM due to cgo memory allocation in wasmtime-go
func BenchmarkBuiltinPerformanceTargets(b *testing.B) {
	benchmarkWithTargets(b, func(b *testing.B, target string) {
		ctx := context.Background()

		// Test sprintf builtin which requires wasmtime for WASM
		policy := `package test
		result := sprintf("user:%s action:%s resource:%s", [input.user, input.action, input.resource])`

		input := map[string]interface{}{
			"user":     "alice",
			"action":   "read",
			"resource": "document",
		}

		r := rego.New(
			rego.Query("data.test.result"),
			rego.Module("test", policy),
			rego.Input(input),
			rego.Target(target),
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
			benchmarkWithTargets(b, func(b *testing.B, target string) {
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

				policy := `package authz
				allow {
					some i
					data.users[i].id == input.user_id
				}`

				input := map[string]interface{}{
					"user_id": fmt.Sprintf("user%d", n/2), // Look for user in middle
				}

				store := inmem.NewFromObject(data)
				r := rego.New(
					rego.Query("data.authz.allow"),
					rego.Module("authz", policy),
					rego.Input(input),
					rego.Store(store),
					rego.Target(target),
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
			benchmarkWithTargets(b, func(b *testing.B, target string) {
				ctx := context.Background()

				// Generate policy with n permission rules
				var rules []string
				for i := 0; i < n; i++ {
					rules = append(rules, fmt.Sprintf("allow { input.permissions[_] == \"perm_%d\" }", i))
				}

				policy := fmt.Sprintf(`package authz
				default allow = false
				%s`, strings.Join(rules, "\n"))

				// Generate input with matching permission
				input := map[string]interface{}{
					"permissions": []string{fmt.Sprintf("perm_%d", n/2)}, // Permission in middle
				}

				r := rego.New(
					rego.Query("data.authz.allow"),
					rego.Module("authz", policy),
					rego.Input(input),
					rego.Target(target),
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