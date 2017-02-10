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

	rule := MustParseModule(`package a.b

import input.x.y as z

t[x] = y { p[x] = {"foo": [y, 2, {"bar": 3}]}; not q[x]; y = [[x, z] | x = "x"; z = "z"]; count({1, 2, 3}, n) with input.foo.bar as x }`,
	)
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
				input.x.y
					input
					x
					y
				z
			rule
				head
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
						with
							input.foo.bar
								input
								foo
								bar
							baz
	*/
	if len(vis.elems) != 64 {
		t.Errorf("Expected exactly 64 elements in AST but got %d: %v", len(vis.elems), vis.elems)
	}

}

func TestWalkVars(t *testing.T) {
	x := MustParseBody(`x = 1; data.abc[2] = y; y[z] = [q | q = 1]`)
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

func TestVarVisitor(t *testing.T) {

	tests := []struct {
		stmt     string
		params   VarVisitorParams
		expected string
	}{
		{"data.foo[x] = bar.baz[y]", VarVisitorParams{SkipRefHead: true}, "[eq, x, y]"},
		{"{x: y}", VarVisitorParams{SkipObjectKeys: true}, "[y]"},
		{`foo = [x | data.a[i] = x]`, VarVisitorParams{SkipClosures: true}, "[eq, foo]"},
		{`x = 1; y = 2; z = x + y; count([x, y, z], z)`, VarVisitorParams{SkipBuiltinOperators: true}, "[x, y, z]"},
		{"foo with input.bar.baz as qux[corge]", VarVisitorParams{SkipWithTarget: true}, "[foo, qux, corge]"},
	}

	for _, tc := range tests {
		stmt := MustParseStatement(tc.stmt)

		vis := NewVarVisitor().WithParams(tc.params)
		Walk(vis, stmt)

		expected := NewVarSet()
		for _, x := range MustParseTerm(tc.expected).Value.(Array) {
			expected.Add(x.Value.(Var))
		}

		if !vis.Vars().Equal(expected) {
			t.Errorf("For %v w/ %v expected %v but got: %v", stmt, tc.params, expected, vis.Vars())
		}
	}
}
