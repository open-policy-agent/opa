// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package authz

import (
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/logging"
	"github.com/open-policy-agent/opa/v1/rego"
	"github.com/open-policy-agent/opa/v1/storage"
	"github.com/open-policy-agent/opa/v1/storage/disk"
	"github.com/open-policy-agent/opa/v1/storage/inmem"
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

	ctx := b.Context()
	data := GenerateDataset(profile)
	useDisk := len(extras) > 0 && extras[0]

	var store storage.Store
	if useDisk {
		var err error
		if store, err = disk.New(ctx, logging.NewNoOpLogger(), nil, disk.Options{Dir: b.TempDir()}); err != nil {
			b.Fatal(err)
		}

		if err = storage.WriteOne(ctx, store, storage.AddOp, storage.Path{}, data); err != nil {
			b.Fatal(err)
		}
	} else {
		store = inmem.NewFromObjectWithOpts(data)
	}

	compiler := ast.NewCompiler()
	if compiler.Compile(map[string]*ast.Module{"": ast.MustParseModule(Policy)}); compiler.Failed() {
		b.Fatalf("Unexpected error(s): %v", compiler.Errors)
	}

	r := rego.New(
		rego.Compiler(compiler),
		rego.Store(store),
		rego.Transaction(storage.NewTransactionOrDie(ctx, store)),
		rego.Query(AllowQuery),
	)

	pq, err := r.PrepareForEval(ctx)
	if err != nil {
		b.Fatalf("Unexpected error(s): %v", err)
	}

	input, expected := GenerateInput(profile, mode)

	inputAST, err := ast.InterfaceToValue(input)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()

	for range b.N {
		rs, err := pq.Eval(ctx, rego.EvalParsedInput(inputAST))
		if err != nil {
			b.Fatalf("Unexpected error(s): %v", err)
		}

		if rs.Allowed() != expected {
			b.Fatalf("Unexpected result: %v", rs)
		}
	}
}
