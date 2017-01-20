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

	"github.com/open-policy-agent/opa/util/test"
)

func TestModuleTree(t *testing.T) {

	mods := getCompilerTestModules()
	tree := NewModuleTree(mods)
	expectedSize := 6

	if tree.Size() != expectedSize {
		t.Fatalf("Expected size of %v in module tree but got: %v", expectedSize, tree.Size())
	}

	r1 := findRules(tree, MustParseRef("data.a.b.c"))
	expected1 := []*Rule{}
	expected1 = append(expected1, mods["mod1"].Rules...)
	expected1 = append(expected1, mods["mod2"].Rules...)
	sort.Sort(ruleSlice(r1))
	sort.Sort(ruleSlice(expected1))

	if !reflect.DeepEqual(r1, expected1) {
		t.Fatalf("Expected %v result from findRules(data.a.b.c) but got: %v", expected1, r1)
	}

	r2 := findRules(tree, MustParseRef("a[x]"))
	var expected2 []*Rule

	if !reflect.DeepEqual(r2, expected2) {
		t.Fatalf("Expected %v result from findRules(a[x]) but got: %v", expected2, r2)
	}

	r3 := findRules(tree, MustParseRef("data.a.b.c.p"))
	expected3 := []*Rule{mods["mod1"].Rules[0]}

	if !reflect.DeepEqual(r3, expected3) {
		t.Fatalf("Expected %v from findRules(data.a.b.c.p) but got: %v", expected3, r3)
	}

	r4 := findRules(tree, MustParseRef("data.a.b.c.p[x]"))
	if !reflect.DeepEqual(r4, expected3) {
		t.Fatalf("Expected %v from findRules(data.a.b.c.p[x]) but got: %v", expected3, r4)
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
		t.Fatalf("Expected %v from findRules(data.a.b[i][j][k]) but got: %v", expected5, r5)
	}

	// This ref refers to all rules (same as above but without vars)
	r6 := []string{}
	for _, r := range findRules(tree, MustParseRef("data.a.b")) {
		r6 = append(r6, string(r.Name))
	}
	sort.Strings(r6)

	expected6 := expected5

	if !reflect.DeepEqual(r6, expected6) {
		t.Fatalf("Expected %v from findRules(data.a.b) but got: %v", expected6, r6)
	}

	// This ref refers to all rules (same as above but with var in last position)
	r7 := []string{}
	for _, r := range findRules(tree, MustParseRef("data.a.b[x]")) {
		r7 = append(r7, string(r.Name))
	}
	sort.Strings(r7)

	expected7 := expected6

	if !reflect.DeepEqual(r7, expected7) {
		t.Fatalf("Expected %v from findRules(data.a.b[x]) but got: %v", expected7, r7)
	}

}

func TestRuleTree(t *testing.T) {

	mods := getCompilerTestModules()
	mods["mod-incr"] = MustParseModule(`
	package a.b.c
	s[1] :- true
	s[2] :- true
	`)
	tree := NewRuleTree(mods)
	expectedNumRules := 18

	if tree.Size() != expectedNumRules {
		t.Errorf("Expected %v but got %v rules", expectedNumRules, tree.Size())
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

func TestCompilerCheckSafetyHead(t *testing.T) {
	c := NewCompiler()
	c.Modules = getCompilerTestModules()
	c.Modules["newMod"] = MustParseModule(`
	package a.b
	unboundKey[x] = y :- q[y] = {"foo": [1,2,[{"bar": y}]]}
	unboundVal[y] = x :- q[y] = {"foo": [1,2,[{"bar": y}]]}
	unboundCompositeVal[y] = [{"foo": x, "bar": y}] :- q[y] = {"foo": [1,2,[{"bar": y}]]}
	unboundCompositeKey[[{"x": x}]] :- q[y]
	unboundBuiltinOperator = eq :- x = 1
	`)
	compileStages(c, "", "checkSafetyHead")

	makeErrMsg := func(rule, v string) string {
		return fmt.Sprintf("%s: %s is unsafe (variable %s must appear in at least one expression within the body of %s)", rule, v, v, rule)
	}

	expected := []string{
		makeErrMsg("unboundCompositeKey", "x"),
		makeErrMsg("unboundCompositeVal", "x"),
		makeErrMsg("unboundKey", "x"),
		makeErrMsg("unboundVal", "x"),
		makeErrMsg("unboundBuiltinOperator", "eq"),
	}

	result := compilerErrsToStringSlice(c.Errors)
	sort.Strings(expected)

	if len(result) != len(expected) {
		t.Fatalf("Expected %d:\n%v\nBut got %d:\n%v", len(expected), strings.Join(expected, "\n"), len(result), strings.Join(result, "\n"))
	}

	for i := range result {
		if expected[i] != result[i] {
			t.Errorf("Expected %v but got: %v", expected[i], result[i])
		}
	}

}

func TestCompilerCheckSafetyBodyReordering(t *testing.T) {
	tests := []struct {
		note     string
		body     string
		expected string
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

		if c.Failed() {
			t.Errorf("%v (#%d): Unexpected compilation error: %v", tc.note, i, c.Errors)
			return
		}
		e := MustParseBody(tc.expected)
		if !e.Equal(c.Modules["mod"].Rules[0].Body) {
			t.Errorf("%v (#%d): Expected body to be ordered and equal to %v but got: %v", tc.note, i, e, c.Modules["mod"].Rules[0].Body)
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
		v          = [null | true],
		data.b[i]  = j,
		xs         = [x | a = [y | y = data.c[j], y != 1], a[i] = x],
		xs[j]      > 0
	`)
	if !result1.Equal(expected1) {
		t.Errorf("Expected reordered body to be equal to:\n%v\nBut got:\n%v", expected1, result1)
	}

	result2 := c.Modules["mod"].Rules[1].Body
	expected2 := MustParseBody(`
		_ = [x | x = data.b[i]],
		_ = data.b[j],
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

	import input.aref.b.c as foo
	import input.avar as bar
	import data.m.n as baz

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
	unsafeBuiltinOperator :- count(eq, 1)

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
	unboundBuiltinOperatorArrayCompr :- 1 = 1, [true | eq != 2]

	unsafeClosure1 :- x = [x | x = 1]
	unsafeClosure2 :- x = y, x = [y | y = 1]

	unsafeNestedHead :- count(baz[i].attr[bar[dead.beef]], n)

	negatedImport1 = true :- not foo
	negatedImport2 = true :- not bar
	negatedImport3 = true :- not baz

	rewriteUnsafe[{"foo": dead[i]}] :- true  # dead is not imported
	`)}
	compileStages(c, "", "checkSafetyBody")

	makeErrMsg := func(rule string, varName string) string {
		return fmt.Sprintf("%v: %v is unsafe (variable %v must appear in the output position of at least one non-negated expression)", rule, varName, varName)
	}

	expected := []string{
		makeErrMsg("unboundRef1", "a"),
		makeErrMsg("unboundRef2", "a"),
		makeErrMsg("unboundNegated1", "i"),
		makeErrMsg("unboundNegated1", "x"),
		makeErrMsg("unboundNegated2", "i"),
		makeErrMsg("unboundNegated2", "x"),
		makeErrMsg("unboundNegated3", "i"),
		makeErrMsg("unboundNegated3", "j"),
		makeErrMsg("unboundNegated3", "x"),
		makeErrMsg("unboundNegated4", "i"),
		makeErrMsg("unboundNegated4", "j"),
		makeErrMsg("unsafeBuiltin", "x"),
		makeErrMsg("unsafeBuiltinOperator", "eq"),
		makeErrMsg("unboundNoTarget", "x"),
		makeErrMsg("unboundArrayComprBody1", "y"),
		makeErrMsg("unboundArrayComprBody2", "z"),
		makeErrMsg("unboundArrayComprBody3", "x"),
		makeErrMsg("unboundArrayComprTerm1", "u"),
		makeErrMsg("unboundArrayComprTerm2", "w"),
		makeErrMsg("unboundArrayComprTerm3", "i"),
		makeErrMsg("unboundArrayComprMixed1", "x"),
		makeErrMsg("unboundArrayComprMixed1", "z"),
		makeErrMsg("unboundBuiltinOperatorArrayCompr", "eq"),
		makeErrMsg("unsafeClosure1", "x"),
		makeErrMsg("unsafeClosure2", "y"),
		makeErrMsg("unsafeNestedHead", "dead"),
		makeErrMsg("rewriteUnsafe", "dead"),
	}

	result := compilerErrsToStringSlice(c.Errors)
	sort.Strings(expected)

	if len(result) != len(expected) {
		t.Fatalf("Expected %d:\n%v\nBut got %d:\n%v", len(expected), strings.Join(expected, "\n"), len(result), strings.Join(result, "\n"))
	}

	for i := range result {
		if expected[i] != result[i] {
			t.Errorf("Expected %v but got: %v", expected[i], result[i])
		}
	}

}

func TestCompilerCheckBuiltins(t *testing.T) {
	c := NewCompiler()
	c.Modules = map[string]*Module{
		"mod": MustParseModule(`
			package badbuiltin
			p :- count(1)
			q :- count([1,2,3], x, 1)
			r :- [ x | deadbeef(1,2,x) ]
			`),
	}
	compileStages(c, "", "checkBuiltins")

	expected := []string{
		"p: wrong number of arguments (expression count(1) must specify 2 arguments to built-in function count)",
		"q: wrong number of arguments (expression count([1,2,3], x, 1) must specify 2 arguments to built-in function count)",
		"r: unknown built-in function deadbeef",
	}

	assertCompilerErrorStrings(t, c, expected)
}

func TestCompilerCheckRuleConflicts(t *testing.T) {

	c := getCompilerWithParsedModules(map[string]string{
		"mod1.rego": `
			package badrules
			p[x] :- x = 1
			p[x] = y :- x = y, x = "a"
			q[1] :- true
			q = {1,2,3} :- true
			r[x] = y :- x = y, x = "a"
			r[x] = y :- x = y, x = "a"
		`,
		"mod2.rego": `
			package badrules.r
			q[1] :- true
		`,
	})

	compileStages(c, "", "checkRuleConflicts")

	expected := []string{
		"p: conflicting rule types (all definitions of p must have the same type)",
		"package badrules.r: package declaration conflicts with rule defined at mod1.rego:7",
		"package badrules.r: package declaration conflicts with rule defined at mod1.rego:8",
		"q: conflicting rule types (all definitions of q must have the same type)",
	}

	assertCompilerErrorStrings(t, c, expected)
}

func TestCompilerImportsResolved(t *testing.T) {

	modules := map[string]*Module{
		"mod1": MustParseModule(`
			package ex

			import data
			import input
			import data.foo
			import input.bar
			import data.abc as baz
			import input.abc as qux
		`),
	}

	c := NewCompiler()
	c.Compile(modules)

	assertNotFailed(t, c)

	if len(c.Modules["mod1"].Imports) != 0 {
		t.Fatalf("Expected imports to be empty after compile but got: %v", c.Modules)
	}

}

func TestCompilerResolveAllRefs(t *testing.T) {
	c := NewCompiler()
	c.Modules = getCompilerTestModules()
	c.Modules["head"] = MustParseModule(`package head
	import data.doc1 as bar
	import input.x.y.foo
	import input.qux as baz
	p[foo[bar[i]]] = {"baz": baz} :- true
	`)
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
	e = MustParseTerm("{input.x.secret: [{input.x.keyid}]}")
	if !term.Equal(e) {
		t.Errorf("Wrong term (nested refs): expected %v but got: %v", e, term)
	}

	// Array comprehensions.
	mod5 := c.Modules["mod5"]

	ac := func(r *Rule) *ArrayComprehension {
		return r.Body[0].Terms.(*Term).Value.(*ArrayComprehension)
	}

	acTerm1 := ac(mod5.Rules[0])
	assertTermEqual(t, acTerm1.Term, MustParseTerm("input.x.a"))
	acTerm2 := ac(mod5.Rules[1])
	assertTermEqual(t, acTerm2.Term, MustParseTerm("input.a.b.c.q.a"))
	acTerm3 := ac(mod5.Rules[2])
	assertTermEqual(t, acTerm3.Body[0].Terms.([]*Term)[1], MustParseTerm("input.x.a"))
	acTerm4 := ac(mod5.Rules[3])
	assertTermEqual(t, acTerm4.Body[0].Terms.([]*Term)[1], MustParseTerm("input.a.b.c.q[i]"))
	acTerm5 := ac(mod5.Rules[4])
	assertTermEqual(t, acTerm5.Body[0].Terms.([]*Term)[2].Value.(*ArrayComprehension).Term, MustParseTerm("input.x.a"))
	acTerm6 := ac(mod5.Rules[5])
	assertTermEqual(t, acTerm6.Body[0].Terms.([]*Term)[2].Value.(*ArrayComprehension).Body[0].Terms.([]*Term)[1], MustParseTerm("input.a.b.c.q[i]"))

	// Nested references.
	mod6 := c.Modules["mod6"]
	nested1 := mod6.Rules[0].Body[0].Terms.(*Term)
	assertTermEqual(t, nested1, MustParseTerm("data.x[input.x[i].a[data.z.b[j]]]"))

	nested2 := mod6.Rules[1].Body[1].Terms.(*Term)
	assertTermEqual(t, nested2, MustParseTerm("v[input.x[i]]"))

	nested3 := mod6.Rules[3].Body[0].Terms.(*Term)
	assertTermEqual(t, nested3, MustParseTerm("data.x[data.a.b.nested.r]"))

	// Refs in head.
	mod7 := c.Modules["head"]
	assertTermEqual(t, mod7.Rules[0].Key, MustParseTerm("input.x.y.foo[data.doc1[i]]"))
	assertTermEqual(t, mod7.Rules[0].Value, MustParseTerm(`{"baz": input.qux}`))
}

func TestCompilerRewriteRefsInHead(t *testing.T) {
	c := NewCompiler()
	c.Modules["head"] = MustParseModule(`package head
	import data.doc1 as bar
	import data.doc2 as corge
	import input.x.y.foo
	import input.qux as baz
	p[foo[bar[i]]] = {"baz": baz, "corge": corge} :- true
	`)

	compileStages(c, "", "rewriteRefsInHead")
	assertNotFailed(t, c)

	rule := c.Modules["head"].Rules[0]

	assertTermEqual(t, rule.Key, MustParseTerm("__local0__"))
	assertTermEqual(t, rule.Value, MustParseTerm("__local1__"))

	if len(rule.Body) != 3 {
		t.Fatalf("Expected rule body to contain 3 expressions but got: %v", rule)
	}

	assertExprEqual(t, rule.Body[1], MustParseExpr("__local0__ = input.x.y.foo[data.doc1[i]]"))
	assertExprEqual(t, rule.Body[2], MustParseExpr(`__local1__ = {"baz": input.qux, "corge": data.doc2}`))
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
		"newMod8": MustParseModule(`
						package rec7
						prefix :- data.rec7
						`),
		"newMod9": MustParseModule(`
						package rec8
						dataref :- data
						`),
	}

	compileStages(c, "", "checkRecursion")

	makeErrMsg := func(rule string, loop ...string) string {
		return fmt.Sprintf("%v: recursive reference: %s (recursion is not allowed)", rule, strings.Join(loop, " -> "))
	}

	expected := []string{
		makeErrMsg("s", "s", "t", "s"),
		makeErrMsg("t", "t", "s", "t"),
		makeErrMsg("a", "a", "b", "c", "e", "a"),
		makeErrMsg("b", "b", "c", "e", "a", "b"),
		makeErrMsg("c", "c", "e", "a", "b", "c"),
		makeErrMsg("e", "e", "a", "b", "c", "e"),
		makeErrMsg("p", "p", "q", "p"),
		makeErrMsg("q", "q", "p", "q"),
		makeErrMsg("acq", "acq", "acp", "acq"),
		makeErrMsg("acp", "acp", "acq", "acp"),
		makeErrMsg("np", "np", "nq", "np"),
		makeErrMsg("nq", "nq", "np", "nq"),
		makeErrMsg("prefix", "prefix", "prefix"),
		makeErrMsg("dataref", "dataref", "dataref"),
	}

	result := compilerErrsToStringSlice(c.Errors)
	sort.Strings(expected)

	if len(result) != len(expected) {
		t.Fatalf("Expected %d:\n%v\nBut got %d:\n%v", len(expected), strings.Join(expected, "\n"), len(result), strings.Join(result, "\n"))
	}

	for i := range result {
		if result[i] != expected[i] {
			t.Errorf("Expected %v but got: %v", expected[i], result[i])
		}
	}
}

func TestCompilerGetRulesExact(t *testing.T) {
	mods := getCompilerTestModules()

	// Add incrementally defined rules.
	mods["mod-incr"] = MustParseModule(`
	package a.b.c
	p[1] :- true
	p[2] :- true
	`)

	c := NewCompiler()
	c.Compile(mods)
	assertNotFailed(t, c)

	tests := []struct {
		note     string
		ref      interface{}
		expected []*Rule
	}{
		{"exact", "data.a.b.c.p", []*Rule{
			c.Modules["mod-incr"].Rules[0],
			c.Modules["mod-incr"].Rules[1],
			c.Modules["mod1"].Rules[0],
		}},
		{"too short", "data.a", []*Rule{}},
		{"too long/not found", "data.a.b.c.p.q", []*Rule{}},
		{"outside data", "req.a.b.c.p", []*Rule{}},
	}

	for _, tc := range tests {
		test.Subtest(t, tc.note, func(t *testing.T) {
			var ref Ref
			switch r := tc.ref.(type) {
			case string:
				ref = MustParseRef(r)
			case Ref:
				ref = r
			}
			rules := c.GetRulesExact(ref)
			if len(rules) != len(tc.expected) {
				t.Fatalf("Expected exactly %v rules but got: %v", len(tc.expected), rules)
			}
			for i := range rules {
				found := false
				for j := range tc.expected {
					if rules[i].Equal(tc.expected[j]) {
						found = true
						break
					}
				}
				if !found {
					t.Fatalf("Expected exactly %v but got: %v", tc.expected, rules)
				}
			}
		})
	}
}

func TestCompilerGetRulesForVirtualDocument(t *testing.T) {
	mods := getCompilerTestModules()

	// Add incrementally defined rules.
	mods["mod-incr"] = MustParseModule(`
	package a.b.c
	p[1] :- true
	p[2] :- true
	`)

	c := NewCompiler()
	c.Compile(mods)
	assertNotFailed(t, c)

	tests := []struct {
		note     string
		ref      interface{}
		expected []*Rule
	}{
		{"exact", "data.a.b.c.p", []*Rule{
			c.Modules["mod-incr"].Rules[0],
			c.Modules["mod-incr"].Rules[1],
			c.Modules["mod1"].Rules[0],
		}},
		{"deep", "data.a.b.c.p.q", []*Rule{
			c.Modules["mod-incr"].Rules[0],
			c.Modules["mod-incr"].Rules[1],
			c.Modules["mod1"].Rules[0],
		}},
		{"too short", "data.a", []*Rule{}},
		{"non-existent", "data.a.deadbeef", []*Rule{}},
	}

	for _, tc := range tests {
		test.Subtest(t, tc.note, func(t *testing.T) {
			var ref Ref
			switch r := tc.ref.(type) {
			case string:
				ref = MustParseRef(r)
			case Ref:
				ref = r
			}
			rules := c.GetRulesForVirtualDocument(ref)
			if len(rules) != len(tc.expected) {
				t.Fatalf("Expected exactly %v rules but got: %v", len(tc.expected), rules)
			}
			for i := range rules {
				found := false
				for j := range tc.expected {
					if rules[i].Equal(tc.expected[j]) {
						found = true
						break
					}
				}
				if !found {
					t.Fatalf("Expected exactly %v but got: %v", tc.expected, rules)
				}
			}
		})
	}
}

func TestCompilerGetRulesWithPrefix(t *testing.T) {
	mods := getCompilerTestModules()

	// Add incrementally defined rules.
	mods["mod-incr"] = MustParseModule(`
	package a.b.c
	p[1] :- true
	p[2] :- true
	q[3] :- true
	`)

	c := NewCompiler()
	c.Compile(mods)
	assertNotFailed(t, c)

	tests := []struct {
		note     string
		ref      interface{}
		expected []*Rule
	}{
		{"exact", "data.a.b.c.p", []*Rule{
			c.Modules["mod-incr"].Rules[0],
			c.Modules["mod-incr"].Rules[1],
			c.Modules["mod1"].Rules[0],
		}},
		{"too deep", "data.a.b.c.p.q", []*Rule{}},
		{"prefix", "data.a.b.c", []*Rule{
			c.Modules["mod1"].Rules[0],
			c.Modules["mod1"].Rules[1],
			c.Modules["mod1"].Rules[2],
			c.Modules["mod2"].Rules[0],
			c.Modules["mod-incr"].Rules[0],
			c.Modules["mod-incr"].Rules[1],
			c.Modules["mod-incr"].Rules[2],
		}},
		{"non-existent", "data.a.deadbeef", []*Rule{}},
	}

	for _, tc := range tests {
		test.Subtest(t, tc.note, func(t *testing.T) {
			var ref Ref
			switch r := tc.ref.(type) {
			case string:
				ref = MustParseRef(r)
			case Ref:
				ref = r
			}
			rules := c.GetRulesWithPrefix(ref)
			if len(rules) != len(tc.expected) {
				t.Fatalf("Expected exactly %v rules but got: %v", len(tc.expected), rules)
			}
			for i := range rules {
				found := false
				for j := range tc.expected {
					if rules[i].Equal(tc.expected[j]) {
						found = true
						break
					}
				}
				if !found {
					t.Fatalf("Expected exactly %v but got: %v", tc.expected, rules)
				}
			}
		})
	}
}

func TestCompilerLazyLoadingError(t *testing.T) {

	testLoader := func(map[string]*Module) (map[string]*Module, error) {
		return nil, fmt.Errorf("something went horribly wrong")
	}

	compiler := NewCompiler().WithModuleLoader(testLoader)

	compiler.Compile(nil)

	expected := Errors{
		NewError(CompileErr, nil, "something went horribly wrong"),
	}

	if !reflect.DeepEqual(expected, compiler.Errors) {
		t.Fatalf("Expected error %v but got: %v", expected, compiler.Errors)
	}
}

func TestCompilerLazyLoading(t *testing.T) {

	mod1 := MustParseModule(`
			package a.b.c
			import data.x.z1 as z2
			p :- q, r
			q :- z2`)

	mod2 := MustParseModule(`
			package a.b.c
			r = true`)

	mod3 := MustParseModule(`package x
			import data.foo.bar
			import input.input
			z1 :- [ localvar | count(bar.baz.qux, localvar) ]`)

	mod4 := MustParseModule(`
			package foo.bar.baz
			qux = grault :- true`)

	mod5 := MustParseModule(`
			package foo.bar.baz
			import data.d.e.f
			deadbeef = f :- true
			grault = deadbeef :- true`)

	// testLoader will return 4 rounds of parsed modules.
	rounds := []map[string]*Module{
		{"mod1": mod1, "mod2": mod2},
		{"mod3": mod3},
		{"mod4": mod4},
		{"mod5": mod5},
	}

	// For each round, run checks.
	tests := []func(map[string]*Module){
		func(map[string]*Module) {
			// first round, no modules because compiler is invoked with empty
			// collection.
		},
		func(partial map[string]*Module) {
			p := MustParseRule(`p :- data.a.b.c.q, data.a.b.c.r`)
			if !partial["mod1"].Rules[0].Equal(p) {
				t.Errorf("Expected %v but got %v", p, partial["mod1"].Rules[0])
			}
			q := MustParseRule(`q :- data.x.z1`)
			if !partial["mod1"].Rules[1].Equal(q) {
				t.Errorf("Expected %v but got %v", q, partial["mod1"].Rules[0])
			}
		},
		func(partial map[string]*Module) {
			z1 := MustParseRule(`z1 :- [ localvar | count(data.foo.bar.baz.qux, localvar) ]`)
			if !partial["mod3"].Rules[0].Equal(z1) {
				t.Errorf("Expected %v but got %v", z1, partial["mod3"].Rules[0])
			}
		},
		func(partial map[string]*Module) {
			qux := MustParseRule(`qux = grault :- true`)
			if !partial["mod4"].Rules[0].Equal(qux) {
				t.Errorf("Expected %v but got %v", qux, partial["mod4"].Rules[0])
			}
		},
		func(partial map[string]*Module) {
			grault := MustParseRule("qux = data.foo.bar.baz.grault :- true") // rewrite has not happened yet
			f := MustParseRule("deadbeef = data.d.e.f :- true")
			if !partial["mod4"].Rules[0].Equal(grault) {
				t.Errorf("Expected %v but got %v", grault, partial["mod4"].Rules[0])
			}
			if !partial["mod5"].Rules[0].Equal(f) {
				t.Errorf("Expected %v but got %v", f, partial["mod5"].Rules[0])
			}
		},
	}

	round := 0

	testLoader := func(modules map[string]*Module) (map[string]*Module, error) {
		tests[round](modules)
		if round >= len(rounds) {
			return nil, nil
		}
		result := rounds[round]
		round++
		return result, nil
	}

	compiler := NewCompiler().WithModuleLoader(testLoader)

	if compiler.Compile(nil); compiler.Failed() {
		t.Fatalf("Got unexpected error from compiler: %v", compiler.Errors)
	}
}

func TestQueryCompiler(t *testing.T) {
	tests := []struct {
		note     string
		q        string
		pkg      string
		imports  []string
		expected interface{}
	}{
		{"exports resolved", "z", "package a.b.c", nil, "data.a.b.c.z"},
		{"imports resolved", "z", "package a.b.c.d", []string{"import data.a.b.c.z"}, "data.a.b.c.z"},
		{"unsafe vars", "z", "", nil, fmt.Errorf("1 error occurred: 1:1: z is unsafe (variable z must appear in the output position of at least one non-negated expression)")},
		{"safe vars", "data, abc", "package ex", []string{"import input.xyz as abc"}, "data, input.xyz"},
		{"reorder", "x != 1, x = 0", "", nil, "x = 0, x != 1"},
		{"bad builtin", "deadbeef(1,2,3)", "", nil, fmt.Errorf("1 error occurred: 1:1: unknown built-in function deadbeef")},
	}

	for _, tc := range tests {
		runQueryCompilerTest(t, tc.note, tc.q, tc.pkg, tc.imports, tc.expected)
	}
}

func assertCompilerErrorStrings(t *testing.T, compiler *Compiler, expected []string) {
	result := compilerErrsToStringSlice(compiler.Errors)
	if len(result) != len(expected) {
		t.Fatalf("Expected %d:\n%v\nBut got %d:\n%v", len(expected), strings.Join(expected, "\n"), len(result), strings.Join(result, "\n"))
	}
	for i := range result {
		if expected[i] != result[i] {
			t.Errorf("Expected %v but got: %v", expected[i], result[i])
		}
	}
}

func assertNotFailed(t *testing.T, c *Compiler) {
	if c.Failed() {
		t.Errorf("Unexpected compilation error: %v", c.Errors)
	}
}

func getCompilerWithParsedModules(mods map[string]string) *Compiler {

	parsed := map[string]*Module{}

	for id, input := range mods {
		mod, err := ParseModule(id, input)
		if err != nil {
			panic(err)
		}
		parsed[id] = mod
	}

	compiler := NewCompiler()
	compiler.Modules = parsed

	return compiler
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
	import input.x as y
	t = true :- input = {y.secret: [{y.keyid}]}
	x = false :- true
	`)

	mod4 := MustParseModule(`
	package a.b.empty
	`)

	mod5 := MustParseModule(`
	package a.b.compr

	import input.x as y
	import input.a.b.c.q

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
	import data.z
	import input.x as y

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

func compilerErrsToStringSlice(errors []*Error) []string {
	result := []string{}
	for _, e := range errors {
		msg := strings.SplitN(e.Error(), ":", 3)[2]
		result = append(result, strings.TrimSpace(msg))
	}
	sort.Strings(result)
	return result
}

func runQueryCompilerTest(t *testing.T, note, q, pkg string, imports []string, expected interface{}) {
	test.Subtest(t, note, func(t *testing.T) {
		c := NewCompiler()
		c.Compile(getCompilerTestModules())
		assertNotFailed(t, c)
		qc := c.QueryCompiler()
		query := MustParseBody(q)
		var qctx *QueryContext
		if pkg != "" {
			pkg := MustParsePackage(pkg)
			qctx = NewQueryContext(pkg, nil)
			if len(imports) > 0 {
				qctx.Imports = MustParseImports(strings.Join(imports, "\n"))
			}
		}
		if qctx != nil {
			qc.WithContext(qctx)
		}
		switch expected := expected.(type) {
		case string:
			expectedQuery := MustParseBody(expected)
			result, err := qc.Compile(query)
			if err != nil {
				t.Fatalf("Unexpected error from %v: %v", query, err)
			}
			if !expectedQuery.Equal(result) {
				t.Fatalf("Expected:\n%v\n\nGot:\n%v", expectedQuery, result)
			}
		case error:
			result, err := qc.Compile(query)
			if err == nil {
				t.Fatalf("Expected error from %v but got: %v", query, result)
			}
			if err.Error() != expected.Error() {
				t.Fatalf("Expected error %v but got: %v", expected, err)
			}
		}
	})
}
