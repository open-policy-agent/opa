// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast_test

import (
	"fmt"

	"github.com/open-policy-agent/opa/ast"
)

func ExampleCompiler_Compile() {

	// Define an input module that will be compiled.
	exampleModule := `

		package opa.example

		import data.foo
		import bar

		p[x] :- foo[x], not bar[x], x >= min_x

		min_x = 100

	`

	// Parse the input module to obtain the AST representation.
	mod, err := ast.ParseModule("my_module", exampleModule)
	if err != nil {
		fmt.Println("Parse error:", err)
	}

	// Create a new compiler instance and compile the module.
	c := ast.NewCompiler()

	mods := map[string]*ast.Module{
		"my_module": mod,
	}

	if c.Compile(mods); c.Failed() {
		fmt.Println("Compile error:", c.Errors)
	}

	fmt.Println("Expr 1:", c.Modules["my_module"].Rules[0].Body[0])
	fmt.Println("Expr 2:", c.Modules["my_module"].Rules[0].Body[1])
	fmt.Println("Expr 3:", c.Modules["my_module"].Rules[0].Body[2])

	// Output:
	//
	// Expr 1: data.foo[x]
	// Expr 2: not bar[x]
	// Expr 3: gte(x, data.opa.example.min_x)
}

func ExampleQueryCompiler_Compile() {

	// Define an input module that will be compiled.
	exampleModule := `

		package opa.example

		import data.foo
		import bar

		p[x] :- foo[x], not bar[x], x >= min_x

		min_x = 100

	`

	// Parse the input module to obtain the AST representation.
	mod, err := ast.ParseModule("my_module", exampleModule)
	if err != nil {
		fmt.Println("Parse error:", err)
	}

	// Create a new compiler instance and compile the module.
	c := ast.NewCompiler()

	mods := map[string]*ast.Module{
		"my_module": mod,
	}

	if c.Compile(mods); c.Failed() {
		fmt.Println("Compile error:", c.Errors)
	}

	// Obtain the QueryCompiler from the compiler instance. Note, we will
	// compile this query within the context of the opa.example package and
	// declare that a query input named "queryinput" must be supplied.
	qc := c.QueryCompiler().
		WithContext(
			ast.NewQueryContext(
				// Note, the ast.MustParse<X> functions are meant for test
				// purposes only. They will panic if an error occurs. Prefer the
				// ast.Parse<X> functions that return meaningful error messages
				// instead.
				ast.MustParsePackage("package opa.example"),
				ast.MustParseImports("import queryinput"),
			))

	// Parse the input query to obtain the AST representation.
	query, err := ast.ParseBody("p[x], x < queryinput")
	if err != nil {
		fmt.Println("Parse error:", err)
	}

	compiled, err := qc.Compile(query)
	if err != nil {
		fmt.Println("Compile error:", err)
	}

	fmt.Println("Compiled:", compiled)

	// Output:
	//
	// Compiled: data.opa.example.p[x], lt(x, queryinput)
}
