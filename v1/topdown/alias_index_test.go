// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import "testing"

func TestTopDownRuleIndexAliases(t *testing.T) {
	aliasModule := `package alias

path := input.path`

	chainedModule := `package alias

req := input.attributes

method := req.method`

	tests := []struct {
		note     string
		rules    []string
		modules  []string
		input    string
		expected any
	}{
		{
			note:     "match through alias",
			rules:    []string{`p if { data.alias.path == ["b"] }`},
			modules:  []string{aliasModule},
			input:    `{"path": ["b"]}`,
			expected: "true",
		},
		{
			note:     "match through chained alias",
			rules:    []string{`p if { data.alias.method == "GET" }`},
			modules:  []string{chainedModule},
			input:    `{"attributes": {"method": "GET"}}`,
			expected: "true",
		},
		{
			note: "wildcard ref through alias",
			rules: []string{
				`p if { data.alias.path[_] == "a" }`,
				`p if { data.alias.path[_] == "b" }`,
			},
			modules:  []string{aliasModule},
			input:    `{"path": ["b"]}`,
			expected: "true",
		},
		{
			note: "membership through alias",
			rules: []string{
				`p if { "a" in data.alias.path }`,
				`p if { "b" in data.alias.path }`,
			},
			modules:  []string{aliasModule},
			input:    `{"path": ["b"]}`,
			expected: "true",
		},
		{
			note: "no wildcard match through alias leaves the rule undefined",
			rules: []string{
				`p if { not q }`,
				`q if { data.alias.path[_] == "a" }`,
			},
			modules:  []string{aliasModule},
			input:    `{"path": ["b"]}`,
			expected: "true",
		},
		{
			note: "with mock on the alias falls back to the unindexed rule set",
			rules: []string{
				`p if { q with data.alias.path as ["b"] }`,
				`q if { data.alias.path == ["b"] }`,
			},
			modules:  []string{aliasModule},
			input:    `{"path": ["a"]}`,
			expected: "true",
		},
		{
			note: "with mock on the alias package falls back to the unindexed rule set",
			rules: []string{
				`p if { q with data.alias as {"path": ["b"]} }`,
				`q if { data.alias.path == ["b"] }`,
			},
			modules:  []string{aliasModule},
			input:    `{"path": ["a"]}`,
			expected: "true",
		},
		{
			note: "with mock on input resolves through the index",
			rules: []string{
				`p if { q with input.path as ["b"] }`,
				`q if { data.alias.path == ["b"] }`,
			},
			modules:  []string{aliasModule},
			input:    `{"path": ["a"]}`,
			expected: "true",
		},
		{
			note: "missing input key leaves the rule undefined",
			rules: []string{
				`p if { not q }`,
				`q if { data.alias.path == ["b"] }`,
			},
			modules:  []string{aliasModule},
			input:    `{}`,
			expected: "true",
		},
	}

	for _, tc := range tests {
		runTopDownTestCaseWithModules(t, map[string]any{}, tc.note, tc.rules, tc.modules, tc.input, tc.expected)
	}
}
