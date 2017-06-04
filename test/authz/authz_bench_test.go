// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package authz

import (
	"context"
	"testing"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/storage/inmem"
	"github.com/open-policy-agent/opa/topdown"
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
	path := ast.MustParseRef("data.restauthz.allow")

	compiler.Compile(map[string]*ast.Module{"": module})
	if compiler.Failed() {
		b.Fatalf("Unexpected error(s): %v", compiler.Errors)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		input, expected := generateInput(profile, mode)
		params := topdown.NewQueryParams(ctx, compiler, store, txn, input, path)
		rs, err := topdown.Query(params)
		if err != nil {
			b.Fatalf("Unexpected error(s): %v", err)
		}
		if util.Compare(rs[0].Result, expected) != 0 {
			b.Fatalf("Unexpected result: %v", rs)
		}
	}
}
