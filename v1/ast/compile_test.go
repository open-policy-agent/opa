// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/v1/metrics"
	"github.com/open-policy-agent/opa/v1/types"
	"github.com/open-policy-agent/opa/v1/util"
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

			for v := range outputVarsForBody(body, arity, safe, nil) {
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
				q.r[s] = 1 if { s := "foo" }`),
			exports: map[string][]string{"data.p": {"q.r"}},
		},
		{
			note: "simple multi-value ref rule",
			modules: modules(`package p
				q.r.s contains 1 if { true }`),
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
				a.b.q[x] = y if { x := 1; y := true }
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

	hashMap := func(ms map[string][]string) *util.HasherMap[Ref, []Ref] {
		rules := util.NewHasherMap[Ref, []Ref](RefEqual)
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
				c.Modules[strconv.Itoa(i)] = m
				c.sorted = append(c.sorted, strconv.Itoa(i))
			}
			if exp, act := hashMap(tc.exports), c.getExports(); !refMapEqual(exp, act) {
				t.Errorf("expected %v, got %v", exp, act)
			}
		})
	}
}

func refMapEqual(a, b *util.HasherMap[Ref, []Ref]) bool {
	if a.Len() != b.Len() {
		return false
	}
	return !a.Iter(func(k Ref, v []Ref) bool {
		v2, ok := b.Get(k)
		if !ok {
			return true
		}
		if !refSliceEqual(v, v2) {
			return true
		}
		return false
	})
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
				p.q[i].r = 1 if { i := 10 }`,
			),
		},
		{
			note: "valid: ref is single-value rule with var key",
			modules: modules(
				`package x
				p.q.r[i] if { i := 10 }`,
			),
		},
		{
			note: "valid: ref is single-value rule with var key and value",
			modules: modules(
				`package x
				p.q.r[i] = j if { i := 10; j := 11 }`,
			),
		},
		{
			note: "valid: ref is single-value rule with var key and static value",
			modules: modules(
				`package x
				p.q.r[i] = "ten" if { i := 10 }`,
			),
		},
		{
			note: "valid: ref is single-value rule with number key",
			modules: modules(
				`package x
				p.q.r[1] if { true }`,
			),
		},
		{
			note: "valid: ref is single-value rule with boolean key",
			modules: modules(
				`package x
				p.q.r[true] if { true }`,
			),
		},
		{
			note: "valid: ref is single-value rule with null key",
			modules: modules(
				`package x
				p.q.r[null] if { true }`,
			),
		},
		{
			note: "valid: ref is single-value rule with set literal key",
			modules: modules(
				`package x
				p.q.r[set()] if { true }`,
			),
		},
		{
			note: "valid: ref is single-value rule with array literal key",
			modules: modules(
				`package x
				p.q.r[[]] if { true }`,
			),
		},
		{
			note: "valid: ref is single-value rule with object literal key",
			modules: modules(
				`package x
				p.q.r[{}] if { true }`,
			),
		},
		{
			note: "valid: ref is single-value rule with ref key",
			modules: modules(
				`package x
				x := [1,2,3]
				p.q.r[x[i]] if { i := 0}`,
			),
		},
		{
			note: "invalid: ref in ref",
			modules: modules(
				`package x
				p.q[arr[0]].r if { i := 10 }`,
			),
		},
		{
			note: "invalid: non-string in ref (not last position)",
			modules: modules(
				`package x
				p.q[10].r if { true }`,
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
		{
			note: "rewrite: single-value with non-var ref term",
			modules: modules(
				`package x
				p.q[y.z].r if y := {"z": "a"}`,
			),
			expected: MustParseRule(`p.q[__local0__].r  { y := {"z": "a"}; __local0__ = y.z }`),
		},
		{
			note: "rewrite: single-value with non-var ref term and key",
			modules: modules(
				`package x
				p.q[a.b][c.d] if y := {"z": "a"}`,
			),
			expected: MustParseRule(`p.q[__local0__][__local1__]  { y := {"z": "a"}; __local0__ = a.b; __local1__ = c.d }`),
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			mods := make(map[string]*Module, len(tc.modules))
			for i, m := range tc.modules {
				mods[strconv.Itoa(i)] = m
			}
			c := NewCompiler()
			c.Modules = mods
			compileStages(c, StageRewriteRuleHeadRefs)
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
				p.q.r = 1 if { input == 1 }`,
				`package x.p.q
				r = 2 if { input == 2 }`,
			),
			size: 2,
		},
		{
			note: "conflict: two modules, both using ref head, different package paths",
			modules: modules(
				`package x
				p.q.r = 1 if { input == 1 }`, // x.p.q.r = 1
				`package x.p
				q.r.s = 2 if { input == 2 }`, // x.p.q.r.s = 2
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
				p.q.x[y] = y if { y := "y" }`,
			),
			size:  4,
			depth: 6,
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			c := NewCompiler()
			for i, m := range tc.modules {
				c.Modules[strconv.Itoa(i)] = m
				c.sorted = append(c.sorted, strconv.Itoa(i))
			}
			compileStages(c, StageSetRuleTree)
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

func TestRuleIndices(t *testing.T) {
	tests := []struct {
		note    string
		modules []*Module
		exp     map[string][]Ref
	}{
		{
			note: "regression test for #6930 (no if)",
			modules: modules(
				`package test

				p.q contains "foo"

				p[q] := r if {
					q := "bar"
					r := "baz"
				}`,
			),
			exp: map[string][]Ref{
				"data.test.p": {
					MustParseRef("p.q"),
					MustParseRef("p[__local0__]"),
				},
			},
		},
		{
			note: "regression test for #6930 (if)",
			modules: modules(
				`package test

				p.q contains "foo"

				p[q] := r if {
					q := "bar"
					r := "baz"
				}`,
			),
			exp: map[string][]Ref{
				"data.test.p": {
					MustParseRef("p.q"),
					MustParseRef("p[__local0__]"),
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			c := NewCompiler()
			for i, m := range tc.modules {
				c.Modules[strconv.Itoa(i)] = m
				c.sorted = append(c.sorted, strconv.Itoa(i))
			}
			compileStages(c, StageBuildRuleIndices)

			for k, expIndex := range tc.exp {
				kref := MustParseRef(k)
				node := c.RuleTree.Find(kref)
				if node == nil || node.Index == nil {
					t.Fatalf("expected rule indices for %v", k)
				}
				index := node.Index.(*baseDocEqIndex)
				for _, expRef := range expIndex {
					found := false
					for _, r := range index.root.rules {
						if r.rule.Head.Ref().Equal(expRef) {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("expected rule %v in index for %v", expRef, k)
					}
				}
			}
		})
	}
}

func TestRuleTreeWithVars(t *testing.T) {
	opts := ParserOptions{
		RegoVersion:        RegoV0,
		AllFutureKeywords:  true,
		unreleasedKeywords: true,
	}

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
		if exp, act := MustParseRef("c.d.e"), node.Values[0].Head.Ref(); !exp.Equal(act) {
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
		if exp, act := MustParseRef("c.d.e"), node.Values[0].Head.Ref(); !exp.Equal(act) {
			t.Errorf("expected rule ref %v, found %v", exp, act)
		}
		if exp, act := MustParseRef("d.e"), node.Values[1].Head.Ref(); !exp.Equal(act) {
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
		if exp, act := MustParseRef("d.e.f"), node.Values[0].Head.Ref(); !exp.Equal(act) {
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
		if exp, act := (Ref{VarTerm("b")}), node.Values[0].Head.Ref(); !exp.Equal(act) {
			t.Errorf("expected rule ref %v, found %v", exp, act)
		}
		if act := node.Values[0].Head.Value; act != nil {
			t.Errorf("expected rule value nil, found %v", act)
		}
		if exp, act := VarTerm("c"), node.Values[0].Head.Key; !exp.Equal(act) {
			t.Errorf("expected rule key %v, found %v", exp, act)
		}
		if exp, act := (Ref{VarTerm("b")}), node.Values[1].Head.Ref(); !exp.Equal(act) {
			t.Errorf("expected rule ref %v, found %v", exp, act)
		}
		if act := node.Values[1].Head.Value; act != nil {
			t.Errorf("expected rule value nil, found %v", act)
		}
		if exp, act := VarTerm("d"), node.Values[1].Head.Key; !exp.Equal(act) {
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
		if exp, act := (Ref{VarTerm("b")}), node.Values[0].Head.Ref(); !exp.Equal(act) {
			t.Errorf("expected rule ref %v, found %v", exp, act)
		}
		if act := node.Values[0].Head.Value; act != nil {
			t.Errorf("expected rule value nil, found %v", act)
		}
		if exp, act := IntNumberTerm(1), node.Values[0].Head.Key; !exp.Equal(act) {
			t.Errorf("expected rule key %v, found %v", exp, act)
		}
		if exp, act := (Ref{VarTerm("b")}), node.Values[1].Head.Ref(); !exp.Equal(act) {
			t.Errorf("expected rule ref %v, found %v", exp, act)
		}
		if act := node.Values[1].Head.Value; act != nil {
			t.Errorf("expected rule value nil, found %v", act)
		}
		if exp, act := IntNumberTerm(2), node.Values[1].Head.Key; !exp.Equal(act) {
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
		if exp, act := MustParseRef("b[1]"), node.Values[0].Head.Ref(); !exp.Equal(act) {
			t.Errorf("expected rule ref %v, found %v", exp, act)
		}
		if exp, act := IntNumberTerm(1), node.Values[0].Head.Value; !exp.Equal(act) {
			t.Errorf("expected rule value %v, found %v", exp, act)
		}
		if exp, act := IntNumberTerm(1), node.Values[0].Head.Key; !exp.Equal(act) {
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
		if exp, act := MustParseRef("b[2]"), node.Values[0].Head.Ref(); !exp.Equal(act) {
			t.Errorf("expected rule ref %v, found %v", exp, act)
		}
		if exp, act := IntNumberTerm(2), node.Values[0].Head.Value; !exp.Equal(act) {
			t.Errorf("expected rule value %v, found %v", exp, act)
		}
		if exp, act := IntNumberTerm(2), node.Values[0].Head.Key; !exp.Equal(act) {
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
		"0.rego": MustParseModule(`package p
import rego.v1
r = 1 if { true }`),
		"1.rego": MustParseModule(`package p
import rego.v1
r = 2 if { true }`),
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
	import rego.v1

	s contains 1 if { true }
	s contains 2 if { true }`,
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
	m := MustParseModuleWithOpts(testModule, ParserOptions{AllFutureKeywords: true})
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
		m := MustParseModuleWithOpts(testModule, ParserOptions{AllFutureKeywords: true})
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
import rego.v1

q if {
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
				modules[name], err = ParseModuleWithOpts(name, module, ParserOptions{RegoVersion: RegoV0})
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
	import rego.v1

	r = y if { y = true; x = z }

	s[x] = y if {
		z = y + x
	}

	t contains x if { split(x, y, z) }
	`),
	}

	c := NewCompiler().SetErrorLimit(2)
	c.Compile(modules)

	errs := c.Errors
	exp := []string{
		"4:23: rego_unsafe_var_error: var x is unsafe",
		"4:23: rego_unsafe_var_error: var z is unsafe",
		"rego_compile_error: error limit reached",
	}

	result := make([]string, 0, len(errs))
	for _, err := range errs {
		result = append(result, err.Error())
	}

	sort.Strings(exp)
	sort.Strings(result)
	if !slices.Equal(exp, result) {
		t.Errorf("Expected errors %v, got %v", exp, result)
	}
}

func TestCompilerCheckSafetyHead(t *testing.T) {
	c := NewCompiler()
	c.Modules = getCompilerTestModules()
	popts := ParserOptions{AllFutureKeywords: true, unreleasedKeywords: true}
	c.Modules["newMod"] = MustParseModuleWithOpts(`package a.b

unboundKey[x1] = y if { q[y] = {"foo": [1, 2, [{"bar": y}]]} }
unboundVal[y] = x2 if { q[y] = {"foo": [1, 2, [{"bar": y}]]} }
unboundCompositeVal[y] = [{"foo": x3, "bar": y}] if { q[y] = {"foo": [1, 2, [{"bar": y}]]} }
unboundCompositeKey contains [{"x": x4}] if { q[y] }
unboundBuiltinOperator = eq if { 4 = 1 }
unboundElse if { false } else = else_var if { true }
c.d.e[x5] if true
f.g.h[y] = x6 if y := "y"
i.j.k contains x7 if true
`, popts)
	compileStages(c, StageCheckSafetyRuleHeads)

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
			opts := ParserOptions{
				RegoVersion:        RegoV0,
				AllFutureKeywords:  true,
				unreleasedKeywords: true,
			}
			c := NewCompiler()
			c.Modules = getCompilerTestModules()
			c.Modules["reordering"] = MustParseModuleWithOpts(fmt.Sprintf(
				`package test
				p { %s }`, tc.body), opts)

			compileStages(c, StageCheckSafetyRuleBodies)

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
import rego.v1
import data.b
import data.c
p = true if { v = [null | true]; xs = [x | a[i] = x; a = [y | y != 1; y = c[j]]]; xs[j] > 0; z = [true | data.a.b.d.t with input as i2; i2 = i]; b[i] = j }
`),
			exp: MustParseBody(`v = [null | true]; data.b[i] = j; xs = [x | a = [y | y = data.c[j]; y != 1]; a[i] = x]; xs[j] > 0; z = [true | i2 = i; data.a.b.d.t with input as i2]`),
		},
		{
			note: "comprehensions-2",
			mod: MustParseModule(`package compr
import rego.v1
import data.b
import data.c
q = true if { _ = [x | x = b[i]]; _ = b[j]; _ = [x | x = true; x != false]; true != false; _ = [x | data.foo[_] = x]; data.foo[_] = _ }
`),
			exp: MustParseBody(`_ = [x | x = data.b[i]]; _ = data.b[j]; _ = [x | x = true; x != false]; true != false; _ = [x | data.foo[_] = x]; data.foo[_] = _`),
		},

		{
			note: "comprehensions-3",
			mod: MustParseModule(`package compr
import rego.v1
import data.b
import data.c
fn(x) = y if {
	trim(x, ".", y)
}
r = true if { a = [x | split(y, ".", z); x = z[i]; fn("...foo.bar..", y)] }
`),
			exp: MustParseBody(`a = [x | data.compr.fn("...foo.bar..", y); split(y, ".", z); x = z[i]]`),
		},
		{
			note: "closure over function output",
			mod: MustParseModule(`package test
import rego.v1

p if {
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
			compileStages(c, StageCheckSafetyRuleBodies)
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
		{"ref-head", `p if { a.b.c = "foo" }`, `{a,}`},
		{"ref-head-2", `p if { {"foo": [{"bar": a.b.c}]} = {"foo": [{"bar": "baz"}]} }`, `{a,}`},
		{"negation", `p if { a = [1, 2, 3, 4]; not a[i] = x }`, `{i, x}`},
		{"negation-head", `p contains x if { a = [1, 2, 3, 4]; not a[i] = x }`, `{i,x}`},
		{"negation-multiple", `p if { a = [1, 2, 3, 4]; b = [1, 2, 3, 4]; not a[i] = x; not b[j] = x }`, `{i, x, j}`},
		{"negation-nested", `p if { a = [{"foo": ["bar", "baz"]}]; not a[0].foo = [a[0].foo[i], a[0].foo[j]] } `, `{i, j}`},
		{"builtin-input", `p if { count([1, 2, x], x) }`, `{x,}`},
		{"builtin-input-name", `p if { count(eq, 1) }`, `{eq,}`},
		{"builtin-multiple", `p if { x > 0; x <= 3; x != 2 }`, `{x,}`},
		{"unordered-object-keys", `p if { x = "a"; [{x: y, z: a}] = [{"a": 1, "b": 2}]}`, `{a,y,z}`},
		{"unordered-sets", `p if { x = "a"; [{x, y}] = [{1, 2}]}`, `{y,}`},
		{"array-compr", `p if { _ = [x | x = data.a[_]; y > 1] }`, `{y,}`},
		{"array-compr-nested", `p if { _ = [x | x = a[_]; a = [y | y = data.a[_]; z > 1]] }`, `{z,}`},
		{"array-compr-closure", `p if { _ = [v | v = [x | x = data.a[_]]; x > 1] }`, `{x,}`},
		{"array-compr-term", `p if { _ = [u | true] }`, `{u,}`},
		{"array-compr-term-nested", `p if { _ = [v | v = [w | w != 0]] }`, `{w,}`},
		{"array-compr-mixed", `p if { _ = [x | y = [a | a = z[i]]] }`, `{a, x, z, i}`},
		{"array-compr-builtin", `p if { [true | eq != 2] }`, `{eq,}`},
		{"closure-self", `p if { x = [x | x = 1] }`, `{x,}`},
		{"closure-transitive", `p if { x = y; x = [y | y = 1] }`, `{x,y}`},
		{"nested", `p if { count(baz[i].attr[bar[dead.beef]], n) }`, `{dead,}`},
		{"negated-import", `p if { not foo; not bar; not baz }`, `set()`},
		{"rewritten", `p contains {"foo": dead[i]} if { true }`, `{dead, i}`},
		{"with-value", `p if { data.a.b.d.t with input as x }`, `{x,}`},
		{"with-value-2", `p if { x = data.a.b.d.t with input as x }`, `{x,}`},
		{"else-kw", "p if { false } else if { count(x, 1) }", `{x,}`},
		{"function", "foo(x) = [y, z] if { split(x, y, z) }", `{y,z}`},
		{"call-vars-input", "p if { f(x, x) } f(x) = x if { true }", `{x,}`},
		{"call-no-output", "p if { f(x) } f(x) = x if { true }", `{x,}`},
		{"call-too-few", "p if { f(1,x) } f(x,y) if { true }", "{x,}"},
		{"object-key-comprehension", "p if { { {p|x}: 0 } }", "{x,}"},
		{"set-value-comprehension", "p if { {1, {p|x}} }", "{x,}"},
		{"every", "p if { every y in [10] { x > y } }", "{x,}"},
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
			popts := ParserOptions{
				AllFutureKeywords:  true,
				unreleasedKeywords: true,
			}
			c := NewCompiler()
			c.Modules = map[string]*Module{
				"newMod": MustParseModuleWithOpts(fmt.Sprintf(`

				%v

				%v

				`, moduleBegin, tc.moduleContent), popts),
			}

			compileStages(c, StageCheckSafetyRuleBodies)

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
import rego.v1

p if {
	not x
	x > y
}`})

	if err == nil {
		t.Fatal("expected error")
	}

	errs := err.(Errors)

	if !strings.Contains(errs[0].Message, "var x is unsafe") || errs[0].Location.Row != 5 {
		t.Fatal("expected error on row 5 but got:", err)
	}

	if !strings.Contains(errs[1].Message, "var y is unsafe") || errs[1].Location.Row != 6 {
		t.Fatal("expected y is unsafe on row 6 but got:", err)
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
	compileStages(c, StageCheckTypes)
	assertNotFailed(t, c)
}

// Regression test for GH issue #6790
func TestCompilerCheckEveryWithNestedDomainCalls(t *testing.T) {
	c := NewCompiler()
	c.Modules = map[string]*Module{"test": MustParseModule(`package test
import rego.v1

x if {
	every p in [1 / 2] {
		p == true
	}
}`)}
	compileStages(c, StageCheckTypes)
	assertNotFailed(t, c)
}

func TestCompilerCheckRuleConflicts(t *testing.T) {

	c := getCompilerWithParsedModules(map[string]string{
		"mod1.rego": `package badrules

p contains x if { x = 1 }
p[x] = y if { x = y; x = "a" }
q contains 1 if { true }
q = {1, 2, 3} if { true }
r[x] = y if { x = y; x = "a" }
r[x] = y if { x = y; x = "a" }
s contains x if { x = "a" }
s contains x if { x = "b" }
t := x if { x = "a"}`,

		// valid extension of r in mod1.rego
		"mod2a.rego": `package badrules.r

q contains 1 if { true }`,

		// invalid override of s in mod1.rego
		"mod2b.rego": `package badrules.s

q contains 1 if { true }`,

		// invalid override of t in mod1.rego
		"mod2c.rego": `package badrules.t

q contains 1 if { true }`,

		"mod3.rego": `package badrules.defkw

default foo = 1
default foo = 2
foo = 3 if { true }

default p.q.bar = 1
default p.q.bar = 2
p.q.bar = 3 if { true }
`,
		"mod4.rego": `package badrules.arity

f(1) if { true }
f if { true }

g(1) if { true }
g(1,2) if { true }

p.q.h(1) if { true }
p.q.h if { true }

p.q.i(1) if { true }
p.q.i(1,2) if { true }`,
		"mod5.rego": `package badrules.dataoverlap

p if { true }`,
		"mod6.rego": `package badrules.existserr

p if { true }`,

		"mod7.rego": `package badrules.foo

bar.baz contains "quz" if true`,
		"mod8.rego": `package badrules.complete_partial
p := 1
p[r] := 2 if { r := "foo" }`,
	})

	c.WithPathConflictsCheck(func(path []string) (bool, error) {
		if slices.Equal(path, []string{"badrules", "dataoverlap", "p"}) {
			return true, nil
		} else if slices.Equal(path, []string{"badrules", "existserr", "p"}) {
			return false, errors.New("unexpected error")
		}
		return false, nil
	})

	compileStages(c, StageCheckRuleConflicts)

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
		"rego_type_error: multiple default rules data.badrules.defkw.foo found at mod3.rego:3, mod3.rego:4",
		"rego_type_error: multiple default rules data.badrules.defkw.p.q.bar found at mod3.rego:7, mod3.rego:8",
		"rego_type_error: package badrules.s conflicts with rule s defined at mod1.rego:10",
		"rego_type_error: package badrules.s conflicts with rule s defined at mod1.rego:9",
		"rego_type_error: package badrules.t conflicts with rule t defined at mod1.rego:11",
		"rego_type_error: rule data.badrules.s conflicts with [data.badrules.s.q]",
		"rego_type_error: rule data.badrules.t conflicts with [data.badrules.t.q]",
	}

	assertCompilerErrorStrings(t, c, expected)
}

func TestCompilerCheckRuleConflictsWithRoots(t *testing.T) {

	c := getCompilerWithParsedModules(map[string]string{
		"mod1.rego": `package badrules.dataoverlap
p if { true }`,
		"mod2.rego": `package badrules.existserr
p if { true }`,

		// this does not trigger conflict check because
		// WithPathConflictsCheckRoots limits the root to "badrules".
		"mod3.rego": `package badrules_outside_root.dataoverlap
p if { true }`,
	})

	c.WithPathConflictsCheck(func(path []string) (bool, error) {
		if slices.Contains(path, "dataoverlap") {
			return true, nil
		} else if slices.Equal(path, []string{"badrules", "existserr", "p"}) {
			return false, errors.New("unexpected error")
		}
		return false, nil
	}).WithPathConflictsCheckRoots([]string{"badrules"})

	compileStages(c, StageCheckRuleConflicts)

	expected := []string{
		"rego_compile_error: conflict check for data path badrules/existserr/p: unexpected error",
		"rego_compile_error: conflicting rule for data path badrules/dataoverlap/p found",
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
				f(x, y) = x if {
                   x == y
				}`),
			err: "rego_type_error: conflicting rules data.pkg.f found",
		},
	}
	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			mods := make(map[string]*Module, len(tc.modules))
			for i, m := range tc.modules {
				mods[strconv.Itoa(i)] = m
			}
			c := NewCompiler()
			c.Modules = mods
			compileStages(c, StageCheckRuleConflicts)
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
				p.q.r if { true }`,
				`package pkg.p.q
				r(_) = 2`),
			err: "rego_type_error: conflicting rules data.pkg.p.q.r found",
		},
		{
			note: "two default rules, ref and non-ref rule",
			modules: modules(
				`package pkg
				default p.q.r = 3
				p.q.r if { true }`,
				`package pkg.p.q
				default r = 4
				r = 2`),
			err: "rego_type_error: multiple default rules data.pkg.p.q.r found at mod0.rego:2, mod1.rego:2",
		},
		{
			note: "arity mismatch, ref and ref rule",
			modules: modules(
				`package pkg.a.b
				p.q.r if { true }`,
				`package pkg.a
				b.p.q.r(_) = 2`),
			err: "rego_type_error: conflicting rules data.pkg.a.b.p.q.r found",
		},
		{
			note: "two default rules, ref and ref rule",
			modules: modules(
				`package pkg
				default p.q.w.r = 3
				p.q.w.r if { true }`,
				`package pkg.p
				default q.w.r = 4
				q.w.r = 2`),
			err: "rego_type_error: multiple default rules data.pkg.p.q.w.r found at mod0.rego:2, mod1.rego:2",
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
				r if { true }`,
				`package pkg.q.r`),
			err: "rego_type_error: package pkg.q.r conflicts with rule r defined at mod0.rego:2",
		},
		{
			note: "module conflict: ref rule",
			modules: modules(
				`package pkg
				p.q.r if { true }`,
				`package pkg.p.q.r`),
			err: "rego_type_error: package pkg.p.q.r conflicts with rule p.q.r defined at mod0.rego:2",
		},
		{
			note: "single-value with other rule overlap",
			modules: modules(
				`package pkg
				p.q.r if { true }`,
				`package pkg
				p.q.r.s if { true }`),
			err: "rego_type_error: rule data.pkg.p.q.r conflicts with [data.pkg.p.q.r.s]",
		},
		{
			note: "single-value with other rule overlap",
			modules: modules(
				`package pkg
				p.q.r if { true }
				p.q.r.s if { true }
				p.q.r.t if { true }`),
			err: "rego_type_error: rule data.pkg.p.q.r conflicts with [data.pkg.p.q.r.s data.pkg.p.q.r.t]",
		},
		{
			note: "single-value with other partial object (same ref) overlap",
			modules: modules(
				`package pkg
				p.q := 1
				p.q[r] := 2 if { r := "foo" }`),
			err: "rego_type_error: conflicting rules data.pkg.p.q[r] foun",
		},
		{
			note: "single-value with other rule overlap, unknown key",
			modules: modules(
				`package pkg
				p.q[r] = x if { r = input.key; x = input.foo }
				p.q.r.s = x if { true }
				`),
		},
		{
			note: "single-value with other rule overlap, unknown ref var and key",
			modules: modules(
				`package pkg
				p.q[r][s] = x if { r = input.key1; s = input.key2; x = input.foo }
				p.q.r.s.t = x if { true }
				`),
		},
		{
			note: "single-value partial object with other partial object rule overlap, unknown keys (regression test for #5855; invalidated by multi-var refs)",
			modules: modules(
				`package pkg
				p[r] := x if { r = input.key; x = input.bar }
				p.q[r] := x if { r = input.key; x = input.bar }
				`),
		},
		{
			note: "single-value partial object with other partial object (implicit 'true' value) rule overlap, unknown keys",
			modules: modules(
				`package pkg
				p[r] := x if { r = input.key; x = input.bar }
				p.q[r] if { r = input.key }
				`),
		},
		{
			note: "single-value partial object with multi-value rule (ref head) overlap, unknown key",
			modules: modules(
				`package pkg
				import future.keywords
				p[r] := x if { r = input.key; x = input.bar }
				p.q contains r if { r = input.key }
				`),
		},
		{
			note: "single-value partial object with multi-value rule overlap, unknown key",
			modules: modules(
				`package pkg
				p[r] := x if { r = input.key; x = input.bar }
				p contains q if { true }
				`),
			err: "rego_type_error: conflicting rules data.pkg.p found",
		},
		{
			note: "single-value rule with known and unknown key",
			modules: modules(
				`package pkg
				p.q[r] = x if { r = input.key; x = input.foo }
				p.q.s = "x" if { true }
				`),
		},
		{
			note: "multi-value rule with other rule overlap",
			modules: modules(
				`package pkg
				p contains v if { v := ["a", "b"][_] }
				p.q := 42
				`),
			err: "rego_type_error: rule data.pkg.p conflicts with [data.pkg.p.q]",
		},
		{
			note: "multi-value rule with other rule (ref) overlap",
			modules: modules(
				`package pkg
				p contains v if { v := ["a", "b"][_] }
				p.q.r if { true }
				`),
			err: "rego_type_error: rule data.pkg.p conflicts with [data.pkg.p.q.r]",
		},
		{
			note: "multi-value rule (dots in head) with other rule (ref) overlap",
			modules: modules(
				`package pkg
				import future.keywords
				p.q contains v if { v := ["a", "b"][_] }
				p.q.r if { true }
				`),
			err: "rule data.pkg.p.q conflicts with [data.pkg.p.q.r]",
		},
		{
			note: "multi-value rule (dots and var in head) with other rule (ref) overlap",
			modules: modules(
				`package pkg
				import future.keywords
				p[q] contains v if { v := ["a", "b"][_] }
				p.q.r if { true }
				`),
		},
		{
			note: "function with other rule (ref) overlap",
			modules: modules(
				`package pkg
				p(x) := x
				p.q.r if { true }
				`),
			err: "rego_type_error: rule data.pkg.p conflicts with [data.pkg.p.q.r]",
		},
		{
			note: "function with other rule (ref) overlap",
			modules: modules(
				`package pkg
				p(x) := x
				p.q.r if { true }
				`),
			err: "rego_type_error: rule data.pkg.p conflicts with [data.pkg.p.q.r]",
		},
		{
			note: "function (ref) with other rule (ref) overlap",
			modules: modules(
				`package pkg
				p.q(x) := x
				p.q.r if { true }
				`),
			err: "rego_type_error: rule data.pkg.p.q conflicts with [data.pkg.p.q.r]",
		},
	}
	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			mods := make(map[string]*Module, len(tc.modules))
			for i, m := range tc.modules {
				mods[strconv.Itoa(i)] = m
			}
			c := NewCompiler()
			c.Modules = mods
			compileStages(c, StageCheckRuleConflicts)
			if tc.err != "" {
				assertCompilerErrorStrings(t, c, []string{tc.err})
			} else {
				assertCompilerErrorStrings(t, c, []string{})
			}
		})
	}
}

func TestCompilerCheckRulePkgConflicts(t *testing.T) {
	tests := []struct {
		note    string
		modules []*Module
		err     []string
	}{
		{
			note: "Package can be declared within dynamic extent of rule (#6387 regression test)",
			modules: modules(
				`package test
					p[x] := y if { x := "a"; y := "b" }`,
				`package test.p
					q := 1`),
		},
		{
			note: "Package can be declared deep within dynamic extent of rule (#6387 regression test)",
			modules: modules(
				`package test
					p[x] := y if { x := "a"; y := "b" }`,
				`package test.p.q.r.s
					t := 1`),
		},
		{
			note: "Package cannot be declared within extent of single-value rule (ground ref)",
			modules: modules(
				`package test
					p := x if { x := "a" }`,
				`package test.p
					q := 1`),
			err: []string{
				"rego_type_error: package test.p conflicts with rule p defined at mod0.rego:2",
				"rego_type_error: rule data.test.p conflicts with [data.test.p.q]",
			},
		},
		{
			note: "Package cannot be declared within extent of multi-value rule",
			modules: modules(
				`package test
					p contains x if { x := "a" }`,
				`package test.p
					q := 1`),
			err: []string{
				"rego_type_error: package test.p conflicts with rule p defined at mod0.rego:2",
				"rego_type_error: rule data.test.p conflicts with [data.test.p.q]",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			mods := make(map[string]*Module, len(tc.modules))
			for i, m := range tc.modules {
				mods[strconv.Itoa(i)] = m
			}
			c := NewCompiler()
			c.Modules = mods
			compileStages(c, StageCheckRuleConflicts)
			if len(tc.err) > 0 {
				assertCompilerErrorStrings(t, c, tc.err)
			} else {
				assertCompilerErrorStrings(t, c, []string{})
			}
		})
	}
}

func TestCompilerCheckUndefinedFuncs(t *testing.T) {

	module := `
		package test
		import rego.v1

		undefined_function if {
			data.deadbeef(x)
		}

		undefined_global if {
			deadbeef(x)
		}

		# NOTE: all the dynamic dispatch examples here are not supported,
		#       we're checking assertions about the error returned.
		undefined_dynamic_dispatch if {
			x = "f"; data.test2[x](1)
		}

		undefined_dynamic_dispatch_declared_var if {
			y := "f"; data.test2[y](1)
		}

		undefined_dynamic_dispatch_declared_var_in_array if {
			z := "f"; data.test2[[z]](1)
		}

		arity_mismatch_1 if {
			data.test2.f(1,2,3)
		}

		arity_mismatch_2 if {
			data.test2.f()
		}

		arity_mismatch_3 if {
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
		"test.rego:32: rego_type_error: function data.test2.f has arity 1, got 0 arguments",
		"test.rego:36: rego_type_error: function data.test2.f has arity 1, got 0 arguments",
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
		expected any
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

				f(__local0__) = __local5__ { data.test.g(__local0__, __local5__) }

				pi = __local6__ { plus(3, 0.14, __local6__) }

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

				f(__local0__[0]) { __local0__ = [1] }`,
		},
		{
			note: "every: domain (array)",
			module: `
			package test

			p { every x in [1,2] { x } }`,
			expected: `
			package test

			p { __local2__ = [1, 2]; every __local0__, __local1__ in __local2__ { __local1__ } }`,
		},
		{
			note: "every: domain (call)",
			module: `
			package test

			p { every x in numbers.range(1, 3) { x } }`,
			expected: `
			package test

			p = true {
				numbers.range(1, 3, __local3__)
				__local2__ = __local3__
				every __local0__, __local1__ in __local2__ {
					__local1__
				}
			}`,
		},
		{
			note: "every: domain (nested calls)",
			module: `
			package test

			p { every x in numbers.range(1 + 2, 3 * 4) { x } }`,
			expected: `
			package test

			p = true {
				plus(1, 2, __local3__)
				mul(3, 4, __local4__)
				numbers.range(__local3__, __local4__, __local5__)
				__local2__ = __local5__
				every __local0__, __local1__ in __local2__ {
					__local1__
				}
			}`,
		},
		// Regression test for GH issue #6790
		{
			note: "every: domain (array with call)",
			module: `
			package test

			p { every x in [1 / 2, "foo", abs(-1)] { x } }`,
			expected: `
			package test

			p = true {
				div(1, 2, __local3__)
				abs(-1, __local4__)
				__local2__ = [__local3__, "foo", __local4__]
				every __local0__, __local1__ in __local2__ {
					__local1__
				}
			}`,
		},
		{
			note: "every: domain (nested array with call)",
			module: `
			package test

			p { every x in [1 / 2, ["foo", abs(-1)]] { x } }`,
			expected: `
			package test

			p = true {
				div(1, 2, __local3__)
				abs(-1, __local4__)
				__local2__ = [__local3__, ["foo", __local4__]]
				every __local0__, __local1__ in __local2__ {
					__local1__
				}
			}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			compiler := NewCompiler()
			opts := ParserOptions{
				RegoVersion:        RegoV0,
				AllFutureKeywords:  true,
				unreleasedKeywords: true,
			}

			compiler.Modules = map[string]*Module{
				"test": MustParseModuleWithOpts(tc.module, opts),
			}
			compileStages(compiler, StageRewriteExprTerms)

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
			opts := ParserOptions{
				RegoVersion:        RegoV0,
				AllFutureKeywords:  true,
				unreleasedKeywords: true,
			}

			compiler.Modules = map[string]*Module{
				"test": MustParseModuleWithOpts(tc.module, opts),
			}
			compileStages(compiler, StageRewriteLocalVars)

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
			note: "ignore unused rego import",
			module: `package p
			import rego.v1
			r if { 10 == 10 }
			`,
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
			note: "rule names (set construction)",
			module: `package test
				input.a { true }
				p { true }
				data.b { true }
			`,
			expectedErrors: Errors{
				&Error{
					Location: NewLocation([]byte("input.a { true }"), "", 2, 5),
					Message:  "rules must not shadow input (use a different rule name)",
				},
				&Error{
					Location: NewLocation([]byte("data.b { true }"), "", 4, 5),
					Message:  "rules must not shadow data (use a different rule name)",
				},
			},
		},
		{
			note: "rule names (object construction)",
			module: `package test
				input.a := 1 { true }
				p { true }
				data.b := 2 { true }
			`,
			expectedErrors: Errors{
				&Error{
					Location: NewLocation([]byte("input.a := 1 { true }"), "", 2, 5),
					Message:  "rules must not shadow input (use a different rule name)",
				},
				&Error{
					Location: NewLocation([]byte("data.b := 2 { true }"), "", 4, 5),
					Message:  "rules must not shadow data (use a different rule name)",
				},
			},
		},
		{
			note: "leading term in rule refs",
			module: `package test
				input.a.b { true }
				p { true }
				data.b.c := "foo" { true }
			`,
			expectedErrors: Errors{
				&Error{
					Location: NewLocation([]byte("input.a.b { true }"), "", 2, 5),
					Message:  "rules must not shadow input (use a different rule name)",
				},
				&Error{
					Location: NewLocation([]byte(`data.b.c := "foo" { true }`), "", 4, 5),
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
				"test": MustParseModuleWithOpts(tc.module, ParserOptions{RegoVersion: RegoV0}),
			}
			compileStages(compiler, "")

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

func TestCompileRegoV1Import(t *testing.T) {
	cases := []struct {
		note           string
		modules        map[string]string
		expectedErrors Errors
	}{
		// Duplicate imports
		{
			note: "duplicate imports",
			modules: map[string]string{
				"policy.rego": `package test
					import rego.v1
					import data.foo
					import data.bar.foo
					p if {
						foo == "bar"
					}`,
			},
			expectedErrors: Errors{
				&Error{
					Message:  "import must not shadow import data.foo",
					Location: &Location{Text: []byte("import"), File: "policy.rego", Row: 4, Col: 6},
				},
			},
		},
		{
			note: "duplicate imports (alias)",
			modules: map[string]string{
				"policy.rego": `package test
					import rego.v1
					import data.foo
					import data.bar as foo
					p if {
						foo == "bar"
					}`,
			},
			expectedErrors: Errors{
				&Error{
					Message:  "import must not shadow import data.foo",
					Location: &Location{Text: []byte("import"), File: "policy.rego", Row: 4, Col: 6},
				},
			},
		},
		{
			note: "duplicate imports (alias, different order)",
			modules: map[string]string{
				"policy.rego": `package test
					import rego.v1
					import data.bar as foo
					import data.foo
					p if {
						foo == "bar"
					}`,
			},
			expectedErrors: Errors{
				&Error{
					Message:  "import must not shadow import data.bar as foo",
					Location: &Location{Text: []byte("import"), File: "policy.rego", Row: 4, Col: 6},
				},
			},
		},
		{
			note: "duplicate imports (repeat)",
			modules: map[string]string{
				"policy.rego": `package test
					import rego.v1
					import data.foo
					import data.foo
					p if {
						foo == "bar"
					}`,
			},
			expectedErrors: Errors{
				&Error{
					Message:  "import must not shadow import data.foo",
					Location: &Location{Text: []byte("import"), File: "policy.rego", Row: 4, Col: 6},
				},
			},
		},
		{
			note: "duplicate imports (multiple modules)",
			modules: map[string]string{
				"policy1.rego": `package test
					import rego.v1
					import data.foo
					import data.bar.foo
					p if {
						foo == "bar"
					}`,
				"policy2.rego": `package test
					import rego.v1
					import data.foo
					import data.bar.foo
					q if {
						foo == "bar"
					}`,
			},
			expectedErrors: Errors{
				&Error{
					Message:  "import must not shadow import data.foo",
					Location: &Location{Text: []byte("import"), File: "policy1.rego", Row: 4, Col: 6},
				},
				&Error{
					Message:  "import must not shadow import data.foo",
					Location: &Location{Text: []byte("import"), File: "policy2.rego", Row: 4, Col: 6},
				},
			},
		},
		{
			note: "duplicate imports (multiple modules, not all strict)",
			modules: map[string]string{
				"policy1.rego": `package test
					import future.keywords.if
					import data.foo
					import data.bar.foo
					p if {
						foo == "bar"
					}`,
				"policy2.rego": `package test
					import rego.v1
					import data.foo
					import data.bar.foo
					q if {
						foo == "bar"
					}`,
			},
			expectedErrors: Errors{
				&Error{
					Message:  "import must not shadow import data.foo",
					Location: &Location{Text: []byte("import"), File: "policy2.rego", Row: 4, Col: 6},
				},
			},
		},
		// var shadowing
		{
			note: "var shadows input",
			modules: map[string]string{
				"policy.rego": `package test
					import rego.v1
					p if {
						input := 1
						input == 1
					}`,
			},
			expectedErrors: Errors{
				&Error{
					Message:  "variables must not shadow input (use a different variable name)",
					Location: &Location{Text: []byte("input := 1"), File: "policy.rego", Row: 4, Col: 7},
				},
			},
		},
		{
			note: "var shadows input (multiple modules)",
			modules: map[string]string{
				"policy1.rego": `package test
					import rego.v1
					p if {
						input := 1
						input == 1
					}`,
				"policy2.rego": `package test
					import rego.v1
					q if {
						input := 1
						input == 1
					}`,
			},
			expectedErrors: Errors{
				&Error{
					Message:  "variables must not shadow input (use a different variable name)",
					Location: &Location{Text: []byte("input := 1"), File: "policy1.rego", Row: 4, Col: 7},
				},
				&Error{
					Message:  "variables must not shadow input (use a different variable name)",
					Location: &Location{Text: []byte("input := 1"), File: "policy2.rego", Row: 4, Col: 7},
				},
			},
		},
		{
			note: "var shadows input (multiple modules, not all strict)",
			modules: map[string]string{
				"policy1.rego": `package test
					import future.keywords.if
					p if {
						input := 1
						input == 1
					}`,
				"policy2.rego": `package test
					import rego.v1
					q if {
						input := 1
						input == 1
					}`,
			},
			expectedErrors: Errors{
				&Error{
					Message:  "variables must not shadow input (use a different variable name)",
					Location: &Location{Text: []byte("input := 1"), File: "policy2.rego", Row: 4, Col: 7},
				},
			},
		},
		{
			note: "var shadows data",
			modules: map[string]string{
				"policy.rego": `package test
					import rego.v1
					p if {
						data := 1
						data == 1
					}`,
			},
			expectedErrors: Errors{
				&Error{
					Message:  "variables must not shadow data (use a different variable name)",
					Location: &Location{Text: []byte("data := 1"), File: "policy.rego", Row: 4, Col: 7},
				},
			},
		},
		{
			note: "var shadows data (multiple modules)",
			modules: map[string]string{
				"policy1.rego": `package test
					import rego.v1
					p if {
						data := 1
						data == 1
					}`,
				"policy2.rego": `package test
					import rego.v1
					q if {
						data := 1
						data == 1
					}`,
			},
			expectedErrors: Errors{
				&Error{
					Message:  "variables must not shadow data (use a different variable name)",
					Location: &Location{Text: []byte("data := 1"), File: "policy1.rego", Row: 4, Col: 7},
				},
				&Error{
					Message:  "variables must not shadow data (use a different variable name)",
					Location: &Location{Text: []byte("data := 1"), File: "policy2.rego", Row: 4, Col: 7},
				},
			},
		},
		{
			note: "var shadows data (multiple modules, not all strict)",
			modules: map[string]string{
				"policy1.rego": `package test
					import future.keywords.if
					p if {
						data := 1
						data == 1
					}`,
				"policy2.rego": `package test
					import rego.v1
					q if {
						data := 1
						data == 1
					}`,
			},
			expectedErrors: Errors{
				&Error{
					Message:  "variables must not shadow data (use a different variable name)",
					Location: &Location{Text: []byte("data := 1"), File: "policy2.rego", Row: 4, Col: 7},
				},
			},
		},
		// rule shadowing
		{
			note: "rule shadows input",
			modules: map[string]string{
				"policy.rego": `package test
					import rego.v1
					input := 1`,
			},
			expectedErrors: Errors{
				&Error{
					Message:  "rules must not shadow input (use a different rule name)",
					Location: &Location{Text: []byte("input := 1"), File: "policy.rego", Row: 3, Col: 6},
				},
			},
		},
		{
			note: "rule (object) shadows input",
			modules: map[string]string{
				"policy.rego": `package test
					import rego.v1
					input.a := "b" if { true }`,
			},
			expectedErrors: Errors{
				&Error{
					Message:  "rules must not shadow input (use a different rule name)",
					Location: &Location{Text: []byte(`input.a := "b" if { true }`), File: "policy.rego", Row: 3, Col: 6},
				},
			},
		},
		{
			note: "rule (set) shadows input",
			modules: map[string]string{
				"policy.rego": `package test
					import rego.v1
					input contains "a" if { true }`,
			},
			expectedErrors: Errors{
				&Error{
					Message:  "rules must not shadow input (use a different rule name)",
					Location: &Location{Text: []byte(`input contains "a" if { true }`), File: "policy.rego", Row: 3, Col: 6},
				},
			},
		},
		{
			note: "rule ref shadows input",
			modules: map[string]string{
				"policy.rego": `package test
					import rego.v1
					input.a.b.c := 1`,
			},
			expectedErrors: Errors{
				&Error{
					Message:  "rules must not shadow input (use a different rule name)",
					Location: &Location{Text: []byte("input.a.b.c := 1"), File: "policy.rego", Row: 3, Col: 6},
				},
			},
		},
		{
			note: "rule shadows input (multiple modules)",
			modules: map[string]string{
				"policy1.rego": `package test
					import rego.v1
					input := 1`,
				"policy2.rego": `package test2
					import rego.v1
					input := 2`,
			},
			expectedErrors: Errors{
				&Error{
					Message:  "rules must not shadow input (use a different rule name)",
					Location: &Location{Text: []byte("input := 1"), File: "policy1.rego", Row: 3, Col: 6},
				},
				&Error{
					Message:  "rules must not shadow input (use a different rule name)",
					Location: &Location{Text: []byte("input := 2"), File: "policy2.rego", Row: 3, Col: 6},
				},
			},
		},
		{
			note: "rule shadows input (multiple modules, not all strict)",
			modules: map[string]string{
				"policy1.rego": `package test
					input := 1`,
				"policy2.rego": `package test2
					import rego.v1
					input := 2`,
			},
			expectedErrors: Errors{
				&Error{
					Message:  "rules must not shadow input (use a different rule name)",
					Location: &Location{Text: []byte("input := 2"), File: "policy2.rego", Row: 3, Col: 6},
				},
			},
		},
		{
			note: "rule shadows data",
			modules: map[string]string{
				"policy.rego": `package test
					import rego.v1
					data := 1`,
			},
			expectedErrors: Errors{
				&Error{
					Message:  "rules must not shadow data (use a different rule name)",
					Location: &Location{Text: []byte("data := 1"), File: "policy.rego", Row: 3, Col: 6},
				},
			},
		},
		{
			note: "rule (object) shadows input",
			modules: map[string]string{
				"policy.rego": `package test
					import rego.v1
					data.a := "b" if { true }`,
			},
			expectedErrors: Errors{
				&Error{
					Message:  "rules must not shadow data (use a different rule name)",
					Location: &Location{Text: []byte(`data.a := "b" if { true }`), File: "policy.rego", Row: 3, Col: 6},
				},
			},
		},
		{
			note: "rule (set) shadows input",
			modules: map[string]string{
				"policy.rego": `package test
					import rego.v1
					data contains "a" if { true }`,
			},
			expectedErrors: Errors{
				&Error{
					Message:  "rules must not shadow data (use a different rule name)",
					Location: &Location{Text: []byte(`data contains "a" if { true }`), File: "policy.rego", Row: 3, Col: 6},
				},
			},
		},
		{
			note: "rule ref shadows input",
			modules: map[string]string{
				"policy.rego": `package test
					import rego.v1
					data.a.b.c := 1`,
			},
			expectedErrors: Errors{
				&Error{
					Message:  "rules must not shadow data (use a different rule name)",
					Location: &Location{Text: []byte("data.a.b.c := 1"), File: "policy.rego", Row: 3, Col: 6},
				},
			},
		},
		{
			note: "rule shadows data (multiple modules)",
			modules: map[string]string{
				"policy1.rego": `package test
					import rego.v1
					data := 1`,
				"policy2.rego": `package test2
					import rego.v1
					data := 2`,
			},
			expectedErrors: Errors{
				&Error{
					Message:  "rules must not shadow data (use a different rule name)",
					Location: &Location{Text: []byte("data := 1"), File: "policy1.rego", Row: 3, Col: 6},
				},
				&Error{
					Message:  "rules must not shadow data (use a different rule name)",
					Location: &Location{Text: []byte("data := 2"), File: "policy2.rego", Row: 3, Col: 6},
				},
			},
		},
		{
			note: "rule shadows data (multiple modules, not all strict)",
			modules: map[string]string{
				"policy1.rego": `package test
					data := 1`,
				"policy2.rego": `package test2
					import rego.v1
					data := 2`,
			},
			expectedErrors: Errors{
				&Error{
					Message:  "rules must not shadow data (use a different rule name)",
					Location: &Location{Text: []byte("data := 2"), File: "policy2.rego", Row: 3, Col: 6},
				},
			},
		},
		// deprecated built-ins
		{
			note: "deprecated built-in",
			modules: map[string]string{
				"policy.rego": `package test
					import rego.v1
					p := all([true, false])`,
			},
			expectedErrors: Errors{
				&Error{
					Message:  "deprecated built-in function calls in expression: all",
					Location: &Location{Text: []byte("all([true, false])"), File: "policy.rego", Row: 3, Col: 11},
				},
			},
		},
		{
			note: "deprecated built-in (multiple)",
			modules: map[string]string{
				"policy.rego": `package test
					import rego.v1
					p := all([true, false])
					q := any([true, false])`,
			},
			expectedErrors: Errors{
				&Error{
					Message:  "deprecated built-in function calls in expression: all",
					Location: &Location{Text: []byte("all([true, false])"), File: "policy.rego", Row: 3, Col: 11},
				},
				&Error{
					Message:  "deprecated built-in function calls in expression: any",
					Location: &Location{Text: []byte("any([true, false])"), File: "policy.rego", Row: 4, Col: 11},
				},
			},
		},
		{
			note: "deprecated built-in (multiple modules)",
			modules: map[string]string{
				"policy1.rego": `package test
					import rego.v1
					p := all([true, false])`,
				"policy2.rego": `package test
					import rego.v1
					q := all([true, false])`,
			},
			expectedErrors: Errors{
				&Error{
					Message:  "deprecated built-in function calls in expression: all",
					Location: &Location{Text: []byte("all([true, false])"), File: "policy1.rego", Row: 3, Col: 11},
				},
				&Error{
					Message:  "deprecated built-in function calls in expression: all",
					Location: &Location{Text: []byte("all([true, false])"), File: "policy2.rego", Row: 3, Col: 11},
				},
			},
		},
		{
			note: "deprecated built-in (multiple modules, not all strict)",
			modules: map[string]string{
				"policy1.rego": `package test
					p := all([true, false])`,
				"policy2.rego": `package test
					import rego.v1
					q := all([true, false])`,
			},
			expectedErrors: Errors{
				&Error{
					Message:  "deprecated built-in function calls in expression: all",
					Location: &Location{Text: []byte("all([true, false])"), File: "policy2.rego", Row: 3, Col: 11},
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			compiler := NewCompiler()
			compiler.Modules = map[string]*Module{}
			for name, mod := range tc.modules {
				if parsed, err := ParseModuleWithOpts(name, mod, ParserOptions{RegoVersion: RegoV0}); err != nil {
					t.Fatal(err)
				} else {
					compiler.Modules[name] = parsed
				}
			}
			compileStages(compiler, "")
			assertErrors(t, compiler.Errors, tc.expectedErrors, true)
		})
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
a.b.c = 1 if { true }
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
p := __local0__ if { __local1__ = [x | data.test.q[x]]; count(__local1__, __local0__) }
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

import rego.v1
import data.doc1 as bar
import input.x.y.foo
import input.qux as baz

p[foo[bar[i]]] := {"baz": baz} if { true }`)

	c.Modules["elsekw"] = MustParseModule(`package elsekw

	import rego.v1
	import input.x.y.foo
	import data.doc1 as bar
	import input.baz

	p if {
		false
	} else = foo if {
		bar
	} else = baz if {
		true
	}
	`)

	c.Modules["nestedexprs"] = MustParseModule(`package nestedexprs
		import rego.v1

		x = 1

		p if {
			f(g(x))
		}`)

	c.Modules["assign"] = MustParseModule(`package assign
		import rego.v1

		x = 1
		y = 1

		p if {
			x := y
			[true | x := y]
		}`)

	c.Modules["someinassign"] = MustParseModule(`package someinassign
		import rego.v1

		x = 1
		y = 1

		p[x] if {
			some x in [1, 2, y]
		}`)

	c.Modules["someinassignwithkey"] = MustParseModule(`package someinassignwithkey
		import rego.v1

		x = 1
		y = 1

		p[x] if {
			some k, v in [1, 2, y]
		}`)

	c.Modules["donotresolve"] = MustParseModule(`package donotresolve
		import rego.v1

		x = 1

		f(x) if {
			x = 2
		}
		`)

	c.Modules["indirectrefs"] = MustParseModule(`package indirectrefs
		import rego.v1

		f(x) = [x] if {true}

		p if {
			f(1)[0]
		}
		`)

	c.Modules["comprehensions"] = MustParseModule(`package comprehensions
		import rego.v1

		nums = [1, 2, 3]

		f(x) = [x] if {true}

		p[[1]] if {true}

		q if {
			p[[x | x = nums[_]]]
		}

		r = [y | y = f(1)[0]]
		`)

	c.Modules["everykw"] = MustParseModule(`package everykw
		import rego.v1

		nums = {1, 2, 3}
		f(_) = true
		x = 100
		xs = [1, 2, 3]
		p if {
			every x in xs {
				nums[x]
				x > 10
			}
		}`)

	c.Modules["heads_with_dots"] = MustParseModule(`package heads_with_dots
		import rego.v1

		this_is_not = true
		this.is.dotted if { this_is_not }
	`)

	compileStages(c, StageResolveRefs)
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
			import rego.v1

			f([input]) if { true }
		`),
	}

	compileStages(c, StageResolveRefs)

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
			mod: module(`package head
import data.doc1 as bar
import data.doc2 as corge
import input.x.y.foo
import input.qux as baz

p[foo[bar[i]]] = {"baz": baz, "corge": corge} if { true }
`),
			exp: MustParseRule(`p[__local0__] = __local1__ { __local0__ = input.x.y.foo[data.doc1[i]]; __local1__ = {"baz": input.qux, "corge": data.doc2} }`),
		},
		{
			note: "array comprehension value",
			mod: module(`package head
q = [true | true] if { true }
`),
			exp: MustParseRule(`q = __local0__ { __local0__ = [true | true] }`),
		},
		{
			note: "array comprehension value in else head",
			mod: module(`package head
q if {
	false
} else = [true | true] if {
	true
}
`),
			exp: MustParseRule(`q = true { false } else = __local0__ { __local0__ = [true | true] }`),
		},
		{
			note: "array comprehension value in head (comprehension-local var)",
			mod: module(`package head
q = [a | a := true] if {
	false
} else = [a | a := true] if {
	true
}
`),
			exp: MustParseRule(`q = __local2__ { false; __local2__ = [__local0__ | __local0__ = true] } else = __local3__ { __local3__ = [__local1__ | __local1__ = true] }`),
		},
		{
			note: "array comprehension value in function head (comprehension-local var)",
			mod: module(`package head
f(x) = [a | a := true] if {
	false
} else = [a | a := true] if {
	true
}
`),
			exp: MustParseRule(`f(__local0__) = __local3__ { false; __local3__ = [__local1__ | __local1__ = true] } else = __local4__ { __local4__ = [__local2__ | __local2__ = true] }`),
		},
		{
			note: "array comprehension value in else-func head (reused arg rewrite)",
			mod: module(`package head
f(x, y) = [x | y] if {
	false
} else = [x | y] if {
	true
}
`),
			exp: MustParseRule(`f(__local0__, __local1__) = __local2__ { false; __local2__ = [__local0__ | __local1__] } else = __local3__ { __local3__ = [__local0__ | __local1__] }`),
		},
		{
			note: "object comprehension value",
			mod: module(`package head
r = {"true": true | true} if { true }
`),
			exp: MustParseRule(`r = __local0__ { __local0__ = {"true": true | true} }`),
		},
		{
			note: "object comprehension value in else head",
			mod: module(`package head
q if {
	false
} else = {"true": true | true} if {
	true
}
`),
			exp: MustParseRule(`q = true { false } else = __local0__ { __local0__ = {"true": true | true} }`),
		},
		{
			note: "object comprehension value in head (comprehension-local var)",
			mod: module(`package head
q = {"a": a | a := true} if {
	false
} else = {"a": a | a := true} if {
	true
}
`),
			exp: MustParseRule(`q = __local2__ { false; __local2__ = {"a": __local0__ | __local0__ = true} } else = __local3__ { __local3__ = {"a": __local1__ | __local1__ = true} }`),
		},
		{
			note: "object comprehension value in function head (comprehension-local var)",
			mod: module(`package head
f(x) = {"a": a | a := true} if {
	false
} else = {"a": a | a := true} if {
	true
}
`),
			exp: MustParseRule(`f(__local0__) = __local3__ { false; __local3__ = {"a": __local1__ | __local1__ = true} } else = __local4__ { __local4__ = {"a": __local2__ | __local2__ = true} }`),
		},
		{
			note: "object comprehension value in else-func head (reused arg rewrite)",
			mod: module(`package head
f(x, y) = {x: y | true} if {
	false
} else = {x: y | true} if {
	true
}
`),
			exp: MustParseRule(`f(__local0__, __local1__) = __local2__ { false; __local2__ = {__local0__: __local1__ | true} } else = __local3__ { __local3__ = {__local0__: __local1__ | true} }`),
		},
		{
			note: "set comprehension value",
			mod: module(`package head
s = {true | true} if { true }
`),
			exp: MustParseRule(`s = __local0__ { __local0__ = {true | true} }`),
		},
		{
			note: "set comprehension value in else head",
			mod: module(`package head
q = {false | false} if {
	false
} else = {true | true} if {
	true
}
`),
			exp: MustParseRule(`q = __local0__ { false; __local0__ = {false | false} } else = __local1__ { __local1__ = {true | true} }`),
		},
		{
			note: "set comprehension value in head (comprehension-local var)",
			mod: module(`package head
q = {a | a := true} if {
	false
} else = {a | a := true} if {
	true
}
`),
			exp: MustParseRule(`q = __local2__ { false; __local2__ = {__local0__ | __local0__ = true} } else = __local3__ { __local3__ = {__local1__ | __local1__ = true} }`),
		},
		{
			note: "set comprehension value in function head (comprehension-local var)",
			mod: module(`package head
f(x) = {a | a := true} if {
	false
} else = {a | a := true} if {
	true
}
`),
			exp: MustParseRule(`f(__local0__) = __local3__ { false; __local3__ = {__local1__ | __local1__ = true} } else = __local4__ { __local4__ = {__local2__ | __local2__ = true} }`),
		},
		{
			note: "set comprehension value in else-func head (reused arg rewrite)",
			mod: module(`package head
f(x, y) = {x | y} if {
	false
} else = {x | y} if {
	true
}
`),
			exp: MustParseRule(`f(__local0__, __local1__) = __local2__ { false; __local2__ = {__local0__ | __local1__} } else = __local3__ { __local3__ = {__local0__ | __local1__} }`),
		},
		{
			note: "import in else value",
			mod: module(`package head
import input.qux as baz
elsekw if {
	false
} else = baz if {
	true
}
`),
			exp: MustParseRule(`elsekw { false } else = __local0__ { __local0__ = input.qux }`),
		},
		{
			note: "import ref in last ref head term",
			mod: module(`package head
import data.doc1 as bar
x.y.z[bar[i]] = true
`),
			exp: MustParseRule(`x.y.z[__local0__] = true { __local0__ = data.doc1[i] }`),
		},
		{
			note: "import ref in multi-value ref rule",
			mod: module(`package head
import data.doc1 as bar
x.y.w contains bar[i] if true
`),
			exp: func() *Rule {
				exp, _ := ParseRuleWithOpts(`x.y.w contains __local0__ if { __local0__ = data.doc1[i] }`, popts)
				return exp
			}(),
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			c := NewCompiler()
			c.Modules["head"] = tc.mod
			compileStages(c, StageRewriteRefsInHead)
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
			compileStages(c, StageRewriteRefsInHead)
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

p if {
	rego.metadata.chain()[0].path == ["test", "p"]
	rego.metadata.rule() == {}
}`,
			exp: `package test

p = true if {
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

p if {
	rego.metadata.chain()
	rego.metadata.rule()
}`,
			exp: `package test

p = true if {
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
p if {
	rego.metadata.chain()[0].title == "My P Rule"
	rego.metadata.chain()[1].description == "A test package"
}

# METADATA
# title: My Other P Rule
p if {
	rego.metadata.rule().title == "My Other P Rule"
}`,
			exp: `# METADATA
# {"scope":"package","description":"A test package"}
package test

# METADATA
# {"scope":"rule","title":"My P Rule"}
p = true if {
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
p = true if {
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

p if {
	rego.metadata.chain()[0].path == ["test", "p"]
	rego.metadata.chain()[1].path == ["test"]
}`,
			exp: `# METADATA
# {"scope":"package","description":"TEST"}
package test

p = true if {
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

p := __local0__ if {
	__local1__ = [{"path": ["test", "p"]}]
	__local0__ = __local1__
}`,
		},
		{
			note: "rego.metadata argument in function call",
			module: `package test

p if {
	q(rego.metadata.chain())
}

q(s) if {
	s == ["test", "p"]
}`,
			exp: `package test

p = true if {
	__local2__ = [{"path": ["test", "p"]}]
	__local1__ = __local2__
	data.test.q(__local1__)
}

q(__local0__) = true if {
	equal(__local0__, ["test", "p"])
}`,
		},
		{
			note: "rego.metadata used in array comprehension",
			module: `package test

p = [x | x := rego.metadata.chain()]`,
			exp: `package test

p = [__local0__ | __local1__ = __local2__; __local0__ = __local1__] if {
	__local2__ = [{"path": ["test", "p"]}]
	true
}`,
		},
		{
			note: "rego.metadata used in nested array comprehension",
			module: `package test

p if {
	y := [x | x := rego.metadata.chain()]
	y[0].path == ["test", "p"]
}`,
			exp: `package test

p = true if {
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

p = {__local0__ | __local1__ = __local2__; __local0__ = __local1__} if {
	__local2__ = [{"path": ["test", "p"]}]
	true
}`,
		},
		{
			note: "rego.metadata used in nested set comprehension",
			module: `package test

p if {
	y := {x | x := rego.metadata.chain()}
	y[0].path == ["test", "p"]
}`,
			exp: `package test

p = true if {
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

p = {i: __local0__ | __local1__ = __local2__; __local0__ = __local1__[i]} if {
	__local2__ = [{"path": ["test", "p"]}]
	true
}`,
		},
		{
			note: "rego.metadata used in nested object comprehension",
			module: `package test

p if {
	y := {i: x | x := rego.metadata.chain()[i]}
	y[0].path == ["test", "p"]
}`,
			exp: `package test

p = true if {
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
				"test.rego": module(tc.module),
			}
			compileStages(c, StageRewriteRegoMetadataCalls)
			assertNotFailed(t, c)

			result := c.Modules["test.rego"]
			exp := MustParseModuleWithOpts(tc.exp, ParserOptions{
				AllFutureKeywords:  true,
				unreleasedKeywords: true,
				ProcessAnnotation:  true,
			})

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

	compileStages(c, "")
	assertNotFailed(t, c)
}

func TestCompilerRewriteLocalAssignments(t *testing.T) {

	tests := []struct {
		module          string
		exp             any
		expRewrittenMap map[Var]Var
		regoVersion     RegoVersion
	}{
		{
			module: `
				package test
				body if { a := 1; a > 0 }
			`,
			exp: `
				package test
				body = true if { __local0__ = 1; gt(__local0__, 0) }
			`,
			expRewrittenMap: map[Var]Var{
				Var("__local0__"): Var("a"),
			},
		},
		{
			module: `
				package test
				head_vars(a) = b if { b := a }
			`,
			exp: `
				package test
				head_vars(__local0__) = __local1__ if { __local1__ = __local0__ }
			`,
			expRewrittenMap: map[Var]Var{
				Var("__local0__"): Var("a"),
				Var("__local1__"): Var("b"),
			},
		},
		{
			module: `
				package test
				head_key contains a if { a := 1 }
			`,
			exp: `
				package test
				head_key contains __local0__ if { __local0__ = 1 }
			`,
			expRewrittenMap: map[Var]Var{
				Var("__local0__"): Var("a"),
			},
		},
		{
			module: `
				package test
				head_unsafe_var contains a if { some a }
			`,
			exp: `
				package test
				head_unsafe_var contains __local0__ if { true }
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
				head_nested contains p[x] if {
					some x
				}`,
			exp: `
					package test
					p = {1,2,3}
					x = 4
					head_nested contains data.test.p[__local0__]
			`,
			expRewrittenMap: map[Var]Var{
				Var("__local0__"): Var("x"),
			},
		},
		{
			module: `
				package test
				p = {1,2}
				head_closure_nested contains p[x] if {
					y = [true | some x; x = 1]
				}
			`,
			exp: `
				package test
				p = {1,2}
				head_closure_nested contains data.test.p[x] if {
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
				nested if {
					a := [1,2,3]
					x := [true | a[i] > 1]
				}
			`,
			exp: `
				package test
				nested = true if { __local0__ = [1, 2, 3]; __local1__ = [true | gt(__local0__[i], 1)] }
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
				shadow_globals contains x if { x := 1 }
			`,
			exp: `
				package test
				x = 2 if { true }
				shadow_globals contains __local0__ if { __local0__ = 1 }
			`,
			expRewrittenMap: map[Var]Var{
				Var("__local0__"): Var("x"),
			},
		},
		{
			module: `
				package test
				shadow_rule contains shadow_rule if { shadow_rule := 1 }
			`,
			exp: `
				package test
				shadow_rule contains __local0__ if { __local0__ = 1 }
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
			regoVersion: RegoV0, // shadowing only allowed in v0
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
			regoVersion: RegoV0, // shadowing only allowed in v0
		},
		{
			module: `
				package test
				skip_with_target {
					a := 1
					input := 2
					data.p with input as a
					data.p with input.foo as a
				}
			`,
			exp: `
				package test
				skip_with_target = true { __local0__ = 1; __local1__ = 2; data.p with input as __local0__; data.p with input.foo as __local0__ }
			`,
			expRewrittenMap: map[Var]Var{
				Var("__local0__"): Var("a"),
				Var("__local1__"): Var("input"),
			},
			regoVersion: RegoV0, // shadowing only allowed in v0
		},
		{
			module: `
				package test
				shadow_comprehensions if {
					a := 1
					[true | a := 2; b := 1]
					b := 2
				}
			`,
			exp: `
				package test
				shadow_comprehensions = true if { __local0__ = 1; [true | __local1__ = 2; __local2__ = 1]; __local3__ = 2 }
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
					scoping if {
						[true | a := 1]
						[true | a := 2]
					}
			`,
			exp: `
				package test
				scoping = true if { [true | __local0__ = 1]; [true | __local1__ = 2] }
			`,
			expRewrittenMap: map[Var]Var{
				Var("__local0__"): Var("a"),
				Var("__local1__"): Var("a"),
			},
		},
		{
			module: `
				package test
				object_keys if {
					{k: v1, "k2": v2} := {"foo": 1, "k2": 2}
				}
			`,
			exp: `
				package test
				object_keys = true if { {"k2": __local0__, k: __local1__} = {"foo": 1, "k2": 2} }
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
				head_array_comprehensions = [[__local0__] | __local0__ = 1] if { true }
				head_set_comprehensions = {[__local1__] | __local1__ = 1} if { true }
				head_object_comprehensions = {__local2__: [__local3__] | __local2__ = "foo"; __local3__ = 1} if { true }
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
				rewritten_object_key if {
					k := "foo"
					{k: 1}
				}
			`,
			exp: `
				package test
				rewritten_object_key = true if { __local0__ = "foo"; {__local0__: 1} }
			`,
			expRewrittenMap: map[Var]Var{
				Var("__local0__"): Var("k"),
			},
		},
		{
			module: `
				package test
				rewritten_object_key_head contains [{k: 1}] if {
					k := "foo"
				}
			`,
			exp: `
				package test
				rewritten_object_key_head contains [{__local0__: 1}] if { __local0__ = "foo" }
			`,
			expRewrittenMap: map[Var]Var{
				Var("__local0__"): Var("k"),
			},
		},
		{
			module: `
				package test
				rewritten_object_key_head_value = [{k: 1}] if {
					k := "foo"
				}
			`,
			exp: `
				package test
				rewritten_object_key_head_value = [{__local0__: 1}] if { __local0__ = "foo" }
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
					a := [true | true with input as 2; true with input.foo as 3]
				}
			`,
			exp: `
				package test
				skip_with_target_in_assignment = true { __local0__ = 1; __local1__ = [true | true with input as 2; true with input.foo as 3] }
			`,
			expRewrittenMap: map[Var]Var{
				Var("__local0__"): Var("input"),
				Var("__local1__"): Var("a"),
			},
			regoVersion: RegoV0, // shadowing only allowed in v0
		},
		{
			module: `
				package test
				rewrite_with_value_in_assignment if {
					a := 1
					b := 1 with input as [a]
				}
			`,
			exp: `
				package test
				rewrite_with_value_in_assignment = true if { __local0__ = 1; __local1__ = 1 with input as [__local0__] }
			`,
			expRewrittenMap: map[Var]Var{
				Var("__local0__"): Var("a"),
				Var("__local1__"): Var("b"),
			},
		},
		{
			module: `
				package test
				rewrite_with_value_in_expr if {
					a := 1
					a > 0 with input as [a]
				}
			`,
			exp: `
				package test
				rewrite_with_value_in_expr = true if { __local0__ = 1; gt(__local0__, 0) with input as [__local0__] }
			`,
			expRewrittenMap: map[Var]Var{
				Var("__local0__"): Var("a"),
			},
		},
		{
			module: `
				package test
				rewrite_nested_with_value_in_expr if {
					a := 1
					a > 0 with input as object.union({"a": a}, {"max_a": max([a])})
				}
			`,
			exp: `
				package test
				rewrite_nested_with_value_in_expr = true if { __local0__ = 1; gt(__local0__, 0) with input as object.union({"a": __local0__}, {"max_a": max([__local0__])}) }
			`,
			expRewrittenMap: map[Var]Var{
				Var("__local0__"): Var("a"),
			},
		},
		{
			module: `
				package test
				global = {}
				ref_shadowed if {
					global := {"a": 1}
					global.a > 0
				}
			`,
			exp: `
				package test
				global = {} if { true }
				ref_shadowed = true if { __local0__ = {"a": 1}; gt(__local0__.a, 0) }
			`,
			expRewrittenMap: map[Var]Var{
				Var("__local0__"): Var("global"),
			},
		},
		{
			module: `
				package test
				f(x) = y if {
					x == 1
					y := 2
				} else = y if {
					x == 3
					y := 4
				}
			`,
			// Each "else" rule has a separate rule head and the vars in the
			// args will be rewritten. Since we cannot currently redefine the
			// args, we must parse the module and then manually update the args.
			exp: func() *Module {
				module := module(`
					package test

					f(__local0__) = __local1__ if { __local0__ == 1; __local1__ = 2 } else = __local2__ if { __local0__ == 3; __local2__ = 4 }
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
				f({"x": [x]}) = y if { x == 1; y := 2 }`,
			exp: `
				package test

				f({"x": [__local0__]}) = __local1__ if { __local0__ == 1; __local1__ = 2 }`,
			expRewrittenMap: map[Var]Var{
				Var("__local0__"): Var("x"),
				Var("__local1__"): Var("y"),
			},
		},
		{
			module: `
				package test

				f(x, [x]) = x if { x == 1 }
			`,
			exp: `
				package test

				f(__local0__, [__local0__]) = __local0__ if { __local0__ == 1 }
			`,
			expRewrittenMap: map[Var]Var{
				Var("__local0__"): Var("x"),
			},
		},
		{
			module: `
				package test

				f(x) = {x[0]: 1} if { true }
			`,
			exp: `
				package test

				f(__local0__) = {__local0__[0]: 1} if { true }
			`,
			expRewrittenMap: map[Var]Var{
				Var("__local0__"): Var("x"),
			},
		},
		{
			module: `
				package test

				f({{t | t := 0}: 1}) if {
					true
				}
			`,
			exp: `
				package test

				f({{__local0__ | __local0__ = 0}: 1}) if { true }
			`,
			expRewrittenMap: map[Var]Var{
				Var("__local0__"): Var("t"),
			},
		},
		{
			module: `
				package test

				f({{t | t := 0}}) if {
					true
				}
			`,
			exp: `
				package test

				f({{__local0__ | __local0__ = 0}}) if { true }
			`,
			expRewrittenMap: map[Var]Var{
				Var("__local0__"): Var("t"),
			},
		},
	}

	for i, tc := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			setRegoVersion := func(po ParserOptions) ParserOptions {
				po.RegoVersion = tc.regoVersion
				return po
			}

			c := NewCompiler()
			c.Modules = map[string]*Module{
				"test.rego": module(tc.module, setRegoVersion),
			}
			compileStages(c, StageRewriteLocalVars)
			assertNotFailed(t, c)
			result := c.Modules["test.rego"]
			var exp *Module
			switch e := tc.exp.(type) {
			case string:
				exp = module(e, setRegoVersion)
			case func() *Module:
				exp = e()
			default:
				panic("expected value must be string or func() *Module")
			}
			if result.Compare(exp) != 0 {
				t.Fatalf("\nExpected:\n\n%v\n\nGot:\n\n%v", exp, result)
			}
			if !maps.Equal(c.RewrittenVars, tc.expRewrittenMap) {
				t.Fatalf("\nExpected Rewritten Vars:\n\n\t%+v\n\nGot:\n\n\t%+v\n\n", tc.expRewrittenMap, c.RewrittenVars)
			}
		})
	}

}

func TestRewriteLocalVarDeclarationErrors(t *testing.T) {

	c := NewCompiler()

	c.Modules["test"] = module(`package test

	redeclaration if {
		r1 = 1
		r1 := 2
		r2 := 1
		[b, r2] := [1, 2]
		foo.path == 1
		foo := "foo"
		_ := [1 | nested := 1; nested := 2]
	}

	negation if {
		not a := 1
	}

	bad_assign if {
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

	arg_redeclared(arg1) if {
		arg1 := 1
	}

	arg_nested_redeclared({{arg_nested| arg_nested := 1; arg_nested := 2}}) if { true }
	`)

	compileStages(c, StageRewriteLocalVars)

	expectedErrors := []string{
		"var r1 referenced above",
		"var r2 assigned above",
		"var foo referenced above",
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

	result := make([]string, 0, len(c.Errors))

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

				p if {
					a := {"a": "a"}
					{a.a: a.a}
				}
			`,
			exp: `
				package test

				p if {
					__local0__ = {"a": "a"}
					{__local0__.a: __local0__.a}
				}
			`,
		},
		{
			note: "set ref element",
			module: `
				package test

				p if {
					a := {"a": "a"}
					{a.a}
				}
			`,
			exp: `
				package test

				p if {
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
				"test.rego": module(tc.module),
			}

			compileStages(c, StageRewriteLocalVars)

			exp := module(tc.exp)
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
				p if { some x; input = [x, y] }
			`,
			exp: `
				package test
				x = 1
				y = 2
				p if { __local1__ = data.test.y; input = [__local0__, __local1__] }
			`,
		},
		{
			note: "rewrite call",
			module: `
				package test
				x = []
				y = {}
				p if { some x; walk(y, [x, y]) }
			`,
			exp: `
				package test
				x = []
				y = {}
				p if { __local1__ = data.test.y; __local2__ = data.test.y; walk(__local1__, [__local0__, __local2__]) }
			`,
		},
		{
			note: "rewrite term",
			module: `
				package test
				x = "a"
				y = 1
				q contains [2, "b"]
				p if { some x; q[[y,x]] }
			`,
			exp: `
				package test
				x = "a"
				y = 1
				q contains [2, "b"]
				p if { __local1__ = data.test.y; data.test.q[[__local1__, __local0__]] }
			`,
		},
		{
			note: "with: rewrite target",
			module: `
				package test
				p if {
					x := "foo"
					true with input[x] as 1
				}
			`,
			exp: `
				package test
				p if {
					__local0__ = "foo";
					true with input[__local0__] as 1
				}
			`,
		},
		{
			note: "with: rewrite target in comprehension term",
			module: `
				package test
				p if {
					foo := "bar"
					{ { 2 | true with input[foo] as 1} | true }
				}
			`,
			exp: `
				package test
				p if {
					__local0__ = "bar"
					{__local1__ | __local1__ = { 2 | true with input[__local0__] as 1 }}
				}
			`,
		},
		{
			note: "single-value rule with ref head",
			module: `
				package test

				p.r.q[s] = t if {
					t := 1
					s := input.foo
				}
			`,
			exp: `
				package test

				p.r.q[__local1__] = __local0__ if {
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
				p if { some x in xs; x == "a" }
			`,
			exp: `
				package test
				xs = ["a", "b", "c"]
				p if { __local2__ = data.test.xs[__local1__]; __local2__ = "a" }
			`,
		},
		{
			note: "rewrite some k, x in xs",
			module: `
				package test
				import future.keywords.in
				xs = ["a", "b", "c"]
				p if { some k, x in xs; x == "a"; k == 2 }
			`,
			exp: `
				package test
				xs = ["a", "b", "c"]
				p if { __local1__ = data.test.xs[__local0__]; __local1__ = "a"; __local0__ = 2 }
			`,
		},
		{
			note: "rewrite some k, x in xs[i]",
			module: `
				package test
				import future.keywords.in
				xs = [["a", "b", "c"], []]
				p if {
					some i
					some k, x in xs[i]
					x == "a"
					k == 2
				}
			`,
			exp: `
				package test
				xs = [["a", "b", "c"], []]
				p = true if { __local2__ = data.test.xs[__local0__][__local1__]; __local2__ = "a"; __local1__ = 2 }
			`,
		},
		{
			note: "rewrite some k, x in xs[i] with `i` as ref",
			module: `
				package test
				import future.keywords.in
				i = 0
				xs = [["a", "b", "c"], []]
				p if {
					some k, x in xs[i]
					x == "a"
					k == 2
				}
			`,
			exp: `
				package test
				i = 0
				xs = [["a", "b", "c"], []]
				p = true if { __local2__ = data.test.i; __local1__ = data.test.xs[__local2__][__local0__]; __local1__ = "a"; __local0__ = 2 }
			`,
		},
		{
			note: "rewrite some: with modifier on domain",
			module: `
				package test
				p if {
					some k, x in input with input as [1, 1, 1]
					k == 0
					x == 1
				}
			`,
			exp: `
				package test
				p if {
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
				p if {
					every k, v in xs { k + v > i }
				}
			`,
			exp: `
				package test
				i = 0
				xs = [1, 2]
				k = "foo"
				v = "bar"
				p = true if {
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
				p if {
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

				p if {
					every __local0__, v in [1] { v >= 1 }
				}
			`,
			exp: `
				package test
				p = true if {
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
				p if {
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
				p if {
					every k, _ in [1] { k >= 0 }
				}
			`,
			exp: `
				package test
				p = true if {
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
				p if {
					every _, _ in [1] { true }
				}
			`,
			exp: `
				package test
				p = true if { __local0__ = [1]; every _, _ in __local0__ { true } }
			`,
		},
		{
			note: "rewrite every: declared vars with different scopes",
			module: `
				package test
				# import future.keywords.in
				# import future.keywords.every
				p if {
					some x
					x = 10
					every x in [1] { x == 1 }
				}
			`,
			exp: `
				package test
				p = true if {
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
				p if {
					some y
					y = 10
					every x in [1] { x == y }
				}
			`,
			exp: `
				package test
				p = true if {
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
				p contains x if {
					some x
					x = 10
					every _ in [1] { true }
				}
			`,
			exp: `
				package test
				p contains __local0__ if { __local0__ = 10; __local2__ = [1]; every __local1__, _ in __local2__ { true } }
			`,
		},
		{
			note: "rewrite every: nested",
			module: `
				package test
				# import future.keywords.in
				# import future.keywords.every
				p if {
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
				p = true if {
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
				p if {
					every x in input { x == 1 } with input as [1, 1, 1]
				}
			`,
			exp: `
				package test
				p if {
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
				p if {
					xs := [1, 2]
					every x in input { x == 1 } with input as xs
				}
			`,
			exp: `
				package test
				p if {
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
				p if {
					every x in [2] { x == input } with input as 2
				}
			`,
			exp: `
				package test
				p if {
					__local2__ = [2] with input as 2
					every __local0__, __local1__ in __local2__ {
						__local1__ = input
					} with input as 2
				}
			`,
		},
		{
			note: "rewrite every: with modifier on body, using every's key+value",
			module: `
				package test
				# import future.keywords.in
				# import future.keywords.every
				p if {
					every x, y in input { true with data.test.q[x][y] as 100 }
				}
			`,
			exp: `
				package test
				p if {
				    __local2__ = input
					every __local0__, __local1__ in __local2__ {
						true with data.test.q[__local0__][__local1__] as 100
					}
				}
			`,
		},
		{
			note: "rewrite closures",
			module: `
				package test
				x = 1
				y = 2
				p if {
					some x, z
					z = 3
					[x | x = 2; y = 2; some z; z = 4]
				}
			`,
			exp: `
				package test
				x = 1
				y = 2
				p if {
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
				p[x] = [y, z] if {
					some x, z
					x = "b"
					z = 4
				}`,
			exp: `
				package test
				x = "a"
				y = 1
				z = 2
				p[__local0__] = __local2__ if {
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

				p if {
					f(input, "bar")
				}

				f(x, y) if {
					x[y]
				}
				`,
			exp: `
				package test

				p = true if {
					__local2__ = input;
					data.test.f(__local2__, "bar")
				}

				f(__local0__, __local1__) = true if {
					__local0__[__local1__]
				}
			`,
		},
		{
			note: "redeclare err",
			module: `
				package test
				p if {
					some x
					some x
				}
			`,
			wantErr: errors.New("test.rego:5: rego_compile_error: var x declared above"),
		},
		{
			note: "redeclare err, some/in",
			module: `
				package test
				p if {
					some x
					some i, x in []
				}
			`,
			wantErr: errors.New("test.rego:5: rego_compile_error: var x declared above"),
		},
		{
			note: "redeclare assigned err",
			module: `
				package test
				p if {
					x := 1
					some x
				}
			`,
			wantErr: errors.New("test.rego:5: rego_compile_error: var x assigned above"),
		},
		{
			note: "redeclare assigned err, some/in",
			module: `
				package test
				p if {
					x := 1
					some i, x in []
				}
			`,
			wantErr: errors.New("test.rego:5: rego_compile_error: var x assigned above"),
		},
		{
			note: "redeclare reference err",
			module: `
				package test
				p if {
					data.q[x]
					some x
				}
			`,
			wantErr: errors.New("test.rego:5: rego_compile_error: var x referenced above"),
		},
		{
			note: "redeclare reference err, some/in",
			module: `
				package test
				p if {
					data.q[x]
					some i, x in []
				}
			`,
			wantErr: errors.New("test.rego:5: rego_compile_error: var x referenced above"),
		},
		{
			note: "declare unused err",
			module: `
				package test
				p if {
					some x
				}
			`,
			wantErr: errors.New("declared var x unused"),
		},
		{
			note: "declare unsafe err",
			module: `
				package test
				p contains x if {
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

			f([a]) if {
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
					t.Fatalf("Expected:\n\n%v\n\nbut got:\n\n%v", tc.wantErr, err)
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
			func(x, y) if {
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
			a.b.c.func(x, y) if {
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
			func(x, y) if {
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
			func(x, y) if {
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
			func(x, _) if {
				input.test == "foo"
				x = 1
			}`,
			expectedErrors: Errors{},
		},
		{
			note: "wildcard argvar that's ignored - issue 5602 regression test",
			module: `package test
			func(x, _) if {
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
			func(x) := x if {
				input.test == "foo"
			}`,
			expectedErrors: Errors{},
		},
		{
			note: "argvar not used in body but in head value comprehension",
			module: `package test
			a := {"foo": 1}
			func(x) := { x: v | v := a[x] } if {
				input.test == "foo"
			}`,
			expectedErrors: Errors{},
		},
		{
			note: "argvar not used in body but in else-head value comprehension",
			module: `package test
			a := {"foo": 1}
			func(x) if {
				input.test == "foo"
			} else := { x: v | v := a[x] } if {
				input.test == "bar"
			}`,
			expectedErrors: Errors{},
		},
		{
			note: "argvar not used in body and shadowed in head value comprehension",
			module: `package test
			a := {"foo": 1}
			func(x) := { x: v | x := "foo"; v := a[x] } if {
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
			func(x) if {
				input.test == x
			} else := false if {
				input.test == "foo"
			}`,
			expectedErrors: Errors{},
		},
		{
			note: "argvar used in primary body but not in else body (with wildcard)",
			module: `package test
			func(x, _) if {
				input.test == x
			} else := false if {
				input.test == "foo"
			}`,
			expectedErrors: Errors{},
		},
		{
			note: "argvar not used in primary body but in else body",
			module: `package test
			func(x) if {
				input.test == "foo"
			} else := false if {
				input.test == x
			}`,
			expectedErrors: Errors{},
		},
		{
			note: "argvar not used in primary body but in else body (with wildcard)",
			module: `package test
			func(x, _) if {
				input.test == "foo"
			} else := false if {
				input.test == x
			}`,
			expectedErrors: Errors{},
		},
		{
			note: "argvar used in primary body but not in implicit else body",
			module: `package test
			func(x) if {
				input.test == x
			} else := false`,
			expectedErrors: Errors{},
		},
		{
			note: "argvars usage spread over multiple bodies",
			module: `package test
			func(x, y, z) if {
				input.test == x
			} else if {
				input.test == y
			} else if {
				input.test == z
			}`,
			expectedErrors: Errors{},
		},
		{
			note: "argvars usage spread over multiple bodies, missing in first",
			module: `package test
			func(x, y, z) if {
				input.test == "foo"
			} else if {
				input.test == y
			} else if {
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
			func(x, y, z) if {
				input.test == x
			} else if {
				input.test == "bar"
			} else if {
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
			func(x, y, z) if {
				input.test == x
			} else if {
				input.test == y
			} else if {
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
				"test": module(tc.module),
			}
			compileStages(compiler, "")

			assertErrors(t, compiler.Errors, tc.expectedErrors, true)
		})
	}
}

func TestCompileUnusedAssignedVarsErrorLocations(t *testing.T) {
	tests := []strictnessTestCase{
		{
			note: "one of the two function args is not used - issue 5662 regression test",
			module: `package test
			func(x, y) if {
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
			allow if {
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
				"test": module(tc.module),
			}
			compileStages(compiler, "")
			assertErrors(t, compiler.Errors, tc.expectedErrors, true)
		})
	}

}

func TestCompileUnusedDeclaredVarsErrorLocations(t *testing.T) {
	tests := []strictnessTestCase{
		{
			note: "simple unused some var - issue 4238 regression test",
			module: `package test

			foo if {
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

			foo if {
				print("Hello world")
				some i
			}

			bar if {
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
			foo2 if {
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
				"test": module(tc.module),
			}
			compileStages(compiler, "")

			assertErrors(t, compiler.Errors, tc.expectedErrors, true)
		})
	}
}

func TestCompileInvalidEqAssignExpr(t *testing.T) {
	tests := []struct {
		note        string
		regoVersion RegoVersion
	}{
		{
			note:        "v0",
			regoVersion: RegoV0,
		},
		{
			note:        "v1",
			regoVersion: RegoV1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			c := NewCompiler()

			c.Modules["error"] = MustParseModuleWithOpts(`package errors

				p if {
					# Arity mismatches are caught in the checkUndefinedFuncs check,
					# and invalid eq/assign calls are passed along until then.
					assign()
					assign(1)
					eq()
					eq(1)
				}`, ParserOptions{RegoVersion: tc.regoVersion, AllFutureKeywords: true})

			// Run up to CheckRuleConflicts (the stage before CheckUndefinedFuncs)
			compileStages(c, StageCheckRuleConflicts)
			assertNotFailed(t, c)
		})
	}
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

	compileStages(c, StageRewriteComprehensionTerms)
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
			input: "p if { x = 1; x == 1; y = [1,2,3]; y == [1,2,3] }",
			exp:   `x = 1; x = 1; y = [1,2,3]; y = [1,2,3]`,
		},
		{
			note:  "refs",
			input: "p if { input.x == data.y }",
			exp:   `input.x = data.y`,
		},
		{
			note:  "comprehensions",
			input: "p if { [1|true] == [2|true] }",
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
			input: "p if { count([1,2]) == 2 }",
			exp:   `count([1,2], __local0__); __local0__ = 2`,
		},
		{
			note:  "embedded",
			input: "p if { x = 1; y = [x == 0] }",
			exp:   `x = 1; equal(x, 0, __local0__); y = [__local0__]`,
		},
		{
			note:  "embedded in call",
			input: `p if { x = 0; neq(true, x == 1) }`,
			exp:   `x = 0; equal(x, 1, __local0__); neq(true, __local0__)`,
		},
		{
			note:  "comprehension in object key",
			input: `p if { {{1 | 0 == 0}: 2} }`,
			exp:   `{{1 | 0 = 0}: 2}`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			c := NewCompiler()
			c.Modules["test"] = module("package test\n" + tc.input)
			compileStages(c, StageRewriteEquals)
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
		{`arr if { [str] }`, `__local0__ = data.test.str; [__local0__]`},
		{`arr2 if { [[str]] }`, `__local0__ = data.test.str; [[__local0__]]`},
		{`obj if { {"x": str} }`, `__local0__ = data.test.str; {"x": __local0__}`},
		{`obj2 if { {"x": {"y": str}} }`, `__local0__ = data.test.str; {"x": {"y": __local0__}}`},
		{`set if { {str} }`, `__local0__ = data.test.str; {__local0__}`},
		{`set2 if { {{str}} }`, `__local0__ = data.test.str; {{__local0__}}`},
		{`ref if { str[str] }`, `__local0__ = data.test.str; data.test.str[__local0__]`},
		{`ref2 if { str[str[str]] }`, `__local0__ = data.test.str; __local1__ = data.test.str[__local0__]; data.test.str[__local1__]`},
		{`arr_compr if { [1 | [str]] }`, `[1 | __local0__ = data.test.str; [__local0__]]`},
		{`arr_compr2 if { [1 | [1 | [str]]] }`, `[1 | [1 | __local0__ = data.test.str; [__local0__]]]`},
		{`set_compr if { {1 | [str]} }`, `{1 | __local0__ = data.test.str; [__local0__]}`},
		{`set_compr2 if { {1 | {1 | [str]}} }`, `{1 | {1 | __local0__ = data.test.str; [__local0__]}}`},
		{`obj_compr if { {"a": "b" | [str]} }`, `{"a": "b" | __local0__ = data.test.str; [__local0__]}`},
		{`obj_compr2 if { {"a": "b" | {"a": "b" | [str]}} }`, `{"a": "b" | {"a": "b" | __local0__ = data.test.str; [__local0__]}}`},
		{`equality if { str = str }`, `data.test.str = data.test.str`},
		{`equality2 if { [str] = [str] }`, `__local0__ = data.test.str; __local1__ = data.test.str; [__local0__] = [__local1__]`},
		{`call if { startswith(str, "") }`, `__local0__ = data.test.str; startswith(__local0__, "")`},
		{`call2 if { count([str], n) }`, `__local0__ = data.test.str; count([__local0__], n)`},
		{`eq_with if { [str] = [1] with input as 1 }`, `__local0__ = data.test.str with input as 1; [__local0__] = [1] with input as 1`},
		{`term_with if { [[str]] with input as 1 }`, `__local0__ = data.test.str with input as 1; [[__local0__]] with input as 1`},
		{`call_with if { count(str) with input as 1 }`, `__local0__ = data.test.str with input as 1; count(__local0__) with input as 1`},
		{`call_func if { f(input, "foo") } f(x,y) if { x[y] }`, `__local2__ = input; data.test.f(__local2__, "foo")`},
		{`call_func2 if { f(input.foo, "foo") } f(x,y) if { x[y] }`, `__local2__ = input.foo; data.test.f(__local2__, "foo")`},
		{`every_domain if { every _ in str { true } }`, `__local1__ = data.test.str; every __local0__, _ in __local1__ { true }`},
		{`every_domain_array if { every _ in [1, 2, 3] { true } }`, `__local1__ = [1, 2, 3]; every __local0__, _ in __local1__ { true }`},
		{`every_domain_call if { every _ in numbers.range(1, 10) { true } }`, `numbers.range(1, 10, __local2__); __local1__ = __local2__; every __local0__, _ in __local1__ { true }`},
		{`every_domain_array_w_calls if { every _ in [1 / 2, "foo", abs(-1)] { true } }`, `div(1, 2, __local2__); abs(-1, __local3__); __local1__ = [__local2__, "foo", __local3__]; every __local0__, _ in __local1__ { true }`},
		{`every_body if { every _ in [] { [str] } }`,
			`__local1__ = []; every __local0__, _ in __local1__ { __local2__ = data.test.str; [__local2__] }`},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			c := NewCompiler()
			opts := ParserOptions{AllFutureKeywords: true, unreleasedKeywords: true}
			c.Modules["test"] = module(fixture + tc.input)
			compileStages(c, StageRewriteDynamicTerms)
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
			input:    `p if { true with input as 1 }`,
			expected: `p if { true with input as 1 }`,
		},
		{
			note:     "refs",
			input:    `p if { true with input as arr }`,
			expected: `p if { __local0__ = data.test.arr; true with input as __local0__ }`,
		},
		{
			note:     "array comprehension",
			input:    `p if { true with input as [true | true] }`,
			expected: `p if { __local0__ = [true | true]; true with input as __local0__ }`,
		},
		{
			note:     "set comprehension",
			input:    `p if { true with input as {true | true} }`,
			expected: `p if { __local0__ = {true | true}; true with input as __local0__ }`,
		},
		{
			note:     "object comprehension",
			input:    `p if { true with input as {"k": true | true} }`,
			expected: `p if { __local0__ = {"k": true | true}; true with input as __local0__ }`,
		},
		{
			note:     "comprehension nested",
			input:    `p if { true with input as [true | true with input as arr] }`,
			expected: `p if { __local0__ = [true | __local1__ = data.test.arr; true with input as __local1__]; true with input as __local0__ }`,
		},
		{
			note:     "multiple",
			input:    `p if { true with input.a as arr[0] with input.b as arr[1] }`,
			expected: `p if { __local0__ = data.test.arr[0]; __local1__ = data.test.arr[1]; true with input.a as __local0__ with input.b as __local1__ }`,
		},
		{
			note:    "invalid target",
			input:   `p if { true with foo.q as 1 }`,
			wantErr: errors.New("rego_type_error: with keyword target must reference existing input, data, or a function"),
		},
		{
			note:     "built-in function: replaced by (unknown) var",
			input:    `p if { true with time.now_ns as foo }`,
			expected: `p if { true with time.now_ns as foo }`, // `foo` still a Var here
		},
		{
			note: "built-in function: valid, arity 0",
			input: `
				p if { true with time.now_ns as now }
				now() = 1
			`,
			expected: `p if { true with time.now_ns as data.test.now }`,
		},
		{
			note: "built-in function: valid func ref, arity 1",
			input: `
				p if { true with http.send as mock_http_send }
				mock_http_send(_) = { "body": "yay" }
			`,
			expected: `p if { true with http.send as data.test.mock_http_send }`,
		},
		{
			note: "built-in function: replaced by value",
			input: `
				p if { true with http.send as { "body": "yay" } }
			`,
			expected: `p if { true with http.send as {"body": "yay"} }`,
		},
		{
			note: "built-in function: replaced by var",
			input: `
				p if {
					resp := { "body": "yay" }
					true with http.send as resp
				}
			`,
			expected: `p if { __local0__ = {"body": "yay"}; true with http.send as __local0__ }`,
		},
		{
			note: "non-built-in function: replaced by var",
			input: `
				p if {
					resp := true
					f(true) with f as resp
				}
				f(false) if { true }
			`,
			expected: `p if { __local0__ = true; data.test.f(true) with data.test.f as __local0__ }`,
		},
		{
			note: "built-in function: replaced by comprehension",
			input: `
				p if { true with http.send as { x: true | x := ["a", "b"][_] } }
			`,
			expected: `p if { __local2__ = {__local0__: true | __local1__ = ["a", "b"]; __local0__ = __local1__[_]}; true with http.send as __local2__ }`,
		},
		{
			note: "built-in function: replaced by ref",
			input: `
				p if { true with http.send as resp }
				resp := { "body": "yay" }
			`,
			expected: `p if { true with http.send as data.test.resp }`,
		},
		{
			note: "built-in function: replaced by another built-in (ref)",
			input: `
				p if { true with http.send as object.union_n }
			`,
			expected: `p if { true with http.send as object.union_n }`,
		},
		{
			note: "built-in function: replaced by another built-in (simple)",
			input: `
				p if { true with http.send as count }
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
				p if { q with is_object as http.send }
			`,
			opts:    func(c *Compiler) *Compiler { return c.WithUnsafeBuiltins(map[string]struct{}{"http.send": {}}) },
			wantErr: errors.New("rego_compile_error: with keyword replacing built-in function: target must not be unsafe: \"http.send\""),
		},
		{
			note: "non-built-in function: replaced by another built-in that's marked unsafe",
			input: `
			r(_) = {}
			q := r({"url": "https://httpbin.org", "method": "GET"})
			p if {
				q with r as http.send
			}`,
			opts:    func(c *Compiler) *Compiler { return c.WithUnsafeBuiltins(map[string]struct{}{"http.send": {}}) },
			wantErr: errors.New("rego_compile_error: with keyword replacing built-in function: target must not be unsafe: \"http.send\""),
		},
		{
			note: "built-in function: valid, arity 1, non-compound name",
			input: `
				p if { concat("/", input) with concat as mock_concat }
				mock_concat(_, _) = "foo/bar"
			`,
			expectedRule: func() *Rule {
				r := MustParseRuleWithOpts(`p if { concat("/", input) with concat as data.test.mock_concat }`,
					ParserOptions{RegoVersion: RegoV1})
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
			c.Modules["test"] = module(fixture + tc.input)
			compileStages(c, StageRewriteWithValues)
			if tc.wantErr == nil {
				assertNotFailed(t, c)
				expected := tc.expectedRule
				if expected == nil {
					expected = MustParseRuleWithOpts(tc.expected, ParserOptions{RegoVersion: RegoV1})
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
			p if { true }`,
			exp: `package test
			p if { true }`,
		},
		{
			note: "replace empty body with true",
			module: `package test

			p if { print(1) }
			`,
			exp: `package test

			p if { true } `,
		},
		{
			note: "rule body",
			module: `package test

			p if { false; print(1) }
			`,
			exp: `package test

			p if { false } `,
		},
		{
			note: "set comprehension body",
			module: `package test

			p if { {1 | false; print(1)} }
			`,
			exp: `package test

			p if { {1 | false} } `,
		},
		{
			note: "array comprehension body",
			module: `package test

			p if { [1 | false; print(1)] }
			`,
			exp: `package test

			p if { [1 | false] } `,
		},
		{
			note: "object comprehension body",
			module: `package test

			p if { {"x": 1 | false; print(1)} }
			`,
			exp: `package test

			p if { {"x": 1 | false} } `,
		},
		{
			note: "every body",
			module: `package test

			p if { every _ in [] { false; print(1) } }
			`,
			exp: `package test

			p = true if { __local1__ = []; every __local0__, _ in __local1__ { false } }`,
		},
		{
			note: "in head",
			module: `package test

			p = {1 | print("x")}`,
			exp: `package test

			p = __local0__ if { __local0__ = {1 | true} }`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			c := NewCompiler().WithEnablePrintStatements(false)
			c.Compile(map[string]*Module{"test.rego": module(tc.module)})
			assertNotFailed(t, c)

			if exp := module(tc.exp); !exp.Equal(c.Modules["test.rego"]) {
				t.Fatalf("Expected:\n\n%v\n\nGot:\n\n%v", exp, c.Modules["test.rego"])
			}
		})
	}
}

func TestCompilerRewritePrintCallsErrors(t *testing.T) {
	cases := []struct {
		note    string
		module  string
		exp     error
		errCode string
	}{
		{
			note: "non-existent var",
			module: `package test

			p if { print(x) }`,
			exp:     errors.New("var x is undeclared"),
			errCode: CompileErr,
		},
		{
			note: "declared after print",
			module: `package test

			p if { print(x); x = 7 }`,
			exp:     errors.New("var x is undeclared"),
			errCode: CompileErr,
		},
		{
			note: "inside comprehension",
			module: `package test
			p if { {1 | print(x)} = {1 | print(7)} }
			`,
			exp:     errors.New("var x is undeclared"),
			errCode: CompileErr,
		},
		{
			note: "inside template-string",
			module: `package test
			p if { $"<{print(42)}>" }
			`,
			exp:     errors.New("print(42) used as value"),
			errCode: TypeErr,
		},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			c := NewCompiler().WithEnablePrintStatements(true)
			c.Compile(map[string]*Module{
				"test.rego": module(tc.module),
			})
			if !c.Failed() {
				t.Fatal("expected error")
			}
			if c.Errors[0].Code != tc.errCode || c.Errors[0].Message != tc.exp.Error() {
				t.Fatal("unexpected error:", c.Errors)
			}
		})
	}
}

// Regression test for bug #7647
// head values in nested comprehensions should not lead to undeclared error.
func TestCompilterRewritePrintCallsNestedComprehensionLocalsSafe(t *testing.T) {
	cases := []struct {
		note   string
		module string
	}{
		{
			note: "print variable from nested comprehension without error",
			module: `package test
		    f(_) := {"a": [1, 2, 3], "b": [4, 5, 6], "c": [7, 8, 9]}

		    p := [v |
			m := {l | l := f(true)[k]}[_]
			v := m[_]
			print(v)
			]`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			c := NewCompiler().WithEnablePrintStatements(true)
			c.Compile(map[string]*Module{"test.rego": module(tc.module)})
			assertNotFailed(t, c)
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

			p if { print(1) }`,
			exp: `package test

			p = true if { __local1__ = {__local0__ | __local0__ = 1}; internal.print([__local1__]) }`,
		},
		{
			note: "print multiple",
			module: `package test

			p if { print(1, 2) }`,
			exp: `package test

			p = true if { __local2__ = {__local0__ | __local0__ = 1}; __local3__ = {__local1__ | __local1__ = 2}; internal.print([__local2__, __local3__]) }`,
		},
		{
			note: "print inside set comprehension",
			module: `package test

			p if { x = 1; {2 | print(x)} }`,
			exp: `package test

			p = true if { x = 1; {2 | __local1__ = {__local0__ | __local0__ = x}; internal.print([__local1__])} }`,
		},
		{
			note: "print inside array comprehension",
			module: `package test

			p if { x = 1; [2 | print(x)] }`,
			exp: `package test

			p = true if { x = 1; [2 | __local1__ = {__local0__ | __local0__ = x}; internal.print([__local1__])] }`,
		},
		{
			note: "print inside object comprehension",
			module: `package test

			p if { x = 1; {"x": 2 | print(x)} }`,
			exp: `package test

			p = true if { x = 1; {"x": 2 | __local1__ = {__local0__ | __local0__ = x}; internal.print([__local1__])} }`,
		},
		{
			note: "print inside every",
			module: `package test

			p if { every x in [1,2] { print(x) } }`,
			exp: `package test

			p = true if {
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

			p if {
				x := split("abc", "")[y]
				print(x, y)
			}`,
			exp: `package test

			p = true if { split("abc", "", __local3__); __local0__ = __local3__[y]; __local4__ = {__local1__ | __local1__ = __local0__}; __local5__ = {__local2__ | __local2__ = y}; internal.print([__local4__, __local5__]) }`,
		},
		{
			note: "print call in head",
			module: `package test

			p = {1 | print("x") }`,
			exp: `package test

			p = __local1__ if {
				__local1__ = {1 | __local2__ = { __local0__ | __local0__ = "x"}; internal.print([__local2__])}
			}`,
		},
		{
			note: "print call in head - args treated as safe",
			module: `package test

			f(a) = {1 | a[x]; print(x)}`,
			exp: `package test

			f(__local0__) = __local2__ if { __local2__ = {1 | __local0__[x]; __local3__ = {__local1__ | __local1__ = x}; internal.print([__local3__])} }
			`,
		},
		{
			note: "print call of var in head key",
			module: `package test
			f(_) = [1, 2, 3]
			p contains x if { [_, x, _] := f(true); print(x) }`,
			exp: `package test
			f(__local0__) = [1, 2, 3] if { true }
			p contains __local2__ if { data.test.f(true, __local5__); [__local1__, __local2__, __local3__] = __local5__; __local6__ = {__local4__ | __local4__ = __local2__}; internal.print([__local6__]) }
			`,
		},
		{
			note: "print call of var in head value",
			module: `package test
			f(_) = [1, 2, 3]
			p = x if { [_, x, _] := f(true); print(x) }`,
			exp: `package test
			f(__local0__) = [1, 2, 3] if { true }
			p = __local2__ if { data.test.f(true, __local5__); [__local1__, __local2__, __local3__] = __local5__; __local6__ = {__local4__ | __local4__ = __local2__}; internal.print([__local6__]) }
			`,
		},
		{
			note: "print call of vars in head key and value",
			module: `package test
			f(_) = [1, 2, 3]
			p[x] = y if { [_, x, y] := f(true); print(x) }`,
			exp: `package test
			f(__local0__) = [1, 2, 3] if { true }
			p[__local2__] = __local3__ if { data.test.f(true, __local5__); [__local1__, __local2__, __local3__] = __local5__; __local6__ = {__local4__ | __local4__ = __local2__}; internal.print([__local6__]) }
			`,
		},
		{
			note: "print call of vars altered with 'with' and call",
			module: `package test
			q = input
			p if {
				x := q with input as json.unmarshal("{}")
				print(x)
			}`,
			exp: `package test
			q = __local3__ if { __local3__ = input }
			p = true if {
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
			c.Compile(map[string]*Module{"test.rego": module(tc.module)})
			assertNotFailed(t, c)

			if exp := module(tc.exp); !exp.Equal(c.Modules["test.rego"]) {
				t.Fatalf("Expected:\n\n%v\n\nGot:\n\n%v", exp, c.Modules["test.rego"])
			}
		})
	}
}

func TestCompilerRewriteTemplateStrings(t *testing.T) {
	t.Parallel()

	type rewriteTest struct {
		note   string
		module string
		exp    string
	}

	cases := func(rewriteCases []rewriteTest) func(t *testing.T) {
		return func(t *testing.T) {
			t.Parallel()
			t.Helper()
			for _, tc := range rewriteCases {
				t.Run(tc.note, func(t *testing.T) {
					t.Parallel()
					t.Helper()
					c := MustCompileModules(map[string]string{"test.rego": tc.module})
					if exp, act := module(tc.exp), c.Modules["test.rego"]; !exp.Equal(act) {
						t.Fatalf("Expected:\n\n%v\n\nGot:\n\n%v", exp, act)
					}
				})
			}
		}
	}

	t.Run("empty template string", cases([]rewriteTest{{
		note: "empty template string, head value",
		module: `package test
			p := $""`,
		exp: `package test
			p := __local0__ if { 
				internal.template_string([""], __local0__) 
			}`,
	}, {
		note: "empty template string, head set value",
		module: `package test
			p contains $""`,
		exp: `package test
			p contains __local0__ if { 
				internal.template_string([""], __local0__) 
			}`,
	}, {
		note: "empty template string, head map key",
		module: `package test
			p[$""] := true`,
		exp: `package test
			p[__local0__] := true if {
				internal.template_string([""], __local1__)
				__local0__ = __local1__
			}`,
	}, {
		note: "empty template string, in body",
		module: `package test
			p := x if {
				x := $""
			}`,
		exp: `package test
			p := __local0__ if { 
				internal.template_string([""], __local1__)
				__local0__ = __local1__
			}`,
	}, {
		note: "empty template string, in body, no output arg",
		module: `package test
			p if {
				$""
			}`,
		exp: `package test
			p = true if {
				internal.template_string([""], __local0__)
				__local0__
			}`,
	},
	}))

	t.Run("no template expressions", cases([]rewriteTest{{
		note: "non-empty template string, no template expression, head value",
		module: `package test
			p := $"foo bar"`,
		exp: `package test
			p := __local0__ if { 
				internal.template_string(["foo bar"], __local0__) 
			}`,
	}, {
		note: "non-empty template string, no template expression, head set value",
		module: `package test
			p contains $"foo bar"`,
		exp: `package test
			p contains __local0__ if { 
				internal.template_string(["foo bar"], __local0__) 
			}`,
	}, {
		note: "non-empty template string, no template expression, head map key",
		module: `package test
			p[$"foo bar"] := true`,
		exp: `package test
			p[__local0__] := true if { 
				internal.template_string(["foo bar"], __local1__)
				__local0__ = __local1__
			}`,
	}, {
		note: "non-empty template string, no template expression, in body",
		module: `package test
			p := x if {
				x := $"foo bar"
			}`,
		exp: `package test
			p := __local0__ if { 
				internal.template_string(["foo bar"], __local1__)
				__local0__ = __local1__
			}`,
	}, {
		note: "non-empty template string, no template expression, in body, no output arg",
		module: `package test
			p if {
				$"foo bar"
			}`,
		exp: `package test
			p = true if {
				internal.template_string(["foo bar"], __local0__)
				__local0__
			}`,
	}}))

	t.Run("ref template expression", cases([]rewriteTest{{
		note: "single template expression, ref, head value",
		module: `package test
			p := $"{input.x}"`,
		exp: `package test
			p := __local1__ if { 
				__local2__ = {__local0__ | __local0__ = input.x}; internal.template_string([__local2__], __local1__)
			}`,
	}, {
		note: "single template expression, ref, head set value",
		module: `package test
			p contains $"{input.x}"`,
		exp: `package test
			p contains __local1__ if { 
				__local2__ = {__local0__ | __local0__ = input.x}
				internal.template_string([__local2__], __local1__)
			}`,
	}, {
		note: "single template expression, ref, head map key",
		module: `package test
			p[$"{input.x}"] := true`,
		exp: `package test
			p[__local0__] := true if { 
				__local3__ = {__local1__ | __local1__ = input.x}
				internal.template_string([__local3__], __local2__)
				__local0__ = __local2__
			}`,
	}, {
		note: "single template expression, ref, in body",
		module: `package test
			p := x if {
				x := $"{input.x}"
			}`,
		exp: `package test
			p := __local0__ if { 
				__local3__ = {__local1__ | __local1__ = input.x}
				internal.template_string([__local3__], __local2__)
				__local0__ = __local2__
			}`,
	}, {
		note: "single template expression, ref, in body, no output arg",
		module: `package test
			p if {
				$"{input.x}"
			}`,
		exp: `package test
			p = true if { 
				__local2__ = {__local0__ | 
					__local0__ = input.x
				}
				internal.template_string([__local2__], __local1__)
				__local1__
			}`,
	}, {
		note: "single template expression, ref, in function arg",
		module: `package test
			f($"{input.x}") := 42`,
		exp: `package test
			f(__local1__) := 42 if { 
				__local2__ = {__local0__ | __local0__ = input.x}
				internal.template_string([__local2__], __local1__)
			}`,
	}}))

	t.Run("var template expression", cases([]rewriteTest{{
		note: "single template expression, var, head value",
		module: `package test
			p := $"{x}" if {
				x := 42
			}`,
		exp: `package test
			p := __local1__ if { 
				__local0__ = 42
				internal.template_string([{__local0__}], __local1__)
			}`,
	}, {
		note: "single template expression, var, indirection, head value",
		module: `package test
			p := $"{x}" if {
				x := input.x
			}`,
		exp: `package test
			p := __local1__ if { 
				__local0__ = input.x
				internal.template_string([{__local0__}], __local1__)
			}`,
	}, {
		note: "single template expression, var, head set value",
		module: `package test
			p contains $"{x}" if {
				x := 42
			}`,
		exp: `package test
			p contains __local1__ if { 
				__local0__ = 42
				internal.template_string([{__local0__}], __local1__)
			}`,
	}, {
		note: "single template expression, var, head map key",
		module: `package test
			p[$"{x}"] := true if {
				x := 42
			}`,
		exp: `package test
			p[__local0__] := true if { 
				__local1__ = 42
				internal.template_string([{__local1__}], __local2__)
				__local0__ = __local2__
			}`,
	}, {
		note: "single template expression, var, in body",
		module: `package test
			p := y if {
				x := 42
				y := $"{x}"
			}`,
		exp: `package test
			p := __local1__ if { 
				__local0__ = 42
				internal.template_string([{__local0__}], __local2__)
				__local1__ = __local2__
			}`,
	}, {
		note: "single template expression, var, in body, no output arg",
		module: `package test
			p if {
				x := 42
				$"{x}"
			}`,
		exp: `package test
			p = true if { 
				__local0__ = 42
				internal.template_string([{__local0__}], __local1__)
				__local1__
			}`,
	}, {
		note: "single template expression, var from function args, in head args",
		module: `package test
			f($"{x}") := 42 if {
				x := "foo"
			}`,
		exp: `package test
			f(__local1__) := 42 if { 
				__local0__ = "foo"
				internal.template_string([{__local0__}], __local1__)
			}`,
	}, {
		note: "single template expression, var from function args, in head value",
		module: `package test
				f(x) := $"{x}"`,
		exp: `package test
				f(__local0__) := __local1__ if { 
					internal.template_string([{__local0__}], __local1__)
				}`,
	}, {
		note: "single template expression, var from function args, in body",
		module: `package test
			f(x) := y if {
				y := $"{x}"
			}`,
		exp: `package test
			f(__local0__) := __local1__ if {
				internal.template_string([{__local0__}], __local2__)
				__local1__ = __local2__
			}`,
	}}))

	t.Run("primitives", cases([]rewriteTest{{
		note: "primitives",
		module: `package test
			p := $"{false}, {42}, {13.37}, {"foo"}, {` + "`bar`" + `}, {null}"`,
		exp: `package test
			p := __local0__ if { 
				internal.template_string([false, ", ", 42, ", ", 13.37, ", ", "foo", ", ", "bar", ", ", null], __local0__) 
			}`,
	}}))

	t.Run("collections", cases([]rewriteTest{{
		note: "collections",
		module: `package test
			p := $"{[1, 2, 3]}, {{false, true}}, {{"a": "b"}}"`,
		exp: `package test
			p := __local3__ if { 
				__local4__ = {__local0__ | __local0__ = [1, 2, 3]}
				__local5__ = {__local1__ | __local1__ = {false, true}}
				__local6__ = {__local2__ | __local2__ = {"a": "b"}}
				internal.template_string([__local4__, ", ", __local5__, ", ", __local6__], __local3__) 
			}`,
	}}))

	t.Run("call template-expression", cases([]rewriteTest{{
		note: "single template expression, call, head value",
		module: `package test
			f(x) := x
			p := $"{f(input.x)}"`,
		exp: `package test
			f(__local0__) := __local0__ if { true }
			p := __local3__ if { 
				__local5__ = {__local1__ | 
					__local4__ = input.x
					data.test.f(__local4__, __local2__)
					__local1__ = __local2__
				}
				internal.template_string([__local5__], __local3__)
			}`,
	}, {
		note: "single template expression, call, head set value",
		module: `package test
			f(x) := x
			p contains $"{f(input.x)}"`,
		exp: `package test
			f(__local0__) := __local0__ if { true }
			p contains __local3__ if { 
				__local5__ = {__local1__ | 
					__local4__ = input.x
					data.test.f(__local4__, __local2__)
					__local1__ = __local2__
				}
				internal.template_string([__local5__], __local3__)
			}`,
	}, {
		note: "single template expression, call, head map key",
		module: `package test
			f(x) := x
			p[$"{f(input.x)}"] := true`,
		exp: `package test
			f(__local1__) := __local1__ if { true }
			p[__local0__] := true if { 
				__local6__ = {__local2__ | 
					__local5__ = input.x
					data.test.f(__local5__, __local3__)
					__local2__ = __local3__
				}
				internal.template_string([__local6__], __local4__)
				__local0__ = __local4__
			}`,
	}, {
		note: "single template expression, call, in body",
		module: `package test
			f(x) := x
			p := x if {
				x := $"{f(input.x)}"
			}`,
		exp: `package test
			f(__local0__) := __local0__ if { true }
			p := __local1__ if { 
				__local6__ = {__local2__ | 
					__local5__ = input.x
					data.test.f(__local5__, __local3__)
					__local2__ = __local3__
				}
				internal.template_string([__local6__], __local4__)
				__local1__ = __local4__
			}`,
	}}))

	t.Run("call infix template-expression", cases([]rewriteTest{{
		note: "single template expression, infix, head value",
		module: `package test
			p := $"{input.x + 2}"`,
		exp: `package test
			p := __local2__ if { 
				__local4__ = {__local0__ | 
					__local3__ = input.x
					plus(__local3__, 2, __local1__)
					__local0__ = __local1__
				}
				internal.template_string([__local4__], __local2__)
			}`,
	}, {
		note: "single template expression, infix, head set value",
		module: `package test
				p contains $"{input.x + 2}"`,
		exp: `package test
			p contains __local2__ if { 
				__local4__ = {__local0__ | 
					__local3__ = input.x
					plus(__local3__, 2, __local1__)
					__local0__ = __local1__
				}
				internal.template_string([__local4__], __local2__)
			}`,
	}, {
		note: "single template expression, infix, head map key",
		module: `package test
			p[$"{input.x + 2}"] := true`,
		exp: `package test
			p[__local0__] := true if {
				__local5__ = {__local1__ | 
					__local4__ = input.x
					plus(__local4__, 2, __local2__)
					__local1__ = __local2__
				}
				internal.template_string([__local5__], __local3__)
				__local0__ = __local3__
			}`,
	}, {
		note: "single template expression, infix, in body",
		module: `package test
			p := x if {
				x := $"{input.x + 2}"
			}`,
		exp: `package test
			p := __local0__ if { 
				__local5__ = {__local1__ |
					__local4__ = input.x
					plus(__local4__, 2, __local2__)
					__local1__ = __local2__
				}
				internal.template_string([__local5__], __local3__)
				__local0__ = __local3__
			}`,
	}, {
		note: "single template expression, infix, equal (==), in body",
		module: `package test
			p := x if {
				x := $"{input.x == 2}"
			}`,
		exp: `package test
			p := __local0__ if { 
				__local5__ = {__local1__ | __local4__ = input.x
					equal(__local4__, 2, __local2__)
					__local1__ = __local2__
				}
				internal.template_string([__local5__], __local3__)
				__local0__ = __local3__
			}`,
	}, {
		note: "single template expression, reference to default rule, not wrapped",
		module: `package test
			default x := 42

			p := $"{x}"`,
		exp: `package test
			default x := 42
			p := __local0__ if {
				__local1__ = data.test.x
				internal.template_string([{__local1__}], __local0__) }
			`,
	}, {
		note: "single template expression, no exact reference to default rule, wrapped",
		module: `package test
			default a.b := {"c": 1}
			a.b := {"d": 2} if input.x

			p := $"{a.b.c}"`,
		exp: `package test
			default a.b := {"c": 1}
			a.b := {"d": 2} if { input.x }
			p := __local1__ if {
				__local2__ = {__local0__ | __local0__ = data.test.a.b.c}
				internal.template_string([__local2__], __local1__)
			}`,
	}}))

	t.Run("comprehensions", cases([]rewriteTest{{
		note: "inside array comprehension, body",
		module: `package test
			p if {
				[x | x := $"{input.x}"]
			}`,
		exp: `package test
			p = true if { 
				[__local0__ | 
					__local3__ = {__local1__ | __local1__ = input.x}
					internal.template_string([__local3__], __local2__)
					__local0__ = __local2__
				]
			}`,
	}, {
		note: "inside array comprehension, body, nested",
		module: `package test
			p if {
				a := 1
				[x | 
					b := 2
					x := [y | y := $"{a} {b}"]
				]
			}`,
		exp: `package test
			p = true if {
				__local0__ = 1
				[__local3__ | 
					__local1__ = 2
					__local3__ = [__local2__ |
						internal.template_string([{__local0__}, " ", {__local1__}], __local4__)
						__local2__ = __local4__]
					]
				}`,
	}, {
		note: "inside array comprehension, head",
		module: `package test
			p if {
				[$"{x} {input.y}" | x := input.x]
			}`,
		exp: `package test
			p = true if {
				[__local2__ |
					__local0__ = input.x
					__local3__ = {__local1__ | __local1__ = input.y}
					internal.template_string([{__local0__}, " ", __local3__], __local2__)
				] 
			}`,
	}, {
		note: "inside array comprehension, head, nested",
		module: `package test
			p if {
				a := 1
				[x | 
					b := 2
					x := [$"{a} {b} {c}" | c := 3 ]
				]
			}`,
		exp: `package test
			p = true if {
				__local0__ = 1
				[__local3__ | __local1__ = 2
					 __local3__ = [__local4__ | __local2__ = 3
					 	 internal.template_string([{__local0__}, " ", {__local1__}, " ", {__local2__}], __local4__)
					]
				]
			}`,
	}, {
		note: "inside set comprehension, body",
		module: `package test
			p if {
				{x | x := $"{input.x}"}
			}`,
		exp: `package test
			p = true if { 
				{__local0__ | 
					__local3__ = {__local1__ | __local1__ = input.x}
					internal.template_string([__local3__], __local2__)
					__local0__ = __local2__
				}
			}`,
	}, {
		note: "inside set comprehension, head",
		module: `package test
			p if {
				{$"{x} {input.y}" | x := input.x}
			}`,
		exp: `package test
			p = true if {
				{__local2__ |
					__local0__ = input.x
					__local3__ = {__local1__ | __local1__ = input.y}
					internal.template_string([{__local0__}, " ", __local3__], __local2__)} 
				}`,
	}, {
		note: "inside object comprehension, body",
		module: `package test
			p if {
				{x: y | 
					x := $"{input.x}"
					y := $"{input.y}"
				}
			}`,
		exp: `package test
			p = true if { 
				{__local0__: __local1__ | 
					__local6__ = {__local2__ | 
						__local2__ = input.x
					}
					internal.template_string([__local6__], __local4__)
					__local0__ = __local4__
					__local7__ = {__local3__ | 
						__local3__ = input.y
					}
					internal.template_string([__local7__], __local5__)
					__local1__ = __local5__
				} 
			}`,
	}, {
		note: "inside object comprehension, head",
		module: `package test
			p if {
				{$"{input.x} {y}": $"{x} {input.y}" | 
					x := input.x
					y := input.y
				}
			}`,
		exp: `package test
			p = true if { {__local4__: __local5__ | __local0__ = input.x; __local1__ = input.y; __local6__ = {__local2__ | __local2__ = input.x}; internal.template_string([__local6__, " ", {__local1__}], __local4__); __local7__ = {__local3__ | __local3__ = input.y}; internal.template_string([{__local0__}, " ", __local7__], __local5__)} }`,
	}, {
		note: "single template expression, nested comprehension with local var, in function arg",
		module: `package test
			f($"{[x | y := input.ys[_]; x := y]}") := 42`,
		exp: `package test
			f(__local3__) := 42 if {
				__local4__ = {__local2__ | 
					__local2__ = [__local1__ | __local0__ = input.ys[_]
					__local1__ = __local0__]
				}
				internal.template_string([__local4__], __local3__)
			}`,
	}}))

	t.Run("every", cases([]rewriteTest{{
		note: "inside every expression, body",
		module: `package test
			p if {
				every i, x in input.l1 {
					x == $"<{input.l2[i]}>"
				}
			}`,
		exp: `package test
			p = true if { 
				__local3__ = input.l1
				every __local0__, __local1__ in __local3__ { 
					__local5__ = {__local2__ | 
						__local2__ = input.l2[__local0__]
					}
					internal.template_string(["<", __local5__, ">"], __local4__)
					__local1__ = __local4__
				}
			}`,
	}, {
		note: "inside every expression, domain",
		module: `package test
			p if {
				every _, x in [$"{42} {input.x}"] {
					x == $"42 foo"
				}
			}`,
		exp: `package test
			p = true if { 
				__local5__ = {__local1__ | __local1__ = input.x}
				internal.template_string([42, " ", __local5__], __local3__)
				__local2__ = [__local3__]
				every _, __local0__ in __local2__ {
					internal.template_string(["42 foo"], __local4__)
					__local0__ = __local4__
				}
			}`,
	}}))

	t.Run("some", cases([]rewriteTest{{
		note: "inside some",
		module: `package test
			users := {"alice_1", "alice_2"}
			id := 1
			
			t if {
				$"user_{id}" in users
			}`,
		exp: `package test
			users := {"alice_1", "alice_2"} if { true }
			id := 1 if { true }
			t = true if {
				__local1__ = data.test.id
				internal.template_string(["user_", {__local1__}], __local0__)
				__local2__ = data.test.users
				internal.member_2(__local0__, __local2__)
			}`,
	}, {
		note: "inside some, domain",
		module: `package test
			t if {
				some "user_1" in [$"alice_{1}", $"alice_{2}"]
			}`,
		exp: `package test
			t = true if { 
				internal.template_string(["alice_", 1], __local2__)
				internal.template_string(["alice_", 2], __local3__)
				__local4__ = [__local2__, __local3__]
				"user_1" = __local4__[__local1__]
			}`,
	}, {
		note: "template string in head referencing var from some with template string in domain (issue #8162)",
		module: `package test
			r contains $"{val}" if {
				some val in [1, $"{1 + 1}"]
			}`,
		exp: `package test
			r contains __local4__ if {
				__local8__ = {__local3__ | plus(1, 1, __local5__)
				__local3__ = __local5__}
				internal.template_string([__local8__], __local6__)
				__local7__ = [1, __local6__];
				__local2__ = __local7__[__local1__]
				internal.template_string([{__local2__}], __local4__)
			}`,
	}}))

	t.Run("else", cases([]rewriteTest{{
		note: "in else body",
		module: `package test
			p if {
				false
			} else := msg if {
				msg := $"foo: {input.y}"
			}`,
		exp: `package test
			p = true if { 
				false 
			} else := __local0__ if { 
				__local3__ = {__local1__ | 
					__local1__ = input.y
				}
				internal.template_string(["foo: ", __local3__], __local2__)
				__local0__ = __local2__
			}`,
	}, {
		note: "in else head",
		module: `package test
			p if {
				false
			} else := $"foo: {input.y}"`,
		exp: `package test
			p = true if { 
				false 
			} else := __local1__ if { 
				__local2__ = {__local0__ | 
					__local0__ = input.y
				}
				internal.template_string(["foo: ", __local2__], __local1__) 
			}`,
	}}))

	t.Run("nested template strings", cases([]rewriteTest{{
		note: "body",
		module: `package test
			p := x if {
				x := $"foo {$"bar {data.a}"}"
			}`,
		exp: `package test
			p := __local0__ if { 
				__local6__ = {__local1__ | 
					__local5__ = {__local2__ | 
						__local2__ = data.a
					}
					internal.template_string(["bar ", __local5__], __local3__)
					__local1__ = __local3__
				}
				internal.template_string(["foo ", __local6__], __local4__)
				__local0__ = __local4__
			}`,
	}, {
		note: "head value",
		module: `package test
			p := $"foo {$"bar {data.a}"}"`,
		exp: `package test
			p := __local3__ if { 
				__local5__ = {__local0__ |
					__local4__ = {__local1__ | 
						__local1__ = data.a
					}
					internal.template_string(["bar ", __local4__], __local2__)
					__local0__ = __local2__
				}
				internal.template_string(["foo ", __local5__], __local3__) 
			}`,
	}, {
		note: "head set value",
		module: `package test
			p contains $"foo {$"bar {data.a}"}"`,
		exp: `package test
			p contains __local3__ if { 
				__local5__ = {__local0__ |
					__local4__ = {__local1__ | 
						__local1__ = data.a
					}
					internal.template_string(["bar ", __local4__], __local2__)
					__local0__ = __local2__
				}
				internal.template_string(["foo ", __local5__], __local3__) 
			}`,
	}, {
		note: "head map key",
		module: `package test
			p[$"foo {$"bar {data.a}"}"] := true`,
		exp: `package test
			p[__local0__] := true if { 
				__local6__ = {__local1__ | 
					__local5__ = {__local2__ | 
						__local2__ = data.a
					}
					internal.template_string(["bar ", __local5__], __local3__)
					__local1__ = __local3__
				}
				internal.template_string(["foo ", __local6__], __local4__)
				__local0__ = __local4__
			}`,
	}, {
		note: "inner template in head of comprehension",
		module: `package test
			p := x if {
				x := $"foo {[$"bar {x} {input.y}" | x := input.x]}"
			}`,
		exp: `package test
			p := __local1__ if {
				__local7__ = {__local2__ |
					__local2__ = [__local4__ |
						__local0__ = input.x
						__local6__ = {__local3__ |
							__local3__ = input.y
						}
						internal.template_string(["bar ", {__local0__}, " ", __local6__], __local4__)
					]
				}
				internal.template_string(["foo ", __local7__], __local5__)
				__local1__ = __local5__
			}`,
	}}))

	t.Run("with", cases([]rewriteTest{{
		note: "modifier inside template-expression",
		module: `package test
			a := input
			p := $"{a with input as 42} {a with input as {"x": true}}"`,
		exp: `package test
			a := __local3__ if { __local3__ = input }
			p := __local2__ if { 
				__local4__ = {__local0__ | __local0__ = data.test.a with input as 42}
				__local5__ = {__local1__ | __local1__ = data.test.a with input as {"x": true}}
				internal.template_string([__local4__, " ", __local5__], __local2__)
			}`,
	}, {
		note: "modifier outside string-template",
		module: `package test
			a := input
			b := input
			p if {
				$"{a} {b}" with input as 42
			}`,
		exp: `package test
			a := __local3__ if { __local3__ = input }
			b := __local4__ if { __local4__ = input }
			p = true if { 
				__local5__ = {__local0__ | __local0__ = data.test.a} with input as 42
				__local6__ = {__local1__ | __local1__ = data.test.b} with input as 42
				internal.template_string([__local5__, " ", __local6__], __local2__) with input as 42
				__local2__ with input as 42 
			}`,
	}, {
		note: "modifier inside template-expression and outside string-template",
		module: `package test
			a := input.x + input.y
			p := x if {
				x := $"{a with input.x as 1}" with input.y as 2
			}`,
		exp: `package test
			a := __local2__ if {
				__local4__ = input.x
				__local5__ = input.y
				plus(__local4__, __local5__, __local2__)
			}
			p := __local0__ if {
				__local6__ = {__local1__ | __local1__ = data.test.a with input.x as 1} with input.y as 2
				internal.template_string([__local6__], __local3__) with input.y as 2
				__local0__ = __local3__ with input.y as 2
			}`,
	}}))

	t.Run("other", cases([]rewriteTest{{
		note: "var used in template-expression preceding assignment through unification",
		module: `package test
			p := msg if {
				msg := $"{x}"
				x = 42
			}`,
		exp: `package test
			p := __local0__ if { 
				__local0__ = __local1__
				x = 42; 
				internal.template_string([{x}], __local1__)
			}`,
	}, {
		note: "refs to known defined rules are not wrapped in comprehensions",
		module: `package test
			default a.b := "c"
			pi := 3.14
			multi contains "value"

			result := $"{a.b} {pi} {multi}"`,
		exp: `package test
			default a.b := "c"
			pi := 3.14 if { true }
			multi contains "value" if { true }

			result := __local0__ if { 
				__local1__ = data.test.a.b
				__local2__ = data.test.pi
				__local3__ = data.test.multi
				internal.template_string([{__local1__}, " ", {__local2__}, " ", {__local3__}], __local0__)
			}`,
	}, {
		note: "attribute ref of safe var is still not known to be safe, and gets wrapped",
		module: `package test
			p := msg if {
				x := object.union({"a": 1}, {"b": 2})
				msg := $"{x.c}"
			}`,
		exp: `package test

		p := __local1__ if { 
			object.union({"a": 1}, {"b": 2}, __local3__)
			__local0__ = __local3__
			__local5__ = {__local2__ | __local2__ = __local0__.c}
			internal.template_string([__local5__], __local4__)
			__local1__ = __local4__
		}
		`,
	}}))
}

func TestCompilerRewriteTemplateStringsErrors(t *testing.T) {
	cases := []struct {
		note   string
		module string
		exp    string
	}{
		{
			note: "undeclared var, rule head",
			module: `package test
				p := $"{x}"`,
			exp: "var x is unsafe",
		},
		{
			note: "undeclared var, rule body",
			module: `package test
				p := msg if {
					msg := $"{x}"
				}`,
			exp: "var x is unsafe",
		},
		{
			note: "undeclared var (wildcard)",
			module: `package test
				p := msg if {
					a := ["a", "b"]
					msg := $"{a[_]}"
				}`,
			exp: "var _ is undeclared",
		},
		{
			note: "undeclared var (enum)",
			module: `package test
				p := msg if {
					a := ["a", "b"]
					msg := $"{a[x]}"
				}`,
			exp: "var x is undeclared",
		},
		{
			note: "undeclared var, nested inside template-string",
			module: `package test
				p := $"{$"{x}"}"`,
			exp: "var x is unsafe",
		},
		{
			note: "undeclared var, inside array comprehension body",
			module: `package test
				a := ["a", "b"]
				p := [x | x := $"{a[_]}"]`,
			exp: "var _ is undeclared",
		},
		{
			note: "undeclared var, inside array comprehension head",
			module: `package test
				a := ["a", "b"]
				p := [$"{a[_]}" | x := 42]`,
			exp: "var _ is undeclared",
		},
		{
			note: "undeclared var, inside set comprehension body",
			module: `package test
				a := ["a", "b"]
				p := {x | x := $"{a[_]}"}`,
			exp: "var _ is undeclared",
		},
		{
			note: "undeclared var, inside set comprehension head",
			module: `package test
				a := ["a", "b"]
				p := {$"{a[_]}" | x := 42}`,
			exp: "var _ is undeclared",
		},
		{
			note: "undeclared var, inside object comprehension body",
			module: `package test
				a := ["a", "b"]
				p := {x: y | x := $"{a[_]}"; y := 42}`,
			exp: "var _ is undeclared",
		},
		{
			note: "undeclared var, inside object comprehension key",
			module: `package test
				a := ["a", "b"]
				p := {$"{a[_]}": 42 | x := 42}`,
			exp: "var _ is undeclared",
		},
		{
			note: "undeclared var, inside object comprehension value",
			module: `package test
				a := ["a", "b"]
				p := {42: $"{a[_]}" | x := 42}`,
			exp: "var _ is undeclared",
		},
		{
			note: "undeclared var, inside every domain",
			module: `package test
				p if {
					every x in {"a", $"{x}"} {
						x != "b"
					}
				}`,
			exp: "var x is unsafe",
		},
		{
			note: "undeclared var, inside every body",
			module: `package test
				p if {
					every x in {"a", "b"} {
						x != $"{y}"
					}
				}`,
			exp: "var y is unsafe",
		},
		{
			note: "walk built-in call",
			module: `package test
p := $"{walk(["a", "b"])}"`,
			exp: "illegal call to relation built-in 'walk' that may cause multiple outputs",
		},
		{
			note: "undeclared var, some-in with undeclared collection (issue #8157)",
			module: `package test
				items contains item if {
					some label in labels
					item := $"{label}"
				}`,
			exp: "contains: is unsafe",
		},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			c := NewCompiler().WithEnablePrintStatements(true)
			c.Compile(map[string]*Module{
				"test.rego": module(tc.module),
			})
			if !c.Failed() {
				t.Fatal("expected error, got none")
			}
			if c.Errors[0].Message != tc.exp {
				if strings.HasPrefix(tc.exp, "contains:") {
					if exp := strings.TrimPrefix(tc.exp, "contains:"); !strings.Contains(c.Errors[0].Message, exp) {
						t.Fatalf("expected error containing:\n\n%s\n\ngot:\n\n%s", tc.exp, c.Errors[0].Message)
					}
				} else {
					t.Fatalf("expected error:\n\n%s\n\ngot:\n\n%s", tc.exp, c.Errors[0].Message)
				}
			}
		})
	}
}

func TestRewritePrintCallsWithElseImplicitArgs(t *testing.T) {

	mod := `package test

	f(x, y) if {
		x = y
	}

	else = false if {
		print(x, y)
	}`

	c := NewCompiler().WithEnablePrintStatements(true)
	c.Compile(map[string]*Module{
		"test.rego": module(mod),
	})

	if c.Failed() {
		t.Fatal(c.Errors)
	}

	exp := module(`package test

	f(__local0__, __local1__) = true if { __local0__ = __local1__ }
	else = false if { __local4__ = {__local2__ | __local2__ = __local0__}; __local5__ = {__local3__ | __local3__ = __local1__}; internal.print([__local4__, __local5__]) }
	`)

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
				p if { true with time.now_ns as now }
			`,
		},
		{
			note: "simple valid, simple name",
			module: `package test
				mock_concat(_, _) = "foo/bar"
				p if { concat("/", input) with concat as mock_concat }
			`,
		},
		{
			note: "invalid ref: nonexistant",
			module: `package test
				p if { true with time.now_ns as now }
			`,
			err: "rego_unsafe_var_error: var now is unsafe", // we're running all compiler stages here
		},
		{
			note: "valid ref: not a function, but arity = 0",
			module: `package test
				now = 1
				p if { true with time.now_ns as now }
			`,
		},
		{
			note: "ref: not a function, arity > 0",
			module: `package test
				http_send = { "body": "nope" }
				p if { true with http.send as http_send }
			`,
		},
		{
			note: "invalid ref: arity mismatch",
			module: `package test
				http_send(_, _) = { "body": "nope" }
				p if { true with http.send as http_send }
			`,
			err: "rego_type_error: http.send: arity mismatch\n\thave: (any, any)\n\twant: (request: object[string: any])",
		},
		{
			note: "invalid ref: arity mismatch (in call)",
			module: `package test
				http_send(_, _) = { "body": "nope" }
				p if { http.send({}) with http.send as http_send }
			`,
			err: "rego_type_error: http.send: arity mismatch\n\thave: (any, any)\n\twant: (request: object[string: any])",
		},
		{
			note: "invalid ref: value another built-in with different type",
			module: `package test
				p if { true with http.send as net.lookup_ip_addr }
			`,
			err: "rego_type_error: http.send: arity mismatch\n\thave: (string)\n\twant: (request: object[string: any])",
		},
		{
			note: "ref: value another built-in with compatible type",
			module: `package test
				p if { true with count as object.union_n }
			`,
		},
		{
			note: "valid: package import",
			extra: `package mocks
				http_send(_) = {}
			`,
			module: `package test
				import data.mocks
				p if { true with http.send as mocks.http_send }
			`,
		},
		{
			note: "valid: function import",
			extra: `package mocks
				http_send(_) = {}
			`,
			module: `package test
				import data.mocks.http_send
				p if { true with http.send as http_send }
			`,
		},
		{
			note: "invalid target: relation",
			module: `package test
				my_walk(_, _)
				p if { true with walk as my_walk }
			`,
			err: "rego_compile_error: with keyword replacing built-in function: target must not be a relation",
		},
		{
			note: "invalid target: eq",
			module: `package test
				my_eq(_, _)
				p if { true with eq as my_eq }
			`,
			err: `rego_compile_error: with keyword replacing built-in function: replacement of "eq" invalid`,
		},
		{
			note: "invalid target: rego.metadata.chain",
			module: `package test
				p if { true with rego.metadata.chain as [] }
			`,
			err: `rego_compile_error: with keyword replacing built-in function: replacement of "rego.metadata.chain" invalid`,
		},
		{
			note: "invalid target: rego.metadata.rule",
			module: `package test
				p if { true with rego.metadata.rule as {} }
			`,
			err: `rego_compile_error: with keyword replacing built-in function: replacement of "rego.metadata.rule" invalid`,
		},
		{
			note: "invalid target: internal.print",
			module: `package test
				my_print(_, _)
				p if { true with internal.print as my_print }
			`,
			err: `rego_compile_error: with keyword replacing built-in function: replacement of internal function "internal.print" invalid`,
		},
		{
			note: "mocking custom built-in",
			module: `package test
				mock(_)
				mock_mock(_)
				p if { bar(foo.bar("one")) with bar as mock with foo.bar as mock_mock }
			`,
		},
		{
			note: "non-built-in function replaced value",
			module: `package test
				original(_)
				p if { original(true) with original as 123 }
			`,
		},
		{
			note: "non-built-in function replaced by another, arity 0",
			module: `package test
				original() = 1
				mock() = 2
				p if { original() with original as mock }
			`,
			err: "rego_type_error: undefined function data.test.original", // TODO(sr): file bug -- this doesn't depend on "with" used or not
		},
		{
			note: "non-built-in function replaced by another, arity 1",
			module: `package test
				original(_)
				mock(_)
				p if { original(true) with original as mock }
			`,
		},
		{
			note: "non-built-in function replaced by built-in",
			module: `package test
				original(_)
				p if { original([1]) with original as count }
			`,
		},
		{
			note: "non-built-in function replaced by another, arity mismatch",
			module: `package test
				original(_)
				mock(_, _)
				p if { original([1]) with original as mock }
			`,
			err: "rego_type_error: data.test.original: arity mismatch\n\thave: (any, any)\n\twant: (any)",
		},
		{
			note: "non-built-in function replaced by built-in, arity mismatch",
			module: `package test
				original(_)
				p if { original([1]) with original as concat }
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
				c.Modules["extra"] = module(tc.extra)
			}
			c.Modules["test"] = module(tc.module)

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

	c.Modules["test"] = module(`
	package test
	p = {"a": 1}
	q = x if { p = x with p.a as 2 }
	`)

	compileStages(c, StageRewriteWithValues)
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
				p if {
					_ := 1
				}
			`,
		},
		{
			note: "simple rule",
			module: `package test
				p if {
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
				p = x if {
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
				p if {
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
				p if {
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
				p if {
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
				p if {
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
				p if {
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
				p if {
					some i
					_ := [z | z := [1, 2][i]]
				}
			`,
		},
		{
			note: "rule with nested set comprehension",
			module: `package test
				p if {
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
				p if {
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
				p if {
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
				p if {
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
				p if {
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
				f() if {
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
				f() if {
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
				f() = x if {
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
				p if { every i in [1] { y := 10; i == 1 } }
			`,
			expectedErrors: Errors{
				&Error{Message: "assigned var y unused"},
			},
		},
		{
			note: "general ref in rule head",
			module: `package test
						p[q].r[s] := 1 if {
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
						p[q].r[s] := 1 if {
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
			compiler.Modules = map[string]*Module{
				"test": module(tc.module),
			}
			compileStages(compiler, StageRewriteLocalVars)

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
	c.Modules["elsekw"] = module(`
	package elsekw

	p if {
		false
	} else = q if {
		false
	} else if {
		r
	}

	q = true
	r = true

	s if { t }
	t if { false } else if { true }

	`)
	compileStages(c, StageSetGraph)

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

	p if { q }
	q if { r }
	r if { s }
	s if { q }`

	c := NewCompiler()
	c.Modules = map[string]*Module{
		"mod1": module(mod1),
	}

	compileStages(c, StageSetGraph)
	assertNotFailed(t, c)

	_, ok := c.Graph.Sort()
	if ok {
		t.Fatalf("Expected to find cycle in rule graph")
	}

	elsekw := `package elsekw

	p if {
		false
	} else = q if {
		true
	}

	q if {
		false
	} else if {
		r
	}

	r if { s }

	s if { p }
	`

	c = NewCompiler()
	c.Modules = map[string]*Module{
		"elsekw": module(elsekw),
	}

	compileStages(c, StageSetGraph)
	assertNotFailed(t, c)

	_, ok = c.Graph.Sort()
	if ok {
		t.Fatalf("Expected to find cycle in rule graph")
	}

}

func TestCompilerCheckRecursion(t *testing.T) {
	c := NewCompiler()
	c.Modules = map[string]*Module{
		"newMod1": module(`package rec

s = true if { t }
t = true if { s }
a = true if { b }
b = true if { c }
c = true if { d; e }
d = true if { true }
e = true if { a }`),
		"newMod2": module(`package rec

x = true if { s }`,
		),
		"newMod3": module(`package rec2

import data.rec.x

y = true if { x }`),
		"newMod4": module(`package rec3

p[x] = y if { data.rec4[x][y] = z }`,
		),
		"newMod5": module(`package rec4

import data.rec3.p

q[x] = y if { p[x] = y }`),
		"newMod6": module(`package rec5

acp contains x if { acq[x] }
acq contains x if { a = [true | acp[_]]; a[_] = x }
`,
		),
		"newMod7": module(`package rec6

np[x] = y if { data.a[data.b.c[nq[x]]] = y }
nq[x] = y if { data.d[data.e[x].f[np[y]]] }`,
		),
		"newMod8": module(`package rec7

prefix = true if { data.rec7 }`,
		),
		"newMod9": module(`package rec8

dataref = true if { data }`,
		),
		"newMod10": module(`package rec9

		else_self if { false } else if { else_self }

		elsetop if {
			false
		} else = elsemid if {
			true
		}

		elsemid if {
			false
		} else if {
			elsebottom
		}

		elsebottom if { elsetop }
		`),
		"fnMod1": module(`package f0

		fn(x) = y if {
			fn(x, y)
		}`),
		"fnMod2": module(`package f1

		foo(x) = y if {
			bar("buz", x, y)
		}

		bar(x, y) = z if {
			foo([x, y], z)
		}`),
		"fnMod3": module(`package f2

		foo(x) = y if {
			bar("buz", x, y)
		}

		bar(x, y) = z if {
			x = p[y]
			z = x
		}

		p[x] = y if {
			x = "foo.bar"
			foo(x, y)
		}`),
		"everyMod": module(`package everymod
		import future.keywords.every
		everyp if {
			every x in [true, false] { x; everyp }
		}
		everyq contains 1 if {
			every x in everyq { x == 1 }
		}`),
	}

	compileStages(c, StageCheckRecursion)

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
			mod: module(`
package recursion
pkg = "recursion"
foo contains x if {
	data[pkg]["foo"][x]
}
`),
			err: "rego_recursion_error: rule data.recursion.foo is recursive: data.recursion.foo -> data.recursion.foo",
		},
		{note: "system.main",
			mod: module(`
package system.main
foo if {
	data[input]
}
`),
			err: "rego_recursion_error: rule data.system.main.foo is recursive: data.system.main.foo -> data.system.main.foo",
		},
	} {
		t.Run(tc.note, func(t *testing.T) {
			c := NewCompiler()
			c.Modules = map[string]*Module{tc.note: tc.mod}
			compileStages(c, StageCheckRecursion)

			result := compilerErrsToStringSlice(c.Errors)
			expected := tc.err

			if len(result) != 1 || result[0] != expected {
				t.Errorf("Expected %v but got: %v", expected, result)
			}
		})
	}
}

// This is a regression test for a scenario that could make recursion checking miss a recursion scenario in OPA versions older than 0.56.0.
func TestCompilerCheckPartialRuleRecursion(t *testing.T) {
	// In the below policy, R2 and R3 has a recursion cycle. In OPA < 0.56.0, R1 hides this cycle from the recursion checker,
	// and no error is reported.
	policy := `package test

# R1
results[id] := 1 if {
  id := "bar"
}

# R2
results.foo := 2 if {
  final_allow
}

# R3
final_allow if {
  results.foo == 3
}`
	c := NewCompiler()
	c.Modules = map[string]*Module{"test": module(policy)}
	compileStages(c, StageCheckRecursion)

	expected := Errors{
		&Error{Code: "rego_recursion_error", Message: "rule data.test.results.foo is recursive: data.test.results.foo -> data.test.final_allow -> data.test.results.foo"},
		&Error{Code: "rego_recursion_error", Message: "rule data.test.final_allow is recursive: data.test.final_allow -> data.test.results.foo -> data.test.final_allow"},
	}

	assertErrors(t, c.Errors, expected, false)
}

func TestCompilerCheckVoidCalls(t *testing.T) {
	c := NewCompiler().WithCapabilities(&Capabilities{Builtins: []*Builtin{
		{
			Name: "test",
			Decl: types.NewFunction([]types.Type{types.B}, nil),
		},
	}})
	c.Compile(map[string]*Module{
		"test.rego": module(`package test

		p if {
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
	mods["mod-incr"] = module(`package a.b.c

p contains 1 if { true }
p contains 2 if { true }`,
	)

	c := NewCompiler()
	c.Compile(mods)
	assertNotFailed(t, c)

	tests := []struct {
		note     string
		ref      any
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
				found := slices.ContainsFunc(tc.expected, rules[i].Equal)
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
	mods["mod-incr"] = module(`package a.b.c

p contains 1 if { true }
p contains 2 if { true }`,
	)

	c := NewCompiler()
	c.Compile(mods)
	assertNotFailed(t, c)

	tests := []struct {
		note     string
		ref      any
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
				found := slices.ContainsFunc(tc.expected, rules[i].Equal)
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
	mods["mod-incr"] = module(`package a.b.c

p contains 1 if { true }
p contains 2 if { true }
q contains 3 if { true }`,
	)

	c := NewCompiler()
	c.Compile(mods)
	assertNotFailed(t, c)

	tests := []struct {
		note     string
		ref      any
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
				found := slices.ContainsFunc(tc.expected, rules[i].Equal)
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

p[x] = y if { q[x] = y }
q["a"] = 1 if { true }
q["b"] = 2 if { true }`,
	})

	compileStages(compiler, "")

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
				found := slices.ContainsFunc(tc.expected, result[i].Equal)
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
r5[x] = 5 if { x := "foo" }
r5.bar = 6 if { input.x }
r5.baz = 7 if { input.y }
`,
	})

	compileStages(compiler, "")

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
				found := slices.ContainsFunc(tc.expected, result[i].Equal)
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
		"test.rego": module(`
			package test

			p if { baz("x") = x }
			q if { foo.bar("x") = x }
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
		"test.rego": module(`
			package test

			p if { baz(1) = x }  # type error
			q if { foo.bar(1) = x }  # type error
		`),
	})

	assertCompilerErrorStrings(t, compiler, []string{exp1, exp2})
}

func TestCompilerLazyLoadingError(t *testing.T) {

	testLoader := func(map[string]*Module) (map[string]*Module, error) {
		return nil, errors.New("something went horribly wrong")
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

	mod1 := module(`package a.b.c

import data.x.z1 as z2

p = true if { q; r }
q = true if { z2 }`)
	orig1 := mod1.Copy()

	mod2 := module(`package a.b.c

r = true if { true }`)
	orig2 := mod2.Copy()

	mod3 := module(`package x

import data.foo.bar
import input.input

z1 = true if { [localvar | count(bar.baz.qux, localvar)] }`)
	orig3 := mod3.Copy()

	mod4 := module(`package foo.bar.baz

qux = grault if { true }`)
	orig4 := mod4.Copy()

	mod5 := module(`package foo.bar.baz

import data.d.e.f

deadbeef = f if { true }
grault = deadbeef if { true }`)
	orig5 := mod5.Copy()

	// testLoader will return 4 rounds of parsed modules.
	rounds := []map[string]*Module{
		{"mod1": mod1, "mod2": mod2},
		{"mod3": mod3},
		{"mod4": mod4},
		{"mod5": mod5},
	}

	popts := ParserOptions{AllFutureKeywords: true, unreleasedKeywords: true}

	// For each round, run checks.
	tests := []func(map[string]*Module){
		func(map[string]*Module) {
			// first round, no modules because compiler is invoked with empty
			// collection.
		},
		func(partial map[string]*Module) {
			p := MustParseRuleWithOpts(`p = true { data.a.b.c.q; data.a.b.c.r }`, popts)
			if !partial["mod1"].Rules[0].Equal(p) {
				t.Errorf("Expected %v but got %v", p, partial["mod1"].Rules[0])
			}
			q := MustParseRuleWithOpts(`q = true { data.x.z1 }`, popts)
			if !partial["mod1"].Rules[1].Equal(q) {
				t.Errorf("Expected %v but got %v", q, partial["mod1"].Rules[0])
			}
		},
		func(partial map[string]*Module) {
			z1 := MustParseRuleWithOpts(`z1 = true { [localvar | count(data.foo.bar.baz.qux, localvar)] }`, popts)
			if !partial["mod3"].Rules[0].Equal(z1) {
				t.Errorf("Expected %v but got %v", z1, partial["mod3"].Rules[0])
			}
		},
		func(partial map[string]*Module) {
			qux := MustParseRuleWithOpts(`qux = grault { true }`, popts)
			if !partial["mod4"].Rules[0].Equal(qux) {
				t.Errorf("Expected %v but got %v", qux, partial["mod4"].Rules[0])
			}
		},
		func(partial map[string]*Module) {
			grault := MustParseRuleWithOpts(`qux = data.foo.bar.baz.grault { true }`, popts) // rewrite has not happened yet
			f := MustParseRuleWithOpts(`deadbeef = data.d.e.f { true }`, popts)
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
	mod := MustParseModuleWithOpts(testModule, ParserOptions{AllFutureKeywords: true})

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

	mod := MustParseModuleWithOpts(testModule, ParserOptions{AllFutureKeywords: true})

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

				p if {
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

				p if {
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

				p if {
					[v | input[i] = v]  # skip because no assignment
				}`,
			wantDebug: 0,
		},
		{
			note: "skip: due to with modifier",
			module: `
				package test

				p if {
					v = input[i]
					ks = [j | input[j] = v] with data.x as 1  # skip because of with modifier
				}`,
			wantDebug: 0,
		},
		{
			note: "skip: due to negation",
			module: `
				package test

				p if {
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

				p if {
					v = input[i]
				}`,
			wantDebug: 0, // nothing interesting to report here
		},
		{
			note: "skip: due to unsafe comprehension body",
			module: `
				package test

				f(x) if {
					v = input[i]
					ys = [y | y = x[j]]  # x is not safe
				}`,
			wantDebug: 1,
		},
		{
			note: "skip: due to no candidates",
			module: `
				package test

				p if {
					ys = [y | y = input[j]]
				}`,
			wantDebug: 1,
		},
		{
			note: "mixed: due to nested comprehension containing candidate + indexed nested comprehension with key from rule body",
			module: `
				package test

				p if {
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

				f(x) if {
					y = input[x]
					ys = [y | y = input[x]]
				}`,
			wantDebug: 1,
		},
		{
			note: "skip: avoid increasing runtime (head key)",
			module: `
				package test

				p contains x if {
					y = input[x]
					ys = [y | y = input[x]]
				}`,
			wantDebug: 1,
		},
		{
			note: "skip: avoid increasing runtime (walk)",
			module: `
				package test

				p contains x if {
					y = input.bar[x]
					ys = [y | a = input.foo; walk(a, [x, y])]
				}`,
			wantDebug: 1,
		},
		{
			note: "bypass: use intermediate var to skip regression check",
			module: `
				package test

				p contains x if {
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
			mod, err := ParseModuleWithOpts("test.rego", tc.module, ParserOptions{AllFutureKeywords: true, unreleasedKeywords: true})
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

func TestCompilerBuildRequiredCapabilities(t *testing.T) {
	tests := []struct {
		note     string
		module   string
		opts     CompileOpts
		builtins []string
		features []string
		keywords []string
	}{
		{
			note: "trivial v0",
			module: `
				package x

				p { input > 7 }
			`,
			opts:     CompileOpts{ParserOptions: ParserOptions{RegoVersion: RegoV0}},
			builtins: []string{"eq", "gt"},
		},
		{
			note: "trivial v1",
			module: `
				package x

				p if { input > 7 }
			`,
			opts:     CompileOpts{ParserOptions: ParserOptions{RegoVersion: RegoV1}},
			builtins: []string{"eq", "gt"},
			features: []string{"rego_v1"},
		},
		{
			note: "rego.v1 import, v0 module",
			module: `
				package x

				import rego.v1

				p if { true }
			`,
			opts:     CompileOpts{ParserOptions: ParserOptions{RegoVersion: RegoV0}},
			features: []string{"rego_v1_import"},
		},
		{
			note: "rego.v1 import, v1 module",
			module: `
				package x

				import rego.v1

				p if { true }
			`,
			opts:     CompileOpts{ParserOptions: ParserOptions{RegoVersion: RegoV1}},
			features: []string{"rego_v1"},
		},
		{
			note: "rego.v1 import, default rego-version module (v1)",
			module: `
				package x

				import rego.v1

				p if { true }
			`,
			opts:     CompileOpts{ParserOptions: ParserOptions{RegoVersion: RegoV1}},
			features: []string{"rego_v1"},
		},
		{
			note: "future.keywords wildcard, v0 module",
			module: `
				package x

				import future.keywords
			`,
			opts:     CompileOpts{ParserOptions: ParserOptions{RegoVersion: RegoV0}},
			keywords: []string{"contains", "every", "if", "in"},
		},
		{
			note: "future.keywords wildcard, v1 module",
			module: `
				package x

				import future.keywords
			`,
			opts:     CompileOpts{ParserOptions: ParserOptions{RegoVersion: RegoV1}},
			features: []string{"rego_v1"},
		},
		{
			note: "future.keywords wildcard, default rego-version module (v1)",
			module: `
				package x

				import future.keywords
			`,
			features: []string{"rego_v1"},
		},
		{
			note: "future.keywords specific, v0 module",
			module: `
				package x

				import future.keywords.in
				import future.keywords.if
				import future.keywords.contains
				import future.keywords.every
			`,
			opts:     CompileOpts{ParserOptions: ParserOptions{RegoVersion: RegoV0}},
			keywords: []string{"contains", "every", "if", "in"},
		},
		{
			note: "future.keywords specific, v1 module",
			module: `
				package x

				import future.keywords.in
				import future.keywords.if
				import future.keywords.contains
				import future.keywords.every
			`,
			opts:     CompileOpts{ParserOptions: ParserOptions{RegoVersion: RegoV1}},
			features: []string{"rego_v1"},
		},
		{
			note: "future.keywords specific, default rego-version module (v1)",
			module: `
				package x

				import future.keywords.in
				import future.keywords.if
				import future.keywords.contains
				import future.keywords.every
			`,
			features: []string{"rego_v1"},
		},
		{
			note: "rewriting erases assignment",
			module: `
				package x

				p if { a := 7 }
			`,
			opts:     CompileOpts{ParserOptions: ParserOptions{RegoVersion: RegoV1}},
			builtins: []string{"assign", "eq"},
			features: []string{"rego_v1"},
		},
		{
			note: "rewriting erases equals",
			module: `
				package x

				p if { input == 7 }
			`,
			opts:     CompileOpts{ParserOptions: ParserOptions{RegoVersion: RegoV1}},
			builtins: []string{"eq", "equal"},
			features: []string{"rego_v1"},
		},
		{
			note: "rewriting erases print",
			module: `
				package x

				p if { print(7) }
			`,
			opts:     CompileOpts{EnablePrintStatements: true, ParserOptions: ParserOptions{RegoVersion: RegoV1}},
			builtins: []string{"eq", "internal.print", "print"},
			features: []string{"rego_v1"},
		},

		{
			note: "rewriting erases print but disabled",
			module: `
				package x

				p if { print(7) }
			`,
			opts:     CompileOpts{EnablePrintStatements: false, ParserOptions: ParserOptions{RegoVersion: RegoV1}},
			builtins: []string{"print"}, // only print required because compiler will replace with true
			features: []string{"rego_v1"},
		},
		{
			note: "dots in the head, v0 module",
			module: `
				package x

				a.b.c := 7
			`,
			opts:     CompileOpts{ParserOptions: ParserOptions{RegoVersion: RegoV0}},
			features: []string{"rule_head_ref_string_prefixes"},
		},
		{
			note: "dots in the head, v1 module",
			module: `
				package x

				a.b.c := 7
			`,
			opts:     CompileOpts{ParserOptions: ParserOptions{RegoVersion: RegoV1}},
			features: []string{"rego_v1"}, // rego_v1 includes rule_head_ref_string_prefixes
		},
		{
			note: "dots in the head, default rego-version module (v1)",
			module: `
				package x

				a.b.c := 7
			`,
			features: []string{"rego_v1"}, // rego_v1 includes rule_head_ref_string_prefixes
		},
		{
			note: "dynamic dots in the head, v0 module",
			module: `
				package x

				a[x].c[y] := z { x := "b"; y := "c"; z := "d" }
			`,
			opts:     CompileOpts{ParserOptions: ParserOptions{RegoVersion: RegoV0}},
			builtins: []string{"assign", "eq"},
			features: []string{"rule_head_refs"},
		},
		{
			note: "dynamic dots in the head, v1 module",
			module: `
				package x

				a[x].c[y] := z if { x := "b"; y := "c"; z := "d" }
			`,
			opts:     CompileOpts{ParserOptions: ParserOptions{RegoVersion: RegoV1}},
			builtins: []string{"assign", "eq"},
			features: []string{"rego_v1"}, // rego_v1 includes rule_head_refs
		},
		{
			note: "dynamic dots in the head, default rego-version module (v1)",
			module: `
				package x

				a[x].c[y] := z if { x := "b"; y := "c"; z := "d" }
			`,
			builtins: []string{"assign", "eq"},
			features: []string{"rego_v1"}, // rego_v1 includes rule_head_refs
		},
		{
			note: "template-string",
			module: `package test
				p := $"foo {42}"`,
			builtins: []string{"internal.template_string"},
			features: []string{FeatureRegoV1, FeatureTemplateStrings},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			compiler := MustCompileModulesWithOpts(map[string]string{
				"test.rego": tc.module,
			}, tc.opts)

			var names []string
			for i := range compiler.Required.Builtins {
				names = append(names, compiler.Required.Builtins[i].Name)
			}

			if !slices.Equal(names, tc.builtins) {
				t.Fatalf("expected builtins to be %v but got %v", tc.builtins, names)
			}

			if !slices.Equal(compiler.Required.FutureKeywords, tc.keywords) {
				t.Fatalf("expected keywords to be %v but got %v", tc.keywords, compiler.Required.FutureKeywords)
			}

			if !slices.Equal(compiler.Required.Features, tc.features) {
				t.Fatalf("expected features to be %v but got %v", tc.features, compiler.Required.Features)
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
		note        string
		q           string
		pkg         string
		imports     []string
		input       string
		regoVersion RegoVersion
		expected    any
	}{
		{
			note:     "empty query",
			q:        "   \t \n # foo \n",
			expected: errors.New("1 error occurred: rego_compile_error: empty query cannot be compiled"),
		},
		{
			note:     "invalid eq",
			q:        "eq()",
			expected: errors.New("1 error occurred: 1:1: rego_type_error: eq: arity mismatch\n\thave: ()\n\twant: (any, any)"),
		},
		{
			note:     "invalid eq",
			q:        "eq(1)",
			expected: errors.New("1 error occurred: 1:1: rego_type_error: eq: arity mismatch\n\thave: (number)\n\twant: (any, any)"),
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
			expected: errors.New("1 error occurred: 1:1: rego_unsafe_var_error: var z is unsafe"),
		},
		{
			note:        "unsafe var that is a future keyword",
			q:           "1 in 2",
			expected:    errors.New("1 error occurred: 1:3: rego_unsafe_var_error: var in is unsafe (hint: `import future.keywords.in` to import a future keyword)"),
			regoVersion: RegoV0,
		},
		{
			note:     "unsafe declared var",
			q:        "[1 | some x; x == 1]",
			pkg:      "",
			imports:  nil,
			expected: errors.New("1 error occurred: 1:14: rego_unsafe_var_error: var x is unsafe"),
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
			expected: errors.New("1 error occurred: 1:12: rego_type_error: with keyword target must reference existing input, data, or a function"),
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
			expected: errors.New("1 error occurred: 1:1: rego_type_error: startswith: arity mismatch\n\thave: (string)\n\twant: (search: string, base: string)"),
		},
		{
			note:     "built-in function arity mismatch (arity 0)",
			q:        `x := opa.runtime("foo")`,
			pkg:      "",
			imports:  nil,
			expected: errors.New("1 error occurred: 1:6: rego_type_error: opa.runtime: arity mismatch\n\thave: (string, ???)\n\twant: ()"),
		},
		{
			note:     "built-in function arity mismatch, nested",
			q:        "count(sum())",
			pkg:      "",
			imports:  nil,
			expected: errors.New("1 error occurred: 1:7: rego_type_error: sum: arity mismatch\n\thave: (???)\n\twant: (collection: any<array[number], set[number]>)"),
		},
		{
			note:     "check types",
			q:        "x = data.a.b.c.z; y = null; x = y",
			pkg:      "",
			imports:  nil,
			expected: errors.New("match error\n\tleft  : number\n\tright : null"),
		},
		{
			note:     "undefined function",
			q:        "data.deadbeef(x)",
			expected: errors.New("rego_type_error: undefined function data.deadbeef"),
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
			expected: errors.New("rego_type_error: print(1) used as value"),
		},
		{
			note:     "print call erasure",
			q:        `print(1)`,
			expected: "true",
		},
	}
	for _, tc := range tests {
		popts := ParserOptions{RegoVersion: tc.regoVersion}
		t.Run(tc.note, runQueryCompilerTest(tc.q, popts, tc.pkg, tc.imports, tc.expected))
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
			expectedErrors: errors.New("1 error occurred: 1:1: rego_type_error: deprecated built-in function calls in expression: all"),
		},
		{
			note:           "any() built-in",
			query:          "any([true, false])",
			expectedErrors: errors.New("1 error occurred: 1:1: rego_type_error: deprecated built-in function calls in expression: any"),
		},
	}

	runStrictnessQueryTestCase(t, cases)
}

func TestQueryCompilerWithUnusedAssignedVar(t *testing.T) {
	cases := []strictnessQueryTestCase{
		{
			note:           "array comprehension",
			query:          "[1 | x := 2]",
			expectedErrors: errors.New("1 error occurred: 1:6: rego_compile_error: assigned var x unused"),
		},
		{
			note:           "set comprehension",
			query:          "{1 | x := 2}",
			expectedErrors: errors.New("1 error occurred: 1:6: rego_compile_error: assigned var x unused"),
		},
		{
			note:           "object comprehension",
			query:          "{1: 2 | x := 2}",
			expectedErrors: errors.New("1 error occurred: 1:9: rego_compile_error: assigned var x unused"),
		},
		{
			note:           "every: unused var in body",
			query:          "every _ in [] { x := 10 }",
			expectedErrors: errors.New("1 error occurred: 1:17: rego_compile_error: assigned var x unused"),
		},
	}

	runStrictnessQueryTestCase(t, cases)
}

func TestQueryCompilerCheckKeywordOverrides(t *testing.T) {
	cases := []strictnessQueryTestCase{
		{
			note:           "input assigned",
			query:          "input := 1",
			expectedErrors: errors.New("1 error occurred: 1:1: rego_compile_error: variables must not shadow input (use a different variable name)"),
		},
		{
			note:           "data assigned",
			query:          "data := 1",
			expectedErrors: errors.New("1 error occurred: 1:1: rego_compile_error: variables must not shadow data (use a different variable name)"),
		},
		{
			note:           "nested input assigned",
			query:          "d := [input | input := 1]",
			expectedErrors: errors.New("1 error occurred: 1:15: rego_compile_error: variables must not shadow input (use a different variable name)"),
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
			} else if err != nil {
				t.Fatalf("Unexpected error from %v: %v", tc.query, err)
			}
		}
	}

	for _, tc := range cases {
		t.Run(tc.note+"_strict", makeTestRunner(tc, true))
		t.Run(tc.note+"_non-strict", makeTestRunner(tc, false))
	}
}

func TestQueryCompilerRewriteTemplateStrings(t *testing.T) {
	cases := []struct {
		note  string
		query string
		exp   string
	}{
		{
			note:  "empty template string",
			query: `$""`,
			exp:   `internal.template_string([""], __localq0__); __localq0__`,
		},
		{
			note:  "non-empty template string with no expressions",
			query: `$"foo"`,
			exp:   `internal.template_string(["foo"], __localq0__); __localq0__`,
		},
		{
			note:  "template string with value expressions",
			query: `$"{true} {null} {42} {1.2} {"foo"} {[1, 2]} {{1, 2}} {{"a": 1, "b": 2}}"`,
			exp: `__localq4__ = {__localq0__ | __localq0__ = [1, 2]}
				__localq5__ = {__localq1__ | __localq1__ = {1, 2}}
				__localq6__ = {__localq2__ | __localq2__ = {"a": 1, "b": 2}}
				internal.template_string([true, " ", null, " ", 42, " ", 1.2, " ", "foo", " ", __localq4__, " ", __localq5__, " ", __localq6__], __localq3__)
				__localq3__`,
		},
		{
			note:  "template string with var and ref expressions",
			query: `x := 42; $"{x} {input.y}"`,
			exp: `__localq0__ = 42
				__localq3__ = {__localq1__ | __localq1__ = input.y}
				internal.template_string([{__localq0__}, " ", __localq3__], __localq2__)
				__localq2__`,
		},
		{
			note:  "template string with call expressions",
			query: `$"{array.concat([1], [2])} {1 + 2} {true != false}"`,
			exp: `__localq7__ = {__localq0__ | array.concat([1], [2], __localq3__); __localq0__ = __localq3__}
				__localq8__ = {__localq1__ | plus(1, 2, __localq4__); __localq1__ = __localq4__}
				__localq9__ = {__localq2__ | neq(true, false, __localq5__); __localq2__ = __localq5__}
				internal.template_string([__localq7__, " ", __localq8__, " ", __localq9__], __localq6__)
				__localq6__`,
		},
		{
			note:  "template string with comprehension expressions",
			query: `$"{[x | x := input.xs[_]]} {{y | y := input.ys[_]}} {{a: b | a := input.as[_]; b := input.bs[_]}}"`,
			exp: `__localq8__ = {__localq4__ | __localq4__ = [__localq0__ | __localq0__ = input.xs[_]]}
				__localq9__ = {__localq5__ | __localq5__ = {__localq1__ | __localq1__ = input.ys[_]}}
				__localq10__ = {__localq6__ | __localq6__ = {__localq2__: __localq3__ | __localq2__ = input["as"][_]; __localq3__ = input.bs[_]}}
				internal.template_string([__localq8__, " ", __localq9__, " ", __localq10__], __localq7__)
				__localq7__`,
		},
		{
			note:  "binding",
			query: `x := 42; y := $"{x}"`,
			exp: `__localq0__ = 42
				internal.template_string([{__localq0__}], __localq2__);
				__localq1__ = __localq2__`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			c := NewCompiler()
			qc := c.QueryCompiler()

			result, err := qc.Compile(MustParseBody(tc.query))

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			exp := MustParseBody(tc.exp)

			if !exp.Equal(result) {
				t.Fatalf("Expected:\n%v\n\nGot:\n%v", exp, result)
			}
		})
	}
}

func TestQueryCompilerRewriteTemplateStringsErrors(t *testing.T) {
	cases := []struct {
		note   string
		query  string
		expErr string
	}{
		{
			note:   "unsafe var",
			query:  `$"{x}"`,
			expErr: "rego_unsafe_var_error: var x is unsafe",
		},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			c := NewCompiler()
			qc := c.QueryCompiler()
			_, err := qc.Compile(MustParseBody(tc.query))

			if err == nil {
				t.Fatal("Expected error but got none")
			}

			if !strings.Contains(err.Error(), tc.expErr) {
				t.Fatalf("Expected error %v but got: %v", tc.expErr, err)
			}
		})
	}
}

func assertCompilerErrorStrings(t *testing.T, compiler *Compiler, expected []string) {
	t.Helper()
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
	popts := ParserOptions{AllFutureKeywords: true, unreleasedKeywords: true}

	for id, input := range mods {
		mod, err := ParseModuleWithOpts(id, input, popts)
		if err != nil {
			panic(err)
		}
		parsed[id] = mod
	}

	compiler := NewCompiler()
	compiler.Modules = parsed

	return compiler
}

// compileStages is a helper function to run compiler up to a given stage.
// If stageID is empty, a normal full compile run is performed.
// This works directly on c.Modules that are already set by tests.
func compileStages(c *Compiler, stageID StageID) {
	c.init()

	c.sorted = make([]string, 0, len(c.Modules))
	for name := range c.Modules {
		c.sorted = append(c.sorted, name)
	}
	sort.Strings(c.sorted)

	c = c.SetErrorLimit(0) // Tests need to see all errors, not just the first few

	if stageID != "" {
		c = c.WithOnlyStagesUpTo(stageID)
	}

	c.compile()
}

func getCompilerTestModules() map[string]*Module {

	mod1 := MustParseModule(`package a.b.c
import rego.v1
import data.x.y.z as foo
import data.g.h.k

p contains x if { q[x]; not r[x] }
q contains x if { foo[i] = x }
z = 400 if { true }`,
	)

	mod2 := MustParseModule(`package a.b.c
import rego.v1
import data.bar
import data.x.y.p

r contains x if { bar[x] = 100; p = 101 }`)

	mod3 := MustParseModule(`package a.b.d
import rego.v1
import input.x as y

t = true if { input = {y.secret: [{y.keyid}]} }
x = false if { true }`)

	mod4 := MustParseModule(`package a.b.empty`)

	mod5 := MustParseModule(`package a.b.compr
import rego.v1
import input.x as y
import data.a.b.c.q

p = true if { [y.a | true] }
r = true if { [q.a | true] }
s = true if { [true | y.a = 0] }
t = true if { [true | q[i] = 1] }
u = true if { [true | _ = [y.a | true]] }
v = true if { [true | _ = [true | q[i] = 1]] }
`,
	)

	mod6 := MustParseModule(`package a.b.nested
import rego.v1
import data.x
import data.z
import input.x as y

p = true if { x[y[i].a[z.b[j]]] }
q = true if { x = v; v[y[i]] }
r = 1 if { true }
s = true if { x[r] }`,
	)

	mod7 := MustParseModule(`package a.b.funcs
import rego.v1
fn(x) = y if {
	trim(x, ".", y)
}

bar([x, y]) = [a, [b, c]] if {
	fn(x, a)
	y[1].b = b
	y[i].a = "hi"
	c = y[i].b
}

foorule = true if {
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
	result := make([]string, 0, len(errors))
	for _, e := range errors {
		msg := strings.SplitN(e.Error(), ":", 3)[2]
		result = append(result, strings.TrimSpace(msg))
	}
	sort.Strings(result)
	return result
}

func runQueryCompilerTest(q string, popts ParserOptions, pkg string, imports []string, expected any) func(*testing.T) {
	return func(t *testing.T) {
		t.Helper()
		c := NewCompiler().WithEnablePrintStatements(false)
		c.Compile(getCompilerTestModules())
		assertNotFailed(t, c)
		qc := c.QueryCompiler()
		query := MustParseBodyWithOpts(q, popts)
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

func TestCompilerCapabilitiesFeatures(t *testing.T) {
	cases := []struct {
		note        string
		module      string
		features    []string
		builtins    []*Builtin
		expectedErr string
	}{
		{
			note: "no features, no ref-head rules",
			module: `package test
				p := 42`,
		},
		{
			note: "no features, ref-head rule",
			module: `package test
				p.q.r := 42`,
			expectedErr: "rego_compile_error: rule heads with refs are not supported: p.q.r",
		},
		{
			note: "no features, general-ref-head rule",
			module: `package test
				p[q].r[s] := 42 if { q := "foo"; s := "bar" }`,
			expectedErr: "rego_compile_error: rule heads with refs are not supported: p[q].r[s]",
		},
		{
			note: "string-prefix-ref-head feature, no ref-head rules",
			features: []string{
				FeatureRefHeadStringPrefixes,
			},
			module: `package test
				p := 42`,
		},
		{
			note: "string-prefix-ref-head feature, ref-head rule",
			features: []string{
				FeatureRefHeadStringPrefixes,
			},
			module: `package test
				p.q.r := 42`,
		},
		{
			note: "ref-head feature, ref-head rule",
			features: []string{
				FeatureRefHeads,
			},
			module: `package test
				p.q.r := 42`,
		},
		{
			note: "rego-v1 feature, ref-head rule",
			features: []string{
				FeatureRegoV1,
			},
			module: `package test
				p.q.r := 42`,
		},
		{
			note: "string-prefix-ref-head feature, general-ref-head rule",
			features: []string{
				FeatureRefHeadStringPrefixes,
			},
			module: `package test
				p[q].r[s] := 42 if { q := "foo"; s := "bar" }`,
			expectedErr: "rego_type_error: rule heads with general refs (containing variables) are not supported: p[q].r[s]",
		},
		{
			note: "ref-head feature, general-ref-head rule",
			features: []string{
				FeatureRefHeads,
			},
			module: `package test
				p[q].r[s] := 42 if { q := "foo"; s := "bar" }`,
		},
		{
			note: "rego-v1 feature, general-ref-head rule",
			features: []string{
				FeatureRegoV1,
			},
			module: `package test
				p[q].r[s] := 42 if { q := "foo"; s := "bar" }`,
		},
		{
			note: "string-prefix-ref-head & ref-head features, general-ref-head rule",
			features: []string{
				FeatureRefHeadStringPrefixes,
				FeatureRefHeads,
			},
			module: `package test
				p[q].r[s] := 42 if { q := "foo"; s := "bar" }`,
		},
		{
			note: "string-prefix-ref-head & ref-head & rego-v1 features, general-ref-head rule",
			features: []string{
				FeatureRefHeadStringPrefixes,
				FeatureRefHeads,
				FeatureRegoV1,
			},
			module: `package test
				p[q].r[s] := 42 if { q := "foo"; s := "bar" }`,
		},
		{
			note: "string-prefix-ref-head & ref-head features, ref-head rule",
			features: []string{
				FeatureRefHeadStringPrefixes,
				FeatureRefHeads,
			},
			module: `package test
				p.q.r := 42`,
		},
		{
			note: "string-prefix-ref-head & ref-head & rego-v1 features, ref-head rule",
			features: []string{
				FeatureRefHeadStringPrefixes,
				FeatureRefHeads,
				FeatureRegoV1,
			},
			module: `package test
				p.q.r := 42`,
		},
		{
			note:     "no features, string-prefix-ref-head with contains kw",
			features: []string{},
			module: `package test
				import future.keywords.contains
				p.x contains 1`,
			expectedErr: "rego_compile_error: rule heads with refs are not supported: p.x",
		},
		{
			note: "string-prefix-ref-head feature, string-prefix-ref-head with contains kw",
			features: []string{
				FeatureRefHeadStringPrefixes,
			},
			module: `package test
				import future.keywords.contains
				p.x contains 1`,
		},
		{
			note: "ref-head feature, string-prefix-ref-head with contains kw",
			features: []string{
				FeatureRefHeads,
			},
			module: `package test
				import future.keywords.contains
				p.x contains 1`,
		},
		{
			note: "rego-v1 feature, string-prefix-ref-head with contains kw",
			features: []string{
				FeatureRegoV1,
			},
			module: `package test
				import future.keywords.contains
				p.x contains 1`,
		},

		{
			note:     "no features, general-ref-head with contains kw",
			features: []string{},
			module: `package test
				import future.keywords
				p[x] contains 1 if x = "foo"`,
			expectedErr: "rego_compile_error: rule heads with refs are not supported: p[x]",
		},
		{
			note: "string-prefix-ref-head feature, general-ref-head with contains kw",
			features: []string{
				FeatureRefHeadStringPrefixes,
			},
			module: `package test
				import future.keywords
				p[x] contains 1 if x = "foo"`,
			expectedErr: "rego_type_error: rule heads with general refs (containing variables) are not supported: p[x]",
		},
		{
			note: "ref-head feature, general-ref-head with contains kw",
			features: []string{
				FeatureRefHeads,
			},
			module: `package test
				import future.keywords
				p[x] contains 1 if x = "foo"`,
		},
		{
			note: "rego-v1 feature, general-ref-head with contains kw",
			features: []string{
				FeatureRegoV1,
			},
			module: `package test
				import future.keywords
				p[x] contains 1 if x = "foo"`,
		},

		{
			note: "no features, rego.v1 import",
			module: `package test
				import rego.v1
				p if { true }`,
			expectedErr: "rego_compile_error: rego.v1 import is not supported",
		},
		{
			note: "rego-v1-import feature, rego.v1 import",
			module: `package test
				import rego.v1
				p if { true }`,
			features: []string{
				FeatureRegoV1Import,
			},
		},
		{
			note: "rego-v1-import feature, rego.v1 import",
			module: `package test
				import rego.v1
				p if { true }`,
			features: []string{
				FeatureRegoV1,
			},
		},
		{
			note: "no features, template-string",
			module: `package test
				p := $"foo {42}"`,
			expectedErr: "rego_compile_error: template-strings are not supported",
		},
		{
			note: "template-string feature, no internal.template_string built-in, template-string",
			module: `package test
				p := $"foo {42}"`,
			features: []string{
				FeatureTemplateStrings,
			},
			builtins:    []*Builtin{},
			expectedErr: "rego_compile_error: template-strings are not supported",
		},
		{
			note: "template-string feature, internal.template_string built-in, template-string",
			module: `package test
				p := $"foo {42}"`,
			features: []string{
				FeatureTemplateStrings,
			},
			builtins: []*Builtin{
				InternalTemplateString,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			capabilities := CapabilitiesForThisVersion()
			capabilities.Features = tc.features

			if tc.builtins != nil {
				capabilities.Builtins = tc.builtins
			}

			// Modules are parsed with full set of capabilities
			mod := module(tc.module)

			compiler := NewCompiler().WithCapabilities(capabilities)
			compiler.Compile(map[string]*Module{"test": mod})
			if tc.expectedErr != "" {
				if !compiler.Failed() {
					t.Fatal("expected error but got success")
				}
				if !strings.Contains(compiler.Errors.Error(), tc.expectedErr) {
					t.Fatalf("expected error:\n\n%s\n\nbut got:\n\n%v", tc.expectedErr, compiler.Errors)
				}
			} else if compiler.Failed() {
				t.Fatalf("unexpected error(s): %v", compiler.Errors)
			}
		})
	}
}

func TestCustomBuiltinWithCompileModulesWithOpt(t *testing.T) {
	tests := []struct {
		name              string
		module            string
		expectedErrorCode string
		skipCapabilities  bool
	}{
		{
			name: "custom builtin",
			module: `package test

			p if { bar(2) }`,
		},
		{
			name: "missing custom builtin",
			module: `package test
	
			p if { foo(1,2,x) }`,
			expectedErrorCode: "rego_type_error",
		},
		{
			name: "no capabilities, using custom builtin",
			module: `package test

			p if { bar(2) }`,
			skipCapabilities:  true,
			expectedErrorCode: "rego_type_error",
		},
		{
			name: "no capabilities",
			module: `package test
				import rego.v1
				p if { true }`,
			skipCapabilities: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			customBuiltin := &Builtin{
				Name: "bar",
				Decl: types.NewFunction([]types.Type{types.N}, types.B),
			}

			capabilities := CapabilitiesForThisVersion()
			capabilities.Builtins = append(capabilities.Builtins, customBuiltin)

			var err error
			if tc.skipCapabilities {
				_, err = CompileModulesWithOpt(map[string]string{"x": tc.module}, CompileOpts{})
			} else {
				_, err = CompileModulesWithOpt(map[string]string{"x": tc.module}, CompileOpts{
					ParserOptions: ParserOptions{
						Capabilities: capabilities,
					},
				})
			}

			if tc.expectedErrorCode == "" && err != nil {
				t.Fatal(err)
			}
			if tc.expectedErrorCode != "" {
				if err == nil {
					t.Fatalf("expected error code %s but got success", tc.expectedErrorCode)
				}
				var astError Errors
				if errors.As(err, &astError) {
					if astError[0].Code != tc.expectedErrorCode {
						t.Fatalf("expected error code %s but got %s", tc.expectedErrorCode, astError[0].Code)
					}
				} else {
					t.Fatal(err)
				}
			}
		})
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

	module1 := module(`package test

	p if { foo(1); bar(2) }`)
	module2 := module(`package test

	p if { plus(1,2,x) }`)

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
	modules := map[string]*Module{"mod1": module(`package a.b.c
deny if {
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

allow if {
    input.identity = "foo"
}

allow if {
    input.path = ["foo", "bar"]
}

allow if {
    input.params = {"foo": "bar"}
}`

	module2 := `
package policy

default allow := false

allow if {
    input.identty = "foo"
}`

	module3 := `
package policy

default allow := false

allow if {
    input.path = "foo"
}`

	module4 := `
package policy

default allow := false

allow if {
    input.identty = "foo"
}

allow if {
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
					ProcessAnnotation:  true,
					AllFutureKeywords:  true,
					unreleasedKeywords: true,
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

			errs := NewCompiler().WithSchemas(schemaSet).PassesTypeCheckRules(elems)

			if len(errs) > 0 {
				if len(tc.errs) == 0 {
					t.Fatalf("Unexpected error: %v", errs)
				}

				result := compilerErrsToStringSlice(errs)

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

deny if {
	input.Something.Y.X.Name == "Something"
}
`

	c := NewCompiler()
	var schema any
	if err := json.Unmarshal([]byte(jsonSchema), &schema); err != nil {
		t.Fatal(err)
	}
	schemaSet := NewSchemaSet()
	schemaSet.Put(MustParseRef("schema.input"), schema)
	c.WithSchemas(schemaSet)

	m := MustParseModuleWithOpts(exampleModule, ParserOptions{
		ProcessAnnotation:  true,
		AllFutureKeywords:  true,
		unreleasedKeywords: true,
	})
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

deny if {
	input.Something.Y.X.ThisDoesNotExist == "Something"
}
`

	c := NewCompiler().
		WithUseTypeCheckAnnotations(true)
	var schema any
	if err := json.Unmarshal([]byte(jsonSchema), &schema); err != nil {
		t.Fatal(err)
	}
	schemaSet := NewSchemaSet()
	schemaSet.Put(MustParseRef("schema.input"), schema)
	c.WithSchemas(schemaSet)

	m := MustParseModuleWithOpts(exampleModule, ParserOptions{
		ProcessAnnotation:  true,
		AllFutureKeywords:  true,
		unreleasedKeywords: true,
	})
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

// FIXME(v1-test-refactor): In OPA 1.0, a call to here can be replaced with a call to MustParseModule.
func module(raw string, opts ...func(ParserOptions) ParserOptions) *Module {
	popts := ParserOptions{AllFutureKeywords: true, unreleasedKeywords: true}

	for _, opt := range opts {
		popts = opt(popts)
	}

	lessRaw := strings.TrimSpace(raw)
	if !strings.HasPrefix(lessRaw, "package ") && !strings.HasPrefix(lessRaw, "#") {
		raw = "package test\n\n" + raw
	}

	return MustParseModuleWithOpts(raw, popts)
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

deny if {
	queue := input.aws.sqs.queues[_]
	policy := queue.policies[_]
	doc := json.unmarshal(policy.document.value)
	statement = doc.Statement[_]
	action := statement.Action[_]
	action == "*"
}
`

	var schema any
	if err := json.Unmarshal([]byte(jsonSchema), &schema); err != nil {
		t.Fatal(err)
	}
	schemaSet := NewSchemaSet()
	schemaSet.Put(MustParseRef("schema.input"), schema)

	c := NewCompiler().WithSchemas(schemaSet)
	c.Compile(map[string]*Module{"testMod": MustParseModuleWithOpts(exampleModule, ParserOptions{
		ProcessAnnotation:  true,
		AllFutureKeywords:  true,
		unreleasedKeywords: true,
	})})
	assertNotFailed(t, c)
}

func TestCompilerRewriteTestRulesForTracing(t *testing.T) {
	tests := []struct {
		note    string
		rewrite bool
		module  string
		exp     string
	}{
		{
			note: "ref comparison, no rewrite",
			module: `package test

a := 1
b := 2

test_something if {
	a == b
}`,
			exp: `package test

a := 1 if { true }
b := 2 if { true }

test_something = true if {
	data.test.a = data.test.b
}`,
		},
		{
			note:    "ref comparison, rewrite",
			rewrite: true,
			module: `package test

a := 1
b := 2

test_something if {
	a == b
}`,
			// When the test fails on '__local0__ = __local1__', the values for 'a' and 'b' are captured in local bindings,
			// accessible by the tracer.
			exp: `package test

a := 1 if { true }
b := 2 if { true }

test_something = true if {
	__local0__ = data.test.a
	__local1__ = data.test.b
	__local0__ = __local1__
}`,
		},
		{
			note:    "ref comparison, not-stmt, rewrite",
			rewrite: true,
			module: `package test

a := 1
b := 2

test_something if {
	not a == b
}`,
			// We don't break out local vars from a not-stmt, as that would change the semantics of the rule.
			exp: `package test

a := 1 if { true }
b := 2 if { true }

test_something = true if {
	not data.test.a = data.test.b
}`,
		},
		{
			note: "ref comparison, inside every-stmt, no rewrite",
			module: `package test

a := 1
b := 2
l := [1, 2, 3]

test_something if {
	every x in l {
		a < b + x
	}
}`,
			exp: `package test

a := 1 if { true }
b := 2 if { true }
l := [1, 2, 3] if { true }

test_something = true if {
	__local2__ = data.test.l
	every __local0__, __local1__ in __local2__ {
		__local4__ = data.test.b
		plus(__local4__, __local1__, __local3__)
		__local5__ = data.test.a
		lt(__local5__, __local3__)
	}
}`,
		},
		{
			note:    "ref comparison, inside every-stmt, rewrite",
			rewrite: true,
			module: `package test

a := 1
b := 2
l := [1, 2, 3]

test_something if {
	every x in l {
		a < b + x
	}
}`,
			// When tests contain an 'every' statement, we're interested in the circumstances that made the every fail,
			// so it's body is rewritten.
			exp: `package test

a := 1 if { true }
b := 2 if { true }
l := [1, 2, 3] if { true }

test_something = true if {
	__local2__ = data.test.l;
	every __local0__, __local1__ in __local2__ {
		__local4__ = data.test.b
		plus(__local4__, __local1__, __local3__)
		__local5__ = data.test.a
		lt(__local5__, __local3__)
	}
}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			ms := map[string]string{
				"test.rego": tc.module,
			}
			c := getCompilerWithParsedModules(ms).
				WithRewriteTestRules(tc.rewrite)

			compileStages(c, StageRewriteTestRulesForTracing)
			assertNotFailed(t, c)

			result := c.Modules["test.rego"]
			exp := module(tc.exp)
			exp.Imports = nil // We strip the imports since the compiler will too
			if result.Compare(exp) != 0 {
				t.Fatalf("\nExpected:\n\n%v\n\nGot:\n\n%v", exp, result)
			}
		})
	}
}

func TestCompile_DefaultRegoVersion(t *testing.T) {
	tests := []struct {
		note    string
		modules map[string]*Module
		expErrs Errors
	}{
		{
			note: "no module rego-version, no v1 violations",
			modules: map[string]*Module{
				"test": {
					Package: MustParsePackage(`package test`),
					Imports: MustParseImports(`import data.foo
						import data.bar`),
				},
			},
		},
		{
			note: "no module rego-version, v1 violations", // default is v1, errors expected
			modules: map[string]*Module{
				"test": {
					Package: MustParsePackage(`package test`),
					Imports: MustParseImports(`import data.foo
						import data.bar as foo`),
				},
			},
			expErrs: Errors{
				&Error{
					Code:    CompileErr,
					Message: "import must not shadow import data.foo",
				},
			},
		},
		{
			note: "v0 module, v1 violations",
			modules: map[string]*Module{
				"test": MustParseModuleWithOpts(`package test
						import data.foo
						import data.bar as foo`,
					ParserOptions{RegoVersion: RegoV0}),
			},
		},
		{
			note: "v1 module, v1 violations",
			modules: map[string]*Module{
				"test": MustParseModuleWithOpts(`package test
						import data.foo
						import data.bar as foo`,
					ParserOptions{RegoVersion: RegoV1}),
			},
			expErrs: Errors{
				&Error{
					Code:    CompileErr,
					Message: "import must not shadow import data.foo",
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			compiler := NewCompiler()

			compiler.Compile(tc.modules)

			if len(tc.expErrs) > 0 {
				assertErrors(t, compiler.Errors, tc.expErrs, false)
			} else if len(compiler.Errors) > 0 {
				t.Fatalf("Unexpected errors: %v", compiler.Errors)
			}
		})
	}
}

func TestCompilerInitWithDefaultModuleLoader(t *testing.T) {
	// Reset the global variable after the test
	defer func() { defaultModuleLoader = nil }()

	// a dummy loader that adds "foo"
	loader1 := func(res map[string]*Module) (map[string]*Module, error) {
		mod := MustParseModule(`package foo`)
		resCopy := map[string]*Module{}
		maps.Copy(resCopy, res)
		resCopy["foo.rego"] = mod
		return resCopy, nil
	}

	// a dummy loader that adds "bar"
	loader2 := func(res map[string]*Module) (map[string]*Module, error) {
		mod := MustParseModule(`package bar`)
		resCopy := map[string]*Module{}
		maps.Copy(resCopy, res)
		resCopy["bar.rego"] = mod
		return resCopy, nil
	}

	DefaultModuleLoader(loader2)

	c := NewCompiler().WithModuleLoader(loader1)
	c.init()

	got, err := c.moduleLoader(make(map[string]*Module))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := map[string]*Module{
		"foo.rego": MustParseModule(`package foo`),
		"bar.rego": MustParseModule(`package bar`),
	}
	// check both modules are present
	for k, v := range expected {
		gotMod, ok := got[k]
		if !ok {
			t.Errorf("expected key %q in result", k)
			continue
		}
		if !reflect.DeepEqual(gotMod, v) {
			t.Errorf("unexpected module for %q: got %v want %v", k, gotMod, v)
		}
	}

	// Now, test defaultModuleLoader only
	c2 := NewCompiler()
	c2.init()
	got2, err := c2.moduleLoader(make(map[string]*Module))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := got2["bar.rego"]; !ok {
		t.Error("expected bar.rego from defaultModuleLoader in result")
	}
}

// Verify fix for https://github.com/open-policy-agent/opa/issues/8158
func TestCompilerCopiesTemplateStrings(t *testing.T) {
	mod := MustParseModule(`package p
	s contains z if {
		some y in [1, 2, 3]
		z := $"{y} "
	}`)
	cpy := mod.Copy()

	c1 := NewCompiler()
	if c1.Compile(map[string]*Module{"p.rego": mod}); c1.Failed() {
		t.Fatalf("unexpected compile errors: %v", c1.Errors)
	}

	c2 := NewCompiler()
	if c2.Compile(map[string]*Module{"p.rego": mod}); c2.Failed() {
		t.Fatalf("unexpected compile errors: %v", c2.Errors)
	}

	if !mod.Equal(cpy) {
		t.Fatalf("expected module to be unchanged after compilation")
	}
}
