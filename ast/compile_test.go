// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/metrics"
	"github.com/open-policy-agent/opa/types"
	"github.com/open-policy-agent/opa/util"
)

func TestOutputVarsForNode(t *testing.T) {

	tests := []struct {
		note      string
		query     string
		arities   map[string]int
		extraSafe string
		exp       string
	}{
		{
			note:  "single var",
			query: "x",
			exp:   "set()",
		},
		{
			note:  "trivial eq",
			query: "x = 1",
			exp:   "{x}",
		},
		{
			note:  "negation",
			query: "not x = 1",
			exp:   "set()",
		},
		{
			note:  "embedded array",
			query: "[x, [1]] = [1, [y]]",
			exp:   "{x, y}",
		},
		{
			note:  "embedded sets",
			query: "{x, [1]} = {1, [y]}",
			exp:   "set()",
		},
		{
			note:  "embedded object values",
			query: `{"foo": x, "bar": {"baz": 1}} = {"foo": 1, "bar": {"baz": y}}`,
			exp:   "{x, y}",
		},
		{
			note:  "object keys are like sets",
			query: `{"foo": x} = {y: 1}`,
			exp:   `set()`,
		},
		{
			note:    "built-ins",
			query:   `count([1,2,3], x)`,
			exp:     "{x}",
			arities: map[string]int{"count": 1},
		},
		{
			note:  "built-ins - input args",
			query: `count(x)`,
			exp:   "set()",
		},
		{
			note:    "functions - no arity",
			query:   `f(1,x)`,
			arities: map[string]int{},
			exp:     "set()",
		},
		{
			note:    "functions",
			query:   `f(1,x)`,
			arities: map[string]int{"f": 1},
			exp:     "{x}",
		},
		{
			note:    "functions - input args",
			query:   `f(1,x)`,
			arities: map[string]int{"f": 2},
			exp:     "set()",
		},
		{
			note:    "functions - embedded refs",
			query:   `f(data.p[x], y)`,
			arities: map[string]int{"f": 1},
			exp:     `{x, y}`,
		},
		{
			note:    "functions - skip ref head",
			query:   `f(x[1])`,
			arities: map[string]int{"f": 1},
			exp:     `set()`,
		},
		{
			note:    "functions - skip sets",
			query:   `f(1, {x})`,
			arities: map[string]int{"f": 1},
			exp:     `set()`,
		},
		{
			note:    "functions - skip object keys",
			query:   `f(1, {x: 1})`,
			arities: map[string]int{"f": 1},
			exp:     `set()`,
		},
		{
			note:    "functions - skip closures",
			query:   `f(1, {x | x = 1})`,
			arities: map[string]int{"f": 1},
			exp:     `set()`,
		},
		{
			note:    "functions - unsafe input",
			query:   `f(x, y)`,
			arities: map[string]int{"f": 1},
			exp:     `set()`,
		},
		{
			note:  "with keyword",
			query: "1 with input as y",
			exp:   "set()",
		},
		{
			note:  "with keyword - unsafe",
			query: "x = 1 with input as y",
			exp:   "set()",
		},
		{
			note:      "with keyword - safe",
			query:     "x = 1 with input as y",
			extraSafe: "{y}",
			exp:       "{x}",
		},
		{
			note:  "ref operand",
			query: "data.p[x]",
			exp:   "{x}",
		},
		{
			note:  "ref operand - unsafe head",
			query: "p[x]",
			exp:   "set()",
		},
		{
			note:  "ref operand - negation",
			query: "not data.p[x]",
			exp:   "set()",
		},
		{
			note:  "ref operand - nested",
			query: "data.p[data.q[x]]",
			exp:   "{x}",
		},
		{
			note:  "comprehension",
			query: "[x | x = 1]",
			exp:   "set()",
		},
		{
			note:  "comprehension containing safe ref",
			query: "[x | data.p[x]]",
			exp:   "set()",
		},
		{
			note:  "accumulate on exprs",
			query: "x = 1; y = x; z = y",
			exp:   "{x, y, z}",
		},
		{
			note:  "composite head",
			query: "{1, 2}[1] = x",
			exp:   `{x}`,
		},
		{
			note:  "composite head",
			query: "x = 1; {x, 2}[1] = y",
			exp:   `{x, y}`,
		},
		{
			note:  "composite head",
			query: "{x, 2}[1] = y",
			exp:   `set()`,
		},
		{
			note:  "nested function calls",
			query: `z = "abc"; x = split(z, "")[y]`,
			exp:   `{x, y, z}`,
		},
		{
			note:  "unsafe nested function calls",
			query: `z = "abc"; x = split(z, a)[y]`,
			exp:   `{z}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {

			body := MustParseBody(tc.query)
			arity := func(r Ref) int {
				a, ok := tc.arities[r.String()]
				if !ok {
					return -1
				}
				return a
			}

			safe := ReservedVars.Copy()

			if tc.extraSafe != "" {
				MustParseTerm(tc.extraSafe).Value.(Set).Foreach(func(x *Term) {
					safe.Add(x.Value.(Var))
				})
			}

			vs := NewSet()

			for v := range outputVarsForBody(body, arity, safe) {
				vs.Add(NewTerm(v))
			}

			exp := MustParseTerm(tc.exp)

			if exp.Value.Compare(vs) != 0 {
				t.Fatalf("Expected %v but got %v", exp, vs)
			}
		})
	}

}

func TestModuleTree(t *testing.T) {

	mods := getCompilerTestModules()
	mods["system-mod"] = MustParseModule(`
	package system.foo

	p = 1
	`)
	mods["non-system-mod"] = MustParseModule(`
	package user.system

	p = 1
	`)
	tree := NewModuleTree(mods)
	expectedSize := 9

	if tree.Size() != expectedSize {
		t.Fatalf("Expected %v but got %v modules", expectedSize, tree.Size())
	}

	if !tree.Children[Var("data")].Children[String("system")].Hide {
		t.Fatalf("Expected system node to be hidden")
	}

	if tree.Children[Var("data")].Children[String("system")].Children[String("foo")].Hide {
		t.Fatalf("Expected system.foo node to be visible")
	}

	if tree.Children[Var("data")].Children[String("user")].Children[String("system")].Hide {
		t.Fatalf("Expected user.system node to be visible")
	}

}

func TestRuleTree(t *testing.T) {

	mods := getCompilerTestModules()
	mods["system-mod"] = MustParseModule(`
	package system.foo

	p = 1
	`)
	mods["non-system-mod"] = MustParseModule(`
	package user.system

	p = 1
	`)
	mods["mod-incr"] = MustParseModule(`package a.b.c

s[1] { true }
s[2] { true }`,
	)

	tree := NewRuleTree(NewModuleTree(mods))
	expectedNumRules := 23

	if tree.Size() != expectedNumRules {
		t.Errorf("Expected %v but got %v rules", expectedNumRules, tree.Size())
	}

	// Check that empty packages are represented as leaves with no rules.
	node := tree.Children[Var("data")].Children[String("a")].Children[String("b")].Children[String("empty")]

	if node == nil || len(node.Children) != 0 || len(node.Values) != 0 {
		t.Fatalf("Unexpected nil value or non-empty leaf of non-leaf node: %v", node)
	}

	system := tree.Child(Var("data")).Child(String("system"))
	if !system.Hide {
		t.Fatalf("Expected system node to be hidden")
	}

	if system.Child(String("foo")).Hide {
		t.Fatalf("Expected system.foo node to be visible")
	}

	user := tree.Child(Var("data")).Child(String("user")).Child(String("system"))
	if user.Hide {
		t.Fatalf("Expected user.system node to be visible")
	}

	if !isVirtual(tree, MustParseRef("data.a.b.empty")) {
		t.Fatal("Expected data.a.b.empty to be virtual")
	}

	abc := tree.Children[Var("data")].Children[String("a")].Children[String("b")].Children[String("c")]
	exp := []Value{String("p"), String("q"), String("r"), String("s"), String("z")}

	if len(abc.Sorted) != len(exp) {
		t.Fatal("expected", exp, "but got", abc)
	}

	for i := range exp {
		if exp[i].Compare(abc.Sorted[i]) != 0 {
			t.Fatal("expected", exp, "but got", abc)
		}
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

func TestCompilerWithStageAfter(t *testing.T) {
	c := NewCompiler().WithStageAfter(
		"CheckRecursion",
		CompilerStageDefinition{"MockStage", "mock_stage", mockStageFunctionCall},
	)
	m := MustParseModule(testModule)
	c.Compile(map[string]*Module{"testMod": m})

	if !c.Failed() {
		t.Errorf("Expected compilation error")
	}
}

func TestCompilerFunctions(t *testing.T) {
	tests := []struct {
		note    string
		modules []string
		wantErr bool
	}{
		{
			note: "multiple input types",
			modules: []string{`package x

				f([x]) = y {
					y = x
				}

				f({"foo": x}) = y {
					y = x
				}`},
		},
		{
			note: "multiple input types",
			modules: []string{`package x

				f([x]) = y {
					y = x
				}

				f([[x]]) = y {
					y = x
				}`},
		},
		{
			note: "constant input",
			modules: []string{`package x

				f(1) = y {
					y = "foo"
				}

				f(2) = y {
					y = "bar"
				}`},
		},
		{
			note: "constant input",
			modules: []string{`package x

				f(1, x) = y {
					y = x
				}

				f(x, y) = z {
					z = x+y
				}`},
		},
		{
			note: "constant input",
			modules: []string{`package x

				f(x, 1) = y {
					y = x
				}

				f(x, [y]) = z {
					z = x+y
				}`},
		},
		{
			note: "multiple input types (nested)",
			modules: []string{`package x

				f({"foo": {"bar": x}}) = y {
					y = x
				}

				f({"foo": [x]}) = y {
					y = x
				}`},
		},
		{
			note: "multiple output types",
			modules: []string{`package x

				f(1) = y {
					y = "foo"
				}

				f(2) = y {
					y = 2
				}`},
		},
		{
			note: "namespacing",
			modules: []string{
				`package x

				f(x) = y {
					data.y.f[x] = y
				}`,
				`package y

				f[x] = y {
					y = "bar"
					x = "foo"
				}`,
			},
		},
		{
			note: "implicit value",
			modules: []string{
				`package x

				f(x) {
					x = "foo"
				}`},
		},
		{
			note: "resolving",
			modules: []string{
				`package x

				f(x) = x { true }`,

				`package y

				import data.x
				import data.x.f as g

				p { g(1, a) }
				p { x.f(1, b) }
				p { data.x.f(1, c) }
				`,
			},
		},
		{
			note: "undefined",
			modules: []string{
				`package x

				p {
					f(1)
				}`,
			},
			wantErr: true,
		},
		{
			note: "must apply",
			modules: []string{
				`package x

				f(1)

				p {
					f
				}
				`,
			},
			wantErr: true,
		},
		{
			note: "must apply",
			modules: []string{
				`package x
				f(1)
				p { f.x }`,
			},
			wantErr: true,
		},
		{
			note: "call argument ref output vars",
			modules: []string{
				`package x

				f(x)

				p { f(data.foo[i]) }`,
			},
			wantErr: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			var err error
			modules := map[string]*Module{}
			for i, module := range tc.modules {
				name := fmt.Sprintf("mod%d", i)
				modules[name], err = ParseModule(name, module)
				if err != nil {
					panic(err)
				}
			}
			c := NewCompiler()
			c.Compile(modules)
			if tc.wantErr && !c.Failed() {
				t.Errorf("Expected compilation error")
			} else if !tc.wantErr && c.Failed() {
				t.Errorf("Unexpected compilation error(s): %v", c.Errors)
			}
		})
	}
}

func TestCompilerErrorLimit(t *testing.T) {
	modules := map[string]*Module{
		"test": MustParseModule(`package test
	r = y { y = true; x = z }

	s[x] = y {
		z = y + x
	}

	t[x] { split(x, y, z) }
	`),
	}

	c := NewCompiler().SetErrorLimit(2)
	c.Compile(modules)

	errs := c.Errors
	exp := []string{
		"2:20: rego_unsafe_var_error: var x is unsafe",
		"2:20: rego_unsafe_var_error: var z is unsafe",
		"rego_compile_error: error limit reached",
	}

	var result []string
	for _, err := range errs {
		result = append(result, err.Error())
	}

	sort.Strings(exp)
	sort.Strings(result)
	if !reflect.DeepEqual(exp, result) {
		t.Errorf("Expected errors %v, got %v", exp, result)
	}
}

func TestCompilerCheckSafetyHead(t *testing.T) {
	c := NewCompiler()
	c.Modules = getCompilerTestModules()
	c.Modules["newMod"] = MustParseModule(`package a.b

unboundKey[x] = y { q[y] = {"foo": [1, 2, [{"bar": y}]]} }
unboundVal[y] = x { q[y] = {"foo": [1, 2, [{"bar": y}]]} }
unboundCompositeVal[y] = [{"foo": x, "bar": y}] { q[y] = {"foo": [1, 2, [{"bar": y}]]} }
unboundCompositeKey[[{"x": x}]] { q[y] }
unboundBuiltinOperator = eq { x = 1 }
unboundElse { false } else = else_var { true }
`,
	)
	compileStages(c, c.checkSafetyRuleHeads)

	makeErrMsg := func(v string) string {
		return fmt.Sprintf("rego_unsafe_var_error: var %v is unsafe", v)
	}

	expected := []string{
		makeErrMsg("x"),
		makeErrMsg("x"),
		makeErrMsg("x"),
		makeErrMsg("x"),
		makeErrMsg("eq"),
		makeErrMsg("else_var"),
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
		{"noop", `x = 1; x != 0`, `x = 1; x != 0`},
		{"var/ref", `a[i] = x; a = [1, 2, 3, 4]`, `a = [1, 2, 3, 4]; a[i] = x`},
		{"var/ref (nested)", `a = [1, 2, 3, 4]; a[b[i]] = x; b = [0, 0, 0, 0]`, `a = [1, 2, 3, 4]; b = [0, 0, 0, 0]; a[b[i]] = x`},
		{"negation",
			`a = [true, false]; b = [true, false]; not a[i]; b[i]`,
			`a = [true, false]; b = [true, false]; b[i]; not a[i]`},
		{"built-in", `x != 0; count([1, 2, 3], x)`, `count([1, 2, 3], x); x != 0`},
		{"var/var 1", `x = y; z = 1; y = z`, `z = 1; y = z; x = y`},
		{"var/var 2", `x = y; 1 = z; z = y`, `1 = z; z = y; x = y`},
		{"var/var 3", `x != 0; y = x; y = 1`, `y = 1; y = x; x != 0`},
		{"array compr/var", `x != 0; [y | y = 1] = x`, `[y | y = 1] = x; x != 0`},
		{"array compr/array", `[1] != [x]; [y | y = 1] = [x]`, `[y | y = 1] = [x]; [1] != [x]`},
		{"with", `data.a.b.d.t with input as x; x = 1`, `x = 1; data.a.b.d.t with input as x`},
		{"with-2", `data.a.b.d.t with input.x as x; x = 1`, `x = 1; data.a.b.d.t with input.x as x`},
		{"with-nop", "data.somedoc[x] with input as true", "data.somedoc[x] with input as true"},
		{"ref-head", `s = [["foo"], ["bar"]]; x = y[0]; y = s[_]; contains(x, "oo")`, `
		s = [["foo"], ["bar"]];
		y = s[_];
		x = y[0];
		contains(x, "oo")
	`},
		{"userfunc", `split(y, ".", z); data.a.b.funcs.fn("...foo.bar..", y)`, `data.a.b.funcs.fn("...foo.bar..", y); split(y, ".", z)`},
	}

	for i, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			c := NewCompiler()
			c.Modules = getCompilerTestModules()
			c.Modules["reordering"] = MustParseModule(fmt.Sprintf(
				`package test
				p { %s }`, tc.body))

			compileStages(c, c.checkSafetyRuleBodies)

			if c.Failed() {
				t.Errorf("%v (#%d): Unexpected compilation error: %v", tc.note, i, c.Errors)
				return
			}

			expected := MustParseBody(tc.expected)
			result := c.Modules["reordering"].Rules[0].Body

			if !expected.Equal(result) {
				t.Errorf("%v (#%d): Expected body to be ordered and equal to %v but got: %v", tc.note, i, expected, result)
			}
		})
	}
}

func TestCompilerCheckSafetyBodyReorderingClosures(t *testing.T) {
	c := NewCompiler()
	c.Modules = map[string]*Module{
		"mod": MustParseModule(
			`package compr

import data.b
import data.c

fn(x) = y {
	trim(x, ".", y)
}

p = true { v = [null | true]; xs = [x | a[i] = x; a = [y | y != 1; y = c[j]]]; xs[j] > 0; z = [true | data.a.b.d.t with input as i2; i2 = i]; b[i] = j }
q = true { _ = [x | x = b[i]]; _ = b[j]; _ = [x | x = true; x != false]; true != false; _ = [x | data.foo[_] = x]; data.foo[_] = _ }
r = true { a = [x | split(y, ".", z); x = z[i]; fn("...foo.bar..", y)] }`,
		),
	}

	compileStages(c, c.checkSafetyRuleBodies)
	assertNotFailed(t, c)

	result1 := c.Modules["mod"].Rules[1].Body
	expected1 := MustParseBody(`v = [null | true]; data.b[i] = j; xs = [x | a = [y | y = data.c[j]; y != 1]; a[i] = x]; z = [true | i2 = i; data.a.b.d.t with input as i2]; xs[j] > 0`)
	if !result1.Equal(expected1) {
		t.Errorf("Expected reordered body to be equal to:\n%v\nBut got:\n%v", expected1, result1)
	}

	result2 := c.Modules["mod"].Rules[2].Body
	expected2 := MustParseBody(`_ = [x | x = data.b[i]]; _ = data.b[j]; _ = [x | x = true; x != false]; true != false; _ = [x | data.foo[_] = x]; data.foo[_] = _`)
	if !result2.Equal(expected2) {
		t.Errorf("Expected pre-ordered body to equal:\n%v\nBut got:\n%v", expected2, result2)
	}

	result3 := c.Modules["mod"].Rules[3].Body
	expected3 := MustParseBody(`a = [x | data.compr.fn("...foo.bar..", y); split(y, ".", z); x = z[i]]`)
	if !result3.Equal(expected3) {
		t.Errorf("Expected pre-ordered body to equal:\n%v\nBut got:\n%v", expected3, result3)
	}
}

func TestCompilerCheckSafetyBodyErrors(t *testing.T) {

	moduleBegin := `
		package a.b

		import input.aref.b.c as foo
		import input.avar as bar
		import data.m.n as baz
	`

	tests := []struct {
		note          string
		moduleContent string
		expected      string
	}{
		{"ref-head", `p { a.b.c = "foo" }`, `{a,}`},
		{"ref-head-2", `p { {"foo": [{"bar": a.b.c}]} = {"foo": [{"bar": "baz"}]} }`, `{a,}`},
		{"negation", `p { a = [1, 2, 3, 4]; not a[i] = x }`, `{i, x}`},
		{"negation-head", `p[x] { a = [1, 2, 3, 4]; not a[i] = x }`, `{i,x}`},
		{"negation-multiple", `p { a = [1, 2, 3, 4]; b = [1, 2, 3, 4]; not a[i] = x; not b[j] = x }`, `{i, x, j}`},
		{"negation-nested", `p { a = [{"foo": ["bar", "baz"]}]; not a[0].foo = [a[0].foo[i], a[0].foo[j]] } `, `{i, j}`},
		{"builtin-input", `p { count([1, 2, x], x) }`, `{x,}`},
		{"builtin-input-name", `p { count(eq, 1) }`, `{eq,}`},
		{"builtin-multiple", `p { x > 0; x <= 3; x != 2 }`, `{x,}`},
		{"unordered-object-keys", `p { x = "a"; [{x: y, z: a}] = [{"a": 1, "b": 2}]}`, `{a,y,z}`},
		{"unordered-sets", `p { x = "a"; [{x, y}] = [{1, 2}]}`, `{y,}`},
		{"array-compr", `p { _ = [x | x = data.a[_]; y > 1] }`, `{y,}`},
		{"array-compr-nested", `p { _ = [x | x = a[_]; a = [y | y = data.a[_]; z > 1]] }`, `{z,}`},
		{"array-compr-closure", `p { _ = [v | v = [x | x = data.a[_]]; x > 1] }`, `{x,}`},
		{"array-compr-term", `p { _ = [u | true] }`, `{u,}`},
		{"array-compr-term-nested", `p { _ = [v | v = [w | w != 0]] }`, `{w,}`},
		{"array-compr-mixed", `p { _ = [x | y = [a | a = z[i]]] }`, `{a, x, z, i}`},
		{"array-compr-builtin", `p { [true | eq != 2] }`, `{eq,}`},
		{"closure-self", `p { x = [x | x = 1] }`, `{x,}`},
		{"closure-transitive", `p { x = y; x = [y | y = 1] }`, `{y,}`},
		{"nested", `p { count(baz[i].attr[bar[dead.beef]], n) }`, `{dead,}`},
		{"negated-import", `p { not foo; not bar; not baz }`, `set()`},
		{"rewritten", `p[{"foo": dead[i]}] { true }`, `{dead, i}`},
		{"with-value", `p { data.a.b.d.t with input as x }`, `{x,}`},
		{"with-value-2", `p { x = data.a.b.d.t with input as x }`, `{x,}`},
		{"else-kw", "p { false } else { count(x, 1) }", `{x,}`},
		{"function", "foo(x) = [y, z] { split(x, y, z) }", `{y,z}`},
		{"call-vars-input", "p { f(x, x) } f(x) = x { true }", `{x,}`},
		{"call-no-output", "p { f(x) } f(x) = x { true }", `{x,}`},
		{"call-too-few", "p { f(1,x) } f(x,y) { true }", "{x,}"},
		{"object-key-comprehension", "p { { {p|x}: 0 } }", "{x,}"},
		{"set-value-comprehension", "p { {1, {p|x}} }", "{x,}"},
	}

	makeErrMsg := func(varName string) string {
		return fmt.Sprintf("rego_unsafe_var_error: var %v is unsafe", varName)
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {

			// Build slice of expected error messages.
			expected := []string{}

			_ = MustParseTerm(tc.expected).Value.(Set).Iter(func(x *Term) error {
				expected = append(expected, makeErrMsg(string(x.Value.(Var))))
				return nil
			}) // cannot return error

			sort.Strings(expected)

			// Compile test module.
			c := NewCompiler()
			c.Modules = map[string]*Module{
				"newMod": MustParseModule(fmt.Sprintf(`

				%v

				%v

				`, moduleBegin, tc.moduleContent)),
			}

			compileStages(c, c.checkSafetyRuleBodies)

			// Get errors.
			result := compilerErrsToStringSlice(c.Errors)

			// Check against expected.
			if len(result) != len(expected) {
				t.Fatalf("Expected %d:\n%v\nBut got %d:\n%v", len(expected), strings.Join(expected, "\n"), len(result), strings.Join(result, "\n"))
			}

			for i := range result {
				if expected[i] != result[i] {
					t.Errorf("Expected %v but got: %v", expected[i], result[i])
				}
			}

		})
	}
}

func TestCompilerCheckSafetyVarLoc(t *testing.T) {

	_, err := CompileModules(map[string]string{"test.rego": `package test

p {
	not x
	x > y
}`})

	if err == nil {
		t.Fatal("expected error")
	}

	errs := err.(Errors)

	if !strings.Contains(errs[0].Message, "var x is unsafe") || errs[0].Location.Row != 4 {
		t.Fatal("expected error on row 4 but got:", err)
	}

	if !strings.Contains(errs[1].Message, "var y is unsafe") || errs[1].Location.Row != 5 {
		t.Fatal("expected y is unsafe on row 5 but got:", err)
	}
}

func TestCompilerCheckTypes(t *testing.T) {
	c := NewCompiler()
	modules := getCompilerTestModules()
	c.Modules = map[string]*Module{"mod6": modules["mod6"], "mod7": modules["mod7"]}
	compileStages(c, c.checkTypes)
	assertNotFailed(t, c)
}

func TestCompilerCheckTypesWithSchema(t *testing.T) {
	c := NewCompiler()
	var schema interface{}
	err := util.Unmarshal([]byte(objectSchema), &schema)
	if err != nil {
		t.Fatal("Unexpected error:", err)
	}
	schemaSet := NewSchemaSet()
	schemaSet.Put(SchemaRootRef, schema)
	c.WithSchemas(schemaSet)
	compileStages(c, c.checkTypes)
	assertNotFailed(t, c)
}

func TestCompilerCheckTypesWithAllOfSchema(t *testing.T) {

	tests := []struct {
		note          string
		schema        string
		expectedError error
	}{
		{
			note:          "allOf with mergeable Object types in schema",
			schema:        allOfObjectSchema,
			expectedError: nil,
		},
		{
			note:          "allOf with mergeable Array types in schema",
			schema:        allOfArraySchema,
			expectedError: nil,
		},
		{
			note:          "allOf without a parent schema",
			schema:        allOfSchemaParentVariation,
			expectedError: nil,
		},
		{
			note:          "allOf with empty schema",
			schema:        emptySchema,
			expectedError: nil,
		},
		{
			note:          "allOf with mergeable Array of Object types in schema",
			schema:        allOfArrayOfObjects,
			expectedError: nil,
		},
		{
			note:          "allOf with mergeable Object types in schema with type declaration missing",
			schema:        allOfObjectMissing,
			expectedError: nil,
		},
		{
			note:          "allOf with Array of mergeable different types in schema",
			schema:        allOfArrayDifTypes,
			expectedError: nil,
		},
		{
			note:          "allOf with mergeable Object containing Array types in schema",
			schema:        allOfArrayInsideObject,
			expectedError: nil,
		},
		{
			note:          "allOf with mergeable Array types in schema with type declaration missing",
			schema:        allOfArrayMissing,
			expectedError: nil,
		},
		{
			note:          "allOf with mergeable types inside of core schema",
			schema:        allOfInsideCoreSchema,
			expectedError: nil,
		},
		{
			note:          "allOf with mergeable String types in schema",
			schema:        allOfStringSchema,
			expectedError: nil,
		},
		{
			note:          "allOf with mergeable Integer types in schema",
			schema:        allOfIntegerSchema,
			expectedError: nil,
		},
		{
			note:          "allOf with mergeable Boolean types in schema",
			schema:        allOfBooleanSchema,
			expectedError: nil,
		},
		{
			note:          "allOf with mergeable Array types with uneven numbers of items",
			schema:        allOfSchemaWithUnevenArray,
			expectedError: nil,
		},
		{
			note:          "allOf schema with unmergeable Array of Arrays",
			schema:        allOfArrayOfArrays,
			expectedError: fmt.Errorf("unable to merge these schemas"),
		},
		{
			note:          "allOf schema with Array and Object types as siblings",
			schema:        allOfObjectAndArray,
			expectedError: fmt.Errorf("unable to merge these schemas"),
		},
		{
			note:          "allOf schema with Array type that contains different unmergeable types",
			schema:        allOfArrayDifTypesWithError,
			expectedError: fmt.Errorf("unable to merge these schemas"),
		},
		{
			note:          "allOf schema with different unmergeable types",
			schema:        allOfTypeErrorSchema,
			expectedError: fmt.Errorf("unable to merge these schemas"),
		},
		{
			note:          "allOf unmergeable schema with different parent and items types",
			schema:        allOfSchemaWithParentError,
			expectedError: fmt.Errorf("unable to merge these schemas"),
		},
		{
			note:          "allOf schema of Array type with uneven numbers of items to merge",
			schema:        allOfSchemaWithUnevenArray,
			expectedError: nil,
		},
		{
			note:          "allOf schema with unmergeable types String and Boolean",
			schema:        allOfStringSchemaWithError,
			expectedError: fmt.Errorf("unable to merge these schemas"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			c := NewCompiler()
			var schema interface{}
			err := util.Unmarshal([]byte(tc.schema), &schema)
			if err != nil {
				t.Fatal("Unexpected error:", err)
			}
			schemaSet := NewSchemaSet()
			schemaSet.Put(SchemaRootRef, schema)
			c.WithSchemas(schemaSet)
			compileStages(c, c.checkTypes)
			if tc.expectedError != nil {
				if errors.Is(c.Errors, tc.expectedError) {
					t.Fatal("Unexpected error:", err)
				}
			} else {
				assertNotFailed(t, c)
			}
		})
	}
}

func TestCompilerCheckRuleConflicts(t *testing.T) {

	c := getCompilerWithParsedModules(map[string]string{
		"mod1.rego": `package badrules

p[x] { x = 1 }
p[x] = y { x = y; x = "a" }
q[1] { true }
q = {1, 2, 3} { true }
r[x] = y { x = y; x = "a" }
r[x] = y { x = y; x = "a" }`,

		"mod2.rego": `package badrules.r

q[1] { true }`,

		"mod3.rego": `package badrules.defkw

default foo = 1
default foo = 2
foo = 3 { true }`,
		"mod4.rego": `package adrules.arity

f(1) { true }
f { true }

g(1) { true }
g(1,2) { true }`,
		"mod5.rego": `package badrules.dataoverlap

p { true }`,
		"mod6.rego": `package badrules.existserr

p { true }`,
		"mod7.rego": `package badrules.redeclaration

p1 := 1
p1 := 2

p2 = 1
p2 := 2`})

	c.WithPathConflictsCheck(func(path []string) (bool, error) {
		if reflect.DeepEqual(path, []string{"badrules", "dataoverlap", "p"}) {
			return true, nil
		} else if reflect.DeepEqual(path, []string{"badrules", "existserr", "p"}) {
			return false, fmt.Errorf("unexpected error")
		}
		return false, nil
	})

	compileStages(c, c.checkRuleConflicts)

	expected := []string{
		"rego_compile_error: conflict check for data path badrules/existserr/p: unexpected error",
		"rego_compile_error: conflicting rule for data path badrules/dataoverlap/p found",
		"rego_type_error: conflicting rules named f found",
		"rego_type_error: conflicting rules named g found",
		"rego_type_error: conflicting rules named p found",
		"rego_type_error: conflicting rules named q found",
		"rego_type_error: multiple default rules named foo found",
		"rego_type_error: package badrules.r conflicts with rule defined at mod1.rego:7",
		"rego_type_error: package badrules.r conflicts with rule defined at mod1.rego:8",
		"rego_type_error: rule named p1 redeclared at mod7.rego:4",
		"rego_type_error: rule named p2 redeclared at mod7.rego:7",
	}

	assertCompilerErrorStrings(t, c, expected)
}

func TestCompilerCheckUndefinedFuncs(t *testing.T) {

	module := `
		package test

		undefined_function {
			data.deadbeef(x)
		}

		undefined_global {
			deadbeef(x)
		}

		undefined_dynamic_dispatch {
			x = "f"; data.test2[x](1)  # not currently supported
		}
	`

	module2 := `
		package test2

		f(x) = x
	`

	_, err := CompileModules(map[string]string{
		"test.rego":  module,
		"test2.rego": module2,
	})
	if err == nil {
		t.Fatal("expected errors")
	}

	result := err.Error()
	want := []string{
		"rego_type_error: undefined function data.deadbeef",
		"rego_type_error: undefined function deadbeef",
		"rego_type_error: undefined function data.test2[x]",
	}

	for _, w := range want {
		if !strings.Contains(result, w) {
			t.Fatalf("Expected %q in result but got: %v", w, result)
		}
	}
}

func TestCompilerImportsResolved(t *testing.T) {

	modules := map[string]*Module{
		"mod1": MustParseModule(`package ex

import data
import input
import data.foo
import input.bar
import data.abc as baz
import input.abc as qux`,
		),
	}

	c := NewCompiler()
	c.Compile(modules)

	assertNotFailed(t, c)

	if len(c.Modules["mod1"].Imports) != 0 {
		t.Fatalf("Expected imports to be empty after compile but got: %v", c.Modules)
	}

}

func TestCompilerExprExpansion(t *testing.T) {

	tests := []struct {
		note     string
		input    string
		expected []*Expr
	}{
		{
			note:  "identity",
			input: "x",
			expected: []*Expr{
				MustParseExpr("x"),
			},
		},
		{
			note:  "single",
			input: "x+y",
			expected: []*Expr{
				MustParseExpr("x+y"),
			},
		},
		{
			note:  "chained",
			input: "x+y+z+w",
			expected: []*Expr{
				MustParseExpr("plus(x, y, __local0__)"),
				MustParseExpr("plus(__local0__, z, __local1__)"),
				MustParseExpr("plus(__local1__, w)"),
			},
		},
		{
			note:  "assoc",
			input: "x+y*z",
			expected: []*Expr{
				MustParseExpr("mul(y, z, __local0__)"),
				MustParseExpr("plus(x, __local0__)"),
			},
		},
		{
			note:  "refs",
			input: "p[q[f(x)]][g(x)]",
			expected: []*Expr{
				MustParseExpr("f(x, __local0__)"),
				MustParseExpr("g(x, __local1__)"),
				MustParseExpr("p[q[__local0__]][__local1__]"),
			},
		},
		{
			note:  "arrays",
			input: "[[f(x)], g(x)]",
			expected: []*Expr{
				MustParseExpr("f(x, __local0__)"),
				MustParseExpr("g(x, __local1__)"),
				MustParseExpr("[[__local0__], __local1__]"),
			},
		},
		{
			note:  "objects",
			input: `{f(x): {g(x): h(x)}}`,
			expected: []*Expr{
				MustParseExpr("f(x, __local0__)"),
				MustParseExpr("g(x, __local1__)"),
				MustParseExpr("h(x, __local2__)"),
				MustParseExpr("{__local0__: {__local1__: __local2__}}"),
			},
		},
		{
			note:  "sets",
			input: `{f(x), {g(x)}}`,
			expected: []*Expr{
				MustParseExpr("f(x, __local0__)"),
				MustParseExpr("g(x, __local1__)"),
				MustParseExpr("{__local0__, {__local1__,}}"),
			},
		},
		{
			note:  "unify",
			input: "f(x) = g(x)",
			expected: []*Expr{
				MustParseExpr("f(x, __local0__)"),
				MustParseExpr("g(x, __local1__)"),
				MustParseExpr("__local0__ = __local1__"),
			},
		},
		{
			note:  "unify: composites",
			input: "[x, f(x)] = [g(y), y]",
			expected: []*Expr{
				MustParseExpr("f(x, __local0__)"),
				MustParseExpr("g(y, __local1__)"),
				MustParseExpr("[x, __local0__] = [__local1__, y]"),
			},
		},
		{
			note:  "with: term expr",
			input: "f[x+1] with input as q",
			expected: []*Expr{
				MustParseExpr("plus(x, 1, __local0__) with input as q"),
				MustParseExpr("f[__local0__] with input as q"),
			},
		},
		{
			note:  "with: call expr",
			input: `f(x) = g(x) with input as p`,
			expected: []*Expr{
				MustParseExpr("f(x, __local0__) with input as p"),
				MustParseExpr("g(x, __local1__) with input as p"),
				MustParseExpr("__local0__ = __local1__ with input as p"),
			},
		},
		{
			note:  "comprehensions",
			input: `f(y) = [[plus(x,1) | x = sum(y[z+1])], g(w)]`,
			expected: []*Expr{
				MustParseExpr("f(y, __local0__)"),
				MustParseExpr("g(w, __local4__)"),
				MustParseExpr("__local0__ = [[__local1__ | plus(z,1,__local2__); sum(y[__local2__], __local3__); eq(x, __local3__); plus(x, 1, __local1__)], __local4__]"),
			},
		},
		{
			note:  "indirect references",
			input: `[1, 2, 3][i]`,
			expected: []*Expr{
				MustParseExpr("__local0__ = [1, 2, 3]"),
				MustParseExpr("__local0__[i]"),
			},
		},
		{
			note:  "multiple indirect references",
			input: `split(split("foo.bar:qux", ".")[_], ":")[i]`,
			expected: []*Expr{
				MustParseExpr(`split("foo.bar:qux", ".", __local0__)`),
				MustParseExpr(`split(__local0__[_], ":", __local1__)`),
				MustParseExpr(`__local1__[i]`),
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			gen := newLocalVarGenerator("", NullTerm())
			expr := MustParseExpr(tc.input)
			result := expandExpr(gen, expr.Copy())
			if len(result) != len(tc.expected) {
				t.Fatalf("Expected %v exprs but got %v:\n\nExpected:\n\n%v\n\nGot:\n\n%v", len(tc.expected), len(result), Body(tc.expected), Body(result))
			}
			for i := range tc.expected {
				if !tc.expected[i].Equal(result[i]) {
					t.Fatalf("Expected expr %d to be %v but got: %v\n\nExpected:\n\n%v\n\nGot:\n\n%v", i, tc.expected[i], result[i], Body(tc.expected), Body(result))
				}
			}
		})
	}
}

func TestCompilerRewriteExprTerms(t *testing.T) {
	cases := []struct {
		note     string
		module   string
		expected interface{}
	}{
		{
			note: "base",
			module: `
				package test

				p { x = a + b * y }

				q[[data.test.f(x)]] { x = 1 }

				r = [data.test.f(x)] { x = 1 }

				f(x) = data.test.g(x)

				pi = 3 + .14

				with_value { 1 with input as f(1) }
			`,
			expected: `
				package test

				p { mul(b, y, __local1__); plus(a, __local1__, __local2__); eq(x, __local2__) }

				q[[__local3__]] { x = 1; data.test.f(x, __local3__) }

				r = [__local4__] { x = 1; data.test.f(x, __local4__) }

				f(__local0__) = __local5__ { true; data.test.g(__local0__, __local5__) }

				pi = __local6__ { true; plus(3, 0.14, __local6__) }

				with_value { data.test.f(1, __local7__); 1 with input as __local7__ }
			`,
		},
		{
			note: "builtin calls in head",
			module: `
				package test

				f(1+1) = 7
			`,
			expected: Errors{&Error{Message: "rule arguments cannot contain calls"}},
		},
		{
			note: "builtin calls in head",
			module: `
				package test

				f(object.get(x)) { object := {"a": 1}; object.a == x }
			`,
			expected: Errors{&Error{Message: "rule arguments cannot contain calls"}},
		},
		{
			note: "indirect ref in args",
			module: `
				package test

				f([1][0]) { true }`,
			expected: `
				package test

				f(__local0__[0]) { true; __local0__ = [1] }`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			compiler := NewCompiler()
			compiler.Modules = map[string]*Module{
				"test": MustParseModule(tc.module),
			}
			compileStages(compiler, compiler.rewriteExprTerms)

			switch exp := tc.expected.(type) {
			case string:
				assertNotFailed(t, compiler)

				expected := MustParseModule(exp)

				if !expected.Equal(compiler.Modules["test"]) {
					t.Fatalf("Expected modules to be equal. Expected:\n\n%v\n\nGot:\n\n%v", expected, compiler.Modules["test"])
				}
			case Errors:
				if len(exp) != len(compiler.Errors) {
					t.Fatalf("Expected %d errors, got %d:\n\n%s\n", len(exp), len(compiler.Errors), compiler.Errors.Error())
				}
				incorrectErrs := false
				for _, e := range exp {
					found := false
					for _, actual := range compiler.Errors {
						if e.Message == actual.Message {
							found = true
							break
						}
					}
					if !found {
						incorrectErrs = true
					}
				}
				if incorrectErrs {
					t.Fatalf("Expected errors:\n\n%s\n\nGot:\n\n%s\n", exp.Error(), compiler.Errors.Error())
				}
			default:
				t.Fatalf("Unsupported value type for test case 'expected' field: %v", exp)
			}

		})
	}
}

func TestCompilerResolveAllRefs(t *testing.T) {
	c := NewCompiler()
	c.Modules = getCompilerTestModules()
	c.Modules["head"] = MustParseModule(`package head

import data.doc1 as bar
import input.x.y.foo
import input.qux as baz

p[foo[bar[i]]] = {"baz": baz} { true }`)

	c.Modules["elsekw"] = MustParseModule(`package elsekw

	import input.x.y.foo
	import data.doc1 as bar
	import input.baz

	p {
		false
	} else = foo {
		bar
	} else = baz {
		true
	}
	`)

	c.Modules["nestedexprs"] = MustParseModule(`package nestedexprs

		x = 1

		p {
			f(g(x))
		}`)

	c.Modules["assign"] = MustParseModule(`package assign

		x = 1
		y = 1

		p {
			x := y
			[true | x := y]
		}`)

	c.Modules["someinassign"] = MustParseModule(`package someinassign
		import future.keywords.in
		x = 1
		y = 1

		p[x] {
			some x in [1, 2, y]
		}`)

	c.Modules["someinassignwithkey"] = MustParseModule(`package someinassignwithkey
		import future.keywords.in
		x = 1
		y = 1

		p[x] {
			some k, v in [1, 2, y]
		}`)

	c.Modules["donotresolve"] = MustParseModule(`package donotresolve

		x = 1

		f(x) {
			x = 2
		}
		`)

	c.Modules["indirectrefs"] = MustParseModule(`package indirectrefs

		f(x) = [x] {true}

		p {
			f(1)[0]
		}
		`)

	c.Modules["comprehensions"] = MustParseModule(`package comprehensions

		nums = [1, 2, 3]

		f(x) = [x] {true}

		p[[1]] {true}

		q {
			p[[x | x = nums[_]]]
		}

		r = [y | y = f(1)[0]]
		`)

	compileStages(c, c.resolveAllRefs)
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
	assertTermEqual(t, acTerm2.Term, MustParseTerm("data.a.b.c.q.a"))
	acTerm3 := ac(mod5.Rules[2])
	assertTermEqual(t, acTerm3.Body[0].Terms.([]*Term)[1], MustParseTerm("input.x.a"))
	acTerm4 := ac(mod5.Rules[3])
	assertTermEqual(t, acTerm4.Body[0].Terms.([]*Term)[1], MustParseTerm("data.a.b.c.q[i]"))
	acTerm5 := ac(mod5.Rules[4])
	assertTermEqual(t, acTerm5.Body[0].Terms.([]*Term)[2].Value.(*ArrayComprehension).Term, MustParseTerm("input.x.a"))
	acTerm6 := ac(mod5.Rules[5])
	assertTermEqual(t, acTerm6.Body[0].Terms.([]*Term)[2].Value.(*ArrayComprehension).Body[0].Terms.([]*Term)[1], MustParseTerm("data.a.b.c.q[i]"))

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
	assertTermEqual(t, mod7.Rules[0].Head.Key, MustParseTerm("input.x.y.foo[data.doc1[i]]"))
	assertTermEqual(t, mod7.Rules[0].Head.Value, MustParseTerm(`{"baz": input.qux}`))

	// Refs in else.
	mod8 := c.Modules["elsekw"]
	assertTermEqual(t, mod8.Rules[0].Else.Head.Value, MustParseTerm("input.x.y.foo"))
	assertTermEqual(t, mod8.Rules[0].Else.Body[0].Terms.(*Term), MustParseTerm("data.doc1"))
	assertTermEqual(t, mod8.Rules[0].Else.Else.Head.Value, MustParseTerm("input.baz"))

	// Refs in calls.
	mod9 := c.Modules["nestedexprs"]
	assertTermEqual(t, mod9.Rules[1].Body[0].Terms.([]*Term)[1], CallTerm(RefTerm(VarTerm("g")), MustParseTerm("data.nestedexprs.x")))

	// Ignore assigned vars.
	mod10 := c.Modules["assign"]
	assertTermEqual(t, mod10.Rules[2].Body[0].Terms.([]*Term)[1], VarTerm("x"))
	assertTermEqual(t, mod10.Rules[2].Body[0].Terms.([]*Term)[2], MustParseTerm("data.assign.y"))
	assignCompr := mod10.Rules[2].Body[1].Terms.(*Term).Value.(*ArrayComprehension)
	assertTermEqual(t, assignCompr.Body[0].Terms.([]*Term)[1], VarTerm("x"))
	assertTermEqual(t, assignCompr.Body[0].Terms.([]*Term)[2], MustParseTerm("data.assign.y"))

	// Args
	mod11 := c.Modules["donotresolve"]
	assertTermEqual(t, mod11.Rules[1].Head.Args[0], VarTerm("x"))
	assertExprEqual(t, mod11.Rules[1].Body[0], MustParseExpr("x = 2"))

	// Locations.
	parsedLoc := getCompilerTestModules()["mod1"].Rules[0].Body[0].Terms.(*Term).Value.(Ref)[0].Location
	compiledLoc := c.Modules["mod1"].Rules[0].Body[0].Terms.(*Term).Value.(Ref)[0].Location
	if parsedLoc.Row != compiledLoc.Row {
		t.Fatalf("Expected parsed location (%v) and compiled location (%v) to be equal", parsedLoc.Row, compiledLoc.Row)
	}

	// Indirect references.
	mod12 := c.Modules["indirectrefs"]
	assertExprEqual(t, mod12.Rules[1].Body[0], MustParseExpr("data.indirectrefs.f(1)[0]"))

	// Comprehensions
	mod13 := c.Modules["comprehensions"]
	assertExprEqual(t, mod13.Rules[3].Body[0].Terms.(*Term).Value.(Ref)[3].Value.(*ArrayComprehension).Body[0], MustParseExpr("x = data.comprehensions.nums[_]"))
	assertExprEqual(t, mod13.Rules[4].Head.Value.Value.(*ArrayComprehension).Body[0], MustParseExpr("y = data.comprehensions.f(1)[0]"))

	// Ignore vars assigned via `some x in xs`.
	mod14 := c.Modules["someinassign"]
	someInAssignCall := mod14.Rules[2].Body[0].Terms.(*SomeDecl).Symbols[0].Value.(Call)
	assertTermEqual(t, someInAssignCall[1], VarTerm("x"))
	collectionLastElem := someInAssignCall[2].Value.(*Array).Get(IntNumberTerm(2))
	assertTermEqual(t, collectionLastElem, MustParseTerm("data.someinassign.y"))

	// Ignore key and val vars assigned via `some k, v in xs`.
	mod15 := c.Modules["someinassignwithkey"]
	someInAssignCall = mod15.Rules[2].Body[0].Terms.(*SomeDecl).Symbols[0].Value.(Call)
	assertTermEqual(t, someInAssignCall[1], VarTerm("k"))
	assertTermEqual(t, someInAssignCall[2], VarTerm("v"))
	collectionLastElem = someInAssignCall[3].Value.(*Array).Get(IntNumberTerm(2))
	assertTermEqual(t, collectionLastElem, MustParseTerm("data.someinassignwithkey.y"))
}

func TestCompilerResolveErrors(t *testing.T) {

	c := NewCompiler()
	c.Modules = map[string]*Module{
		"shadow-globals": MustParseModule(`
			package shadow_globals

			f([input]) { true }
		`),
	}

	compileStages(c, c.resolveAllRefs)

	expected := []string{
		`args must not shadow input`,
	}

	assertCompilerErrorStrings(t, c, expected)
}

func TestCompilerRewriteTermsInHead(t *testing.T) {
	c := NewCompiler()
	c.Modules["head"] = MustParseModule(`package head

import data.doc1 as bar
import data.doc2 as corge
import input.x.y.foo
import input.qux as baz

p[foo[bar[i]]] = {"baz": baz, "corge": corge} { true }
q = [true | true] { true }
r = {"true": true | true} { true }
s = {true | true} { true }

elsekw {
	false
} else = baz {
	true
}
`)

	compileStages(c, c.rewriteRefsInHead)
	assertNotFailed(t, c)

	rule1 := c.Modules["head"].Rules[0]
	expected1 := MustParseRule(`p[__local0__] = __local1__ { true; __local0__ = input.x.y.foo[data.doc1[i]]; __local1__ = {"baz": input.qux, "corge": data.doc2} }`)
	assertRulesEqual(t, rule1, expected1)

	rule2 := c.Modules["head"].Rules[1]
	expected2 := MustParseRule(`q = __local2__ { true; __local2__ = [true | true] }`)
	assertRulesEqual(t, rule2, expected2)

	rule3 := c.Modules["head"].Rules[2]
	expected3 := MustParseRule(`r = __local3__ { true; __local3__ = {"true": true | true} }`)
	assertRulesEqual(t, rule3, expected3)

	rule4 := c.Modules["head"].Rules[3]
	expected4 := MustParseRule(`s = __local4__ { true; __local4__ = {true | true} }`)
	assertRulesEqual(t, rule4, expected4)

	rule5 := c.Modules["head"].Rules[4]
	expected5 := MustParseRule(`elsekw { false } else = __local5__ { true; __local5__ = input.qux }`)
	assertRulesEqual(t, rule5, expected5)
}

func TestCompilerRewriteLocalAssignments(t *testing.T) {

	tests := []struct {
		module          string
		exp             interface{}
		expRewrittenMap map[Var]Var
	}{
		{
			module: `
				package test
				body { a := 1; a > 0 }
			`,
			exp: `
				package test
				body = true { __local0__ = 1; gt(__local0__, 0) }
			`,
			expRewrittenMap: map[Var]Var{
				Var("__local0__"): Var("a"),
			},
		},
		{
			module: `
				package test
				head_vars(a) = b { b := a }
			`,
			exp: `
				package test
				head_vars(__local0__) = __local1__ { __local1__ = __local0__ }
			`,
			expRewrittenMap: map[Var]Var{
				Var("__local0__"): Var("a"),
				Var("__local1__"): Var("b"),
			},
		},
		{
			module: `
				package test
				head_key[a] { a := 1 }
			`,
			exp: `
				package test
				head_key[__local0__] { __local0__ = 1 }
			`,
			expRewrittenMap: map[Var]Var{
				Var("__local0__"): Var("a"),
			},
		},
		{
			module: `
				package test
				head_unsafe_var[a] { some a }
			`,
			exp: `
				package test
				head_unsafe_var[__local0__] { true }
			`,
			expRewrittenMap: map[Var]Var{
				Var("__local0__"): Var("a"),
			},
		},
		{
			module: `
				package test
				p = {1,2,3}
				x = 4
				head_nested[p[x]] {
					some x
				}`,
			exp: `
					package test
					p = {1,2,3}
					x = 4
					head_nested[data.test.p[__local0__]]
			`,
			expRewrittenMap: map[Var]Var{
				Var("__local0__"): Var("x"),
			},
		},
		{
			module: `
				package test
				p = {1,2}
				head_closure_nested[p[x]] {
					y = [true | some x; x = 1]
				}
			`,
			exp: `
				package test
				p = {1,2}
				head_closure_nested[data.test.p[x]] {
					y = [true | __local0__ = 1]
				}
			`,
			expRewrittenMap: map[Var]Var{
				Var("__local0__"): Var("x"),
			},
		},
		{
			module: `
				package test
				nested {
					a := [1,2,3]
					x := [true | a[i] > 1]
				}
			`,
			exp: `
				package test
				nested = true { __local0__ = [1, 2, 3]; __local1__ = [true | gt(__local0__[i], 1)] }
			`,
			expRewrittenMap: map[Var]Var{
				Var("__local0__"): Var("a"),
				Var("__local1__"): Var("x"),
			},
		},
		{
			module: `
				package test
				x = 2
				shadow_globals[x] { x := 1 }
			`,
			exp: `
				package test
				x = 2 { true }
				shadow_globals[__local0__] { __local0__ = 1 }
			`,
			expRewrittenMap: map[Var]Var{
				Var("__local0__"): Var("x"),
			},
		},
		{
			module: `
				package test
				shadow_rule[shadow_rule] { shadow_rule := 1 }
			`,
			exp: `
				package test
				shadow_rule[__local0__] { __local0__ = 1 }
			`,
			expRewrittenMap: map[Var]Var{
				Var("__local0__"): Var("shadow_rule"),
			},
		},
		{
			module: `
				package test
				shadow_roots_1 { data := 1; input := 2; input > data }
			`,
			exp: `
				package test
				shadow_roots_1 = true { __local0__ = 1; __local1__ = 2; gt(__local1__, __local0__) }
			`,
			expRewrittenMap: map[Var]Var{
				Var("__local0__"): Var("data"),
				Var("__local1__"): Var("input"),
			},
		},
		{
			module: `
				package test
				shadow_roots_2 { input := {"a": 1}; input.a > 0  }
			`,
			exp: `
				package test
				shadow_roots_2 = true { __local0__ = {"a": 1}; gt(__local0__.a, 0) }
			`,
			expRewrittenMap: map[Var]Var{
				Var("__local0__"): Var("input"),
			},
		},
		{
			module: `
				package test
				skip_with_target { a := 1; input := 2; data.p with input as a }
			`,
			exp: `
				package test
				skip_with_target = true { __local0__ = 1; __local1__ = 2; data.p with input as __local0__ }
			`,
			expRewrittenMap: map[Var]Var{
				Var("__local0__"): Var("a"),
				Var("__local1__"): Var("input"),
			},
		},
		{
			module: `
				package test
				shadow_comprehensions {
					a := 1
					[true | a := 2; b := 1]
					b := 2
				}
			`,
			exp: `
				package test
				shadow_comprehensions = true { __local0__ = 1; [true | __local1__ = 2; __local2__ = 1]; __local3__ = 2 }
			`,
			expRewrittenMap: map[Var]Var{
				Var("__local0__"): Var("a"),
				Var("__local1__"): Var("a"),
				Var("__local2__"): Var("b"),
				Var("__local3__"): Var("b"),
			},
		},
		{
			module: `
				package test
					scoping {
						[true | a := 1]
						[true | a := 2]
					}
			`,
			exp: `
				package test
				scoping = true { [true | __local0__ = 1]; [true | __local1__ = 2] }
			`,
			expRewrittenMap: map[Var]Var{
				Var("__local0__"): Var("a"),
				Var("__local1__"): Var("a"),
			},
		},
		{
			module: `
				package test
				object_keys {
					{k: v1, "k2": v2} := {"foo": 1, "k2": 2}
				}
			`,
			exp: `
				package test
				object_keys = true { {"k2": __local0__, k: __local1__} = {"foo": 1, "k2": 2} }
			`,
			expRewrittenMap: map[Var]Var{
				Var("__local0__"): Var("v2"),
				Var("__local1__"): Var("v1"),
			},
		},
		{
			module: `
				package test
				head_array_comprehensions = [[x] | x := 1]
					head_set_comprehensions = {[x] | x := 1}
					head_object_comprehensions = {k: [x] | k := "foo"; x := 1}
			`,
			exp: `
				package test
				head_array_comprehensions = [[__local0__] | __local0__ = 1] { true }
				head_set_comprehensions = {[__local1__] | __local1__ = 1} { true }
				head_object_comprehensions = {__local2__: [__local3__] | __local2__ = "foo"; __local3__ = 1} { true }
			`,
			expRewrittenMap: map[Var]Var{
				Var("__local0__"): Var("x"),
				Var("__local1__"): Var("x"),
				Var("__local2__"): Var("k"),
				Var("__local3__"): Var("x"),
			},
		},
		{
			module: `
				package test
				rewritten_object_key {
					k := "foo"
					{k: 1}
				}
			`,
			exp: `
				package test
				rewritten_object_key = true { __local0__ = "foo"; {__local0__: 1} }
			`,
			expRewrittenMap: map[Var]Var{
				Var("__local0__"): Var("k"),
			},
		},
		{
			module: `
				package test
				rewritten_object_key_head[[{k: 1}]] {
					k := "foo"
				}
			`,
			exp: `
				package test
				rewritten_object_key_head[[{__local0__: 1}]] { __local0__ = "foo" }
			`,
			expRewrittenMap: map[Var]Var{
				Var("__local0__"): Var("k"),
			},
		},
		{
			module: `
				package test
				rewritten_object_key_head_value = [{k: 1}] {
					k := "foo"
				}
			`,
			exp: `
				package test
				rewritten_object_key_head_value = [{__local0__: 1}] { __local0__ = "foo" }
			`,
			expRewrittenMap: map[Var]Var{
				Var("__local0__"): Var("k"),
			},
		},
		{
			module: `
				package test
				skip_with_target_in_assignment {
					input := 1
					a := [true | true with input as 2; true with input as 3]
				}
			`,
			exp: `
				package test
				skip_with_target_in_assignment = true { __local0__ = 1; __local1__ = [true | true with input as 2; true with input as 3] }
			`,
			expRewrittenMap: map[Var]Var{
				Var("__local0__"): Var("input"),
				Var("__local1__"): Var("a"),
			},
		},
		{
			module: `
				package test
				rewrite_value_in_assignment {
					a := 1
					b := 1 with input as [a]
				}
			`,
			exp: `
				package test
				rewrite_value_in_assignment = true { __local0__ = 1; __local1__ = 1 with input as [__local0__] }
			`,
			expRewrittenMap: map[Var]Var{
				Var("__local0__"): Var("a"),
				Var("__local1__"): Var("b"),
			},
		},
		{
			module: `
				package test
				global = {}
				ref_shadowed {
					global := {"a": 1}
					global.a > 0
				}
			`,
			exp: `
				package test
				global = {} { true }
				ref_shadowed = true { __local0__ = {"a": 1}; gt(__local0__.a, 0) }
			`,
			expRewrittenMap: map[Var]Var{
				Var("__local0__"): Var("global"),
			},
		},
		{
			module: `
				package test
				f(x) = y {
					x == 1
					y := 2
				} else = y {
					x == 3
					y := 4
				}
			`,
			// Each "else" rule has a separate rule head and the vars in the
			// args will be rewritten. Since we cannot currently redefine the
			// args, we must parse the module and then manually update the args.
			exp: func() *Module {
				module := MustParseModule(`
					package test

					f(__local0__) = __local1__ { __local0__ == 1; __local1__ = 2 } else = __local3__ { __local2__ == 3; __local3__ = 4 }
				`)
				module.Rules[0].Else.Head.Args[0].Value = Var("__local2__")
				return module
			},
			expRewrittenMap: map[Var]Var{
				Var("__local0__"): Var("x"),
				Var("__local1__"): Var("y"),
				Var("__local2__"): Var("x"),
				Var("__local3__"): Var("y"),
			},
		},
		{
			module: `
				package test
				f({"x": [x]}) = y { x == 1; y := 2 }`,
			exp: `
				package test

				f({"x": [__local0__]}) = __local1__ { __local0__ == 1; __local1__ = 2 }`,
			expRewrittenMap: map[Var]Var{
				Var("__local0__"): Var("x"),
				Var("__local1__"): Var("y"),
			},
		},
		{
			module: `
				package test

				f(x, [x]) = x { x == 1 }
			`,
			exp: `
				package test

				f(__local0__, [__local0__]) = __local0__ { __local0__ == 1 }
			`,
			expRewrittenMap: map[Var]Var{
				Var("__local0__"): Var("x"),
			},
		},
		{
			module: `
				package test

				f(x) = {x[0]: 1} { true }
			`,
			exp: `
				package test

				f(__local0__) = {__local0__[0]: 1} { true }
			`,
			expRewrittenMap: map[Var]Var{
				Var("__local0__"): Var("x"),
			},
		},
		{
			module: `
				package test

				f({{t | t := 0}: 1}) {
					true
				}
			`,
			exp: `
				package test

				f({{__local0__ | __local0__ = 0}: 1}) { true }
			`,
			expRewrittenMap: map[Var]Var{
				Var("__local0__"): Var("t"),
			},
		},
		{
			module: `
				package test

				f({{t | t := 0}}) {
					true
				}
			`,
			exp: `
				package test

				f({{__local0__ | __local0__ = 0}}) { true }
			`,
			expRewrittenMap: map[Var]Var{
				Var("__local0__"): Var("t"),
			},
		},
	}

	for i, tc := range tests {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			c := NewCompiler()
			c.Modules = map[string]*Module{
				"test.rego": MustParseModule(tc.module),
			}
			compileStages(c, c.rewriteLocalVars)
			assertNotFailed(t, c)
			result := c.Modules["test.rego"]
			var exp *Module
			switch e := tc.exp.(type) {
			case string:
				exp = MustParseModule(e)
			case func() *Module:
				exp = e()
			default:
				panic("expected value must be string or func() *Module")
			}
			if result.Compare(exp) != 0 {
				t.Fatalf("\nExpected:\n\n%v\n\nGot:\n\n%v", exp, result)
			}
			if !reflect.DeepEqual(c.RewrittenVars, tc.expRewrittenMap) {
				t.Fatalf("\nExpected Rewritten Vars:\n\n\t%+v\n\nGot:\n\n\t%+v\n\n", tc.expRewrittenMap, c.RewrittenVars)
			}
		})
	}

}
func TestRewriteLocalVarDeclarationErrors(t *testing.T) {

	c := NewCompiler()

	c.Modules["test"] = MustParseModule(`package test

	redeclaration {
		r1 = 1
		r1 := 2
		r2 := 1
		[b, r2] := [1, 2]
		input.path == 1
		input := "foo"
		_ := [1 | nested := 1; nested := 2]
	}

	negation {
		not a := 1
	}

	bad_assign {
		null := x
		true := x
		4.5 := x
		"foo" := x
		[true | true] := []
		{true | true} := set()
		{"foo": true | true} := {}
		x + 1 := 2
		data.foo := 1
		[z, 1] := [1, 2]
	}

	arg_redeclared(arg1) {
		arg1 := 1
	}

	arg_nested_redeclared({{arg_nested| arg_nested := 1; arg_nested := 2}}) { true }
	`)

	compileStages(c, c.rewriteLocalVars)

	expectedErrors := []string{
		"var r1 referenced above",
		"var r2 assigned above",
		"var input referenced above",
		"var nested assigned above",
		"arg arg1 redeclared",
		"var arg_nested assigned above",
		"cannot assign vars inside negated expression",
		"cannot assign to ref",
		"cannot assign to arraycomprehension",
		"cannot assign to setcomprehension",
		"cannot assign to objectcomprehension",
		"cannot assign to call",
		"cannot assign to number",
		"cannot assign to number",
		"cannot assign to boolean",
		"cannot assign to string",
		"cannot assign to null",
	}

	sort.Strings(expectedErrors)

	result := []string{}

	for i := range c.Errors {
		result = append(result, c.Errors[i].Message)
	}

	sort.Strings(result)

	if len(expectedErrors) != len(result) {
		t.Fatalf("Expected %d errors but got %d:\n\n%v\n\nGot:\n\n%v", len(expectedErrors), len(result), strings.Join(expectedErrors, "\n"), strings.Join(result, "\n"))
	}

	for i := range result {
		if result[i] != expectedErrors[i] {
			t.Fatalf("Expected:\n\n%v\n\nGot:\n\n%v", strings.Join(expectedErrors, "\n"), strings.Join(result, "\n"))
		}
	}
}

func TestRewriteDeclaredVarsStage(t *testing.T) {

	// Unlike the following test case, this only executes up to the
	// RewriteLocalVars stage. This is done so that later stages like
	// RewriteDynamics are not executed.

	tests := []struct {
		note   string
		module string
		exp    string
	}{
		{
			note: "object ref key",
			module: `
				package test

				p {
					a := {"a": "a"}
					{a.a: a.a}
				}
			`,
			exp: `
				package test

				p {
					__local0__ = {"a": "a"}
					{__local0__.a: __local0__.a}
				}
			`,
		},
		{
			note: "set ref element",
			module: `
				package test

				p {
					a := {"a": "a"}
					{a.a}
				}
			`,
			exp: `
				package test

				p {
					__local0__ = {"a": "a"}
					{__local0__.a}
				}
			`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {

			c := NewCompiler()

			c.Modules = map[string]*Module{
				"test.rego": MustParseModule(tc.module),
			}

			compileStages(c, c.rewriteLocalVars)

			exp := MustParseModule(tc.exp)
			result := c.Modules["test.rego"]

			if !exp.Equal(result) {
				t.Fatalf("Expected:\n\n%v\n\nGot:\n\n%v", exp, result)
			}
		})
	}
}

func TestRewriteDeclaredVars(t *testing.T) {
	tests := []struct {
		note    string
		module  string
		exp     string
		wantErr error
	}{
		{
			note: "rewrite unify",
			module: `
				package test
				x = 1
				y = 2
				p { some x; input = [x, y] }
			`,
			exp: `
				package test
				x = 1
				y = 2
				p { __local1__ = data.test.y; input = [__local0__, __local1__] }
			`,
		},
		{
			note: "rewrite call",
			module: `
				package test
				x = []
				y = {}
				p { some x; walk(y, [x, y]) }
			`,
			exp: `
				package test
				x = []
				y = {}
				p { __local1__ = data.test.y; __local2__ = data.test.y; walk(__local1__, [__local0__, __local2__]) }
			`,
		},
		{
			note: "rewrite term",
			module: `
				package test
				x = "a"
				y = 1
				q[[2, "b"]]
				p { some x; q[[y,x]] }
			`,
			exp: `
				package test
				x = "a"
				y = 1
				q[[2, "b"]]
				p { __local1__ = data.test.y; data.test.q[[__local1__, __local0__]] }
			`,
		},
		{
			note: "rewrite some x in xs",
			module: `
				package test
				import future.keywords.in
				xs = ["a", "b", "c"]
				p { some x in xs; x == "a" }
			`,
			exp: `
				package test
				xs = ["a", "b", "c"]
				p { __local2__ = data.test.xs[__local1__]; __local2__ = "a" }
			`,
		},
		{
			note: "rewrite some k, x in xs",
			module: `
				package test
				import future.keywords.in
				xs = ["a", "b", "c"]
				p { some k, x in xs; x == "a"; k == 2 }
			`,
			exp: `
				package test
				xs = ["a", "b", "c"]
				p { __local1__ = data.test.xs[__local0__]; __local1__ = "a"; __local0__ = 2 }
			`,
		},
		{
			note: "rewrite some k, x in xs[i]",
			module: `
				package test
				import future.keywords.in
				xs = [["a", "b", "c"], []]
				p {
					some i
					some k, x in xs[i]
					x == "a"
					k == 2
				}
			`,
			exp: `
				package test
				xs = [["a", "b", "c"], []]
				p = true { __local2__ = data.test.xs[__local0__][__local1__]; __local2__ = "a"; __local1__ = 2 }
			`,
		},
		{
			note: "rewrite some k, x in xs[i] with `i` as ref",
			module: `
				package test
				import future.keywords.in
				i = 0
				xs = [["a", "b", "c"], []]
				p {
					some k, x in xs[i]
					x == "a"
					k == 2
				}
			`,
			exp: `
				package test
				i = 0
				xs = [["a", "b", "c"], []]
				p = true { __local2__ = data.test.i; __local1__ = data.test.xs[__local2__][__local0__]; __local1__ = "a"; __local0__ = 2 }
			`,
		},
		{
			note: "rewrite closures",
			module: `
				package test
				x = 1
				y = 2
				p {
					some x, z
					z = 3
					[x | x = 2; y = 2; some z; z = 4]
				}
			`,
			exp: `
				package test
				x = 1
				y = 2
				p {
					__local1__ = 3
					[__local0__ | __local0__ = 2; data.test.y = 2; __local2__ = 4]
				}
			`,
		},
		{
			note: "rewrite head var",
			module: `
				package test
				x = "a"
				y = 1
				z = 2
				p[x] = [y, z] {
					some x, z
					x = "b"
					z = 4
				}`,
			exp: `
				package test
				x = "a"
				y = 1
				z = 2
				p[__local0__] = __local2__ {
					__local0__ = "b"
					__local1__ = 4;
					__local3__ = data.test.y
					__local2__ = [__local3__, __local1__]
				}
			`,
		},
		{
			note: "rewrite call with root document ref as arg",
			module: `
				package test

				p {
					f(input, "bar")
				}

				f(x, y) {
					x[y]
				}
				`,
			exp: `
				package test

				p = true {
					__local2__ = input;
					data.test.f(__local2__, "bar")
				}

				 f(__local0__, __local1__) = true {
					__local0__[__local1__]
				}
			`,
		},
		{
			note: "redeclare err",
			module: `
				package test
				p {
					some x
					some x
				}
			`,
			wantErr: errors.New("var x declared above"),
		},
		{
			note: "redeclare assigned err",
			module: `
				package test
				p {
					x := 1
					some x
				}
			`,
			wantErr: errors.New("var x assigned above"),
		},
		{
			note: "redeclare reference err",
			module: `
				package test
				p {
					data.q[x]
					some x
				}
			`,
			wantErr: errors.New("var x referenced above"),
		},
		{
			note: "declare unused err",
			module: `
				package test
				p {
					some x
				}
			`,
			wantErr: errors.New("declared var x unused"),
		},
		{
			note: "declare unsafe err",
			module: `
				package test
				p[x] {
					some x
					x == 1
				}
			`,
			wantErr: errors.New("var x is unsafe"),
		},
		{
			note: "declare arg err",
			module: `
			package test

			f([a]) {
				some a
				a = 1
			}
			`,
			wantErr: errors.New("arg a redeclared"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			compiler, err := CompileModules(map[string]string{"test.rego": tc.module})
			if tc.wantErr != nil {
				if err == nil {
					t.Fatal("Expected error but got success")
				}
				if !strings.Contains(err.Error(), tc.wantErr.Error()) {
					t.Fatalf("Expected %v but got %v", tc.wantErr, err)
				}
			} else if err != nil {
				t.Fatal(err)
			} else {
				exp := MustParseModule(tc.exp)
				result := compiler.Modules["test.rego"]
				if exp.Compare(result) != 0 {
					t.Fatalf("Expected:\n\n%v\n\nGot:\n\n%v", exp, result)
				}
			}
		})
	}
}

func TestCompileInvalidEqAssignExpr(t *testing.T) {

	c := NewCompiler()

	c.Modules["error"] = MustParseModule(`package errors


	p {
		# Type checking runs at a later stage so these errors will not be #
		# caught until then. The stages before type checking should be tolerant
		# of invalid eq and assign calls.
		assign()
		assign(1)
		eq()
		eq(1)
	}`)

	var prev func()
	checkRecursion := reflect.ValueOf(c.checkRecursion)

	for _, stage := range c.stages {
		if reflect.ValueOf(stage.f).Pointer() == checkRecursion.Pointer() {
			break
		}
		prev = stage.f
	}

	compileStages(c, prev)
	assertNotFailed(t, c)
}

func TestCompilerRewriteComprehensionTerm(t *testing.T) {

	c := NewCompiler()
	c.Modules["head"] = MustParseModule(`package head
	arr = [[1], [2], [3]]
	arr2 = [["a"], ["b"], ["c"]]
	arr_comp = [[x[i]] | arr[j] = x]
	set_comp = {[x[i]] | arr[j] = x}
	obj_comp = {x[i]: x[i] | arr2[j] = x}
	`)

	compileStages(c, c.rewriteComprehensionTerms)
	assertNotFailed(t, c)

	arrCompRule := c.Modules["head"].Rules[2]
	exp1 := MustParseRule(`arr_comp = [__local0__ | data.head.arr[j] = x; __local0__ = [x[i]]] { true }`)
	assertRulesEqual(t, arrCompRule, exp1)

	setCompRule := c.Modules["head"].Rules[3]
	exp2 := MustParseRule(`set_comp = {__local1__ | data.head.arr[j] = x; __local1__ = [x[i]]} { true }`)
	assertRulesEqual(t, setCompRule, exp2)

	objCompRule := c.Modules["head"].Rules[4]
	exp3 := MustParseRule(`obj_comp = {__local2__: __local3__ | data.head.arr2[j] = x; __local2__ = x[i]; __local3__ = x[i]} { true }`)
	assertRulesEqual(t, objCompRule, exp3)
}

func TestCompilerRewriteDoubleEq(t *testing.T) {
	tests := []struct {
		note  string
		input string
		exp   string
	}{
		{
			note:  "vars and constants",
			input: "p { x = 1; x == 1; y = [1,2,3]; y == [1,2,3] }",
			exp:   `x = 1; x = 1; y = [1,2,3]; y = [1,2,3]`,
		},
		{
			note:  "refs",
			input: "p { input.x == data.y }",
			exp:   `input.x = data.y`,
		},
		{
			note:  "comprehensions",
			input: "p { [1|true] == [2|true] }",
			exp:   `[1|true] = [2|true]`,
		},
		// TODO(tsandall): improve support for calls so that extra unification step is
		// not required. This requires more changes to the compiler as the initial
		// stages that rewrite term exprs needs to be updated to handle == differently
		// and then other stages need to be reviewed to make sure they can deal with
		// nested calls. Alternatively, the compiler could keep track of == exprs that
		// have been converted into = and then the safety check would need to be updated.
		{
			note:  "calls",
			input: "p { count([1,2]) == 2 }",
			exp:   `count([1,2], __local0__); __local0__ = 2`,
		},
		{
			note:  "embedded",
			input: "p { x = 1; y = [x == 0] }",
			exp:   `x = 1; equal(x, 0, __local0__); y = [__local0__]`,
		},
		{
			note:  "embedded in call",
			input: `p { x = 0; neq(true, x == 1) }`,
			exp:   `x = 0; equal(x, 1, __local0__); neq(true, __local0__)`,
		},
		{
			note:  "comprehension in object key",
			input: `p { {{1 | 0 == 0}: 2} }`,
			exp:   `{{1 | 0 = 0}: 2}`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			c := NewCompiler()
			c.Modules["test"] = MustParseModule("package test\n" + tc.input)
			compileStages(c, c.rewriteEquals)
			assertNotFailed(t, c)
			exp := MustParseBody(tc.exp)
			result := c.Modules["test"].Rules[0].Body
			if result.Compare(exp) != 0 {
				t.Fatalf("\nExp: %v\nGot: %v", exp, result)
			}
		})
	}
}

func TestCompilerRewriteDynamicTerms(t *testing.T) {

	fixture := `
		package test
		str = "hello"
	`

	tests := []struct {
		input    string
		expected string
	}{
		{`arr { [str] }`, `__local0__ = data.test.str; [__local0__]`},
		{`arr2 { [[str]] }`, `__local0__ = data.test.str; [[__local0__]]`},
		{`obj { {"x": str} }`, `__local0__ = data.test.str; {"x": __local0__}`},
		{`obj2 { {"x": {"y": str}} }`, `__local0__ = data.test.str; {"x": {"y": __local0__}}`},
		{`set { {str} }`, `__local0__ = data.test.str; {__local0__}`},
		{`set2 { {{str}} }`, `__local0__ = data.test.str; {{__local0__}}`},
		{`ref { str[str] }`, `__local0__ = data.test.str; data.test.str[__local0__]`},
		{`ref2 { str[str[str]] }`, `__local0__ = data.test.str; __local1__ = data.test.str[__local0__]; data.test.str[__local1__]`},
		{`arr_compr { [1 | [str]] }`, `[1 | __local0__ = data.test.str; [__local0__]]`},
		{`arr_compr2 { [1 | [1 | [str]]] }`, `[1 | [1 | __local0__ = data.test.str; [__local0__]]]`},
		{`set_compr { {1 | [str]} }`, `{1 | __local0__ = data.test.str; [__local0__]}`},
		{`set_compr2 { {1 | {1 | [str]}} }`, `{1 | {1 | __local0__ = data.test.str; [__local0__]}}`},
		{`obj_compr { {"a": "b" | [str]} }`, `{"a": "b" | __local0__ = data.test.str; [__local0__]}`},
		{`obj_compr2 { {"a": "b" | {"a": "b" | [str]}} }`, `{"a": "b" | {"a": "b" | __local0__ = data.test.str; [__local0__]}}`},
		{`equality { str = str }`, `data.test.str = data.test.str`},
		{`equality2 { [str] = [str] }`, `__local0__ = data.test.str; __local1__ = data.test.str; [__local0__] = [__local1__]`},
		{`call { startswith(str, "") }`, `__local0__ = data.test.str; startswith(__local0__, "")`},
		{`call2 { count([str], n) }`, `__local0__ = data.test.str; count([__local0__], n)`},
		{`eq_with { [str] = [1] with input as 1 }`, `__local0__ = data.test.str with input as 1; [__local0__] = [1] with input as 1`},
		{`term_with { [[str]] with input as 1 }`, `__local0__ = data.test.str with input as 1; [[__local0__]] with input as 1`},
		{`call_with { count(str) with input as 1 }`, `__local0__ = data.test.str with input as 1; count(__local0__) with input as 1`},
		{`call_func { f(input, "foo") } f(x,y) { x[y] }`, `__local2__ = input; data.test.f(__local2__, "foo")`},
		{`call_func2 { f(input.foo, "foo") } f(x,y) { x[y] }`, `__local2__ = input.foo; data.test.f(__local2__, "foo")`},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			c := NewCompiler()
			module := fixture + tc.input
			c.Modules["test"] = MustParseModule(module)
			compileStages(c, c.rewriteDynamicTerms)
			assertNotFailed(t, c)
			expected := MustParseBody(tc.expected)
			result := c.Modules["test"].Rules[1].Body
			if result.Compare(expected) != 0 {
				t.Fatalf("\nExp: %v\nGot: %v", expected, result)
			}
		})
	}
}

func TestCompilerRewriteWithValue(t *testing.T) {
	fixture := `package test

	arr = ["hello", "goodbye"]

	`

	tests := []struct {
		note     string
		input    string
		expected string
		wantErr  error
	}{
		{
			note:     "nop",
			input:    `p { true with input as 1 }`,
			expected: `p { true with input as 1 }`,
		},
		{
			note:     "refs",
			input:    `p { true with input as arr }`,
			expected: `p { __local0__ = data.test.arr; true with input as __local0__ }`,
		},
		{
			note:     "array comprehension",
			input:    `p { true with input as [true | true] }`,
			expected: `p { __local0__ = [true | true]; true with input as __local0__ }`,
		},
		{
			note:     "set comprehension",
			input:    `p { true with input as {true | true} }`,
			expected: `p { __local0__ = {true | true}; true with input as __local0__ }`,
		},
		{
			note:     "object comprehension",
			input:    `p { true with input as {"k": true | true} }`,
			expected: `p { __local0__ = {"k": true | true}; true with input as __local0__ }`,
		},
		{
			note:     "comprehension nested",
			input:    `p { true with input as [true | true with input as arr] }`,
			expected: `p { __local0__ = [true | __local1__ = data.test.arr; true with input as __local1__]; true with input as __local0__ }`,
		},
		{
			note:     "multiple",
			input:    `p { true with input.a as arr[0] with input.b as arr[1] }`,
			expected: `p { __local0__ = data.test.arr[0]; __local1__ = data.test.arr[1]; true with input.a as __local0__ with input.b as __local1__ }`,
		},
		{
			note:    "invalid target",
			input:   `p { true with foo.q as 1 }`,
			wantErr: fmt.Errorf("rego_type_error: with keyword target must start with input or data"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			c := NewCompiler()
			module := fixture + tc.input
			c.Modules["test"] = MustParseModule(module)
			compileStages(c, c.rewriteWithModifiers)
			if tc.wantErr == nil {
				assertNotFailed(t, c)
				expected := MustParseRule(tc.expected)
				result := c.Modules["test"].Rules[1]
				if result.Compare(expected) != 0 {
					t.Fatalf("\nExp: %v\nGot: %v", expected, result)
				}
			} else {
				assertCompilerErrorStrings(t, c, []string{tc.wantErr.Error()})
			}
		})
	}
}

func TestCompilerRewritePrintCallsErasure(t *testing.T) {

	cases := []struct {
		note   string
		module string
		exp    string
	}{
		{
			note: "no-op",
			module: `package test
			p { true }`,
			exp: `package test
			p { true }`,
		},
		{
			note: "replace empty body with true",
			module: `package test

			p { print(1) }
			`,
			exp: `package test

			p { true } `,
		},
		{
			note: "rule body",
			module: `package test

			p { false; print(1) }
			`,
			exp: `package test

			p { false } `,
		},
		{
			note: "set comprehension body",
			module: `package test

			p { {1 | false; print(1)} }
			`,
			exp: `package test

			p { {1 | false} } `,
		},
		{
			note: "array comprehension body",
			module: `package test

			p { [1 | false; print(1)] }
			`,
			exp: `package test

			p { [1 | false] } `,
		},
		{
			note: "object comprehension body",
			module: `package test

			p { {"x": 1 | false; print(1)} }
			`,
			exp: `package test

			p { {"x": 1 | false} } `,
		},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			c := NewCompiler().WithEnablePrintStatements(false)
			c.Compile(map[string]*Module{
				"test.rego": MustParseModule(tc.module),
			})
			if c.Failed() {
				t.Fatal(c.Errors)
			}
			exp := MustParseModule(tc.exp)
			if !exp.Equal(c.Modules["test.rego"]) {
				t.Fatalf("Expected:\n\n%v\n\nGot:\n\n%v", exp, c.Modules["test.rego"])
			}
		})
	}
}

func TestCompilerRewritePrintCallsErrors(t *testing.T) {
	cases := []struct {
		note   string
		module string
		exp    error
	}{
		{
			note: "non-existent var",
			module: `package test

			p { print(x) }`,
			exp: errors.New("var x is undeclared"),
		},
		{
			note: "declared after print",
			module: `package test

			p { print(x); x = 7 }`,
			exp: errors.New("var x is undeclared"),
		},
		{
			note: "inside comprehension",
			module: `package test
			p { {1 | print(x)} }
			`,
			exp: errors.New("var x is undeclared"),
		},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			c := NewCompiler().WithEnablePrintStatements(true)
			c.Compile(map[string]*Module{
				"test.rego": MustParseModule(tc.module),
			})
			if !c.Failed() {
				t.Fatal("expected error")
			}
			if c.Errors[0].Code != CompileErr || c.Errors[0].Message != tc.exp.Error() {
				t.Fatal("unexpected error:", c.Errors)
			}
		})
	}
}

func TestCompilerRewritePrintCalls(t *testing.T) {
	cases := []struct {
		note   string
		module string
		exp    string
	}{
		{
			note: "print one",
			module: `package test

			p { print(1) }`,
			exp: `package test

			p = true { __local1__ = {__local0__ | __local0__ = 1}; internal.print([__local1__]) }`,
		},
		{
			note: "print multiple",
			module: `package test

			p { print(1, 2) }`,
			exp: `package test

			p = true { __local2__ = {__local0__ | __local0__ = 1}; __local3__ = {__local1__ | __local1__ = 2}; internal.print([__local2__, __local3__]) }`,
		},
		{
			note: "print inside set comprehension",
			module: `package test

			p { x = 1; {2 | print(x)} }`,
			exp: `package test

			p = true { x = 1; {2 | __local1__ = {__local0__ | __local0__ = x}; internal.print([__local1__])} }`,
		},
		{
			note: "print inside array comprehension",
			module: `package test

			p { x = 1; [2 | print(x)] }`,
			exp: `package test

			p = true { x = 1; [2 | __local1__ = {__local0__ | __local0__ = x}; internal.print([__local1__])] }`,
		},
		{
			note: "print inside object comprehension",
			module: `package test

			p { x = 1; {"x": 2 | print(x)} }`,
			exp: `package test

			p = true { x = 1; {"x": 2 | __local1__ = {__local0__ | __local0__ = x}; internal.print([__local1__])} }`,
		},
		{
			note: "print output of nested call",
			module: `package test

			p {
				x := split("abc", "")[y]
				print(x, y)
			}`,
			exp: `package test

			p = true { split("abc", "", __local3__); __local0__ = __local3__[y]; __local4__ = {__local1__ | __local1__ = __local0__}; __local5__ = {__local2__ | __local2__ = y}; internal.print([__local4__, __local5__]) }`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			c := NewCompiler().WithEnablePrintStatements(true)
			c.Compile(map[string]*Module{
				"test.rego": MustParseModule(tc.module),
			})
			if c.Failed() {
				t.Fatal(c.Errors)
			}
			exp := MustParseModule(tc.exp)
			if !exp.Equal(c.Modules["test.rego"]) {
				t.Fatalf("Expected:\n\n%v\n\nGot:\n\n%v", exp, c.Modules["test.rego"])
			}
		})
	}
}

func TestCompilerMockFunction(t *testing.T) {
	c := NewCompiler()
	c.Modules["test"] = MustParseModule(`
	package test

	is_allowed(label) {
	    label == "test_label"
	}

	p {true with data.test.is_allowed as "blah" }
	`)
	compileStages(c, c.rewriteWithModifiers)
	assertCompilerErrorStrings(t, c, []string{"rego_compile_error: with keyword cannot replace functions"})
}

func TestCompilerMockVirtualDocumentPartially(t *testing.T) {
	c := NewCompiler()

	c.Modules["test"] = MustParseModule(`
	package test
	p = {"a": 1}
	q = x { p = x with p.a as 2 }
	`)

	compileStages(c, c.rewriteWithModifiers)
	assertCompilerErrorStrings(t, c, []string{"rego_compile_error: with keyword cannot partially replace virtual document(s)"})
}

func TestCompilerSetGraph(t *testing.T) {
	c := NewCompiler()
	c.Modules = getCompilerTestModules()
	c.Modules["elsekw"] = MustParseModule(`
	package elsekw

	p {
		false
	} else = q {
		false
	} else {
		r
	}

	q = true
	r = true

	s { t }
	t { false } else { true }

	`)
	compileStages(c, c.setGraph)

	assertNotFailed(t, c)

	mod1 := c.Modules["mod1"]
	p := mod1.Rules[0]
	q := mod1.Rules[1]
	mod2 := c.Modules["mod2"]
	r := mod2.Rules[0]
	mod5 := c.Modules["mod5"]

	edges := map[util.T]struct{}{
		q: {},
		r: {},
	}

	if !reflect.DeepEqual(edges, c.Graph.Dependencies(p)) {
		t.Fatalf("Expected dependencies for p to be q and r but got: %v", c.Graph.Dependencies(p))
	}

	// NOTE(tsandall): this is the correct result but it's chosen arbitrarily for the test.
	expDependents := []struct {
		x    *Rule
		want map[util.T]struct{}
	}{
		{
			x:    p,
			want: nil,
		},
		{
			x:    q,
			want: map[util.T]struct{}{p: {}, mod5.Rules[1]: {}, mod5.Rules[3]: {}, mod5.Rules[5]: {}},
		},
		{
			x:    r,
			want: map[util.T]struct{}{p: {}},
		},
	}

	for _, exp := range expDependents {
		if !reflect.DeepEqual(exp.want, c.Graph.Dependents(exp.x)) {
			t.Fatalf("Expected dependents for %v to be %v but got: %v", exp.x, exp.want, c.Graph.Dependents(exp.x))
		}
	}

	sorted, ok := c.Graph.Sort()
	if !ok {
		t.Fatalf("Expected sort to succeed.")
	}

	numRules := 0

	for _, module := range c.Modules {
		WalkRules(module, func(*Rule) bool {
			numRules++
			return false
		})
	}

	if len(sorted) != numRules {
		t.Fatalf("Expected numRules (%v) to be same as len(sorted) (%v)", numRules, len(sorted))
	}

	// Probe rules with dependencies. Ordering is not stable for ties because
	// nodes are stored in a map.
	probes := [][2]*Rule{
		{c.Modules["mod1"].Rules[1], c.Modules["mod1"].Rules[0]},               // mod1.q before mod1.p
		{c.Modules["mod2"].Rules[0], c.Modules["mod1"].Rules[0]},               // mod2.r before mod1.p
		{c.Modules["mod1"].Rules[1], c.Modules["mod5"].Rules[1]},               // mod1.q before mod5.r
		{c.Modules["mod1"].Rules[1], c.Modules["mod5"].Rules[3]},               // mod1.q before mod6.t
		{c.Modules["mod1"].Rules[1], c.Modules["mod5"].Rules[5]},               // mod1.q before mod6.v
		{c.Modules["mod6"].Rules[2], c.Modules["mod6"].Rules[3]},               // mod6.r before mod6.s
		{c.Modules["elsekw"].Rules[1], c.Modules["elsekw"].Rules[0].Else},      // elsekw.q before elsekw.p.else
		{c.Modules["elsekw"].Rules[2], c.Modules["elsekw"].Rules[0].Else.Else}, // elsekw.r before elsekw.p.else.else
		{c.Modules["elsekw"].Rules[4], c.Modules["elsekw"].Rules[3]},           // elsekw.t before elsekw.s
		{c.Modules["elsekw"].Rules[4].Else, c.Modules["elsekw"].Rules[3]},      // elsekw.t.else before elsekw.s
	}

	getSortedIdx := func(r *Rule) int {
		for i := range sorted {
			if sorted[i] == r {
				return i
			}
		}
		return -1
	}

	for num, probe := range probes {
		i := getSortedIdx(probe[0])
		j := getSortedIdx(probe[1])
		if i == -1 || j == -1 {
			t.Fatalf("Expected to find probe %d in sorted slice but got: i=%d, j=%d", num+1, i, j)
		}
		if i >= j {
			t.Errorf("Sort order of probe %d (A) %v and (B) %v and is wrong (expected A before B)", num+1, probe[0], probe[1])
		}
	}
}

func TestGraphCycle(t *testing.T) {
	mod1 := `package a.b.c

	p { q }
	q { r }
	r { s }
	s { q }`

	c := NewCompiler()
	c.Modules = map[string]*Module{
		"mod1": MustParseModule(mod1),
	}

	compileStages(c, c.setGraph)
	assertNotFailed(t, c)

	_, ok := c.Graph.Sort()
	if ok {
		t.Fatalf("Expected to find cycle in rule graph")
	}

	elsekw := `package elsekw

	p {
		false
	} else = q {
		true
	}

	q {
		false
	} else {
		r
	}

	r { s }

	s { p }
	`

	c = NewCompiler()
	c.Modules = map[string]*Module{
		"elsekw": MustParseModule(elsekw),
	}

	compileStages(c, c.setGraph)
	assertNotFailed(t, c)

	_, ok = c.Graph.Sort()
	if ok {
		t.Fatalf("Expected to find cycle in rule graph")
	}

}

func TestCompilerCheckRecursion(t *testing.T) {
	c := NewCompiler()
	c.Modules = map[string]*Module{
		"newMod1": MustParseModule(`package rec

s = true { t }
t = true { s }
a = true { b }
b = true { c }
c = true { d; e }
d = true { true }
e = true { a }`),
		"newMod2": MustParseModule(`package rec

x = true { s }`,
		),
		"newMod3": MustParseModule(`package rec2

import data.rec.x

y = true { x }`),
		"newMod4": MustParseModule(`package rec3

p[x] = y { data.rec4[x][y] = z }`,
		),
		"newMod5": MustParseModule(`package rec4

import data.rec3.p

q[x] = y { p[x] = y }`),
		"newMod6": MustParseModule(`package rec5

acp[x] { acq[x] }
acq[x] { a = [true | acp[_]]; a[_] = x }
`,
		),
		"newMod7": MustParseModule(`package rec6

np[x] = y { data.a[data.b.c[nq[x]]] = y }
nq[x] = y { data.d[data.e[x].f[np[y]]] }`,
		),
		"newMod8": MustParseModule(`package rec7

prefix = true { data.rec7 }`,
		),
		"newMod9": MustParseModule(`package rec8

dataref = true { data }`,
		),
		"newMod10": MustParseModule(`package rec9

		else_self { false } else { else_self }

		elsetop {
			false
		} else = elsemid {
			true
		}

		elsemid {
			false
		} else {
			elsebottom
		}

		elsebottom { elsetop }
		`),
		"fnMod1": MustParseModule(`package f0

		fn(x) = y {
			fn(x, y)
		}`),
		"fnMod2": MustParseModule(`package f1

		foo(x) = y {
			bar("buz", x, y)
		}

		bar(x, y) = z {
			foo([x, y], z)
		}`),
		"fnMod3": MustParseModule(`package f2

		foo(x) = y {
			bar("buz", x, y)
		}

		bar(x, y) = z {
			x = p[y]
			z = x
		}

		p[x] = y {
			x = "foo.bar"
			foo(x, y)
		}`),
	}

	compileStages(c, c.checkRecursion)

	makeRuleErrMsg := func(rule string, loop ...string) string {
		return fmt.Sprintf("rego_recursion_error: rule %v is recursive: %v", rule, strings.Join(loop, " -> "))
	}

	expected := []string{
		makeRuleErrMsg("s", "s", "t", "s"),
		makeRuleErrMsg("t", "t", "s", "t"),
		makeRuleErrMsg("a", "a", "b", "c", "e", "a"),
		makeRuleErrMsg("b", "b", "c", "e", "a", "b"),
		makeRuleErrMsg("c", "c", "e", "a", "b", "c"),
		makeRuleErrMsg("e", "e", "a", "b", "c", "e"),
		makeRuleErrMsg("p", "p", "q", "p"),
		makeRuleErrMsg("q", "q", "p", "q"),
		makeRuleErrMsg("acq", "acq", "acp", "acq"),
		makeRuleErrMsg("acp", "acp", "acq", "acp"),
		makeRuleErrMsg("np", "np", "nq", "np"),
		makeRuleErrMsg("nq", "nq", "np", "nq"),
		makeRuleErrMsg("prefix", "prefix", "prefix"),
		makeRuleErrMsg("dataref", "dataref", "dataref"),
		makeRuleErrMsg("else_self", "else_self", "else_self"),
		makeRuleErrMsg("elsetop", "elsetop", "elsemid", "elsebottom", "elsetop"),
		makeRuleErrMsg("elsemid", "elsemid", "elsebottom", "elsetop", "elsemid"),
		makeRuleErrMsg("elsebottom", "elsebottom", "elsetop", "elsemid", "elsebottom"),
		makeRuleErrMsg("fn", "fn", "fn"),
		makeRuleErrMsg("foo", "foo", "bar", "foo"),
		makeRuleErrMsg("bar", "bar", "foo", "bar"),
		makeRuleErrMsg("bar", "bar", "p", "foo", "bar"),
		makeRuleErrMsg("foo", "foo", "bar", "p", "foo"),
		makeRuleErrMsg("p", "p", "foo", "bar", "p"),
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

func TestCompilerCheckDynamicRecursion(t *testing.T) {
	// This test tries to circumvent the recursion check by using dynamic
	// references.  For more background info, see
	// <https://github.com/open-policy-agent/opa/issues/1565>.
	c := NewCompiler()
	c.Modules = map[string]*Module{
		"recursion": MustParseModule(`package recursion

pkg = "recursion"

foo[x] {
  data[pkg]["foo"][x]
}`),
	}

	compileStages(c, c.checkRecursion)

	result := compilerErrsToStringSlice(c.Errors)
	expected := "rego_recursion_error: rule foo is recursive: foo -> foo"

	if len(result) != 1 || result[0] != expected {
		t.Errorf("Expected %v but got: %v", expected, result)
	}
}

func TestCompilerCheckVoidCalls(t *testing.T) {
	c := NewCompiler().WithCapabilities(&Capabilities{Builtins: []*Builtin{
		{
			Name: "test",
			Decl: types.NewFunction([]types.Type{types.B}, nil),
		},
	}})
	c.Compile(map[string]*Module{
		"test.rego": MustParseModule(`package test

		p {
			x = test(true)
		}`),
	})
	if !c.Failed() {
		t.Fatal("expected error")
	} else if c.Errors[0].Code != TypeErr || c.Errors[0].Message != "test(true) used as value" {
		t.Fatal("unexpected error:", c.Errors)
	}
}

func TestCompilerGetRulesExact(t *testing.T) {
	mods := getCompilerTestModules()

	// Add incrementally defined rules.
	mods["mod-incr"] = MustParseModule(`package a.b.c

p[1] { true }
p[2] { true }`,
	)

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
		{"outside data", "input.a.b.c.p", []*Rule{}},
		{"non-string/var", "data.a.b[data.foo]", []*Rule{}},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
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
	mods["mod-incr"] = MustParseModule(`package a.b.c

p[1] { true }
p[2] { true }`,
	)

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
		{"non-string/var", "data.a.b[data.foo]", []*Rule{}},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
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
	mods["mod-incr"] = MustParseModule(`package a.b.c

p[1] { true }
p[2] { true }
q[3] { true }`,
	)

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
		{"non-string/var", "data.a.b[data.foo]", []*Rule{}},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
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
					t.Fatalf("Expected %v but got: %v", tc.expected, rules)
				}
			}
		})
	}
}

func TestCompilerGetRules(t *testing.T) {
	compiler := getCompilerWithParsedModules(map[string]string{
		"mod1": `package a.b.c

p[x] = y { q[x] = y }
q["a"] = 1 { true }
q["b"] = 2 { true }`,
	})

	compileStages(compiler, nil)

	rule1 := compiler.Modules["mod1"].Rules[0]
	rule2 := compiler.Modules["mod1"].Rules[1]
	rule3 := compiler.Modules["mod1"].Rules[2]

	tests := []struct {
		input    string
		expected []*Rule
	}{
		{"data.a.b.c.p", []*Rule{rule1}},
		{"data.a.b.c.p.x", []*Rule{rule1}},
		{"data.a.b.c.q", []*Rule{rule2, rule3}},
		{"data.a.b.c", []*Rule{rule1, rule2, rule3}},
		{"data.a.b.d", nil},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := compiler.GetRules(MustParseRef(tc.input))

			if len(result) != len(tc.expected) {
				t.Fatalf("Expected %v but got: %v", tc.expected, result)
			}

			for i := range result {
				found := false
				for j := range tc.expected {
					if result[i].Equal(tc.expected[j]) {
						found = true
						break
					}
				}
				if !found {
					t.Fatalf("Expected %v but got: %v", tc.expected, result)
				}
			}
		})
	}

}

func TestCompilerGetRulesDynamic(t *testing.T) {
	compiler := getCompilerWithParsedModules(map[string]string{
		"mod1": `package a.b.c.d
r1 = 1`,
		"mod2": `package a.b.c.e
r2 = 2`,
		"mod3": `package a.b
r3 = 3`,
	})

	compileStages(compiler, nil)

	rule1 := compiler.Modules["mod1"].Rules[0]
	rule2 := compiler.Modules["mod2"].Rules[0]
	rule3 := compiler.Modules["mod3"].Rules[0]

	tests := []struct {
		input    string
		expected []*Rule
	}{
		{"data.a.b.c.d.r1", []*Rule{rule1}},
		{"data.a.b[x]", []*Rule{rule1, rule2, rule3}},
		{"data.a.b[x].d", []*Rule{rule1, rule3}},
		{"data.a.b.c", []*Rule{rule1, rule2}},
		{"data.a.b.d", nil},
		{"data[x]", []*Rule{rule1, rule2, rule3}},
		{"data[data.complex_computation].b[y]", []*Rule{rule1, rule2, rule3}},
		{"data[x][y].c.e", []*Rule{rule2}},
		{"data[x][y].r3", []*Rule{rule3}},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := compiler.GetRulesDynamic(MustParseRef(tc.input))

			if len(result) != len(tc.expected) {
				t.Fatalf("Expected %v but got: %v", tc.expected, result)
			}

			for i := range result {
				found := false
				for j := range tc.expected {
					if result[i].Equal(tc.expected[j]) {
						found = true
						break
					}
				}
				if !found {
					t.Fatalf("Expected %v but got: %v", tc.expected, result)
				}
			}
		})
	}

}

func TestCompileCustomBuiltins(t *testing.T) {

	compiler := NewCompiler().WithBuiltins(map[string]*Builtin{
		"baz": {
			Name: "baz",
			Decl: types.NewFunction([]types.Type{types.S}, types.A),
		},
		"foo.bar": {
			Name: "foo.bar",
			Decl: types.NewFunction([]types.Type{types.S}, types.A),
		},
	})

	compiler.Compile(map[string]*Module{
		"test.rego": MustParseModule(`
			package test

			p { baz("x") = x }
			q { foo.bar("x") = x }
		`),
	})

	// Ensure no type errors occur.
	if compiler.Failed() {
		t.Fatal("Unexpected compilation error:", compiler.Errors)
	}

	_, err := compiler.QueryCompiler().Compile(MustParseBody(`baz("x") = x; foo.bar("x") = x`))
	if err != nil {
		t.Fatal("Unexpected compilation error:", err)
	}

	// Ensure type errors occur.
	exp1 := `rego_type_error: baz: invalid argument(s)`
	exp2 := `rego_type_error: foo.bar: invalid argument(s)`

	_, err = compiler.QueryCompiler().Compile(MustParseBody(`baz(1) = x; foo.bar(1) = x`))
	if err == nil {
		t.Fatal("Expected compilation error")
	} else if !strings.Contains(err.Error(), exp1) {
		t.Fatalf("Expected:\n\n%v\n\nGot:\n\n%v", exp1, err)
	} else if !strings.Contains(err.Error(), exp2) {
		t.Fatalf("Expected:\n\n%v\n\nGot:\n\n%v", exp2, err)
	}

	compiler.Compile(map[string]*Module{
		"test.rego": MustParseModule(`
			package test

			p { baz(1) = x }  # type error
			q { foo.bar(1) = x }  # type error
		`),
	})

	assertCompilerErrorStrings(t, compiler, []string{exp1, exp2})
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

	mod1 := MustParseModule(`package a.b.c

import data.x.z1 as z2

p = true { q; r }
q = true { z2 }`)
	orig1 := mod1.Copy()

	mod2 := MustParseModule(`package a.b.c

r = true { true }`)
	orig2 := mod2.Copy()

	mod3 := MustParseModule(`package x

import data.foo.bar
import input.input

z1 = true { [localvar | count(bar.baz.qux, localvar)] }`)
	orig3 := mod3.Copy()

	mod4 := MustParseModule(`package foo.bar.baz

qux = grault { true }`)
	orig4 := mod4.Copy()

	mod5 := MustParseModule(`package foo.bar.baz

import data.d.e.f

deadbeef = f { true }
grault = deadbeef { true }`)
	orig5 := mod5.Copy()

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
			p := MustParseRule(`p = true { data.a.b.c.q; data.a.b.c.r }`)
			if !partial["mod1"].Rules[0].Equal(p) {
				t.Errorf("Expected %v but got %v", p, partial["mod1"].Rules[0])
			}
			q := MustParseRule(`q = true { data.x.z1 }`)
			if !partial["mod1"].Rules[1].Equal(q) {
				t.Errorf("Expected %v but got %v", q, partial["mod1"].Rules[0])
			}
		},
		func(partial map[string]*Module) {
			z1 := MustParseRule(`z1 = true { [localvar | count(data.foo.bar.baz.qux, localvar)] }`)
			if !partial["mod3"].Rules[0].Equal(z1) {
				t.Errorf("Expected %v but got %v", z1, partial["mod3"].Rules[0])
			}
		},
		func(partial map[string]*Module) {
			qux := MustParseRule(`qux = grault { true }`)
			if !partial["mod4"].Rules[0].Equal(qux) {
				t.Errorf("Expected %v but got %v", qux, partial["mod4"].Rules[0])
			}
		},
		func(partial map[string]*Module) {
			grault := MustParseRule(`qux = data.foo.bar.baz.grault { true }`) // rewrite has not happened yet
			f := MustParseRule(`deadbeef = data.d.e.f { true }`)
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

	// Check the original modules are still untouched.
	if !mod1.Equal(orig1) || !mod2.Equal(orig2) || !mod3.Equal(orig3) || !mod4.Equal(orig4) || !mod5.Equal(orig5) {
		t.Errorf("Compiler lazy loading modified the original modules")
	}
}

func TestCompilerWithMetrics(t *testing.T) {
	m := metrics.New()
	c := NewCompiler().WithMetrics(m)
	mod := MustParseModule(testModule)

	c.Compile(map[string]*Module{"testMod": mod})
	assertNotFailed(t, c)

	if len(m.All()) == 0 {
		t.Error("Expected to have metrics after compiling")
	}
}

func TestCompilerWithStageAfterWithMetrics(t *testing.T) {
	m := metrics.New()
	c := NewCompiler().WithStageAfter(
		"CheckRecursion",
		CompilerStageDefinition{"MockStage", "mock_stage", mockStageFunctionCallNoErr},
	)

	c.WithMetrics(m)

	mod := MustParseModule(testModule)

	c.Compile(map[string]*Module{"testMod": mod})
	assertNotFailed(t, c)

	if len(m.All()) == 0 {
		t.Error("Expected to have metrics after compiling")
	}
}

func TestCompilerBuildComprehensionIndexKeySet(t *testing.T) {

	type expectedComprehension struct {
		term, keys string
	}
	type exp map[int]expectedComprehension
	tests := []struct {
		note      string
		module    string
		expected  exp
		wantDebug int
	}{
		{
			note: "example: invert object",
			module: `
				package test

				p {
					value = input[i]
					keys = [j | value = input[j]]
				}
			`,
			expected: exp{6: {
				term: `[j | value = input[j]]`,
				keys: `[value]`,
			}},
			wantDebug: 1,
		},
		{
			note: "example: multiple keys from body",
			module: `
				package test

				p {
					v1 = input[i].v1
					v2 = input[i].v2
					keys = [j | v1 = input[j].v1; v2 = input[j].v2]
				}
			`,
			expected: exp{7: {
				term: `[j | v1 = input[j].v1; v2 = input[j].v2]`,
				keys: `[v1, v2]`,
			}},
			wantDebug: 1,
		},
		{
			note: "example: nested comprehensions are supported",
			module: `
				package test

				p = {x: ys |
					x = input[i]
					ys = {y | x = input[y]}
				}
			`,
			expected: exp{6: {
				term: `{y | x = input[y]}`,
				keys: `[x]`,
			}},
			// there are still things going on here that'll be reported, besides successful indexing
			wantDebug: 2,
		},
		{
			note: "skip: lone comprehensions",
			module: `
				package test

				p {
					[v | input[i] = v]  # skip because no assignment
				}`,
			wantDebug: 0,
		},
		{
			note: "skip: due to with modifier",
			module: `
				package test

				p {
					v = input[i]
					ks = [j | input[j] = v] with data.x as 1  # skip because of with modifier
				}`,
			wantDebug: 0,
		},
		{
			note: "skip: due to negation",
			module: `
				package test

				p {
					v = input[i]
					a = []
					not a = [j | input[j] = v] # skip due to negation
				}`,
			wantDebug: 0,
		},
		{
			note: "skip: due to lack of comprehension",
			module: `
				package test

				p {
					v = input[i]
				}`,
			wantDebug: 0, // nothing interesting to report here
		},
		{
			note: "skip: due to unsafe comprehension body",
			module: `
				package test

				f(x) {
					v = input[i]
					ys = [y | y = x[j]]  # x is not safe
				}`,
			wantDebug: 1,
		},
		{
			note: "skip: due to no candidates",
			module: `
				package test

				p {
					ys = [y | y = input[j]]
				}`,
			wantDebug: 1,
		},
		{
			note: "mixed: due to nested comprehension containing candidate + indexed nested comprehension with key from rule body",
			module: `
				package test

				p {
					x = input[i]                # 'x' is a candidate for z (line 7)
					y = 2                       # 'y' is a candidate for z
					z = [1 |
						x = data.foo[j]             # 'x' is an index key for z
						t = [1 | data.bar[k] = y]   # 'y' disqualifies indexing of z because it is nested inside a comprehension
					]
				}
			`,
			// Note: no comprehension index for line 7 (`z = [ ...`)
			expected: exp{9: {
				keys: `[y]`,
				term: `[1 | data.bar[k] = y]`,
			}},
			wantDebug: 2,
		},
		{
			note: "skip: avoid increasing runtime (func arg)",
			module: `
				package test

				f(x) {
					y = input[x]
					ys = [y | y = input[x]]
				}`,
			wantDebug: 1,
		},
		{
			note: "skip: avoid increasing runtime (head key)",
			module: `
				package test

				p[x] {
					y = input[x]
					ys = [y | y = input[x]]
				}`,
			wantDebug: 1,
		},
		{
			note: "skip: avoid increasing runtime (walk)",
			module: `
				package test

				p[x] {
					y = input.bar[x]
					ys = [y | a = input.foo; walk(a, [x, y])]
				}`,
			wantDebug: 1,
		},
		{
			note: "bypass: use intermediate var to skip regression check",
			module: `
				package test

				p[x] {
					y = input[x]
					ys = [y | y = input[z]; z = x]
				}`,
			expected: exp{6: {
				term: ` [y | y = input[z]; z = x]`,
				keys: `[x, y]`,
			}},
			wantDebug: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			dbg := bytes.Buffer{}
			m := metrics.New()
			compiler := NewCompiler().WithMetrics(m).WithDebug(&dbg)
			mod, err := ParseModule("test.rego", tc.module)
			if err != nil {
				t.Fatal(err)
			}
			compiler.Compile(map[string]*Module{"test.rego": mod})
			if compiler.Failed() {
				t.Fatal(compiler.Errors)
			}

			messages := strings.Split(dbg.String(), "\n")
			messages = messages[:len(messages)-1] // last one is an empty string
			if exp, act := tc.wantDebug, len(messages); exp != act {
				t.Errorf("expected %d debug messages, got %d", exp, act)
				for i, m := range messages {
					t.Logf("%d: %s\n", i, m)
				}
			}

			n := m.Counter(compileStageComprehensionIndexBuild).Value().(uint64)
			if exp, act := len(tc.expected), len(compiler.comprehensionIndices); exp != act {
				t.Fatalf("expected %d indices to be built. got: %d", exp, act)
			}
			if len(tc.expected) == 0 {
				return
			}
			if n == 0 {
				t.Fatal("expected counter to be incremented")
			}

			for row, exp := range tc.expected {
				var comprehension *Term
				WalkTerms(compiler.Modules["test.rego"], func(x *Term) bool {
					if !IsComprehension(x.Value) {
						return true
					}
					_, ok := tc.expected[x.Location.Row]
					if !ok {
						return false
					} else if comprehension != nil {
						t.Fatal("expected at most one comprehension per line in test module")
					}
					comprehension = x
					return false
				})
				if comprehension == nil {
					t.Fatal("expected comprehension at line:", row)
				}

				result := compiler.ComprehensionIndex(comprehension)
				if result == nil {
					t.Fatal("expected result")
				}

				expTerm := MustParseTerm(exp.term)
				if !result.Term.Equal(expTerm) {
					t.Fatalf("expected term to be %v but got: %v", expTerm, result.Term)
				}

				expKeys := MustParseTerm(exp.keys).Value.(*Array)
				if NewArray(result.Keys...).Compare(expKeys) != 0 {
					t.Fatalf("expected keys to be %v but got: %v", expKeys, result.Keys)
				}
			}
		})
	}
}

func TestQueryCompiler(t *testing.T) {
	tests := []struct {
		note     string
		q        string
		pkg      string
		imports  []string
		input    string
		expected interface{}
	}{
		{
			note:     "empty query",
			q:        "   \t \n # foo \n",
			expected: fmt.Errorf("1 error occurred: rego_compile_error: empty query cannot be compiled"),
		},
		{
			note:     "invalid eq",
			q:        "eq()",
			expected: fmt.Errorf("too few arguments"),
		},
		{
			note:     "invalid eq",
			q:        "eq(1)",
			expected: fmt.Errorf("too few arguments"),
		},
		{
			note:     "rewrite assignment",
			q:        "a := 1; [b, c] := data.foo",
			pkg:      "",
			imports:  nil,
			expected: "__localq0__ = 1; [__localq1__, __localq2__] = data.foo",
		},
		{
			note:     "exports resolved",
			q:        "z",
			pkg:      `package a.b.c`,
			imports:  nil,
			expected: "data.a.b.c.z",
		},
		{
			note:     "imports resolved",
			q:        "z",
			pkg:      `package a.b.c.d`,
			imports:  []string{"import data.a.b.c.z"},
			expected: "data.a.b.c.z",
		},
		{
			note:     "rewrite comprehensions",
			q:        "[x[i] | a = [[1], [2]]; x = a[j]]",
			pkg:      "",
			imports:  nil,
			expected: "[__localq0__ | a = [[1], [2]]; x = a[j]; __localq0__ = x[i]]",
		},
		{
			note:     "unsafe vars",
			q:        "z",
			pkg:      "",
			imports:  nil,
			expected: fmt.Errorf("1 error occurred: 1:1: rego_unsafe_var_error: var z is unsafe"),
		},
		{
			note:     "unsafe var that is a future keyword",
			q:        "1 in 2",
			expected: fmt.Errorf("1 error occurred: 1:3: rego_unsafe_var_error: var in is unsafe (hint: `import future.keywords.in` to import a future keyword)"),
		},
		{
			note:     "unsafe declared var",
			q:        "[1 | some x; x == 1]",
			pkg:      "",
			imports:  nil,
			expected: fmt.Errorf("1 error occurred: 1:14: rego_unsafe_var_error: var x is unsafe"),
		},
		{
			note:     "safe vars",
			q:        `data; abc`,
			pkg:      `package ex`,
			imports:  []string{"import input.xyz as abc"},
			expected: `data; input.xyz`,
		},
		{
			note:     "reorder",
			q:        `x != 1; x = 0`,
			pkg:      "",
			imports:  nil,
			expected: `x = 0; x != 1`,
		},
		{
			note:     "bad with target",
			q:        "x = 1 with foo.p as null",
			pkg:      "",
			imports:  nil,
			expected: fmt.Errorf("1 error occurred: 1:12: rego_type_error: with keyword target must start with input or data"),
		},
		{
			note:     "rewrite with value",
			q:        `1 with input as [z]`,
			pkg:      "package a.b.c",
			imports:  nil,
			expected: `__localq1__ = data.a.b.c.z; __localq0__ = [__localq1__]; 1 with input as __localq0__`,
		},
		{
			note:     "unsafe exprs",
			q:        "count(sum())",
			pkg:      "",
			imports:  nil,
			expected: fmt.Errorf("1 error occurred: 1:1: rego_unsafe_var_error: expression is unsafe"),
		},
		{
			note:     "check types",
			q:        "x = data.a.b.c.z; y = null; x = y",
			pkg:      "",
			imports:  nil,
			expected: fmt.Errorf("match error\n\tleft  : number\n\tright : null"),
		},
		{
			note:     "undefined function",
			q:        "data.deadbeef(x)",
			expected: fmt.Errorf("rego_type_error: undefined function data.deadbeef"),
		},
		{
			note:     "imports resolved without package",
			q:        "abc",
			pkg:      "",
			imports:  []string{"import input.xyz as abc"},
			expected: "input.xyz",
		},
		{
			note:     "void call used as value",
			q:        "x = print(1)",
			expected: fmt.Errorf("rego_type_error: print(1) used as value"),
		},
		{
			note:     "print call erasure",
			q:        `print(1)`,
			expected: "true",
		},
	}
	for _, tc := range tests {
		runQueryCompilerTest(t, tc.note, tc.q, tc.pkg, tc.imports, tc.expected)
	}
}

func TestQueryCompilerRewrittenVars(t *testing.T) {
	tests := []struct {
		note string
		q    string
		vars map[string]string
	}{
		{"assign", "a := 1", map[string]string{"__localq0__": "a"}},
		{"suppress only seen", "b = 1; a := b", map[string]string{"__localq0__": "a"}},
	}
	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			c := NewCompiler()
			c.Compile(nil)
			assertNotFailed(t, c)
			qc := c.QueryCompiler()
			body, err := ParseBody(tc.q)
			if err != nil {
				t.Fatal(err)
			}
			_, err = qc.Compile(body)
			if err != nil {
				t.Fatal(err)
			}
			vars := qc.RewrittenVars()
			if len(tc.vars) != len(vars) {
				t.Fatalf("Expected %v but got: %v", tc.vars, vars)
			}
			for k := range vars {
				if vars[k] != Var(tc.vars[string(k)]) {
					t.Fatalf("Expected %v but got: %v", tc.vars, vars)
				}
			}
		})
	}
}

func TestQueryCompilerRecompile(t *testing.T) {

	// Query which contains terms that will be rewritten.
	parsed := MustParseBody(`a := [1]; data.bar == data.foo[a[0]]`)
	parsed0 := parsed

	qc := NewCompiler().QueryCompiler()
	compiled, err := qc.Compile(parsed)
	if err != nil {
		t.Fatal(err)
	}

	compiled2, err := qc.Compile(parsed)
	if err != nil {
		t.Fatal(err)
	}

	if !compiled2.Equal(compiled) {
		t.Fatalf("Expected same compiled query. Expected: %v, Got: %v", compiled, compiled2)
	}

	if !parsed0.Equal(parsed) {
		t.Fatalf("Expected parsed query to be unmodified. Expected %v, Got: %v", parsed0, parsed)
	}

}

func TestQueryCompilerWithMetrics(t *testing.T) {
	m := metrics.New()
	c := NewCompiler().WithMetrics(m)
	c.Compile(getCompilerTestModules())
	assertNotFailed(t, c)
	m.Clear()

	qc := c.QueryCompiler()

	query := MustParseBody("a = 1; a > 2")
	_, err := qc.Compile(query)
	if err != nil {
		t.Fatalf("Unexpected error from %v: %v", query, err)
	}

	if len(m.All()) == 0 {
		t.Error("Expected to have metrics after compiling")
	}
}

func TestQueryCompilerWithStageAfterWithMetrics(t *testing.T) {
	m := metrics.New()
	c := NewCompiler().WithMetrics(m)
	c.Compile(getCompilerTestModules())
	assertNotFailed(t, c)
	m.Clear()

	qc := c.QueryCompiler().WithStageAfter(
		"CheckSafety",
		QueryCompilerStageDefinition{
			"MockStage",
			"mock_stage",
			func(qc QueryCompiler, b Body) (Body, error) {
				return b, nil
			},
		})

	query := MustParseBody("a = 1; a > 2")
	_, err := qc.Compile(query)
	if err != nil {
		t.Fatalf("Unexpected error from %v: %v", query, err)
	}

	if len(m.All()) == 0 {
		t.Error("Expected to have metrics after compiling")
	}
}

func TestQueryCompilerWithUnsafeBuiltins(t *testing.T) {
	c := NewCompiler().WithUnsafeBuiltins(map[string]struct{}{
		"count": {},
	})

	_, err := c.QueryCompiler().WithUnsafeBuiltins(map[string]struct{}{}).Compile(MustParseBody("count([])"))
	if err != nil {
		t.Fatal(err)
	}
}

func assertCompilerErrorStrings(t *testing.T, compiler *Compiler, expected []string) {
	result := compilerErrsToStringSlice(compiler.Errors)

	if len(result) != len(expected) {
		t.Fatalf("Expected %d:\n%v\nBut got %d:\n%v", len(expected), strings.Join(expected, "\n"), len(result), strings.Join(result, "\n"))
	}
	for i := range result {
		if !strings.Contains(result[i], expected[i]) {
			t.Errorf("Expected %v but got: %v", expected[i], result[i])
		}
	}
}

func assertNotFailed(t *testing.T, c *Compiler) {
	if c.Failed() {
		t.Fatalf("Unexpected compilation error: %v", c.Errors)
	}
}

func mockStageFunctionCall(c *Compiler) *Error {
	return NewError(CompileErr, &Location{}, "mock stage error")
}

func mockStageFunctionCallNoErr(c *Compiler) *Error {
	return nil
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

// helper function to run compiler upto given stage. If nil is provided, a
// normal compile run is performed.
func compileStages(c *Compiler, upto func()) {

	c.init()

	for name := range c.Modules {
		c.sorted = append(c.sorted, name)
	}

	c.localvargen = newLocalVarGeneratorForModuleSet(c.sorted, c.Modules)

	sort.Strings(c.sorted)
	c.SetErrorLimit(0)

	if upto == nil {
		c.compile()
		return
	}

	target := reflect.ValueOf(upto)

	for _, s := range c.stages {
		if s.f(); c.Failed() {
			return
		}
		if reflect.ValueOf(s.f).Pointer() == target.Pointer() {
			break
		}
	}
}

func getCompilerTestModules() map[string]*Module {

	mod1 := MustParseModule(`package a.b.c

import data.x.y.z as foo
import data.g.h.k

p[x] { q[x]; not r[x] }
q[x] { foo[i] = x }
z = 400 { true }`,
	)

	mod2 := MustParseModule(`package a.b.c

import data.bar
import data.x.y.p

r[x] { bar[x] = 100; p = 101 }`)

	mod3 := MustParseModule(`package a.b.d

import input.x as y

t = true { input = {y.secret: [{y.keyid}]} }
x = false { true }`)

	mod4 := MustParseModule(`package a.b.empty`)

	mod5 := MustParseModule(`package a.b.compr

import input.x as y
import data.a.b.c.q

p = true { [y.a | true] }
r = true { [q.a | true] }
s = true { [true | y.a = 0] }
t = true { [true | q[i] = 1] }
u = true { [true | _ = [y.a | true]] }
v = true { [true | _ = [true | q[i] = 1]] }
`,
	)

	mod6 := MustParseModule(`package a.b.nested

import data.x
import data.z
import input.x as y

p = true { x[y[i].a[z.b[j]]] }
q = true { x = v; v[y[i]] }
r = 1 { true }
s = true { x[r] }`,
	)

	mod7 := MustParseModule(`package a.b.funcs

fn(x) = y {
	trim(x, ".", y)
}

bar([x, y]) = [a, [b, c]] {
	fn(x, a)
	y[1].b = b
	y[i].a = "hi"
	c = y[i].b
}

foorule = true {
	bar(["hi.there", [{"a": "hi", "b": 1}, {"a": "bye", "b": 0}]], [a, [b, c]])
}`)

	return map[string]*Module{
		"mod1": mod1,
		"mod2": mod2,
		"mod3": mod3,
		"mod4": mod4,
		"mod5": mod5,
		"mod6": mod6,
		"mod7": mod7,
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
	t.Run(note, func(t *testing.T) {
		c := NewCompiler().WithEnablePrintStatements(false)
		c.Compile(getCompilerTestModules())
		assertNotFailed(t, c)
		qc := c.QueryCompiler()
		query := MustParseBody(q)
		var qctx *QueryContext

		if pkg != "" {
			qctx = qctx.WithPackage(MustParsePackage(pkg))
		}
		if len(imports) != 0 {
			qctx = qctx.WithImports(MustParseImports(strings.Join(imports, "\n")))
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
			if !strings.Contains(err.Error(), expected.Error()) {
				t.Fatalf("Expected error %v but got: %v", expected, err)
			}
		}
	})
}

func TestCompilerCapabilitiesExtendedWithCustomBuiltins(t *testing.T) {

	compiler := NewCompiler().WithCapabilities(&Capabilities{
		Builtins: []*Builtin{
			{
				Name: "foo",
				Decl: types.NewFunction([]types.Type{types.N}, types.B),
			},
		},
	}).WithBuiltins(map[string]*Builtin{
		"bar": {
			Name: "bar",
			Decl: types.NewFunction([]types.Type{types.N}, types.B),
		},
	})

	module1 := MustParseModule(`package test

	p { foo(1); bar(2) }`)
	module2 := MustParseModule(`package test

	p { plus(1,2,x) }`)

	compiler.Compile(map[string]*Module{"x": module1})
	if compiler.Failed() {
		t.Fatal("unexpected error:", compiler.Errors)
	}

	compiler.Compile(map[string]*Module{"x": module2})
	if !compiler.Failed() {
		t.Fatal("expected error but got success")
	}

}

func TestCompilerWithUnsafeBuiltins(t *testing.T) {
	// Rego includes a number of built-in functions. In some cases, you may not
	// want all builtins to be available to a program. This test shows how to
	// mark a built-in as unsafe.
	compiler := NewCompiler().WithUnsafeBuiltins(map[string]struct{}{"re_match": {}})

	// This query should not compile because the `re_match` built-in is no
	// longer available.
	_, err := compiler.QueryCompiler().Compile(MustParseBody(`re_match("a", "a")`))
	if err == nil {
		t.Fatalf("Expected error for unsafe built-in")
	} else if !strings.Contains(err.Error(), "unsafe built-in function") {
		t.Fatalf("Expected error for unsafe built-in but got %v", err)
	}

	// These modules should not compile for the same reason.
	modules := map[string]*Module{"mod1": MustParseModule(`package a.b.c
deny {
    re_match(input.user, ".*bob.*")
}`)}
	compiler.Compile(modules)
	if !compiler.Failed() {
		t.Fatalf("Expected error for unsafe built-in")
	} else if !strings.Contains(compiler.Errors[0].Error(), "unsafe built-in function") {
		t.Fatalf("Expected error for unsafe built-in but got %v", err)
	}
}

func TestCompilerPassesTypeCheck(t *testing.T) {
	c := NewCompiler().
		WithCapabilities(&Capabilities{Builtins: []*Builtin{Split}})
	// Must compile to initialize type environment after WithCapabilities
	c.Compile(nil)
	if c.PassesTypeCheck(MustParseBody(`a = input.a; split(a, ":", x); a0 = x[0]; a0 = null`)) {
		t.Fatal("Did not successfully detect a type-checking violation")
	}
}

func TestCompilerPassesTypeCheckNegative(t *testing.T) {
	c := NewCompiler().
		WithCapabilities(&Capabilities{Builtins: []*Builtin{Split, StartsWith}})
	// Must compile to initialize type environment after WithCapabilities
	c.Compile(nil)
	if !c.PassesTypeCheck(MustParseBody(`a = input.a; split(a, ":", x); a0 = x[0]; startswith(a0, "foo", true)`)) {
		t.Fatal("Incorrectly detected a type-checking violation")
	}
}

func TestWithSchema(t *testing.T) {
	c := NewCompiler()
	schemaSet := NewSchemaSet()
	schemaSet.Put(SchemaRootRef, objectSchema)
	c.WithSchemas(schemaSet)
	if c.schemaSet == nil {
		t.Fatalf("WithSchema did not set the schema correctly in the compiler")
	}
}

func TestAnyOfObjectSchema1(t *testing.T) {
	c := NewCompiler()
	schemaSet := NewSchemaSet()
	schemaSet.Put(SchemaRootRef, anyOfExtendCoreSchema)
	c.WithSchemas(schemaSet)
	if c.schemaSet == nil {
		t.Fatalf("Did not correctly compile an object type schema with anyOf outside core schema")
	}
}

func TestAnyOfObjectSchema2(t *testing.T) {
	c := NewCompiler()
	schemaSet := NewSchemaSet()
	schemaSet.Put(SchemaRootRef, anyOfInsideCoreSchema)
	c.WithSchemas(schemaSet)
	if c.schemaSet == nil {
		t.Fatalf("Did not correctly compile an object type schema with anyOf inside core schema")
	}
}

func TestAnyOfArraySchema(t *testing.T) {
	c := NewCompiler()
	schemaSet := NewSchemaSet()
	schemaSet.Put(SchemaRootRef, anyOfArraySchema)
	c.WithSchemas(schemaSet)
	if c.schemaSet == nil {
		t.Fatalf("Did not correctly compile an array type schema with anyOf")
	}
}

func TestAnyOfObjectMissing(t *testing.T) {
	c := NewCompiler()
	schemaSet := NewSchemaSet()
	schemaSet.Put(SchemaRootRef, anyOfObjectMissing)
	c.WithSchemas(schemaSet)
	if c.schemaSet == nil {
		t.Fatalf("Did not correctly compile an object type schema with anyOf where one of the props did not explicitly claim type")
	}
}

func TestAnyOfArrayMissing(t *testing.T) {
	c := NewCompiler()
	schemaSet := NewSchemaSet()
	schemaSet.Put(SchemaRootRef, anyOfArrayMissing)
	c.WithSchemas(schemaSet)
	if c.schemaSet == nil {
		t.Fatalf("Did not correctly compile an array type schema with anyOf where items are inside anyOf")
	}
}

const objectSchema = `{
	"$schema": "http://json-schema.org/draft-07/schema",
	"$id": "http://example.com/example.json",
	"type": "object",
	"title": "The root schema",
	"description": "The root schema comprises the entire JSON document.",
	"required": [
		"foo",
		"b"
	],
	"properties": {
		"foo": {
			"$id": "#/properties/foo",
			"type": "string",
			"title": "The foo schema",
			"description": "An explanation about the purpose of this instance."
		},
		"b": {
			"$id": "#/properties/b",
			"type": "array",
			"title": "The b schema",
			"description": "An explanation about the purpose of this instance.",
			"additionalItems": false,
			"items": {
				"$id": "#/properties/b/items",
				"type": "object",
				"title": "The items schema",
				"description": "An explanation about the purpose of this instance.",
				"required": [
					"a",
					"b",
					"c"
				],
				"properties": {
					"a": {
						"$id": "#/properties/b/items/properties/a",
						"type": "integer",
						"title": "The a schema",
						"description": "An explanation about the purpose of this instance."
					},
					"b": {
						"$id": "#/properties/b/items/properties/b",
						"type": "array",
						"title": "The b schema",
						"description": "An explanation about the purpose of this instance.",
						"additionalItems": false,
						"items": {
							"$id": "#/properties/b/items/properties/b/items",
							"type": "integer",
							"title": "The items schema",
							"description": "An explanation about the purpose of this instance."
						}
					},
					"c": {
						"$id": "#/properties/b/items/properties/c",
						"type": "null",
						"title": "The c schema",
						"description": "An explanation about the purpose of this instance."
					}
				},
				"additionalProperties": false
			}
		}
	},
	"additionalProperties": false
}`

const arrayNoItemsSchema = `{
	"$schema": "http://json-schema.org/draft-07/schema",
	"$id": "http://example.com/example.json",
	"type": "object",
	"title": "The root schema",
	"description": "The root schema comprises the entire JSON document.",
	"required": [
		"b"
	],
	"properties": {
		"b": {
			"$id": "#/properties/b",
			"type": "array",
			"title": "The b schema",
			"description": "An explanation about the purpose of this instance.",
			"additionalItems": true
		}
	},
	"additionalProperties": false
}`

const noChildrenObjectSchema = `{
	"$schema": "http://json-schema.org/draft-07/schema",
	"$id": "http://example.com/example.json",
	"type": "object",
	"title": "The root schema",
	"description": "The root schema comprises the entire JSON document.",
	"additionalProperties": true
}`

const untypedFieldObjectSchema = `{
	"$schema": "http://json-schema.org/draft-07/schema",
	"$id": "http://example.com/example.json",
	"type": "object",
	"title": "The root schema",
	"description": "The root schema comprises the entire JSON document.",
	"required": [
		"foo"
	],
	"properties": {
		"foo": {
			"$id": "#/properties/foo"
		}
	},
	"additionalProperties": false
}`

const booleanSchema = `{
	"$schema": "http://json-schema.org/draft-07/schema",
	"$id": "http://example.com/example.json",
	"type": "object",
	"title": "The root schema",
	"description": "The root schema comprises the entire JSON document.",
	"required": [
		"a"
	],
	"properties": {
		"a": {
			"$id": "#/properties/foo",
			"type": "boolean",
			"title": "The foo schema",
			"description": "An explanation about the purpose of this instance."
		}
	},
	"additionalProperties": false
}`

const refSchema = `
{
    "description": "Pod is a collection of containers that can run on a host. This resource is created by clients and scheduled onto hosts.",
	"type": "object",
	"properties": {
      "apiVersion": {
        "description": "APIVersion defines the versioned schema of this representation of an object. Servers should convert recognized schemas to the latest internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#resources",
        "type": [
          "string",
          "null"
        ]
	  },

      "kind": {
        "description": "Kind is a string value representing the REST resource this object represents. Servers may infer this from the endpoint the client submits requests to. Cannot be updated. In CamelCase. More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#types-kinds",
        "type": [
          "string",
          "null"
        ],
        "enum": [
          "Pod"
        ]
      },
      "metadata": {
        "$ref": "https://kubernetesjsonschema.dev/v1.14.0/_definitions.json#/definitions/io.k8s.apimachinery.pkg.apis.meta.v1.ObjectMeta",
        "description": "Standard object's metadata. More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#metadata"
	  }
	}
}
`
const podSchema = `
{
    "description": "Pod is a collection of containers that can run on a host. This resource is created by clients and scheduled onto hosts.",
    "properties": {
      "apiVersion": {
        "description": "APIVersion defines the versioned schema of this representation of an object. Servers should convert recognized schemas to the latest internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#resources",
        "type": [
          "string",
          "null"
        ]
      },
      "kind": {
        "description": "Kind is a string value representing the REST resource this object represents. Servers may infer this from the endpoint the client submits requests to. Cannot be updated. In CamelCase. More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#types-kinds",
        "type": [
          "string",
          "null"
        ],
        "enum": [
          "Pod"
        ]
      },
      "metadata": {
        "$ref": "https://kubernetesjsonschema.dev/v1.14.0/_definitions.json#/definitions/io.k8s.apimachinery.pkg.apis.meta.v1.ObjectMeta",
        "description": "Standard object's metadata. More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#metadata"
      },
      "spec": {
        "$ref": "https://kubernetesjsonschema.dev/v1.14.0/_definitions.json#/definitions/io.k8s.api.core.v1.PodSpec",
        "description": "Specification of the desired behavior of the pod. More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#spec-and-status"
      },
      "status": {
        "$ref": "https://kubernetesjsonschema.dev/v1.14.0/_definitions.json#/definitions/io.k8s.api.core.v1.PodStatus",
        "description": "Most recently observed status of the pod. This data may not be up to date. Populated by the system. Read-only. More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#spec-and-status"
      }
    },
    "type": "object",
    "x-kubernetes-group-version-kind": [
      {
        "group": "",
        "kind": "Pod",
        "version": "v1"
      }
    ],
    "$schema": "http://json-schema.org/schema#"
  }`

const anyOfArraySchema = `{
	"type": "object",
	"properties": {
		"familyMembers": {
			"type": "array",
			"items": {
				"anyOf": [
					{
						"type": "object",
						"properties": {
							"age": { "type": "integer" },
							"name": {"type": "string"}
						}
					},{
						"type": "object",
						"properties": {
							"personality": { "type": "string" },
							"nickname": { "type": "string"  }
						}
					}
				]
			}
		}
	}
}`

const anyOfExtendCoreSchema = `{
	"type": "object",
	"properties": {
		"AddressLine": { "type": "string" }
	},
	"anyOf": [
		{
			"type": "object",
			"properties": {
				"State":   { "type": "string" },
				"ZipCode": { "type": "string" }
			}
		},
		{
			"type": "object",
			"properties": {
				"County":   { "type": "string" },
				"PostCode": { "type": "integer" }
			}
		}
	]
}`

const allOfObjectSchema = `{
    "$schema": "http://json-schema.org/draft-04/schema#",
    "type": "object",
    "title": "My schema",
    "properties": {
        "AddressLine1": { "type": "string" },
        "AddressLine2": { "type": "string" },
        "City":         { "type": "string" }
    },
    "allOf": [
        {
            "type": "object",
            "properties": {
                "State":   { "type": "string" },
                "ZipCode": { "type": "string" }
            },
        },
        {
            "type": "object",
            "properties": {
                "County":   { "type": "string" },
                "PostCode": { "type": "string" }
            },
        }
    ]
}`

const allOfArraySchema = `{
	"$schema": "http://json-schema.org/draft-04/schema#",
	"type": "array",
	"title": "The b schema",
	"description": "An explanation about the purpose of this instance.",
	"items": {
		"type": "integer",
		"title": "The items schema",
		"description": "An explanation about the purpose of this instance."
	},
	"allOf": [
		{
		"type": "array",
		"title": "The b schema",
		"description": "An explanation about the purpose of this instance.",
		"items": {
			"type": "integer",
			"title": "The items schema",
			"description": "An explanation about the purpose of this instance."
		}
		},
		{
		"type": "array",
		"title": "The b schema",
		"description": "An explanation about the purpose of this instance.",
		"items": {
			"type": "integer",
			"title": "The items schema",
			"description": "An explanation about the purpose of this instance."
		}
		}
	]
}`

const allOfSchemaParentVariation = `{
    "$schema": "http://json-schema.org/draft-04/schema#",
    "allOf": [
        {
            "type": "object",
            "properties": {
                "State":   { "type": "string" },
                "ZipCode": { "type": "string" }
            },
        },
        {
            "type": "object",
            "properties": {
                "County":   { "type": "string" },
                "PostCode": { "type": "string" }
            },
        }
    ]
}`

const emptySchema = `{
	"allof" : []
 }`

const allOfArrayOfArrays = `{
	"$schema": "http://json-schema.org/draft-04/schema#",
	"type": "array",
	"title": "The b schema",
	"description": "An explanation about the purpose of this instance.",
	"items": {
		"type": "array",
		"title": "The items schema",
		"description": "An explanation about the purpose of this instance.",
		"items": {
			"type": "integer",
			"title": "The items schema",
			"description": "An explanation about the purpose of this instance."
		}
	},
	"allOf": [{
			"type": "array",
			"title": "The b schema",
			"description": "An explanation about the purpose of this instance.",
			"items": {
				"type": "array",
				"title": "The items schema",
				"description": "An explanation about the purpose of this instance.",
				"items": {
					"type": "integer",
					"title": "The items schema",
					"description": "An explanation about the purpose of this instance."
				}
			}
		},
		{
			"type": "array",
			"title": "The b schema",
			"description": "An explanation about the purpose of this instance.",
			"items": {
				"type": "integer",
				"title": "The items schema",
				"description": "An explanation about the purpose of this instance."
			}
		}
	]
}`

const anyOfInsideCoreSchema = ` {
	"type": "object",
	"properties": {
		"AddressLine": { "type": "string" },
		"RandomInfo": {
			"anyOf": [
				{ "type": "object",
				  "properties": {
					  "accessMe": {"type": "string"}
				  }
				},
				{ "type": "number", "minimum": 0 }
			  ]
		}
	}
}`

const anyOfObjectMissing = `{
	"type": "object",
	"properties": {
		"AddressLine": { "type": "string" }
	},
	"anyOf": [
		{
			"type": "object",
			"properties": {
				"State":   { "type": "string" },
				"ZipCode": { "type": "string" }
			}
		},
		{
			"properties": {
				"County":   { "type": "string" },
				"PostCode": { "type": "integer" }
			}
		}
	]
}`

const allOfArrayOfObjects = `{
	"$schema": "http://json-schema.org/draft-04/schema#",
	"type": "array",
	"title": "The b schema",
	"description": "An explanation about the purpose of this instance.",
	"items": {
		"type": "object",
		"title": "The items schema",
		"description": "An explanation about the purpose of this instance.",
		"properties": {
			"State": {
				"type": "string"
			},
			"ZipCode": {
				"type": "string"
			}
		},
		"allOf": [{
				"type": "object",
				"title": "The b schema",
				"description": "An explanation about the purpose of this instance.",
				"properties": {
					"County": {
						"type": "string"
					},
					"PostCode": {
						"type": "string"
					}
				}
			},
			{
				"type": "object",
				"title": "The b schema",
				"description": "An explanation about the purpose of this instance.",
				"properties": {
					"Street": {
						"type": "string"
					},
					"House": {
						"type": "string"
					}
				}
			}
		]
	}
}`

const allOfObjectAndArray = `{
	"$schema": "http://json-schema.org/draft-04/schema#",
	"type": "object",
	"title": "My schema",
	"properties": {
		"AddressLine1": {
			"type": "string"
		},
		"AddressLine2": {
			"type": "string"
		},
		"City": {
			"type": "string"
		}
	},
	"allOf": [{
			"type": "object",
			"properties": {
				"State": {
					"type": "string"
				},
				"ZipCode": {
					"type": "string"
				}
			}
		},
		{
			"type": "array",
			"title": "The b schema",
			"description": "An explanation about the purpose of this instance.",
			"items": {
				"type": "integer",
				"title": "The items schema",
				"description": "An explanation about the purpose of this instance."
			}
		}
	]
}`

const allOfObjectMissing = `{
	"type": "object",
	"properties": {
		"AddressLine": { "type": "string" }
	},
	"allOf": [
		{
			"type": "object",
			"properties": {
				"State":   { "type": "string" },
				"ZipCode": { "type": "string" }
			}
		},
		{
			"properties": {
				"County":   { "type": "string" },
				"PostCode": { "type": "integer" }
			}
		}
	]
}`

const allOfArrayDifTypes = `{
	"$schema": "http://json-schema.org/draft-04/schema#",
	"type": "array",
	"title": "The b schema",
	"description": "An explanation about the purpose of this instance.",
	"allOf": [{
			"type": "array",
			"items": [{
					"type": "string"
				},
				{
					"type": "integer"
				}
			]
		},
		{
			"type": "array",
			"items": [{
					"type": "string"
				},
				{
					"type": "integer"
				}
			]
		}
	]
}`

const allOfArrayInsideObject = `{
	"type": "object",
	"properties": {
		"familyMembers": {
			"type": "array",
			"items": {
				"allOf": [{
					"type": "object",
					"properties": {
						"age": {
							"type": "integer"
						},
						"name": {
							"type": "string"
						}
					}
				}, {
					"type": "object",
					"properties": {
						"personality": {
							"type": "string"
						},
						"nickname": {
							"type": "string"
						}
					}
				}]
			}
		}
	}
}`

const anyOfArrayMissing = `{
	"type": "array",
	"anyOf": [
		{
			"items": [
				{"type": "number"},
				{"type": "string"}]
            },
		{	"items": [
				{"type": "integer"}]
		}
	]
}`

const allOfArrayMissing = `{
	"type": "array",
	"allOf": [{
			"items": [{
					"type": "integer"
				},
				{
					"type": "integer"
				}
			]
		},
		{
			"items": [{
				"type": "integer"
			}]
		}
	]
}`

const anyOfSchemaParentVariation = `{
    "$schema": "http://json-schema.org/draft-04/schema#",
    "anyOf": [
        {
            "type": "object",
            "properties": {
                "State":   { "type": "string" },
                "ZipCode": { "type": "string" }
            },
        },
        {
            "type": "object",
            "properties": {
                "County":   { "type": "string" },
                "PostCode": { "type": "string" }
            },
        }
    ]
	}
}`

const allOfInsideCoreSchema = `{
	"type": "object",
	"properties": {
		"AddressLine": { "type": "string" },
		"RandomInfo": {
			"allOf": [
				{ "type": "object",
				  "properties": {
					  "accessMe": {"type": "string"}
				  }
				},
				{ "type": "object",
					"properties": {
						"accessYou": {"type": "string"}
					}}
			  ]
		}
	}
}`

const allOfArrayDifTypesWithError = `{
	"$schema": "http://json-schema.org/draft-04/schema#",
	"type": "array",
	"title": "The b schema",
	"description": "An explanation about the purpose of this instance.",
	"allOf": [{
			"type": "array",
			"items": [{
					"type": "string"
				},
				{
					"type": "integer"
				}
			]
		},
		{
			"type": "array",
			"items": [{
					"type": "boolean"
				},
				{
					"type": "integer"
				}
			]
		}
	]
}`

const allOfStringSchema = `{
	"$schema": "http://json-schema.org/draft-04/schema#",
	"type": "string",
	"title": "The b schema",
	"description": "An explanation about the purpose of this instance.",
	"allOf": [{
			"type": "string",
		},
		{
			"type": "string",
		}
	]
}`

const allOfIntegerSchema = `{
	"$schema": "http://json-schema.org/draft-04/schema#",
	"type": "integer",
	"title": "The b schema",
	"description": "An explanation about the purpose of this instance.",
	"allOf": [{
			"type": "integer",
		},
		{
			"type": "integer",
		}
	]
}`

const allOfBooleanSchema = `{
	"$schema": "http://json-schema.org/draft-04/schema#",
	"type": "boolean",
	"title": "The b schema",
	"description": "An explanation about the purpose of this instance.",
	"allOf": [{
			"type": "boolean",
		},
		{
			"type": "boolean",
		}
	]
}`

const allOfTypeErrorSchema = `{
	"$schema": "http://json-schema.org/draft-04/schema#",
	"type": "string",
	"title": "The b schema",
	"description": "An explanation about the purpose of this instance.",
	"allOf": [{
			"type": "string",
		},
		{
			"type": "integer",
		}
	]
}`

const allOfStringSchemaWithError = `{
	"$schema": "http://json-schema.org/draft-04/schema#",
	"type": "string",
	"title": "The b schema",
	"description": "An explanation about the purpose of this instance.",
	"allOf": [{
			"type": "string",
		},
		{
			"type": "string",
		},
		{
			"type": "boolean",
		}
	]
}`

const allOfSchemaWithParentError = `{
	"$schema": "http://json-schema.org/draft-04/schema#",
	"type": "string",
	"title": "The b schema",
	"description": "An explanation about the purpose of this instance.",
	"allOf": [{
			"type": "integer",
		},
		{
			"type": "integer",
		}
	]
}`

const allOfSchemaWithUnevenArray = `{
	"type": "array",
	"allOf": [{
			"items": [{
					"type": "integer"
				},
				{
					"type": "integer"
				}
			]
		},
		{
			"items": [{
				"type": "integer"
			},
			{
				"type": "integer"
			},
			{
				"type": "string"
			}]
		}
	]
}`
