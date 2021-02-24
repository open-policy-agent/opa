// Copyright 2021 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// +build opa_wasm

package opa_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/open-policy-agent/opa/internal/wasm/sdk/internal/wasm"
	"github.com/open-policy-agent/opa/internal/wasm/sdk/opa"
	"github.com/open-policy-agent/opa/rego"
	"github.com/open-policy-agent/opa/util/test"
)

func BenchmarkWasmRego(b *testing.B) {
	policy := compileRegoToWasm("a = true", "data.p.a = x", false)
	instance, _ := opa.New().
		WithPolicyBytes(policy).
		WithMemoryLimits(131070, 2*131070). // TODO: For some reason unlimited memory slows down the eval_ctx_new().
		WithPoolSize(1).
		Init()

	b.ReportAllocs()
	b.ResetTimer()

	ctx := context.Background()
	var input interface{} = make(map[string]interface{})

	for i := 0; i < b.N; i++ {
		if _, err := instance.Eval(ctx, opa.EvalOpts{Input: &input}); err != nil {
			panic(err)
		}
	}
}

func BenchmarkGoRego(b *testing.B) {
	pq := compileRego(`package p

a = true`, "data.p.a = x")

	b.ReportAllocs()
	b.ResetTimer()

	ctx := context.Background()
	input := make(map[string]interface{})

	for i := 0; i < b.N; i++ {
		if _, err := pq.Eval(ctx, rego.EvalInput(input)); err != nil {
			panic(err)
		}
	}
}

func BenchmarkWASMArrayIteration(b *testing.B) {
	sizes := []int{10, 100, 1000, 10000}
	for _, n := range sizes {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			benchmarkIteration(b, test.ArrayIterationBenchmarkModule(n))
		})
	}
}

func BenchmarkWASMSetIteration(b *testing.B) {
	sizes := []int{10, 100, 1000, 10000}
	for _, n := range sizes {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			benchmarkIteration(b, test.SetIterationBenchmarkModule(n))
		})
	}
}

func BenchmarkWASMObjectIteration(b *testing.B) {
	sizes := []int{10, 100, 1000, 10000}
	for _, n := range sizes {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			benchmarkIteration(b, test.ObjectIterationBenchmarkModule(n))
		})
	}
}

var r *opa.Result

func benchmarkIteration(b *testing.B, module string) {
	query := "data.test.main = x"
	policy := compileRegoToWasm(module, query, false)

	instance, err := opa.New().
		WithPolicyBytes(policy).
		WithMemoryLimits(2*wasm.PageSize, 47*wasm.PageSize).
		WithPoolSize(1).
		Init()
	if err != nil {
		b.Fatalf("init sdk: %v", err)
	}

	b.ResetTimer()
	ctx := context.Background()
	var input interface{} = make(map[string]interface{})

	for i := 0; i < b.N; i++ {
		r, err = instance.Eval(ctx, opa.EvalOpts{Input: &input})
		if err != nil {
			b.Fatalf("Unexpected query error: %v", err)
		}
		if string(r.Result) != `{{"x":true}}` {
			b.Errorf("unexpected result: %s", string(r.Result))
		}
	}
}

func BenchmarkWASMLargeJSON(b *testing.B) {
	for _, kv := range []struct{ key, val int }{
		{10, 10},
		{10, 100},
		{10, 1000},
		{10, 10000},
		{100, 100},
		{100, 1000},
	} {
		b.Run(fmt.Sprintf("%dx%d", kv.key, kv.val), func(b *testing.B) {
			ctx := context.Background()
			data := test.GenerateJSONBenchmarkData(kv.key, kv.val)

			// Read data.values N times inside query.
			query := "data.keys[_] = x; data.values = y"
			policy := compileRegoToWasm("", query, false)

			instance, err := opa.New().
				WithPolicyBytes(policy).
				WithDataJSON(data).
				WithMemoryLimits(200*wasm.PageSize, 600*wasm.PageSize). // This is rather much
				WithPoolSize(1).
				Init()
			if err != nil {
				b.Fatalf("init sdk: %v", err)
			}

			b.ResetTimer()
			var input interface{} = make(map[string]interface{})

			for i := 0; i < b.N; i++ {
				r, err = instance.Eval(ctx, opa.EvalOpts{Input: &input})
				if err != nil {
					b.Fatalf("Unexpected query error: %v", err)
				}
			}
		})
	}
}

func BenchmarkWASMVirtualDocs(b *testing.B) {
	for _, kv := range []struct{ total, hit int }{
		{1, 1},
		{10, 1},
		{100, 1},
		{1000, 1},
		{10, 10},
		{100, 10},
		{1000, 10},
		{100, 100},
		{1000, 100},
		{1000, 1000},
	} {
		b.Run(fmt.Sprintf("total=%d/hit=%d", kv.total, kv.hit), func(b *testing.B) {
			runVirtualDocsBenchmark(b, kv.total, kv.hit)
		})
	}
}

func runVirtualDocsBenchmark(b *testing.B, numTotalRules, numHitRules int) {
	ctx := context.Background()
	module, input := test.GenerateVirtualDocsBenchmarkData(numTotalRules, numHitRules)
	query := "data.a.b.c.allow = x"

	policy := compileRegoToWasm(module, query, false)

	instance, err := opa.New().
		WithPolicyBytes(policy).
		WithMemoryLimits(8*wasm.PageSize, 8*wasm.PageSize).
		WithPoolSize(1).
		Init()
	if err != nil {
		b.Fatalf("init sdk: %v", err)
	}

	b.ResetTimer()
	var inp interface{} = input

	for i := 0; i < b.N; i++ {
		r, err = instance.Eval(ctx, opa.EvalOpts{Input: &inp})
		if err != nil {
			b.Fatalf("Unexpected query error: %v", err)
		}
	}
}
