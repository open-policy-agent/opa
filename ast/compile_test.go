// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestModuleTree(t *testing.T) {

	mods := getCompilerTestModules()
	tree := NewModuleTree(mods)
	expectedSize := 6

	if tree.Size() != expectedSize {
		t.Errorf("Expected size of %v in module tree but got: %v", expectedSize, tree.Size())
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
	m := MustParseModule(testModule)
	c.Compile(map[string]*Module{"testMod": m})
	assertNotFailed(t, c)
}

func TestCompilerSetExports(t *testing.T) {
	c := NewCompiler()
	c.Modules = getCompilerTestModules()
	compileStages(c, "", "setExports")

	assertNotFailed(t, c)
	assertExports(t, c, "data.a.b.d", []string{"t", "x"})
	assertExports(t, c, "data.a.b.c", []string{"p", "q", "r", "z"})
	assertExports(t, c, "data.a.b.empty", nil)
}

func TestCompilerSetGlobals(t *testing.T) {
	c := NewCompiler()
	c.Modules = getCompilerTestModules()
	compileStages(c, "", "setGlobals")

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

func TestCompilerCheckSafetyHead(t *testing.T) {
	c := NewCompiler()
	c.Modules = getCompilerTestModules()
	c.Modules["newMod"] = MustParseModule(`
	package a.b
	unboundKey[x] = y :- q[y] = {"foo": [1,2,[{"bar": y}]]}
	unboundVal[y] = x :- q[y] = {"foo": [1,2,[{"bar": y}]]}
	unboundCompositeVal[y] = [{"foo": x, "bar": y}] :- q[y] = {"foo": [1,2,[{"bar": y}]]}
	`)
	compileStages(c, "", "checkSafetyHead")

	if len(c.Errors) != 3 {
		t.Errorf("Expected exactly 3 errors but got: %v", c.Errors)
		return
	}
}

func TestCompilerCheckSafetyBodyReordering(t *testing.T) {
	tests := []struct {
		note     string
		body     string
		expected interface{}
	}{
		// trivial cases
		{"noop", "x = 1, x != 0", "x = 1, x != 0"},
		{"var/ref", "a[i] = x, a = [1,2,3,4]", "a = [1,2,3,4], a[i] = x"},
		{"var/ref (nested)", "a = [1,2,3,4], a[b[i]] = x, b = [0,0,0,0]", "a = [1,2,3,4], b = [0,0,0,0], a[b[i]] = x"},
		{"negation",
			"a = [true, false], b = [true, false], not a[i], b[i]",
			"a = [true, false], b = [true, false], b[i], not a[i]"},
		{"built-in", "x != 0, count([1,2,3], x)", "count([1,2,3], x), x != 0"},
		{"var/var 1", "x = y, z = 1, y = z", "z = 1, y = z, x = y"},
		{"var/var 2", "x = y, 1 = z, z = y", "1 = z, z = y, x = y"},
		{"var/var 3", "x != 0, y = x, y = 1", "y = 1, y = x, x != 0"},

		// comprehensions
		{"array compr/var", "x != 0, [y | y = 1] = x", "[y | y = 1] = x, x != 0"},
		{"array compr/array", "[1] != [x], [y | y = 1] = [x]", "[y | y = 1] = [x], [1] != [x]"},
	}

	for i, tc := range tests {
		c := NewCompiler()
		c.Modules = map[string]*Module{
			"mod": MustParseModule(
				fmt.Sprintf(`package test
						 p :- %s`, tc.body)),
		}

		compileStages(c, "", "checkSafetyBody")
		switch exp := tc.expected.(type) {
		case string:
			if c.Failed() {
				t.Errorf("%v (#%d): Unexpected compilation error: %v", tc.note, i, c.FlattenErrors())
				return
			}
			e := MustParseBody(exp)
			if !e.Equal(c.Modules["mod"].Rules[0].Body) {
				t.Errorf("%v (#%d): Expected body to be ordered and equal to %v but got: %v", tc.note, i, e, c.Modules["mod"].Rules[0].Body)
			}
		case error:
			if len(c.Errors) > 0 {
				if !reflect.DeepEqual(c.Errors[0], exp) {
					t.Errorf("%v (#%d): Expected compiler error %v but got: %v", tc.note, i, exp, c.Errors[0])
				}
			} else {
				t.Errorf("%v (#%d): Expected compiler error but got: %v", tc.note, i, c.Modules["mod"].Rules[0])
			}
		}
	}
}

func TestCompilerCheckSafetyBodyReorderingClosures(t *testing.T) {
	c := NewCompiler()
	c.Modules = map[string]*Module{
		"mod": MustParseModule(
			`
			package compr

			import data.b
			import data.c

			p :- v     = [null | true],                               # leave untouched
				 xs    = [x | a[i] = x, a = [y | y != 1, y = c[j]]],  # close over 'i' and 'j', 2-level reorder
				 xs[j] > 0,
				 b[i]  = j

			# test that reordering is not performed when closing over different globals, e.g.,
			# built-ins, data, imports.
			q :- _ = [x | x = b[i]],
				 _ = b[j],

				 _ = [x | x = true, x != false],
				 true != false,

				 _ = [x | data.foo[_] = x],
				 data.foo[_] = _
			`),
	}

	compileStages(c, "", "checkSafetyBody")
	assertNotFailed(t, c)

	result1 := c.Modules["mod"].Rules[0].Body
	expected1 := MustParseBody(`
		v     = [null | true],
		b[i]  = j,
		xs    = [x | a = [y | y = c[j], y != 1], a[i] = x],
		xs[j] > 0
	`)
	if !result1.Equal(expected1) {
		t.Errorf("Expected reordered body to be equal to:\n%v\nBut got:\n%v", expected1, result1)
	}

	result2 := c.Modules["mod"].Rules[1].Body
	expected2 := MustParseBody(`
		_ = [x | x = b[i]],
		_ = b[j],
		_ = [x | x = true, x != false],
		true != false,
		_ = [x | data.foo[_] = x],
		data.foo[_] = _
	`)
	if !result2.Equal(expected2) {
		t.Errorf("Expected pre-ordered body to equal:\n%v\nBut got:\n%v", expected2, result2)
	}
}

func TestCompilerCheckSafetyBodyErrors(t *testing.T) {
	c := NewCompiler()

	c.Modules = getCompilerTestModules()
	c.Modules = map[string]*Module{
		"newMod": MustParseModule(`
	package a.b

	import a.b.c as foo
	import x as bar
	import data.m.n as baz

	# deadbeef is not a built-interface{}
	badBuiltin = true :- deadbeef(1,2,3)

	# a would be unbound
	unboundRef1 = true :- a.b.c = "foo"

	# a would be unbound
	unboundRef2 = true :- {"foo": [{"bar": a.b.c}]} = {"foo": [{"bar": "baz"}]}

	# i will be bound even though it's in a non-output position
	inputPosRef = true :- a = [1,2,3,4], a[i] != 100

	# i and x would be unbound
	unboundNegated1 = true :- a = [1,2,3,4], not a[i] = x

	# i and x would be unbound even though x appears in head
	unboundNegated2[x] :- a = [1,2,3,4], not a[i] = x

	# x, i, and j would be unbound even though they appear in other expressions
	unboundNegated3[x] = true :- a = [1,2,3,4], b = [1,2,3,4], not a[i] = x, not b[j] = x

	# i and j would be unbound even though they are in embedded references
	unboundNegated4 = true :- a = [{"foo": ["bar", "baz"]}], not a[0].foo = [a[0].foo[i], a[0].foo[j]]

	# x would be unbound as input to count
	unsafeBuiltin :- count([1,2,x], x)

	# i and x would be bound in the last expression so the third expression is safe
	negatedSafe = true :- a = [1,2,3,4], b = [1,2,3,4], not a[i] = x, b[i] = x

	# x would be unbound because it does not appear in the target position of any expression
	unboundNoTarget = true :- x > 0, x <= 3, x != 2

	unboundArrayComprBody1 :- _ = [x | x = data.a[_], y > 1]
	unboundArrayComprBody2 :- _ = [x | x = a[_], a = [y | y = data.a[_], z > 1]]
	unboundArrayComprBody3 :- _ = [v | v = [x | x = data.a[_]], x > 1]
	unboundArrayComprTerm1 :- _ = [u | true]
	unboundArrayComprTerm2 :- _ = [v | v = [w | w != 0]]
	unboundArrayComprTerm3 :- _ = [x[i] | x = []]
	unboundArrayComprMixed1 :- _ = [x | y = [a | a = z[i]]]

	unsafeClosure1 :- x = [x | x = 1]
	unsafeClosure2 :- x = y, x = [y | y = 1]

	unsafeNestedHead :- count(baz[i].attr[bar[dead.beef]], n)

	negatedImport1 = true :- not foo
	negatedImport2 = true :- not bar
	negatedImport3 = true :- not baz
	`)}
	compileStages(c, "", "checkSafetyBody")

	expected := []error{
		fmt.Errorf("unsafe variables in badBuiltin: [deadbeef]"),
		fmt.Errorf("unsafe variables in unboundRef1: [a]"),
		fmt.Errorf("unsafe variables in unboundRef2: [a]"),
		fmt.Errorf("unsafe variables in unboundNegated1: [i x]"),
		fmt.Errorf("unsafe variables in unboundNegated2: [i x]"),
		fmt.Errorf("unsafe variables in unboundNegated3: [i j x]"),
		fmt.Errorf("unsafe variables in unboundNegated4: [i j]"),
		fmt.Errorf("unsafe variables in unsafeBuiltin: [x]"),
		fmt.Errorf("unsafe variables in unboundNoTarget: [x]"),
		fmt.Errorf("unsafe variables in unboundArrayComprBody1: [y]"),
		fmt.Errorf("unsafe variables in unboundArrayComprBody2: [z]"),
		fmt.Errorf("unsafe variables in unboundArrayComprBody3: [x]"),
		fmt.Errorf("unsafe variables in unboundArrayComprTerm1: [u]"),
		fmt.Errorf("unsafe variables in unboundArrayComprTerm2: [w]"),
		fmt.Errorf("unsafe variables in unboundArrayComprTerm3: [i]"),
		fmt.Errorf("unsafe variables in unboundArrayComprMixed1: [x z]"),
		fmt.Errorf("unsafe variables in unsafeClosure1: [x]"),
		fmt.Errorf("unsafe variables in unsafeClosure2: [y]"),
		fmt.Errorf("unsafe variables in unsafeNestedHead: [dead]"),
	}

	if !reflect.DeepEqual(expected, c.Errors) {
		e := []string{}
		for _, x := range expected {
			e = append(e, x.Error())
		}
		r := []string{}
		for _, x := range c.Errors {
			r = append(r, x.Error())
		}
		t.Errorf("Expected:\n%v\nBut got:\n%v", strings.Join(e, "\n"), strings.Join(r, "\n"))
	}

}

func TestCompilerResolveAllRefs(t *testing.T) {
	c := NewCompiler()
	c.Modules = getCompilerTestModules()
	compileStages(c, "", "resolveAllRefs")

	assertNotFailed(t, c)

	// Basic test cases.
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

	// Array comprehensions.
	mod5 := c.Modules["mod5"]

	ac := func(r *Rule) *ArrayComprehension {
		return r.Body[0].Terms.(*Term).Value.(*ArrayComprehension)
	}

	acTerm1 := ac(mod5.Rules[0])
	assertTermEqual(t, acTerm1.Term, MustParseTerm("x.a"))
	acTerm2 := ac(mod5.Rules[1])
	assertTermEqual(t, acTerm2.Term, MustParseTerm("a.b.c.q.a"))
	acTerm3 := ac(mod5.Rules[2])
	assertTermEqual(t, acTerm3.Body[0].Terms.([]*Term)[1], MustParseTerm("x.a"))
	acTerm4 := ac(mod5.Rules[3])
	assertTermEqual(t, acTerm4.Body[0].Terms.([]*Term)[1], MustParseTerm("a.b.c.q[i]"))
	acTerm5 := ac(mod5.Rules[4])
	assertTermEqual(t, acTerm5.Body[0].Terms.([]*Term)[2].Value.(*ArrayComprehension).Term, MustParseTerm("x.a"))
	acTerm6 := ac(mod5.Rules[5])
	assertTermEqual(t, acTerm6.Body[0].Terms.([]*Term)[2].Value.(*ArrayComprehension).Body[0].Terms.([]*Term)[1], MustParseTerm("a.b.c.q[i]"))

	// Nested references.
	mod6 := c.Modules["mod6"]
	nested1 := mod6.Rules[0].Body[0].Terms.(*Term)
	assertTermEqual(t, nested1, MustParseTerm("data.x[x[i].a[data.z.b[j]]]"))

	nested2 := mod6.Rules[1].Body[1].Terms.(*Term)
	assertTermEqual(t, nested2, MustParseTerm("v[x[i]]"))

	nested3 := mod6.Rules[3].Body[0].Terms.(*Term)
	assertTermEqual(t, nested3, MustParseTerm("data.x[data.a.b.nested.r]"))

}

func TestCompilerSetRuleGraph(t *testing.T) {
	c := NewCompiler()
	c.Modules = getCompilerTestModules()
	compileStages(c, "", "setRuleGraph")

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
		"newMod6": MustParseModule(`
						package rec5
						acp[x] :- acq[x]
						acq[x] :- a = [x | acp[x]], a[i] = x
						`),
		"newMod7": MustParseModule(`
						package rec6
						np[x] = y :- data.a[data.b.c[nq[x]]] = y
						nq[x] = y :- data.d[data.e[x].f[np[y]]]
						`),
	}

	compileStages(c, "", "checkRecursion")

	expected := []error{
		fmt.Errorf("recursion found in s: s, t, s"),
		fmt.Errorf("recursion found in t: t, s, t"),
		fmt.Errorf("recursion found in a: a, b, c, e, a"),
		fmt.Errorf("recursion found in b: b, c, e, a, b"),
		fmt.Errorf("recursion found in c: c, e, a, b, c"),
		fmt.Errorf("recursion found in e: e, a, b, c, e"),
		fmt.Errorf("recursion found in p: p, q, p"),
		fmt.Errorf("recursion found in q: q, p, q"),
		fmt.Errorf("recursion found in acq: acq, acp, acq"),
		fmt.Errorf("recursion found in acp: acp, acq, acp"),
		fmt.Errorf("recursion found in np: np, nq, np"),
		fmt.Errorf("recursion found in nq: nq, np, nq"),
	}

	if len(c.Errors) != len(expected) {
		t.Errorf("Expected exactly %v errors but got %v: %v", len(expected), len(c.Errors), c.Errors)
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

func TestRecompile(t *testing.T) {
	c := NewCompiler()

	mod := MustParseModule(`
	package abc
	import xyz as foo
	p = true :- foo.bar = true`)

	c.Compile(map[string]*Module{"": mod})
	assertNotFailed(t, c)
	c.Compile(c.Modules)
	assertNotFailed(t, c)
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

func compileStages(c *Compiler, from string, to string) {
	start := 0
	end := len(c.stages) - 1
	for i, s := range c.stages {
		if s.name == from {
			start = i
			break
		}
	}
	for i, s := range c.stages {
		if s.name == to {
			end = i
			break
		}
	}
	for i := start; i <= end; i++ {
		s := c.stages[i]
		if s.f(); c.Failed() {
			return
		}
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

	mod5 := MustParseModule(`
	package a.b.compr

	import x as y
	import a.b.c.q

	p :- [y.a | true]
	r :- [q.a | true]
	s :- [true | y.a = 0]
	t :- [true | q[i] = 1]
	u :- [true | _ = [y.a | true]]
	v :- [true | _ = [ true | q[i] = 1]]
	`)

	mod6 := MustParseModule(`
	package a.b.nested

	import data.x
	import x as y
	import data.z

	p :- x[y[i].a[z.b[j]]]
	q :- x = v, v[y[i]]
	r = 1 :- true
	s :- x[r]
	`)

	return map[string]*Module{
		"mod1": mod1,
		"mod2": mod2,
		"mod3": mod3,
		"mod4": mod4,
		"mod5": mod5,
		"mod6": mod6,
	}
}
