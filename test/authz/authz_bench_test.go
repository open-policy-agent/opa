// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package authz

import (
	"context"
	"testing"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/rego"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/storage/inmem"
	"github.com/open-policy-agent/opa/util"
)

func BenchmarkAuthzForbidAuthn(b *testing.B) {
	runAuthzBenchmark(b, forbidIdentity, 10)
}

func BenchmarkAuthzForbidPath(b *testing.B) {
	runAuthzBenchmark(b, forbidPath, 10)
}

func BenchmarkAuthzForbidMethod(b *testing.B) {
	runAuthzBenchmark(b, forbidMethod, 10)
}

func BenchmarkAuthzAllow10Paths(b *testing.B) {
	runAuthzBenchmark(b, allow, 10)
}

func BenchmarkAuthzAllow100Paths(b *testing.B) {
	runAuthzBenchmark(b, allow, 100)
}

func BenchmarkAuthzAllow1000Paths(b *testing.B) {
	runAuthzBenchmark(b, allow, 1000)
}

func runAuthzBenchmark(b *testing.B, mode inputMode, numPaths int) {

	profile := dataSetProfile{
		numTokens: 1000,
		numPaths:  numPaths,
	}

	ctx := context.Background()
	data := generateDataset(profile)
	store := inmem.NewFromObject(data)
	txn := storage.NewTransactionOrDie(ctx, store)
	compiler := ast.NewCompiler()
	module := ast.MustParseModule(policy)

	compiler.Compile(map[string]*ast.Module{"": module})
	if compiler.Failed() {
		b.Fatalf("Unexpected error(s): %v", compiler.Errors)
	}

	b.ResetTimer()

	r := rego.New(
		rego.Compiler(compiler),
		rego.Store(store),
		rego.Transaction(txn),
		rego.Query("data.restauthz.allow"),
	)

	// Pre-process as much as we can
	pq, err := r.PrepareForEval(ctx)
	if err != nil {
		b.Fatalf("Unexpected error(s): %v", err)
	}

	for i := 0; i < b.N; i++ {
		input, expected := generateInput(profile, mode)

		rs, err := pq.Eval(ctx, rego.EvalInput(input))
		if err != nil {
			b.Fatalf("Unexpected error(s): %v", err)
		}

		if len(rs) != 1 || util.Compare(rs[0].Expressions[0].Value, expected) != 0 {
			b.Fatalf("Unexpected result: %v", rs)
		}
	}
}
