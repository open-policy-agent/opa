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
	some x0, y0, z0
	count({1, 2, 3}, n) with input.foo.bar as x
}

p { false } else { false } else { true }

fn([x, y]) = z { json.unmarshal(x, z); z > y }
`)
	vis := &testVis{}
	NewGenericVisitor(vis.Visit).Walk(rule)

	if exp, act := 254, len(vis.elems); exp != act {
		t.Errorf("Expected exactly %d elements in AST but got %d: %v", exp, act, vis.elems)
	}
}

func TestVisitorAnnotations(t *testing.T) {

	module, err := ParseModuleWithOpts("test.rego", `package test

# METADATA
# scope: rule
p := 7`, ParserOptions{ProcessAnnotation: true})
	if err != nil {
		t.Fatal(err)
	}

	vis := &testVis{}

	NewGenericVisitor(vis.Visit).Walk(module)

	exp := 20

	if len(vis.elems) != exp {
		t.Fatalf("expected %d elements but got %v: %v", exp, len(vis.elems), vis.elems)
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
	some x0, y0, z0
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

	if len(elems) != 254 {
		t.Errorf("Expected exactly 254 elements in AST but got %d: %v", len(elems), elems)
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
	some x0, y0, z0
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

	if exp, act := 264, len(before); exp != act {
		t.Errorf("Expected exactly %d before elements in AST but got %d: %v", exp, act, before)
	}

	if exp, act := 264, len(before); exp != act {
		t.Errorf("Expected exactly %d after elements in AST but got %d: %v", exp, act, after)
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
		{"some x, y", VarVisitorParams{}, "[x, y]"},
	}

	for _, tc := range tests {
		t.Run(tc.stmt, func(t *testing.T) {
			stmt := MustParseStatement(tc.stmt)

			expected := NewVarSet()
			MustParseTerm(tc.expected).Value.(*Array).Foreach(func(x *Term) {
				expected.Add(x.Value.(Var))
			})

			vis := NewVarVisitor().WithParams(tc.params)
			vis.Walk(stmt)

			if !vis.Vars().Equal(expected) {
				t.Errorf("Params %#v expected %v but got: %v", tc.params, expected, vis.Vars())
			}
		})
	}
}

func TestGenericVisitorLazyObject(t *testing.T) {
	o := LazyObject(map[string]interface{}{"foo": 3})
	act := 0
	WalkTerms(o, func(n *Term) bool {
		switch n.Value {
		case String("foo"):
			act++
		case Number("3"):
			act++
		}

		return false
	})
	if exp := 2; exp != act {
		t.Errorf("expected %v, got %v", exp, act)
	}
}

func TestGenericBeforeAfterVisitorLazyObject(t *testing.T) {
	o := LazyObject(map[string]interface{}{"foo": 3})
	act := 0
	vis := NewBeforeAfterVisitor(func(x interface{}) bool {
		t, ok := x.(*Term)
		if !ok {
			return false
		}
		switch t.Value {
		case String("foo"):
			act++
		case Number("3"):
			act++
		}

		return false
	},
		func(interface{}) {})
	vis.Walk(o)
	if exp := 2; exp != act {
		t.Errorf("expected %v, got %v", exp, act)
	}
}
