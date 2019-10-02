// Copyright 2019 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package analyze

import (
	"testing"

	"github.com/open-policy-agent/opa/ast"
)

func TestRuntimeComplexityAssignment(t *testing.T) {

	module := ast.MustParseModule(`
		package example

		allow {
			x := 1
			y := 2
		}
	`)

	actual := CalculateRuntimeComplexity(module)

	expected := "O(1)"

	if len(actual.Result["allow"]) != 1 {
		t.Fatalf("Expected 1 result for rule \"allow\" but got %v", len(actual.Result["allow"]))
	}

	if expected != actual.Result["allow"][0] {
		t.Fatalf("Expected runtime complexity %v but got %v", expected, actual.Result["allow"][0])
	}
}

func TestRuntimeComplexityRuleWithHelper(t *testing.T) {

	module := ast.MustParseModule(`
		package example

		expect_container_resource_requirements[reason] {
			some container
			input_container[container]
			not container.resources.requests.cpu
			not container.resources.limits.cpu
			reason := sprintf("Resource %v container %v is missing CPU requirements", ["some_id", container.name])
		}

		input_container[c] {
			c := input.request.object.spec.containers[_]
		}

		input_container[c] {
			c := input.request.object.spec.initContainers[_]
		}

		input_container[c] {
			c := input.request.object.spec.specialContainers[_]
		}
	`)

	actual := CalculateRuntimeComplexity(module)

	if len(actual.Result["input_container"]) != 3 {
		t.Fatalf("Expected 3 results for rule \"input_container\" but got %v", len(actual.Result["input_container"]))
	}

	expected := map[string]bool{
		"O(input.request.object.spec.containers)":        true,
		"O(input.request.object.spec.initContainers)":    true,
		"O(input.request.object.spec.specialContainers)": true}
	for _, val := range actual.Result["input_container"] {
		if _, ok := expected[val]; !ok {
			t.Fatalf("Expected runtime complexity for rule \"input_container\" %v but got %v", expected, val)
		}
	}

	if len(actual.Result["expect_container_resource_requirements"]) != 1 {
		t.Fatalf("Expected 1 result for rule \"expect_container_resource_requirements\" but got %v", len(actual.Result["expect_container_resource_requirements"]))
	}

	expectedRes := "[O(input.request.object.spec.containers) + O(input.request.object.spec.initContainers) + O(input.request.object.spec.specialContainers)]"
	if expectedRes != actual.Result["expect_container_resource_requirements"][0] {
		t.Fatalf("Expected runtime complexity for rule \"expect_container_resource_requirements\" %v but got %v", expectedRes, actual.Result["expect_container_resource_requirements"][0])
	}
}

func TestRuntimeComplexityLinearIteration(t *testing.T) {

	module1 := ast.MustParseModule(`
			package example

			linear_iteration {
				input.foo[y].bar[z]
			}
		`)

	module2 := ast.MustParseModule(`
		package example

		linear_iteration_local_var {
			x := input.foo[y]
			x.bar[z]
		}
	`)

	tests := map[string]struct {
		input *ast.Module
		want  string
	}{
		"linear_iteration":           {input: module1, want: "O(input.foo)"},
		"linear_iteration_local_var": {input: module2, want: "O(input.foo)"},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			actual := CalculateRuntimeComplexity(tc.input)
			if tc.want != actual.Result[name][0] {
				t.Fatalf("Expected runtime complexity %v but got %v", tc.want, actual.Result[name][0])
			}
		})
	}
}

func TestRuntimeComplexityNonLinearIteration(t *testing.T) {

	module1 := ast.MustParseModule(`
		package example

		non_linear_iteration_w_input {
			input.foo[x]
			input.bar[y]
		}
	`)

	module2 := ast.MustParseModule(`
		package example

		non_linear_iteration_w_rule {
			input.foo[x]
			block_master_toleration[reason]
			input.bar[y]
		}
	`)

	module3 := ast.MustParseModule(`
		package example

		non_linear_iteration_w_multiple_rule {
			input.foo[x]
			expect_container_resource_requirements[reason]
			input.bar[y]
		}

		expect_container_resource_requirements[reason] {
		   some container
		   input_container[container]
		   not container.resources.requests.cpu
		   not container.resources.limits.cpu
		   reason := sprintf("Resource %v container %v is missing CPU requirements", ["some_id", container.name])
		}

		input_container[c] {
			c := input.request.object.spec.containers[_]
		}

		input_container[c] {
			c := input.request.object.spec.initContainers[_]
		}

		input_container[c] {
			c := input.request.object.spec.specialContainers[_]
		}
	`)

	tests := map[string]struct {
		input *ast.Module
		want  string
	}{
		"non_linear_iteration_w_input":         {input: module1, want: "O(input.foo) * O(input.bar)"},
		"non_linear_iteration_w_rule":          {input: module2, want: "O(input.foo) * [ O(block_master_toleration) * O(input.bar) ]"},
		"non_linear_iteration_w_multiple_rule": {input: module3, want: "O(input.foo) * [ [O(input.request.object.spec.containers) + O(input.request.object.spec.initContainers) + O(input.request.object.spec.specialContainers)] * O(input.bar) ]"},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			actual := CalculateRuntimeComplexity(tc.input)
			if tc.want != actual.Result[name][0] {
				t.Fatalf("Expected runtime complexity %v but got %v", tc.want, actual.Result[name][0])
			}
		})
	}
}

func TestRuntimeComplexityReference(t *testing.T) {

	module1 := ast.MustParseModule(`
		package example

		ref_constant {
			v := 1
			data.foo.bar[v]
		}
	`)

	module2 := ast.MustParseModule(`
			package example

			ref_linear {
				data.foo.bar[v]
			}
		`)

	tests := map[string]struct {
		input *ast.Module
		want  string
	}{
		"ref_constant": {input: module1, want: "O(1)"},
		"ref_linear":   {input: module2, want: "O(data.foo.bar)"},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			actual := CalculateRuntimeComplexity(tc.input)
			if tc.want != actual.Result[name][0] {
				t.Fatalf("Expected runtime complexity %v but got %v", tc.want, actual.Result[name][0])
			}
		})
	}
}

func TestRuntimeComplexityBuiltin(t *testing.T) {

	module := ast.MustParseModule(`
			package example

			builtin_upper = x {
				x := upper("hello")
			}
		`)

	tests := map[string]struct {
		input *ast.Module
		want  string
	}{
		"builtin_upper": {input: module, want: "O(1)"},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			actual := CalculateRuntimeComplexity(tc.input)
			if tc.want != actual.Result[name][0] {
				t.Fatalf("Expected runtime complexity %v but got %v", tc.want, actual.Result[name][0])
			}
		})
	}
}

func TestRuntimeComplexityMissing(t *testing.T) {

	moduleCompre := ast.MustParseModule(`
		package example

		comprehension =  original_set {
			original := ["test", "test", "big", "opa", "rego", "rego"]
			original_set := {x | x := original[_]}
		}
	`)

	moduleFuncCall := ast.MustParseModule(`
		package example

		block_master_toleration[reason] {
			tolerations := get_tolerations(input.request)
			toleration := tolerations[_]

			toleration.operator == "Exists"
			not toleration.key
			reason := sprintf("Resource %v tolerates everything", [toleration.name])
		}

		get_tolerations(request) = result {
			request.kind.kind == "Pod"
			result := request.object.spec.tolerations
		}
	`)

	moduleWalk := ast.MustParseModule(`
		package example

		example_data = {
			"apiVersion": "v1"
		}

		walk_example {
			some path, value
			walk(example_data, [path, value])
			path[count(path)-1] == "apiVersion"
		}
	`)

	tests := map[string]struct {
		input *ast.Module
		want  string
	}{
		"missing_w_compre":    {input: moduleCompre, want: "comprehension"},
		"missing_w_func_call": {input: moduleFuncCall, want: "block_master_toleration"},
		"missing_w_walk":      {input: moduleWalk, want: "walk_example"},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			actual := CalculateRuntimeComplexity(tc.input)

			if len(actual.Missing) == 0 {
				t.Fatal("Expected missing complexity results but got none")
			}

			if _, ok := actual.Missing[tc.want]; !ok {
				t.Fatalf("Expected missing complexity result for rule %v", tc.want)
			}
		})
	}
}
