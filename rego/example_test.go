// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// nolint // example code
package rego_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/open-policy-agent/opa/logging"
	"github.com/open-policy-agent/opa/storage/disk"
	"github.com/open-policy-agent/opa/topdown/builtins"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/rego"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/storage/inmem"
	"github.com/open-policy-agent/opa/topdown"
	"github.com/open-policy-agent/opa/types"
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

func ExampleRego_Eval_allowed() {

	ctx := context.Background()

	// Create query that returns a single boolean value.
	rego := rego.New(
		rego.Query("data.authz.allow"),
		rego.Module("example.rego",
			`package authz

default allow = false
allow {
	input.open == "sesame"
}`,
		),
		rego.Input(map[string]interface{}{"open": "sesame"}),
	)

	// Run evaluation.
	rs, err := rego.Eval(ctx)
	if err != nil {
		panic(err)
	}

	// Inspect result.
	fmt.Println("allowed:", rs.Allowed())

	// Output:
	//
	//	allowed: true
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
	// bindings["x"]: goodbye (i=0)
	// value: bob (i=0)
	// bindings["x"]: hello (i=1)
	// value: alice (i=1)
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

	// Compile the module. The keys are used as identifiers in error messages.
	compiler, err := ast.CompileModules(map[string]string{
		"example.rego": module,
	})

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
	fmt.Println("allowed:", rs.Allowed()) // helper method

	// Output:
	//
	// len: 1
	// value: true
	// allowed: true
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
	if err != nil {
		// Handle error.
	}

	// Inspect the result.
	fmt.Println("value:", rs[0].Expressions[0].Value)

	// Output:
	//
	// value: [dogs clouds]
}

func ExampleRego_Eval_persistent_storage() {

	ctx := context.Background()

	data := `{
        "example": {
            "users": {
				"alice": {
					"likes": ["dogs", "clouds"]
				},
				"bob": {
					"likes": ["pizza", "cats"]
				}
			}
        }
    }`

	var json map[string]interface{}

	err := util.UnmarshalJSON([]byte(data), &json)
	if err != nil {
		// Handle error.
	}

	// Manually create a persistent storage-layer in a temporary directory.
	rootDir, err := ioutil.TempDir("", "rego_example")
	if err != nil {
		panic(err)
	}

	defer os.RemoveAll(rootDir)

	// Configure the store to partition data at `/example/users` so that each
	// user's data is stored on a different row. Assuming the policy only reads
	// data for a single user to process the policy query, OPA can avoid loading
	// _all_ user data into memory this way.
	store, err := disk.New(ctx, logging.NewNoOpLogger(), nil, disk.Options{
		Dir:        rootDir,
		Partitions: []storage.Path{{"example", "user"}},
	})
	if err != nil {
		// Handle error.
	}

	err = storage.WriteOne(ctx, store, storage.AddOp, storage.Path{}, json)
	if err != nil {
		// Handle error
	}

	// Run a query that returns the value
	rs, err := rego.New(
		rego.Query(`data.example.users["alice"].likes`),
		rego.Store(store)).Eval(ctx)
	if err != nil {
		// Handle error.
	}

	// Inspect the result.
	fmt.Println("value:", rs[0].Expressions[0].Value)

	// Re-open the store in the same directory.
	store.Close(ctx)

	store2, err := disk.New(ctx, logging.NewNoOpLogger(), nil, disk.Options{
		Dir:        rootDir,
		Partitions: []storage.Path{{"example", "user"}},
	})
	if err != nil {
		// Handle error.
	}

	// Run the same query with a new store.
	rs, err = rego.New(
		rego.Query(`data.example.users["alice"].likes`),
		rego.Store(store2)).Eval(ctx)
	if err != nil {
		// Handle error.
	}

	// Inspect the result and observe the same result.
	fmt.Println("value:", rs[0].Expressions[0].Value)

	// Output:
	//
	// value: [dogs clouds]
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
	// the hood, the rego package will create it's own transaction to
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

	// Run evaluation INSIDE the transaction.
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
	case ast.Errors:
		for _, e := range err {
			fmt.Println("code:", e.Code)
			fmt.Println("row:", e.Location.Row)
			fmt.Println("filename:", e.Location.File)
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

func ExampleRego_PartialResult() {

	ctx := context.Background()

	// Define a role-based access control (RBAC) policy that decides whether to
	// allow or deny requests. Requests are allowed if the user is bound to a
	// role that grants permission to perform the operation on the resource.
	module := `
		package example

		import data.bindings
		import data.roles

		default allow = false

		allow {
			user_has_role[role_name]
			role_has_permission[role_name]
		}

		user_has_role[role_name] {
			b = bindings[_]
			b.role = role_name
			b.user = input.subject.user
		}

		role_has_permission[role_name] {
			r = roles[_]
			r.name = role_name
			match_with_wildcard(r.operations, input.operation)
			match_with_wildcard(r.resources, input.resource)
		}

		match_with_wildcard(allowed, value) {
			allowed[_] = "*"
		}

		match_with_wildcard(allowed, value) {
			allowed[_] = value
		}
	`

	// Define dummy roles and role bindings for the example. In real-world
	// scenarios, this data would be pushed or pulled into the service
	// embedding OPA either from an external API or configuration file.
	store := inmem.NewFromReader(bytes.NewBufferString(`{
		"roles": [
			{
				"resources": ["documentA", "documentB"],
				"operations": ["read"],
				"name": "analyst"
			},
			{
				"resources": ["*"],
				"operations": ["*"],
				"name": "admin"
			}
		],
		"bindings": [
			{
				"user": "bob",
				"role": "admin"
			},
			{
				"user": "alice",
				"role": "analyst"
			}
		]
	}`))

	// Prepare and run partial evaluation on the query. The result of partial
	// evaluation can be cached for performance. When the data or policy
	// change, partial evaluation should be re-run.
	r := rego.New(
		rego.Query("data.example.allow"),
		rego.Module("example.rego", module),
		rego.Store(store),
	)

	pr, err := r.PartialResult(ctx)
	if err != nil {
		// Handle error.
	}

	// Define example inputs (representing requests) that will be used to test
	// the policy.
	examples := []map[string]interface{}{
		{
			"resource":  "documentA",
			"operation": "write",
			"subject": map[string]interface{}{
				"user": "bob",
			},
		},
		{
			"resource":  "documentB",
			"operation": "write",
			"subject": map[string]interface{}{
				"user": "alice",
			},
		},
		{
			"resource":  "documentB",
			"operation": "read",
			"subject": map[string]interface{}{
				"user": "alice",
			},
		},
	}

	for i := range examples {

		// Prepare and run normal evaluation from the result of partial
		// evaluation.
		r := pr.Rego(
			rego.Input(examples[i]),
		)

		rs, err := r.Eval(ctx)

		if err != nil || len(rs) != 1 || len(rs[0].Expressions) != 1 {
			// Handle erorr.
		} else {
			fmt.Printf("input %d allowed: %v\n", i+1, rs[0].Expressions[0].Value)
		}
	}

	// Output:
	//
	// input 1 allowed: true
	// input 2 allowed: false
	// input 3 allowed: true
}

func ExampleRego_Partial() {

	ctx := context.Background()

	// Define a simple policy for example purposes.
	module := `package test

	allow {
		input.method = read_methods[_]
		input.path = ["reviews", user]
		input.user = user
	}

	allow {
		input.method = read_methods[_]
		input.path = ["reviews", _]
		input.is_admin
	}

	read_methods = ["GET"]
	`

	r := rego.New(rego.Query("data.test.allow == true"), rego.Module("example.rego", module))
	pq, err := r.Partial(ctx)
	if err != nil {
		// Handle error.
	}

	// Inspect result.
	for i := range pq.Queries {
		fmt.Printf("Query #%d: %v\n", i+1, pq.Queries[i])
	}

	// Output:
	//
	// Query #1: "GET" = input.method; input.path = ["reviews", _]; input.is_admin
	// Query #2: "GET" = input.method; input.path = ["reviews", user3]; user3 = input.user
}

func ExampleRego_Eval_trace_simple() {

	ctx := context.Background()

	// Create very simple query that binds a single variable and enables tracing.
	r := rego.New(
		rego.Query("x = 1"),
		rego.Trace(true),
	)

	// Run evaluation.
	r.Eval(ctx)

	// Inspect results.
	rego.PrintTraceWithLocation(os.Stdout, r)

	// Output:
	//
	// query:1     Enter x = 1
	// query:1     | Eval x = 1
	// query:1     | Exit x = 1
	// query:1     Redo x = 1
	// query:1     | Redo x = 1
}

func ExampleRego_Eval_tracer() {

	ctx := context.Background()

	buf := topdown.NewBufferTracer()

	// Create very simple query that binds a single variable and provides a tracer.
	rego := rego.New(
		rego.Query("x = 1"),
		rego.QueryTracer(buf),
	)

	// Run evaluation.
	rego.Eval(ctx)

	// Inspect results.
	topdown.PrettyTraceWithLocation(os.Stdout, *buf)

	// Output:
	//
	// query:1     Enter x = 1
	// query:1     | Eval x = 1
	// query:1     | Exit x = 1
	// query:1     Redo x = 1
	// query:1     | Redo x = 1
}

func ExampleRego_PrepareForEval() {
	ctx := context.Background()

	// Create a simple query
	r := rego.New(
		rego.Query("input.x == 1"),
	)

	// Prepare for evaluation
	pq, err := r.PrepareForEval(ctx)

	if err != nil {
		// Handle error.
	}

	// Raw input data that will be used in the first evaluation
	input := map[string]interface{}{"x": 2}

	// Run the evaluation
	rs, err := pq.Eval(ctx, rego.EvalInput(input))

	if err != nil {
		// Handle error.
	}

	// Inspect results.
	fmt.Println("initial result:", rs[0].Expressions[0])

	// Update input
	input["x"] = 1

	// Run the evaluation with new input
	rs, err = pq.Eval(ctx, rego.EvalInput(input))

	if err != nil {
		// Handle error.
	}

	// Inspect results.
	fmt.Println("updated result:", rs[0].Expressions[0])

	// Output:
	//
	// initial result: false
	// updated result: true
}

func ExampleRego_PrepareForPartial() {

	ctx := context.Background()

	// Define a simple policy for example purposes.
	module := `package test

	allow {
		input.method = read_methods[_]
		input.path = ["reviews", user]
		input.user = user
	}

	allow {
		input.method = read_methods[_]
		input.path = ["reviews", _]
		input.is_admin
	}

	read_methods = ["GET"]
	`

	r := rego.New(
		rego.Query("data.test.allow == true"),
		rego.Module("example.rego", module),
	)

	pq, err := r.PrepareForPartial(ctx)
	if err != nil {
		// Handle error.
	}

	pqs, err := pq.Partial(ctx)
	if err != nil {
		// Handle error.
	}

	// Inspect result
	fmt.Println("First evaluation")
	for i := range pqs.Queries {
		fmt.Printf("Query #%d: %v\n", i+1, pqs.Queries[i])
	}

	// Evaluate with specified input
	exampleInput := map[string]string{
		"method": "GET",
	}

	// Evaluate again with different input and unknowns
	pqs, err = pq.Partial(ctx,
		rego.EvalInput(exampleInput),
		rego.EvalUnknowns([]string{"input.user", "input.is_admin", "input.path"}),
	)
	if err != nil {
		// Handle error.
	}

	// Inspect result
	fmt.Println("Second evaluation")
	for i := range pqs.Queries {
		fmt.Printf("Query #%d: %v\n", i+1, pqs.Queries[i])
	}

	// Output:
	//
	// First evaluation
	// Query #1: "GET" = input.method; input.path = ["reviews", _]; input.is_admin
	// Query #2: "GET" = input.method; input.path = ["reviews", user3]; user3 = input.user
	// Second evaluation
	// Query #1: input.path = ["reviews", _]; input.is_admin
	// Query #2: input.path = ["reviews", user3]; user3 = input.user
}

func ExampleRego_custom_functional_builtin() {

	r := rego.New(
		// An example query that uses a custom function.
		rego.Query(`x = trim_and_split("/foo/bar/baz/", "/")`),

		// A custom function that trims and splits strings on the same delimiter.
		rego.Function2(
			&rego.Function{
				Name: "trim_and_split",
				Decl: types.NewFunction(
					types.Args(types.S, types.S), // two string inputs
					types.NewArray(nil, types.S), // variable-length string array output
				),
			},
			func(_ rego.BuiltinContext, a, b *ast.Term) (*ast.Term, error) {

				str, ok1 := a.Value.(ast.String)
				delim, ok2 := b.Value.(ast.String)

				// The function is undefined for non-string inputs. Built-in
				// functions should only return errors in unrecoverable cases.
				if !ok1 || !ok2 {
					return nil, nil
				}

				result := strings.Split(strings.Trim(string(str), string(delim)), string(delim))

				arr := make([]*ast.Term, len(result))
				for i := range result {
					arr[i] = ast.StringTerm(result[i])
				}

				return ast.ArrayTerm(arr...), nil
			},
		),
	)

	rs, err := r.Eval(context.Background())
	if err != nil {
		// handle error
	}

	fmt.Println(rs[0].Bindings["x"])

	// Output:
	//
	// [foo bar baz]
}

func ExampleRego_custom_function_caching() {
	i := 0

	r := rego.New(
		// An example query that uses a custom function.
		rego.Query(`x = mycounter("foo"); y = mycounter("foo")`),

		// A custom function that uses caching.
		rego.FunctionDyn(
			&rego.Function{
				Name:    "mycounter",
				Memoize: true,
				Decl: types.NewFunction(
					types.Args(types.S), // one string input
					types.N,             // one number output
				),
			},
			func(_ topdown.BuiltinContext, args []*ast.Term) (*ast.Term, error) {
				i++
				return ast.IntNumberTerm(i), nil
			},
		),
	)

	rs, err := r.Eval(context.Background())
	if err != nil {
		// handle error
	}

	fmt.Println("x:", rs[0].Bindings["x"])
	fmt.Println("y:", rs[0].Bindings["y"])

	// Output:
	//
	// x: 1
	// y: 1
}

func ExampleRego_custom_function_nondeterministic() {
	ndbCache := builtins.NDBCache{}
	r := rego.New(
		// An example query that uses a custom function.
		rego.Query(`x = myrandom()`),

		// A custom function that uses caching.
		rego.FunctionDyn(
			&rego.Function{
				Name:             "myrandom",
				Memoize:          true,
				Nondeterministic: true,
				Decl: types.NewFunction(
					nil,     // one string input
					types.N, // one number output
				),
			},
			func(_ topdown.BuiltinContext, args []*ast.Term) (*ast.Term, error) {
				i := 55555
				return ast.IntNumberTerm(i), nil
			},
		),
		rego.NDBuiltinCache(ndbCache),
	)

	rs, err := r.Eval(context.Background())
	if err != nil {
		// handle error
	}

	// Check the binding, and what the NDBCache saw.
	// This ensures the Nondeterministic flag propagated through correctly.
	fmt.Println("x:", rs[0].Bindings["x"])
	fmt.Println("NDBCache:", ndbCache["myrandom"])

	// Output:
	//
	// x: 55555
	// NDBCache: {[]: 55555}
}

func ExampleRego_custom_function_global() {

	decl := &rego.Function{
		Name: "trim_and_split",
		Decl: types.NewFunction(
			types.Args(types.S, types.S), // two string inputs
			types.NewArray(nil, types.S), // variable-length string array output
		),
	}

	impl := func(_ rego.BuiltinContext, a, b *ast.Term) (*ast.Term, error) {

		str, ok1 := a.Value.(ast.String)
		delim, ok2 := b.Value.(ast.String)

		// The function is undefined for non-string inputs. Built-in
		// functions should only return errors in unrecoverable cases.
		if !ok1 || !ok2 {
			return nil, nil
		}

		result := strings.Split(strings.Trim(string(str), string(delim)), string(delim))

		arr := make([]*ast.Term, len(result))
		for i := range result {
			arr[i] = ast.StringTerm(result[i])
		}

		return ast.ArrayTerm(arr...), nil
	}

	// The rego package exports helper functions for different arities and a
	// special version of the function that accepts a dynamic number.
	rego.RegisterBuiltin2(decl, impl)

	r := rego.New(
		// An example query that uses a custom function.
		rego.Query(`x = trim_and_split("/foo/bar/baz/", "/")`),
	)

	rs, err := r.Eval(context.Background())
	if err != nil {
		// handle error
	}

	fmt.Println(rs[0].Bindings["x"])

	// Output:
	//
	// [foo bar baz]
}

func ExampleRego_print_statements() {

	var buf bytes.Buffer

	r := rego.New(
		rego.Query("data.example.rule_containing_print_call"),
		rego.Module("example.rego", `
			package example

			rule_containing_print_call {
				print("input.foo is:", input.foo, "and input.bar is:", input.bar)
			}
		`),
		rego.Input(map[string]interface{}{
			"foo": 7,
		}),
		rego.EnablePrintStatements(true),
		rego.PrintHook(topdown.NewPrintHook(&buf)),
	)

	_, err := r.Eval(context.Background())
	if err != nil {
		// handle error
	}

	fmt.Println("buf:", buf.String())

	// Output:
	//
	// buf: input.foo is: 7 and input.bar is: <undefined>
}
