// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package util

import "testing"
import "math"

func TestCompare(t *testing.T) {
	tests := []struct {
		a        interface{}
		b        interface{}
		expected int
	}{
		{nil, nil, 0},
		{nil, true, -1},
		{nil, false, -1},
		{false, false, 0},
		{false, true, -1},
		{true, true, 0},
		{true, false, 1},
		{true, float64(0), -1},
		{float64(0), float64(0), 0},
		{float64(0), float64(-1), 1},
		{float64(-1), float64(0), -1},
		{math.MaxFloat64, math.SmallestNonzeroFloat64, 1},
		{float64(-1), "", -1},
		{"", "", 0},
		{"hello", "", 1},
		{"hello world", "hello worldz", -1},
		{[]interface{}{}, "", 1},
		{[]interface{}{}, []interface{}{}, 0},
		{[]interface{}{true, false}, []interface{}{true, nil}, 1},
		{[]interface{}{true, true}, []interface{}{true, true}, 0},
		{[]interface{}{true, false}, []interface{}{true, true}, -1},
		{map[string]interface{}{}, []interface{}{}, 1},
		{map[string]interface{}{"foo": []interface{}{true, false}, "bar": []interface{}{true, true}}, map[string]interface{}{"foo": []interface{}{true, false}, "bar": []interface{}{true, true}}, 0},
		{map[string]interface{}{"foo": []interface{}{true, false}, "bar": []interface{}{true, nil}}, map[string]interface{}{"foo": []interface{}{true, false}, "bar": []interface{}{true, true}}, -1},
		{map[string]interface{}{"foo": []interface{}{true, true}, "bar": []interface{}{true, true}}, map[string]interface{}{"foo": []interface{}{true, false}, "bar": []interface{}{true, true}}, 1},
		{map[string]interface{}{"foo": true, "barr": false}, map[string]interface{}{"foo": true, "bar": false}, 1},
		{map[string]interface{}{"foo": true, "bar": false, "qux": false}, map[string]interface{}{"foo": true, "bar": false}, 1},
		{map[string]interface{}{"foo": true, "bar": false, "baz": false}, map[string]interface{}{"foo": true, "bar": false}, -1},
	}
	for i, tc := range tests {
		result := Compare(tc.a, tc.b)
		if result != tc.expected {
			t.Errorf("Test case %d: expected %d but got: %d", i, tc.expected, result)
		}
	}
}
