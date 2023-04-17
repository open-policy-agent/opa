// Copyright 2022 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"testing"

	"github.com/open-policy-agent/opa/ast"
)

func TestObjectUnionNBuiltin(t *testing.T) {
	tests := []struct {
		note     string
		query    string
		input    string
		expected string
	}{
		// NOTE(philipc): These tests assume that erroneous types are
		// checked elsewhere, and focus only on functional correctness.
		{
			note:     "Empty",
			input:    `[]`,
			expected: `{}`,
		},
		{
			note:     "Singletons",
			input:    `[{1: true}, {2: true}, {3: true}]`,
			expected: `{1: true, 2: true, 3: true}`,
		},
		{
			note:     "One object",
			input:    `[{1: true, 2: true, 3: true}]`,
			expected: `{1: true, 2: true, 3: true}`,
		},
		{
			note:     "One object + empty",
			input:    `[{1: true, 2: true, 3: true}, {}]`,
			expected: `{1: true, 2: true, 3: true}`,
		},
		{
			note:     "Multiple objects, with scalar duplicates",
			input:    `[{"A": 1, "B": 2, "C": 3}, {"A": 1, "B": 2}, {"C": 3}, {"D": 4, "E": 5}]`,
			expected: `{"A": 1, "B": 2, "C": 3, "D": 4, "E": 5}`,
		},
		{
			note:     "2x objects, with simple merge on key",
			input:    `[{"A": 1, "B": {"D": 4}, "C": 3},  {"B": 200}]`,
			expected: `{"A": 1, "B": 200, "C": 3,}`,
		},
		{
			note:     "2x objects, with complex merge on nested object",
			input:    `[{"A": 1, "B": {"N1": {"X": true, "Z": false}}, "C": 3},  {"B": {"N1": {"X": 49, "Z": 50}}}]`,
			expected: `{"A": 1, "B": {"N1": {"X": 49, "Z": 50}}, "C": 3}`,
		},
		{
			note:     "Multiple objects, with scalar, then object, overwrite on nested key",
			input:    `[{"A": 1, "B": {"N1": {"X": true, "Z": false}}, "C": 3}, {"B": {"N1": 23}}, {"B": {"N1": {"Z": 50}}}]`,
			expected: `{"A": 1, "B": {"N1": {"Z": 50}}, "C": 3}`,
		},
		{
			note:     "Multiple objects, with complex overwrite on nested key",
			input:    `[{"A": 1, "B": {"N1": {"X": true, "Z": false}}, "C": 3}, {"B": {"N1": 23}}, {"B": {"N1": {"Z": 50}}}, {"B": {"N1": {"Z": 35}}}]`,
			expected: `{"A": 1, "B": {"N1": {"Z": 35}}, "C": 3}`,
		},
	}

	for _, tc := range tests {
		inputs := ast.MustParseTerm(tc.input)
		result, err := getResult(builtinObjectUnionN, inputs)
		if err != nil {
			t.Fatal(err)
		}

		expected := ast.MustParseTerm(tc.expected)
		if !result.Equal(expected) {
			t.Fatalf("Expected %v but got %v", expected, result)
		}
	}
}
