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
)

func TestAuthz(t *testing.T) {

	profile := DataSetProfile{
		NumTokens: 1000,
		NumPaths:  10,
	}

	ctx := context.Background()
	data := GenerateDataset(profile)
	store := inmem.NewFromObject(data)
	txn := storage.NewTransactionOrDie(ctx, store)
	compiler := ast.NewCompiler()
	module := ast.MustParseModule(Policy)

	compiler.Compile(map[string]*ast.Module{"": module})
	if compiler.Failed() {
		t.Fatalf("Unexpected error(s): %v", compiler.Errors)
	}

	input, expected := GenerateInput(profile, ForbidPath)

	r := rego.New(
		rego.Compiler(compiler),
		rego.Store(store),
		rego.Transaction(txn),
		rego.Input(input),
		rego.Query(AllowQuery),
	)

	rs, err := r.Eval(ctx)

	if err != nil {
		t.Fatalf("Unexpected error(s): %v", err)
	}

	if rs.Allowed() != expected {
		t.Fatalf("Unexpected result: want %v, got %v", expected, rs.Allowed())
	}
}
