// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"testing"
)

func TestReachable(t *testing.T) {
	data := map[string]interface{}{}
	modules := []string{}
	tests := []struct {
		note     string
		rules    []string
		expected interface{}
		input    string
	}{
		{
			"empty",
			[]string{`p = x {x := graph.reachable({}, {"a"})}`},
			`[]`,
			`{}`,
		},
		{
			"cycle",
			[]string{
				`p = x {
				  x := sort(graph.reachable(
				    {
				      "a": {"b"},
				      "b": {"c"},
				      "c": {"a"},
				    },
				    {"a"}
				  ))
				}`},
			`["a", "b", "c"]`,
			`{}`,
		},
		{
			"components",
			[]string{
				`p = x {
				  x := sort(graph.reachable(
				    {
				      "a": {"b", "c"},
				      "b": {"d"},
				      "c": {"d"},
				      "d": set(),
				      "e": {"f"},
				      "f": {"e"},
				      "x": {"x"},
				    },
				    {"b", "e"}
				  ))
				}`},
			`["b", "d", "e", "f"]`,
			`{}`,
		},
		{
			"arrays",
			[]string{
				`p = x {
				  x := sort(graph.reachable(
				    {
				      "a": ["b"],
				      "b": ["c"],
				      "c": ["a"],
				    },
				    ["a"]
				  ))
				}`},
			`["a", "b", "c"]`,
			`{}`,
		},
		{
			"malformed 1",
			[]string{`p = x {x := graph.reachable(input.graph, input.initial)}`},
			`[]`,
			`{"graph": 1, "initial": [1]}`,
		},
		{
			"malformed 2",
			[]string{`p = x {x := graph.reachable(input.graph, input.initial)}`},
			`["a"]`,
			`{"graph": {"a": null}, "initial": ["a"]}`,
		},
		{
			"malformed 3",
			[]string{`p = x {x := graph.reachable(input.graph, input.initial)}`},
			`[]`,
			`{"graph": {"a": []}, "initial": "a"}`,
		},
	}

	for _, tc := range tests {
		runTopDownTestCaseWithModules(t, data, tc.note, tc.rules, modules, tc.input, tc.expected)
	}
}
