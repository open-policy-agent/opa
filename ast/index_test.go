// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"fmt"
	"testing"

	"github.com/open-policy-agent/opa/util/test"
)

type testResolver struct {
	input   *Term
	failRef Ref
}

func (r testResolver) Resolve(ref Ref) (Value, error) {
	if ref.Equal(r.failRef) {
		return nil, fmt.Errorf("some error")
	}
	if ref.HasPrefix(InputRootRef) {
		v, err := r.input.Value.Find(ref[1:])
		if err != nil {
			return nil, nil
		}
		return v, nil
	}
	panic("illegal value")
}

func TestBaseDocEqIndexing(t *testing.T) {

	module := MustParseModule(`
	package test

	exact {
		input.x = 1
		input.y = 2
	} {
		input.x = 3
		input.y = 4
	}

	scalars {
		input.x = 0
		input.y = 1
	} {
		1 = input.y  # exercise ordering
		input.x = 0
	} {
		input.y = 2
		input.z = 2
	} {
		input.x = 2
	}

	vars {
		input.x = 1
		input.y = 2
	} {
		input.x = x
		input.y = 3
	} {
		input.x = 4
		input.z = 5
	}

	composite_arr {
		input.x = 1
		input.y = [1,2,3]
		input.z = 1
	} {
		input.x = 1
		input.y = [1,2,4,x]
	} {
		input.y = [1,2,y,5]
		input.z = 3
	} {
		input.y = []
	} {
		# Must be included in all results as nested composites are not indexed.
		input.y = [1,[2,3],4]
	}

	composite_obj {
		input.y = {"foo": "bar", "bar": x}
	}

	# filtering ruleset contains rules that cannot be indexed (for different reasons).
	filtering {
		count([], x)
	} {
		not input.x = 0
	} {
		x = [1,2,3]
		x[0] = 1
	} {
		input.x[_] = 1
	} {
		input.x[input.y] = 1
	} {
		# include one rule that can be indexed to exercise merging of root non-indexable
		# rules with other rules.
		input.x = 1
	}

	# exercise default keyword
	default allow = false
	allow {
		input.x = 1
	} {
		input.x = 0
	}
	`)

	tests := []struct {
		note       string
		ruleset    string
		input      string
		expectedRS interface{}
		expectedDR *Rule
	}{
		{
			note:    "exact match",
			ruleset: "exact",
			input:   `{"x": 3, "y": 4}`,
			expectedRS: []string{
				`exact { input.x = 3; input.y = 4 }`,
			},
		},
		{
			note:    "undefined match",
			ruleset: "scalars",
			input:   `{"x": 2, "y": 2}`,
			expectedRS: []string{
				`scalars { input.x = 2 }`},
		},
		{
			note:    "disjoint match",
			ruleset: "scalars",
			input:   `{"x": 2, "y": 2, "z": 2}`,
			expectedRS: []string{
				`scalars { input.x = 2 }`,
				`scalars { input.y = 2; input.z = 2}`},
		},
		{
			note:    "ordering match",
			ruleset: "scalars",
			input:   `{"x": 0, "y": 1}`,
			expectedRS: []string{
				`scalars { input.x = 0; input.y = 1 }`,
				`scalars { 1 = input.y; input.x = 0 }`},
		},
		{
			note:    "type no match",
			ruleset: "vars",
			input:   `{"y": 3, "x": {1,2,3}}`,
			expectedRS: []string{
				`vars { input.x = x; input.y = 3 }`,
			},
		},
		{
			note:    "var match",
			ruleset: "vars",
			input:   `{"x": 1, "y": 3}`,
			expectedRS: []string{
				`vars { input.x = x; input.y = 3 }`,
			},
		},
		{
			note:    "var match disjoint",
			ruleset: "vars",
			input:   `{"x": 4, "z": 5, "y": 3}`,
			expectedRS: []string{
				`vars { input.x = x; input.y = 3 }`,
				`vars { input.x = 4; input.z = 5 }`,
			},
		},
		{
			note:    "array match",
			ruleset: "composite_arr",
			input: `{
				"x": 1,
				"y": [1,2,3],
				"z": 1,
			}`,
			expectedRS: []string{
				`composite_arr { input.x = 1; input.y = [1,2,3]; input.z = 1 }`,
				`composite_arr { input.y = [1,[2,3],4] }`,
			},
		},
		{
			note:    "array var match",
			ruleset: "composite_arr",
			input: `{
				"x": 1,
				"y": [1,2,4,5],
			}`,
			expectedRS: []string{
				`composite_arr { input.x = 1; input.y = [1,2,4,x] }`,
				`composite_arr { input.y = [1,[2,3],4] }`,
			},
		},
		{
			note:    "array var multiple match",
			ruleset: "composite_arr",
			input: `{
				"x": 1,
				"y": [1,2,4,5],
				"z": 3,
			}`,
			expectedRS: []string{
				`composite_arr { input.x = 1; input.y = [1,2,4,x] }`,
				`composite_arr { input.y = [1,2,y,5]; input.z = 3 }`,
				`composite_arr { input.y = [1,[2,3],4] }`,
			},
		},
		{
			note:    "array nested match non-indexable rules",
			ruleset: "composite_arr",
			input: `{
				"x": 1,
				"y": [1,[2,3],4],
			}`,
			expectedRS: []string{
				`composite_arr { input.y = [1,[2,3],4] }`,
			},
		},
		{
			note:    "array empty match",
			ruleset: "composite_arr",
			input:   `{"y": []}`,
			expectedRS: []string{
				`composite_arr { input.y = [] }`,
				`composite_arr { input.y = [1,[2,3],4] }`,
			},
		},
		{
			note:    "object match non-indexable rule",
			ruleset: "composite_obj",
			input:   `{"y": {"foo": "bar", "bar": "baz"}}`,
			expectedRS: []string{
				`composite_obj { input.y = {"foo": "bar", "bar": x} }`,
			},
		},
		{
			note:       "default rule only",
			ruleset:    "allow",
			input:      `{"x": 2}`,
			expectedRS: []string{},
			expectedDR: MustParseRule(`default allow = false`),
		},
		{
			note:       "match and default rule",
			ruleset:    "allow",
			input:      `{"x": 1}`,
			expectedRS: []string{"allow { input.x = 1 }"},
			expectedDR: MustParseRule(`default allow = false`),
		},
		{
			note:       "match and non-indexable rules",
			ruleset:    "filtering",
			input:      `{"x": 1}`,
			expectedRS: module.RuleSet(Var("filtering")),
		},
		{
			note:       "non-indexable rules",
			ruleset:    "filtering",
			input:      `{}`,
			expectedRS: module.RuleSet(Var("filtering")).Diff(NewRuleSet(MustParseRule(`filtering { input.x = 1 }`))),
		},
	}

	for _, tc := range tests {
		test.Subtest(t, tc.note, func(t *testing.T) {

			rules := []*Rule{}
			for _, rule := range module.Rules {
				if rule.Head.Name == Var(tc.ruleset) {
					rules = append(rules, rule)
				}
			}

			input := MustParseTerm(tc.input)
			var expectedRS RuleSet

			switch e := tc.expectedRS.(type) {
			case []string:
				for _, r := range e {
					expectedRS.Add(MustParseRule(r))
				}
			case RuleSet:
				expectedRS = e
			default:
				panic("Unexpected test case expected value")
			}

			index := newBaseDocEqIndex(func(Ref) bool {
				return false
			})

			if !index.Build(rules) {
				t.Fatalf("Expected index build to succeed")
			}

			result, err := index.Lookup(testResolver{input, nil})
			if err != nil {
				t.Fatalf("Unexpected error during index lookup: %v", err)
			}

			if !NewRuleSet(result.Rules...).Equal(expectedRS) {
				t.Fatalf("Expected ruleset %v but got: %v", expectedRS, result.Rules)
			}

			if result.Default == nil && tc.expectedDR != nil {
				t.Fatalf("Expected default rule but got nil")
			} else if result.Default != nil && tc.expectedDR == nil {
				t.Fatalf("Unexpected default rule %v", result.Default)
			} else if result.Default != nil && tc.expectedDR != nil && !result.Default.Equal(tc.expectedDR) {
				t.Fatalf("Expected default rule %v but got: %v", tc.expectedDR, result.Default)
			}
		})
	}

}

func TestBaseDocEqIndexingPriorities(t *testing.T) {

	module := MustParseModule(`
	package test

	p {						# r1
		false
	} else {				# r2
		input.x = "x1"
		input.y = "y1"
	} else {				# r3
		input.z = "z1"
	}

	p {						# r4
		input.x = "x1"
	}

	p {						# r5
		input.z = "z2"
	} else {				# r6
		input.z = "z1"
	}
	`)

	index := newBaseDocEqIndex(func(Ref) bool { return false })

	ok := index.Build(module.Rules)
	if !ok {
		t.Fatalf("Expected index build to succeed")
	}

	input := MustParseTerm(`{"x": "x1", "y": "y1", "z": "z1"}`)

	result, err := index.Lookup(testResolver{input, nil})
	if err != nil {
		t.Fatalf("Unexpected error during index lookup: %v", err)
	}

	expectedRules := NewRuleSet(
		module.Rules[0],
		module.Rules[1],
		module.Rules[2].Else)

	expectedElse := map[*Rule]RuleSet{
		module.Rules[0]: []*Rule{
			module.Rules[0].Else,
			module.Rules[0].Else.Else,
		},
	}

	if result.Default != nil {
		t.Fatalf("Expected default rule to be nil")
	}

	if !NewRuleSet(result.Rules...).Equal(expectedRules) {
		t.Fatalf("Expected rules to be %v but got: %v", expectedRules, result.Rules)
	}

	r1 := module.Rules[0]

	if !NewRuleSet(result.Else[r1]...).Equal(expectedElse[r1]) {
		t.Fatalf("Expected else to be %v but got: %v", result.Else[r1], expectedElse[r1])
	}
}

func TestBaseDocEqIndexingErrors(t *testing.T) {
	index := newBaseDocEqIndex(func(Ref) bool {
		return false
	})

	module := MustParseModule(`
	package ex

	p { input.raise_error = 1 }`)

	if !index.Build(module.Rules) {
		t.Fatalf("Expected index to build")
	}

	_, err := index.Lookup(testResolver{MustParseTerm(`{}`), MustParseRef("input.raise_error")})

	if err == nil || err.Error() != "some error" {
		t.Fatalf("Expected error but got: %v", err)
	}

	index = newBaseDocEqIndex(func(Ref) bool { return true })
	if index.Build(nil) {
		t.Fatalf("Expected index build to fail")
	}
}
