// Copyright 2024 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package inmem

import (
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/storage"
)

func TestSetInAst(t *testing.T) {
	tests := []struct {
		note     string
		value    string
		path     string
		newValue string
		expected string
	}{
		{
			note:     "zero length path",
			value:    `{}`,
			path:     "/",
			newValue: "42",
			expected: "{}",
		},
		{
			note:     "set object key",
			value:    `{"a": 1, "b": 2, "c": 3}`,
			path:     "/b",
			newValue: "42",
			expected: `{"a": 1, "b": 42, "c": 3}`,
		},
		{
			note:     "set nested object key",
			value:    `{"a": {"b": 1, "c": 2, "d": 3}, "b": 4}`,
			path:     "/a/c",
			newValue: "42",
			expected: `{"a": {"b": 1, "c": 42, "d": 3}, "b": 4}`,
		},
		// new keys can be added to objects
		{
			note:     "add object key",
			value:    `{"a": 1, "b": 2, "c": 3}`,
			path:     "/d",
			newValue: "42",
			expected: `{"a": 1, "b": 2, "c": 3, "d": 42}`,
		},
		{
			note:     "add nested object key",
			value:    `{"a": {"b": 1, "c": 2, "d": 3}, "b": 4}`,
			path:     "/a/e",
			newValue: "42",
			expected: `{"a": {"b": 1, "c": 2, "d": 3, "e": 42}, "b": 4}`,
		},

		{
			note:     "set array element",
			value:    `[1, 2, 3]`,
			path:     "/1",
			newValue: "42",
			expected: `[1, 42, 3]`,
		},
		{
			note:     "set nested array element",
			value:    `[[1, 2], [3, 4], [5, 6]]`,
			path:     "/1/0",
			newValue: "42",
			expected: `[[1, 2], [42, 4], [5, 6]]`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			value := ast.MustParseTerm(tc.value).Value
			path := storage.MustParsePath(tc.path)
			newValue := ast.MustParseTerm(tc.newValue).Value
			expected := ast.MustParseTerm(tc.expected).Value

			result, err := setInAst(value, path, newValue)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if expected.Compare(result) != 0 {
				t.Fatalf("Expected:\n\n%v\n\nbut got:\n\n%v", expected, result)
			}

			if result.Hash() != expected.Hash() {
				t.Fatalf("Expected hash:\n\n%v\n\nbut got:\n\n%v", expected.Hash(), result.Hash())
			}
		})
	}
}

func TestRemoveInAst(t *testing.T) {
	tests := []struct {
		note     string
		value    string
		path     string
		expected string
	}{
		{
			note:     "zero length path (no-op)",
			value:    `{"a": 1, "b": 2, "c": 3}`,
			path:     "/",
			expected: `{"a": 1, "b": 2, "c": 3}`,
		},
		{
			note:     "remove object key",
			value:    `{"a": 1, "b": 2, "c": 3}`,
			path:     "/b",
			expected: `{"a": 1, "c": 3}`,
		},
		{
			note:     "remove object key, no hit",
			value:    `{"a": 1, "b": 2, "c": 3}`,
			path:     "/d",
			expected: `{"a": 1, "b": 2, "c": 3}`,
		},
		{
			note:     "remove nested object key",
			value:    `{"a": {"b": 1, "c": 2, "d": 3}, "b": 4}`,
			path:     "/a/c",
			expected: `{"a": {"b": 1, "d": 3}, "b": 4}`,
		},
		{
			note:     "remove nested object key, no hit",
			value:    `{"a": {"b": 1, "c": 2, "d": 3}, "b": 4}`,
			path:     "/a/e",
			expected: `{"a": {"b": 1, "c": 2, "d": 3}, "b": 4}`,
		},

		{
			note:     "remove array element",
			value:    `[1, 2, 3]`,
			path:     "/1",
			expected: `[1, 3]`,
		},
		{
			note:     "remove array element, no hit (over)",
			value:    `[1, 2, 3]`,
			path:     "/4",
			expected: `[1, 2, 3]`,
		},
		{
			note:     "remove array element, no hit (under)",
			value:    `[1, 2, 3]`,
			path:     "/-1",
			expected: `[1, 2, 3]`,
		},
		{
			note:     "remove nested array element",
			value:    `[[1, 2], [3, 4], [5, 6]]`,
			path:     "/1/0",
			expected: `[[1, 2], [4], [5, 6]]`,
		},
		{
			note:     "remove nested array element, no hit",
			value:    `[[1, 2], [3, 4], [5, 6]]`,
			path:     "/1/2",
			expected: `[[1, 2], [3, 4], [5, 6]]`,
		},
		{
			note:     "remove array element nested inside object",
			value:    `{"a": [1, 2, 3], "b": [4, 5, 6]}`,
			path:     "/a/1",
			expected: `{"a": [1, 3], "b": [4, 5, 6]}`,
		},
		{
			note:     "remove object key nested inside array",
			value:    `[{"a": 1, "b": 2}, {"a": 3, "b": 4}]`,
			path:     "/1/a",
			expected: `[{"a": 1, "b": 2}, {"b": 4}]`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			value := ast.MustParseTerm(tc.value).Value
			path := storage.MustParsePath(tc.path)
			expected := ast.MustParseTerm(tc.expected).Value

			result, err := removeInAst(value, path)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if expected.Compare(result) != 0 {
				t.Fatalf("Expected:\n\n%v\n\nbut got:\n\n%v", expected, result)
			}

			if result.Hash() != expected.Hash() {
				t.Fatalf("Expected hash:\n\n%v\n\nbut got:\n\n%v", expected.Hash(), result.Hash())
			}
		})
	}
}
