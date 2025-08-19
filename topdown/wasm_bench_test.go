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

// BenchmarkWASMvsRego compares WASM and regular Rego performance
func BenchmarkWASMvsRego(b *testing.B) {
	ctx := context.Background()

	// Simple authorization policy
	simplePolicy := `package authz
	default allow = false
	allow { input.user.role == "admin" }
	allow { input.user.id == input.resource.owner }`

	simpleInput := map[string]interface{}{
		"user": map[string]interface{}{
			"id":   "user123",
			"role": "user",
		},
		"resource": map[string]interface{}{
			"id":    "doc456",
			"owner": "user123",
		},
	}

	// Run benchmarks for both targets
	for _, target := range []string{"rego", "wasm"} {
		b.Run(target, func(b *testing.B) {
			// Try to compile once to check if target is available
			r := rego.New(
				rego.Query("data.authz.allow"),
				rego.Module("authz", simplePolicy),
				rego.Input(simpleInput),
				rego.Target(target),
			)
			_, err := r.PrepareForEval(ctx)
			if err != nil {
				if target == "wasm" && strings.Contains(err.Error(), "not found") {
					b.Skip("WASM engine not available")
				}
				b.Fatal(err)
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				r := rego.New(
					rego.Query("data.authz.allow"),
					rego.Module("authz", simplePolicy),
					rego.Input(simpleInput),
					rego.Target(target),
				)

				rs, err := r.Eval(ctx)
				if err != nil {
					b.Fatal(err)
				}

				if len(rs) == 0 || rs[0].Expressions[0].Value != true {
					b.Fatal("unexpected result")
				}
			}
		})
	}
}

// BenchmarkWASMScaling tests WASM performance with different policy sizes
func BenchmarkWASMScaling(b *testing.B) {
	ctx := context.Background()
	sizes := []int{10, 100, 1000}

	for _, n := range sizes {
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			// Generate policy with n rules
			var rules []string
			for i := 0; i < n; i++ {
				rules = append(rules, fmt.Sprintf("allow { input.user.permissions[_] == \"perm_%d\" }", i))
			}
			
			policy := fmt.Sprintf(`package authz
			default allow = false
			%s`, strings.Join(rules, "\n"))

			// Generate input with some matching permissions
			perms := make([]string, 0, n/10)
			for i := 0; i < n/10; i++ {
				perms = append(perms, fmt.Sprintf("perm_%d", i*10))
			}

			input := map[string]interface{}{
				"user": map[string]interface{}{
					"permissions": perms,
				},
			}

			// Try to compile once to check if WASM is available
			r := rego.New(
				rego.Query("data.authz.allow"),
				rego.Module("authz", policy),
				rego.Input(input),
				rego.Target("wasm"),
			)
			_, err := r.PrepareForEval(ctx)
			if err != nil {
				if strings.Contains(err.Error(), "not found") {
					b.Skip("WASM engine not available")
				}
				b.Fatal(err)
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				r := rego.New(
					rego.Query("data.authz.allow"),
					rego.Module("authz", policy),
					rego.Input(input),
					rego.Target("wasm"),
				)

				rs, err := r.Eval(ctx)
				if err != nil {
					b.Fatal(err)
				}

				if len(rs) == 0 {
					b.Fatal("no results")
				}
			}
		})
	}
}

// BenchmarkWASMWithData tests WASM performance with external data
func BenchmarkWASMWithData(b *testing.B) {
	ctx := context.Background()

	policy := `package authz
	default allow = false
	
	allow {
		input.user.role == "user"
		perm := sprintf("%s:%s", [input.action, input.resource.type])
		perm == data.permissions[input.user.role][_]
	}`

	data := map[string]interface{}{
		"permissions": map[string]interface{}{
			"admin": []string{"read:any", "write:any", "delete:any"},
			"user":  []string{"read:document", "write:document"},
			"guest": []string{"read:document"},
		},
	}

	input := map[string]interface{}{
		"user": map[string]interface{}{
			"role": "user",
		},
		"action": "read",
		"resource": map[string]interface{}{
			"type": "document",
		},
	}

	store := inmem.NewFromObject(data)

	// Try to compile once to check if WASM is available
	r := rego.New(
		rego.Query("data.authz.allow"),
		rego.Module("authz", policy),
		rego.Input(input),
		rego.Store(store),
		rego.Target("wasm"),
	)
	_, err := r.PrepareForEval(ctx)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			b.Skip("WASM engine not available")
		}
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r := rego.New(
			rego.Query("data.authz.allow"),
			rego.Module("authz", policy),
			rego.Input(input),
			rego.Store(store),
			rego.Target("wasm"),
		)

		rs, err := r.Eval(ctx)
		if err != nil {
			b.Fatal(err)
		}

		if len(rs) == 0 || rs[0].Expressions[0].Value != true {
			b.Fatal("unexpected result")
		}
	}
}