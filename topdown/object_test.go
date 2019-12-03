// Copyright 2019 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"fmt"
	"testing"
)

func TestBuiltinObjectUnion(t *testing.T) {
	cases := []struct {
		note     string
		objectA  string
		objectB  string
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
			expected: `{"a": 1}`,
		},
		{
			note:     "conflict nested and extra field",
			objectA:  `{"a": 1}`,
			objectB:  `{"a": {"b": {"c": 1}}, "d": 7}`,
			expected: `{"a": 1, "d": 7}`,
		},
		{
			note:     "conflict multiple",
			objectA:  `{"a": {"b": {"c": 1}}, "e": 1}`,
			objectB:  `{"a": {"b": "foo", "b1": "bar"}, "d": 7, "e": 17}`,
			expected: `{"a": {"b": {"c": 1}, "b1": "bar"}, "d": 7, "e": 1}`,
		},
	}

	for _, tc := range cases {
		rules := []string{
			fmt.Sprintf("p = x { x := object.union(%s, %s) }", tc.objectA, tc.objectB),
		}
		runTopDownTestCase(t, map[string]interface{}{}, tc.note, rules, tc.expected)
	}
}

func TestBuiltinObjectRemove(t *testing.T) {
	cases := []struct {
		note     string
		object   string
		keys     string
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
			note:     "empty object",
			object:   `{}`,
			keys:     `{"a", "b"}`,
			expected: `{}`,
		},
		{
			note:     "empty keys",
			object:   `{"a": 1, "b": {"c": 3}}`,
			keys:     `set()`,
			expected: `{"a": 1, "b": {"c": 3}}`,
		},
		{
			note:     "key doesnt exist",
			object:   `{"a": 1, "b": {"c": 3}}`,
			keys:     `{"z"}`,
			expected: `{"a": 1, "b": {"c": 3}}`,
		},
	}

	for _, tc := range cases {
		rules := []string{
			fmt.Sprintf("p = x { x := object.remove(%s, %s) }", tc.object, tc.keys),
		}
		runTopDownTestCase(t, map[string]interface{}{}, tc.note, rules, tc.expected)
	}
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
		expected interface{}
	}{
		{
			note:     "base",
			object:   `{"a": {"b": {"c": 7, "d": 8}}, "e": 9}`,
			filters:  `{"a"}`,
			expected: `{"a": {"b": {"c": 7, "d": 8}}}`,
		},
		{
			note:     "multiple roots",
			object:   `{"a": 1, "b": 2, "c": 3, "e": 9}`,
			filters:  `{"a", "e"}`,
			expected: `{"a": 1, "e": 9}`,
		},
		{
			note:     "multiple roots",
			object:   `{"a": 1, "b": 2, "c": 3, "e": 9}`,
			filters:  `["a", "e"]`,
			expected: `{"a": 1, "e": 9}`,
		},
		{
			note:     "duplicate roots",
			object:   `{"a": {"b": {"c": 7, "d": 8}}, "e": 9}`,
			filters:  `{"a", "a"}`,
			expected: `{"a": {"b": {"c": 7, "d": 8}}}`,
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
			filters:  `{"a"}`,
			expected: `{}`,
		},
	}

	for _, tc := range cases {
		rules := []string{
			fmt.Sprintf("p = x { x := object.filter(%s, %s) }", tc.object, tc.filters),
		}
		runTopDownTestCase(t, map[string]interface{}{}, tc.note, rules, tc.expected)
	}
}

func TestBuiltinObjectFilterNonStringKey(t *testing.T) {
	rules := []string{
		`p { x := object.filter({"a": 1, [[7]]: 2}, {[[7]]}); x == {[[7]]: 2} }`,
	}
	runTopDownTestCase(t, map[string]interface{}{}, "non string root", rules, "true")
}
