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

	expected_scalar_number := []string{`
Complexity Results for query "equal(data.example.scalar_number, true)":
O(1)`}

	expected_scalar_array := []string{`
Complexity Results for query "equal(data.example.scalar_array, true)":
O(1)`}

	expected_base_ref_gnd := []string{`
Complexity Results for query "equal(data.example.base_ref_gnd, true)":
O(1)`}

	expected_base_ref_non_gnd_one := `
Complexity Results for query "equal(data.example.base_ref_non_gnd, true)":
O(input.foo + [input.bar * input.baz])`

	expected_base_ref_non_gnd_two := `
Complexity Results for query "equal(data.example.base_ref_non_gnd, true)":
O([input.bar * input.baz] + input.foo)`

	expected_base_ref_non_gnd := []string{expected_base_ref_non_gnd_one, expected_base_ref_non_gnd_two}

	expected_virtual_ref_gnd_one := `
Complexity Results for query "equal(data.example.virtual_ref_gnd, true)":
O([input.foo * input.bar] + [input.foz * input.boz])`

	expected_virtual_ref_gnd_two := `
Complexity Results for query "equal(data.example.virtual_ref_gnd, true)":
O([input.foz * input.boz] + [input.foo * input.bar])`

	expected_virtual_ref_gnd := []string{expected_virtual_ref_gnd_one, expected_virtual_ref_gnd_two}

	expected_virtual_ref_non_gnd_one := `
Complexity Results for query "equal(data.example.virtual_ref_non_gnd, true)":
O([input.foo * input.bar] + [input.foz * input.boz])`

	expected_virtual_ref_non_gnd_two := `
Complexity Results for query "equal(data.example.virtual_ref_non_gnd, true)":
O([input.foz * input.boz] + [input.foo * input.bar])`

	expected_virtual_ref_non_gnd := []string{expected_virtual_ref_non_gnd_one, expected_virtual_ref_non_gnd_two}

	expected_virtual_ref_non_gnd_constant_size := []string{`
Complexity Results for query "equal(data.example.virtual_ref_non_gnd_constant_size, true)":
O([input.foo * input.bar])`}

	tests := map[string]struct {
		compiler *ast.Compiler
		query    string
		want     []string
	}{
		"eq_scalar":                            {compiler: compiler, query: "data.example.scalar_number == true", want: expected_scalar_number},
		"eq_array":                             {compiler: compiler, query: "data.example.scalar_array == true", want: expected_scalar_array},
		"eq_base_ref_gnd":                      {compiler: compiler, query: "data.example.base_ref_gnd == true", want: expected_base_ref_gnd},
		"eq_base_ref_non_gnd":                  {compiler: compiler, query: "data.example.base_ref_non_gnd == true", want: expected_base_ref_non_gnd},
		"eq_virtual_ref_gnd":                   {compiler: compiler, query: "data.example.virtual_ref_gnd == true", want: expected_virtual_ref_gnd},
		"eq_virtual_ref_non_gnd":               {compiler: compiler, query: "data.example.virtual_ref_non_gnd == true", want: expected_virtual_ref_non_gnd},
		"eq_virtual_ref_non_gnd_constant_size": {compiler: compiler, query: "data.example.virtual_ref_non_gnd_constant_size == true", want: expected_virtual_ref_non_gnd_constant_size},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			report := getReport(tc.compiler, tc.query)
			if !assertTrue(report.String(), tc.want) {
				t.Fatalf("Expected a result from %v but got %v", tc.want, report.String())
			}
		})
	}
}

func TestRuntimeComplexityEqualityCompleteRules(t *testing.T) {

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

	expected_p := `
Complexity Results for query "equal(data.example.p, true)":
O(1)`

	expected_myname := `
Complexity Results for query "equal(data.example.myname, true)":
O([[input.bar * input.bar] * input.bar])`

	expected_deny := `
Complexity Results for query "equal(data.example.deny, true)":
O([[input.bar * input.bar] * input.bar])`

	tests := map[string]struct {
		compiler *ast.Compiler
		query    string
		want     string
	}{
		"p":      {compiler: compiler, query: "data.example.p == true", want: expected_p},
		"myname": {compiler: compiler, query: "data.example.myname == true", want: expected_myname},
		"deny":   {compiler: compiler, query: "data.example.deny == true", want: expected_deny},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			report := getReport(tc.compiler, tc.query)
			if report.String() != tc.want {
				t.Fatalf("Expected %v but got %v", tc.want, report.String())
			}
		})
	}
}

func TestRuntimeComplexityEqualityPartialRules(t *testing.T) {

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

	expected_foo_one := `
Complexity Results for query "equal(data.example.foo, true)":
O([data.foo * [[data.baz * input.request.object.spec.containers] + input.request.object.spec.init_containers]])`

	expected_foo_two := `
Complexity Results for query "equal(data.example.foo, true)":
O([data.foo * [input.request.object.spec.init_containers + [data.baz * input.request.object.spec.containers]]])`

	expected_foo := []string{expected_foo_one, expected_foo_two}

	expected_foo_multi := []string{`
Complexity Results for query "equal(data.example.foo_multi, true)":
O([data.foo * [input.request.object.spec.containers * input.request.object.spec.init_containers]])`}

	tests := map[string]struct {
		compiler *ast.Compiler
		query    string
		want     []string
	}{
		"foo":       {compiler: compiler, query: "data.example.foo == true", want: expected_foo},
		"foo_multi": {compiler: compiler, query: "data.example.foo_multi == true", want: expected_foo_multi},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			report := getReport(tc.compiler, tc.query)
			if !assertTrue(report.String(), tc.want) {
				t.Fatalf("Expected a result from %v but got %v", tc.want, report.String())
			}
		})
	}
}

func getReport(compiler *ast.Compiler, query string) *Report {
	calculator := New().WithCompiler(compiler).WithQuery(query)
	report, _ := calculator.Calculate()
	return report
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
