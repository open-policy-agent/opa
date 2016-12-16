// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import "testing"

type testVis struct {
	elems []interface{}
}

func (vis *testVis) Visit(x interface{}) Visitor {
	vis.elems = append(vis.elems, x)
	return vis
}

func TestVisitor(t *testing.T) {

	rule := MustParseModule(`
	package a.b
	import request.x.y as z
	t[x] = y :-
		p[x] = {"foo": [y,2,{"bar": 3}]},
		not q[x],
		y = [ [x,z] | x = "x", z = "z" ],
		count({1,2,3}, n)
	`)
	vis := &testVis{}
	Walk(vis, rule)

	/*
		mod
			package
				data.a.b
					data
					a
					b
			import
				request.x.y
					request
					x
					y
				z
			rule
				t
				x
				y
				body
					expr1
						=
						ref1
							p
							x
						object1
							"foo"
							array
								y
								2
								object2
									"bar"
									3
					expr2
						ref2
							q
							x
					expr3
						=
						y
						compr
							array
								x
								z
							body
								expr4
									=
									x
									"x"
								expr5
									=
									z
									"z"
					expr4
						count
						set
							1
							2
							3
						n
	*/
	if len(vis.elems) != 57 {
		t.Errorf("Expected exactly 57 elements in AST but got %d: %v", len(vis.elems), vis.elems)
	}

}

func TestWalkVars(t *testing.T) {
	x := MustParseBody("x = 1, data.abc[2] = y, y[z] = [q | q = 1]")
	found := NewVarSet()
	WalkVars(x, func(v Var) bool {
		found.Add(v)
		return false
	})
	expected := NewVarSet(Var("x"), Var("data"), Var("y"), Var("z"), Var("q"), Var("eq"))
	if !expected.Equal(found) {
		t.Fatalf("Expected %v but got: %v", expected, found)
	}
}
