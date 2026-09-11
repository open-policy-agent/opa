// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"slices"
	"testing"
)

func TestBaseDocEqIndexingLogical(t *testing.T) {
	// `not` for the `not (...)` operand form, which is still gated on the import.
	opts := ParserOptions{FutureKeywords: []string{"and", "or", "not"}}

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
			note: "and: contradicting operands",
			module: `package test
			p if input.x = 1 and input.x = 2	# 0
			p if input.x = 1					# 1
			p if input.x = 2					# 2`,
			// Rule 0 can never be defined, but the index only widens input.x to
			// either value; evaluation is what rules it out.
			input:    `{"x": 1}`,
			expected: []int{0, 1},
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
			note: "and chain, all operands match",
			module: `package test
			p if input.x = 1 and input.y = 2 and input.z = 3	# 0
			p if input.x = 1									# 1`,
			input:    `{"x": 1, "y": 2, "z": 3}`,
			expected: []int{0, 1},
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
			note: "and: alongside sequential expressions, all match",
			module: `package test
			p if {
				input.a = 1
				input.b = 2 and input.c = 3	# 0
			}
			p if input.a = 1				# 1`,
			input:    `{"a": 1, "b": 2, "c": 3}`,
			expected: []int{0, 1},
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
			note: "and: explicit body operands, all match",
			module: `package test
			p if {input.a = 1; input.b = 2} and input.c = 3	# 0
			p if input.a = 1								# 1`,
			input:    `{"a": 1, "b": 2, "c": 3}`,
			expected: []int{0, 1},
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
			note: "and: operand with glob.match, pattern matches",
			module: `package test
			p if {__local0__ = input.x; glob.match("foo:*", [":"], __local0__)} and input.y = 3	# 0
			p if input.y = 3																	# 1`,
			input:    `{"x": "foo:baz", "y": 3}`,
			expected: []int{0, 1},
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
			note: "and: each operand has its own scope for constants, both match",
			module: `package test
			p if {v = 1; input.a = v} and {v = 2; input.b = v}	# 0
			p if input.a = 1									# 1`,
			input:    `{"a": 1, "b": 2}`,
			expected: []int{0, 1},
		},
		{
			note: "and: a var bound to a ref doesn't resolve in the sibling operand",
			module: `package test
			p if {input.b = x} and {x = 1}	# 0
			p if input.b = 1				# 1`,
			// The two x are different vars, so all the index can require of
			// input.b is that it is defined -- not that it is 1.
			input:    `{"b": 5}`,
			expected: []int{0},
		},
		{
			note: "and: negated operand",
			module: `package test
			p if not input.x = 1 and input.y = 2	# 0
			p if input.y = 9						# 1`,
			input:    `{"x": 5, "y": 2}`,
			expected: []int{0},
		},

		{
			note: "and: whole expression negated",
			module: `package test
			p if not (input.x = 1 and input.y = 2)	# 0
			p if input.x = 9						# 1`,
			// Nothing is indexed for a negated expression: the rule is defined
			// for the inputs its operands reject.
			input:    `{"x": 1, "y": 2}`,
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
			note: "or: distinct refs, lhs satisfied",
			module: `package test
			p if input.x = 1 or input.y = 2	# 0
			p if input.x = 9				# 1`,
			input:    `{"x": 1, "y": 9}`,
			expected: []int{0},
		},
		{
			note: "or: distinct refs, rhs satisfied",
			module: `package test
			p if input.x = 1 or input.y = 2	# 0
			p if input.x = 9				# 1`,
			input:    `{"x": 5, "y": 2}`,
			expected: []int{0},
		},
		{
			note: "or: distinct refs, neither operand satisfied",
			module: `package test
			p if input.x = 1 or input.y = 2	# 0
			p if input.x = 9				# 1`,
			// Each operand is its own path, so a rule whose operands mention
			// different refs can be excluded on both of them.
			input:    `{"x": 5, "y": 3}`,
			expected: []int{},
		},
		{
			note: "or: each operand is indexed on all of its refs",
			module: `package test
			p if {input.x = 1; input.y = 2} or {input.x = 3; input.z = 4}	# 0
			p if input.x = 9												# 1`,
			// input.y rules out the lhs, and input.x the rhs.
			input:    `{"x": 1, "y": 9}`,
			expected: []int{},
		},
		{
			note: "or: lhs operand fully satisfied",
			module: `package test
			p if {input.x = 1; input.y = 2} or {input.x = 3; input.z = 4}	# 0
			p if input.x = 9												# 1`,
			input:    `{"x": 1, "y": 2}`,
			expected: []int{0},
		},
		{
			note: "or: rhs operand fully satisfied",
			module: `package test
			p if {input.x = 1; input.y = 2} or {input.x = 3; input.z = 4}	# 0
			p if input.x = 9												# 1`,
			input:    `{"x": 3, "z": 4}`,
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
			// Neither operand holds, so evaluation won't define rule 0, but
			// count() isn't indexable and the index can't exclude the rule on
			// input.x alone.
			input:    `{"x": 5, "y": [1, 2]}`,
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
			note: "and: nested in or operand, neither branch matches fully",
			module: `package test
			p if {input.x = 1 and input.y = 2} or {input.x = 3 and input.y = 4}	# 0
			p if input.x = 9													# 1`,
			input:    `{"x": 1, "y": 9}`,
			expected: []int{},
		},
		{
			note: "and: nested in or operand, one branch matches fully",
			module: `package test
			p if {input.x = 1 and input.y = 2} or {input.x = 3 and input.y = 4}	# 0
			p if input.x = 9													# 1`,
			input:    `{"x": 3, "y": 4}`,
			expected: []int{0},
		},
		{
			note: "or in both operands of an and",
			module: `package test
			p if {input.x = 1 or input.x = 2} and {input.y = 3 or input.y = 4}	# 0
			p if input.x = 9													# 1`,
			// One path per combination: (1,3), (1,4), (2,3), (2,4).
			input:    `{"x": 2, "y": 3}`,
			expected: []int{0},
		},
		{
			note: "or in both operands of an and, no combination matches",
			module: `package test
			p if {input.x = 1 or input.x = 2} and {input.y = 3 or input.y = 4}	# 0
			p if input.x = 9													# 1`,
			input:    `{"x": 2, "y": 9}`,
			expected: []int{},
		},
		{
			note: "too many combinations to index",
			module: `package test
			p if {
				{input.a = 1 or input.a = 2} and
				{input.b = 1 or input.b = 2} and
				{input.c = 1 or input.c = 2} and
				{input.d = 1 or input.d = 2} and
				{input.e = 1 or input.e = 2} and
				{input.f = 1 or input.f = 2}
			}						# 0
			p if input.a = 9		# 1`,
			// 64 combinations is past maxIndexPaths, so the disjunctions are
			// dropped and rule 0 stays a candidate for anything.
			input:    `{"a": 9}`,
			expected: []int{0, 1},
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
