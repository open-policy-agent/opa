// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package rego_test

import (
	"context"
	"fmt"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/rego"
	"github.com/open-policy-agent/opa/storage"
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

	// Manually create the storage layer. storage.InMemoryWithJSONConfig will
	// return configure the storage layer to use an in-memory store with the
	// specified json data. This is mostly for test purposes. See storage
	// package for more information on custom storage backends.
	store := storage.New(storage.InMemoryWithJSONConfig(json))

	// Create new query that returns the value
	rego := rego.New(
		rego.Query("data.example.users[0].likes"),
		rego.Storage(store))

	// Run evaluation.
	rs, err := rego.Eval(ctx)

	// Inspect the result.
	fmt.Println("value:", rs[0].Expressions[0].Value)

	// Output:
	//
	// value: [dogs clouds]
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
	// code: 2
	// row: 3
	// filename: example_error.rego
}
