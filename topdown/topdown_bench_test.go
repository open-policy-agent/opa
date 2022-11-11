// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"bytes"
	"context"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"testing"
	"text/template"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/metrics"
	"github.com/open-policy-agent/opa/storage"
	inmem "github.com/open-policy-agent/opa/storage/inmem/test"
	"github.com/open-policy-agent/opa/util/test"
)

func BenchmarkArrayIteration(b *testing.B) {
	sizes := []int{10, 100, 1000, 10000}
	for _, n := range sizes {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			benchmarkIteration(b, test.ArrayIterationBenchmarkModule(n))
		})
	}
}

func BenchmarkArrayPlugging(b *testing.B) {
	ctx := context.Background()

	sizes := []int{10, 100, 1000, 10000}

	for _, n := range sizes {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			data := make([]interface{}, n)
			for i := 0; i < n; i++ {
				data[i] = fmt.Sprintf("whatever%d", i)
			}
			store := inmem.NewFromObject(map[string]interface{}{"fixture": data})
			module := `package test
			fixture := data.fixture
			main { x := fixture }`

			query := ast.MustParseBody("data.test.main")
			compiler := ast.MustCompileModules(map[string]string{
				"test.rego": module,
			})

			b.ResetTimer()

			for i := 0; i < b.N; i++ {

				err := storage.Txn(ctx, store, storage.TransactionParams{}, func(txn storage.Transaction) error {

					q := NewQuery(query).
						WithCompiler(compiler).
						WithStore(store).
						WithTransaction(txn)

					_, err := q.Run(ctx)
					if err != nil {
						return err
					}

					return nil
				})

				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkSetIteration(b *testing.B) {
	sizes := []int{10, 100, 1000, 10000}
	for _, n := range sizes {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			benchmarkIteration(b, test.SetIterationBenchmarkModule(n))
		})
	}
}

func BenchmarkObjectIteration(b *testing.B) {
	sizes := []int{10, 100, 1000, 10000}
	for _, n := range sizes {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			benchmarkIteration(b, test.ObjectIterationBenchmarkModule(n))
		})
	}
}

func benchmarkIteration(b *testing.B, module string) {
	ctx := context.Background()
	query := ast.MustParseBody("data.test.main")
	compiler := ast.MustCompileModules(map[string]string{
		"test.rego": module,
	})

	b.ResetTimer()

	for i := 0; i < b.N; i++ {

		q := NewQuery(query).WithCompiler(compiler)
		_, err := q.Run(ctx)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkLargeJSON(b *testing.B) {
	data := test.GenerateLargeJSONBenchmarkData()
	ctx := context.Background()
	store := inmem.NewFromObject(data)
	compiler := ast.NewCompiler()

	if compiler.Compile(nil); compiler.Failed() {
		b.Fatal(compiler.Errors)
	}

	b.ResetTimer()

	// Read data.values N times inside query.
	query := ast.MustParseBody("data.keys[_] = x; data.values = y")

	for i := 0; i < b.N; i++ {

		err := storage.Txn(ctx, store, storage.TransactionParams{}, func(txn storage.Transaction) error {

			q := NewQuery(query).
				WithCompiler(compiler).
				WithStore(store).
				WithTransaction(txn)

			_, err := q.Run(ctx)
			if err != nil {
				return err
			}

			return nil
		})

		if err != nil {
			b.Fatal(err)
		}

	}
}

func BenchmarkConcurrency1(b *testing.B) {
	benchmarkConcurrency(b, getParams(1, 0))
}

func BenchmarkConcurrency2(b *testing.B) {
	benchmarkConcurrency(b, getParams(2, 0))
}

func BenchmarkConcurrency4(b *testing.B) {
	benchmarkConcurrency(b, getParams(4, 0))
}

func BenchmarkConcurrency8(b *testing.B) {
	benchmarkConcurrency(b, getParams(8, 0))
}

func BenchmarkConcurrency4Readers1Writer(b *testing.B) {
	benchmarkConcurrency(b, getParams(4, 1))
}

func BenchmarkConcurrency8Writers(b *testing.B) {
	benchmarkConcurrency(b, getParams(0, 8))
}

func benchmarkConcurrency(b *testing.B, params []storage.TransactionParams) {

	mod, data := test.GenerateConcurrencyBenchmarkData()
	ctx := context.Background()
	store := inmem.NewFromObject(data)
	mods := map[string]*ast.Module{"module": ast.MustParseModule(mod)}
	compiler := ast.NewCompiler()

	if compiler.Compile(mods); compiler.Failed() {
		b.Fatalf("Unexpected compiler error: %v", compiler.Errors)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		wg := new(sync.WaitGroup)
		queriesPerCore := 1000 / len(params)
		for j := 0; j < len(params); j++ {
			param := params[j] // capture j'th params before goroutine
			wg.Add(1)
			go func() {
				defer wg.Done()
				for k := 0; k < queriesPerCore; k++ {
					txn := storage.NewTransactionOrDie(ctx, store, param)
					query := NewQuery(ast.MustParseBody("data.test.p = x")).
						WithCompiler(compiler).
						WithStore(store).
						WithTransaction(txn)
					rs, err := query.Run(ctx)
					if err != nil {
						b.Errorf("Unexpected topdown query error: %v", err)
						return
					}
					if len(rs) != 1 || !rs[0][ast.Var("x")].Equal(ast.BooleanTerm(true)) {
						b.Errorf("Unexpected undefined/extra/bad result: %v", rs)
						return
					}
					store.Abort(ctx, txn)
				}
			}()
		}

		wg.Wait()
	}
}

func getParams(nReaders, nWriters int) (sl []storage.TransactionParams) {
	for i := 0; i < nReaders; i++ {
		sl = append(sl, storage.TransactionParams{})
	}
	for i := 0; i < nWriters; i++ {
		sl = append(sl, storage.WriteParams)
	}
	return sl
}

func BenchmarkVirtualDocs1x1(b *testing.B) {
	runVirtualDocsBenchmark(b, 1, 1)
}

func BenchmarkVirtualDocs10x1(b *testing.B) {
	runVirtualDocsBenchmark(b, 10, 1)
}

func BenchmarkVirtualDocs100x1(b *testing.B) {
	runVirtualDocsBenchmark(b, 100, 1)
}

func BenchmarkVirtualDocs1000x1(b *testing.B) {
	runVirtualDocsBenchmark(b, 1000, 1)
}

func BenchmarkVirtualDocs10x10(b *testing.B) {
	runVirtualDocsBenchmark(b, 10, 10)
}

func BenchmarkVirtualDocs100x10(b *testing.B) {
	runVirtualDocsBenchmark(b, 100, 10)
}

func BenchmarkVirtualDocs1000x10(b *testing.B) {
	runVirtualDocsBenchmark(b, 1000, 10)
}

func BenchmarkVirtualDocs100x100(b *testing.B) {
	runVirtualDocsBenchmark(b, 100, 100)
}

func BenchmarkVirtualDocs1000x100(b *testing.B) {
	runVirtualDocsBenchmark(b, 1000, 100)
}

func BenchmarkVirtualDocs1000x1000(b *testing.B) {
	runVirtualDocsBenchmark(b, 1000, 1000)
}

func runVirtualDocsBenchmark(b *testing.B, numTotalRules, numHitRules int) {

	mod, inp := test.GenerateVirtualDocsBenchmarkData(numTotalRules, numHitRules)
	ctx := context.Background()
	compiler := ast.NewCompiler()
	mods := map[string]*ast.Module{"module": ast.MustParseModule(mod)}
	input := ast.NewTerm(ast.MustInterfaceToValue(inp))
	store := inmem.New()
	txn := storage.NewTransactionOrDie(ctx, store)
	if compiler.Compile(mods); compiler.Failed() {
		b.Fatalf("Unexpected compiler error: %v", compiler.Errors)
	}

	query := ast.MustParseBody("data.a.b.c.allow = x")

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		b.StopTimer()

		query := NewQuery(query).
			WithCompiler(compiler).
			WithStore(store).
			WithTransaction(txn).
			WithInput(input)

		b.StartTimer()

		rs, err := query.Run(ctx)
		if err != nil {
			b.Fatalf("Unexpected topdown query error: %v", err)
		}
		if len(rs) != 1 || !rs[0][ast.Var("x")].Equal(ast.BooleanTerm(true)) {
			b.Fatalf("Unexpected undefined/extra/bad result: %v", rs)
		}
	}
}

func BenchmarkPartialEval(b *testing.B) {
	sizes := []int{1, 10, 100, 1000}
	for _, n := range sizes {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			runPartialEvalBenchmark(b, n)
		})
	}
}

func BenchmarkPartialEvalCompile(b *testing.B) {
	sizes := []int{1, 10, 100, 1000}
	for _, n := range sizes {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			runPartialEvalCompileBenchmark(b, n)
		})
	}
}

func runPartialEvalBenchmark(b *testing.B, numRoles int) {

	ctx := context.Background()
	compiler := ast.NewCompiler()
	compiler.Compile(map[string]*ast.Module{
		"authz": ast.MustParseModule(partialEvalBenchmarkPolicy),
	})

	if compiler.Failed() {
		b.Fatal(compiler.Errors)
	}

	var partials []ast.Body
	var support []*ast.Module
	data := generatePartialEvalBenchmarkData(numRoles)
	store := inmem.NewFromObject(data)

	err := storage.Txn(ctx, store, storage.TransactionParams{}, func(txn storage.Transaction) error {
		query := NewQuery(ast.MustParseBody("data.authz.allow = true")).
			WithUnknowns([]*ast.Term{ast.MustParseTerm("input")}).
			WithCompiler(compiler).
			WithStore(store).
			WithTransaction(txn)
		var err error
		partials, support, err = query.PartialRun(ctx)
		return err
	})
	if err != nil {
		b.Fatal(err)
	}

	if len(partials) != numRoles {
		b.Fatal("Expected exactly one partial query result but got:", partials)
	} else if len(support) != 0 {
		b.Fatal("Expected no partial support results but got:", support)
	}

	module := ast.MustParseModule(`package partial.authz`)

	for _, query := range partials {
		rule := &ast.Rule{
			Head:   ast.NewHead(ast.Var("allow"), nil, ast.BooleanTerm(true)),
			Body:   query,
			Module: module,
		}
		module.Rules = append(module.Rules, rule)
	}

	compiler = ast.NewCompiler()
	compiler.Compile(map[string]*ast.Module{
		"partial": module,
	})
	if compiler.Failed() {
		b.Fatal(compiler.Errors)
	}

	input := generatePartialEvalBenchmarkInput(numRoles)

	err = storage.Txn(ctx, store, storage.TransactionParams{}, func(txn storage.Transaction) error {
		query := NewQuery(ast.MustParseBody("data.partial.authz.allow = true")).
			WithCompiler(compiler).
			WithStore(store).
			WithTransaction(txn).
			WithInput(input)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			qrs, err := query.Run(ctx)
			if len(qrs) != 1 || err != nil {
				b.Fatal("Unexpected query result:", qrs, "err:", err)
			}
		}
		return nil
	})
	if err != nil {
		b.Fatal(err)
	}
}

func runPartialEvalCompileBenchmark(b *testing.B, numRoles int) {

	ctx := context.Background()
	data := generatePartialEvalBenchmarkData(numRoles)
	store := inmem.NewFromObject(data)

	err := storage.Txn(ctx, store, storage.TransactionParams{}, func(txn storage.Transaction) error {

		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			// compile original policy
			compiler := ast.NewCompiler()
			compiler.Compile(map[string]*ast.Module{
				"authz": ast.MustParseModule(partialEvalBenchmarkPolicy),
			})
			if compiler.Failed() {
				return compiler.Errors
			}

			// run partial evaluation
			var partials []ast.Body
			var support []*ast.Module
			query := NewQuery(ast.MustParseBody("data.authz.allow = true")).
				WithUnknowns([]*ast.Term{ast.MustParseTerm("input")}).
				WithCompiler(compiler).
				WithStore(store).
				WithTransaction(txn)
			var err error
			partials, support, err = query.PartialRun(ctx)
			if err != nil {
				return err
			}

			if len(partials) != numRoles {
				b.Fatal("Expected exactly one partial query result but got:", partials)
			} else if len(support) != 0 {
				b.Fatal("Expected no partial support results but got:", support)
			}

			// recompile output
			module := ast.MustParseModule(`package partial.authz`)

			for _, query := range partials {
				rule := &ast.Rule{
					Head:   ast.NewHead(ast.Var("allow"), nil, ast.BooleanTerm(true)),
					Body:   query,
					Module: module,
				}
				module.Rules = append(module.Rules, rule)
			}

			compiler = ast.NewCompiler()
			compiler.Compile(map[string]*ast.Module{
				"test": module,
			})

			if compiler.Failed() {
				b.Fatal(compiler.Errors)
			}
		}

		return nil
	})

	if err != nil {
		b.Fatal(err)
	}
}

const partialEvalBenchmarkPolicy = `package authz

	default allow = false

	allow {
		user_has_role[role_name]
		role_has_permission[role_name]
	}

	user_has_role[role_name] {
		data.bindings[_] = binding
		binding.iss = input.iss
		binding.group = input.group
		role_name = binding.role
	}

	role_has_permission[role_name] {
		data.roles[_] = role
		role.name = role_name
		role.operation = input.operation
		role.resource = input.resource
	}
	`

func generatePartialEvalBenchmarkData(numRoles int) map[string]interface{} {
	roles := make([]interface{}, numRoles)
	bindings := make([]interface{}, numRoles)
	for i := 0; i < numRoles; i++ {
		role := map[string]interface{}{
			"name":      fmt.Sprintf("role-%d", i),
			"operation": fmt.Sprintf("operation-%d", i),
			"resource":  fmt.Sprintf("resource-%d", i),
		}
		roles[i] = role
		binding := map[string]interface{}{
			"name":  fmt.Sprintf("binding-%d", i),
			"iss":   fmt.Sprintf("iss-%d", i),
			"group": fmt.Sprintf("group-%d", i),
			"role":  role["name"],
		}
		bindings[i] = binding
	}
	return map[string]interface{}{
		"roles":    roles,
		"bindings": bindings,
	}
}

func generatePartialEvalBenchmarkInput(numRoles int) *ast.Term {

	tmpl, err := template.New("Test").Parse(`{
		"operation": "operation-{{ . }}",
		"resource": "resource-{{ . }}",
		"iss": "iss-{{ . }}",
		"group": "group-{{ . }}"
	}`)
	if err != nil {
		panic(err)
	}

	var buf bytes.Buffer

	err = tmpl.Execute(&buf, numRoles-1)
	if err != nil {
		panic(err)
	}

	return ast.MustParseTerm(buf.String())
}

func BenchmarkWalk(b *testing.B) {

	ctx := context.Background()
	sizes := []int{100, 1000, 2000, 3000}

	for _, n := range sizes {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			data := genWalkBenchmarkData(n)
			store := inmem.NewFromObject(data)
			compiler := ast.NewCompiler()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				err := storage.Txn(ctx, store, storage.TransactionParams{}, func(txn storage.Transaction) error {
					query := ast.MustParseBody(fmt.Sprintf(`walk(data, [["arr", %v], x])`, n-1))
					compiledQuery, err := compiler.QueryCompiler().Compile(query)
					if err != nil {
						b.Fatal(err)
					}
					q := NewQuery(compiledQuery).
						WithStore(store).
						WithCompiler(compiler).
						WithTransaction(txn)
					rs, err := q.Run(ctx)
					if err != nil || len(rs) != 1 || !rs[0][ast.Var("x")].Equal(ast.IntNumberTerm(n-1)) {
						b.Fatal("Unexpected result:", rs, "err:", err)
					}
					return nil
				})
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}

}

func genWalkBenchmarkData(n int) map[string]interface{} {
	sl := make([]interface{}, n)
	for i := 0; i < n; i++ {
		sl[i] = i
	}
	return map[string]interface{}{
		"arr": sl,
	}
}

func BenchmarkComprehensionIndexing(b *testing.B) {
	ctx := context.Background()
	cases := []struct {
		note   string
		module string
		query  string
	}{
		{
			note: "arrays",
			module: `
				package test

				bench_array {
					v := data.items[_]
					ks := [k | some k; v == data.items[k]]
				}
			`,
			query: `data.test.bench_array = true`,
		},
		{
			note: "sets",
			module: `
				package test

				bench_set {
					v := data.items[_]
					ks := {k | some k; v == data.items[k]}
				}
			`,
			query: `data.test.bench_set = true`,
		},
		{
			note: "objects",
			module: `
				package test

				bench_object {
					v := data.items[_]
					ks := {k: 1 | some k; v == data.items[k]}
				}
			`,
			query: `data.test.bench_object = true`,
		},
	}

	sizes := []int{10, 100, 1000}
	for _, tc := range cases {
		for _, n := range sizes {
			b.Run(fmt.Sprintf("%v_%v", tc.note, n), func(b *testing.B) {
				data := genComprehensionIndexingData(n)
				store := inmem.NewFromObject(data)
				compiler := ast.MustCompileModules(map[string]string{
					"test.rego": tc.module,
				})
				query, err := compiler.QueryCompiler().Compile(ast.MustParseBody(tc.query))
				if err != nil {
					b.Fatal(err)
				}
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					err = storage.Txn(ctx, store, storage.TransactionParams{}, func(txn storage.Transaction) error {
						m := metrics.New()
						instr := NewInstrumentation(m)
						q := NewQuery(query).WithStore(store).WithCompiler(compiler).WithTransaction(txn).WithInstrumentation(instr)
						rs, err := q.Run(ctx)
						if m.Counter(evalOpComprehensionCacheMiss).Value().(uint64) > 0 {
							b.Fatal("expected zero cache misses")
						}
						if err != nil || len(rs) != 1 {
							b.Fatal("Unexpected result:", rs, "err:", err)
						}
						return nil
					})
					if err != nil {
						b.Fatal(err)
					}

				}
			})
		}
	}
}

func BenchmarkFunctionArgumentIndex(b *testing.B) {
	ctx := context.Background()

	sizes := []int{10, 100, 1000}

	for _, n := range sizes {
		compiler := ast.MustCompileModules(map[string]string{
			"test.rego": moduleWithDefs(n),
		})
		body := ast.MustParseBody(fmt.Sprintf("data.test.f(%d, x)", n))

		b.Run(fmt.Sprint(n), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				q := NewQuery(body).
					WithCompiler(compiler).
					WithIndexing(true)

				res, err := q.Run(ctx)
				if err != nil {
					b.Fatal(err)
				}

				if len(res) != 1 {
					b.Fatalf("Expected one result, got %d", len(res))
				}
				if !ast.Boolean(true).Equal(res[0][ast.Var("x")].Value) {
					b.Errorf("expected x=>true, got %v", res[0])
				}
			}
		})
	}
}

func moduleWithDefs(n int) string {
	var b strings.Builder

	b.WriteString(`package test
`)
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&b, `f(x) = y { y := true; x == %[1]d }
`, i)
	}
	return b.String()
}

func genComprehensionIndexingData(n int) map[string]interface{} {
	items := map[string]interface{}{}
	for i := 0; i < n; i++ {
		items[fmt.Sprint(i)] = fmt.Sprint(i)
	}
	return map[string]interface{}{"items": items}
}

func BenchmarkObjectSubset(b *testing.B) {
	ctx := context.Background()

	sizes := []int{10, 100, 1000, 10000}

	for _, n := range sizes {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			all := make(map[string]string)
			evens := make(map[string]string)

			for i := 0; i < n; i++ {
				all[fmt.Sprint(i)] = fmt.Sprint(i * 2)
				if i%2 == 0 {
					evens[fmt.Sprint(i)] = fmt.Sprint(i * 2)
				}
			}

			store := inmem.NewFromObject(map[string]interface{}{"all": all, "evens": evens})

			module := `package test
			main {object.subset(data.all, data.evens)}`

			query := ast.MustParseBody("data.test.main")
			compiler := ast.MustCompileModules(map[string]string{
				"test.rego": module,
			})

			b.ResetTimer()

			for i := 0; i < b.N; i++ {

				err := storage.Txn(ctx, store, storage.TransactionParams{}, func(txn storage.Transaction) error {

					q := NewQuery(query).
						WithCompiler(compiler).
						WithStore(store).
						WithTransaction(txn)

					_, err := q.Run(ctx)
					if err != nil {
						return err
					}

					return nil
				})

				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkObjectSubsetSlow(b *testing.B) {
	// This benchmarks the suggested means to implement object.subset
	// without using the builtin, to give us an idea of whether or not
	// the builtin is actually making things any faster.
	ctx := context.Background()

	sizes := []int{10, 100, 1000, 10000}

	for _, n := range sizes {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			all := make(map[string]string)
			evens := make(map[string]string)

			for i := 0; i < n; i++ {
				all[fmt.Sprint(i)] = fmt.Sprint(i * 2)
				if i%2 == 0 {
					evens[fmt.Sprint(i)] = fmt.Sprint(i * 2)
				}
			}

			store := inmem.NewFromObject(map[string]interface{}{"all": all, "evens": evens})

			// Code is lifted from here:
			// https://github.com/open-policy-agent/opa/issues/4358#issue-1141145857

			module := `package test
			path_matches[match] {
			    [path, value] := walk(data.evens)
			    not is_object(value)

			    match := object.get(data.all, path, null) == value
			}

			main { path_matches == {true} }`

			query := ast.MustParseBody("data.test.main")
			compiler := ast.MustCompileModules(map[string]string{
				"test.rego": module,
			})

			b.ResetTimer()

			for i := 0; i < b.N; i++ {

				err := storage.Txn(ctx, store, storage.TransactionParams{}, func(txn storage.Transaction) error {

					q := NewQuery(query).
						WithCompiler(compiler).
						WithStore(store).
						WithTransaction(txn)

					_, err := q.Run(ctx)
					if err != nil {
						return err
					}

					return nil
				})

				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// randomString constructs and returns a string of the specified length
// containing a random assortment of characters from the given symbols list.
func randomString(symbols []rune, length int) string {
	builder := strings.Builder{}
	for i := 0; i < length; i++ {
		builder.WriteRune(symbols[rand.Intn(len(symbols))])
	}
	return builder.String()
}

func BenchmarkGlob(b *testing.B) {
	ctx := context.Background()

	// Benchmark Strategy:
	//
	// We want to test both matching and non-matching, in case one is
	// slower than the other. We also want to test a variety of different
	// patterns, since (at time of writing), there is a pattern cache, so
	// we want to exercise it but not rely on it excessively.
	//
	// For each individual test case, we generate size/2 random strings
	// from the letters a,b,c,d as well as size/2 with those letters plus
	// x. We test matching with the following globs:
	//
	// * "*x*"  (should always have matches)
	// * "*y*"  (should never have matches)
	// * the first half of a randomly chosen test case, followed by *

	sizes := []int{10, 100, 1000}
	length := 32

	for _, n := range sizes {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			haystack := make([]string, n)
			for i := 0; i < n; i++ {
				if i%2 == 0 {
					haystack[i] = randomString([]rune{'a', 'b', 'c', 'd'}, length)
				} else {
					haystack[i] = randomString([]rune{'a', 'b', 'c', 'd', 'x'}, length)
				}
			}

			needleIndex := rand.Intn(len(haystack))
			needle := haystack[needleIndex]
			needleGlob := needle[0:length/2] + "*"

			store := inmem.NewFromObject(map[string]interface{}{
				"haystack":   haystack,
				"needleGlob": needleGlob,
			})

			module := `package test
			main {
				needleMatches := {h | h := data.haystack[_]; glob.match(data.needleGlob, [], h)}
				xMatches := {h | h := data.haystack[_]; glob.match("*x*", [], h)}
				yMtches := {h | h := data.haystack[_]; glob.match("*y*", [], h)}
			}`

			query := ast.MustParseBody("data.test.main")
			compiler := ast.MustCompileModules(map[string]string{
				"test.rego": module,
			})

			b.ResetTimer()

			for i := 0; i < b.N; i++ {

				err := storage.Txn(ctx, store, storage.TransactionParams{}, func(txn storage.Transaction) error {

					q := NewQuery(query).
						WithCompiler(compiler).
						WithStore(store).
						WithTransaction(txn)

					_, err := q.Run(ctx)
					if err != nil {
						return err
					}

					return nil
				})

				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkMemberWithKeyFromBaseDoc(b *testing.B) {
	store := inmem.NewFromObject(test.GenerateLargeJSONBenchmarkData())
	mod := `package test
	import future.keywords.in
	main { "key99", "value99" in data.values }
	`

	ctx := context.Background()
	query := ast.MustParseBody("data.test.main")
	compiler := ast.MustCompileModules(map[string]string{
		"test.rego": mod,
	})

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		err := storage.Txn(ctx, store, storage.TransactionParams{}, func(txn storage.Transaction) error {
			_, err := NewQuery(query).WithCompiler(compiler).WithStore(store).WithTransaction(txn).Run(ctx)
			return err
		})
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkObjectGetFromBaseDoc(b *testing.B) {
	store := inmem.NewFromObject(test.GenerateLargeJSONBenchmarkData())
	mod := `package test
	main { object.get(data.values, "key99", false) == "value99" }
	`

	ctx := context.Background()
	query := ast.MustParseBody("data.test.main")
	compiler := ast.MustCompileModules(map[string]string{
		"test.rego": mod,
	})

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		err := storage.Txn(ctx, store, storage.TransactionParams{}, func(txn storage.Transaction) error {
			_, err := NewQuery(query).WithCompiler(compiler).WithStore(store).WithTransaction(txn).Run(ctx)
			return err
		})
		if err != nil {
			b.Fatal(err)
		}
	}
}
