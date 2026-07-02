// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package util

import (
	"encoding/json"
	"testing"
)

func TestCompare(t *testing.T) {

	tests := []struct {
		a        any
		b        any
		expected int
	}{
		{nil, nil, 0},
		{nil, true, -1},
		{nil, false, -1},
		{false, false, 0},
		{false, true, -1},
		{true, true, 0},
		{true, false, 1},
		{true, json.Number("0"), -1},
		{json.Number("0"), json.Number("0"), 0},
		{json.Number("0"), json.Number("-1"), 1},
		{json.Number("-1"), json.Number("0"), -1},
		{json.Number("1.797693134862315708145274237317043567981e+308"), json.Number("4.940656458412465441765687928682213723651e-324"), 1},
		{json.Number("-1"), "", -1},
		{"", "", 0},
		{"hello", "", 1},
		{"hello world", "hello worldz", -1},
		{[]any{}, "", 1},
		{[]any{}, []any{}, 0},
		{[]any{true, false}, []any{true, nil}, 1},
		{[]any{true, true}, []any{true, true}, 0},
		{[]any{true, false}, []any{true, true}, -1},
		{map[string]any{}, []any{}, 1},
		{map[string]any{"foo": []any{true, false}, "bar": []any{true, true}}, map[string]any{"foo": []any{true, false}, "bar": []any{true, true}}, 0},
		{map[string]any{"foo": []any{true, false}, "bar": []any{true, nil}}, map[string]any{"foo": []any{true, false}, "bar": []any{true, true}}, -1},
		{map[string]any{"foo": []any{true, true}, "bar": []any{true, true}}, map[string]any{"foo": []any{true, false}, "bar": []any{true, true}}, 1},
		{map[string]any{"foo": true, "barr": false}, map[string]any{"foo": true, "bar": false}, 1},
		{map[string]any{"foo": true, "bar": false, "qux": false}, map[string]any{"foo": true, "bar": false}, 1},
		{map[string]any{"foo": true, "bar": false, "baz": false}, map[string]any{"foo": true, "bar": false}, -1},
	}
	for i, tc := range tests {
		result := Compare(tc.a, tc.b)
		if result != tc.expected {
			t.Errorf("Test case %d: expected %d but got: %d", i, tc.expected, result)
		}
	}
}
