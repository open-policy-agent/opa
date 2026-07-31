// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package debug

import (
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
)

func TestTruncatedString(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		max      int
		expected string
	}{
		{
			name:     "no truncation when max is 0",
			s:        strings.Repeat("a", 200),
			max:      0,
			expected: strings.Repeat("a", 200),
		},
		{
			name:     "no truncation when max is negative",
			s:        strings.Repeat("a", 200),
			max:      -1,
			expected: strings.Repeat("a", 200),
		},
		{
			name:     "no truncation when max is 1 (avoids negative slice)",
			s:        "abcdefghij",
			max:      1,
			expected: "abcdefghij",
		},
		{
			name:     "no truncation when max is 2",
			s:        "abcdefghij",
			max:      2,
			expected: "abcdefghij",
		},
		{
			name:     "no truncation when max is 3",
			s:        "abcdefghij",
			max:      3,
			expected: "abcdefghij",
		},
		{
			name:     "truncation at default limit",
			s:        strings.Repeat("a", 200),
			max:      100,
			expected: strings.Repeat("a", 98) + "...",
		},
		{
			name:     "no truncation when value is under limit",
			s:        strings.Repeat("a", 50),
			max:      100,
			expected: strings.Repeat("a", 50),
		},
		{
			name:     "no truncation when value equals limit",
			s:        strings.Repeat("a", 100),
			max:      100,
			expected: strings.Repeat("a", 100),
		},
		{
			name:     "truncation when value exceeds limit by one",
			s:        strings.Repeat("a", 101),
			max:      100,
			expected: strings.Repeat("a", 98) + "...",
		},
		{
			name:     "small truncation limit",
			s:        "abcdefghij",
			max:      5,
			expected: "abc...",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := truncatedString(tc.s, tc.max)
			if got != tc.expected {
				t.Errorf("truncatedString(%q, %d) = %q, want %q", tc.s, tc.max, got, tc.expected)
			}
		})
	}
}

func TestVariableValueLengthLimit(t *testing.T) {
	// A string value longer than the default 100-character limit.
	// ast.String.String() renders strings with surrounding quotes, so the
	// expected values below include those quote characters.
	longValue := strings.Repeat("x", 150)
	// A string value under the limit.
	shortValue := strings.Repeat("y", 50)
	tests := []struct {
		name              string
		maxVariableLength int
		expectedLong      string
		expectedShort     string
	}{
		{
			name:              "default limit truncates long values",
			maxVariableLength: 100,
			expectedLong:      `"` + strings.Repeat("x", 97) + "...",
			expectedShort:     `"` + shortValue + `"`,
		},
		{
			name:              "zero limit disables truncation",
			maxVariableLength: 0,
			expectedLong:      `"` + longValue + `"`,
			expectedShort:     `"` + shortValue + `"`,
		},
		{
			name:              "negative limit disables truncation",
			maxVariableLength: -1,
			expectedLong:      `"` + longValue + `"`,
			expectedShort:     `"` + shortValue + `"`,
		},
		{
			name:              "custom limit",
			maxVariableLength: 120,
			expectedLong:      `"` + strings.Repeat("x", 117) + "...",
			expectedShort:     `"` + shortValue + `"`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			vm := newVariableManager(tc.maxVariableLength)
			ref := vm.addVars(func() []namedVar {
				return []namedVar{
					{name: "long", value: ast.StringTerm(longValue).Value},
					{name: "short", value: ast.StringTerm(shortValue).Value},
				}
			})

			vars, err := vm.vars(ref)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			got := map[string]string{}
			for _, v := range vars {
				got[v.Name()] = v.Value()
			}

			if got["long"] != tc.expectedLong {
				t.Errorf("long variable value = %q (len %d), want %q (len %d)",
					got["long"], len(got["long"]), tc.expectedLong, len(tc.expectedLong))
			}
			if got["short"] != tc.expectedShort {
				t.Errorf("short variable value = %q, want %q", got["short"], tc.expectedShort)
			}
		})
	}
}
