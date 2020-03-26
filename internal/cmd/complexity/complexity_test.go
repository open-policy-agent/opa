// Copyright 2019 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package complexity

import (
	"testing"

	"github.com/open-policy-agent/opa/ast"
)

func TestRuntimeComplexityEqualityExpressionMix(t *testing.T) {

	module := `
		package example

		scalar_number {
			a := 1
		}

		scalar_array {
			b := [1,2,3]
		}

		base_ref_gnd {
			c := input.foo
		}

		base_ref_non_gnd {
			d := input.foo[_]
		}

		# repeated to test that result don't contain duplciate refs
		base_ref_non_gnd {
			e := input.foo[_]
		}

		base_ref_non_gnd {
			f := input.bar[_]
			g := input.baz[_]
		}

		virtual_ref_gnd {
			h := non_linear_iteration
		}

		virtual_ref_non_gnd {
			i := non_linear_iteration[_]
		}

		virtual_ref_non_gnd_constant_size {
			j := non_linear_iteration_array[_]
		}

		non_linear_iteration = k {
			k := input.foo[_]
			l := input.bar[_]
		}

		non_linear_iteration = n {
			m := input.foz[x]
			n := m
			o := input.boz[y]
		}

		non_linear_iteration_array = [p,q] {
			p := input.foo[_]
			q := input.bar[_]
		}
		`

	compiler := getCompiler(module)

	expectedScalarNumber := []string{`
Complexity Results for query "data.example.scalar_number == true":
O(1)`}

	expectedScalarArray := []string{`
Complexity Results for query "data.example.scalar_array == true":
O(1)`}

	expectedBaseRefGnd := []string{`
Complexity Results for query "data.example.base_ref_gnd == true":
O(1)`}

	expectedBaseRefNonGndOne := `
Complexity Results for query "data.example.base_ref_non_gnd == true":
O(input.foo + [input.bar * input.baz])`

	expectedBaseRefNonGndTwo := `
Complexity Results for query "data.example.base_ref_non_gnd == true":
O([input.bar * input.baz] + input.foo)`

	expectedBaseRefNonGnd := []string{expectedBaseRefNonGndOne, expectedBaseRefNonGndTwo}

	expectedVirtualRefGndOne := `
Complexity Results for query "data.example.virtual_ref_gnd == true":
O([input.foo * input.bar] + [input.foz * input.boz])`

	expectedVirtualRefGndTwo := `
Complexity Results for query "data.example.virtual_ref_gnd == true":
O([input.foz * input.boz] + [input.foo * input.bar])`

	expectedVirtualRefGnd := []string{expectedVirtualRefGndOne, expectedVirtualRefGndTwo}

	expectedVirtualRefNonGndOne := `
Complexity Results for query "data.example.virtual_ref_non_gnd == true":
O([input.foo * input.bar] + [input.foz * input.boz])`

	expectedVirtualRefNonGndTwo := `
Complexity Results for query "data.example.virtual_ref_non_gnd == true":
O([input.foz * input.boz] + [input.foo * input.bar])`

	expectedVirtualRefNonGnd := []string{expectedVirtualRefNonGndOne, expectedVirtualRefNonGndTwo}

	expectedVirtualRefNonGndConstantSize := []string{`
Complexity Results for query "data.example.virtual_ref_non_gnd_constant_size == true":
O([input.foo * input.bar])`}

	tests := map[string]struct {
		compiler *ast.Compiler
		query    string
		want     []string
	}{
		"eq_scalar":                            {compiler: compiler, query: "data.example.scalar_number == true", want: expectedScalarNumber},
		"eq_array":                             {compiler: compiler, query: "data.example.scalar_array == true", want: expectedScalarArray},
		"eq_base_ref_gnd":                      {compiler: compiler, query: "data.example.base_ref_gnd == true", want: expectedBaseRefGnd},
		"eq_base_ref_non_gnd":                  {compiler: compiler, query: "data.example.base_ref_non_gnd == true", want: expectedBaseRefNonGnd},
		"eq_virtual_ref_gnd":                   {compiler: compiler, query: "data.example.virtual_ref_gnd == true", want: expectedVirtualRefGnd},
		"eq_virtual_ref_non_gnd":               {compiler: compiler, query: "data.example.virtual_ref_non_gnd == true", want: expectedVirtualRefNonGnd},
		"eq_virtual_ref_non_gnd_constant_size": {compiler: compiler, query: "data.example.virtual_ref_non_gnd_constant_size == true", want: expectedVirtualRefNonGndConstantSize},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			report, err := getReport(tc.compiler, tc.query)
			if err != nil {
				t.Fatalf("Unexpected error %v", err.Error())
			}

			if !assertTrue(report.String(), tc.want) {
				t.Fatalf("Expected a result from %v but got %v", tc.want, report.String())
			}
		})
	}
}

func TestRuntimeComplexityEqualityCompleteRule(t *testing.T) {

	module := `
		package example

		deny[u] {
			input.request.foo == myname
			u := sprintf("something here %v", [myname])
		}

		myname = 7 {
			x := p[_]
			y := p[_]
			z := p[_]
		}

		p = x {
			x = y
			y = z
			z = input.bar

		}`

	compiler := getCompiler(module)

	expectedP := `
Complexity Results for query "data.example.p == true":
O(1)`

	expectedMyname := `
Complexity Results for query "data.example.myname == true":
O([[input.bar * input.bar] * input.bar])`

	expectedDeny := `
Complexity Results for query "data.example.deny == true":
O([[input.bar * input.bar] * input.bar])`

	tests := map[string]struct {
		compiler *ast.Compiler
		query    string
		want     string
	}{
		"p":      {compiler: compiler, query: "data.example.p == true", want: expectedP},
		"myname": {compiler: compiler, query: "data.example.myname == true", want: expectedMyname},
		"deny":   {compiler: compiler, query: "data.example.deny == true", want: expectedDeny},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			report, err := getReport(tc.compiler, tc.query)
			if err != nil {
				t.Fatalf("Unexpected error %v", err.Error())
			}

			if report.String() != tc.want {
				t.Fatalf("Expected %v but got %v", tc.want, report.String())
			}
		})
	}
}

func TestRuntimeComplexityEqualityPartialRule(t *testing.T) {

	module := `
		package example

		input_container[c] {
			a := data.baz[b]
			c := input.request.object.spec.containers[_]
		}

		input_container[c] {
			c := input.request.object.spec.init_containers[_]
		}


		input_container_multi[{"someKeyA": d, "someKeyB": e}] {
			d := input.request.object.spec.containers[_]
			e := input.request.object.spec.init_containers[_]
		}

		foo {
			u := data.foo[v]
			c := input_container[container]
		}

		foo_multi {
			u := data.foo[v]
			c := input_container_multi[container]
		}
		`

	compiler := getCompiler(module)

	expectedFooOne := `
Complexity Results for query "data.example.foo == true":
O([data.foo * [[data.baz * input.request.object.spec.containers] + input.request.object.spec.init_containers]])`

	expectedFooTwo := `
Complexity Results for query "data.example.foo == true":
O([data.foo * [input.request.object.spec.init_containers + [data.baz * input.request.object.spec.containers]]])`

	expectedFoo := []string{expectedFooOne, expectedFooTwo}

	expectedFooMulti := []string{`
Complexity Results for query "data.example.foo_multi == true":
O([data.foo * [input.request.object.spec.containers * input.request.object.spec.init_containers]])`}

	tests := map[string]struct {
		compiler *ast.Compiler
		query    string
		want     []string
	}{
		"foo":       {compiler: compiler, query: "data.example.foo == true", want: expectedFoo},
		"foo_multi": {compiler: compiler, query: "data.example.foo_multi == true", want: expectedFooMulti},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			report, err := getReport(tc.compiler, tc.query)
			if err != nil {
				t.Fatalf("Unexpected error %v", err.Error())
			}

			if !assertTrue(report.String(), tc.want) {
				t.Fatalf("Expected a result from %v but got %v", tc.want, report.String())
			}
		})
	}
}

func TestRuntimeComplexityEqualityComprehension(t *testing.T) {

	module := `
		package example

		deny[u] {
			input.request.foo == my_name
			u := sprintf("something here %v", [my_name])
		}

		my_name = 7 {
			x := a[_]
			y := a[_]
			z := a[_]
		}

		a = d {
			d := [x | x := input.foo[_]; y := input.bar[_]]
		}

		s = e {
			e := {x | x := non_linear_iteration[_]}
		}

		non_linear_iteration = u {
			u := input.foo[_]
			v := input.bar[_]
		}

		o := {app.name: hostnames |
				some i
				app := input.apps[i]
				hostnames := [hostname |
							name := input.apps[i].servers[_]
							s := input.sites[_].servers[_]
							s.name == name
							hostname := s.hostname]
		}`

	compiler := getCompiler(module)

	expectedArrayComp := `
Complexity Results for query "data.example.a == true":
O([input.foo * input.bar])`

	expectedMyName := `
Complexity Results for query "data.example.my_name == true":
O([[[input.foo * input.bar] * [input.foo * input.bar]] * [input.foo * input.bar]])`

	expectedDeny := `
Complexity Results for query "data.example.deny":
O([[[input.foo * input.bar] * [input.foo * input.bar]] * [input.foo * input.bar]])`

	expectedSetComp := `
Complexity Results for query "data.example.s":
O([input.foo * input.bar])`

	expectedObjectComp := `
Complexity Results for query "data.example.o":
O([input.apps * [input.apps * input.sites]])`

	tests := map[string]struct {
		compiler *ast.Compiler
		query    string
		want     string
	}{
		"a":       {compiler: compiler, query: "data.example.a == true", want: expectedArrayComp},
		"my_name": {compiler: compiler, query: "data.example.my_name == true", want: expectedMyName},
		"deny":    {compiler: compiler, query: "data.example.deny", want: expectedDeny},
		"s":       {compiler: compiler, query: "data.example.s", want: expectedSetComp},
		"o":       {compiler: compiler, query: "data.example.o", want: expectedObjectComp},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			report, err := getReport(tc.compiler, tc.query)
			if err != nil {
				t.Fatalf("Unexpected error %v", err.Error())
			}

			if report.String() != tc.want {
				t.Fatalf("Expected %v but got %v", tc.want, report.String())
			}
		})
	}
}

func TestRuntimeComplexityUserFunctions(t *testing.T) {
	module := `
		package example

		array_arg {
			x := input.foo
			y := input.bar
			t := get_blah([x,y])
			t[_]
		}

		var_arg {
			x := input.foo
			y := input.bar
			z := [x, y]
			t := get_blah(z)
			t[_]
		}

		get_blah([a,b]) = x {
			x = a
			x[_] = 1
		}

		deny[reason] {
			t := get_foo(input.request.foo)
			x := t[_]
			reason := x.name
		}

		foo {
			t1 := get_foo(input.request.foo)
			t2 := get_foo(input.request.bar)
			t3 := get_foo(input.request.baz)
		}

		get_foo(request) = result {
			x := request.object.spec[_]
			x.kind.kind == "foo"
			result := request.object.spec
		}`

	compiler := getCompiler(module)

	expectedFoo := `
Complexity Results for query "data.example.foo == true":
O(input.request.foo + input.request.bar + input.request.baz)`

	expectedDeny := `
Complexity Results for query "data.example.deny":
O(input.request.foo)`

	expectedArrayArg := `
Complexity Results for query "data.example.array_arg":
O(input.foo)`

	expectedVarArg := `
Complexity Results for query "data.example.var_arg":
O(input.foo)`

	tests := map[string]struct {
		compiler *ast.Compiler
		query    string
		want     string
	}{
		"foo":       {compiler: compiler, query: "data.example.foo == true", want: expectedFoo},
		"deny":      {compiler: compiler, query: "data.example.deny", want: expectedDeny},
		"array_arg": {compiler: compiler, query: "data.example.array_arg", want: expectedArrayArg},
		"var_arg":   {compiler: compiler, query: "data.example.var_arg", want: expectedVarArg},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			report, err := getReport(tc.compiler, tc.query)
			if err != nil {
				t.Fatalf("Unexpected error %v", err.Error())
			}

			if report.String() != tc.want {
				t.Fatalf("Expected %v but got %v", tc.want, report.String())
			}
		})
	}
}

func TestRuntimeComplexityUserFunctionsWithVarArg(t *testing.T) {
	module := `
		package example

		array_arg {
			x := input.foo
			y := input.bar
			t := foo([x,y])
			t[_]
		}

		var_arg  = t {
			x := input.foo
			y := input.bar
			z := [x, y]
			t := foo(z)
			t[_]
		}

		foo(arr) = x {
			x = arr[0]
			x[_] = 1
		}

		my_name = 7 {
			x := var_arg[_]
			y := var_arg[_]
		}`

	compiler := getCompiler(module)

	expectedArrayArg := `
Complexity Results for query "data.example.array_arg":
O(input.foo + input.bar)`

	expectedVarArg := `
Complexity Results for query "data.example.var_arg":
O(input.foo + input.bar)`

	expectedMyName := `
Complexity Results for query "data.example.my_name":
O([[input.foo + input.bar] * [input.foo + input.bar]])`

	tests := map[string]struct {
		compiler *ast.Compiler
		query    string
		want     string
	}{
		"array_arg": {compiler: compiler, query: "data.example.array_arg", want: expectedArrayArg},
		"var_arg":   {compiler: compiler, query: "data.example.var_arg", want: expectedVarArg},
		"my_name":   {compiler: compiler, query: "data.example.my_name", want: expectedMyName},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			report, err := getReport(tc.compiler, tc.query)
			if err != nil {
				t.Fatalf("Unexpected error %v", err.Error())
			}

			if report.String() != tc.want {
				t.Fatalf("Expected %v but got %v", tc.want, report.String())
			}
		})
	}
}

func TestRuntimeComplexitySingleTerm(t *testing.T) {

	module := `
		package example

		foo {
			x = input.foo
			x[_] = a
		}`

	compiler := getCompiler(module)

	expectedBaseRefGnd := `
Complexity Results for query "input.foo":
O(1)`

	expectedBaseRefNonGnd := `
Complexity Results for query "input.foo[_]":
O(input.foo)`

	expectedFoo := `
Complexity Results for query "data.example.foo":
O(input.foo)`

	tests := map[string]struct {
		compiler *ast.Compiler
		query    string
		want     string
	}{
		"base_ref_gnd":     {compiler: compiler, query: "input.foo", want: expectedBaseRefGnd},
		"base_ref_non_gnd": {compiler: compiler, query: "input.foo[_]", want: expectedBaseRefNonGnd},
		"foo":              {compiler: compiler, query: "data.example.foo", want: expectedFoo},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			report, err := getReport(tc.compiler, tc.query)
			if err != nil {
				t.Fatalf("Unexpected error %v", err.Error())
			}

			if report.String() != tc.want {
				t.Fatalf("Expected %v but got %v", tc.want, report.String())
			}
		})
	}
}

func getReport(compiler *ast.Compiler, query string) (*Report, error) {
	calculator := New().WithCompiler(compiler).WithQuery(query)
	report, err := calculator.Calculate()
	if err != nil {
		return nil, err
	}
	return report, nil
}

func getCompiler(module string) *ast.Compiler {
	parsedModule := ast.MustParseModule(module)
	modules := map[string]*ast.Module{"test": parsedModule}

	compiler := ast.NewCompiler()
	compiler.Compile(modules)

	return compiler
}

func assertTrue(actual string, expected []string) bool {
	for _, r := range expected {
		if actual == r {
			return true
		}
	}
	return false
}
