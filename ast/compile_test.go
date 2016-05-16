// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"fmt"
	"reflect"
	"sort"
	"testing"
)

func TestCompilerEmpty(t *testing.T) {
	c := NewCompiler()
	c.Compile(nil)
	assertNotFailed(t, c)
}

func TestCompilerExample(t *testing.T) {
	c := NewCompiler()
	m, err := ParseModule(testModule)
	if err != nil {
		panic(err)
	}
	c.Compile(map[string]*Module{"testMod": m})
	assertNotFailed(t, c)
}

func TestCompilerSetExports(t *testing.T) {
	c := NewCompiler()
	c.Modules = getCompilerTestModules()

	c.setExports()

	assertNotFailed(t, c)
	assertExports(t, c, "data.a.b.d", []string{"t", "x"})
	assertExports(t, c, "data.a.b.c", []string{"p", "q", "r", "z"})
	assertExports(t, c, "data.a.b.empty", nil)
}

func TestCompilerSetGlobals(t *testing.T) {
	c := NewCompiler()
	c.Modules = getCompilerTestModules()
	c.setExports()

	c.setGlobals()

	assertNotFailed(t, c)
	assertGlobals(t, c, c.Modules["mod4"], "{}")
	assertGlobals(t, c, c.Modules["mod1"], `{
		r: data.a.b.c.r,
		p: data.a.b.c.p,
		q: data.a.b.c.q,
		z: data.a.b.c.z,
		foo: data.x.y.z,
		k: data.g.h.k}`)
	assertGlobals(t, c, c.Modules["mod3"], `{
		t: data.a.b.d.t,
		x: data.a.b.d.x,
		y: x,
		req: req}`)
	assertGlobals(t, c, c.Modules["mod2"], `{
		r: data.a.b.c.r,
		p: data.x.y.p,
		q: data.a.b.c.q,
		z: data.a.b.c.z,
		bar: data.bar}`)
}

func TestCompilerResolveAllRefs(t *testing.T) {
	c := NewCompiler()
	c.Modules = getCompilerTestModules()
	c.setExports()
	c.setGlobals()

	c.resolveAllRefs()

	assertNotFailed(t, c)

	mod1 := c.Modules["mod1"]
	p := mod1.Rules[0]
	expr1 := p.Body[0]
	term := expr1.Terms.(*Term)
	e := MustParseTerm("data.a.b.c.q[x]")
	if !term.Equal(e) {
		t.Errorf("Wrong term (global in same module): expected %v but got: %v", e, term)
	}

	expr2 := p.Body[1]
	term = expr2.Terms.(*Term)
	e = MustParseTerm("data.a.b.c.r[x]")
	if !term.Equal(e) {
		t.Errorf("Wrong term (global in same package/diff module): expected %v but got: %v", e, term)
	}

	mod2 := c.Modules["mod2"]
	r := mod2.Rules[0]
	expr3 := r.Body[1]
	term = expr3.Terms.([]*Term)[1]
	e = MustParseTerm("data.x.y.p")
	if !term.Equal(e) {
		t.Errorf("Wrong term (var import): expected %v but got: %v", e, term)
	}

	mod3 := c.Modules["mod3"]
	expr4 := mod3.Rules[0].Body[0]
	term = expr4.Terms.([]*Term)[2]
	e = MustParseTerm("{x.secret: [x.keyid]}")
	if !term.Equal(e) {
		t.Errorf("Wrong term (nested refs): expected %v but got: %v", e, term)
	}
}

func TestCompilerCheckSafetyHead(t *testing.T) {
	c := NewCompiler()
	c.Modules = getCompilerTestModules()
	c.Modules["newMod"] = MustParseModule(`
	package a.b
	unboundKey[x] = y :- q[y] = {"foo": [1,2,[{"bar": y}]]}
	unboundVal[y] = x :- q[y] = {"foo": [1,2,[{"bar": y}]]}
	unboundCompositeVal[y] = [{"foo": x, "bar": y}] :- q[y] = {"foo": [1,2,[{"bar": y}]]}
	`)
	c.setExports()
	c.setGlobals()
	c.resolveAllRefs()
	assertNotFailed(t, c)
	c.checkSafetyHead()
	if len(c.Errors) != 3 {
		t.Errorf("Expected exactly 3 errors but got: %v", c.Errors)
		return
	}
}

func TestCompilerCheckSafetyBodyReordering(t *testing.T) {
	c := NewCompiler()
	c.Modules = getCompilerTestModules()
	c.Modules["newMod"] = MustParseModule(`
	package a.b
	needsReorder = true :- a[i] = x, a = [1,2,3,4]
	`)
	c.setExports()
	c.setGlobals()
	c.resolveAllRefs()
	c.checkSafetyHead()

	c.checkSafetyBody()

	assertNotFailed(t, c)

	expected := MustParseBody(`
	a = [1,2,3,4], a[i] = x
	`)

	reordered := c.Modules["newMod"].Rules[0].Body

	if !expected.Equal(reordered) {
		t.Errorf("Expected body to be re-ordered and equal to %v but got: %v", expected, reordered)
	}
}

func TestCompilerCheckSafetyBodyErrors(t *testing.T) {
	c := NewCompiler()

	c.Modules = getCompilerTestModules()
	c.Modules["newMod"] = MustParseModule(`
	package a.b

	# a would be unbound
	unboundRef1 = true :- a.b.c = "foo"

	# a would be unbound
	unboundRef2 = true :- {"foo": [{"bar": a.b.c}]} = {"foo": [{"bar": "baz"}]}

	# i and x would be unbound
	unboundNegated1 = true :- a = [1,2,3,4], not a[i] = x

	# i and x would be unbound even though x appears in head
	unboundNegated2[x] :- a = [1,2,3,4], not a[i] = x

	# x, i, and j would be unbound even though they appear in other expressions
	unboundNegated3[x] = true :- a = [1,2,3,4], b = [1,2,3,4], not a[i] = x, not b[j] = x

	# i and j would be unbound even though they are in embedded references
	unboundNegated4 =  true :- a = [{"foo": ["bar", "baz"]}], not a[0].foo = [a[0].foo[i], a[0].foo[j]]

	# i and x would be bound in the last expression so the third expression is safe
	negatedSafe = true :- a = [1,2,3,4], b = [1,2,3,4], not a[i] = x, b[i] = x
	`)

	c.setExports()
	c.setGlobals()
	c.resolveAllRefs()
	c.checkSafetyHead()

	c.checkSafetyBody()

	expected := []error{
		fmt.Errorf("unsafe variables in unboundRef1: a"),
		fmt.Errorf("unsafe variables in unboundRef2: a"),
		fmt.Errorf("unsafe variables in unboundNegated1: i, x"),
		fmt.Errorf("unsafe variables in unboundNegated2: i, x"),
		fmt.Errorf("unsafe variables in unboundNegated3: i, j, x"),
		fmt.Errorf("unsafe variables in unboundNegated4: i, j"),
	}

	if !reflect.DeepEqual(expected, c.Errors) {
		t.Errorf("Expected %v but got:%v", expected, c.Errors)
	}

}

func assertExports(t *testing.T, c *Compiler, path string, expected []string) {

	p := MustParseRef(path)
	v, ok := c.Exports.Get(p)

	if len(expected) == 0 {
		if ok {
			t.Errorf("Unexpected exports for %v: %v", p, v)
		}
		return
	}

	if !ok {
		t.Errorf("Missing exports for: %v", p)
		return
	}

	// Must copy the vars into a string slice because Go will not
	// allow type conversion.
	r := v.([]Var)
	s := []string{}
	for _, x := range r {
		s = append(s, string(x))
	}

	sort.Strings(s)
	sort.Strings(expected)

	if !reflect.DeepEqual(expected, s) {
		t.Errorf("Wrong exports: expected %v but got: %v", expected, s)
	}
}

func assertGlobals(t *testing.T, c *Compiler, mod *Module, expected string) {

	o := MustParseTerm(expected).Value.(Object)

	e := map[Var]Value{}
	for _, i := range o {
		k := i[0].Value.(Var)
		v := i[1].Value
		e[k] = v
	}

	if len(e) == 0 {
		if c.Globals[mod] != nil {
			t.Errorf("Unexpected globals: %v", c.Globals[mod])
		}
		return
	}

	r := c.Globals[mod]

	// Diff the maps...
	if len(r) != len(e) {
		t.Errorf("Wrong globals: expected %d but got %v", len(e), len(r))
		return
	}

	for ek, ev := range e {
		rv := r[ek]
		if rv == nil {
			t.Errorf("Wrong globals: missing %v:%v", ek, ev)
			continue
		}
		if !r[ek].Equal(ev) {
			t.Errorf("Wrong globals: expected %v:%v but got %v:%v", ek, ev, ek, r[ek])
		}
	}

	for rk, rv := range r {
		ev := e[rk]
		if ev == nil {
			t.Errorf("Wrong globals: unexpected %v:%v", rk, rv)
		}
	}
}

func assertNotFailed(t *testing.T, c *Compiler) {
	if c.Failed() {
		t.Errorf("Unexpected compilation error: %v", c.FlattenErrors())
	}
}

func getCompilerTestModules() map[string]*Module {

	mod1 := MustParseModule(`
	package a.b.c

	import data.x.y.z as foo
	import data.g.h.k

	p[x] :- q[x], not r[x]
	q[x] :- foo[i] = x
	z = 400
	`)

	mod2 := MustParseModule(`
	package a.b.c
	import data.bar
	import data.x.y.p
	r[x] :- bar[x] = 100, p = 101
	`)

	mod3 := MustParseModule(`
	package a.b.d
	import req
	import x as y
	t = true :- req = {y.secret: [y.keyid]}
	x = false :- true
	`)

	mod4 := MustParseModule(`
	package a.b.empty
	`)

	return map[string]*Module{"mod2": mod2, "mod3": mod3, "mod1": mod1, "mod4": mod4}
}
