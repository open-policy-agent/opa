// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package authz

import (
	"context"
	"testing"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/logging"
	"github.com/open-policy-agent/opa/rego"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/storage/disk"
	"github.com/open-policy-agent/opa/storage/inmem"
)

func BenchmarkAuthzForbidAuthn(b *testing.B) {
	b.Run("inmem", func(b *testing.B) {
		runAuthzBenchmark(b, ForbidIdentity, 10)
	})
	b.Run("disk", func(b *testing.B) {
		runAuthzBenchmark(b, ForbidIdentity, 10, true)
	})
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

func runAuthzBenchmark(b *testing.B, mode InputMode, numPaths int, extras ...bool) {

	profile := DataSetProfile{
		NumTokens: 1000,
		NumPaths:  numPaths,
	}

	ctx := context.Background()
	data := GenerateDataset(profile)

	useDisk := false
	if len(extras) > 0 {
		useDisk = extras[0]
	}

	var store storage.Store
	var err error
	if useDisk {
		rootDir := b.TempDir()
		store, err = disk.New(ctx, logging.NewNoOpLogger(), nil, disk.Options{
			Dir:        rootDir,
			Partitions: nil,
		})
		if err != nil {
			b.Fatal(err)
		}

		err = storage.WriteOne(ctx, store, storage.AddOp, storage.Path{}, data)
		if err != nil {
			b.Fatal(err)
		}
	} else {
		store = inmem.NewFromObject(data)
	}

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

		if rs.Allowed() != expected {
			b.Fatalf("Unexpected result: %v", rs)
		}
	}
}
