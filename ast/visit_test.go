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
	import x.y as z
	t[x] = y :- p[x] = {"foo": [y,2,{"bar": 3}]}, not q[x]
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
				x.y
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
	*/
	if len(vis.elems) != 33 {
		t.Errorf("Expected exactly 33 elements in AST but got %d: %v", len(vis.elems), vis.elems)
	}

}
