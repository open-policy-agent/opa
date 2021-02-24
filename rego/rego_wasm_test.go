// Copyright 2021 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// +build opa_wasm

package rego

func TestPrepareAndEvalWithWasmTarget(t *testing.T) {
	mod := `
	package test
	default p = false
	p {
		input.x == 1
	}
	`

	ctx := context.Background()

	pq, err := New(
		Query("data.test.p = x"),
		Target("wasm"),
		Module("a.rego", mod),
	).PrepareForEval(ctx)

	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	assertPreparedEvalQueryEval(t, pq, []EvalOption{
		EvalInput(map[string]int{"x": 1}),
	}, "[[true]]")

	pq, err = New(
		Query("a = [1,2]; x = a[i]"),
		Target("wasm"),
	).PrepareForEval(ctx)

	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	assertPreparedEvalQueryEval(t, pq, []EvalOption{}, "[[true, true]]")
}

func TestPrepareAndEvalWithWasmTargetModulesOnCompiler(t *testing.T) {
	mod := `
	package test
	default p = false
	p {
		input.x == data.x.p
	}
	`

	compiler := ast.NewCompiler()

	compiler.Compile(map[string]*ast.Module{
		"a.rego": ast.MustParseModule(mod),
	})

	if len(compiler.Errors) > 0 {
		t.Fatalf("Unexpected compile errors: %s", compiler.Errors)
	}

	ctx := context.Background()

	pq, err := New(
		Compiler(compiler),
		Query("data.test.p"),
		Target("wasm"),
		Store(inmem.NewFromObject(map[string]interface{}{
			"x": map[string]interface{}{"p": 1},
		})),
	).PrepareForEval(ctx)

	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	assertPreparedEvalQueryEval(t, pq, []EvalOption{
		EvalInput(map[string]int{"x": 1}),
	}, "[[true]]")
}
