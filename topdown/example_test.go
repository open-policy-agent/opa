// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/topdown"
)

func ExampleEval() {
	// Initialize context for the example. Normally the caller would obtain the
	// context from an input parameter or instantiate their own.
	ctx := context.Background()

	compiler := ast.NewCompiler()

	// Define a dummy query and some data that the query will execute against.
	query, err := compiler.QueryCompiler().Compile(ast.MustParseBody("data.a[_] = x, x >= 2"))
	if err != nil {
		// Handle error.
	}

	var data map[string]interface{}

	// OPA uses Go's standard JSON library but assumes that numbers have been
	// decoded as json.Number instead of float64. You MUST decode with UseNumber
	// enabled.
	decoder := json.NewDecoder(bytes.NewBufferString(`{"a": [1,2,3,4]}`))
	decoder.UseNumber()

	if err := decoder.Decode(&data); err != nil {
		// Handle error.
	}

	// Instantiate the policy engine's storage layer.
	store := storage.New(storage.InMemoryWithJSONConfig(data))

	// Create a new transaction. Transactions allow the policy engine to
	// evaluate the query over a consistent snapshot fo the storage layer.
	txn, err := store.NewTransaction(ctx)
	if err != nil {
		// Handle error.
	}

	defer store.Close(ctx, txn)

	// Prepare the evaluation parameters. Evaluation executes against the policy
	// engine's storage. In this case, we seed the storage with a single array
	// of number. Other parameters such as globals, tracing configuration, etc.
	// can be set on the Topdown object.
	t := topdown.New(ctx, query, compiler, store, txn)

	result := []interface{}{}

	// Execute the query and provide a callbakc function to accumulate the results.
	err = topdown.Eval(t, func(t *topdown.Topdown) error {

		// Each variable in the query will have an associated "binding" in the context.
		x := t.Binding(ast.Var("x"))

		// The bindings are ast.Value types so we will convert to a native Go value here.
		v, err := topdown.ValueToInterface(x, t)
		if err != nil {
			return err
		}

		result = append(result, v)
		return nil
	})

	// Inspect the query result.
	fmt.Println("result:", result)
	fmt.Println("err:", err)

	// Output:
	// result: [2 3 4]
	// err: <nil>
}

func ExampleQuery() {
	// Initialize context for the example. Normally the caller would obtain the
	// context from an input parameter or instantiate their own.
	ctx := context.Background()

	compiler := ast.NewCompiler()

	// Define a dummy module with rules that produce documents that we will query below.
	module, err := ast.ParseModule("my_module.rego", `

	    package opa.example

	    p[x] :- q[x], not r[x]
	    q[y] :- a = [1,2,3], y = a[_]
	    r[z] :- b = [2,4], z = b[_]

	`)

	mods := map[string]*ast.Module{
		"my_module": module,
	}

	if compiler.Compile(mods); compiler.Failed() {
		fmt.Println(compiler.Errors)
	}

	if err != nil {
		// Handle error.
	}

	// Instantiate the policy engine's storage layer.
	store := storage.New(storage.InMemoryConfig())

	// Create a new transaction. Transactions allow the policy engine to
	// evaluate the query over a consistent snapshot fo the storage layer.
	txn, err := store.NewTransaction(ctx)
	if err != nil {
		// Handle error.
	}

	defer store.Close(ctx, txn)

	// Prepare the query parameters. Queries execute against the policy engine's storage and can
	// accept additional documents (which are referred to as "globals"). In this case we have no
	// additional documents.
	globals := ast.NewValueMap()
	params := topdown.NewQueryParams(ctx, compiler, store, txn, globals, ast.MustParseRef("data.opa.example.p"))

	// Execute the query against "p".
	v1, err1 := topdown.Query(params)

	// Inspect the result.
	fmt.Println("v1:", v1[0].Result)
	fmt.Println("err1:", err1)

	// Output:
	// v1: [1 3]
	// err1: <nil>

}
