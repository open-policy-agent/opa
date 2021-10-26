// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import "testing"

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
		{"object/var-3", `{"x": 1, "y": x} = y`, "[]", "[]"},
		{"object/uneven", `{"x": x, "y": 1} = {"x": y}`, "[]", "[]"},
		{"object/uneven", `{"x": x, "y": 1} = {"x": y}`, "[x]", "[]"},
		{"call", "x = f(y)[z]", "[y]", "[x]"},

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
	}

	for i, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {

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
				t.Fatalf("%s (%d): Missing vars: %v, extra vars: %v", tc.note, i, missing, extra)
			}
		})
	}
}
