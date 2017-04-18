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

func BenchmarkVirtualDocs1(b *testing.B) {
	runVirtualDocsBenchmark(b, 1)
}

func BenchmarkVirtualDocs10(b *testing.B) {
	runVirtualDocsBenchmark(b, 10)
}

func BenchmarkVirtualDocs100(b *testing.B) {
	runVirtualDocsBenchmark(b, 100)
}

func BenchmarkVirtualDocs1000(b *testing.B) {
	runVirtualDocsBenchmark(b, 1000)
}

func runVirtualDocsBenchmark(b *testing.B, numRules int) {

	// Generate test module containing numRules instances of allow.
	testRule := `
	allow {
		input.method = "POST"
		input.path = ["accounts", account_id]
		input.user_id = account_id
	}
	`

	testModuleTmpl := `
	package a.b.c

	{{range . }}
		{{ . }}
	{{end}}
	`

	tmpl, err := template.New("Test").Parse(testModuleTmpl)
	if err != nil {
		b.Fatalf("Unexpected error while parsing template: %v", err)
	}

	var buf bytes.Buffer

	rules := make([]string, numRules)
	for i := range rules {
		rules[i] = testRule
	}

	err = tmpl.Execute(&buf, rules)
	if err != nil {
		b.Fatalf("Unexpected error while executing template: %v", err)
	}

	// Setup evaluation...
	ctx := context.Background()
	compiler := ast.NewCompiler()
	mod := ast.MustParseModule(buf.String())
	mods := map[string]*ast.Module{"module": mod}
	store := storage.New(storage.InMemoryConfig())
	txn := storage.NewTransactionOrDie(ctx, store)
	input := ast.MustParseTerm(`{
			"path": ["accounts", "alice"],
			"method": "POST",
			"user_id": "alice"
		}`).Value

	if compiler.Compile(mods); compiler.Failed() {
		b.Fatalf("Unexpected compiler error: %v", compiler.Errors)
	}

	params := NewQueryParams(ctx, compiler, store, txn, input, ast.MustParseRef("data.a.b.c.allow"))

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
