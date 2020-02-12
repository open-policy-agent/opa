// Copyright 2019 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"fmt"
	"testing"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/topdown/builtins"
)

func TestBuiltinJSONFilter(t *testing.T) {
	cases := []struct {
		note     string
		object   string
		filters  string
		expected interface{}
	}{
		{
			note:     "base",
			object:   `{"a": {"b": {"c": 7, "d": 8}}, "e": 9}`,
			filters:  `{"a/b/c"}`,
			expected: `{"a": {"b": {"c": 7}}}`,
		},
		{
			note:     "multiple roots",
			object:   `{"a": {"b": {"c": 7, "d": 8}}, "e": 9}`,
			filters:  `{"a/b/c", "e"}`,
			expected: `{"a": {"b": {"c": 7}}, "e": 9}`,
		},
		{
			note:     "multiple roots array",
			object:   `{"a": {"b": {"c": 7, "d": 8}}, "e": 9}`,
			filters:  `["a/b/c", "e"]`,
			expected: `{"a": {"b": {"c": 7}}, "e": 9}`,
		},
		{
			note:     "shared roots",
			object:   `{"a": {"b": {"c": 7, "d": 8}, "e": 9}}`,
			filters:  `{"a/b/c", "a/e"}`,
			expected: `{"a": {"b": {"c": 7}, "e": 9}}`,
		},
		{
			note:     "conflict",
			object:   `{"a": {"b": 7}}`,
			filters:  `{"a", "a/b"}`,
			expected: `{"a": {"b": 7}}`,
		},
		{
			note:     "empty list",
			object:   `{"a": 7}`,
			filters:  `set()`,
			expected: `{}`,
		},
		{
			note:     "empty object",
			object:   `{}`,
			filters:  `{"a/b"}`,
			expected: `{}`,
		},
		{
			note:     "arrays",
			object:   `{"a": [{"b": 7, "c": 8}, {"d": 9}]}`,
			filters:  `{"a/0/b", "a/1"}`,
			expected: `{"a": [{"b": 7}, {"d": 9}]}`,
		},
		{
			note:     "object with number keys",
			object:   `{"a": [{"1":["b", "c", "d"]}, {"x": "y"}]}`,
			filters:  `{"a/0/1/2"}`,
			expected: `{"a": [{"1": ["d"]}]}`,
		},
		{
			note:     "arrays of roots",
			object:   `{"a": {"b": {"c": 7, "d": 8}}, "e": 9}`,
			filters:  `{["a", "b", "c"], ["e"]}`,
			expected: `{"a": {"b": {"c": 7}}, "e": 9}`,
		},
		{
			note:     "mixed root types",
			object:   `{"a": {"b": {"c": 7, "d": 8, "x": 0}}, "e": 9}`,
			filters:  `{["a", "b", "c"], "a/b/d"}`,
			expected: `{"a": {"b": {"c": 7, "d": 8}}}`,
		},
	}

	for _, tc := range cases {
		rules := []string{
			fmt.Sprintf("p = x { x := json.filter(%s, %s) }", tc.object, tc.filters),
		}
		runTopDownTestCase(t, map[string]interface{}{}, tc.note, rules, tc.expected)
	}
}

func TestBuiltinJSONFilterIdempotent(t *testing.T) {
	rule := `
	p {
		# "base" should never be mutated
		base := {"a": {"b": 2, "c": 3}}
		json.filter(base, {"a/b"}) == {"a": {"b": 2}}
		json.filter(base, {"a/c"}) == {"a": {"c": 3}}
		base == {"a": {"b": 2, "c": 3}}
	}
	`
	runTopDownTestCase(t, map[string]interface{}{}, t.Name(), []string{rule}, "true")
}

func TestFiltersToObject(t *testing.T) {
	cases := []struct {
		note     string
		filters  []string
		expected string
	}{
		{
			note:     "base",
			filters:  []string{`"a/b/c"`},
			expected: `{"a": {"b": {"c": null}}}`,
		},
		{
			note:     "root prefixed",
			filters:  []string{`"a/b/c"`},
			expected: `{"a": {"b": {"c": null}}}`,
		},
		{
			note:     "trailing slash",
			filters:  []string{`"a/b/c"`},
			expected: `{"a": {"b": {"c": null}}}`,
		},
		{
			note:     "different roots",
			filters:  []string{`"a/b/c"`, `"d/e/f"`},
			expected: `{"a": {"b": {"c": null}}, "d": {"e": {"f": null}}}`,
		},
		{
			note:     "shared root",
			filters:  []string{`"a/b/c"`, `"a/b/d"`},
			expected: `{"a": {"b": {"c": null, "d": null}}}`,
		},
		{
			note:     "multiple shares at different points",
			filters:  []string{`"a/b/c"`, `"a/b/d"`, `"a/e/f"`},
			expected: `{"a": {"b": {"c": null, "d": null}, "e": {"f": null}}}`,
		},
		{
			note:     "conflict with one ordering",
			filters:  []string{`"a"`, `"a/b"`},
			expected: `{"a": null}`,
		},
		{
			note:     "conflict with reverse ordering",
			filters:  []string{`"a/b"`, `"a"`},
			expected: `{"a": null}`,
		},
		{
			note:     "arrays",
			filters:  []string{`"a/1/c"`, `"a/1/b"`},
			expected: `{"a": {"1": {"c": null, "b": null}}}`,
		},
		{
			note:     "non string keys",
			filters:  []string{`[[1], {2}]`, `"a/1/b"`},
			expected: `{"a": {"1": {"b": null}}, [1]: {{2}: null}}`,
		},
		{
			note:     "escaped tilde",
			filters:  []string{`"a/~0b~0/c~0"`},
			expected: `{"a": {"~b~": {"c~": null}}}`,
		},
		{
			note:     "escaped slash",
			filters:  []string{`"a/~1b~1c/d~1"`},
			expected: `{"a": {"/b/c": {"d/": null}}}`,
		},
		{
			note:     "mixed escapes",
			filters:  []string{`"a/~0b~1c/d~1~0"`},
			expected: `{"a": {"~b/c": {"d/~": null}}}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			var paths []ast.Ref
			for _, path := range tc.filters {
				parsedPath, err := parsePath(ast.MustParseTerm(path))
				if err != nil {
					t.Errorf("unexpected error parsing path %s: %s", path, err)
				}
				paths = append(paths, parsedPath)
			}
			actual := pathsToObject(paths)
			expected := ast.MustParseTerm(tc.expected)
			if actual.Compare(expected.Value) != 0 {
				t.Errorf("Unexpected object from filters:\n\nExpected:\n\t%s\n\nActual:\n\t%s\n\n", expected.Value.String(), actual.String())
			}
		})
	}
}

func TestBuiltinJSONRemove(t *testing.T) {
	cases := []struct {
		note     string
		object   string
		paths    string
		input    string
		expected interface{}
	}{
		{
			note:     "base",
			object:   `{"a": {"b": {"c": 7, "d": 8}}, "e": 9}`,
			paths:    `{"a/b/c"}`,
			expected: `{"a": {"b": {"d": 8}}, "e": 9}`,
		},
		{
			note:     "multiple roots",
			object:   `{"a": {"b": {"c": 7, "d": 8}}, "e": 9}`,
			paths:    `{"a/b/c", "e"}`,
			expected: `{"a": {"b": {"d": 8}}}`,
		},
		{
			note:     "multiple roots array",
			object:   `{"a": {"b": {"c": 7, "d": 8}}, "e": 9}`,
			paths:    `["a/b/c", "e"]`,
			expected: `{"a": {"b": {"d": 8}}}`,
		},
		{
			note:     "shared roots",
			object:   `{"a": {"b": {"c": 7, "d": 8}, "e": 9}}`,
			paths:    `{"a/b/c", "a/e"}`,
			expected: `{"a": {"b": {"d": 8}}}`,
		},
		{
			note:     "conflict",
			object:   `{"a": {"b": 7}, "c": 1}`,
			paths:    `{"a", "a/b"}`,
			expected: `{"c": 1}`,
		},
		{
			note:     "empty list",
			object:   `{"a": 7}`,
			paths:    `set()`,
			expected: `{"a": 7}`,
		},
		{
			note:     "empty object",
			object:   `{}`,
			paths:    `{"a/b"}`,
			expected: `{}`,
		},
		{
			note:     "delete all",
			object:   `{"a": {"b": 7}, "c": 1}`,
			paths:    `{"a", "c"}`,
			expected: `{}`,
		},
		{
			note:     "delete last in object",
			object:   `{"a": {"b": 7}, "c": 1}`,
			paths:    `{"a/b", "c"}`,
			expected: `{"a": {}}`,
		},
		{
			note:     "arrays",
			object:   `{"a": [{"b": 7, "c": 8}, {"d": 9}]}`,
			paths:    `{"a/0/b", "a/1"}`,
			expected: `{"a": [{"c": 8}]}`,
		},
		{
			note:     "object with number keys",
			object:   `{"a": [{"1":["b", "c", "d"]}, {"x": "y"}]}`,
			paths:    `{"a/0/1/2"}`,
			expected: `{"a": [{"1":["b", "c"]}, {"x": "y"}]}`,
		},
		{
			note:     "arrays of roots",
			object:   `{"a": {"b": {"c": 7, "d": 8}}, "e": 9}`,
			paths:    `{["a", "b", "c"], ["e"]}`,
			expected: `{"a": {"b": {"d": 8}}}`,
		},
		{
			note:     "mixed root types",
			object:   `{"a": {"b": {"c": 7, "d": 8, "x": 0}}, "e": 9}`,
			paths:    `{["a", "b", "c"], "a/b/d"}`,
			expected: `{"a": {"b": {"x": 0}}, "e": 9}`,
		},
		{
			note:     "error invalid target type string",
			object:   `"foo"`,
			paths:    `{"a/b/c"}`,
			expected: ast.Errors{ast.NewError(ast.TypeErr, nil, "json.remove: invalid argument(s)")},
		},
		{
			note:     "error invalid target type number",
			object:   `22`,
			paths:    `{"a/b/c"}`,
			expected: ast.Errors{ast.NewError(ast.TypeErr, nil, "json.remove: invalid argument(s)")},
		},
		{
			note:     "error invalid target type boolean",
			object:   `false`,
			paths:    `{"a/b/c"}`,
			expected: ast.Errors{ast.NewError(ast.TypeErr, nil, "json.remove: invalid argument(s)")},
		},
		{
			note:     "error invalid target type set",
			object:   `{"a"}`,
			paths:    `{"a/b/c"}`,
			expected: ast.Errors{ast.NewError(ast.TypeErr, nil, "json.remove: invalid argument(s)")},
		},
		{
			note:     "error invalid target type array",
			object:   `["a"]`,
			paths:    `{"a/b/c"}`,
			expected: ast.Errors{ast.NewError(ast.TypeErr, nil, "json.remove: invalid argument(s)")},
		},
		{
			note:     "error invalid target type string input",
			object:   `input.x`,
			paths:    `{"a/b/c"}`,
			input:    `{"x": "foo"}`,
			expected: builtins.NewOperandErr(1, "must be object but got string"),
		},
		{
			note:     "error invalid target type number input",
			object:   `input.x`,
			paths:    `{"a/b/c"}`,
			input:    `{"x": 22}`,
			expected: builtins.NewOperandErr(1, "must be object but got number"),
		},
		{
			note:     "error invalid target type boolean input",
			object:   `input.x`,
			paths:    `{"a/b/c"}`,
			input:    `{"x": true}`,
			expected: builtins.NewOperandErr(1, "must be object but got boolean"),
		},
		{
			note:     "error invalid target type array input",
			object:   `input.x`,
			paths:    `{"a/b/c"}`,
			input:    `{"x": ["a", "b", "c"]}`,
			expected: builtins.NewOperandErr(1, "must be object but got array"),
		},
		{
			note:     "error invalid paths type string",
			object:   `{"a": {"b": {"c": 123}}}`,
			paths:    `"foo"`,
			expected: ast.Errors{ast.NewError(ast.TypeErr, nil, "json.remove: invalid argument(s)")},
		},
		{
			note:     "error invalid paths type number",
			object:   `{"a": {"b": {"c": 123}}}`,
			paths:    `22`,
			expected: ast.Errors{ast.NewError(ast.TypeErr, nil, "json.remove: invalid argument(s)")},
		},
		{
			note:     "error invalid paths type boolean",
			object:   `{"a": {"b": {"c": 123}}}`,
			paths:    `true`,
			expected: ast.Errors{ast.NewError(ast.TypeErr, nil, "json.remove: invalid argument(s)")},
		},
		{
			note:     "error invalid paths type object",
			object:   `{"a": {"b": {"c": 123}}}`,
			paths:    `{"x": 1}`,
			expected: ast.Errors{ast.NewError(ast.TypeErr, nil, "json.remove: invalid argument(s)")},
		},
		{
			note:     "error invalid paths type set with numbers",
			object:   `{"a": {"b": {"c": 123}}}`,
			paths:    `{"a", 1, 2, 3}`,
			expected: ast.Errors{ast.NewError(ast.TypeErr, nil, "json.remove: invalid argument(s)")},
		},
		{
			note:     "error invalid paths type set with objects",
			object:   `{"a": {"b": {"c": 123}}}`,
			paths:    `{"a", {"x": 1}, {"y": 2}}`,
			expected: ast.Errors{ast.NewError(ast.TypeErr, nil, "json.remove: invalid argument(s)")},
		},
		{
			note:     "error invalid paths type array with numbers",
			object:   `{"a": {"b": {"c": 123}}}`,
			paths:    `["a", 1, 2, 3]`,
			expected: ast.Errors{ast.NewError(ast.TypeErr, nil, "json.remove: invalid argument(s)")},
		},
		{
			note:     "error invalid paths type array with objects",
			object:   `{"a": {"b": {"c": 123}}}`,
			paths:    `["a", {"x": 1}, {"y": 2}]`,
			expected: ast.Errors{ast.NewError(ast.TypeErr, nil, "json.remove: invalid argument(s)")},
		},
		{
			note:     "error invalid paths type string",
			object:   `{"a": {"b": {"c": 123}}}`,
			paths:    `input.x`,
			input:    `{"x": "foo"}`,
			expected: builtins.NewOperandErr(2, "must be one of {set, array} but got string"),
		},
		{
			note:     "error invalid paths type number",
			object:   `{"a": {"b": {"c": 123}}}`,
			paths:    `input.x`,
			input:    `{"x": 22}`,
			expected: builtins.NewOperandErr(2, "must be one of {set, array} but got number"),
		},
		{
			note:     "error invalid paths type boolean",
			object:   `{"a": {"b": {"c": 123}}}`,
			paths:    `input.x`,
			input:    `{"x": true}`,
			expected: builtins.NewOperandErr(2, "must be one of {set, array} but got boolean"),
		},
		{
			note:     "error invalid paths type object",
			object:   `{"a": {"b": {"c": 123}}}`,
			paths:    `input.x`,
			input:    `{"x": {"y": 123}}`,
			expected: builtins.NewOperandErr(2, "must be one of {set, array} but got object"),
		},
		{
			note:     "error invalid paths type set with numbers",
			object:   `{"a": {"b": {"c": 123}}}`,
			paths:    `input.x`,
			input:    `{"x": {"a", 1, 2, 3}}`,
			expected: builtins.NewOperandErr(2, "must be one of {set, array} containing string paths or array of path segments but got number"),
		},
		{
			note:     "error invalid paths type set with objects",
			object:   `{"a": {"b": {"c": 123}}}`,
			paths:    `input.x`,
			input:    `{"x": {"a", {"x": 1}, {"y": 2}}}`,
			expected: builtins.NewOperandErr(2, "must be one of {set, array} containing string paths or array of path segments but got object"),
		},
		{
			note:     "error invalid paths type array with numbers",
			object:   `{"a": {"b": {"c": 123}}}`,
			paths:    `input.x`,
			input:    `{"x": ["a", 1, 2, 3]}`,
			expected: builtins.NewOperandErr(2, "must be one of {set, array} containing string paths or array of path segments but got number"),
		},
		{
			note:     "error invalid paths type array with objects",
			object:   `{"a": {"b": {"c": 123}}}`,
			paths:    `input.x`,
			input:    `{"x": ["a", {"x": 1}, {"y": 2}]}`,
			expected: builtins.NewOperandErr(2, "must be one of {set, array} containing string paths or array of path segments but got object"),
		},
	}

	for _, tc := range cases {
		rules := []string{
			fmt.Sprintf("p = x { x := json.remove(%s, %s) }", tc.object, tc.paths),
		}
		runTopDownTestCaseWithModules(t, map[string]interface{}{}, tc.note, rules, nil, tc.input, tc.expected)
	}
}

func TestBuiltinJSONRemoveIdempotent(t *testing.T) {
	rule := `
	p {
		# "base" should never be mutated
		base := {"a": {"b": 2, "c": 3}}
		json.remove(base, {"a"}) == {}
		json.remove(base, {"a/b"}) == {"a": {"c": 3}}
		json.remove(base, {"a/c"}) == {"a": {"b": 2}}
		base == {"a": {"b": 2, "c": 3}}
	}
	`
	runTopDownTestCase(t, map[string]interface{}{}, t.Name(), []string{rule}, "true")
}
