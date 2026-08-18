// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"slices"
	"testing"
)

func TestBaseDocEqIndexingLogical(t *testing.T) {
	opts := ParserOptions{
		Capabilities:   CapabilitiesForThisVersion(CapabilitiesExperimentalKeywords(true)),
		FutureKeywords: []string{"and", "or", "not"},
	}

	tests := []struct {
		note     string
		module   string
		input    string
		expected []int
	}{
		// `and`: both operands constrain the rule.
		{
			note: "and: both operands indexed",
			module: `package test
			p if input.x = 1 and input.y = 2	# 0
			p if input.x = 1					# 1
			p if input.y = 2					# 2`,
			input:    `{"x": 1, "y": 2}`,
			expected: []int{0, 1, 2},
		},
		{
			note: "and: rhs operand excludes rule",
			module: `package test
			p if input.x = 1 and input.y = 2	# 0
			p if input.x = 1					# 1
			p if input.y = 2					# 2`,
			input:    `{"x": 1, "y": 9}`,
			expected: []int{1},
		},
		{
			note: "and: lhs operand excludes rule",
			module: `package test
			p if input.x = 1 and input.y = 2	# 0
			p if input.x = 1					# 1
			p if input.y = 2					# 2`,
			input:    `{"x": 9, "y": 2}`,
			expected: []int{2},
		},
		{
			note: "and chain",
			module: `package test
			p if input.x = 1 and input.y = 2 and input.z = 3	# 0
			p if input.x = 1									# 1`,
			input:    `{"x": 1, "y": 2, "z": 9}`,
			expected: []int{1},
		},
		{
			note: "and: alongside sequential expressions",
			module: `package test
			p if {
				input.a = 1
				input.b = 2 and input.c = 3	# 0
			}
			p if input.a = 1				# 1`,
			input:    `{"a": 1, "b": 2, "c": 9}`,
			expected: []int{1},
		},
		{
			note: "and: explicit body operands",
			module: `package test
			p if {input.a = 1; input.b = 2} and input.c = 3	# 0
			p if input.a = 1								# 1`,
			input:    `{"a": 1, "b": 9, "c": 3}`,
			expected: []int{1},
		},
		{
			note: "and: operand with membership",
			module: `package test
			p if {__local0__ = input.x; internal.member_2(__local0__, {1, 2})} and input.y = 3	# 0
			p if input.y = 3																	# 1`,
			input:    `{"x": 2, "y": 3}`,
			expected: []int{0, 1},
		},
		{
			note: "and: operand with membership, no match",
			module: `package test
			p if {__local0__ = input.x; internal.member_2(__local0__, {1, 2})} and input.y = 3	# 0
			p if input.y = 3																	# 1`,
			input:    `{"x": 3, "y": 3}`,
			expected: []int{1},
		},
		{
			note: "and: operand with glob.match",
			module: `package test
			p if {__local0__ = input.x; glob.match("foo:*", [":"], __local0__)} and input.y = 3	# 0
			p if input.y = 3																	# 1`,
			input:    `{"x": "bar:baz", "y": 3}`,
			expected: []int{1},
		},
		{
			note: "and: non-indexable operand doesn't affect the other",
			module: `package test
			p if input.x = 1 and count(input.y) = 3	# 0
			p if input.x = 2						# 1`,
			input:    `{"x": 1, "y": [1, 2, 3]}`,
			expected: []int{0},
		},
		{
			note: "and: each operand has its own scope for constants",
			module: `package test
			p if {v = 1; input.a = v} and {v = 2; input.b = v}	# 0
			p if input.a = 1									# 1`,
			input:    `{"a": 1, "b": 1}`,
			expected: []int{1},
		},
		{
			note: "and: negated operand",
			module: `package test
			p if not input.x = 1 and input.y = 2	# 0
			p if input.y = 9						# 1`,
			input:    `{"x": 5, "y": 2}`,
			expected: []int{0},
		},

		// `or`: only a ref both operands constrain tells us anything.
		{
			note: "or: same ref, either value matches",
			module: `package test
			p if input.x = 1 or input.x = 2	# 0
			p if input.x = 3				# 1`,
			input:    `{"x": 2}`,
			expected: []int{0},
		},
		{
			note: "or: same ref, neither value matches",
			module: `package test
			p if input.x = 1 or input.x = 2	# 0
			p if input.x = 3				# 1`,
			input:    `{"x": 3}`,
			expected: []int{1},
		},
		{
			note: "or chain, same ref",
			module: `package test
			p if input.x = 1 or input.x = 2 or input.x = 3	# 0
			p if input.x = 4								# 1`,
			input:    `{"x": 3}`,
			expected: []int{0},
		},
		{
			note: "or chain, same ref, no match",
			module: `package test
			p if input.x = 1 or input.x = 2 or input.x = 3	# 0
			p if input.x = 4								# 1`,
			input:    `{"x": 9}`,
			expected: []int{},
		},
		{
			note: "or: distinct refs are not indexed",
			module: `package test
			p if input.x = 1 or input.y = 2	# 0
			p if input.x = 9				# 1`,
			input:    `{"y": 2}`,
			expected: []int{0},
		},
		{
			note: "or: distinct refs are not indexed, both operands unsatisfied",
			module: `package test
			p if input.x = 1 or input.y = 2	# 0
			p if input.x = 9				# 1`,
			// Rule 0 remains a candidate: the indexer can't exclude it, and
			// evaluation is what rules it out.
			input:    `{"x": 5, "y": 5}`,
			expected: []int{0},
		},
		{
			note: "or: only the ref shared by both operands is indexed",
			module: `package test
			p if {input.x = 1; input.y = 2} or {input.x = 3; input.z = 4}	# 0
			p if input.x = 9												# 1`,
			// input.y and input.z are dropped, so the mismatched input.y doesn't
			// exclude the rule, but the input.x value does have to be one of the two.
			input:    `{"x": 1, "y": 9}`,
			expected: []int{0},
		},
		{
			note: "or: shared ref excludes rule",
			module: `package test
			p if {input.x = 1; input.y = 2} or {input.x = 3; input.z = 4}	# 0
			p if input.x = 9												# 1`,
			input:    `{"x": 9, "y": 2}`,
			expected: []int{1},
		},
		{
			note: "or: operand allowing any value widens the shared ref",
			module: `package test
			p if input.x = 1 or input.x	# 0
			p if input.x = 9			# 1`,
			input:    `{"x": true}`,
			expected: []int{0},
		},
		{
			note: "or: operand allowing any value still requires the ref to be defined",
			module: `package test
			p if input.x = 1 or input.x	# 0
			p if input.y = 9			# 1`,
			input:    `{"y": 9}`,
			expected: []int{1},
		},
		{
			note: "or: unindexable operand disables indexing for the expression",
			module: `package test
			p if input.x = 1 or count(input.y) = 3	# 0
			p if input.x = 9						# 1`,
			input:    `{"x": 5, "y": [1, 2, 3]}`,
			expected: []int{0},
		},
		{
			note: "or: negated operand disables indexing for the expression",
			module: `package test
			p if not input.x = 1 or input.x = 2	# 0
			p if input.x = 9					# 1`,
			input:    `{"x": 5}`,
			expected: []int{0},
		},
		{
			note: "or: nested in and operand",
			module: `package test
			p if input.a = 1 and {input.b = 2 or input.b = 3}	# 0
			p if input.b = 9									# 1`,
			input:    `{"a": 1, "b": 3}`,
			expected: []int{0},
		},
		{
			note: "or: nested in and operand, shared ref excludes rule",
			module: `package test
			p if input.a = 1 and {input.b = 2 or input.b = 3}	# 0
			p if input.b = 9									# 1`,
			input:    `{"a": 1, "b": 9}`,
			expected: []int{1},
		},
		{
			note: "and: nested in or operand",
			module: `package test
			p if {input.x = 1 and input.y = 2} or {input.x = 3 and input.y = 4}	# 0
			p if input.x = 9													# 1`,
			input:    `{"x": 9, "y": 2}`,
			expected: []int{1},
		},
		{
			note: "and: nested in or operand, only the first multi-valued ref is indexed",
			module: `package test
			p if {input.x = 1 and input.y = 2} or {input.x = 3 and input.y = 4}	# 0
			p if input.x = 9													# 1`,
			// The trie holds one child per value, so Build stops at the first
			// ref it has several values for (input.x, the more frequent one),
			// leaving rule 0 a candidate for any input.y value.
			input:    `{"x": 1, "y": 9}`,
			expected: []int{0},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			mod := MustParseModuleWithOpts(tc.module, opts)

			index := newBaseDocEqIndex(func(Ref) bool { return false })
			if !index.Build(mod.Rules) {
				t.Fatal("expected index build to succeed")
			}

			t.Log(index.root.mermaid())

			result, err := index.Lookup(testResolver{input: MustParseTerm(tc.input)})
			if err != nil {
				t.Fatalf("unexpected error during index lookup: %v", err)
			}

			got := make([]int, 0, len(result.Rules))
			for _, rule := range result.Rules {
				got = append(got, slices.Index(mod.Rules, rule))
			}
			slices.Sort(got)

			if !slices.Equal(got, tc.expected) {
				t.Errorf("expected rules %v but got %v", tc.expected, got)
				for _, rule := range result.Rules {
					t.Logf("  %v", rule)
				}
			}
		})
	}
}
