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

func TestModuleTree(t *testing.T) {

	mods := getCompilerTestModules()
	tree := NewModuleTree(mods)

	if tree.Size() != 4 {
		t.Errorf("Expected size of 4 in module tree but got: %v", tree.Size())
	}

	if r1 := findRules(tree, MustParseRef("data.a.b.c")); len(r1) != 0 {
		t.Errorf("Expected empty result from findRules(data.a.b.c) but got: %v", r1)
		return
	}

	if r2 := findRules(tree, MustParseRef("a[x]")); len(r2) != 0 {
		t.Errorf("Expected empty result from findRules(a[x]) but got: %v", r2)
		return
	}

	r3 := findRules(tree, MustParseRef("data.a.b.c.p"))
	expected3 := []*Rule{mods["mod1"].Rules[0]}

	if !reflect.DeepEqual(r3, expected3) {
		t.Errorf("Expected %v from findRules(data.a.b.c.p) but got: %v", expected3, r3)
		return
	}

	r4 := findRules(tree, MustParseRef("data.a.b.c.p[x]"))
	if !reflect.DeepEqual(r4, expected3) {
		t.Errorf("Expected %v from findRules(data.a.b.c.p[x]) but got: %v", expected3, r4)
		return
	}

	r5 := []string{}
	for _, r := range findRules(tree, MustParseRef("data.a.b[i][j][k]")) {
		r5 = append(r5, string(r.Name))
	}
	sort.Strings(r5)

	expected5 := []string{}
	for _, m := range mods {
		for _, r := range m.Rules {
			expected5 = append(expected5, string(r.Name))
		}
	}

	sort.Strings(expected5)

	if !reflect.DeepEqual(r5, expected5) {
		t.Errorf("Expected %v from findRules(data.a.b.c.p[x]) but got: %v", expected5, r5)
		return
	}
}

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
	c.Modules = map[string]*Module{
		"newMod": MustParseModule(`
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
	`)}

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

func TestCompilerSetRuleGraph(t *testing.T) {
	c := NewCompiler()
	c.Modules = getCompilerTestModules()

	c.setExports()
	c.setGlobals()
	c.resolveAllRefs()
	c.checkSafetyBody()
	c.setModuleTree()
	c.setRuleGraph()

	assertNotFailed(t, c)

	mod1 := c.Modules["mod1"]
	p := mod1.Rules[0]
	q := mod1.Rules[1]
	mod2 := c.Modules["mod2"]
	r := mod2.Rules[0]

	edges := map[*Rule]struct{}{
		q: struct{}{},
		r: struct{}{},
	}

	if !reflect.DeepEqual(edges, c.RuleGraph[p]) {
		t.Errorf("Expected dependencies for p to be q and r but got: %v", c.RuleGraph[p])
	}

}

func TestCompilerCheckRecursion(t *testing.T) {
	c := NewCompiler()
	c.Modules = map[string]*Module{
		"newMod1": MustParseModule(`
						package rec
						s = true :- t
						t = true :- s
						a = true :- b
						b = true :- c
						c = true :- d, e
						d = true :- true
						e = true :- a`),
		"newMod2": MustParseModule(`
						package rec
						x = true :- s
						`),
		"newMod3": MustParseModule(`
						package rec2
						import data.rec.x
						y = true :- x
						`),
		"newMod4": MustParseModule(`
						package rec3
						p[x] = y :- data.rec4[x][y] = z
						`),
		"newMod5": MustParseModule(`
						package rec4
						import data.rec3.p
						q[x] = y :- p[x] = y
						`),
	}
	c.setExports()
	c.setGlobals()
	c.resolveAllRefs()
	c.checkSafetyBody()
	c.setModuleTree()
	c.setRuleGraph()

	c.checkRecursion()

	expected := []error{
		fmt.Errorf("recursion found in s: s, t, s"),
		fmt.Errorf("recursion found in t: t, s, t"),
		fmt.Errorf("recursion found in a: a, b, c, e, a"),
		fmt.Errorf("recursion found in b: b, c, e, a, b"),
		fmt.Errorf("recursion found in c: c, e, a, b, c"),
		fmt.Errorf("recursion found in e: e, a, b, c, e"),
		fmt.Errorf("recursion found in p: p, q, p"),
		fmt.Errorf("recursion found in q: q, p, q"),
	}

	if len(c.Errors) != len(expected) {
		t.Errorf("Expected exactly %v errors but got: %v", len(expected), c.Errors)
		return
	}

	for _, x := range c.Errors {
		found := false
		for i, y := range expected {
			if reflect.DeepEqual(x, y) {
				found = true
				expected = append(expected[:i], expected[i+1:]...)
				break
			}
		}
		if !found {
			t.Errorf("Unexpected error in recursion check: %v", x)
		}
	}

	if len(expected) > 0 {
		t.Errorf("Missing errors in recursion check: %v", expected)
	}
}

func TestCompilerCheckBuiltinOperators(t *testing.T) {
	c := NewCompiler()
	c.Modules = map[string]*Module{
		"newMod": MustParseModule(
			`
			package a.b.c
			p = true :- deadbeef(1,2,3)
			`,
		),
	}

	c.checkBuiltinOperators()

	expected := []error{
		fmt.Errorf("bad built-in operator in p: deadbeef"),
	}

	if !reflect.DeepEqual(c.Errors, expected) {
		t.Errorf("Expected exactly one error with bad built-in operator but got: %v", c.Errors)
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
