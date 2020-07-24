// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"testing"
)

type testVis struct {
	elems []interface{}
}

func (vis *testVis) Visit(x interface{}) bool {
	vis.elems = append(vis.elems, x)
	return false
}

func TestVisitor(t *testing.T) {

	rule := MustParseModule(`package a.b

import input.x.y as z

t[x] = y {
	p[x] = {"foo": [y, 2, {"bar": 3}]}
	not q[x]
	y = [[x, z] | x = "x"; z = "z"]
	z = {"foo": [x, z] | x = "x"; z = "z"}
	s = {1 | a[i] = "foo"}
	count({1, 2, 3}, n) with input.foo.bar as x
}

p { false } else { false } else { true }

fn([x, y]) = z { json.unmarshal(x, z); z > y }
`)
	vis := &testVis{}

	NewGenericVisitor(vis.Visit).Walk(rule)

	/*
		mod
			package
				data.a.b
					term
						data
					term
						a
					term
						b
			import
				term
					input.x.y
						term
							input
						term
							x
						term
							y
						z
			rule
				head
					t
					args
					term
						x
					term
						y
				body
					expr1
						term
							ref
								term
									=
						term
							ref1
								term
									p
								term
									x
						term
							object1
								term
									"foo"
								term
									array
										term
											y
										term
											2
										term
											object2
												term
													"bar"
												term
													3
					expr2
						term
							ref2
								term
									q
								term
									x
					expr3
						term
							ref
								term
									=
						term
							y
						term
							compr
								term
									array
										term
											x
										term
											z
								body
									expr4
										term
											ref
												term
													=
										term
											x
										term
											"x"
									expr5
										term
											ref
												term
													=
										term
											z
										term
											"z"
					expr4
						term
							ref
								term
									=
						term
							z
						term
							compr
								key
									term
										"foo"
								value
									array
										term
											x
										term
											z
								body
									expr1
										term
											ref
												term
													=
										term
											x
										term
											"x"
									expr2
										term
											ref
												term
													=
										term
											z
										term
											"z"
					expr5
						term
							ref
								term
									=
						term
							s
						term
							compr
								term
									1
								body
									expr1
										term
											ref
												term
													=
										term
											ref
												term
													a
												term
													i

										term
											"foo"
					expr6
						term
							ref
								term
									count
						term
							set
								term
									1
								term
									2
								term
									3
						term
							n
						with
							term
								input.foo.bar
									term
										input
									term
										foo
									term
										bar
							term
								baz
			rule
				head
					p
					args
					<nil> # not counted
					term
						true
				body
					expr
						term
							false
				rule
					head
						p
						args
						<nil> # not counted
						term
							true
					body
						expr
							term
								false
					rule
						head
							p
							args
							<nil> # not counted
							term
								true
						body
							expr
								term
									true
			func
				head
					fn
					args
						term
							array
								term
									x
								term
									y
					term
						z
				body
					expr1
						term
							ref
								term
									json
								term
									unmarshal
						term
							x
						term
							z
					expr2
						term
							ref
								term
									>
						term
							z
						term
							y
	*/
	if len(vis.elems) != 246 {
		t.Errorf("Expected exactly 246 elements in AST but got %d: %v", len(vis.elems), vis.elems)
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

func TestGenericVisitor(t *testing.T) {
	rule := MustParseModule(`package a.b

import input.x.y as z

t[x] = y {
	p[x] = {"foo": [y, 2, {"bar": 3}]}
	not q[x]
	y = [[x, z] | x = "x"; z = "z"]
	z = {"foo": [x, z] | x = "x"; z = "z"}
	s = {1 | a[i] = "foo"}
	count({1, 2, 3}, n) with input.foo.bar as x
}

p { false } else { false } else { true }

fn([x, y]) = z { json.unmarshal(x, z); z > y }
`)

	var elems []interface{}
	vis := NewGenericVisitor(func(x interface{}) bool {
		elems = append(elems, x)
		return false
	})
	vis.Walk(rule)

	if len(elems) != 246 {
		t.Errorf("Expected exactly 246 elements in AST but got %d: %v", len(elems), elems)
	}
}

func TestBeforeAfterVisitor(t *testing.T) {
	rule := MustParseModule(`package a.b

import input.x.y as z

t[x] = y {
	p[x] = {"foo": [y, 2, {"bar": 3}]}
	not q[x]
	y = [[x, z] | x = "x"; z = "z"]
	z = {"foo": [x, z] | x = "x"; z = "z"}
	s = {1 | a[i] = "foo"}
	count({1, 2, 3}, n) with input.foo.bar as x
}

p { false } else { false } else { true }

fn([x, y]) = z { json.unmarshal(x, z); z > y }
`)

	var before, after []interface{}
	vis := NewBeforeAfterVisitor(func(x interface{}) bool {
		before = append(before, x)
		return false
	},
		func(x interface{}) {
			after = append(after, x)
		})
	vis.Walk(rule)

	if len(before) != 246 {
		t.Errorf("Expected exactly 246 before elements in AST but got %d: %v", len(before), before)
	}

	if len(after) != 246 {
		t.Errorf("Expected exactly 246 after elements in AST but got %d: %v", len(after), after)
	}
}

func TestVarVisitor(t *testing.T) {

	tests := []struct {
		stmt     string
		params   VarVisitorParams
		expected string
	}{
		{"{x: y}", VarVisitorParams{SkipObjectKeys: true}, "[y]"},
		{"foo with input.bar.baz as qux[corge]", VarVisitorParams{SkipWithTarget: true}, "[foo, qux, corge]"},
		{"data.foo[x] = bar.baz[y]", VarVisitorParams{SkipRefHead: true}, "[x, y]"},
		{`foo = [x | data.a[i] = x]`, VarVisitorParams{SkipClosures: true}, "[foo, eq]"},
		{`x = 1; y = 2; z = x + y; count([x, y, z], z)`, VarVisitorParams{}, "[x, y, z, eq, plus, count]"},
	}

	for _, tc := range tests {
		stmt := MustParseStatement(tc.stmt)

		expected := NewVarSet()
		MustParseTerm(tc.expected).Value.(Array).Foreach(func(x *Term) {
			expected.Add(x.Value.(Var))
		})

		vis := NewVarVisitor().WithParams(tc.params)
		vis.Walk(stmt)

		if !vis.Vars().Equal(expected) {
			t.Errorf("For %v w/ %v expected %v but got: %v", stmt, tc.params, expected, vis.Vars())
		}
	}
}
