// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package rego_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/rego"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/storage/inmem"
	"github.com/open-policy-agent/opa/util"
)

func ExampleRego_Eval_simple() {

	ctx := context.Background()

	// Create very simple query that binds a single variable.
	rego := rego.New(rego.Query("x = 1"))

	// Run evaluation.
	rs, err := rego.Eval(ctx)

	// Inspect results.
	fmt.Println("len:", len(rs))
	fmt.Println("bindings:", rs[0].Bindings)
	fmt.Println("err:", err)

	// Output:
	//
	// len: 1
	// bindings: map[x:1]
	// err: <nil>
}

func ExampleRego_Eval_input() {

	ctx := context.Background()

	// Raw input data that will be used in evaluation.
	raw := `{"users": [{"id": "bob"}, {"id": "alice"}]}`
	d := json.NewDecoder(bytes.NewBufferString(raw))

	// Numeric values must be represented using json.Number.
	d.UseNumber()

	var input interface{}

	if err := d.Decode(&input); err != nil {
		panic(err)
	}

	// Create a simple query over the input.
	rego := rego.New(
		rego.Query("input.users[idx].id = user_id"),
		rego.Input(input))

	//Run evaluation.
	rs, err := rego.Eval(ctx)

	if err != nil {
		// Handle error.
	}

	// Inspect results.
	fmt.Println("len:", len(rs))
	fmt.Println("bindings.idx:", rs[1].Bindings["idx"])
	fmt.Println("bindings.user_id:", rs[1].Bindings["user_id"])

	// Output:
	//
	// len: 2
	// bindings.idx: 1
	// bindings.user_id: alice
}

func ExampleRego_Eval_multipleBindings() {

	ctx := context.Background()

	// Create query that produces multiple bindings for variable.
	rego := rego.New(
		rego.Query(`a = ["ex", "am", "ple"]; x = a[_]; not p[x]`),
		rego.Package(`example`),
		rego.Module("example.rego", `package example

		p["am"] { true }
		`),
	)

	// Run evaluation.
	rs, err := rego.Eval(ctx)

	// Inspect results.
	fmt.Println("len:", len(rs))
	fmt.Println("err:", err)
	for i := range rs {
		fmt.Printf("bindings[\"x\"]: %v (i=%d)\n", rs[i].Bindings["x"], i)
	}

	// Output:
	//
	// len: 2
	// err: <nil>
	// bindings["x"]: ex (i=0)
	// bindings["x"]: ple (i=1)
}

func ExampleRego_Eval_singleDocument() {

	ctx := context.Background()

	// Create query that produces a single document.
	rego := rego.New(
		rego.Query("data.example.p"),
		rego.Module("example.rego",
			`package example

p = ["hello", "world"] { true }`,
		))

	// Run evaluation.
	rs, err := rego.Eval(ctx)

	// Inspect result.
	fmt.Println("value:", rs[0].Expressions[0].Value)
	fmt.Println("err:", err)

	// Output:
	//
	// value: [hello world]
	// err: <nil>
}

func ExampleRego_Eval_multipleDocuments() {

	ctx := context.Background()

	// Create query that produces multiple documents.
	rego := rego.New(
		rego.Query("data.example.p[x]"),
		rego.Module("example.rego",
			`package example

p = {"hello": "alice", "goodbye": "bob"} { true }`,
		))

	// Run evaluation.
	rs, err := rego.Eval(ctx)

	// Inspect results.
	fmt.Println("len:", len(rs))
	fmt.Println("err:", err)
	for i := range rs {
		fmt.Printf("bindings[\"x\"]: %v (i=%d)\n", rs[i].Bindings["x"], i)
		fmt.Printf("value: %v (i=%d)\n", rs[i].Expressions[0].Value, i)
	}

	// Output:
	//
	// len: 2
	// err: <nil>
	// bindings["x"]: hello (i=0)
	// value: alice (i=0)
	// bindings["x"]: goodbye (i=1)
	// value: bob (i=1)
}

func ExampleRego_Eval_compiler() {

	ctx := context.Background()

	// Define a simple policy.
	module := `
		package example

		default allow = false

		allow {
			input.identity = "admin"
		}

		allow {
			input.method = "GET"
		}
	`

	// Parse the module. The first argument is used as an identifier in error messages.
	parsed, err := ast.ParseModule("example.rego", module)
	if err != nil {
		// Handle error.
	}

	// Create a new compiler and compile the module. The keys are used as
	// identifiers in error messages.
	compiler := ast.NewCompiler()
	compiler.Compile(map[string]*ast.Module{
		"example.rego": parsed,
	})

	if compiler.Failed() {
		// Handle error. Compilation errors are stored on the compiler.
		panic(compiler.Errors)
	}

	// Create a new query that uses the compiled policy from above.
	rego := rego.New(
		rego.Query("data.example.allow"),
		rego.Compiler(compiler),
		rego.Input(
			map[string]interface{}{
				"identity": "bob",
				"method":   "GET",
			},
		),
	)

	// Run evaluation.
	rs, err := rego.Eval(ctx)

	if err != nil {
		// Handle error.
	}

	// Inspect results.
	fmt.Println("len:", len(rs))
	fmt.Println("value:", rs[0].Expressions[0].Value)

	// Output:
	//
	// len: 1
	// value: true
}

func ExampleRego_Eval_storage() {

	ctx := context.Background()

	data := `{
        "example": {
            "users": [
                {
                    "name": "alice",
                    "likes": ["dogs", "clouds"]
                },
                {
                    "name": "bob",
                    "likes": ["pizza", "cats"]
                }
            ]
        }
    }`

	var json map[string]interface{}

	err := util.UnmarshalJSON([]byte(data), &json)
	if err != nil {
		// Handle error.
	}

	// Manually create the storage layer. inmem.NewFromObject returns an
	// in-memory store containing the supplied data.
	store := inmem.NewFromObject(json)

	// Create new query that returns the value
	rego := rego.New(
		rego.Query("data.example.users[0].likes"),
		rego.Store(store))

	// Run evaluation.
	rs, err := rego.Eval(ctx)

	// Inspect the result.
	fmt.Println("value:", rs[0].Expressions[0].Value)

	// Output:
	//
	// value: [dogs clouds]
}

func ExampleRego_Eval_transactions() {

	ctx := context.Background()

	// Create storage layer and load dummy data.
	store := inmem.NewFromReader(bytes.NewBufferString(`{
		"favourites": {
			"pizza": "cheese",
			"colour": "violet"
		}
	}`))

	// Open a write transaction on the store that will perform write operations.
	txn, err := store.NewTransaction(ctx, storage.WriteParams)
	if err != nil {
		// Handle error.
	}

	// Create rego query that uses the transaction created above.
	inside := rego.New(
		rego.Query("data.favourites.pizza"),
		rego.Store(store),
		rego.Transaction(txn),
	)

	// Create rego query that DOES NOT use the transaction created above. Under
	// the hood, the rego package will create it's own read-only transaction to
	// ensure it evaluates over a consistent snapshot of the storage layer.
	outside := rego.New(
		rego.Query("data.favourites.pizza"),
		rego.Store(store),
	)

	// Write change to storage layer inside the transaction.
	err = store.Write(ctx, txn, storage.AddOp, storage.MustParsePath("/favourites/pizza"), "pepperoni")
	if err != nil {
		// Handle error.
	}

	// Run evaluation INSIDE the transction.
	rs, err := inside.Eval(ctx)
	if err != nil {
		// Handle error.
	}

	fmt.Println("value (inside txn):", rs[0].Expressions[0].Value)

	// Run evaluation OUTSIDE the transaction.
	rs, err = outside.Eval(ctx)
	if err != nil {
		// Handle error.
	}

	fmt.Println("value (outside txn):", rs[0].Expressions[0].Value)

	if err := store.Commit(ctx, txn); err != nil {
		// Handle error.
	}

	// Run evaluation AFTER the transaction commits.
	rs, err = outside.Eval(ctx)
	if err != nil {
		// Handle error.
	}

	fmt.Println("value (after txn):", rs[0].Expressions[0].Value)

	// Output:
	//
	// value (inside txn): pepperoni
	// value (outside txn): cheese
	// value (after txn): pepperoni
}

func ExampleRego_Eval_errors() {

	ctx := context.Background()

	r := rego.New(
		rego.Query("data.example.p"),
		rego.Module("example_error.rego",
			`package example

p = true { not q[x] }
q = {1, 2, 3} { true }`,
		))

	_, err := r.Eval(ctx)

	switch err := err.(type) {
	case rego.Errors:
		for i := range err {
			switch e := err[i].(type) {
			case *ast.Error:
				fmt.Println("code:", e.Code)
				fmt.Println("row:", e.Location.Row)
				fmt.Println("filename:", e.Location.File)
			}
		}
	default:
		// Some other error occurred.
	}

	// Output:
	//
	// code: rego_unsafe_var_error
	// row: 3
	// filename: example_error.rego
}
