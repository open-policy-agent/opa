// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import "testing"

func TestCompare(t *testing.T) {

	// Many of the comparison cases are covered by existing equality tests. Here
	// we cover edge cases.
	tests := []struct {
		a        string
		b        string
		expected int
	}{
		// Comparisons to Go nil. Everything is greater than nil and nil is equal to nil
		{"null", "", 1},
		{"", "null", -1},
		{"", "", 0},

		// Booleans
		{"false", "true", -1},
		{"true", "false", 1},

		// Numbers
		{"0", "1", -1},
		{"1", "0", 1},
		{"0", "0", 0},
		{"0", "1.5", -1},
		{"1.5", "0", 1},
		{"123456789123456789123", "123456789123456789123", 0},
		{"123456789123456789123", "123456789123456789122", 1},
		{"123456789123456789122", "123456789123456789123", -1},
		{"123456789123456789123.5", "123456789123456789123.5", 0},
		{"123456789123456789123.5", "123456789123456789122.5", 1},
		{"123456789123456789122.5", "123456789123456789123.5", -1},

		// Object comparisons are consistent
		{`{1: 2, 3: 4}`, `{4: 3, 1: 2}`, -1},
		{`{1: 2, 3: 4}`, `{1: 2, 4: 3}`, -1},
		{`{1: 2, 3: 4}`, `{1: 2, 3: 5}`, -1},
		{`{1: 2, 3: 4}`, `{1: 2, 3: 4, 5: 6}`, -1},
		{`{1: 2, 3: 4, 5: 6}`, `{1: 2, 3: 4}`, 1},

		// Comprehensions
		{`[null | true]`, `[false | null]`, -1},
		{`{null | true}`, `{false | null}`, -1},
		{`{"abc": null | true}`, `{"cba": false | null}`, -1},

		// Expressions
		{`a = b`, `b = a`, -1},
		{`b = a`, `not a = b`, -1},
		{`a = b`, `x`, 1},
		{`a = b`, `a = b with input.foo as bar`, -1},
		{`a = b with input.foo as bar`, `a = b`, 1},
		{`a = b with input.foo as bar`, `a = b with input.foo.bar.baz as qux`, -1},
		{`a = b with input.foo as bar`, `a = b with input.foo as bar with input.baz as qux`, -1},

		// Body
		{`a = b`, `a = b; b = a`, -1},
		{`a = b; b = a`, `a = b`, 1},
	}
	for _, tc := range tests {
		var a, b interface{}
		if len(tc.a) > 0 {
			a = MustParseStatement(tc.a)
		}
		if len(tc.b) > 0 {
			b = MustParseStatement(tc.b)
		}
		result := Compare(a, b)
		if tc.expected != result {
			t.Errorf("Expected %v.Compare(%v) == %v but got %v", a, b, tc.expected, result)
		}
	}
}

func TestCompareModule(t *testing.T) {
	a := MustParseModule(`package a.b.c`)
	b := MustParseModule(`package a.b.d`)
	result := Compare(a, b)

	if result != -1 {
		t.Errorf("Expected %v to be less than %v but got: %v", a, b, result)
	}

	a = MustParseModule(`package a.b.c

import input.x.y`)
	b = MustParseModule(`package a.b.c

import input.x.z`)
	result = Compare(a, b)

	if result != -1 {
		t.Errorf("Expected %v to be less than %v but got: %v", a, b, result)
	}
}
