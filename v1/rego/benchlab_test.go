// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

//go:build opa_wasm

// Curated subset of this package's benchmarks for the nightly benchlab run.
// See build/bench-nightly/set.json.
//
// Every case costs the same fixed slice of wall clock, whatever it measures.
// The *Targets families each measure their shape twice, once per evaluation
// target, and that pair is the reason to run them here: nothing else tracks
// topdown against wasm through the public API. The four size sweeps are left
// out, since the topdown and ast shards already follow how eval scales.

package rego

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/metrics"
	"github.com/open-policy-agent/opa/v1/runtime/info"
	"github.com/open-policy-agent/opa/v1/storage/inmem"
	inmemtest "github.com/open-policy-agent/opa/v1/storage/inmem/test"
	"github.com/open-policy-agent/opa/v1/util/test"
)

// --- both evaluation targets -------------------------------------------

// The per-eval fixed cost, on each target.
func BenchmarkBenchlabTrivialPolicyTargets(b *testing.B) {
	BenchmarkTrivialPolicyTargets(b)
}

// The closest thing here to a real workload.
func BenchmarkBenchlabSimpleAuthzTargets(b *testing.B) {
	BenchmarkSimpleAuthzTargets(b)
}

func BenchmarkBenchlabArrayIterationTargets(b *testing.B) {
	BenchmarkArrayIterationTargets(b)
}

func BenchmarkBenchlabBuiltinPerformanceTargets(b *testing.B) {
	BenchmarkBuiltinPerformanceTargets(b)
}

// Rego-to-wasm compilation and first-eval cost, which users pay on startup
// rather than per request.
func BenchmarkBenchlabWASMCompilationTargets(b *testing.B) {
	BenchmarkWASMCompilationTargets(b)
}

func BenchmarkBenchlabWASMColdStartTargets(b *testing.B) {
	BenchmarkWASMColdStartTargets(b)
}

// --- the public API on realistic policies -------------------------------

func BenchmarkBenchlabAciTestOnlyEval(b *testing.B) {
	BenchmarkAciTestOnlyEval(b)
}

// Cross-module partial object rules, at the size where the cost is visible.
func BenchmarkBenchlabPartialObjectRuleCrossModule(b *testing.B) {
	const n = 1000
	ctx := b.Context()

	b.Run(strconv.Itoa(n), func(b *testing.B) {
		mods := test.PartialObjectBenchmarkCrossModule(n)
		compiler := ast.MustCompileModules(map[string]string{
			"test/foo.rego": mods[0],
			"test/bar.rego": mods[1],
			"test/baz.rego": mods[2],
		})

		input := make(map[string]any)
		for idx := range 4 {
			input[fmt.Sprintf("test_input_%d", idx)] = "test_input_10"
		}
		inputAST, err := ast.InterfaceToValue(input)
		if err != nil {
			b.Fatal(err)
		}

		runtimeInfo, err := info.New()
		if err != nil {
			b.Fatal(err)
		}

		pq, err := New(
			Query("data.test.foo"),
			Compiler(compiler),
			Store(inmemtest.NewFromObject(map[string]any{})),
			Runtime(runtimeInfo),
		).PrepareForEval(ctx)
		if err != nil {
			b.Fatal(err)
		}

		b.ResetTimer()
		for b.Loop() {
			if _, err := pq.Eval(ctx,
				EvalParsedInput(inputAST),
				EvalRuleIndexing(true),
				EvalEarlyExit(true),
			); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// The rule index through the public API, at the larger of the two group counts.
func BenchmarkBenchlabIndexedRulesetEval(b *testing.B) {
	const n = 500
	ctx := b.Context()

	b.Run(fmt.Sprintf("groups=%d", n), func(b *testing.B) {
		module, data := indexedRuleset(n, 20)

		pq, err := New(
			ParsedQuery(ast.MustParseBody("data.test.allow = x")),
			ParsedModule(ast.MustParseModule(module)),
			Store(inmem.NewFromObject(data)),
			GenerateJSON(noOpGenerateJSON),
		).PrepareForEval(ctx)
		if err != nil {
			b.Fatal(err)
		}

		input := ast.MustParseTerm(fmt.Sprintf(`{"subject": "u%d_19", "resource": {"foo": "A"}}`, n-1))

		b.ResetTimer()
		for b.Loop() {
			if _, err := pq.Eval(ctx, EvalParsedInput(input.Value)); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// Whether a reference is resolved once outside the loop or on every iteration;
// the two arms are only meaningful together.
func BenchmarkBenchlabGlobalVsLocalLookup(b *testing.B) {
	BenchmarkGlobalVsLocalLookup(b)
}

func BenchmarkBenchlabAggregatedLabels(b *testing.B) {
	BenchmarkAggregatedLabels(b)
}

// --- store and input ----------------------------------------------------

func BenchmarkBenchlabStoreRead(b *testing.B) {
	BenchmarkStoreRead(b)
}

func BenchmarkBenchlabStoreRefNotFound(b *testing.B) {
	BenchmarkStoreRefNotFound(b)
}

// Raw input parsing, which every request pays before eval begins. The sweep is
// linear in the leaf count, so the largest point stands in for it.
func BenchmarkBenchlabParseRawInput(b *testing.B) {
	const n = 10000
	r := &Rego{}
	input := benchNativeInputTree(n)

	b.Run(fmt.Sprintf("leaves=%d", n), func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			v := any(input)
			if _, err := r.parseRawInput(&v, metrics.New()); err != nil {
				b.Fatal(err)
			}
		}
	})
}
