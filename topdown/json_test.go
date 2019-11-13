// Copyright 2019 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"fmt"
	"testing"

	"github.com/open-policy-agent/opa/ast"
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
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			var paths [][]ast.Value
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
