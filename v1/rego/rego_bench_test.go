package rego

import (
	"fmt"
	"os"
	"strconv"
	"testing"

	"github.com/open-policy-agent/opa/internal/runtime"
	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/loader"
	"github.com/open-policy-agent/opa/v1/metrics"
	"github.com/open-policy-agent/opa/v1/storage"
	inmem "github.com/open-policy-agent/opa/v1/storage/inmem/test"
	"github.com/open-policy-agent/opa/v1/util/test"
)

func BenchmarkPartialObjectRuleCrossModule(b *testing.B) {
	ctx := b.Context()
	sizes := []int{10, 100, 1000}

	for _, n := range sizes {
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			store := inmem.NewFromObject(map[string]any{})
			mods := test.PartialObjectBenchmarkCrossModule(n)
			query := "data.test.foo"

			input := make(map[string]any)
			for idx := range 4 {
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

			for b.Loop() {
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

// Benchmarks of the ACI test data from Regorus
// https://github.com/microsoft/regorus?tab=readme-ov-file#performance

// BenchmarkAciTestBuildAndEval-10    37    30700209 ns/op    16437935 B/op    384211 allocs/op
// BenchmarkAciTestBuildAndEval-12    58    17566909 ns/op    15991409 B/op    304237 allocs/op
func BenchmarkAciTestBuildAndEval(b *testing.B) {
	ctx := b.Context()

	for b.Loop() {
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
// BenchmarkAciTestOnlyEval-10    13521	   86647 ns/op	  47448 B/op	 967 allocs/op // ref.CopyNonGround
// BenchmarkAciTestOnlyEval-12    21007	   57551 ns/op	  45323 B/op	 920 allocs/op
func BenchmarkAciTestOnlyEval(b *testing.B) {
	ctx := b.Context()

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

	for b.Loop() {
		res, err := pq.Eval(ctx, EvalParsedInput(input.Value))
		if err != nil {
			b.Fatal(err)
		}
		_ = res
	}
}

// BenchmarkArrayIteration-10
// 15574    77121 ns/op    67249 B/op    1115 allocs/op    // handleErr wrapping, not inlined
// 33862    35864 ns/op     5768 B/op      93 allocs/op    // handleErr only on error, inlined
func BenchmarkArrayIteration(b *testing.B) {
	ctx := b.Context()

	at := make([]*ast.Term, 512)
	for i := range 511 {
		at[i] = ast.StringTerm("a")
	}
	at[511] = ast.StringTerm("v")

	input := ast.NewObject(ast.Item(ast.StringTerm("foo"), ast.ArrayTerm(at...)))
	module := ast.MustParseModule(`package test

	default r := false

	r if input.foo[_] == "v"`)

	r := New(Query("data.test.r = x"), ParsedModule(module))

	pq, err := r.PrepareForEval(ctx)
	if err != nil {
		b.Fatal(err)
	}

	for b.Loop() {
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
}

// BenchmarkSetIteration-10
// 4800    272403 ns/op    80875 B/op    1193 allocs/op    // handleErr wrapping, not inlined
// 4933	   223234 ns/op	   76772 B/op	  681 allocs/op    // handleErr only on error, not inlined
func BenchmarkSetIteration(b *testing.B) {
	ctx := b.Context()

	at := make([]*ast.Term, 512)
	for i := range 512 {
		at[i] = ast.InternedIntegerString(i)
	}

	input := ast.NewObject(ast.Item(ast.StringTerm("foo"), ast.ArrayTerm(at...)))
	module := ast.MustParseModule(`package test

	s := {x | x := input.foo[_]}

	default r := false

	r if s[_] == "not found"`)

	r := New(Query("data.test.r = x"), ParsedModule(module))

	pq, err := r.PrepareForEval(ctx)
	if err != nil {
		b.Fatal(err)
	}

	for b.Loop() {
		res, err := pq.Eval(ctx, EvalParsedInput(input))
		if err != nil {
			b.Fatal(err)
		}
		if res == nil {
			b.Fatal("expected result")
		}
		if res[0].Bindings["x"].(bool) != false {
			b.Fatalf("expected false, got %v", res[0].Bindings["x"])
		}
	}
}

// BenchmarkObjectIteration-10
// 12067    99582 ns/op    72830 B/op   1126 allocs/op    // handleErr wrapping, not inlined
// 15358    85080 ns/op    27752 B/op    615 allocs/op    // handleErr only on error, not inlined
func BenchmarkObjectIteration(b *testing.B) {
	ctx := b.Context()

	at := make([][2]*ast.Term, 512)
	for i := range 512 {
		at[i] = ast.Item(ast.StringTerm(strconv.Itoa(i)), ast.StringTerm(strconv.Itoa(i)))
	}

	input := ast.NewObject(ast.Item(ast.StringTerm("foo"), ast.ObjectTerm(at...)))
	module := ast.MustParseModule(`package test

	default r := false

	r if {
		input.foo[_] == "512"
	}
		`)

	r := New(Query("data.test.r = x"), ParsedModule(module))

	pq, err := r.PrepareForEval(ctx)
	if err != nil {
		b.Fatal(err)
	}

	for b.Loop() {
		res, err := pq.Eval(ctx, EvalParsedInput(input))
		if err != nil {
			b.Fatal(err)
		}
		if res == nil {
			b.Fatal("expected result")
		}
		if res[0].Bindings["x"].(bool) != false {
			b.Fatalf("expected false, got %v", res[0].Bindings["x"])
		}
	}
}

// Comparing the cost of referencing not found data in Go vs. AST storage
//
// BenchmarkStoreRefNotFound/inmem-go-10         5208    212288 ns/op    160609 B/op     2936 allocs/op
// BenchmarkStoreRefNotFound/inmem-ast-10       13929     90053 ns/op     39614 B/op     1012 allocs/op
func BenchmarkStoreRefNotFound(b *testing.B) {
	ctx := b.Context()

	things := make(map[string]map[string]string, 100)
	for i := range 100 {
		things[strconv.Itoa(i)] = map[string]string{"foo": "bar"}
	}

	stores := map[string]storage.Store{
		"inmem-go":  inmem.NewFromObject(map[string]any{"things": things}),
		"inmem-ast": inmem.NewFromObjectWithASTRead(map[string]any{"things": things}),
	}
	policy := `package p

r contains true if {
	data.things[_].bar
}
`
	for name, store := range stores {
		b.Run(name, func(b *testing.B) {
			r := New(
				Query("data.p.r = x"),
				Store(store),
				ParsedModule(ast.MustParseModule(policy)),
				GenerateJSON(func(*ast.Term, *EvalContext) (any, error) {
					return nil, nil
				}),
			)

			pq, err := r.PrepareForEval(ctx)
			if err != nil {
				b.Fatal(err)
			}

			b.ResetTimer()

			for b.Loop() {
				res, err := pq.Eval(ctx)
				if err != nil {
					b.Fatal(err)
				}

				_ = res
			}
		})
	}
}

// 242.5 ns/op     168 B/op      7 allocs/op  // original implementation
// 176.7 ns/op      96 B/op      4 allocs/op  // sync.Pool in ptr.ValuePtr (saving 1 alloc/op per path part)
func BenchmarkStoreRead(b *testing.B) {
	ctx := b.Context()
	store := inmem.NewFromObjectWithASTRead(map[string]any{
		"foo": map[string]any{
			"bar": map[string]any{
				"baz": "qux",
			},
		},
	})

	txn, err := store.NewTransaction(ctx)
	if err != nil {
		b.Fatal(err)
	}

	ref := ast.MustParseRef("data.foo.bar.baz")

	for b.Loop() {
		// 1 alloc/op
		path, err := storage.NewPathForRef(ref)
		if err != nil {
			b.Fatal(err)
		}

		// 3 allocs/op (down from 6)
		// turns each string in path into a StringTerm only to use it
		// for a Get call in storage (ptr.ValuePtr)
		v, err := store.Read(ctx, txn, path)
		if err != nil {
			b.Fatal(err)
		}

		if v == nil {
			b.Fatal("expected value")
		}
	}
}

// 5730 ns/op	    5737 B/op	      93 allocs/op
// 5222 ns/op	    5639 B/op	      89 allocs/op // ref.CopyNonGround
// 2786 ns/op	    5090 B/op	      77 allocs/op // Lazy init improvements
func BenchmarkTrivialPolicy(b *testing.B) {
	ctx := b.Context()
	r := New(
		ParsedQuery(ast.MustParseBody("data.p.r = x")),
		ParsedModule(ast.MustParseModule(`package p
		r := 1`)),
		GenerateJSON(noOpGenerateJSON),
	)

	pq, err := r.PrepareForEval(ctx)
	if err != nil {
		b.Fatal(err)
	}

	for b.Loop() {
		if _, err := pq.Eval(ctx); err != nil {
			b.Fatal(err)
		}
	}
}

// 1851 ns/op       3376 B/op         53 allocs/op - first measurement
// 1312 ns/op	    2632 B/op	      38 allocs/op - lazy init targetStack, functionMockStack, comprehensionCache
// ------------------------------------------------- and move newResolverTrie call from NewQuery to WithResolver
// 1212 ns/op	    2568 B/op	      33 allocs/op - lazy init eval.Time
func BenchmarkTrivialQuery(b *testing.B) {
	m := metrics.New()
	r := New(ParsedQuery(ast.MustParseBody("1")), GenerateJSON(noOpGenerateJSON), Metrics(m))

	ctx := b.Context()

	pq, err := r.PrepareForEval(ctx)
	if err != nil {
		b.Fatal(err)
	}

	for b.Loop() {
		if _, err := pq.Eval(ctx, EvalMetrics(m)); err != nil {
			b.Fatal(err)
		}
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

func noOpGenerateJSON(*ast.Term, *EvalContext) (any, error) {
	return nil, nil
}

// 46168 ns/op	   14627 B/op	     496 allocs/op
// 25671 ns/op	   11488 B/op	     300 allocs/op
// ...
func BenchmarkGlobalVsLocalLookup(b *testing.B) {
	ctx := b.Context()

	module := ast.MustParseModule(`package p
global := 100

global_ref if {
	some i in numbers.range(1, 100)
	i == global
}

local_var if {
	local := global
	some i in numbers.range(1, 100)
	i == local
}`)

	q1 := ast.MustParseBody("data.p.global_ref = true")
	q2 := ast.MustParseBody("data.p.local_var = true")

	r1 := New(ParsedQuery(q1), ParsedModule(module), GenerateJSON(noOpGenerateJSON))
	r2 := New(ParsedQuery(q2), ParsedModule(module), GenerateJSON(noOpGenerateJSON))

	pq1, err := r1.PrepareForEval(ctx)
	if err != nil {
		b.Fatal(err)
	}

	pq2, err := r2.PrepareForEval(ctx)
	if err != nil {
		b.Fatal(err)
	}

	names := []string{"global_ref", "local_var"}

	for i, pq := range []PreparedEvalQuery{pq1, pq2} {
		b.Run(names[i], func(b *testing.B) {
			for b.Loop() {
				if _, err := pq.Eval(ctx); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
