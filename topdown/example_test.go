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
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/topdown"
	"github.com/open-policy-agent/opa/topdown/builtins"
	"github.com/open-policy-agent/opa/types"
)

func ExampleEval() {
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
	// of number. Other parameters such as the input, tracing configuration,
	// etc. can be set on the Topdown object.
	t := topdown.New(ctx, query, compiler, store, txn)

	result := []interface{}{}

	// Execute the query and provide a callbakc function to accumulate the results.
	err = topdown.Eval(t, func(t *topdown.Topdown) error {

		// Each variable in the query will have an associated "binding".
		x := t.Binding(ast.Var("x"))

		// Alternatively, you can get a mapping of all bound variables.
		x = t.Vars()[ast.Var("x")]

		// The bindings are ast.Value types so we will convert to a native Go value here.
		v, err := ast.ValueToInterface(x, t)
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
	module, err := ast.ParseModule("my_module.rego", `package opa.example

p[x] { q[x]; not r[x] }
q[y] { a = [1, 2, 3]; y = a[_] }
r[z] { b = [2, 4]; z = b[_] }`,
	)

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

	// Prepare query parameters. In this case, there are no additional documents
	// required by the policy so the input is nil.
	var input ast.Value
	params := topdown.NewQueryParams(ctx, compiler, store, txn, input, ast.MustParseRef("data.opa.example.p"))

	// Execute the query against "p".
	v1, err1 := topdown.Query(params)

	// Inspect the result.
	fmt.Println("v1:", v1[0].Result)
	fmt.Println("err1:", err1)

	// Output:
	// v1: [1 3]
	// err1: <nil>

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
		Name: ast.Var("selective_upper"),
		Args: []types.Type{
			types.S,
			types.S,
		},
		TargetPos: []int{1},
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
	query, err := compiler.QueryCompiler().Compile(ast.MustParseBody(`selective_upper("custom", x); not selective_upper("magic", "MAGIC")`))
	if err != nil {
		// Handle error.
	}

	// Evaluate the query.
	t := topdown.New(ctx, query, compiler, nil, nil)

	topdown.Eval(t, func(t *topdown.Topdown) error {
		fmt.Println("x:", t.Binding(ast.Var("x")))
		return nil
	})

	// If you are adding new built-in functions to upstream OPA, you must also
	// update the [Language
	// Reference](http://www.openpolicyagent.org/documentation/references/language/)
	// and [How Do I Write
	// Policies](http://www.openpolicyagent.org/documentation/how-do-i-write-policies/)
	// documents. In addition, you must add tests for your new built-in. See the
	// existing integration tests in the topdown package.

	// Output:
	//
	// x: "CUSTOM"
}
