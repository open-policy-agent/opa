package rego

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/open-policy-agent/opa/internal/runtime"
	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/loader"
	inmem "github.com/open-policy-agent/opa/v1/storage/inmem/test"
	"github.com/open-policy-agent/opa/v1/util/test"
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

			pq, err := New(
				Query(query),
				Compiler(compiler),
				Store(store),
				Runtime(info),
			).PrepareForEval(ctx)

			if err != nil {
				b.Fatal(err)
			}

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				_, err = pq.Eval(
					ctx,
					EvalParsedInput(inputAST),
					EvalRuleIndexing(true),
					EvalEarlyExit(true),
				)

				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkCustomFunctionInHotPath(b *testing.B) {
	ctx := context.Background()
	input := ast.MustParseTerm(mustReadFileAsString(b, "testdata/ast.rego"))
	module := ast.MustParseModule(`package test

	import rego.v1

	r := count(refs)

	refs contains value if {
		walk(input, [_, value])
		is_ref(value)
	}

	is_ref(value) if value.type == "ref"
	is_ref(value) if value[0].type == "ref"`)

	r := New(Query("data.test.r = x"), ParsedModule(module))

	pq, err := r.PrepareForEval(ctx)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		res, err := pq.Eval(ctx, EvalParsedInput(input.Value))
		if err != nil {
			b.Fatal(err)
		}

		if res == nil {
			b.Fatal("expected result")
		}

		if res[0].Bindings["x"].(json.Number) != "402" {
			b.Fatalf("expected 402, got %v", res[0].Bindings["x"])
		}
	}
}

// Benchmarks of the ACI test data from Regorus
// https://github.com/microsoft/regorus?tab=readme-ov-file#performance

// BenchmarkAciTestBuildAndEval-10    37    30700209 ns/op    16437935 B/op    384211 allocs/op
func BenchmarkAciTestBuildAndEval(b *testing.B) {
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		bundle, err := loader.NewFileLoader().
			WithRegoVersion(ast.RegoV0).
			AsBundle("testdata/aci")
		if err != nil {
			b.Fatal(err)
		}

		input := ast.MustParseTerm(mustReadFileAsString(b, "testdata/aci/input.json"))

		r := New(Query("data.framework.mount_overlay = x"), ParsedBundle("", bundle))

		pq, err := r.PrepareForEval(ctx)
		if err != nil {
			b.Fatal(err)
		}

		res, err := pq.Eval(ctx, EvalParsedInput(input.Value))
		if err != nil {
			b.Fatal(err)
		}

		_ = res
	}
}

// BenchmarkAciTestOnlyEval-10    12752    92188 ns/op    50005 B/op    1062 allocs/op
func BenchmarkAciTestOnlyEval(b *testing.B) {
	ctx := context.Background()

	bundle, err := loader.NewFileLoader().
		WithRegoVersion(ast.RegoV0).
		AsBundle("testdata/aci")
	if err != nil {
		b.Fatal(err)
	}

	input := ast.MustParseTerm(mustReadFileAsString(b, "testdata/aci/input.json"))

	r := New(Query("data.framework.mount_overlay = x"), ParsedBundle("", bundle))

	pq, err := r.PrepareForEval(ctx)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {

		res, err := pq.Eval(ctx, EvalParsedInput(input.Value))
		if err != nil {
			b.Fatal(err)
		}

		_ = res
	}
}

func mustReadFileAsString(b *testing.B, path string) string {
	b.Helper()

	bs, err := os.ReadFile(path)
	if err != nil {
		b.Fatal(err)
	}

	return string(bs)
}
