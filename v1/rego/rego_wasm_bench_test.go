//go:build opa_wasm
// +build opa_wasm

package rego

import (
	"context"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
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