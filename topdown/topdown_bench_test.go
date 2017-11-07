// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"bytes"
	"context"
	"sync"
	"testing"
	"text/template"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/storage/inmem"
	"github.com/open-policy-agent/opa/util"
)

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

	mod, data := generateConcurrencyBenchmarkData()
	ctx := context.Background()
	store := inmem.NewFromObject(data)
	mods := map[string]*ast.Module{"module": mod}
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
						b.Fatalf("Unexpected topdown query error: %v", err)
					}
					if len(rs) != 1 || !rs[0][ast.Var("x")].Equal(ast.BooleanTerm(true)) {
						b.Fatalf("Unexpected undefined/extra/bad result: %v", rs)
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

func generateConcurrencyBenchmarkData() (*ast.Module, map[string]interface{}) {
	obj := util.MustUnmarshalJSON([]byte(`
		{
			"objs": [
				{
					"attr1": "get",
					"path": "/foo/bar",
					"user": "bob"
				},
				{
					"attr1": "set",
					"path": "/foo/bar/baz",
					"user": "alice"
				},
				{
					"attr1": "get",
					"path": "/foo",
					"groups": [
						"admin",
						"eng"
					]
				},
				{
					"path": "/foo/bar",
					"user": "alice"
				}
			]
		}
		`))

	mod := `package test

	import data.objs

	p {
		objs[i].attr1 = "get"
		objs[i].groups[j] = "eng"
	}

	p {
		objs[i].user = "alice"
	}
	`

	return ast.MustParseModule(mod), obj.(map[string]interface{})
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

func runVirtualDocsBenchmark(b *testing.B, numTotalRules, numHitRules int) {

	mod, input := generateVirtualDocsBenchmarkData(numTotalRules, numHitRules)
	ctx := context.Background()
	compiler := ast.NewCompiler()
	mods := map[string]*ast.Module{"module": mod}
	store := inmem.New()
	txn := storage.NewTransactionOrDie(ctx, store)
	if compiler.Compile(mods); compiler.Failed() {
		b.Fatalf("Unexpected compiler error: %v", compiler.Errors)
	}

	query := NewQuery(ast.MustParseBody("data.a.b.c.allow = x")).
		WithCompiler(compiler).
		WithStore(store).
		WithTransaction(txn).
		WithInput(input)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		func() {
			rs, err := query.Run(ctx)
			if err != nil {
				b.Fatalf("Unexpected topdown query error: %v", err)
			}
			if len(rs) != 1 || !rs[0][ast.Var("x")].Equal(ast.BooleanTerm(true)) {
				b.Fatalf("Unexpecfted undefined/extra/bad result: %v", rs)
			}
		}()

	}
}

func generateVirtualDocsBenchmarkData(numTotalRules, numHitRules int) (*ast.Module, *ast.Term) {

	hitRule := `
	allow {
		input.method = "POST"
		input.path = ["accounts", account_id]
		input.user_id = account_id
	}
	`

	missRule := `
	allow {
		input.method = "GET"
		input.path = ["salaries", account_id]
		input.user_id = account_id
	}
	`

	testModuleTmpl := `
	package a.b.c

	{{range .MissRules }}
		{{ . }}
	{{end}}

	{{range .HitRules }}
		{{ . }}
	{{end}}
	`

	tmpl, err := template.New("Test").Parse(testModuleTmpl)
	if err != nil {
		panic(err)
	}

	var buf bytes.Buffer

	var missRules []string

	if numTotalRules > numHitRules {
		missRules = make([]string, numTotalRules-numHitRules)
		for i := range missRules {
			missRules[i] = missRule
		}
	}

	hitRules := make([]string, numHitRules)
	for i := range hitRules {
		hitRules[i] = hitRule
	}

	params := struct {
		MissRules []string
		HitRules  []string
	}{
		MissRules: missRules,
		HitRules:  hitRules,
	}

	err = tmpl.Execute(&buf, params)
	if err != nil {
		panic(err)
	}

	input := ast.MustParseTerm(`{
			"path": ["accounts", "alice"],
			"method": "POST",
			"user_id": "alice"
		}`)

	return ast.MustParseModule(buf.String()), input
}
