// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"bytes"
	"context"
	"testing"
	"text/template"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/storage"
)

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

	// Generate test input
	mod, input := generateRulesAndInput(numTotalRules, numHitRules)

	// Setup evaluation...
	ctx := context.Background()
	compiler := ast.NewCompiler()
	mods := map[string]*ast.Module{"module": mod}
	store := storage.New(storage.InMemoryConfig())
	txn := storage.NewTransactionOrDie(ctx, store)
	if compiler.Compile(mods); compiler.Failed() {
		b.Fatalf("Unexpected compiler error: %v", compiler.Errors)
	}

	params := NewQueryParams(ctx, compiler, store, txn, input, ast.MustParseRef("data.a.b.c.allow"))

	b.ResetTimer()

	// Run query N times.
	for i := 0; i < b.N; i++ {
		func() {

			rs, err := Query(params)
			if err != nil {
				b.Fatalf("Unexpected topdown query error: %v", err)
			}

			if rs.Undefined() || len(rs) != 1 || rs[0].Result.(bool) != true {
				b.Fatalf("Unexpecfted undefined/extra/bad result: %v", rs)
			}
		}()

	}
}

func generateRulesAndInput(numTotalRules, numHitRules int) (*ast.Module, ast.Value) {

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
		}`).Value

	return ast.MustParseModule(buf.String()), input
}
