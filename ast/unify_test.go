// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"fmt"
	"testing"
)

func TestUnify(t *testing.T) {

	tests := []struct {
		note     string
		expr     string
		safe     string
		expected string
	}{
		// collection cases
		{"array/ref", "[1,2,x] = a[_]", "[a]", "[x]"},
		{"array/ref (reversed)", "a[_] = [1,2,x]", "[a]", "[x]"},
		{"array/var", "[1,2,x] = y", "[x]", "[y]"},
		{"array/var (reversed)", "y = [1,2,x]", "[x]", "[y]"},
		{"array/var-2", "[1,2,x] = y", "[y]", "[x]"},
		{"array/var-2 (reversed)", "y = [1,2,x]", "[y]", "[x]"},
		{"array/uneven", "[1,2,x] = [y,x]", "[]", "[]"},
		{"array/uneven-2", "[1,2,x] = [y,x]", "[x]", "[]"},
		{"object/ref", `{"x": x} = a[_]`, "[a]", "[x]"},
		{"object/ref (reversed)", `a[_] = {"x": x}`, "[a]", "[x]"},
		{"object/var", `{"x": 1, "y": x} = y`, "[x]", "[y]"},
		{"object/var (reversed)", `y = {"x": 1, "y": x}`, "[x]", "[y]"},
		{"object/var-2", `{"x": 1, "y": x} = y`, "[y]", "[x]"},
		{"object/uneven", `{"x": x, "y": 1} = {"x": y}`, "[]", "[]"},
		{"object/uneven", `{"x": x, "y": 1} = {"x": y}`, "[x]", "[]"},
		{"var/call-ref", "x = f(y)[z]", "[y]", "[x]"},
		{"var/call-ref (reversed)", "f(y)[z] = x", "[y]", "[x]"},
		{"var/call", "x = f(z)", "[z]", "[x]"},
		{"var/call (reversed)", "f(z) = x", "[z]", "[x]"},
		{"array/call", "[x, y] = f(z)", "[z]", "[x,y]"},
		{"array/call (reversed)", "f(z) = [x, y]", "[z]", "[x,y]"},
		{"object/call", `{"a": x} = f(z)`, "[z]", "[x]"},
		{"object/call (reversed)", `f(z) = {"a": x}`, "[z]", "[x]"},

		// transitive cases
		{"trans/redundant", "[x, x] = [x, 0]", "[]", "[x]"},
		{"trans/simple", "[x, 1] = [y, y]", "[]", "[y, x]"},
		{"trans/array", "[x, y] = [y, [z, a]]", "[x]", "[a, y, z]"},
		{"trans/object", `[x, y] = [y, {"a":a,"z":z}]`, "[x]", "[a, y, z]"},
		{"trans/ref", "[x, y, [x, y, i]] = [1, a[i], z]", "[a, i]", "[x, y, z]"},
		{"trans/lazy", "[x, z, 2] = [1, [y, x], y]", "[]", "[x, y, z]"},
		{"trans/redundant-nested", "[x, z, z] = [1, [y, x], [2, 1]]", "[]", "[x, y, z]"},
		{"trans/bidirectional", "[x, z, y] = [[z,y], [1,y], 2]", "[]", "[x, y, z]"},
		{"trans/occurs", "[x, z, y] = [[y,z], [y, 1], [2, x]]", "[]", "[]"},

		// unsafe refs
		{note: "array/ref", expr: "[1,2,x] = a[_]"},
		{note: "array/ref (reversed)", expr: "a[_] = [1,2,x]"},
		{note: "object/ref", expr: `{"x": x} = a[_]`},
		{note: "object/ref (reversed)", expr: `a[_] = {"x": x}`},
		{note: "var/call-ref", expr: "x = f(y)[z]"},
		{note: "var/call-ref (reversed)", expr: "f(y)[z] = x"},

		// unsafe vars
		{note: "array/var", expr: "[1,2,x] = y"},
		{note: "array/var (reversed)", expr: "y = [1,2,x]"},
		{note: "object/var", expr: `{"x": 1, "y": x} = y`},
		{note: "object/var (reversed)", expr: `y = {"x": 1, "y": x}`},
		{note: "var/call", expr: "x = f(z)"},
		{note: "var/call (reversed)", expr: "f(z) = x"},

		// unsafe call args
		{note: "var/call-2", expr: "x = f(z)", safe: "[x]"},
		{note: "var/call-2 (reversed)", expr: "f(z) = x", safe: "[x]"},
		{note: "array/call", expr: "[x, y] = f(z)", safe: "[x,y]"},
		{note: "array/call (reversed)", expr: "f(z) = [x, y]", safe: "[x,y]"},
		{note: "object/call", expr: `{"a": x} = f(z)`, safe: "[x]"},
		{note: "object/call (reversed)", expr: `f(z) = {"a": x}`, safe: "[x]"},

		// partial cases
		{note: "trans/ref", expr: "[x, y, [x, y, i]] = [1, a[i], z]", safe: "[a]", expected: "[x, y]"},
		{note: "trans/ref", expr: "[x, y, [x, y, i]] = [1, a[i], z]", expected: "[x]"},
	}

	for _, tc := range tests {
		if tc.expected == "" {
			tc.expected = "[]"
		}
		if tc.safe == "" {
			tc.safe = "[]"
		}
		t.Run(fmt.Sprintf("%s/%s/%s", tc.note, tc.safe, tc.expected), func(t *testing.T) {

			expr := MustParseBody(tc.expr)[0]
			safe := VarSet{}
			MustParseTerm(tc.safe).Value.(*Array).Foreach(func(x *Term) {
				safe.Add(x.Value.(Var))
			})

			terms := expr.Terms.([]*Term)
			if !expr.IsEquality() {
				panic(expr)
			}

			a, b := terms[1], terms[2]
			unified := Unify(safe, a, b)
			result := VarSet{}
			for k := range unified {
				result.Add(k)
			}

			expected := VarSet{}
			MustParseTerm(tc.expected).Value.(*Array).Foreach(func(x *Term) {
				expected.Add(x.Value.(Var))
			})

			missing := expected.Diff(result)
			extra := result.Diff(expected)
			if len(missing) != 0 || len(extra) != 0 {
				t.Fatalf("missing vars: %v, extra vars: %v", missing, extra)
			}
		})
	}
}
