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
	runAuthzBenchmark(b, ForbidIdentity, 10)
}

func BenchmarkAuthzForbidPath(b *testing.B) {
	runAuthzBenchmark(b, ForbidPath, 10)
}

func BenchmarkAuthzForbidMethod(b *testing.B) {
	runAuthzBenchmark(b, ForbidMethod, 10)
}

func BenchmarkAuthzAllow10Paths(b *testing.B) {
	runAuthzBenchmark(b, Allow, 10)
}

func BenchmarkAuthzAllow100Paths(b *testing.B) {
	runAuthzBenchmark(b, Allow, 100)
}

func BenchmarkAuthzAllow1000Paths(b *testing.B) {
	runAuthzBenchmark(b, Allow, 1000)
}

func runAuthzBenchmark(b *testing.B, mode InputMode, numPaths int) {

	profile := DataSetProfile{
		NumTokens: 1000,
		NumPaths:  numPaths,
	}

	ctx := context.Background()
	data := GenerateDataset(profile)
	store := inmem.NewFromObject(data)
	txn := storage.NewTransactionOrDie(ctx, store)
	compiler := ast.NewCompiler()
	module := ast.MustParseModule(Policy)

	compiler.Compile(map[string]*ast.Module{"": module})
	if compiler.Failed() {
		b.Fatalf("Unexpected error(s): %v", compiler.Errors)
	}

	r := rego.New(
		rego.Compiler(compiler),
		rego.Store(store),
		rego.Transaction(txn),
		rego.Query(AllowQuery),
	)

	// Pre-process as much as we can
	pq, err := r.PrepareForEval(ctx)
	if err != nil {
		b.Fatalf("Unexpected error(s): %v", err)
	}

	input, expected := GenerateInput(profile, mode)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		b.StartTimer()
		rs, err := pq.Eval(
			ctx,
			rego.EvalInput(input),
		)
		if err != nil {
			b.Fatalf("Unexpected error(s): %v", err)
		}
		b.StopTimer()

		if len(rs) != 1 || util.Compare(rs[0].Expressions[0].Value, expected) != 0 {
			b.Fatalf("Unexpected result: %v", rs)
		}
	}
}
