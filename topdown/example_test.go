// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"strings"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/storage/inmem"
	"github.com/open-policy-agent/opa/topdown"
	"github.com/open-policy-agent/opa/topdown/builtins"
	"github.com/open-policy-agent/opa/types"
)

func ExampleQuery_Iter() {
	// Initialize context for the example. Normally the caller would obtain the
	// context from an input parameter or instantiate their own.
	ctx := context.Background()

	compiler := ast.NewCompiler()

	// Define a dummy query and some data that the query will execute against.
	query, err := compiler.QueryCompiler().Compile(ast.MustParseBody(`data.a[_] = x; x >= 2`))
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
	store := inmem.NewFromObject(data)

	// Create a new transaction. Transactions allow the policy engine to
	// evaluate the query over a consistent snapshot fo the storage layer.
	txn, err := store.NewTransaction(ctx)
	if err != nil {
		// Handle error.
	}

	defer store.Abort(ctx, txn)

	// Prepare the evaluation parameters. Evaluation executes against the policy
	// engine's storage. In this case, we seed the storage with a single array
	// of number. Other parameters such as the input, tracing configuration,
	// etc. can be set on the query object.
	q := topdown.NewQuery(query).
		WithCompiler(compiler).
		WithStore(store).
		WithTransaction(txn)

	result := []interface{}{}

	// Execute the query and provide a callback function to accumulate the results.
	err = q.Iter(ctx, func(qr topdown.QueryResult) error {

		// Each variable in the query will have an associated binding.
		x := qr[ast.Var("x")]

		// The bindings are ast.Value types so we will convert to a native Go value here.
		v, err := ast.JSON(x.Value)
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

func ExampleQuery_Run() {
	// Initialize context for the example. Normally the caller would obtain the
	// context from an input parameter or instantiate their own.
	ctx := context.Background()

	compiler := ast.NewCompiler()

	// Define a dummy query and some data that the query will execute against.
	query, err := compiler.QueryCompiler().Compile(ast.MustParseBody(`data.a[_] = x; x >= 2`))
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
	store := inmem.NewFromObject(data)

	// Create a new transaction. Transactions allow the policy engine to
	// evaluate the query over a consistent snapshot fo the storage layer.
	txn, err := store.NewTransaction(ctx)
	if err != nil {
		// Handle error.
	}

	defer store.Abort(ctx, txn)

	// Prepare the evaluation parameters. Evaluation executes against the policy
	// engine's storage. In this case, we seed the storage with a single array
	// of number. Other parameters such as the input, tracing configuration,
	// etc. can be set on the query object.
	q := topdown.NewQuery(query).
		WithCompiler(compiler).
		WithStore(store).
		WithTransaction(txn)

	rs, err := q.Run(ctx)

	// Inspect the query result set.
	fmt.Println("len:", len(rs))
	for i := range rs {
		fmt.Printf("rs[%d][\"x\"]: %v\n", i, rs[i]["x"])
	}
	fmt.Println("err:", err)

	// Output:
	// len: 3
	// rs[0]["x"]: 2
	// rs[1]["x"]: 3
	// rs[2]["x"]: 4
	// err: <nil>
}

func ExampleQuery_PartialRun() {

	// Initialize context for the example. Normally the caller would obtain the
	// context from an input parameter or instantiate their own.
	ctx := context.Background()

	var data map[string]interface{}
	decoder := json.NewDecoder(bytes.NewBufferString(`{
		"roles": [
			{
				"permissions": ["read_bucket"],
				"groups": ["dev", "test", "sre"]
			},
			{
				"permissions": ["write_bucket", "delete_bucket"],
				"groups": ["sre"]
			}
		]
	}`))
	if err := decoder.Decode(&data); err != nil {
		// Handle error.
	}

	// Instantiate the policy engine's storage layer.
	store := inmem.NewFromObject(data)

	// Create a new transaction. Transactions allow the policy engine to
	// evaluate the query over a consistent snapshot fo the storage layer.
	txn, err := store.NewTransaction(ctx)
	if err != nil {
		// Handle error.
	}

	defer store.Abort(ctx, txn)

	// Define policy that searches for roles that match input request. If no
	// roles are found, allow is undefined and the caller will reject the
	// request. This is the user supplied policy that OPA will partially
	// evaluate.
	modules := map[string]*ast.Module{
		"authz.rego": ast.MustParseModule(`
			package example

			default allow = false

			allow {
				role = data.roles[i]
				input.group = role.groups[j]
				input.permission = role.permissions[k]
			}
		`),
	}

	// Compile policy.
	compiler := ast.NewCompiler()
	if compiler.Compile(modules); compiler.Failed() {
		// Handle error.
	}

	// Construct query and mark the entire input document as partial.
	q := topdown.NewQuery(ast.MustParseBody("data.example.allow = true")).
		WithCompiler(compiler).
		WithUnknowns([]*ast.Term{
			ast.MustParseTerm("input"),
		}).
		WithStore(store).
		WithTransaction(txn)

	// Execute partial evaluation.
	partial, _, err := q.PartialRun(ctx)
	if err != nil {
		// Handle error.
	}

	// Show result of partially evaluating the policy.
	fmt.Printf("# partial evaluation result (%d items):\n", len(partial))
	for i := range partial {
		fmt.Println(partial[i])
	}

	// Construct a new policy to contain the result of partial evaluation.
	module := ast.MustParseModule("package partial")

	for i := range partial {
		rule := &ast.Rule{
			Head: &ast.Head{
				Name:  ast.Var("allow"),
				Value: ast.BooleanTerm(true),
			},
			Body:   partial[i],
			Module: module,
		}
		module.Rules = append(module.Rules, rule)
	}

	// Compile the partially evaluated policy with the original policy.
	modules["partial"] = module

	if compiler.Compile(modules); compiler.Failed() {
		// Handle error.
	}

	// Test different inputs against partially evaluated policy.
	inputs := []string{
		`{"group": "dev", "permission": "read_bucket"}`,  // allow
		`{"group": "dev", "permission": "write_bucket"}`, // deny
		`{"group": "sre", "permission": "write_bucket"}`, // allow
	}

	fmt.Println()
	fmt.Println("# evaluation results:")

	for i := range inputs {

		// Query partially evaluated policy.
		q = topdown.NewQuery(ast.MustParseBody("data.partial.allow = true")).
			WithCompiler(compiler).
			WithStore(store).
			WithTransaction(txn).
			WithInput(ast.MustParseTerm(inputs[i]))

		qrs, err := q.Run(ctx)
		if err != nil {
			// Handle error.
		}

		// Check if input is allowed.
		allowed := len(qrs) == 1

		fmt.Printf("input %d allowed: %v\n", i+1, allowed)
	}

	// Output:
	//
	// # partial evaluation result (5 items):
	// "dev" = input.group; "read_bucket" = input.permission
	// "test" = input.group; "read_bucket" = input.permission
	// "sre" = input.group; "read_bucket" = input.permission
	// "sre" = input.group; "write_bucket" = input.permission
	// "sre" = input.group; "delete_bucket" = input.permission
	//
	// # evaluation results:
	// input 1 allowed: true
	// input 2 allowed: false
	// input 3 allowed: true

}

func ExampleRegisterFunctionalBuiltin1() {

	// Rego includes a number of built-in functions ("built-ins") for performing
	// standard operations like string manipulation, regular expression
	// matching, and computing aggregates.
	//
	// This test shows how to add a new built-in to Rego and OPA.

	// Initialize context for the example. Normally the caller would obtain the
	// context from an input parameter or instantiate their own.
	ctx := context.Background()

	// The ast package contains a registry that enumerates the built-ins
	// included in Rego. When adding a new built-in, you must update the
	// registry to include your built-in. Otherwise, the compiler will complain
	// when it encounters your built-in.
	builtin := &ast.Builtin{
		Name: "mybuiltins.upper",
		Decl: types.NewFunction(
			types.Args(types.S),
			types.S,
		),
	}

	ast.RegisterBuiltin(builtin)

	// This is the implementation of the built-in that will be called during
	// query evaluation.
	builtinImpl := func(a ast.Value) (ast.Value, error) {

		str, err := builtins.StringOperand(a, 1)

		if err != nil {
			return nil, err
		}

		if str.Equal(ast.String("magic")) {
			// topdown.BuiltinEmpty indicates to the evaluation engine that the
			// expression is false/not defined.
			return nil, topdown.BuiltinEmpty{}
		}

		return ast.String(strings.ToUpper(string(str))), nil
	}

	// See documentation for registering functions that take different numbers
	// of arguments.
	topdown.RegisterFunctionalBuiltin1(builtin.Name, builtinImpl)

	// At this point, the new built-in has been registered and can be used in
	// queries. Our custom built-in converts strings to upper case but is not
	// defined for the input "magic".
	compiler := ast.NewCompiler()
	query, err := compiler.QueryCompiler().Compile(ast.MustParseBody(`mybuiltins.upper("custom", x); not mybuiltins.upper("magic", "MAGIC")`))
	if err != nil {
		// Handle error.
	}

	// Evaluate the query.
	q := topdown.NewQuery(query).WithCompiler(compiler)

	q.Iter(ctx, func(qr topdown.QueryResult) error {
		fmt.Println("x:", qr[ast.Var("x")])
		return nil
	})

	// If you add a new built-in function to OPA, you should:
	//
	// 1. Update the Language Reference: http://www.openpolicyagent.org/docs/language-reference.html.
	// 2. Add an integration test to the topdown package.

	// Output:
	//
	// x: "CUSTOM"
}
