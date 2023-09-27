// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"bytes"
	"encoding/json"
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
		{
			note:  "every: simple: no output vars",
			query: `every k, v in [1, 2] { k < v }`,
			exp:   `set()`,
		},
		{
			note:  "every: output vars in domain",
			query: `xs = []; every k, v in xs[i] { k < v }`,
			exp:   `{xs, i}`,
		},
		{
			note:  "every: output vars in body",
			query: `every k, v in [] { k < v; i = 1 }`,
			exp:   `set()`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {

			opts := ParserOptions{AllFutureKeywords: true, unreleasedKeywords: true}
			body, err := ParseBodyWithOpts(tc.query, opts)
			if err != nil {
				t.Fatal(err)
			}
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

	mods := getCompilerTestModules() // 7 modules
	mods["system-mod"] = MustParseModule(`
	package system.foo

	p = 1
	`)
	mods["non-system-mod"] = MustParseModule(`
	package user.system

	p = 1
	`)
	mods["dots-in-heads"] = MustParseModule(`
	package dots

	a.b.c = 12
	d.e.f.g = 34
	`)
	tree := NewModuleTree(mods)
	expectedSize := 10

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
func TestCompilerGetExports(t *testing.T) {
	tests := []struct {
		note    string
		modules []*Module
		exports map[string][]string
	}{
		{
			note: "simple",
			modules: modules(`package p
				r = 1`),
			exports: map[string][]string{"data.p": {"r"}},
		},
		{
			note: "simple single-value ref rule",
			modules: modules(`package p
				q.r.s = 1`),
			exports: map[string][]string{"data.p": {"q.r.s"}},
		},
		{
			note: "var key single-value ref rule",
			modules: modules(`package p
				q.r[s] = 1 { s := "foo" }`),
			exports: map[string][]string{"data.p": {"q.r"}},
		},
		{
			note: "simple multi-value ref rule",
			modules: modules(`package p
				import future.keywords

				q.r.s contains 1 { true }`),
			exports: map[string][]string{"data.p": {"q.r.s"}},
		},
		{
			note: "two simple, multiple rules",
			modules: modules(`package p
				r = 1
				s = 11`,
				`package q
				x = 2
				y = 22`),
			exports: map[string][]string{"data.p": {"r", "s"}, "data.q": {"x", "y"}},
		},
		{
			note: "ref head + simple, multiple rules",
			modules: modules(`package p.a.b.c
				r = 1
				s = 11`,
				`package q
				a.b.x = 2
				a.b.c.y = 22`),
			exports: map[string][]string{
				"data.p.a.b.c": {"r", "s"},
				"data.q":       {"a.b.x", "a.b.c.y"},
			},
		},
		{
			note: "two ref head, multiple rules",
			modules: modules(`package p.a.b.c
				r = 1
				s = 11`,
				`package p
				a.b.x = 2
				a.b.c.y = 22`),
			exports: map[string][]string{
				"data.p.a.b.c": {"r", "s"},
				"data.p":       {"a.b.x", "a.b.c.y"},
			},
		},
		{
			note: "single-value rule with number key",
			modules: modules(`package p
				q[1] = 1
				q[2] = 2`),
			exports: map[string][]string{
				"data.p": {"q[1]", "q[2]"}, // TODO(sr): is this really what we want?
			},
		},
		{
			note: "single-value (ref) rule with number key",
			modules: modules(`package p
				a.b.q[1] = 1
				a.b.q[2] = 2`),
			exports: map[string][]string{
				"data.p": {"a.b.q[1]", "a.b.q[2]"},
			},
		},
		{
			note: "single-value (ref) rule with var key",
			modules: modules(`package p
				a.b.q[x] = y { x := 1; y := true }
				a.b.q[2] = 2`),
			exports: map[string][]string{
				"data.p": {"a.b.q", "a.b.q[2]"}, // TODO(sr): GroundPrefix? right thing here?
			},
		},
		{ // NOTE(sr): An ast.Module can be constructed in various ways, this is to assert that
			//         our compilation process doesn't explode here if we're fed a Rule that has no Ref.
			note: "synthetic",
			modules: func() []*Module {
				ms := modules(`package p
				r = 1`)
				ms[0].Rules[0].Head.Reference = nil
				return ms
			}(),
			exports: map[string][]string{"data.p": {"r"}},
		},
		// TODO(sr): add multi-val rule, and ref-with-var single-value rule.
	}

	hashMap := func(ms map[string][]string) *util.HashMap {
		rules := util.NewHashMap(func(a, b util.T) bool {
			switch a := a.(type) {
			case Ref:
				return a.Equal(b.(Ref))
			case []Ref:
				b := b.([]Ref)
				if len(b) != len(a) {
					return false
				}
				for i := range a {
					if !a[i].Equal(b[i]) {
						return false
					}
				}
				return true
			default:
				panic("unreachable")
			}
		}, func(v util.T) int {
			return v.(Ref).Hash()
		})
		for r, rs := range ms {
			refs := make([]Ref, len(rs))
			for i := range rs {
				refs[i] = toRef(rs[i])
			}
			rules.Put(MustParseRef(r), refs)
		}
		return rules
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			c := NewCompiler()
			for i, m := range tc.modules {
				c.Modules[fmt.Sprint(i)] = m
				c.sorted = append(c.sorted, fmt.Sprint(i))
			}
			if exp, act := hashMap(tc.exports), c.getExports(); !exp.Equal(act) {
				t.Errorf("expected %v, got %v", exp, act)
			}
		})
	}
}

func toRef(s string) Ref {
	switch t := MustParseTerm(s).Value.(type) {
	case Var:
		return Ref{NewTerm(t)}
	case Ref:
		return t
	default:
		panic("unreachable")
	}
}

func TestCompilerCheckRuleHeadRefs(t *testing.T) {
	tests := []struct {
		note     string
		modules  []*Module
		expected *Rule
		err      string
	}{
		{
			note: "ref contains var",
			modules: modules(
				`package x
				p.q[i].r = 1 { i := 10 }`,
			),
		},
		{
			note: "valid: ref is single-value rule with var key",
			modules: modules(
				`package x
				p.q.r[i] { i := 10 }`,
			),
		},
		{
			note: "valid: ref is single-value rule with var key and value",
			modules: modules(
				`package x
				p.q.r[i] = j { i := 10; j := 11 }`,
			),
		},
		{
			note: "valid: ref is single-value rule with var key and static value",
			modules: modules(
				`package x
				p.q.r[i] = "ten" { i := 10 }`,
			),
		},
		{
			note: "valid: ref is single-value rule with number key",
			modules: modules(
				`package x
				p.q.r[1] { true }`,
			),
		},
		{
			note: "valid: ref is single-value rule with boolean key",
			modules: modules(
				`package x
				p.q.r[true] { true }`,
			),
		},
		{
			note: "valid: ref is single-value rule with null key",
			modules: modules(
				`package x
				p.q.r[null] { true }`,
			),
		},
		{
			note: "valid: ref is single-value rule with set literal key",
			modules: modules(
				`package x
				p.q.r[set()] { true }`,
			),
		},
		{
			note: "valid: ref is single-value rule with array literal key",
			modules: modules(
				`package x
				p.q.r[[]] { true }`,
			),
		},
		{
			note: "valid: ref is single-value rule with object literal key",
			modules: modules(
				`package x
				p.q.r[{}] { true }`,
			),
		},
		{
			note: "valid: ref is single-value rule with ref key",
			modules: modules(
				`package x
				x := [1,2,3]
				p.q.r[x[i]] { i := 0}`,
			),
		},
		{
			note: "invalid: ref in ref",
			modules: modules(
				`package x
				p.q[arr[0]].r { i := 10 }`,
			),
		},
		{
			note: "invalid: non-string in ref (not last position)",
			modules: modules(
				`package x
				p.q[10].r { true }`,
			),
		},
		{
			note: "valid: multi-value with var key",
			modules: modules(
				`package x
				p.q.r contains i if i := 10`,
			),
		},
		{
			note: "rewrite: single-value with non-var key (ref)",
			modules: modules(
				`package x
				p.q.r[y.z] if y := {"z": "a"}`,
			),
			expected: MustParseRule(`p.q.r[__local0__]  { y := {"z": "a"}; __local0__ = y.z }`),
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			mods := make(map[string]*Module, len(tc.modules))
			for i, m := range tc.modules {
				mods[fmt.Sprint(i)] = m
			}
			c := NewCompiler()
			c.Modules = mods
			compileStages(c, c.rewriteRuleHeadRefs)
			if tc.err != "" {
				assertCompilerErrorStrings(t, c, []string{tc.err})
			} else {
				if len(c.Errors) > 0 {
					t.Fatalf("expected no errors, got %v", c.Errors)
				}
				if tc.expected != nil {
					assertRulesEqual(t, tc.expected, mods["0"].Rules[0])
				}
			}
		})
	}
}

func TestRuleTreeWithDotsInHeads(t *testing.T) {

	// TODO(sr): multi-val with var key in ref
	tests := []struct {
		note    string
		modules []*Module
		size    int // expected tree size = number of leaves
		depth   int // expected tree depth
	}{
		{
			note: "two modules, same package, one rule each",
			modules: modules(
				`package x
				p.q.r = 1`,
				`package x
				p.q.w = 2`,
			),
			size: 2,
		},
		{
			note: "two modules, sub-package, one rule each",
			modules: modules(
				`package x
				p.q.r = 1`,
				`package x.p
				q.w.z = 2`,
			),
			size: 2,
		},
		{
			note: "three modules, sub-package, incl simple rule",
			modules: modules(
				`package x
				p.q.r = 1`,
				`package x.p
				q.w.z = 2`,
				`package x.p.q.w
				y = 3`,
			),
			size: 3,
		},
		{
			note: "simple: two modules",
			modules: modules(
				`package x
				p.q.r = 1`,
				`package y
				p.q.w = 2`,
			),
			size: 2,
		},
		{
			note: "conflict: one module",
			modules: modules(
				`package q
				p[x] = 1
				p = 2`,
			),
			size: 2,
		},
		{
			note: "conflict: two modules",
			modules: modules(
				`package q
				p.r.s[x] = 1`,
				`package q.p
				r.s = 2 if true`,
			),
			size: 2,
		},
		{
			note: "simple: two modules, one using ref head, one package path",
			modules: modules(
				`package x
				p.q.r = 1 { input == 1 }`,
				`package x.p.q
				r = 2 { input == 2 }`,
			),
			size: 2,
		},
		{
			note: "conflict: two modules, both using ref head, different package paths",
			modules: modules(
				`package x
				p.q.r = 1 { input == 1 }`, // x.p.q.r = 1
				`package x.p
				q.r.s = 2 { input == 2 }`, // x.p.q.r.s = 2
			),
			size: 2,
		},
		{
			note: "overlapping: one module, two ref head",
			modules: modules(
				`package x
				p.q.r = 1
				p.q.w.v = 2`,
			),
			size:  2,
			depth: 6,
		},
		{
			note: "last ref term != string",
			modules: modules(
				`package x
				p.q.w[1] = 2
				p.q.w[{"foo": "baz"}] = 20
				p.q.x[true] = false
				p.q.x[y] = y { y := "y" }`,
			),
			size:  4,
			depth: 6,
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			c := NewCompiler()
			for i, m := range tc.modules {
				c.Modules[fmt.Sprint(i)] = m
				c.sorted = append(c.sorted, fmt.Sprint(i))
			}
			compileStages(c, c.setRuleTree)
			if len(c.Errors) > 0 {
				t.Fatal(c.Errors)
			}
			tree := c.RuleTree
			tree.DepthFirst(func(n *TreeNode) bool {
				t.Log(n)
				if !sort.SliceIsSorted(n.Sorted, func(i, j int) bool {
					return n.Sorted[i].Compare(n.Sorted[j]) < 0
				}) {
					t.Errorf("expected sorted to be sorted: %v", n.Sorted)
				}
				return false
			})
			if tc.depth > 0 {
				if exp, act := tc.depth, depth(tree); exp != act {
					t.Errorf("expected tree depth %d, got %d", exp, act)
				}
			}
			if exp, act := tc.size, tree.Size(); exp != act {
				t.Errorf("expected tree size %d, got %d", exp, act)
			}
		})
	}
}

func TestRuleTreeWithVars(t *testing.T) {
	opts := ParserOptions{AllFutureKeywords: true, unreleasedKeywords: true}

	t.Run("simple single-value rule", func(t *testing.T) {
		mod0 := `package a.b
c.d.e = 1 if true`

		mods := map[string]*Module{"0.rego": MustParseModuleWithOpts(mod0, opts)}
		tree := NewRuleTree(NewModuleTree(mods))

		node := tree.Find(MustParseRef("data.a.b.c.d.e"))
		if node == nil {
			t.Fatal("expected non-nil leaf node")
		}
		if exp, act := 1, len(node.Values); exp != act {
			t.Errorf("expected %d values, found %d", exp, act)
		}
		if exp, act := 0, len(node.Children); exp != act {
			t.Errorf("expected %d children, found %d", exp, act)
		}
		if exp, act := MustParseRef("c.d.e"), node.Values[0].(*Rule).Head.Ref(); !exp.Equal(act) {
			t.Errorf("expected rule ref %v, found %v", exp, act)
		}
	})

	t.Run("two single-value rules", func(t *testing.T) {
		mod0 := `package a.b
c.d.e = 1 if true`
		mod1 := `package a.b.c
d.e = 2 if true`

		mods := map[string]*Module{
			"0.rego": MustParseModuleWithOpts(mod0, opts),
			"1.rego": MustParseModuleWithOpts(mod1, opts),
		}
		tree := NewRuleTree(NewModuleTree(mods))

		node := tree.Find(MustParseRef("data.a.b.c.d.e"))
		if node == nil {
			t.Fatal("expected non-nil leaf node")
		}
		if exp, act := 2, len(node.Values); exp != act {
			t.Errorf("expected %d values, found %d", exp, act)
		}
		if exp, act := 0, len(node.Children); exp != act {
			t.Errorf("expected %d children, found %d", exp, act)
		}
		if exp, act := MustParseRef("c.d.e"), node.Values[0].(*Rule).Head.Ref(); !exp.Equal(act) {
			t.Errorf("expected rule ref %v, found %v", exp, act)
		}
		if exp, act := MustParseRef("d.e"), node.Values[1].(*Rule).Head.Ref(); !exp.Equal(act) {
			t.Errorf("expected rule ref %v, found %v", exp, act)
		}
	})

	t.Run("one multi-value rule, one single-value, with var", func(t *testing.T) {
		mod0 := `package a.b
c.d.e.g contains 1 if true`
		mod1 := `package a.b.c
d.e.f = 2 if true`

		mods := map[string]*Module{
			"0.rego": MustParseModuleWithOpts(mod0, opts),
			"1.rego": MustParseModuleWithOpts(mod1, opts),
		}
		tree := NewRuleTree(NewModuleTree(mods))

		// var-key rules should be included in the results
		node := tree.Find(MustParseRef("data.a.b.c.d.e.g"))
		if node == nil {
			t.Fatal("expected non-nil leaf node")
		}
		if exp, act := 1, len(node.Values); exp != act {
			t.Fatalf("expected %d values, found %d", exp, act)
		}
		if exp, act := 0, len(node.Children); exp != act {
			t.Fatalf("expected %d children, found %d", exp, act)
		}
		node = tree.Find(MustParseRef("data.a.b.c.d.e.f"))
		if node == nil {
			t.Fatal("expected non-nil leaf node")
		}
		if exp, act := 1, len(node.Values); exp != act {
			t.Fatalf("expected %d values, found %d", exp, act)
		}
		if exp, act := MustParseRef("d.e.f"), node.Values[0].(*Rule).Head.Ref(); !exp.Equal(act) {
			t.Errorf("expected rule ref %v, found %v", exp, act)
		}
	})

	t.Run("two multi-value rules, back compat", func(t *testing.T) {
		mod0 := `package a
b[c] { c := "foo" }`
		mod1 := `package a
b[d] { d := "bar" }`

		mods := map[string]*Module{
			"0.rego": MustParseModuleWithOpts(mod0, opts),
			"1.rego": MustParseModuleWithOpts(mod1, opts),
		}
		tree := NewRuleTree(NewModuleTree(mods))

		node := tree.Find(MustParseRef("data.a.b"))
		if node == nil {
			t.Fatal("expected non-nil leaf node")
		}
		if exp, act := 2, len(node.Values); exp != act {
			t.Fatalf("expected %d values, found %d: %v", exp, act, node.Values)
		}
		if exp, act := 0, len(node.Children); exp != act {
			t.Errorf("expected %d children, found %d", exp, act)
		}
		if exp, act := (Ref{VarTerm("b")}), node.Values[0].(*Rule).Head.Ref(); !exp.Equal(act) {
			t.Errorf("expected rule ref %v, found %v", exp, act)
		}
		if act := node.Values[0].(*Rule).Head.Value; act != nil {
			t.Errorf("expected rule value nil, found %v", act)
		}
		if exp, act := VarTerm("c"), node.Values[0].(*Rule).Head.Key; !exp.Equal(act) {
			t.Errorf("expected rule key %v, found %v", exp, act)
		}
		if exp, act := (Ref{VarTerm("b")}), node.Values[1].(*Rule).Head.Ref(); !exp.Equal(act) {
			t.Errorf("expected rule ref %v, found %v", exp, act)
		}
		if act := node.Values[1].(*Rule).Head.Value; act != nil {
			t.Errorf("expected rule value nil, found %v", act)
		}
		if exp, act := VarTerm("d"), node.Values[1].(*Rule).Head.Key; !exp.Equal(act) {
			t.Errorf("expected rule key %v, found %v", exp, act)
		}
	})

	t.Run("two multi-value rules, back compat with short style", func(t *testing.T) {
		mod0 := `package a
b[1]`
		mod1 := `package a
b[2]`
		mods := map[string]*Module{
			"0.rego": MustParseModuleWithOpts(mod0, opts),
			"1.rego": MustParseModuleWithOpts(mod1, opts),
		}
		tree := NewRuleTree(NewModuleTree(mods))

		node := tree.Find(MustParseRef("data.a.b"))
		if node == nil {
			t.Fatal("expected non-nil leaf node")
		}
		if exp, act := 2, len(node.Values); exp != act {
			t.Fatalf("expected %d values, found %d: %v", exp, act, node.Values)
		}
		if exp, act := 0, len(node.Children); exp != act {
			t.Errorf("expected %d children, found %d", exp, act)
		}
		if exp, act := (Ref{VarTerm("b")}), node.Values[0].(*Rule).Head.Ref(); !exp.Equal(act) {
			t.Errorf("expected rule ref %v, found %v", exp, act)
		}
		if act := node.Values[0].(*Rule).Head.Value; act != nil {
			t.Errorf("expected rule value nil, found %v", act)
		}
		if exp, act := IntNumberTerm(1), node.Values[0].(*Rule).Head.Key; !exp.Equal(act) {
			t.Errorf("expected rule key %v, found %v", exp, act)
		}
		if exp, act := (Ref{VarTerm("b")}), node.Values[1].(*Rule).Head.Ref(); !exp.Equal(act) {
			t.Errorf("expected rule ref %v, found %v", exp, act)
		}
		if act := node.Values[1].(*Rule).Head.Value; act != nil {
			t.Errorf("expected rule value nil, found %v", act)
		}
		if exp, act := IntNumberTerm(2), node.Values[1].(*Rule).Head.Key; !exp.Equal(act) {
			t.Errorf("expected rule key %v, found %v", exp, act)
		}
	})

	t.Run("two single-value rules, back compat with short style", func(t *testing.T) {
		mod0 := `package a
b[1] = 1`
		mod1 := `package a
b[2] = 2`
		mods := map[string]*Module{
			"0.rego": MustParseModuleWithOpts(mod0, opts),
			"1.rego": MustParseModuleWithOpts(mod1, opts),
		}
		tree := NewRuleTree(NewModuleTree(mods))

		// branch point
		node := tree.Find(MustParseRef("data.a.b"))
		if node == nil {
			t.Fatal("expected non-nil leaf node")
		}
		if exp, act := 0, len(node.Values); exp != act {
			t.Fatalf("expected %d values, found %d: %v", exp, act, node.Values)
		}
		if exp, act := 2, len(node.Children); exp != act {
			t.Fatalf("expected %d children, found %d", exp, act)
		}

		// branch 1
		node = tree.Find(MustParseRef("data.a.b[1]"))
		if node == nil {
			t.Fatal("expected non-nil leaf node")
		}
		if exp, act := 1, len(node.Values); exp != act {
			t.Fatalf("expected %d values, found %d: %v", exp, act, node.Values)
		}
		if exp, act := MustParseRef("b[1]"), node.Values[0].(*Rule).Head.Ref(); !exp.Equal(act) {
			t.Errorf("expected rule ref %v, found %v", exp, act)
		}
		if exp, act := IntNumberTerm(1), node.Values[0].(*Rule).Head.Value; !exp.Equal(act) {
			t.Errorf("expected rule value %v, found %v", exp, act)
		}
		if exp, act := IntNumberTerm(1), node.Values[0].(*Rule).Head.Key; !exp.Equal(act) {
			t.Errorf("expected rule key %v, found %v", exp, act)
		}

		// branch 2
		node = tree.Find(MustParseRef("data.a.b[2]"))
		if node == nil {
			t.Fatal("expected non-nil leaf node")
		}
		if exp, act := 1, len(node.Values); exp != act {
			t.Fatalf("expected %d values, found %d: %v", exp, act, node.Values)
		}
		if exp, act := MustParseRef("b[2]"), node.Values[0].(*Rule).Head.Ref(); !exp.Equal(act) {
			t.Errorf("expected rule ref %v, found %v", exp, act)
		}
		if exp, act := IntNumberTerm(2), node.Values[0].(*Rule).Head.Value; !exp.Equal(act) {
			t.Errorf("expected rule value %v, found %v", exp, act)
		}
		if exp, act := IntNumberTerm(2), node.Values[0].(*Rule).Head.Key; !exp.Equal(act) {
			t.Errorf("expected rule key %v, found %v", exp, act)
		}
	})

	// NOTE(sr): Now this test seems obvious, but it's a bug that had snuck into the
	// NewRuleTree code during development.
	t.Run("root node and data node unhidden if there are no system nodes", func(t *testing.T) {
		mod0 := `package a
p = 1`
		mods := map[string]*Module{
			"0.rego": MustParseModuleWithOpts(mod0, opts),
		}
		tree := NewRuleTree(NewModuleTree(mods))

		if exp, act := false, tree.Hide; act != exp {
			t.Errorf("expected tree.Hide=%v, got %v", exp, act)
		}
		dataNode := tree.Child(Var("data"))
		if dataNode == nil {
			t.Fatal("expected data node")
		}
		if exp, act := false, dataNode.Hide; act != exp {
			t.Errorf("expected dataNode.Hide=%v, got %v", exp, act)
		}
	})
}

func depth(n *TreeNode) int {
	d := -1
	for _, m := range n.Children {
		if d0 := depth(m); d0 > d {
			d = d0
		}
	}
	return d + 1
}

func TestModuleTreeFilenameOrder(t *testing.T) {
	// NOTE(sr): It doesn't matter that these are conflicting; but that's where it
	// becomes very apparent: before this change, the rule that was reported as
	// "conflicting" was that of either one of the input files, randomly.
	mods := map[string]*Module{
		"0.rego": MustParseModule("package p\nr = 1 { true }"),
		"1.rego": MustParseModule("package p\nr = 2 { true }"),
	}
	tree := NewModuleTree(mods)
	vals := tree.Children[Var("data")].Children[String("p")].Modules
	if exp, act := 2, len(vals); exp != act {
		t.Fatalf("expected %d rules, found %d", exp, act)
	}
	mod0 := vals[0]
	mod1 := vals[1]
	if exp, act := IntNumberTerm(1), mod0.Rules[0].Head.Value; !exp.Equal(act) {
		t.Errorf("expected value %v, got %v", exp, act)
	}
	if exp, act := IntNumberTerm(2), mod1.Rules[0].Head.Value; !exp.Equal(act) {
		t.Errorf("expected value %v, got %v", exp, act)
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

	p = 1`)
	mods["mod-incr"] = MustParseModule(`
	package a.b.c

	s[1] { true }
	s[2] { true }`,
	)

	mods["dots-in-heads"] = MustParseModule(`
		package dots

		a.b.c = 12
		d.e.f.g = 34
		`)

	tree := NewRuleTree(NewModuleTree(mods))
	expectedNumRules := 25

	if tree.Size() != expectedNumRules {
		t.Errorf("Expected %v but got %v rules", expectedNumRules, tree.Size())
	}

	// Check that empty packages are represented as leaves with no rules.
	node := tree.Children[Var("data")].Children[String("a")].Children[String("b")].Children[String("empty")]
	if node == nil || len(node.Children) != 0 || len(node.Values) != 0 {
		t.Fatalf("Unexpected nil value or non-empty leaf of non-leaf node: %v", node)
	}

	// Check that root node is not hidden
	if exp, act := false, tree.Hide; act != exp {
		t.Errorf("expected tree.Hide=%v, got %v", exp, act)
	}

	system := tree.Child(Var("data")).Child(String("system"))
	if !system.Hide {
		t.Fatalf("Expected system node to be hidden: %v", system)
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
	t.Run("after failing means overall failure", func(t *testing.T) {
		c := NewCompiler().WithStageAfter(
			"CheckRecursion",
			CompilerStageDefinition{"MockStage", "mock_stage",
				func(*Compiler) *Error { return NewError(CompileErr, &Location{}, "mock stage error") }},
		)
		m := MustParseModule(testModule)
		c.Compile(map[string]*Module{"testMod": m})

		if !c.Failed() {
			t.Errorf("Expected compilation error")
		}
	})

	t.Run("first 'after' failure inhibits other 'after' stages", func(t *testing.T) {
		c := NewCompiler().
			WithStageAfter("CheckRecursion",
				CompilerStageDefinition{"MockStage", "mock_stage",
					func(*Compiler) *Error { return NewError(CompileErr, &Location{}, "mock stage error") }}).
			WithStageAfter("CheckRecursion",
				CompilerStageDefinition{"MockStage2", "mock_stage2",
					func(*Compiler) *Error { return NewError(CompileErr, &Location{}, "mock stage error two") }},
			)
		m := MustParseModule(`package p
q := true`)

		c.Compile(map[string]*Module{"testMod": m})

		if !c.Failed() {
			t.Errorf("Expected compilation error")
		}
		if exp, act := 1, len(c.Errors); exp != act {
			t.Errorf("expected %d errors, got %d: %v", exp, act, c.Errors)
		}
	})

	t.Run("'after' failure inhibits other ordinary stages", func(t *testing.T) {
		c := NewCompiler().
			WithStageAfter("CheckRecursion",
				CompilerStageDefinition{"MockStage", "mock_stage",
					func(*Compiler) *Error { return NewError(CompileErr, &Location{}, "mock stage error") }})
		m := MustParseModule(`package p
q {
	1 == "a" # would fail "CheckTypes", the next stage
}
`)
		c.Compile(map[string]*Module{"testMod": m})

		if !c.Failed() {
			t.Errorf("Expected compilation error")
		}
		if exp, act := 1, len(c.Errors); exp != act {
			t.Errorf("expected %d errors, got %d: %v", exp, act, c.Errors)
		}
	})
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

	result := make([]string, 0, len(errs))
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
	popts := ParserOptions{AllFutureKeywords: true, unreleasedKeywords: true}
	c.Modules["newMod"] = MustParseModuleWithOpts(`package a.b

unboundKey[x1] = y { q[y] = {"foo": [1, 2, [{"bar": y}]]} }
unboundVal[y] = x2 { q[y] = {"foo": [1, 2, [{"bar": y}]]} }
unboundCompositeVal[y] = [{"foo": x3, "bar": y}] { q[y] = {"foo": [1, 2, [{"bar": y}]]} }
unboundCompositeKey[[{"x": x4}]] { q[y] }
unboundBuiltinOperator = eq { 4 = 1 }
unboundElse { false } else = else_var { true }
c.d.e[x5] if true
f.g.h[y] = x6 if y := "y"
i.j.k contains x7 if true
`, popts)
	compileStages(c, c.checkSafetyRuleHeads)

	makeErrMsg := func(v string) string {
		return fmt.Sprintf("rego_unsafe_var_error: var %v is unsafe", v)
	}

	expected := []string{
		makeErrMsg("x1"),
		makeErrMsg("x2"),
		makeErrMsg("x3"),
		makeErrMsg("x4"),
		makeErrMsg("x5"),
		makeErrMsg("x6"),
		makeErrMsg("x7"),
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
		{"every", `every _ in [] { x != 1 }; x = 1`, `__local4__ = []; x = 1; every __local3__, _ in __local4__ { x != 1}`},
		{"every-domain", `every _ in xs { true }; xs = [1]`, `xs = [1]; __local4__ = xs; every __local3__, _ in __local4__ { true }`},
	}

	for i, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			opts := ParserOptions{AllFutureKeywords: true, unreleasedKeywords: true}
			c := NewCompiler()
			c.Modules = getCompilerTestModules()
			c.Modules["reordering"] = MustParseModuleWithOpts(fmt.Sprintf(
				`package test
				p { %s }`, tc.body), opts)

			compileStages(c, c.checkSafetyRuleBodies)

			if c.Failed() {
				t.Errorf("%v (#%d): Unexpected compilation error: %v", tc.note, i, c.Errors)
				return
			}

			expected := MustParseBodyWithOpts(tc.expected, opts)
			result := c.Modules["reordering"].Rules[0].Body

			if !expected.Equal(result) {
				t.Errorf("%v (#%d): Expected body to be ordered and equal to %v but got: %v", tc.note, i, expected, result)
			}
		})
	}
}

func TestCompilerCheckSafetyBodyReorderingClosures(t *testing.T) {
	opts := ParserOptions{AllFutureKeywords: true, unreleasedKeywords: true}

	tests := []struct {
		note string
		mod  *Module
		exp  Body
	}{
		{
			note: "comprehensions-1",
			mod: MustParseModule(`package compr

import data.b
import data.c
p = true { v = [null | true]; xs = [x | a[i] = x; a = [y | y != 1; y = c[j]]]; xs[j] > 0; z = [true | data.a.b.d.t with input as i2; i2 = i]; b[i] = j }
`),
			exp: MustParseBody(`v = [null | true]; data.b[i] = j; xs = [x | a = [y | y = data.c[j]; y != 1]; a[i] = x]; xs[j] > 0; z = [true | i2 = i; data.a.b.d.t with input as i2]`),
		},
		{
			note: "comprehensions-2",
			mod: MustParseModule(`package compr

import data.b
import data.c
q = true { _ = [x | x = b[i]]; _ = b[j]; _ = [x | x = true; x != false]; true != false; _ = [x | data.foo[_] = x]; data.foo[_] = _ }
`),
			exp: MustParseBody(`_ = [x | x = data.b[i]]; _ = data.b[j]; _ = [x | x = true; x != false]; true != false; _ = [x | data.foo[_] = x]; data.foo[_] = _`),
		},

		{
			note: "comprehensions-3",
			mod: MustParseModule(`package compr

import data.b
import data.c
fn(x) = y {
	trim(x, ".", y)
}
r = true { a = [x | split(y, ".", z); x = z[i]; fn("...foo.bar..", y)] }
`),
			exp: MustParseBody(`a = [x | data.compr.fn("...foo.bar..", y); split(y, ".", z); x = z[i]]`),
		},
		{
			note: "closure over function output",
			mod: MustParseModule(`package test
import future.keywords

p {
	object.get(input.subject.roles[_], comp, [""], output)
	comp = [ 1 | true ]
	every y in [2] {
		y in output
	}
}`),
			exp: MustParseBodyWithOpts(`comp = [1 | true]
				__local2__ = [2]
				object.get(input.subject.roles[_], comp, [""], output)
				every __local0__, __local1__ in __local2__ { internal.member_2(__local1__, output) }`, opts),
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			c := NewCompiler()
			c.Modules = map[string]*Module{"mod": tc.mod}
			compileStages(c, c.checkSafetyRuleBodies)
			assertNotFailed(t, c)
			last := len(c.Modules["mod"].Rules) - 1
			actual := c.Modules["mod"].Rules[last].Body
			if !actual.Equal(tc.exp) {
				t.Errorf("Expected reordered body to be equal to:\n%v\nBut got:\n%v", tc.exp, actual)
			}
		})
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
		{"closure-transitive", `p { x = y; x = [y | y = 1] }`, `{x,y}`},
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
		{"every", "p { every y in [10] { x > y } }", "{x,}"},
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
			popts := ParserOptions{AllFutureKeywords: true, unreleasedKeywords: true}
			c := NewCompiler()
			c.Modules = map[string]*Module{
				"newMod": MustParseModuleWithOpts(fmt.Sprintf(`

				%v

				%v

				`, moduleBegin, tc.moduleContent), popts),
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

func TestCompilerCheckSafetyFunctionAndContainsKeyword(t *testing.T) {
	_, err := CompileModules(map[string]string{"test.rego": `package play

			import future.keywords.contains

			p(id) contains x {
				x := id
			}`})
	if err == nil {
		t.Fatal("expected error")
	}

	errs := err.(Errors)
	if !strings.Contains(errs[0].Message, "the contains keyword can only be used with multi-value rule definitions (e.g., p contains <VALUE> { ... })") {
		t.Fatal("wrong error message:", err)
	}
	if errs[0].Location.Row != 5 {
		t.Fatal("expected error on line 5 but got:", errs[0].Location.Row)
	}
}

func TestCompilerCheckTypes(t *testing.T) {
	c := NewCompiler()
	modules := getCompilerTestModules()
	c.Modules = map[string]*Module{"mod6": modules["mod6"], "mod7": modules["mod7"]}
	compileStages(c, c.checkTypes)
	assertNotFailed(t, c)
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
foo = 3 { true }

default p.q.bar = 1
default p.q.bar = 2
p.q.bar = 3 { true }
`,
		"mod4.rego": `package badrules.arity

f(1) { true }
f { true }

g(1) { true }
g(1,2) { true }

p.q.h(1) { true }
p.q.h { true }

p.q.i(1) { true }
p.q.i(1,2) { true }`,
		"mod5.rego": `package badrules.dataoverlap

p { true }`,
		"mod6.rego": `package badrules.existserr

p { true }`,

		"mod7.rego": `package badrules.foo
import future.keywords

bar.baz contains "quz" if true`,
		"mod8.rego": `package badrules.complete_partial
p := 1
p[r] := 2 { r := "foo" }`,
	})

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
		"rego_type_error: conflicting rules data.badrules.arity.f found",
		"rego_type_error: conflicting rules data.badrules.arity.g found",
		"rego_type_error: conflicting rules data.badrules.arity.p.q.h found",
		"rego_type_error: conflicting rules data.badrules.arity.p.q.i found",
		"rego_type_error: conflicting rules data.badrules.complete_partial.p[r] found",
		"rego_type_error: conflicting rules data.badrules.p[x] found",
		"rego_type_error: conflicting rules data.badrules.q found",
		"rego_type_error: multiple default rules data.badrules.defkw.foo found",
		"rego_type_error: multiple default rules data.badrules.defkw.p.q.bar found",
		"rego_type_error: package badrules.r conflicts with rule r[x] defined at mod1.rego:7",
		"rego_type_error: package badrules.r conflicts with rule r[x] defined at mod1.rego:8",
	}

	assertCompilerErrorStrings(t, c, expected)
}

func TestCompilerCheckRuleConflictsDefaultFunction(t *testing.T) {
	tests := []struct {
		note    string
		modules []*Module
		err     string
	}{
		{
			note: "conflicting rules",
			modules: modules(
				`package pkg
				default f(_) = 100
				f(x, y) = x {
                   x == y
				}`),
			err: "rego_type_error: conflicting rules data.pkg.f found",
		},
	}
	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			mods := make(map[string]*Module, len(tc.modules))
			for i, m := range tc.modules {
				mods[fmt.Sprint(i)] = m
			}
			c := NewCompiler()
			c.Modules = mods
			compileStages(c, c.checkRuleConflicts)
			if tc.err != "" {
				assertCompilerErrorStrings(t, c, []string{tc.err})
			} else {
				assertCompilerErrorStrings(t, c, []string{})
			}
		})
	}
}

func TestCompilerCheckRuleConflictsDotsInRuleHeads(t *testing.T) {
	tests := []struct {
		note    string
		modules []*Module
		err     string
	}{
		{
			note: "arity mismatch, ref and non-ref rule",
			modules: modules(
				`package pkg
				p.q.r { true }`,
				`package pkg.p.q
				r(_) = 2`),
			err: "rego_type_error: conflicting rules data.pkg.p.q.r found",
		},
		{
			note: "two default rules, ref and non-ref rule",
			modules: modules(
				`package pkg
				default p.q.r = 3
				p.q.r { true }`,
				`package pkg.p.q
				default r = 4
				r = 2`),
			err: "rego_type_error: multiple default rules data.pkg.p.q.r found",
		},
		{
			note: "arity mismatch, ref and ref rule",
			modules: modules(
				`package pkg.a.b
				p.q.r { true }`,
				`package pkg.a
				b.p.q.r(_) = 2`),
			err: "rego_type_error: conflicting rules data.pkg.a.b.p.q.r found",
		},
		{
			note: "two default rules, ref and ref rule",
			modules: modules(
				`package pkg
				default p.q.w.r = 3
				p.q.w.r { true }`,
				`package pkg.p
				default q.w.r = 4
				q.w.r = 2`),
			err: "rego_type_error: multiple default rules data.pkg.p.q.w.r found",
		},
		{
			note: "multi-value + single-value rules, both with same ref prefix",
			modules: modules(
				`package pkg
				p.q.w[x] = 1 if x := "foo"`,
				`package pkg
				p.q.w contains "bar"`),
			err: "rego_type_error: conflicting rules data.pkg.p.q.w found",
		},
		{
			note: "two multi-value rules, both with same ref",
			modules: modules(
				`package pkg
				p.q.w contains "baz"`,
				`package pkg
				p.q.w contains "bar"`),
		},
		{
			note: "module conflict: non-ref rule",
			modules: modules(
				`package pkg.q
				r { true }`,
				`package pkg.q.r`),
			err: "rego_type_error: package pkg.q.r conflicts with rule r defined at mod0.rego:2",
		},
		{
			note: "module conflict: ref rule",
			modules: modules(
				`package pkg
				p.q.r { true }`,
				`package pkg.p.q.r`),
			err: "rego_type_error: package pkg.p.q.r conflicts with rule p.q.r defined at mod0.rego:2",
		},
		{
			note: "single-value with other rule overlap",
			modules: modules(
				`package pkg
				p.q.r { true }`,
				`package pkg
				p.q.r.s { true }`),
			err: "rego_type_error: rule data.pkg.p.q.r conflicts with [data.pkg.p.q.r.s]",
		},
		{
			note: "single-value with other rule overlap",
			modules: modules(
				`package pkg
				p.q.r { true }
				p.q.r.s { true }
				p.q.r.t { true }`),
			err: "rego_type_error: rule data.pkg.p.q.r conflicts with [data.pkg.p.q.r.s data.pkg.p.q.r.t]",
		},
		{
			note: "single-value with other partial object (same ref) overlap",
			modules: modules(
				`package pkg
				p.q := 1
				p.q[r] := 2 { r := "foo" }`),
			err: "rego_type_error: conflicting rules data.pkg.p.q[r] foun",
		},
		{
			note: "single-value with other rule overlap, unknown key",
			modules: modules(
				`package pkg
				p.q[r] = x { r = input.key; x = input.foo }
				p.q.r.s = x { true }
				`),
		},
		{
			note: "single-value with other rule overlap, unknown ref var and key",
			modules: modules(
				`package pkg
				p.q[r][s] = x { r = input.key1; s = input.key2; x = input.foo }
				p.q.r.s.t = x { true }
				`),
		},
		{
			note: "single-value partial object with other partial object rule overlap, unknown keys (regression test for #5855; invalidated by multi-var refs)",
			modules: modules(
				`package pkg
				p[r] := x { r = input.key; x = input.bar }
				p.q[r] := x { r = input.key; x = input.bar }
				`),
		},
		{
			note: "single-value partial object with other partial object (implicit 'true' value) rule overlap, unknown keys",
			modules: modules(
				`package pkg
				p[r] := x { r = input.key; x = input.bar }
				p.q[r] { r = input.key }
				`),
		},
		{
			note: "single-value partial object with multi-value rule (ref head) overlap, unknown key",
			modules: modules(
				`package pkg
				import future.keywords
				p[r] := x { r = input.key; x = input.bar }
				p.q contains r { r = input.key }
				`),
		},
		{
			note: "single-value partial object with multi-value rule overlap, unknown key",
			modules: modules(
				`package pkg
				p[r] := x { r = input.key; x = input.bar }
				p.q { true }
				`),
			err: "rego_type_error: conflicting rules data.pkg.p found",
		},
		{
			note: "single-value rule with known and unknown key",
			modules: modules(
				`package pkg
				p.q[r] = x { r = input.key; x = input.foo }
				p.q.s = "x" { true }
				`),
		},
		{
			note: "multi-value rule with other rule overlap",
			modules: modules(
				`package pkg
				p[v] { v := ["a", "b"][_] }
				p.q := 42
				`),
			err: "rego_type_error: rule data.pkg.p conflicts with [data.pkg.p.q]",
		},
		{
			note: "multi-value rule with other rule (ref) overlap",
			modules: modules(
				`package pkg
				p[v] { v := ["a", "b"][_] }
				p.q.r { true }
				`),
			err: "rego_type_error: rule data.pkg.p conflicts with [data.pkg.p.q.r]",
		},
		{
			note: "multi-value rule (dots in head) with other rule (ref) overlap",
			modules: modules(
				`package pkg
				import future.keywords
				p.q contains v { v := ["a", "b"][_] }
				p.q.r { true }
				`),
			err: "rule data.pkg.p.q conflicts with [data.pkg.p.q.r]",
		},
		{
			note: "multi-value rule (dots and var in head) with other rule (ref) overlap",
			modules: modules(
				`package pkg
				import future.keywords
				p[q] contains v { v := ["a", "b"][_] }
				p.q.r { true }
				`),
		},
		{
			note: "function with other rule (ref) overlap",
			modules: modules(
				`package pkg
				p(x) := x
				p.q.r { true }
				`),
			err: "rego_type_error: rule data.pkg.p conflicts with [data.pkg.p.q.r]",
		},
		{
			note: "function with other rule (ref) overlap",
			modules: modules(
				`package pkg
				p(x) := x
				p.q.r { true }
				`),
			err: "rego_type_error: rule data.pkg.p conflicts with [data.pkg.p.q.r]",
		},
		{
			note: "function (ref) with other rule (ref) overlap",
			modules: modules(
				`package pkg
				p.q(x) := x
				p.q.r { true }
				`),
			err: "rego_type_error: rule data.pkg.p.q conflicts with [data.pkg.p.q.r]",
		},
	}
	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			mods := make(map[string]*Module, len(tc.modules))
			for i, m := range tc.modules {
				mods[fmt.Sprint(i)] = m
			}
			c := NewCompiler()
			c.Modules = mods
			compileStages(c, c.checkRuleConflicts)
			if tc.err != "" {
				assertCompilerErrorStrings(t, c, []string{tc.err})
			} else {
				assertCompilerErrorStrings(t, c, []string{})
			}
		})
	}
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

		# NOTE: all the dynamic dispatch examples here are not supported,
		#       we're checking assertions about the error returned.
		undefined_dynamic_dispatch {
			x = "f"; data.test2[x](1)
		}

		undefined_dynamic_dispatch_declared_var {
			y := "f"; data.test2[y](1)
		}

		undefined_dynamic_dispatch_declared_var_in_array {
			z := "f"; data.test2[[z]](1)
		}

		arity_mismatch_1 {
			data.test2.f(1,2,3)
		}

		arity_mismatch_2 {
			data.test2.f()
		}

		arity_mismatch_3 {
			x:= data.test2.f()
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
		"rego_type_error: undefined function data.test2[y]",
		"rego_type_error: undefined function data.test2[[z]]",
		"rego_type_error: function data.test2.f has arity 1, got 3 arguments",
		"test.rego:31: rego_type_error: function data.test2.f has arity 1, got 0 arguments",
		"test.rego:35: rego_type_error: function data.test2.f has arity 1, got 0 arguments",
	}
	for _, w := range want {
		if !strings.Contains(result, w) {
			t.Fatalf("Expected %q in result but got: %v", w, result)
		}
	}
}

func TestCompilerQueryCompilerCheckUndefinedFuncs(t *testing.T) {
	compiler := NewCompiler()

	for _, tc := range []struct {
		note, query, err string
	}{

		{note: "undefined function", query: `data.foo(1)`, err: "undefined function data.foo"},
		{note: "undefined global function", query: `foo(1)`, err: "undefined function foo"},
		{note: "var", query: `x = "f"; data[x](1)`, err: "undefined function data[x]"},
		{note: "declared var", query: `x := "f"; data[x](1)`, err: "undefined function data[x]"},
		{note: "declared var in array", query: `x := "f"; data[[x]](1)`, err: "undefined function data[[x]]"},
	} {
		t.Run(tc.note, func(t *testing.T) {
			_, err := compiler.QueryCompiler().Compile(MustParseBody(tc.query))
			if !strings.Contains(err.Error(), tc.err) {
				t.Errorf("Unexpected compilation error: %v (want  %s)", err, tc.err)
			}
		})
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
				MustParseExpr("g(x, __local0__)"),
				MustParseExpr("f(x, __local1__)"),
				MustParseExpr("{__local1__, {__local0__,}}"),
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
		{
			note: "every: domain",
			module: `
			package test

			p { every x in [1,2] { x } }`,
			expected: `
			package test

			p { __local2__ = [1, 2]; every __local0__, __local1__ in __local2__ { __local1__ } }`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			compiler := NewCompiler()
			opts := ParserOptions{AllFutureKeywords: true, unreleasedKeywords: true}

			compiler.Modules = map[string]*Module{
				"test": MustParseModuleWithOpts(tc.module, opts),
			}
			compileStages(compiler, compiler.rewriteExprTerms)

			switch exp := tc.expected.(type) {
			case string:
				assertNotFailed(t, compiler)

				expected := MustParseModuleWithOpts(exp, opts)

				if !expected.Equal(compiler.Modules["test"]) {
					t.Fatalf("Expected modules to be equal. Expected:\n\n%v\n\nGot:\n\n%v", expected, compiler.Modules["test"])
				}
			case Errors:
				assertErrors(t, compiler.Errors, exp, false)
			default:
				t.Fatalf("Unsupported value type for test case 'expected' field: %v", exp)
			}

		})
	}
}

func TestIllegalFunctionCallRewrite(t *testing.T) {
	cases := []struct {
		note           string
		module         string
		expectedErrors []string
	}{
		/*{
		  			note: "function call override in function value",
		  			module: `package test
		  foo(x) := x

		  p := foo(bar) {
		  	#foo := 1
		  	bar := 2
		  }`,
		  			expectedErrors: []string{
		  				"undefined function foo",
		  			},
		  		},*/
		{
			note: "function call override in array comprehension value",
			module: `package test
p := [foo(bar) | foo := 1; bar := 2]`,
			expectedErrors: []string{
				"called function foo shadowed",
			},
		},
		{
			note: "function call override in set comprehension value",
			module: `package test
p := {foo(bar) | foo := 1; bar := 2}`,
			expectedErrors: []string{
				"called function foo shadowed",
			},
		},
		{
			note: "function call override in object comprehension value",
			module: `package test
p := {foo(bar): bar(foo) | foo := 1; bar := 2}`,
			expectedErrors: []string{
				"called function bar shadowed",
				"called function foo shadowed",
			},
		},
		{
			note: "function call override in array comprehension value",
			module: `package test
p := [foo.bar(baz) | foo := 1; bar := 2; baz := 3]`,
			expectedErrors: []string{
				"called function foo.bar shadowed",
			},
		},
		{
			note: "nested function call override in array comprehension value",
			module: `package test
p := [baz(foo(bar)) | foo := 1; bar := 2]`,
			expectedErrors: []string{
				"called function foo shadowed",
			},
		},
		{
			note: "function call override of 'input' root document",
			module: `package test
p := [input() | input := 1]`,
			expectedErrors: []string{
				"called function input shadowed",
			},
		},
		{
			note: "function call override of 'data' root document",
			module: `package test
p := [data() | data := 1]`,
			expectedErrors: []string{
				"called function data shadowed",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			compiler := NewCompiler()
			opts := ParserOptions{AllFutureKeywords: true, unreleasedKeywords: true}

			compiler.Modules = map[string]*Module{
				"test": MustParseModuleWithOpts(tc.module, opts),
			}
			compileStages(compiler, compiler.rewriteLocalVars)

			result := make([]string, 0, len(compiler.Errors))
			for i := range compiler.Errors {
				result = append(result, compiler.Errors[i].Message)
			}

			sort.Strings(tc.expectedErrors)
			sort.Strings(result)

			if len(tc.expectedErrors) != len(result) {
				t.Fatalf("Expected %d errors but got %d:\n\n%v\n\nGot:\n\n%v",
					len(tc.expectedErrors), len(result),
					strings.Join(tc.expectedErrors, "\n"), strings.Join(result, "\n"))
			}

			for i := range result {
				if result[i] != tc.expectedErrors[i] {
					t.Fatalf("Expected:\n\n%v\n\nGot:\n\n%v",
						strings.Join(tc.expectedErrors, "\n"), strings.Join(result, "\n"))
				}
			}
		})
	}
}

func TestCompilerCheckUnusedImports(t *testing.T) {
	cases := []strictnessTestCase{
		{
			note: "simple unused: input ref with same name",
			module: `package p
			import data.foo.bar as bar
			r {
				input.bar == 11
			}
			`,
			expectedErrors: Errors{
				&Error{
					Location: NewLocation([]byte("import"), "", 2, 4),
					Message:  "import data.foo.bar as bar unused",
				},
			},
		},
		{
			note: "unused import, but imported ref used",
			module: `package p
			import data.foo # unused
			r { data.foo == 10 }
			`,
			expectedErrors: Errors{
				&Error{
					Location: NewLocation([]byte("import"), "", 2, 4),
					Message:  "import data.foo unused",
				},
			},
		},
		{
			note: "one of two unused",
			module: `package p
			import data.foo
			import data.x.power #unused
			r { foo == 10 }
			`,
			expectedErrors: Errors{
				&Error{
					Location: NewLocation([]byte("import"), "", 3, 4),
					Message:  "import data.x.power unused",
				},
			},
		},
		{
			note: "multiple unused: with input ref of same name",
			module: `package p
			import data.foo
			import data.x.power
			r { input.foo == 10 }
			`,
			expectedErrors: Errors{
				&Error{
					Location: NewLocation([]byte("import"), "", 2, 4),
					Message:  "import data.foo unused",
				},
				&Error{
					Location: NewLocation([]byte("import"), "", 3, 4),
					Message:  "import data.x.power unused",
				},
			},
		},
		{
			note: "import used in comparison",
			module: `package p
			import data.foo.x
			r { x == 10 }
			`,
		},
		{
			note: "multiple used imports in one rule",
			module: `package p
			import data.foo.x
			import data.power.ranger
			r { ranger == x }
			`,
		},
		{
			note: "multiple used imports in separate rules",
			module: `package p
			import data.foo.x
			import data.power.ranger
			r { ranger == 23 }
			t { x == 1 }
			`,
		},
		{
			note: "import used as function operand",
			module: `package p
			import data.foo
			r = count(foo) > 1 # only one operand
			`,
		},
		{
			note: "import used as function operand, compount term",
			module: `package p
			import data.foo
			r = sprintf("%v %d", [foo, 0])
			`,
		},
		{
			note: "import used as plain term",
			module: `package p
			import data.foo
			r {
				foo
			}
			`,
		},
		{
			note: "import used in 'every' domain",
			module: `package p
			import future.keywords.every
			import data.foo
			r {
				every x in foo { x > 1 }
			}
			`,
		},
		{
			note: "import used in 'every' body",
			module: `package p
			import future.keywords.every
			import data.foo
			r {
				every x in [1,2,3] { x > foo }
			}
			`,
		},
		{
			note: "future import kept even if unused",
			module: `package p
			import future.keywords

			r { true }
			`,
		},
		{
			note: "shadowed var name in function arg",
			module: `package p
			import data.foo # unused

			r { f(1) }
			f(foo) = foo == 1
			`,
			expectedErrors: Errors{
				&Error{
					Location: NewLocation([]byte("import"), "", 2, 4),
					Message:  "import data.foo unused",
				},
			},
		},
		{
			note: "shadowed assigned var name",
			module: `package p
			import data.foo # unused

			r { foo := true; foo }
			`,
			expectedErrors: Errors{
				&Error{
					Location: NewLocation([]byte("import"), "", 2, 4),
					Message:  "import data.foo unused",
				},
			},
		},
		{
			note: "used as rule value",
			module: `package p
			import data.bar # unused
			import data.foo

			r = foo { true }
			`,
			expectedErrors: Errors{
				&Error{
					Location: NewLocation([]byte("import"), "", 2, 4),
					Message:  "import data.bar unused",
				},
			},
		},
		{
			note: "unused as rule value (but same data ref)",
			module: `package p
			import data.bar # unused
			import data.foo # unused

			r = data.foo { true }
			`,
			expectedErrors: Errors{
				&Error{
					Location: NewLocation([]byte("import"), "", 2, 4),
					Message:  "import data.bar unused",
				},
				&Error{
					Location: NewLocation([]byte("import"), "", 3, 4),
					Message:  "import data.foo unused",
				},
			},
		},
	}

	runStrictnessTestCase(t, cases, true)
}

func TestCompilerCheckDuplicateImports(t *testing.T) {
	cases := []strictnessTestCase{
		{
			note: "shadow",
			module: `package test
				import input.noconflict
				import input.foo
				import data.foo
				import data.bar.foo

				p := noconflict
				q := foo
			`,
			expectedErrors: Errors{
				&Error{
					Location: NewLocation([]byte("import"), "", 4, 5),
					Message:  "import must not shadow import input.foo",
				},
				&Error{
					Location: NewLocation([]byte("import"), "", 5, 5),
					Message:  "import must not shadow import input.foo",
				},
			},
		}, {
			note: "alias shadow",
			module: `package test
				import input.noconflict
				import input.foo
				import input.bar as foo

				p := noconflict
				q := foo
			`,
			expectedErrors: Errors{
				&Error{
					Location: NewLocation([]byte("import"), "", 4, 5),
					Message:  "import must not shadow import input.foo",
				},
			},
		},
	}

	runStrictnessTestCase(t, cases, true)
}

func TestCompilerCheckKeywordOverrides(t *testing.T) {
	cases := []strictnessTestCase{
		{
			note: "rule names",
			module: `package test
				input { true }
				p { true }
				data { true }
			`,
			expectedErrors: Errors{
				&Error{
					Location: NewLocation([]byte("input { true }"), "", 2, 5),
					Message:  "rules must not shadow input (use a different rule name)",
				},
				&Error{
					Location: NewLocation([]byte("data { true }"), "", 4, 5),
					Message:  "rules must not shadow data (use a different rule name)",
				},
			},
		},
		{
			note: "global assignments",
			module: `package test
				input = 1
				p := 2
				data := 3
			`,
			expectedErrors: Errors{
				&Error{
					Location: NewLocation([]byte("input = 1"), "", 2, 5),
					Message:  "rules must not shadow input (use a different rule name)",
				},
				&Error{
					Location: NewLocation([]byte("data := 3"), "", 4, 5),
					Message:  "rules must not shadow data (use a different rule name)",
				},
			},
		},
		{
			note: "rule-local assignments",
			module: `package test
				p {
					input := 1
					x := 2
				} else {
					data := 3
				}
				q {
					input := 4
				}
			`,
			expectedErrors: Errors{
				&Error{
					Location: NewLocation([]byte("input := 1"), "", 3, 6),
					Message:  "variables must not shadow input (use a different variable name)",
				},
				&Error{
					Location: NewLocation([]byte("data := 3"), "", 6, 6),
					Message:  "variables must not shadow data (use a different variable name)",
				},
				&Error{
					Location: NewLocation([]byte("input := 4"), "", 9, 6),
					Message:  "variables must not shadow input (use a different variable name)",
				},
			},
		},
		{
			note: "array comprehension-local assignments",
			module: `package test
				p = [ x |
					input := 1
					x := 2
					data := 3
				]
			`,
			expectedErrors: Errors{
				&Error{
					Location: NewLocation([]byte("input := 1"), "", 3, 6),
					Message:  "variables must not shadow input (use a different variable name)",
				},
				&Error{
					Location: NewLocation([]byte("data := 3"), "", 5, 6),
					Message:  "variables must not shadow data (use a different variable name)",
				},
			},
		},
		{
			note: "set comprehension-local assignments",
			module: `package test
				p = { x |
					input := 1
					x := 2
					data := 3
				}
			`,
			expectedErrors: Errors{
				&Error{
					Location: NewLocation([]byte("input := 1"), "", 3, 6),
					Message:  "variables must not shadow input (use a different variable name)",
				},
				&Error{
					Location: NewLocation([]byte("data := 3"), "", 5, 6),
					Message:  "variables must not shadow data (use a different variable name)",
				},
			},
		},
		{
			note: "object comprehension-local assignments",
			module: `package test
				p = { x: 1 |
					input := 1
					x := 2
					data := 3
				}
			`,
			expectedErrors: Errors{
				&Error{
					Location: NewLocation([]byte("input := 1"), "", 3, 6),
					Message:  "variables must not shadow input (use a different variable name)",
				},
				&Error{
					Location: NewLocation([]byte("data := 3"), "", 5, 6),
					Message:  "variables must not shadow data (use a different variable name)",
				},
			},
		},
		{
			note: "nested override",
			module: `package test
				p {
					[ x |
						input := 1
						x := 2
						data := 3
					]
				}
			`,
			expectedErrors: Errors{
				&Error{
					Location: NewLocation([]byte("input := 1"), "", 4, 7),
					Message:  "variables must not shadow input (use a different variable name)",
				},
				&Error{
					Location: NewLocation([]byte("data := 3"), "", 6, 7),
					Message:  "variables must not shadow data (use a different variable name)",
				},
			},
		},
	}

	runStrictnessTestCase(t, cases, true)
}

func TestCompilerCheckDeprecatedMethods(t *testing.T) {
	cases := []strictnessTestCase{
		{
			note: "all() built-in",
			module: `package test
				p := all([true, false])
			`,
			expectedErrors: Errors{
				&Error{
					Location: NewLocation([]byte("all([true, false])"), "", 2, 10),
					Message:  "deprecated built-in function calls in expression: all",
				},
			},
		},
		{
			note: "user-defined all()",
			module: `package test
				import future.keywords.in
				all(arr) = {x | some x in arr} == {true}
				p := all([true, false])
			`,
		},
		{
			note: "any() built-in",
			module: `package test
				p := any([true, false])
			`,
			expectedErrors: Errors{
				&Error{
					Location: NewLocation([]byte("any([true, false])"), "", 2, 10),
					Message:  "deprecated built-in function calls in expression: any",
				},
			},
		},
		{
			note: "user-defined any()",
			module: `package test
				import future.keywords.in
				any(arr) := true in arr
				p := any([true, false])
			`,
		},
		{
			note: "re_match built-in",
			module: `package test
				p := re_match("[a]", "a")
			`,
			expectedErrors: Errors{
				&Error{
					Location: NewLocation([]byte(`re_match("[a]", "a")`), "", 2, 10),
					Message:  "deprecated built-in function calls in expression: re_match",
				},
			},
		},
	}

	runStrictnessTestCase(t, cases, true)
}

type strictnessTestCase struct {
	note           string
	module         string
	expectedErrors Errors
}

func runStrictnessTestCase(t *testing.T, cases []strictnessTestCase, assertLocation bool) {
	t.Helper()
	makeTestRunner := func(tc strictnessTestCase, strict bool) func(t *testing.T) {
		return func(t *testing.T) {
			compiler := NewCompiler().WithStrict(strict)
			compiler.Modules = map[string]*Module{
				"test": MustParseModule(tc.module),
			}
			compileStages(compiler, nil)

			if strict {
				assertErrors(t, compiler.Errors, tc.expectedErrors, assertLocation)
			} else {
				assertNotFailed(t, compiler)
			}
		}
	}

	for _, tc := range cases {
		t.Run(tc.note+"_strict", makeTestRunner(tc, true))
		t.Run(tc.note+"_non-strict", makeTestRunner(tc, false))
	}
}

func assertErrors(t *testing.T, actual Errors, expected Errors, assertLocation bool) {
	t.Helper()
	if len(expected) != len(actual) {
		t.Fatalf("Expected %d errors, got %d:\n\n%s\n", len(expected), len(actual), actual.Error())
	}
	incorrectErrs := false
	for _, e := range expected {
		found := false
		for _, actual := range actual {
			if e.Message == actual.Message {
				if !assertLocation || e.Location.Equal(actual.Location) {
					found = true
					break
				}
			}
		}
		if !found {
			incorrectErrs = true
		}
	}
	if incorrectErrs {
		t.Fatalf("Expected errors:\n\n%s\n\nGot:\n\n%s\n", expected.Error(), actual.Error())
	}
}

// NOTE(sr): the tests below this function are unwieldy, let's keep adding new ones to this one
func TestCompilerResolveAllRefsNewTests(t *testing.T) {
	tests := []struct {
		note  string
		mod   string
		exp   string
		extra string
	}{
		{
			note: "ref-rules referenced in body",
			mod: `package test
a.b.c = 1
q if a.b.c == 1
`,
			exp: `package test
a.b.c = 1 { true }
q if data.test.a.b.c = 1
`,
		},
		{
			// NOTE(sr): This is a conservative extension of how it worked before:
			// we will not automatically extend references to other parts of the rule tree,
			// only to ref rules defined on the same level.
			note: "ref-rules from other module referenced in body",
			mod: `package test
q if a.b.c == 1
`,
			extra: `package test
a.b.c = 1
`,
			exp: `package test
q if data.test.a.b.c = 1
`,
		},
		{
			note: "single-value rule in comprehension in call", // NOTE(sr): this is TestRego/partialiter/objects_conflict
			mod: `package test
p := count([x | q[x]])
q[1] = 1
`,
			exp: `package test
p := __local0__ { true; __local1__ = [x | data.test.q[x]]; count(__local1__, __local0__) }
q[1] = 1
`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			opts := ParserOptions{AllFutureKeywords: true, unreleasedKeywords: true}
			c := NewCompiler()
			mod, err := ParseModuleWithOpts("test.rego", tc.mod, opts)
			if err != nil {
				t.Fatal(err)
			}
			exp, err := ParseModuleWithOpts("test.rego", tc.exp, opts)
			if err != nil {
				t.Fatal(err)
			}
			mods := map[string]*Module{"test": mod}
			if tc.extra != "" {
				extra, err := ParseModuleWithOpts("test.rego", tc.extra, opts)
				if err != nil {
					t.Fatal(err)
				}
				mods["extra"] = extra
			}
			c.Compile(mods)
			if err := c.Errors; len(err) > 0 {
				t.Errorf("compile module: %v", err)
			}
			if act := c.Modules["test"]; !exp.Equal(act) {
				t.Errorf("compiled: expected %v, got %v", exp, act)
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

	c.Modules["everykw"] = MustParseModuleWithOpts(`package everykw

	nums = {1, 2, 3}
	f(_) = true
	x = 100
	xs = [1, 2, 3]
	p {
		every x in xs {
			nums[x]
			x > 10
		}
	}`, ParserOptions{unreleasedKeywords: true, FutureKeywords: []string{"every", "in"}})

	c.Modules["heads_with_dots"] = MustParseModule(`package heads_with_dots

		this_is_not = true
		this.is.dotted { this_is_not }
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

	mod16 := c.Modules["everykw"]
	everyExpr := mod16.Rules[len(mod16.Rules)-1].Body[0].Terms.(*Every)
	assertTermEqual(t, everyExpr.Body[0].Terms.(*Term), MustParseTerm("data.everykw.nums[x]"))
	assertTermEqual(t, everyExpr.Domain, MustParseTerm("data.everykw.xs"))

	// 'x' is not resolved
	assertTermEqual(t, everyExpr.Value, VarTerm("x"))
	gt10 := MustParseExpr("x > 10")
	gt10.Index++ // TODO(sr): why?
	assertExprEqual(t, everyExpr.Body[1], gt10)

	// head refs are kept as-is, but their bodies are replaced.
	mod := c.Modules["heads_with_dots"]
	rule := mod.Rules[1]
	body := rule.Body[0].Terms.(*Term)
	assertTermEqual(t, body, MustParseTerm("data.heads_with_dots.this_is_not"))
	if act, exp := rule.Head.Ref(), MustParseRef("this.is.dotted"); act.Compare(exp) != 0 {
		t.Errorf("expected %v to match %v", act, exp)
	}
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
	popts := ParserOptions{AllFutureKeywords: true, unreleasedKeywords: true}

	tests := []struct {
		note string
		mod  *Module
		exp  *Rule
	}{
		{
			note: "imports",
			mod: MustParseModule(`package head
import data.doc1 as bar
import data.doc2 as corge
import input.x.y.foo
import input.qux as baz

p[foo[bar[i]]] = {"baz": baz, "corge": corge} { true }
`),
			exp: MustParseRule(`p[__local0__] = __local1__ { true; __local0__ = input.x.y.foo[data.doc1[i]]; __local1__ = {"baz": input.qux, "corge": data.doc2} }`),
		},
		{
			note: "array comprehension value",
			mod: MustParseModule(`package head
q = [true | true] { true }
`),
			exp: MustParseRule(`q = __local0__ { true; __local0__ = [true | true] }`),
		},
		{
			note: "array comprehension value in else head",
			mod: MustParseModule(`package head
q { 
	false 
} else = [true | true] { 
	true 
}
`),
			exp: MustParseRule(`q = true { false } else = __local0__ { true; __local0__ = [true | true] }`),
		},
		{
			note: "array comprehension value in head (comprehension-local var)",
			mod: MustParseModule(`package head
q = [a | a := true] {
	false
} else = [a | a := true] {
	true
}
`),
			exp: MustParseRule(`q = __local2__ { false; __local2__ = [__local0__ | __local0__ = true] } else = __local3__ { true; __local3__ = [__local1__ | __local1__ = true] }`),
		},
		{
			note: "array comprehension value in function head (comprehension-local var)",
			mod: MustParseModule(`package head
f(x) = [a | a := true] {
	false
} else = [a | a := true] {
	true
}
`),
			exp: MustParseRule(`f(__local0__) = __local3__ { false; __local3__ = [__local1__ | __local1__ = true] } else = __local4__ { true; __local4__ = [__local2__ | __local2__ = true] }`),
		},
		{
			note: "array comprehension value in else-func head (reused arg rewrite)",
			mod: MustParseModule(`package head
f(x, y) = [x | y] { 
	false 
} else = [x | y] {
	true 
}
`),
			exp: MustParseRule(`f(__local0__, __local1__) = __local2__ { false; __local2__ = [__local0__ | __local1__] } else = __local3__ { true; __local3__ = [__local0__ | __local1__] }`),
		},
		{
			note: "object comprehension value",
			mod: MustParseModule(`package head
r = {"true": true | true} { true }
`),
			exp: MustParseRule(`r = __local0__ { true; __local0__ = {"true": true | true} }`),
		},
		{
			note: "object comprehension value in else head",
			mod: MustParseModule(`package head
q { 
	false 
} else = {"true": true | true} { 
	true 
}
`),
			exp: MustParseRule(`q = true { false } else = __local0__ { true; __local0__ = {"true": true | true} }`),
		},
		{
			note: "object comprehension value in head (comprehension-local var)",
			mod: MustParseModule(`package head
q = {"a": a | a := true} {
	false
} else = {"a": a | a := true} {
	true
}
`),
			exp: MustParseRule(`q = __local2__ { false; __local2__ = {"a": __local0__ | __local0__ = true} } else = __local3__ { true; __local3__ = {"a": __local1__ | __local1__ = true} }`),
		},
		{
			note: "object comprehension value in function head (comprehension-local var)",
			mod: MustParseModule(`package head
f(x) = {"a": a | a := true} {
	false
} else = {"a": a | a := true} {
	true
}
`),
			exp: MustParseRule(`f(__local0__) = __local3__ { false; __local3__ = {"a": __local1__ | __local1__ = true} } else = __local4__ { true; __local4__ = {"a": __local2__ | __local2__ = true} }`),
		},
		{
			note: "object comprehension value in else-func head (reused arg rewrite)",
			mod: MustParseModule(`package head
f(x, y) = {x: y | true} { 
	false 
} else = {x: y | true} {
	true 
}
`),
			exp: MustParseRule(`f(__local0__, __local1__) = __local2__ { false; __local2__ = {__local0__: __local1__ | true} } else = __local3__ { true; __local3__ = {__local0__: __local1__ | true} }`),
		},
		{
			note: "set comprehension value",
			mod: MustParseModule(`package head
s = {true | true} { true }
`),
			exp: MustParseRule(`s = __local0__ { true; __local0__ = {true | true} }`),
		},
		{
			note: "set comprehension value in else head",
			mod: MustParseModule(`package head
q = {false | false} { 
	false 
} else = {true | true} { 
	true 
}
`),
			exp: MustParseRule(`q = __local0__ { false; __local0__ = {false | false} } else = __local1__ { true; __local1__ = {true | true} }`),
		},
		{
			note: "set comprehension value in head (comprehension-local var)",
			mod: MustParseModule(`package head
q = {a | a := true} {
	false
} else = {a | a := true} {
	true
}
`),
			exp: MustParseRule(`q = __local2__ { false; __local2__ = {__local0__ | __local0__ = true} } else = __local3__ { true; __local3__ = {__local1__ | __local1__ = true} }`),
		},
		{
			note: "set comprehension value in function head (comprehension-local var)",
			mod: MustParseModule(`package head
f(x) = {a | a := true} {
	false
} else = {a | a := true} {
	true
}
`),
			exp: MustParseRule(`f(__local0__) = __local3__ { false; __local3__ = {__local1__ | __local1__ = true} } else = __local4__ { true; __local4__ = {__local2__ | __local2__ = true} }`),
		},
		{
			note: "set comprehension value in else-func head (reused arg rewrite)",
			mod: MustParseModule(`package head
f(x, y) = {x | y} { 
	false 
} else = {x | y} {
	true 
}
`),
			exp: MustParseRule(`f(__local0__, __local1__) = __local2__ { false; __local2__ = {__local0__ | __local1__} } else = __local3__ { true; __local3__ = {__local0__ | __local1__} }`),
		},
		{
			note: "import in else value",
			mod: MustParseModule(`package head
import input.qux as baz
elsekw {
	false
} else = baz {
	true
}
`),
			exp: MustParseRule(`elsekw { false } else = __local0__ { true; __local0__ = input.qux }`),
		},
		{
			note: "import ref in last ref head term",
			mod: MustParseModule(`package head
import data.doc1 as bar
x.y.z[bar[i]] = true
`),
			exp: MustParseRule(`x.y.z[__local0__] = true { true; __local0__ = data.doc1[i] }`),
		},
		{
			note: "import ref in multi-value ref rule",
			mod: MustParseModule(`package head
import future.keywords.if
import future.keywords.contains
import data.doc1 as bar
x.y.w contains bar[i] if true
`),
			exp: func() *Rule {
				exp, _ := ParseRuleWithOpts(`x.y.w contains __local0__ if {true; __local0__ = data.doc1[i] }`, popts)
				return exp
			}(),
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			c := NewCompiler()
			c.Modules["head"] = tc.mod
			compileStages(c, c.rewriteRefsInHead)
			assertNotFailed(t, c)
			act := c.Modules["head"].Rules[0]
			assertRulesEqual(t, act, tc.exp)
		})
	}
}

func TestCompilerRefHeadsNeedCapability(t *testing.T) {
	popts := ParserOptions{AllFutureKeywords: true, unreleasedKeywords: true}
	for _, tc := range []struct {
		note string
		mod  *Module
		err  string
	}{
		{
			note: "one-dot ref, single-value rule, short+compat",
			mod: MustParseModule(`package t
p[1] = 2`),
		},
		{
			note: "function, short",
			mod: MustParseModule(`package t
p(1)`),
		},
		{
			note: "function",
			mod: MustParseModuleWithOpts(`package t
p(1) if true`, popts),
		},
		{
			note: "function with value",
			mod: MustParseModuleWithOpts(`package t
p(1) = 2 if true`, popts),
		},
		{
			note: "function with value",
			mod: MustParseModule(`package t
p(1) = 2`),
		},
		{
			note: "one-dot ref, single-value rule, compat",
			mod: MustParseModuleWithOpts(`package t
p[3] = 4 if true`, popts),
		},
		{
			note: "multi-value non-ref head",
			mod: MustParseModuleWithOpts(`package t
p contains 1 if true`, popts),
		},
		{ // NOTE(sr): this was previously forbidden because we need the `if` for disambiguation
			note: "one-dot ref head",
			mod: MustParseModuleWithOpts(`package t
p[1] if true`, popts),
			err: "rule heads with refs are not supported: p[1]",
		},
		{
			note: "single-value ref rule",
			mod: MustParseModuleWithOpts(`package t
a.b.c[x] if x := input`, popts),
			err: "rule heads with refs are not supported: a.b.c[x]",
		},
		{
			note: "ref head function",
			mod: MustParseModuleWithOpts(`package t
a.b.c(x) if x == input`, popts),
			err: "rule heads with refs are not supported: a.b.c",
		},
		{
			note: "multi-value ref rule",
			mod: MustParseModuleWithOpts(`package t
a.b.c contains x if x := input`, popts),
			err: "rule heads with refs are not supported: a.b.c",
		},
	} {
		t.Run(tc.note, func(t *testing.T) {
			caps, err := LoadCapabilitiesVersion("v0.44.0")
			if err != nil {
				t.Fatal(err)
			}
			c := NewCompiler().WithCapabilities(caps)
			c.Modules["test"] = tc.mod
			compileStages(c, c.rewriteRefsInHead)
			if tc.err != "" {
				assertErrorWithMessage(t, c.Errors, tc.err)
			} else {
				assertNotFailed(t, c)
			}
		})
	}
}

func TestCompilerRewriteRegoMetadataCalls(t *testing.T) {
	tests := []struct {
		note   string
		module string
		exp    string
	}{
		{
			note: "rego.metadata called, no metadata",
			module: `package test

p {
	rego.metadata.chain()[0].path == ["test", "p"]
	rego.metadata.rule() == {}
}`,
			exp: `package test

p = true {
	__local2__ = [{"path": ["test", "p"]}]
	__local3__ = {}
	__local0__ = __local2__
	equal(__local0__[0].path, ["test", "p"])
	__local1__ = __local3__
	equal(__local1__, {})
}`,
		},
		{
			note: "rego.metadata called, no output var, no metadata",
			module: `package test

p {
	rego.metadata.chain()
	rego.metadata.rule()
}`,
			exp: `package test

p = true {
	__local0__ = [{"path": ["test", "p"]}]
	__local1__ = {}
	__local0__
	__local1__
}`,
		},
		{
			note: "rego.metadata called, with metadata",
			module: `# METADATA
# description: A test package
package test

# METADATA
# title: My P Rule
p {
	rego.metadata.chain()[0].title == "My P Rule"
	rego.metadata.chain()[1].description == "A test package"
}

# METADATA
# title: My Other P Rule
p {
	rego.metadata.rule().title == "My Other P Rule"
}`,
			exp: `# METADATA
# {"scope":"package","description":"A test package"}
package test

# METADATA
# {"scope":"rule","title":"My P Rule"}
p = true {
	__local3__ = [
		{"annotations": {"scope": "rule", "title": "My P Rule"}, "path": ["test", "p"]},
		{"annotations": {"description": "A test package", "scope": "package"}, "path": ["test"]}
	]
	__local0__ = __local3__
	equal(__local0__[0].title, "My P Rule")
	__local1__ = __local3__
	equal(__local1__[1].description, "A test package")
}

# METADATA
# {"scope":"rule","title":"My Other P Rule"}
p = true {
	__local4__ = {"scope": "rule", "title": "My Other P Rule"}
	__local2__ = __local4__
	equal(__local2__.title, "My Other P Rule")
}`,
		},
		{
			note: "rego.metadata referenced multiple times",
			module: `# METADATA
# description: TEST
package test

p {
	rego.metadata.chain()[0].path == ["test", "p"]
	rego.metadata.chain()[1].path == ["test"]
}`,
			exp: `# METADATA
# {"scope":"package","description":"TEST"}
package test

p = true {
	__local2__ = [
		{"path": ["test", "p"]},
		{"annotations": {"description": "TEST", "scope": "package"}, "path": ["test"]}
	]
	__local0__ = __local2__
	equal(__local0__[0].path, ["test", "p"])
	__local1__ = __local2__
	equal(__local1__[1].path, ["test"]) }`,
		},
		{
			note: "rego.metadata return value",
			module: `package test

p := rego.metadata.chain()`,
			exp: `package test

p := __local0__ {
	__local1__ = [{"path": ["test", "p"]}]
	true
	__local0__ = __local1__
}`,
		},
		{
			note: "rego.metadata argument in function call",
			module: `package test

p {
	q(rego.metadata.chain())
}

q(s) {
	s == ["test", "p"]
}`,
			exp: `package test

p = true {
	__local2__ = [{"path": ["test", "p"]}]
	__local1__ = __local2__
	data.test.q(__local1__)
}

q(__local0__) = true {
	equal(__local0__, ["test", "p"])
}`,
		},
		{
			note: "rego.metadata used in array comprehension",
			module: `package test

p = [x | x := rego.metadata.chain()]`,
			exp: `package test

p = [__local0__ | __local1__ = __local2__; __local0__ = __local1__] {
	__local2__ = [{"path": ["test", "p"]}]
	true
}`,
		},
		{
			note: "rego.metadata used in nested array comprehension",
			module: `package test

p {
	y := [x | x := rego.metadata.chain()]
	y[0].path == ["test", "p"]
}`,
			exp: `package test

p = true {
	__local3__ = [{"path": ["test", "p"]}];
	__local1__ = [__local0__ | __local2__ = __local3__; __local0__ = __local2__];
	equal(__local1__[0].path, ["test", "p"])
}`,
		},
		{
			note: "rego.metadata used in set comprehension",
			module: `package test

p = {x | x := rego.metadata.chain()}`,
			exp: `package test

p = {__local0__ | __local1__ = __local2__; __local0__ = __local1__} {
	__local2__ = [{"path": ["test", "p"]}]
	true
}`,
		},
		{
			note: "rego.metadata used in nested set comprehension",
			module: `package test

p {
	y := {x | x := rego.metadata.chain()}
	y[0].path == ["test", "p"]
}`,
			exp: `package test

p = true {
	__local3__ = [{"path": ["test", "p"]}]
	__local1__ = {__local0__ | __local2__ = __local3__; __local0__ = __local2__}
	equal(__local1__[0].path, ["test", "p"])
}`,
		},
		{
			note: "rego.metadata used in object comprehension",
			module: `package test

p = {i: x | x := rego.metadata.chain()[i]}`,
			exp: `package test

p = {i: __local0__ | __local1__ = __local2__; __local0__ = __local1__[i]} {
	__local2__ = [{"path": ["test", "p"]}]
	true
}`,
		},
		{
			note: "rego.metadata used in nested object comprehension",
			module: `package test

p {
	y := {i: x | x := rego.metadata.chain()[i]}
	y[0].path == ["test", "p"]
}`,
			exp: `package test

p = true {
	__local3__ = [{"path": ["test", "p"]}]
	__local1__ = {i: __local0__ | __local2__ = __local3__; __local0__ = __local2__[i]}
	equal(__local1__[0].path, ["test", "p"])
}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			c := NewCompiler()
			c.Modules = map[string]*Module{
				"test.rego": MustParseModule(tc.module),
			}
			compileStages(c, c.rewriteRegoMetadataCalls)
			assertNotFailed(t, c)

			result := c.Modules["test.rego"]
			exp := MustParseModuleWithOpts(tc.exp, ParserOptions{ProcessAnnotation: true})

			if result.Compare(exp) != 0 {
				t.Fatalf("\nExpected:\n\n%v\n\nGot:\n\n%v", exp, result)
			}
		})
	}
}

func TestCompilerOverridingSelfCalls(t *testing.T) {
	c := NewCompiler()
	c.Modules = map[string]*Module{
		"self.rego": MustParseModule(`package self.metadata

chain(x) = "foo"
rule := "bar"`),
		"test.rego": MustParseModule(`package test
import data.self

p := self.metadata.chain(42)
q := self.metadata.rule`),
	}

	compileStages(c, nil)
	assertNotFailed(t, c)
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
				rewrite_with_value_in_assignment {
					a := 1
					b := 1 with input as [a]
				}
			`,
			exp: `
				package test
				rewrite_with_value_in_assignment = true { __local0__ = 1; __local1__ = 1 with input as [__local0__] }
			`,
			expRewrittenMap: map[Var]Var{
				Var("__local0__"): Var("a"),
				Var("__local1__"): Var("b"),
			},
		},
		{
			module: `
				package test
				rewrite_with_value_in_expr {
					a := 1
					a > 0 with input as [a]
				}
			`,
			exp: `
				package test
				rewrite_with_value_in_expr = true { __local0__ = 1; gt(__local0__, 0) with input as [__local0__] }
			`,
			expRewrittenMap: map[Var]Var{
				Var("__local0__"): Var("a"),
			},
		},
		{
			module: `
				package test
				rewrite_nested_with_value_in_expr {
					a := 1
					a > 0 with input as object.union({"a": a}, {"max_a": max([a])})
				}
			`,
			exp: `
				package test
				rewrite_nested_with_value_in_expr = true { __local0__ = 1; gt(__local0__, 0) with input as object.union({"a": __local0__}, {"max_a": max([__local0__])}) }
			`,
			expRewrittenMap: map[Var]Var{
				Var("__local0__"): Var("a"),
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

					f(__local0__) = __local1__ { __local0__ == 1; __local1__ = 2 } else = __local2__ { __local0__ == 3; __local2__ = 4 }
				`)
				module.Rules[0].Else.Head.Args[0].Value = Var("__local0__")
				return module
			},
			expRewrittenMap: map[Var]Var{
				Var("__local0__"): Var("x"),
				Var("__local1__"): Var("y"),
				Var("__local2__"): Var("y"),
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
			note: "single-value rule with ref head",
			module: `
				package test

				p.r.q[s] = t {
					t := 1
					s := input.foo
				}
			`,
			exp: `
				package test

				p.r.q[__local1__] = __local0__ {
					__local0__ = 1
					__local1__ = input.foo
				}
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
			note: "rewrite some: with modifier on domain",
			module: `
				package test
				p {
					some k, x in input with input as [1, 1, 1]
					k == 0
					x == 1
				}
			`,
			exp: `
				package test
				p {
					__local1__ = input[__local0__] with input as [1, 1, 1]
					__local0__ = 0
					__local1__ = 1
				}
			`,
		},
		{
			note: "rewrite every",
			module: `
				package test
				# import future.keywords.in
				# import future.keywords.every
				i = 0
				xs = [1, 2]
				k = "foo"
				v = "bar"
				p {
					every k, v in xs { k + v > i }
				}
			`,
			exp: `
				package test
				i = 0
				xs = [1, 2]
				k = "foo"
				v = "bar"
				p = true {
					__local2__ = data.test.xs
					every __local0__, __local1__ in __local2__ {
						plus(__local0__, __local1__, __local3__)
						__local4__ = data.test.i
						gt(__local3__, __local4__)
					}
				}			`,
		},
		{
			note: "rewrite every: unused key var",
			module: `
				package test
				# import future.keywords.in
				# import future.keywords.every
				p {
					every k, v in [1] { v >= 1 }
				}
			`,
			wantErr: errors.New("declared var k unused"),
		},
		{
			// NOTE(sr): this would happen when compiling modules twice:
			// the first run rewrites every to include a generated key var,
			// the second one bails because it's not used.
			// Seen in the wild when using `opa test -b` on a bundle that
			// used `every`, https://github.com/open-policy-agent/opa/issues/4420
			note: "rewrite every: unused generated key var",
			module: `
				package test

				p {
					every __local0__, v in [1] { v >= 1 }
				}
			`,
			exp: `
				package test
				p = true {
					__local3__ = [1]
					every __local1__, __local2__ in __local3__ { __local2__ >= 1 }
				}
		`,
		},
		{
			note: "rewrite every: unused value var",
			module: `
				package test
				# import future.keywords.in
				# import future.keywords.every
				p {
					every v in [1] { true }
				}
			`,
			wantErr: errors.New("declared var v unused"),
		},
		{
			note: "rewrite every: wildcard value var, used key",
			module: `
				package test
				# import future.keywords.in
				# import future.keywords.every
				p {
					every k, _ in [1] { k >= 0 }
				}
			`,
			exp: `
				package test
				p = true {
					__local1__ = [1]
					every __local0__, _ in __local1__ { gte(__local0__, 0) }
				}
			`,
		},
		{
			note: "rewrite every: wildcard key+value var", // NOTE(sr): may be silly, but valid
			module: `
				package test
				# import future.keywords.in
				# import future.keywords.every
				p {
					every _, _ in [1] { true }
				}
			`,
			exp: `
				package test
				p = true { __local0__ = [1]; every _, _ in __local0__ { true } }
			`,
		},
		{
			note: "rewrite every: declared vars with different scopes",
			module: `
				package test
				# import future.keywords.in
				# import future.keywords.every
				p {
					some x
					x = 10
					every x in [1] { x == 1 }
				}
			`,
			exp: `
				package test
				p = true {
					__local0__ = 10
					__local3__ = [1]
					every __local1__, __local2__ in __local3__ { __local2__ = 1 }
				}
			`,
		},
		{
			note: "rewrite every: declared vars used in body",
			module: `
				package test
				# import future.keywords.in
				# import future.keywords.every
				p {
					some y
					y = 10
					every x in [1] { x == y }
				}
			`,
			exp: `
				package test
				p = true {
					__local0__ = 10
					__local3__ = [1]
					every __local1__, __local2__ in __local3__ {
						__local2__ = __local0__
					}
				}
			`,
		},
		{
			note: "rewrite every: pops declared var stack",
			module: `
				package test
				# import future.keywords.in
				# import future.keywords.every
				p[x] {
					some x
					x = 10
					every _ in [1] { true }
				}
			`,
			exp: `
				package test
				p[__local0__] { __local0__ = 10; __local2__ = [1]; every __local1__, _ in __local2__ { true } }
			`,
		},
		{
			note: "rewrite every: nested",
			module: `
				package test
				# import future.keywords.in
				# import future.keywords.every
				p {
					xs := [[1], [2]]
					every v in [1] {
						every w in xs[v] {
							w == 2
						}
					}
				}
			`,
			exp: `
				package test
				p = true {
					__local0__ = [[1], [2]]
					__local5__ = [1]
					every __local1__, __local2__ in __local5__ {
						__local6__ = __local0__[__local2__]
						every __local3__, __local4__ in __local6__ {
							__local4__ = 2
						}
					}
				}
			`,
		},
		{
			note: "rewrite every: with modifier on domain",
			module: `
				package test
				# import future.keywords.in
				# import future.keywords.every
				p {
					every x in input { x == 1 } with input as [1, 1, 1]
				}
			`,
			exp: `
				package test
				p {
					__local2__ = input with input as [1, 1, 1]
					every __local0__, __local1__ in __local2__ {
						__local1__ = 1
					} with input as [1, 1, 1]
				}
			`,
		},
		{
			note: "rewrite every: with modifier on domain with declared var",
			module: `
				package test
				# import future.keywords.in
				# import future.keywords.every
				p {
					xs := [1, 2]
					every x in input { x == 1 } with input as xs
				}
			`,
			exp: `
				package test
				p {
					__local0__ = [1, 2]
					__local3__ = input with input as __local0__
					every __local1__, __local2__ in __local3__ {
						__local2__ = 1
					} with input as __local0__
				}
			`,
		},
		{
			note: "rewrite every: with modifier on body",
			module: `
				package test
				# import future.keywords.in
				# import future.keywords.every
				p {
					every x in [2] { x == input } with input as 2
				}
			`,
			exp: `
				package test
				p {
					__local2__ = [2] with input as 2
					every __local0__, __local1__ in __local2__ {
						__local1__ = input
					} with input as 2
				}
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
			opts := CompileOpts{ParserOptions: ParserOptions{AllFutureKeywords: true, unreleasedKeywords: true}}
			compiler, err := CompileModulesWithOpt(map[string]string{"test.rego": tc.module}, opts)
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
				exp := MustParseModuleWithOpts(tc.exp, opts.ParserOptions)
				result := compiler.Modules["test.rego"]
				if exp.Compare(result) != 0 {
					t.Fatalf("Expected:\n\n%v\n\nGot:\n\n%v", exp, result)
				}
			}
		})
	}
}

func TestCheckUnusedFunctionArgVars(t *testing.T) {
	tests := []strictnessTestCase{
		{
			note: "one of the two function args is not used - issue 5602 regression test",
			module: `package test
			func(x, y) {
				x = 1
			}`,
			expectedErrors: Errors{
				&Error{
					Code:     CompileErr,
					Location: NewLocation([]byte("func(x, y)"), "", 2, 4),
					Message:  "unused argument y. (hint: use _ (wildcard variable) instead)",
				},
			},
		},
		{
			note: "one of the two ref-head function args is not used",
			module: `package test
			a.b.c.func(x, y) {
				x = 1
			}`,
			expectedErrors: Errors{
				&Error{
					Code:     CompileErr,
					Location: NewLocation([]byte("a.b.c.func(x, y)"), "", 2, 4),
					Message:  "unused argument y. (hint: use _ (wildcard variable) instead)",
				},
			},
		},
		{
			note: "multiple unused argvar in scope - issue 5602 regression test",
			module: `package test
			func(x, y) {
				input.baz = 1
				input.test == "foo"
			}`,
			expectedErrors: Errors{
				&Error{
					Code:     CompileErr,
					Location: NewLocation([]byte("func(x, y)"), "", 2, 4),
					Message:  "unused argument x. (hint: use _ (wildcard variable) instead)",
				},
				&Error{
					Code:     CompileErr,
					Location: NewLocation([]byte("func(x, y)"), "", 2, 4),
					Message:  "unused argument y. (hint: use _ (wildcard variable) instead)",
				},
			},
		},
		{
			note: "some unused argvar in scope - issue 5602 regression test",
			module: `package test
			func(x, y) {
				input.test == "foo"
				x = 1
			}`,
			expectedErrors: Errors{
				&Error{
					Code:     CompileErr,
					Location: NewLocation([]byte("func(x, y)"), "", 2, 4),
					Message:  "unused argument y. (hint: use _ (wildcard variable) instead)",
				},
			},
		},
		{
			note: "wildcard argvar that's ignored - issue 5602 regression test",
			module: `package test
			func(x, _) {
				input.test == "foo"
				x = 1
			}`,
			expectedErrors: Errors{},
		},
		{
			note: "wildcard argvar that's ignored - issue 5602 regression test",
			module: `package test
			func(x, _) {
				input.test == "foo"
			 }`,
			expectedErrors: Errors{
				&Error{
					Code:     CompileErr,
					Location: NewLocation([]byte("func(x, _)"), "", 2, 4),
					Message:  "unused argument x. (hint: use _ (wildcard variable) instead)",
				},
			},
		},
		{
			note: "argvar not used in body but in head - issue 5602 regression test",
			module: `package test
			func(x) := x {
				input.test == "foo"
			}`,
			expectedErrors: Errors{},
		},
		{
			note: "argvar not used in body but in head value comprehension",
			module: `package test
			a := {"foo": 1}
			func(x) := { x: v | v := a[x] } {
				input.test == "foo"
			}`,
			expectedErrors: Errors{},
		},
		{
			note: "argvar not used in body but in else-head value comprehension",
			module: `package test
			a := {"foo": 1}
			func(x) {
				input.test == "foo"
			} else := { x: v | v := a[x] } {
				input.test == "bar"
			}`,
			expectedErrors: Errors{},
		},
		{
			note: "argvar not used in body and shadowed in head value comprehension",
			module: `package test
			a := {"foo": 1}
			func(x) := { x: v | x := "foo"; v := a[x] } {
				input.test == "foo"
			}`,
			expectedErrors: Errors{
				&Error{
					Code:     CompileErr,
					Location: NewLocation([]byte("func(x) := { x: v | x := \"foo\"; v := a[x] }"), "", 3, 4),
					Message:  "unused argument x. (hint: use _ (wildcard variable) instead)",
				},
			},
		},
		{
			note: "argvar used in primary body but not in else body",
			module: `package test
			func(x) {
				input.test == x
			} else := false {
				input.test == "foo"
			}`,
			expectedErrors: Errors{},
		},
		{
			note: "argvar used in primary body but not in else body (with wildcard)",
			module: `package test
			func(x, _) {
				input.test == x
			} else := false {
				input.test == "foo"
			}`,
			expectedErrors: Errors{},
		},
		{
			note: "argvar not used in primary body but in else body",
			module: `package test
			func(x) {
				input.test == "foo"
			} else := false {
				input.test == x
			}`,
			expectedErrors: Errors{},
		},
		{
			note: "argvar not used in primary body but in else body (with wildcard)",
			module: `package test
			func(x, _) {
				input.test == "foo"
			} else := false {
				input.test == x
			}`,
			expectedErrors: Errors{},
		},
		{
			note: "argvar used in primary body but not in implicit else body",
			module: `package test
			func(x) {
				input.test == x
			} else := false`,
			expectedErrors: Errors{},
		},
		{
			note: "argvars usage spread over multiple bodies",
			module: `package test
			func(x, y, z) {
				input.test == x
			} else {
				input.test == y
			} else {
				input.test == z
			}`,
			expectedErrors: Errors{},
		},
		{
			note: "argvars usage spread over multiple bodies, missing in first",
			module: `package test
			func(x, y, z) {
				input.test == "foo"
			} else {
				input.test == y
			} else {
				input.test == z
			}`,
			expectedErrors: Errors{
				&Error{
					Code:     CompileErr,
					Location: NewLocation([]byte("func(x, y, z)"), "", 2, 4),
					Message:  "unused argument x. (hint: use _ (wildcard variable) instead)",
				},
			},
		},
		{
			note: "argvars usage spread over multiple bodies, missing in second",
			module: `package test
			func(x, y, z) {
				input.test == x
			} else {
				input.test == "bar"
			} else {
				input.test == z
			}`,
			expectedErrors: Errors{
				&Error{
					Code:     CompileErr,
					Location: NewLocation([]byte("func(x, y, z)"), "", 2, 4),
					Message:  "unused argument y. (hint: use _ (wildcard variable) instead)",
				},
			},
		},
		{
			note: "argvars usage spread over multiple bodies, missing in third",
			module: `package test
			func(x, y, z) {
				input.test == x
			} else {
				input.test == y
			} else {
				input.test == "baz"
			}`,
			expectedErrors: Errors{
				&Error{
					Code:     CompileErr,
					Location: NewLocation([]byte("func(x, y, z)"), "", 2, 4),
					Message:  "unused argument z. (hint: use _ (wildcard variable) instead)",
				},
			},
		},
		{
			note: "unused default function argvar",
			module: `package test
			default func(x) := 0`,
			expectedErrors: Errors{
				&Error{
					Code:     CompileErr,
					Location: NewLocation([]byte("func(x) := 0"), "", 2, 12),
					Message:  "unused argument x. (hint: use _ (wildcard variable) instead)",
				},
			},
		},
	}

	t.Helper()
	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			compiler := NewCompiler().WithStrict(true)
			compiler.Modules = map[string]*Module{
				"test": MustParseModule(tc.module),
			}
			compileStages(compiler, nil)

			assertErrors(t, compiler.Errors, tc.expectedErrors, true)
		})
	}
}

func TestCompileUnusedAssignedVarsErrorLocations(t *testing.T) {
	tests := []strictnessTestCase{
		{
			note: "one of the two function args is not used - issue 5662 regression test",
			module: `package test
			func(x, y) {
				x = 1
			}`,
			expectedErrors: Errors{
				&Error{
					Code:     CompileErr,
					Location: NewLocation([]byte("func(x, y)"), "", 2, 4),
					Message:  "unused argument y. (hint: use _ (wildcard variable) instead)",
				},
			},
		},
		{
			note: "multiple unused assigned var in scope - issue 5662 regression test",
			module: `package test
			allow {
				input.message == "world"
				input.test == "foo"
				input.x == "foo"
				input.y == "baz"
				a := 1
				b := 2
				x := {
					"a": a,
					"b": "bar",
				}
				input.z == "baz"
				c := 3
			}`,
			expectedErrors: Errors{
				&Error{
					Code:     CompileErr,
					Location: NewLocation([]byte("b := 2"), "", 8, 5),
					Message:  "assigned var b unused",
				},
				&Error{
					Code:     CompileErr,
					Location: NewLocation([]byte("x := {\n\t\t\t\t\t\"a\": a,\n\t\t\t\t\t\"b\": \"bar\",\n\t\t\t\t}"), "", 9, 5),
					Message:  "assigned var x unused",
				},
				&Error{
					Code:     CompileErr,
					Location: NewLocation([]byte("c := 3"), "", 14, 5),
					Message:  "assigned var c unused",
				},
			},
		},
	}

	t.Helper()
	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			compiler := NewCompiler().WithStrict(true)
			compiler.Modules = map[string]*Module{
				"test": MustParseModule(tc.module),
			}
			compileStages(compiler, nil)
			assertErrors(t, compiler.Errors, tc.expectedErrors, true)
		})
	}

}

func TestCompileUnusedDeclaredVarsErrorLocations(t *testing.T) {
	tests := []strictnessTestCase{
		{
			note: "simple unused some var - issue 4238 regression test",
			module: `package test

			foo {
				print("Hello world")
				some i
			}`,
			expectedErrors: Errors{
				&Error{
					Code:     CompileErr,
					Location: NewLocation([]byte("some i"), "", 5, 5),
					Message:  "declared var i unused",
				},
			},
		},
		{
			note: "simple unused some vars, 2x rules",
			module: `package test

			foo {
				print("Hello world")
				some i
			}
			
			bar {
				print("Hello world")
				some j
			}`,
			expectedErrors: Errors{
				&Error{
					Code:     CompileErr,
					Location: NewLocation([]byte("some i"), "", 5, 5),
					Message:  "declared var i unused",
				},
				&Error{
					Code:     CompileErr,
					Location: NewLocation([]byte("some j"), "", 10, 5),
					Message:  "declared var j unused",
				},
			},
		},
		{
			note: "multiple unused some vars",
			module: `package test

			x := [1, 1, 1]
			foo2 {
				print("A")
				some a, b, c
				some i, j
				some k
				x[b] == 1
				print("B")
			}`,
			expectedErrors: Errors{
				&Error{
					Code:     CompileErr,
					Location: NewLocation([]byte("some a, b, c"), "", 6, 5),
					Message:  "declared var a unused",
				},
				&Error{
					Code:     CompileErr,
					Location: NewLocation([]byte("some a, b, c"), "", 6, 5),
					Message:  "declared var c unused",
				},
				&Error{
					Code:     CompileErr,
					Location: NewLocation([]byte("some i, j"), "", 7, 5),
					Message:  "declared var i unused",
				},
				&Error{
					Code:     CompileErr,
					Location: NewLocation([]byte("some i, j"), "", 7, 5),
					Message:  "declared var j unused",
				},
				&Error{
					Code:     CompileErr,
					Location: NewLocation([]byte("some k"), "", 8, 5),
					Message:  "declared var k unused",
				},
			},
		},
	}

	// This is similar to the logic for runStrictnessTestCase(), but expects
	// unconditional compiler errors.
	t.Helper()
	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			compiler := NewCompiler().WithStrict(true)
			compiler.Modules = map[string]*Module{
				"test": MustParseModule(tc.module),
			}
			compileStages(compiler, nil)

			assertErrors(t, compiler.Errors, tc.expectedErrors, true)
		})
	}
}

func TestCompileInvalidEqAssignExpr(t *testing.T) {

	c := NewCompiler()

	c.Modules["error"] = MustParseModule(`package errors


	p {
		# Arity mismatches are caught in the checkUndefinedFuncs check,
		# and invalid eq/assign calls are passed along until then.
		assign()
		assign(1)
		eq()
		eq(1)
	}`)

	var prev func()
	checkUndefinedFuncs := reflect.ValueOf(c.checkUndefinedFuncs)

	for _, stage := range c.stages {
		if reflect.ValueOf(stage.f).Pointer() == checkUndefinedFuncs.Pointer() {
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
		{`every_domain { every _ in str { true } }`, `__local1__ = data.test.str; every __local0__, _ in __local1__ { true }`},
		{`every_domain_call { every _ in numbers.range(1, 10) { true } }`, `numbers.range(1, 10, __local1__); every __local0__, _ in __local1__ { true }`},
		{`every_body { every _ in [] { [str] } }`,
			`__local1__ = []; every __local0__, _ in __local1__ { __local2__ = data.test.str; [__local2__] }`},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			c := NewCompiler()
			opts := ParserOptions{AllFutureKeywords: true, unreleasedKeywords: true}
			module := fixture + tc.input
			c.Modules["test"] = MustParseModuleWithOpts(module, opts)
			compileStages(c, c.rewriteDynamicTerms)
			assertNotFailed(t, c)
			expected := MustParseBodyWithOpts(tc.expected, opts)
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
		note         string
		input        string
		opts         func(*Compiler) *Compiler
		expected     string
		expectedRule *Rule
		wantErr      error
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
			wantErr: fmt.Errorf("rego_type_error: with keyword target must reference existing input, data, or a function"),
		},
		{
			note:     "built-in function: replaced by (unknown) var",
			input:    `p { true with time.now_ns as foo }`,
			expected: `p { true with time.now_ns as foo }`, // `foo` still a Var here
		},
		{
			note: "built-in function: valid, arity 0",
			input: `
				p { true with time.now_ns as now }
				now() = 1
			`,
			expected: `p { true with time.now_ns as data.test.now }`,
		},
		{
			note: "built-in function: valid func ref, arity 1",
			input: `
				p { true with http.send as mock_http_send }
				mock_http_send(_) = { "body": "yay" }
			`,
			expected: `p { true with http.send as data.test.mock_http_send }`,
		},
		{
			note: "built-in function: replaced by value",
			input: `
				p { true with http.send as { "body": "yay" } }
			`,
			expected: `p { true with http.send as {"body": "yay"} }`,
		},
		{
			note: "built-in function: replaced by var",
			input: `
				p {
					resp := { "body": "yay" }
					true with http.send as resp
				}
			`,
			expected: `p { __local0__ = {"body": "yay"}; true with http.send as __local0__ }`,
		},
		{
			note: "non-built-in function: replaced by var",
			input: `
				p {
					resp := true
					f(true) with f as resp
				}
				f(false) { true }
			`,
			expected: `p { __local0__ = true; data.test.f(true) with data.test.f as __local0__ }`,
		},
		{
			note: "built-in function: replaced by comprehension",
			input: `
				p { true with http.send as { x: true | x := ["a", "b"][_] } }
			`,
			expected: `p { __local2__ = {__local0__: true | __local1__ = ["a", "b"]; __local0__ = __local1__[_]}; true with http.send as __local2__ }`,
		},
		{
			note: "built-in function: replaced by ref",
			input: `
				p { true with http.send as resp }
				resp := { "body": "yay" }
			`,
			expected: `p { true with http.send as data.test.resp }`,
		},
		{
			note: "built-in function: replaced by another built-in (ref)",
			input: `
				p { true with http.send as object.union_n }
			`,
			expected: `p { true with http.send as object.union_n }`,
		},
		{
			note: "built-in function: replaced by another built-in (simple)",
			input: `
				p { true with http.send as count }
			`,
			expectedRule: func() *Rule {
				r := MustParseRule(`p { true with http.send as count }`)
				r.Body[0].With[0].Value.Value = Ref([]*Term{VarTerm("count")})
				return r
			}(),
		},
		{
			note: "built-in function: replaced by another built-in that's marked unsafe",
			input: `
				q := is_object({"url": "https://httpbin.org", "method": "GET"})
				p { q with is_object as http.send }
			`,
			opts:    func(c *Compiler) *Compiler { return c.WithUnsafeBuiltins(map[string]struct{}{"http.send": {}}) },
			wantErr: fmt.Errorf("rego_compile_error: with keyword replacing built-in function: target must not be unsafe: \"http.send\""),
		},
		{
			note: "non-built-in function: replaced by another built-in that's marked unsafe",
			input: `
			r(_) = {}
			q := r({"url": "https://httpbin.org", "method": "GET"})
			p {
				q with r as http.send
			}`,
			opts:    func(c *Compiler) *Compiler { return c.WithUnsafeBuiltins(map[string]struct{}{"http.send": {}}) },
			wantErr: fmt.Errorf("rego_compile_error: with keyword replacing built-in function: target must not be unsafe: \"http.send\""),
		},
		{
			note: "built-in function: valid, arity 1, non-compound name",
			input: `
				p { concat("/", input) with concat as mock_concat }
				mock_concat(_, _) = "foo/bar"
			`,
			expectedRule: func() *Rule {
				r := MustParseRule(`p { concat("/", input) with concat as data.test.mock_concat }`)
				r.Body[0].With[0].Target.Value = Ref([]*Term{VarTerm("concat")})
				return r
			}(),
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			c := NewCompiler()
			if tc.opts != nil {
				c = tc.opts(c)
			}
			module := fixture + tc.input
			c.Modules["test"] = MustParseModule(module)
			compileStages(c, c.rewriteWithModifiers)
			if tc.wantErr == nil {
				assertNotFailed(t, c)
				expected := tc.expectedRule
				if expected == nil {
					expected = MustParseRule(tc.expected)
				}
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
		{
			note: "every body",
			module: `package test

			p { every _ in [] { false; print(1) } }
			`,
			exp: `package test

			p = true { __local1__ = []; every __local0__, _ in __local1__ { false } }`,
		},
		{
			note: "in head",
			module: `package test

			p = {1 | print("x")}`,
			exp: `package test

			p = __local0__ { true; __local0__ = {1 | true} }`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			c := NewCompiler().WithEnablePrintStatements(false)
			opts := ParserOptions{AllFutureKeywords: true, unreleasedKeywords: true}
			c.Compile(map[string]*Module{
				"test.rego": MustParseModuleWithOpts(tc.module, opts),
			})
			if c.Failed() {
				t.Fatal(c.Errors)
			}
			exp := MustParseModuleWithOpts(tc.exp, opts)
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
			note: "print inside every",
			module: `package test

			p { every x in [1,2] { print(x) } }`,
			exp: `package test

			p = true {
				__local3__ = [1, 2]
				every __local0__, __local1__ in __local3__ {
					__local4__ = {__local2__ | __local2__ = __local1__}
					internal.print([__local4__])
				}
			}`,
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
		{
			note: "print call in head",
			module: `package test

			p = {1 | print("x") }`,
			exp: `package test

			p = __local1__ {
				true
				__local1__ = {1 | __local2__ = { __local0__ | __local0__ = "x"}; internal.print([__local2__])}
			}`,
		},
		{
			note: "print call in head - args treated as safe",
			module: `package test

			f(a) = {1 | a[x]; print(x)}`,
			exp: `package test

			f(__local0__) = __local2__ { true; __local2__ = {1 | __local0__[x]; __local3__ = {__local1__ | __local1__ = x}; internal.print([__local3__])} }
			`,
		},
		{
			note: "print call of var in head key",
			module: `package test
			f(_) = [1, 2, 3]
			p[x] { [_, x, _] := f(true); print(x) }`,
			exp: `package test
			f(__local0__) = [1, 2, 3] { true }
			p[__local2__] { data.test.f(true, __local5__); [__local1__, __local2__, __local3__] = __local5__; __local6__ = {__local4__ | __local4__ = __local2__}; internal.print([__local6__]) }
			`,
		},
		{
			note: "print call of var in head value",
			module: `package test
			f(_) = [1, 2, 3]
			p = x { [_, x, _] := f(true); print(x) }`,
			exp: `package test
			f(__local0__) = [1, 2, 3] { true }
			p = __local2__ { data.test.f(true, __local5__); [__local1__, __local2__, __local3__] = __local5__; __local6__ = {__local4__ | __local4__ = __local2__}; internal.print([__local6__]) }
			`,
		},
		{
			note: "print call of vars in head key and value",
			module: `package test
			f(_) = [1, 2, 3]
			p[x] = y { [_, x, y] := f(true); print(x) }`,
			exp: `package test
			f(__local0__) = [1, 2, 3] { true }
			p[__local2__] = __local3__ { data.test.f(true, __local5__); [__local1__, __local2__, __local3__] = __local5__; __local6__ = {__local4__ | __local4__ = __local2__}; internal.print([__local6__]) }
			`,
		},
		{
			note: "print call of vars altered with 'with' and call",
			module: `package test
			q = input
			p {
				x := q with input as json.unmarshal("{}")
				print(x)
			}`,
			exp: `package test
			q = __local3__ { true; __local3__ = input }
			p = true {
				json.unmarshal("{}", __local2__)
				__local0__ = data.test.q with input as __local2__
				__local4__ = {__local1__ | __local1__ = __local0__}
				internal.print([__local4__])
			}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			c := NewCompiler().WithEnablePrintStatements(true)
			opts := ParserOptions{AllFutureKeywords: true, unreleasedKeywords: true}
			c.Compile(map[string]*Module{
				"test.rego": MustParseModuleWithOpts(tc.module, opts),
			})
			if c.Failed() {
				t.Fatal(c.Errors)
			}
			exp := MustParseModuleWithOpts(tc.exp, opts)
			if !exp.Equal(c.Modules["test.rego"]) {
				t.Fatalf("Expected:\n\n%v\n\nGot:\n\n%v", exp, c.Modules["test.rego"])
			}
		})
	}
}

func TestRewritePrintCallsWithElseImplicitArgs(t *testing.T) {

	module := `package test

	f(x, y) {
		x = y
	}

	else = false {
		print(x, y)
	}`

	c := NewCompiler().WithEnablePrintStatements(true)
	opts := ParserOptions{AllFutureKeywords: true, unreleasedKeywords: true}
	c.Compile(map[string]*Module{
		"test.rego": MustParseModuleWithOpts(module, opts),
	})

	if c.Failed() {
		t.Fatal(c.Errors)
	}

	exp := MustParseModuleWithOpts(`package test

	f(__local0__, __local1__) = true { __local0__ = __local1__ }
	else = false { __local4__ = {__local2__ | __local2__ = __local0__}; __local5__ = {__local3__ | __local3__ = __local1__}; internal.print([__local4__, __local5__]) }
	`, opts)

	// NOTE(tsandall): we have to patch the implicit args on the else rule
	// because of how the parser copies the arg names across from the first
	// rule.
	exp.Rules[0].Else.Head.Args[0] = VarTerm("__local0__")
	exp.Rules[0].Else.Head.Args[1] = VarTerm("__local1__")

	if !exp.Equal(c.Modules["test.rego"]) {
		t.Fatalf("Expected:\n\n%v\n\nGot:\n\n%v", exp, c.Modules["test.rego"])
	}
}

func TestCompilerMockFunction(t *testing.T) {
	tests := []struct {
		note          string
		module, extra string
		err           string
	}{
		{
			note: "simple valid",
			module: `package test
				now() = 123
				p { true with time.now_ns as now }
			`,
		},
		{
			note: "simple valid, simple name",
			module: `package test
				mock_concat(_, _) = "foo/bar"
				p { concat("/", input) with concat as mock_concat }
			`,
		},
		{
			note: "invalid ref: nonexistant",
			module: `package test
				p { true with time.now_ns as now }
			`,
			err: "rego_unsafe_var_error: var now is unsafe", // we're running all compiler stages here
		},
		{
			note: "valid ref: not a function, but arity = 0",
			module: `package test
				now = 1
				p { true with time.now_ns as now }
			`,
		},
		{
			note: "ref: not a function, arity > 0",
			module: `package test
				http_send = { "body": "nope" }
				p { true with http.send as http_send }
			`,
		},
		{
			note: "invalid ref: arity mismatch",
			module: `package test
				http_send(_, _) = { "body": "nope" }
				p { true with http.send as http_send }
			`,
			err: "rego_type_error: http.send: arity mismatch\n\thave: (any, any)\n\twant: (request: object[string: any])",
		},
		{
			note: "invalid ref: arity mismatch (in call)",
			module: `package test
				http_send(_, _) = { "body": "nope" }
				p { http.send({}) with http.send as http_send }
			`,
			err: "rego_type_error: http.send: arity mismatch\n\thave: (any, any)\n\twant: (request: object[string: any])",
		},
		{
			note: "invalid ref: value another built-in with different type",
			module: `package test
				p { true with http.send as net.lookup_ip_addr }
			`,
			err: "rego_type_error: http.send: arity mismatch\n\thave: (string)\n\twant: (request: object[string: any])",
		},
		{
			note: "ref: value another built-in with compatible type",
			module: `package test
				p { true with count as object.union_n }
			`,
		},
		{
			note: "valid: package import",
			extra: `package mocks
				http_send(_) = {}
			`,
			module: `package test
				import data.mocks
				p { true with http.send as mocks.http_send }
			`,
		},
		{
			note: "valid: function import",
			extra: `package mocks
				http_send(_) = {}
			`,
			module: `package test
				import data.mocks.http_send
				p { true with http.send as http_send }
			`,
		},
		{
			note: "invalid target: relation",
			module: `package test
				my_walk(_, _)
				p { true with walk as my_walk }
			`,
			err: "rego_compile_error: with keyword replacing built-in function: target must not be a relation",
		},
		{
			note: "invalid target: eq",
			module: `package test
				my_eq(_, _)
				p { true with eq as my_eq }
			`,
			err: `rego_compile_error: with keyword replacing built-in function: replacement of "eq" invalid`,
		},
		{
			note: "invalid target: rego.metadata.chain",
			module: `package test
				p { true with rego.metadata.chain as [] }
			`,
			err: `rego_compile_error: with keyword replacing built-in function: replacement of "rego.metadata.chain" invalid`,
		},
		{
			note: "invalid target: rego.metadata.rule",
			module: `package test
				p { true with rego.metadata.rule as {} }
			`,
			err: `rego_compile_error: with keyword replacing built-in function: replacement of "rego.metadata.rule" invalid`,
		},
		{
			note: "invalid target: internal.print",
			module: `package test
				my_print(_, _)
				p { true with internal.print as my_print }
			`,
			err: `rego_compile_error: with keyword replacing built-in function: replacement of internal function "internal.print" invalid`,
		},
		{
			note: "mocking custom built-in",
			module: `package test
				mock(_)
				mock_mock(_)
				p { bar(foo.bar("one")) with bar as mock with foo.bar as mock_mock }
			`,
		},
		{
			note: "non-built-in function replaced value",
			module: `package test
				original(_)
				p { original(true) with original as 123 }
			`,
		},
		{
			note: "non-built-in function replaced by another, arity 0",
			module: `package test
				original() = 1
				mock() = 2
				p { original() with original as mock }
			`,
			err: "rego_type_error: undefined function data.test.original", // TODO(sr): file bug -- this doesn't depend on "with" used or not
		},
		{
			note: "non-built-in function replaced by another, arity 1",
			module: `package test
				original(_)
				mock(_)
				p { original(true) with original as mock }
			`,
		},
		{
			note: "non-built-in function replaced by built-in",
			module: `package test
				original(_)
				p { original([1]) with original as count }
			`,
		},
		{
			note: "non-built-in function replaced by another, arity mismatch",
			module: `package test
				original(_)
				mock(_, _)
				p { original([1]) with original as mock }
			`,
			err: "rego_type_error: data.test.original: arity mismatch\n\thave: (any, any)\n\twant: (any)",
		},
		{
			note: "non-built-in function replaced by built-in, arity mismatch",
			module: `package test
				original(_)
				p { original([1]) with original as concat }
			`,
			err: "rego_type_error: data.test.original: arity mismatch\n\thave: (string, any<array[string], set[string]>)\n\twant: (any)",
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			c := NewCompiler().WithBuiltins(map[string]*Builtin{
				"bar": {
					Name: "bar",
					Decl: types.NewFunction([]types.Type{types.S}, types.A),
				},
				"foo.bar": {
					Name: "foo.bar",
					Decl: types.NewFunction([]types.Type{types.S}, types.A),
				},
			})
			if tc.extra != "" {
				c.Modules["extra"] = MustParseModule(tc.extra)
			}
			c.Modules["test"] = MustParseModule(tc.module)

			// NOTE(sr): We're running all compiler stages here, since the type checking of
			// built-in function replacements happens at the type check stage.
			c.Compile(c.Modules)

			if tc.err != "" {
				if !strings.Contains(c.Errors.Error(), tc.err) {
					t.Errorf("expected error to contain %q, got %q", tc.err, c.Errors.Error())
				}
			} else if len(c.Errors) > 0 {
				t.Errorf("expected no errors, got %v", c.Errors)
			}
		})
	}

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

func TestCompilerCheckUnusedAssignedVar(t *testing.T) {
	type testCase struct {
		note           string
		module         string
		expectedErrors Errors
	}

	cases := []testCase{
		{
			note: "global var",
			module: `package test
				x := 1
			`,
		},
		{
			note: "simple rule with wildcard",
			module: `package test
				p {
					_ := 1
				}
			`,
		},
		{
			note: "simple rule",
			module: `package test
				p {
					x := 1
					y := 2
					z := x + 3
				}
			`,
			expectedErrors: Errors{
				&Error{Message: "assigned var y unused"},
				&Error{Message: "assigned var z unused"},
			},
		},
		{
			note: "rule with return",
			module: `package test
				p = x {
					x := 2
					y := 3
				}
			`,
			expectedErrors: Errors{
				&Error{Message: "assigned var y unused"},
			},
		},
		{
			note: "rule with function call",
			module: `package test
				p {
					x := 2
					y := f(x)
				}
			`,
			expectedErrors: Errors{
				&Error{Message: "assigned var y unused"},
			},
		},
		{
			note: "rule with nested array comprehension",
			module: `package test
				p {
					x := 2
					y := [z | z := 2 * x]
				}
			`,
			expectedErrors: Errors{
				&Error{Message: "assigned var y unused"},
			},
		},
		{
			note: "rule with nested array comprehension and shadowing",
			module: `package test
				p {
					x := 2
					y := [x | x := 2 * x]
				}
			`,
			expectedErrors: Errors{
				&Error{Message: "assigned var y unused"},
			},
		},
		{
			note: "rule with nested array comprehension and shadowing (unused shadowed var)",
			module: `package test
				p {
					x := 2
					y := [x | x := 2]
				}
			`,
			expectedErrors: Errors{
				&Error{Message: "assigned var x unused"},
				&Error{Message: "assigned var y unused"},
			},
		},
		{
			note: "rule with nested array comprehension and shadowing (unused shadowing var)",
			module: `package test
				p {
					x := 2
					x > 1
					[1 | x := 2]
				}
			`,
			expectedErrors: Errors{
				&Error{Message: "assigned var x unused"},
			},
		},
		{
			note: "rule with nested array comprehension and some declaration",
			module: `package test
				p {
					some i
					_ := [z | z := [1, 2][i]]
				}
			`,
		},
		{
			note: "rule with nested set comprehension",
			module: `package test
				p {
					x := 2
					y := {z | z := 2 * x}
				}
			`,
			expectedErrors: Errors{
				&Error{Message: "assigned var y unused"},
			},
		},
		{
			note: "rule with nested set comprehension and unused inner var",
			module: `package test
				p {
					x := 2
					y := {z | z := 2 * x; a := 2}
				}
			`,
			expectedErrors: Errors{
				&Error{Message: "assigned var a unused"}, // y isn't reported, as we abort early on errors when moving through the stack
			},
		},
		{
			note: "rule with nested object comprehension",
			module: `package test
				p {
					x := 2
					y := {z: x | z := 2 * x}
				}
			`,
			expectedErrors: Errors{
				&Error{Message: "assigned var y unused"},
			},
		},
		{
			note: "rule with nested closure",
			module: `package test
				p {
					x := 1
					a := 1
					{ y | y := [ z | z:=[1,2,3][a]; z > 1 ][_] }
				}
			`,
			expectedErrors: Errors{
				&Error{Message: "assigned var x unused"},
			},
		},
		{
			note: "rule with nested closure and unused inner var",
			module: `package test
				p {
					x := 1
					{ y | y := [ z | z:=[1,2,3][x]; z > 1; a := 2 ][_] }
				}
			`,
			expectedErrors: Errors{
				&Error{Message: "assigned var a unused"},
			},
		},
		{
			note: "simple function",
			module: `package test
				f() {
					x := 1
					y := 2
				}
			`,
			expectedErrors: Errors{
				&Error{Message: "assigned var x unused"},
				&Error{Message: "assigned var y unused"},
			},
		},
		{
			note: "simple function with wildcard",
			module: `package test
				f() {
					x := 1
					_ := 2
				}
			`,
			expectedErrors: Errors{
				&Error{Message: "assigned var x unused"},
			},
		},
		{
			note: "function with return",
			module: `package test
				f() = x {
					x := 1
					y := 2
				}
			`,
			expectedErrors: Errors{
				&Error{Message: "assigned var y unused"},
			},
		},
		{
			note: "array comprehension",
			module: `package test
				comp = [ 1 |
					x := [1, 2, 3]
					y := 2
					z := x[_]
				]
			`,
			expectedErrors: Errors{
				&Error{Message: "assigned var y unused"},
				&Error{Message: "assigned var z unused"},
			},
		},
		{
			note: "array comprehension nested",
			module: `package test
				comp := [ 1 |
					x := 1
					y := [a | a := x]
				]
			`,
			expectedErrors: Errors{
				&Error{Message: "assigned var y unused"},
			},
		},
		{
			note: "array comprehension with wildcard",
			module: `package test
				comp = [ 1 |
					x := [1, 2, 3]
					_ := 2
					z := x[_]
				]
			`,
			expectedErrors: Errors{
				&Error{Message: "assigned var z unused"},
			},
		},
		{
			note: "array comprehension with return",
			module: `package test
				comp = [ z |
					x := [1, 2, 3]
					y := 2
					z := x[_]
				]
			`,
			expectedErrors: Errors{
				&Error{Message: "assigned var y unused"},
			},
		},
		{
			note: "array comprehension with some",
			module: `package test
				comp = [ i |
					some i
					y := 2
				]
			`,
			expectedErrors: Errors{
				&Error{Message: "assigned var y unused"},
			},
		},
		{
			note: "set comprehension",
			module: `package test
				comp = { 1 |
					x := [1, 2, 3]
					y := 2
					z := x[_]
				}
			`,
			expectedErrors: Errors{
				&Error{Message: "assigned var y unused"},
				&Error{Message: "assigned var z unused"},
			},
		},
		{
			note: "set comprehension nested",
			module: `package test
				comp := { 1 |
					x := 1
					y := [a | a := x]
				}
			`,
			expectedErrors: Errors{
				&Error{Message: "assigned var y unused"},
			},
		},
		{
			note: "set comprehension with wildcard",
			module: `package test
				comp = { 1 |
					x := [1, 2, 3]
					_ := 2
					z := x[_]
				}
			`,
			expectedErrors: Errors{
				&Error{Message: "assigned var z unused"},
			},
		},
		{
			note: "set comprehension with return",
			module: `package test
				comp = { z |
					x := [1, 2, 3]
					y := 2
					z := x[_]
				}
			`,
			expectedErrors: Errors{
				&Error{Message: "assigned var y unused"},
			},
		},
		{
			note: "set comprehension with some",
			module: `package test
				comp = { i |
					some i
					y := 2
				}
			`,
			expectedErrors: Errors{
				&Error{Message: "assigned var y unused"},
			},
		},
		{
			note: "object comprehension",
			module: `package test
				comp = { 1: 2 |
					x := [1, 2, 3]
					y := 2
					z := x[_]
				}
			`,
			expectedErrors: Errors{
				&Error{Message: "assigned var y unused"},
				&Error{Message: "assigned var z unused"},
			},
		},
		{
			note: "object comprehension nested",
			module: `package test
				comp := { 1: 1 |
					x := 1
					y := {a: x | a := x}
				}
			`,
			expectedErrors: Errors{
				&Error{Message: "assigned var y unused"},
			},
		},
		{
			note: "object comprehension with wildcard",
			module: `package test
				comp = { 1: 2 |
					x := [1, 2, 3]
					_ := 2
					z := x[_]
				}
			`,
			expectedErrors: Errors{
				&Error{Message: "assigned var z unused"},
			},
		},
		{
			note: "object comprehension with return",
			module: `package test
				comp = { z: x |
					x := [1, 2, 3]
					y := 2
					z := x[_]
				}
			`,
			expectedErrors: Errors{
				&Error{Message: "assigned var y unused"},
			},
		},
		{
			note: "object comprehension with some",
			module: `package test
				comp = { i |
					some i
					y := 2
				}
			`,
			expectedErrors: Errors{
				&Error{Message: "assigned var y unused"},
			},
		},
		{
			note: "every: unused assigned var in body",
			module: `package test
				p { every i in [1] { y := 10; i == 1 } }
			`,
			expectedErrors: Errors{
				&Error{Message: "assigned var y unused"},
			},
		},
		{
			note: "general ref in rule head",
			module: `package test
						p[q].r[s] := 1 {
							q := "foo"
							s := "bar"
							t := "baz"
						}
		`,
			expectedErrors: Errors{
				&Error{Message: "assigned var t unused"},
			},
		},
		{
			note: "general ref in rule head (no errors)",
			module: `package test
						p[q].r[s] := 1 {
							q := "foo"
							s := "bar"
						}
		`,
			expectedErrors: Errors{},
		},
	}

	makeTestRunner := func(tc testCase, strict bool) func(t *testing.T) {
		return func(t *testing.T) {
			compiler := NewCompiler().WithStrict(strict)
			opts := ParserOptions{AllFutureKeywords: true, unreleasedKeywords: true}
			compiler.Modules = map[string]*Module{
				"test": MustParseModuleWithOpts(tc.module, opts),
			}
			compileStages(compiler, compiler.rewriteLocalVars)

			if strict {
				assertErrors(t, compiler.Errors, tc.expectedErrors, false)
			} else {
				assertNotFailed(t, compiler)
			}
		}
	}

	for _, tc := range cases {
		t.Run(tc.note+"_strict", makeTestRunner(tc, true))
		t.Run(tc.note+"_non-strict", makeTestRunner(tc, false))
	}
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
		"everyMod": MustParseModule(`package everymod
		import future.keywords.every
		everyp {
			every x in [true, false] { x; everyp }
		}
		everyq[1] {
			every x in everyq { x == 1 }
		}`),
	}

	compileStages(c, c.checkRecursion)

	makeRuleErrMsg := func(pkg, rule string, loop ...string) string {
		l := make([]string, len(loop))
		for i, lo := range loop {
			l[i] = "data." + pkg + "." + lo
		}
		return fmt.Sprintf("rego_recursion_error: rule data.%s.%s is recursive: %v", pkg, rule, strings.Join(l, " -> "))
	}

	expected := []string{
		makeRuleErrMsg("rec", "s", "s", "t", "s"),
		makeRuleErrMsg("rec", "t", "t", "s", "t"),
		makeRuleErrMsg("rec", "a", "a", "b", "c", "e", "a"),
		makeRuleErrMsg("rec", "b", "b", "c", "e", "a", "b"),
		makeRuleErrMsg("rec", "c", "c", "e", "a", "b", "c"),
		makeRuleErrMsg("rec", "e", "e", "a", "b", "c", "e"),
		`rego_recursion_error: rule data.rec3.p[x] is recursive: data.rec3.p[x] -> data.rec4.q[x] -> data.rec3.p[x]`, // NOTE(sr): these two are hardcoded: they are
		`rego_recursion_error: rule data.rec4.q[x] is recursive: data.rec4.q[x] -> data.rec3.p[x] -> data.rec4.q[x]`, // the only ones not fitting the pattern.
		makeRuleErrMsg("rec5", "acq", "acq", "acp", "acq"),
		makeRuleErrMsg("rec5", "acp", "acp", "acq", "acp"),
		makeRuleErrMsg("rec6", "np[x]", "np[x]", "nq[x]", "np[x]"),
		makeRuleErrMsg("rec6", "nq[x]", "nq[x]", "np[x]", "nq[x]"),
		makeRuleErrMsg("rec7", "prefix", "prefix", "prefix"),
		makeRuleErrMsg("rec8", "dataref", "dataref", "dataref"),
		makeRuleErrMsg("rec9", "else_self", "else_self", "else_self"),
		makeRuleErrMsg("rec9", "elsetop", "elsetop", "elsemid", "elsebottom", "elsetop"),
		makeRuleErrMsg("rec9", "elsemid", "elsemid", "elsebottom", "elsetop", "elsemid"),
		makeRuleErrMsg("rec9", "elsebottom", "elsebottom", "elsetop", "elsemid", "elsebottom"),
		makeRuleErrMsg("f0", "fn", "fn", "fn"),
		makeRuleErrMsg("f1", "foo", "foo", "bar", "foo"),
		makeRuleErrMsg("f1", "bar", "bar", "foo", "bar"),
		makeRuleErrMsg("f2", "bar", "bar", "p[x]", "foo", "bar"),
		makeRuleErrMsg("f2", "foo", "foo", "bar", "p[x]", "foo"),
		makeRuleErrMsg("f2", "p[x]", "p[x]", "foo", "bar", "p[x]"),
		makeRuleErrMsg("everymod", "everyp", "everyp", "everyp"),
		makeRuleErrMsg("everymod", "everyq", "everyq", "everyq"),
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

	for _, tc := range []struct {
		note, err string
		mod       *Module
	}{
		{
			note: "recursion",
			mod: MustParseModule(`
package recursion
pkg = "recursion"
foo[x] {
	data[pkg]["foo"][x]
}
`),
			err: "rego_recursion_error: rule data.recursion.foo is recursive: data.recursion.foo -> data.recursion.foo",
		},
		{note: "system.main",
			mod: MustParseModule(`
package system.main
foo {
	data[input]
}
`),
			err: "rego_recursion_error: rule data.system.main.foo is recursive: data.system.main.foo -> data.system.main.foo",
		},
	} {
		t.Run(tc.note, func(t *testing.T) {
			c := NewCompiler()
			c.Modules = map[string]*Module{tc.note: tc.mod}
			compileStages(c, c.checkRecursion)

			result := compilerErrsToStringSlice(c.Errors)
			expected := tc.err

			if len(result) != 1 || result[0] != expected {
				t.Errorf("Expected %v but got: %v", expected, result)
			}
		})
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
default r2 = false
r2 = 2`,
		"mod3": `package a.b
r3 = 3`,
		"hidden": `package system.hidden
r4 = 4`,
		"mod4": `package b.c
r5[x] = 5 { x := "foo" }
r5.bar = 6 { input.x }
r5.baz = 7 { input.y }
`,
	})

	compileStages(compiler, nil)

	rule1 := compiler.Modules["mod1"].Rules[0]
	rule2d := compiler.Modules["mod2"].Rules[0]
	rule2 := compiler.Modules["mod2"].Rules[1]
	rule3 := compiler.Modules["mod3"].Rules[0]
	rule4 := compiler.Modules["hidden"].Rules[0]
	rule5 := compiler.Modules["mod4"].Rules[0]
	rule5b := compiler.Modules["mod4"].Rules[1]
	rule5c := compiler.Modules["mod4"].Rules[2]

	tests := []struct {
		input         string
		expected      []*Rule
		excludeHidden bool
	}{
		{input: "data.a.b.c.d.r1", expected: []*Rule{rule1}},
		{input: "data.a.b[x]", expected: []*Rule{rule1, rule2d, rule2, rule3}},
		{input: "data.a.b[x].d", expected: []*Rule{rule1, rule3}},
		{input: "data.a.b.c", expected: []*Rule{rule1, rule2d, rule2}},
		{input: "data.a.b.d"},
		{input: "data", expected: []*Rule{rule1, rule2d, rule2, rule3, rule4, rule5, rule5b, rule5c}},
		{input: "data[x]", expected: []*Rule{rule1, rule2d, rule2, rule3, rule4, rule5, rule5b, rule5c}},
		{input: "data[data.complex_computation].b[y]", expected: []*Rule{rule1, rule2d, rule2, rule3}},
		{input: "data[x][y].c.e", expected: []*Rule{rule2d, rule2}},
		{input: "data[x][y].r3", expected: []*Rule{rule3}},
		{input: "data[x][y]", expected: []*Rule{rule1, rule2d, rule2, rule3, rule5, rule5b, rule5c}, excludeHidden: true}, // old behaviour of GetRulesDynamic
		{input: "data.b.c", expected: []*Rule{rule5, rule5b, rule5c}},
		{input: "data.b.c.r5", expected: []*Rule{rule5, rule5b, rule5c}},
		{input: "data.b.c.r5.bar", expected: []*Rule{rule5, rule5b}}, // rule5 might still define a value for the "bar" key
		{input: "data.b.c.r5.baz", expected: []*Rule{rule5, rule5c}},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := compiler.GetRulesDynamicWithOpts(
				MustParseRef(tc.input),
				RulesOptions{IncludeHiddenModules: !tc.excludeHidden},
			)

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
		CompilerStageDefinition{"MockStage", "mock_stage", func(*Compiler) *Error { return nil }},
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

func TestCompilerAllowMultipleAssignments(t *testing.T) {

	_, err := CompileModules(map[string]string{"test.rego": `
		package test

		p := 7
		p := 8
	`})
	if err != nil {
		t.Fatal(err)
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
			expected: fmt.Errorf("1 error occurred: 1:1: rego_type_error: eq: arity mismatch\n\thave: ()\n\twant: (any, any)"),
		},
		{
			note:     "invalid eq",
			q:        "eq(1)",
			expected: fmt.Errorf("1 error occurred: 1:1: rego_type_error: eq: arity mismatch\n\thave: (number)\n\twant: (any, any)"),
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
			expected: fmt.Errorf("1 error occurred: 1:12: rego_type_error: with keyword target must reference existing input, data, or a function"),
		},
		{
			note:     "rewrite with value",
			q:        `1 with input as [z]`,
			pkg:      "package a.b.c",
			imports:  nil,
			expected: `__localq1__ = data.a.b.c.z; __localq0__ = [__localq1__]; 1 with input as __localq0__`,
		},
		{
			note:     "built-in function arity mismatch",
			q:        `startswith("x")`,
			pkg:      "",
			imports:  nil,
			expected: fmt.Errorf("1 error occurred: 1:1: rego_type_error: startswith: arity mismatch\n\thave: (string)\n\twant: (search: string, base: string)"),
		},
		{
			note:     "built-in function arity mismatch (arity 0)",
			q:        `x := opa.runtime("foo")`,
			pkg:      "",
			imports:  nil,
			expected: fmt.Errorf("1 error occurred: 1:6: rego_type_error: opa.runtime: arity mismatch\n\thave: (string, ???)\n\twant: ()"),
		},
		{
			note:     "built-in function arity mismatch, nested",
			q:        "count(sum())",
			pkg:      "",
			imports:  nil,
			expected: fmt.Errorf("1 error occurred: 1:7: rego_type_error: sum: arity mismatch\n\thave: (???)\n\twant: (collection: any<array[number], set[number]>)"),
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
		t.Run(tc.note, runQueryCompilerTest(tc.q, tc.pkg, tc.imports, tc.expected))
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
			func(_ QueryCompiler, b Body) (Body, error) {
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
	tests := []struct {
		note     string
		query    string
		compiler *Compiler
		opts     func(QueryCompiler) QueryCompiler
		err      string
	}{
		{
			note:     "builtin unsafe via compiler",
			query:    "count([])",
			compiler: NewCompiler().WithUnsafeBuiltins(map[string]struct{}{"count": {}}),
			err:      "unsafe built-in function calls in expression: count",
		},
		{
			note:     "builtin unsafe via query compiler",
			query:    "count([])",
			compiler: NewCompiler(),
			opts: func(qc QueryCompiler) QueryCompiler {
				return qc.WithUnsafeBuiltins(map[string]struct{}{"count": {}})
			},
			err: "unsafe built-in function calls in expression: count",
		},
		{
			note:     "builtin unsafe via compiler, 'with' mocking",
			query:    "is_array([]) with is_array as count",
			compiler: NewCompiler().WithUnsafeBuiltins(map[string]struct{}{"count": {}}),
			err:      `with keyword replacing built-in function: target must not be unsafe: "count"`,
		},
		{
			note:     "builtin unsafe via query compiler,  'with' mocking",
			query:    "is_array([]) with is_array as count",
			compiler: NewCompiler(),
			opts: func(qc QueryCompiler) QueryCompiler {
				return qc.WithUnsafeBuiltins(map[string]struct{}{"count": {}})
			},
			err: `with keyword replacing built-in function: target must not be unsafe: "count"`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			qc := tc.compiler.QueryCompiler()
			if tc.opts != nil {
				qc = tc.opts(qc)
			}
			_, err := qc.Compile(MustParseBody(tc.query))
			var errs Errors
			if !errors.As(err, &errs) {
				t.Fatalf("expected error type %T, got %v %[2]T", errs, err)
			}
			if exp, act := 1, len(errs); exp != act {
				t.Fatalf("expected %d error(s), got %d", exp, act)
			}
			if exp, act := tc.err, errs[0].Message; exp != act {
				t.Errorf("expected message %q, got %q", exp, act)
			}
		})
	}
}

func TestQueryCompilerWithDeprecatedBuiltins(t *testing.T) {
	cases := []strictnessQueryTestCase{
		{
			note:           "all() built-in",
			query:          "all([true, false])",
			expectedErrors: fmt.Errorf("1 error occurred: 1:1: rego_type_error: deprecated built-in function calls in expression: all"),
		},
		{
			note:           "any() built-in",
			query:          "any([true, false])",
			expectedErrors: fmt.Errorf("1 error occurred: 1:1: rego_type_error: deprecated built-in function calls in expression: any"),
		},
	}

	runStrictnessQueryTestCase(t, cases)
}

func TestQueryCompilerWithUnusedAssignedVar(t *testing.T) {
	cases := []strictnessQueryTestCase{
		{
			note:           "array comprehension",
			query:          "[1 | x := 2]",
			expectedErrors: fmt.Errorf("1 error occurred: 1:6: rego_compile_error: assigned var x unused"),
		},
		{
			note:           "set comprehension",
			query:          "{1 | x := 2}",
			expectedErrors: fmt.Errorf("1 error occurred: 1:6: rego_compile_error: assigned var x unused"),
		},
		{
			note:           "object comprehension",
			query:          "{1: 2 | x := 2}",
			expectedErrors: fmt.Errorf("1 error occurred: 1:9: rego_compile_error: assigned var x unused"),
		},
		{
			note:           "every: unused var in body",
			query:          "every _ in [] { x := 10 }",
			expectedErrors: fmt.Errorf("1 error occurred: 1:17: rego_compile_error: assigned var x unused"),
		},
	}

	runStrictnessQueryTestCase(t, cases)
}

func TestQueryCompilerCheckKeywordOverrides(t *testing.T) {
	cases := []strictnessQueryTestCase{
		{
			note:           "input assigned",
			query:          "input := 1",
			expectedErrors: fmt.Errorf("1 error occurred: 1:1: rego_compile_error: variables must not shadow input (use a different variable name)"),
		},
		{
			note:           "data assigned",
			query:          "data := 1",
			expectedErrors: fmt.Errorf("1 error occurred: 1:1: rego_compile_error: variables must not shadow data (use a different variable name)"),
		},
		{
			note:           "nested input assigned",
			query:          "d := [input | input := 1]",
			expectedErrors: fmt.Errorf("1 error occurred: 1:15: rego_compile_error: variables must not shadow input (use a different variable name)"),
		},
	}

	runStrictnessQueryTestCase(t, cases)
}

type strictnessQueryTestCase struct {
	note           string
	query          string
	expectedErrors error
}

func runStrictnessQueryTestCase(t *testing.T, cases []strictnessQueryTestCase) {
	t.Helper()
	makeTestRunner := func(tc strictnessQueryTestCase, strict bool) func(t *testing.T) {
		return func(t *testing.T) {
			c := NewCompiler().WithStrict(strict)
			opts := ParserOptions{AllFutureKeywords: true, unreleasedKeywords: true}
			result, err := c.QueryCompiler().Compile(MustParseBodyWithOpts(tc.query, opts))

			if strict {
				if err == nil {
					t.Fatalf("Expected error from %v but got: %v", tc.query, result)
				}
				if !strings.Contains(err.Error(), tc.expectedErrors.Error()) {
					t.Fatalf("Expected error %v but got: %v", tc.expectedErrors, err)
				}
			} else {
				if err != nil {
					t.Fatalf("Unexpected error from %v: %v", tc.query, err)
				}
			}
		}
	}

	for _, tc := range cases {
		t.Run(tc.note+"_strict", makeTestRunner(tc, true))
		t.Run(tc.note+"_non-strict", makeTestRunner(tc, false))
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
	t.Helper()
	if c.Failed() {
		t.Fatalf("Unexpected compilation error: %v", c.Errors)
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

func runQueryCompilerTest(q, pkg string, imports []string, expected interface{}) func(*testing.T) {
	return func(t *testing.T) {
		t.Helper()
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
	}
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

func TestCompilerPassesTypeCheckRules(t *testing.T) {
	inputSchema := `{
  "$schema": "http://json-schema.org/draft-04/schema#",
  "description": "OPA Authorization Policy Schema",
  "type": "object",
  "properties": {
    "identity": {
      "type": "string"
    },
    "path": {
      "type": "array",
      "items": {}
    },
    "params": {
      "type": "object"
    }
  },
  "required": [
    "identity",
    "path",
    "params"
  ]
}`

	ischema := util.MustUnmarshalJSON([]byte(inputSchema))

	module1 := `
package policy

default allow := false

allow {
    input.identity = "foo"
}

allow {
    input.path = ["foo", "bar"]
}

allow {
    input.params = {"foo": "bar"}
}`

	module2 := `
package policy

default allow := false

allow {
    input.identty = "foo"
}`

	module3 := `
package policy

default allow := false

allow {
    input.path = "foo"
}`

	module4 := `
package policy

default allow := false

allow {
    input.identty = "foo"
}

allow {
    input.path = "foo"
}`

	schemaSet := NewSchemaSet()
	schemaSet.Put(SchemaRootRef, ischema)

	tests := []struct {
		note    string
		modules []string
		errs    []string
	}{
		{note: "no error", modules: []string{module1}},
		{note: "typo", modules: []string{module2}, errs: []string{"undefined ref: input.identty"}},
		{note: "wrong type", modules: []string{module3}, errs: []string{"match error"}},
		{note: "multiple errors", modules: []string{module4}, errs: []string{"match error", "undefined ref: input.identty"}},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			var elems []*Rule

			for i, module := range tc.modules {
				mod, err := ParseModuleWithOpts(fmt.Sprintf("test%d.rego", i+1), module, ParserOptions{
					ProcessAnnotation: true,
				})
				if err != nil {
					t.Fatal(err)
				}

				for _, rule := range mod.Rules {
					elems = append(elems, rule)
					for next := rule.Else; next != nil; next = next.Else {
						elems = append(elems, next)
					}
				}
			}

			errors := NewCompiler().WithSchemas(schemaSet).PassesTypeCheckRules(elems)

			if len(errors) > 0 {
				if len(tc.errs) == 0 {
					t.Fatalf("Unexpected error: %v", errors)
				}

				result := compilerErrsToStringSlice(errors)

				if len(result) != len(tc.errs) {
					t.Fatalf("Expected %d:\n%v\nBut got %d:\n%v", len(tc.errs), strings.Join(tc.errs, "\n"), len(result), strings.Join(result, "\n"))
				}

				for i := range result {
					if !strings.Contains(result[i], tc.errs[i]) {
						t.Errorf("Expected %v but got: %v", tc.errs[i], result[i])
					}
				}
			} else if len(tc.errs) > 0 {
				t.Fatalf("Expected error %q but got success", tc.errs)
			}
		})
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

func TestKeepModules(t *testing.T) {

	t.Run("no keep", func(t *testing.T) {
		c := NewCompiler() // no keep is default

		// This one is overwritten by c.Compile()
		c.Modules["foo.rego"] = MustParseModule("package foo\np = true")

		c.Compile(map[string]*Module{"bar.rego": MustParseModule("package bar\np = input")})

		if len(c.Errors) != 0 {
			t.Fatalf("expected no error; got %v", c.Errors)
		}

		mods := c.ParsedModules()
		if mods != nil {
			t.Errorf("expected ParsedModules == nil, got %v", mods)
		}
	})

	t.Run("keep", func(t *testing.T) {

		c := NewCompiler().WithKeepModules(true)

		// This one is overwritten by c.Compile()
		c.Modules["foo.rego"] = MustParseModule("package foo\np = true")

		c.Compile(map[string]*Module{"bar.rego": MustParseModule("package bar\np = input")})
		if len(c.Errors) != 0 {
			t.Fatalf("expected no error; got %v", c.Errors)
		}

		mods := c.ParsedModules()
		if exp, act := 1, len(mods); exp != act {
			t.Errorf("expected %d modules, found %d: %v", exp, act, mods)
		}
		for k := range mods {
			if k != "bar.rego" {
				t.Errorf("unexpected key: %v, want 'bar.rego'", k)
			}
		}

		for k := range mods {
			compiled := c.Modules[k]
			if compiled.Equal(mods[k]) {
				t.Errorf("expected module %v to not be compiled: %v", k, mods[k])
			}
		}

		// expect ParsedModules to be reset
		c.Compile(map[string]*Module{"baz.rego": MustParseModule("package baz\np = input")})
		mods = c.ParsedModules()
		if exp, act := 1, len(mods); exp != act {
			t.Errorf("expected %d modules, found %d: %v", exp, act, mods)
		}
		for k := range mods {
			if k != "baz.rego" {
				t.Errorf("unexpected key: %v, want 'baz.rego'", k)
			}
		}

		for k := range mods {
			compiled := c.Modules[k]
			if compiled.Equal(mods[k]) {
				t.Errorf("expected module %v to not be compiled: %v", k, mods[k])
			}
		}

		// expect ParsedModules to be reset to nil
		c = c.WithKeepModules(false)
		c.Compile(map[string]*Module{"baz.rego": MustParseModule("package baz\np = input")})
		mods = c.ParsedModules()
		if mods != nil {
			t.Errorf("expected ParsedModules == nil, got %v", mods)
		}
	})

	t.Run("no copies", func(t *testing.T) {
		extra := MustParseModule("package extra\np = input")
		done := false
		testLoader := func(map[string]*Module) (map[string]*Module, error) {
			if done {
				return nil, nil
			}
			done = true
			return map[string]*Module{"extra.rego": extra}, nil
		}

		c := NewCompiler().WithModuleLoader(testLoader).WithKeepModules(true)

		mod := MustParseModule("package bar\np = input")
		c.Compile(map[string]*Module{"bar.rego": mod})
		if len(c.Errors) != 0 {
			t.Fatalf("expected no error; got %v", c.Errors)
		}

		mods := c.ParsedModules()
		if exp, act := 2, len(mods); exp != act {
			t.Errorf("expected %d modules, found %d: %v", exp, act, mods)
		}
		newName := Var("q")
		mods["bar.rego"].Rules[0].Head.Name = newName
		if exp, act := newName, mod.Rules[0].Head.Name; !exp.Equal(act) {
			t.Errorf("expected modified rule name %v, found %v", exp, act)
		}
		mods["extra.rego"].Rules[0].Head.Name = newName
		if exp, act := newName, extra.Rules[0].Head.Name; !exp.Equal(act) {
			t.Errorf("expected modified rule name %v, found %v", exp, act)
		}
	})

	t.Run("keep, with loader", func(t *testing.T) {
		extra := MustParseModule("package extra\np = input")
		done := false
		testLoader := func(map[string]*Module) (map[string]*Module, error) {
			if done {
				return nil, nil
			}
			done = true
			return map[string]*Module{"extra.rego": extra}, nil
		}

		c := NewCompiler().WithModuleLoader(testLoader).WithKeepModules(true)

		// This one is overwritten by c.Compile()
		c.Modules["foo.rego"] = MustParseModule("package foo\np = true")

		c.Compile(map[string]*Module{"bar.rego": MustParseModule("package bar\np = input")})

		if len(c.Errors) != 0 {
			t.Fatalf("expected no error; got %v", c.Errors)
		}

		mods := c.ParsedModules()
		if exp, act := 2, len(mods); exp != act {
			t.Errorf("expected %d modules, found %d: %v", exp, act, mods)
		}
		for k := range mods {
			if k != "bar.rego" && k != "extra.rego" {
				t.Errorf("unexpected key: %v, want 'extra.rego' and 'bar.rego'", k)
			}
		}

		for k := range mods {
			compiled := c.Modules[k]
			if compiled.Equal(mods[k]) {
				t.Errorf("expected module %v to not be compiled: %v", k, mods[k])
			}
		}
	})
}

// see https://github.com/open-policy-agent/opa/issues/5166
func TestCompilerWithRecursiveSchema(t *testing.T) {

	jsonSchema := `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "https://github.com/open-policy-agent/opa/issues/5166",
  "type": "object",
  "properties": {
    "Something": {
      "$ref": "#/$defs/X"
    }
  },
  "$defs": {
    "X": {
      "type": "object",
      "properties": {
		"Name": { "type": "string" },
        "Y": {
          "$ref": "#/$defs/Y"
        }
      }
    },
    "Y": {
      "type": "object",
      "properties": {
        "X": {
          "$ref": "#/$defs/X"
        }
      }
    }
  }
}`

	exampleModule := `# METADATA
# schemas:
# - input: schema.input
package opa.recursion

deny {
	input.Something.Y.X.Name == "Something"
}
`

	c := NewCompiler()
	var schema interface{}
	if err := json.Unmarshal([]byte(jsonSchema), &schema); err != nil {
		t.Fatal(err)
	}
	schemaSet := NewSchemaSet()
	schemaSet.Put(MustParseRef("schema.input"), schema)
	c.WithSchemas(schemaSet)

	m := MustParseModuleWithOpts(exampleModule, ParserOptions{ProcessAnnotation: true})
	c.Compile(map[string]*Module{"testMod": m})
	if c.Failed() {
		t.Errorf("Expected compilation to succeed, but got errors: %v", c.Errors)
	}
}

// see https://github.com/open-policy-agent/opa/issues/5166
func TestCompilerWithRecursiveSchemaAndInvalidSource(t *testing.T) {

	jsonSchema := `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "https://github.com/open-policy-agent/opa/issues/5166",
  "type": "object",
  "properties": {
    "Something": {
      "$ref": "#/$defs/X"
    }
  },
  "$defs": {
    "X": {
      "type": "object",
      "properties": {
		"Name": { "type": "string" },
        "Y": {
          "$ref": "#/$defs/Y"
        }
      }
    },
    "Y": {
      "type": "object",
      "properties": {
        "X": {
          "$ref": "#/$defs/X"
        }
      }
    }
  }
}`

	exampleModule := `# METADATA
# schemas:
# - input: schema.input
package opa.recursion

deny {
	input.Something.Y.X.ThisDoesNotExist == "Something"
}
`

	c := NewCompiler().
		WithUseTypeCheckAnnotations(true)
	var schema interface{}
	if err := json.Unmarshal([]byte(jsonSchema), &schema); err != nil {
		t.Fatal(err)
	}
	schemaSet := NewSchemaSet()
	schemaSet.Put(MustParseRef("schema.input"), schema)
	c.WithSchemas(schemaSet)

	m := MustParseModuleWithOpts(exampleModule, ParserOptions{ProcessAnnotation: true})
	c.Compile(map[string]*Module{"testMod": m})
	if !c.Failed() {
		t.Errorf("Expected compilation to fail, but it succeeded")
	} else if !strings.HasPrefix(c.Errors.Error(), "1 error occurred: 7:2: rego_type_error: undefined ref: input.Something.Y.X.ThisDoesNotExist") {
		t.Errorf("unexpected error: %v", c.Errors.Error())
	}
}

func modules(ms ...string) []*Module {
	opts := ParserOptions{AllFutureKeywords: true, unreleasedKeywords: true}
	mods := make([]*Module, len(ms))
	for i, m := range ms {
		var err error
		mods[i], err = ParseModuleWithOpts(fmt.Sprintf("mod%d.rego", i), m, opts)
		if err != nil {
			panic(err)
		}
	}
	return mods
}

func TestCompilerWithRecursiveSchemaAvoidRace(t *testing.T) {

	jsonSchema := `{
  "type": "object",
  "properties": {
    "aws": {
      "type": "object",
      "$ref": "#/$defs/example.pkg.providers.aws.AWS"
    }
  },
  "$defs": {
    "example.pkg.providers.aws.AWS": {
      "type": "object",
      "properties": {
        "iam": {
          "type": "object",
          "$ref": "#/$defs/example.pkg.providers.aws.iam.IAM"
        },
        "sqs": {
          "type": "object",
          "$ref": "#/$defs/example.pkg.providers.aws.sqs.SQS"
        }
      }
    },
    "example.pkg.providers.aws.iam.Document": {
      "type": "object"
    },
    "example.pkg.providers.aws.iam.IAM": {
      "type": "object",
      "properties": {
        "policies": {
          "type": "array",
          "items": {
            "type": "object",
            "$ref": "#/$defs/example.pkg.providers.aws.iam.Policy"
          }
        }
      }
    },
    "example.pkg.providers.aws.iam.Policy": {
      "type": "object",
      "properties": {
        "builtin": {
          "type": "object",
          "properties": {
            "value": {
              "type": "boolean"
            }
          }
        },
        "document": {
          "type": "object",
          "$ref": "#/$defs/example.pkg.providers.aws.iam.Document"
        }
      }
    },
    "example.pkg.providers.aws.sqs.Queue": {
      "type": "object",
      "properties": {
        "policies": {
          "type": "array",
          "items": {
            "type": "object",
            "$ref": "#/$defs/example.pkg.providers.aws.iam.Policy"
          }
        }
      }
    },
    "example.pkg.providers.aws.sqs.SQS": {
      "type": "object",
      "properties": {
        "queues": {
          "type": "array",
          "items": {
            "type": "object",
            "$ref": "#/$defs/example.pkg.providers.aws.sqs.Queue"
          }
        }
      }
    }
  }
}`

	exampleModule := `# METADATA
# schemas:
#  - input: schema.input
package race.condition

deny {
	queue := input.aws.sqs.queues[_]
	policy := queue.policies[_]
	doc := json.unmarshal(policy.document.value)
	statement = doc.Statement[_]
	action := statement.Action[_]
	action == "*"
}
`

	c := NewCompiler()
	var schema interface{}
	if err := json.Unmarshal([]byte(jsonSchema), &schema); err != nil {
		t.Fatal(err)
	}
	schemaSet := NewSchemaSet()
	schemaSet.Put(MustParseRef("schema.input"), schema)
	c.WithSchemas(schemaSet)

	m := MustParseModuleWithOpts(exampleModule, ParserOptions{ProcessAnnotation: true})
	c.Compile(map[string]*Module{"testMod": m})
	if c.Failed() {
		t.Fatal(c.Errors)
	}
}
