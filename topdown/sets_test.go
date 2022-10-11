// Copyright 2022 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"testing"

	"github.com/open-policy-agent/opa/ast"
)

func TestSetUnionBuiltin(t *testing.T) {
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
			input:    `{set()}`,
			expected: `set()`,
		},
		{
			note:     "Singletons",
			input:    `{{1}, {2}, {3}, {4}, {5}}`,
			expected: `{1, 2, 3, 4, 5}`,
		},
		{
			note:     "One set",
			input:    `{{1, 2, 3, 4, 5}}`,
			expected: `{1, 2, 3, 4, 5}`,
		},
		{
			note:     "One set + empty",
			input:    `{{1, 2, 3, 4, 5}, set()}`,
			expected: `{1, 2, 3, 4, 5}`,
		},
		{
			note:     "Multiple sets, with duplicates",
			input:    `{{1, 2, 3}, {1, 2}, {3}, {4, 5}}`,
			expected: `{1, 2, 3, 4, 5}`,
		},
	}

	for _, tc := range tests {
		inputs := ast.MustParseTerm(tc.input)
		result, err := getResult(builtinSetUnion, inputs)
		if err != nil {
			t.Fatal(err)
		}

		expected := ast.MustParseTerm(tc.expected)
		if !result.Equal(expected) {
			t.Fatalf("Expected %v but got %v", expected, result)
		}
	}
}
