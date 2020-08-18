// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"fmt"
	"testing"

	"github.com/open-policy-agent/opa/ast"
)

func TestObjectGet(t *testing.T) {
	cases := []struct {
		note     string
		object   string
		key      interface{}
		fallback interface{}
		expected interface{}
	}{
		{
			note:     "basic case . found",
			object:   `{"a": "b"}`,
			key:      `"a"`,
			fallback: `"c"`,
			expected: `"b"`,
		},
		{
			note:     "basic case . not found",
			object:   `{"a": "b"}`,
			key:      `"c"`,
			fallback: `"c"`,
			expected: `"c"`,
		},
		{

			note:     "integer key . found",
			object:   "{1: 2}",
			key:      "1",
			fallback: "3",
			expected: "2",
		},
		{
			note:     "integer key . not found",
			object:   "{1: 2}",
			key:      "2",
			fallback: "3",
			expected: "3",
		},
		{
			note:     "complex value . found",
			object:   `{"a": {"b": "c"}}`,
			key:      `"a"`,
			fallback: "true",
			expected: `{"b": "c"}`,
		},
		{
			note:     "complex value . not found",
			object:   `{"a": {"b": "c"}}`,
			key:      `"b"`,
			fallback: "true",
			expected: "true",
		},
	}

	for _, tc := range cases {
		rules := []string{
			fmt.Sprintf("p = x { x := object.get(%s, %s, %s) }", tc.object, tc.key, tc.fallback),
		}
		runTopDownTestCase(t, map[string]interface{}{}, tc.note, rules, tc.expected)
	}
}

func TestBuiltinObjectUnion(t *testing.T) {
	cases := []struct {
		note     string
		objectA  string
		objectB  string
		input    string
		expected interface{}
	}{
		{
			note:     "both empty",
			objectA:  `{}`,
			objectB:  `{}`,
			expected: `{}`,
		},
		{
			note:     "left empty",
			objectA:  `{}`,
			objectB:  `{"a": 1}`,
			expected: `{"a": 1}`,
		},
		{
			note:     "right empty",
			objectA:  `{"a": 1}`,
			objectB:  `{}`,
			expected: `{"a": 1}`,
		},
		{
			note:     "base",
			objectA:  `{"a": 1}`,
			objectB:  `{"b": 2}`,
			expected: `{"a": 1, "b": 2}`,
		},
		{
			note:     "nested",
			objectA:  `{"a": {"b": {"c": 1}}}`,
			objectB:  `{"b": 2}`,
			expected: `{"a": {"b": {"c": 1}}, "b": 2}`,
		},
		{
			note:     "nested reverse",
			objectA:  `{"b": 2}`,
			objectB:  `{"a": {"b": {"c": 1}}}`,
			expected: `{"a": {"b": {"c": 1}}, "b": 2}`,
		},
		{
			note:     "conflict simple",
			objectA:  `{"a": 1}`,
			objectB:  `{"a": 2}`,
			expected: `{"a": 2}`,
		},
		{
			note:     "conflict nested and extra field",
			objectA:  `{"a": 1}`,
			objectB:  `{"a": {"b": {"c": 1}}, "d": 7}`,
			expected: `{"a": {"b": {"c": 1}}, "d": 7}`,
		},
		{
			note:     "conflict multiple",
			objectA:  `{"a": {"b": {"c": 1}}, "e": 1}`,
			objectB:  `{"a": {"b": "foo", "b1": "bar"}, "d": 7, "e": 17}`,
			expected: `{"a": {"b": "foo", "b1": "bar"}, "d": 7, "e": 17}`,
		},
		{
			note:     "error wrong lhs type",
			objectA:  `[1, 2, 3]`,
			objectB:  `{"b": 2}`,
			expected: ast.Errors{ast.NewError(ast.TypeErr, nil, "object.union: invalid argument(s)")},
		},
		{
			note:     "error wrong lhs type input",
			objectA:  `input.a`,
			objectB:  `{"b": 2}`,
			input:    `{"a": [1, 2, 3]}`,
			expected: &Error{Code: TypeErr, Message: "object.union: operand 1 must be object but got array"},
		},
		{
			note:     "error wrong rhs type",
			objectA:  `{"a": 1}`,
			objectB:  `[1, 2, 3]`,
			expected: ast.Errors{ast.NewError(ast.TypeErr, nil, "object.union: invalid argument(s)")},
		},
		{
			note:     "error wrong rhs type input",
			objectA:  `{"a": 1}`,
			objectB:  `input.b`,
			input:    `{"b": [1, 2, 3]}`,
			expected: &Error{Code: TypeErr, Message: "object.union: operand 2 must be object but got array"},
		},
		{
			note:     "error wrong both params",
			objectA:  `"foo"`,
			objectB:  `[1, 2, 3]`,
			expected: ast.Errors{ast.NewError(ast.TypeErr, nil, "object.union: invalid argument(s)")},
		},
	}

	for _, tc := range cases {
		rules := []string{
			fmt.Sprintf("p = x { x := object.union(%s, %s) }", tc.objectA, tc.objectB),
		}
		runTopDownTestCaseWithModules(t, map[string]interface{}{}, tc.note, rules, nil, tc.input, tc.expected)
	}
}

func TestBuiltinObjectRemove(t *testing.T) {
	cases := []struct {
		note     string
		object   string
		keys     string
		input    string
		expected interface{}
	}{
		{
			note:     "base",
			object:   `{"a": 1, "b": {"c": 3}}`,
			keys:     `{"a"}`,
			expected: `{"b": {"c": 3}}`,
		},
		{
			note:     "multiple keys set",
			object:   `{"a": 1, "b": {"c": 3}, "d": 4}`,
			keys:     `{"d", "b"}`,
			expected: `{"a": 1}`,
		},
		{
			note:     "multiple keys array",
			object:   `{"a": 1, "b": {"c": 3}, "d": 4}`,
			keys:     `["d", "b"]`,
			expected: `{"a": 1}`,
		},
		{
			note:     "multiple keys object",
			object:   `{"a": 1, "b": {"c": 3}, "d": 4}`,
			keys:     `{"d": "", "b": 1}`,
			expected: `{"a": 1}`,
		},
		{
			note:     "multiple keys object nested",
			object:   `{"a": {"b": {"c": 2}}, "x": 123}`,
			keys:     `{"a": {"b": {"foo": "bar"}}}`,
			expected: `{"x": 123}`,
		},
		{
			note:     "empty object",
			object:   `{}`,
			keys:     `{"a", "b"}`,
			expected: `{}`,
		},
		{
			note:     "empty keys set",
			object:   `{"a": 1, "b": {"c": 3}}`,
			keys:     `set()`,
			expected: `{"a": 1, "b": {"c": 3}}`,
		},
		{
			note:     "empty keys array",
			object:   `{"a": 1, "b": {"c": 3}}`,
			keys:     `[]`,
			expected: `{"a": 1, "b": {"c": 3}}`,
		},
		{
			note:     "empty keys obj",
			object:   `{"a": 1, "b": {"c": 3}}`,
			keys:     `{}`,
			expected: `{"a": 1, "b": {"c": 3}}`,
		},
		{
			note:     "key doesnt exist",
			object:   `{"a": 1, "b": {"c": 3}}`,
			keys:     `{"z"}`,
			expected: `{"a": 1, "b": {"c": 3}}`,
		},
		{
			note:     "error invalid object param type set",
			object:   `{"a"}`,
			keys:     `{"a"}`,
			expected: ast.Errors{ast.NewError(ast.TypeErr, nil, "object.remove: invalid argument(s)")},
		},
		{
			note:     "error invalid object param type bool",
			object:   `false`,
			keys:     `{"a"}`,
			expected: ast.Errors{ast.NewError(ast.TypeErr, nil, "object.remove: invalid argument(s)")},
		},
		{
			note:     "error invalid object param type array input",
			object:   `input.x`,
			keys:     `{"a"}`,
			input:    `{"x": ["a"]}`,
			expected: &Error{Code: TypeErr, Message: "object.remove: operand 1 must be object but got array"},
		},
		{
			note:     "error invalid object param type bool input",
			object:   `input.x`,
			keys:     `{"a"}`,
			input:    `{"x": false}`,
			expected: &Error{Code: TypeErr, Message: "object.remove: operand 1 must be object but got boolean"},
		},
		{
			note:     "error invalid object param type number input",
			object:   `input.x`,
			keys:     `{"a"}`,
			input:    `{"x": 123}`,
			expected: &Error{Code: TypeErr, Message: "object.remove: operand 1 must be object but got number"},
		},
		{
			note:     "error invalid object param type string input",
			object:   `input.x`,
			keys:     `{"a"}`,
			input:    `{"x": "foo"}`,
			expected: &Error{Code: TypeErr, Message: "object.remove: operand 1 must be object but got string"},
		},
		{
			note:     "error invalid object param type nil input",
			object:   `input.x`,
			keys:     `{"a"}`,
			input:    `{"x": null}`,
			expected: &Error{Code: TypeErr, Message: "object.remove: operand 1 must be object but got null"},
		},
		{
			note:     "error invalid key param type string",
			object:   `{"a": 1}`,
			keys:     `"a"`,
			expected: ast.Errors{ast.NewError(ast.TypeErr, nil, "object.remove: invalid argument(s)")},
		},
		{
			note:     "error invalid key param type boolean",
			object:   `{"a": 1}`,
			keys:     `false`,
			expected: ast.Errors{ast.NewError(ast.TypeErr, nil, "object.remove: invalid argument(s)")},
		},
		{
			note:     "error invalid key param type string input",
			object:   `{"a": 1}`,
			keys:     `input.x`,
			input:    `{"x": "foo"}`,
			expected: &Error{Code: TypeErr, Message: "object.remove: operand 2 must be one of {object, string, array} but got string"},
		},
		{
			note:     "error invalid key param type boolean input",
			object:   `{"a": 1}`,
			keys:     `input.x`,
			input:    `{"x": true}`,
			expected: &Error{Code: TypeErr, Message: "object.remove: operand 2 must be one of {object, string, array} but got boolean"},
		},
		{
			note:     "error invalid key param type number input",
			object:   `{"a": 1}`,
			keys:     `input.x`,
			input:    `{"x": 22}`,
			expected: &Error{Code: TypeErr, Message: "object.remove: operand 2 must be one of {object, string, array} but got number"},
		},
		{
			note:     "error invalid key param type nil input",
			object:   `{"a": 1}`,
			keys:     `input.x`,
			input:    `{"x": null}`,
			expected: &Error{Code: TypeErr, Message: "object.remove: operand 2 must be one of {object, string, array} but got null"},
		},
	}

	for _, tc := range cases {
		rules := []string{
			fmt.Sprintf("p = x { x := object.remove(%s, %s) }", tc.object, tc.keys),
		}
		runTopDownTestCaseWithModules(t, map[string]interface{}{}, tc.note, rules, nil, tc.input, tc.expected)
	}
}

func TestBuiltinObjectRemoveIdempotent(t *testing.T) {
	rule := `
	p {
		# "base" should never be mutated
		base := {"a": 1, "b": 2, "c": 3}
	    object.remove(base, {"a"}) == {"b": 2, "c": 3}
		object.remove(base, {"b"}) == {"a": 1, "c": 3}
		object.remove(base, {"c"}) == {"a": 1, "b": 2}
		base == {"a": 1, "b": 2, "c": 3}
	}
	`
	runTopDownTestCase(t, map[string]interface{}{}, t.Name(), []string{rule}, "true")
}

func TestBuiltinObjectRemoveNonStringKey(t *testing.T) {
	rules := []string{
		`p { x := object.remove({"a": 1, [[7]]: 2}, {[[7]]}); x == {"a": 1} }`,
	}
	runTopDownTestCase(t, map[string]interface{}{}, "non string root", rules, "true")
}

func TestBuiltinObjectFilter(t *testing.T) {
	cases := []struct {
		note     string
		object   string
		filters  string
		input    string
		expected interface{}
	}{
		{
			note:     "base",
			object:   `{"a": {"b": {"c": 7, "d": 8}}, "e": 9}`,
			filters:  `{"a"}`,
			expected: `{"a": {"b": {"c": 7, "d": 8}}}`,
		},
		{
			note:     "multiple roots set",
			object:   `{"a": 1, "b": 2, "c": 3, "e": 9}`,
			filters:  `{"a", "e"}`,
			expected: `{"a": 1, "e": 9}`,
		},
		{
			note:     "multiple roots array",
			object:   `{"a": 1, "b": 2, "c": 3, "e": 9}`,
			filters:  `["a", "e"]`,
			expected: `{"a": 1, "e": 9}`,
		},
		{
			note:     "multiple roots object",
			object:   `{"a": 1, "b": 2, "c": 3, "e": 9}`,
			filters:  `{"a": "foo", "e": ""}`,
			expected: `{"a": 1, "e": 9}`,
		},
		{
			note:     "duplicate roots",
			object:   `{"a": {"b": {"c": 7, "d": 8}}, "e": 9}`,
			filters:  `{"a", "a"}`,
			expected: `{"a": {"b": {"c": 7, "d": 8}}}`,
		},
		{
			note:     "empty roots set",
			object:   `{"a": 7}`,
			filters:  `set()`,
			expected: `{}`,
		},
		{
			note:     "empty roots array",
			object:   `{"a": 7}`,
			filters:  `[]`,
			expected: `{}`,
		},
		{
			note:     "empty roots object",
			object:   `{"a": 7}`,
			filters:  `{}`,
			expected: `{}`,
		},
		{
			note:     "empty object",
			object:   `{}`,
			filters:  `{"a"}`,
			expected: `{}`,
		},
		{
			note:     "error invalid object param type set",
			object:   `{"a"}`,
			filters:  `{"a"}`,
			expected: ast.Errors{ast.NewError(ast.TypeErr, nil, "object.filter: invalid argument(s)")},
		},
		{
			note:     "error invalid object param type bool",
			object:   `false`,
			filters:  `{"a"}`,
			expected: ast.Errors{ast.NewError(ast.TypeErr, nil, "object.filter: invalid argument(s)")},
		},
		{
			note:     "error invalid object param type array input",
			object:   `input.x`,
			filters:  `{"a"}`,
			input:    `{"x": ["a"]}`,
			expected: &Error{Code: TypeErr, Message: "object.filter: operand 1 must be object but got array"},
		},
		{
			note:     "error invalid object param type bool input",
			object:   `input.x`,
			filters:  `{"a"}`,
			input:    `{"x": false}`,
			expected: &Error{Code: TypeErr, Message: "object.filter: operand 1 must be object but got boolean"},
		},
		{
			note:     "error invalid object param type number input",
			object:   `input.x`,
			filters:  `{"a"}`,
			input:    `{"x": 123}`,
			expected: &Error{Code: TypeErr, Message: "object.filter: operand 1 must be object but got number"},
		},
		{
			note:     "error invalid object param type string input",
			object:   `input.x`,
			filters:  `{"a"}`,
			input:    `{"x": "foo"}`,
			expected: &Error{Code: TypeErr, Message: "object.filter: operand 1 must be object but got string"},
		},
		{
			note:     "error invalid object param type nil input",
			object:   `input.x`,
			filters:  `{"a"}`,
			input:    `{"x": null}`,
			expected: &Error{Code: TypeErr, Message: "object.filter: operand 1 must be object but got null"},
		},
		{
			note:     "error invalid key param type string",
			object:   `{"a": 1}`,
			filters:  `"a"`,
			expected: ast.Errors{ast.NewError(ast.TypeErr, nil, "object.filter: invalid argument(s)")},
		},
		{
			note:     "error invalid key param type boolean",
			object:   `{"a": 1}`,
			filters:  `false`,
			expected: ast.Errors{ast.NewError(ast.TypeErr, nil, "object.filter: invalid argument(s)")},
		},
		{
			note:     "error invalid key param type string input",
			object:   `{"a": 1}`,
			filters:  `input.x`,
			input:    `{"x": "foo"}`,
			expected: &Error{Code: TypeErr, Message: "object.filter: operand 2 must be one of {object, string, array} but got string"},
		},
		{
			note:     "error invalid key param type boolean input",
			object:   `{"a": 1}`,
			filters:  `input.x`,
			input:    `{"x": true}`,
			expected: &Error{Code: TypeErr, Message: "object.filter: operand 2 must be one of {object, string, array} but got boolean"},
		},
		{
			note:     "error invalid key param type number input",
			object:   `{"a": 1}`,
			filters:  `input.x`,
			input:    `{"x": 22}`,
			expected: &Error{Code: TypeErr, Message: "object.filter: operand 2 must be one of {object, string, array} but got number"},
		},
		{
			note:     "error invalid key param type nil input",
			object:   `{"a": 1}`,
			filters:  `input.x`,
			input:    `{"x": null}`,
			expected: &Error{Code: TypeErr, Message: "object.filter: operand 2 must be one of {object, string, array} but got null"},
		},
	}

	for _, tc := range cases {
		rules := []string{
			fmt.Sprintf("p = x { x := object.filter(%s, %s) }", tc.object, tc.filters),
		}
		runTopDownTestCaseWithModules(t, map[string]interface{}{}, tc.note, rules, nil, tc.input, tc.expected)
	}
}

func TestBuiltinObjectFilterNonStringKey(t *testing.T) {
	rules := []string{
		`p { x := object.filter({"a": 1, [[7]]: 2}, {[[7]]}); x == {[[7]]: 2} }`,
	}
	runTopDownTestCase(t, map[string]interface{}{}, "non string root", rules, "true")
}

func TestBuiltinObjectFilterIdempotent(t *testing.T) {
	rule := `
	p {
		# "base" should never be mutated
		base := {"a": 1, "b": 2, "c": 3}
	    object.filter(base, {"a"}) == {"a": 1}
		object.filter(base, {"b"}) == {"b": 2}
		object.filter(base, {"c"}) == {"c": 3}
		base == {"a": 1, "b": 2, "c": 3}
	}
	`
	runTopDownTestCase(t, map[string]interface{}{}, t.Name(), []string{rule}, "true")
}
