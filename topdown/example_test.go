// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown_test

import (
	"fmt"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/topdown"
)

func ExampleEval() {

	// Define a dummy query and some data that the query will execute against.
	query, err := ast.CompileQuery("data.a[_] = x, x >= 2")
	if err != nil {
		fmt.Println("Compile error:", err)
	}

	ds := storage.NewDataStoreFromJSONObject(map[string]interface{}{
		"a": []interface{}{float64(1), float64(2), float64(3), float64(4)},
	})

	// Prepare the evaluation parameters. Evaluation executes against the policy engine's
	// storage. In this case, we seed the storage with a single array of number. Other parameters
	// such as globals, tracing configuration, etc. can be set on the context. See the Context
	// documentation for more details.
	ctx := topdown.NewContext(query, ds)

	result := []interface{}{}

	// Execute the query and provide a callbakc function to accumulate the results.
	err = topdown.Eval(ctx, func(ctx *topdown.Context) error {

		// Each variable in the query will have an associated "binding" in the context.
		x := ctx.Binding(ast.Var("x"))

		// The bindings are ast.Value types so we will convert to a native Go value here.
		v, err := topdown.ValueToInterface(x, ctx)
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

	// Define a dummy module with rules that produce documents that we will query below.
	mod, err := ast.CompileModule(`

	    package opa.example

	    p[x] :- q[x], not r[x]
	    q[y] :- a = [1,2,3], y = a[_]
	    r[z] :- b = [2,4], z = b[_]

	`)

	if err != nil {
		fmt.Println("Compile error:", err)
	}

	// Initialize the policy engine's storage.
	ds := storage.NewDataStore()
	ps := storage.NewPolicyStore(ds, "")
	if err := ps.Add("my_module", mod, nil, false); err != nil {
		fmt.Println("Add error:", err)
	}

	// Prepare the query parameters. Queries execute against the policy engine's storage and can
	// accept additional documents (which are referred to as "globals"). In this case we have no
	// additional documents.
	globals := storage.NewBindings()
	params := topdown.NewQueryParams(ds, globals, []interface{}{"opa", "example", "p"})

	// Execute the query against "p".
	v1, err1 := topdown.Query(params)

	// Inspect the result.
	fmt.Println("v1:", v1)
	fmt.Println("err1:", err1)

	// Output:
	// v1: [1 3]
	// err1: <nil>

}
