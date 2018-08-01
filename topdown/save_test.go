// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"testing"

	"github.com/open-policy-agent/opa/ast"
)

func TestSaveSet(t *testing.T) {

	tests := []struct {
		terms    []string
		input    string
		expected bool
	}{
		{
			terms:    []string{},
			input:    `input`,
			expected: false,
		},
		{
			terms:    []string{`input`},
			input:    `data.x`,
			expected: false,
		},
		{
			terms:    []string{`input`},
			input:    `input.x`,
			expected: true,
		},
		{
			terms:    []string{`input.x`, `input.y`},
			input:    `input`,
			expected: true,
		},
		{
			terms:    []string{`input.x`, `input.y`},
			input:    `input.z`,
			expected: false,
		},
		{
			terms:    []string{`input.x`, `input.y`},
			input:    `input.x.foo`,
			expected: true,
		},
	}

	for _, tc := range tests {
		terms := make([]*ast.Term, len(tc.terms))
		for i := range tc.terms {
			terms[i] = ast.MustParseTerm(tc.terms[i])
		}
		saveSet := newSaveSet(terms, nil)
		input := ast.MustParseTerm(tc.input)
		if saveSet.Contains(input, nil) != tc.expected {
			t.Errorf("Expected %v for %v contains %v", tc.expected, tc.terms, input)
		}
	}

}
