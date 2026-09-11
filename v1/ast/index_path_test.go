// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

// countingTrieWalker counts the nodes reachable in a trie.
type countingTrieWalker struct {
	nodes *int
}

func (w countingTrieWalker) Do(any) trieWalker {
	*w.nodes++
	return w
}

func trieNodes(t *testing.T, index RuleIndex) int {
	t.Helper()

	idx, ok := index.(*baseDocEqIndex)
	if !ok {
		t.Fatalf("expected a baseDocEqIndex, got %T", index)
	}

	nodes := 0
	idx.root.Do(countingTrieWalker{nodes: &nodes})
	return nodes
}

func indexFor(t *testing.T, src string) RuleIndex {
	t.Helper()

	c := NewCompiler()
	if c.Compile(map[string]*Module{"p.rego": MustParseModule(src)}); c.Failed() {
		t.Fatal(c.Errors)
	}

	index := c.RuleIndex(MustParseRef("data.test.p"))
	if index == nil {
		t.Fatal("no index built for data.test.p")
	}
	return index
}

func selects(t *testing.T, index RuleIndex, input string) []string {
	t.Helper()

	res, err := index.Lookup(testResolver{input: MustParseTerm(input)})
	if err != nil {
		t.Fatal(err)
	}

	values := make([]string, 0, len(res.Rules))
	for _, rule := range res.Rules {
		values = append(values, rule.Head.Value.String())
	}
	slices.Sort(values)
	return values
}

// TestIndexPathTruncationSelectsSameRules is the correctness side of stopping a
// path at its last constrained level: a level a rule does not constrain cannot
// exclude it, so attaching it above that level has to select the same rules for
// every combination of present and absent refs.
//
// Each rule below constrains exactly one ref, and the refs are ordered by
// frequency, so every rule ends up at a different depth -- rule 1 at the first
// level, rule 3 at the third.
func TestIndexPathTruncationSelectsSameRules(t *testing.T) {
	index := indexFor(t, `package test

p := 1 if input.a == "x"

p := 2 if input.b == "x"

p := 3 if input.c == "x"
`)

	for _, tc := range []struct {
		note  string
		input string
		exp   []string
	}{
		{note: "all present and matching", input: `{"a": "x", "b": "x", "c": "x"}`, exp: []string{"1", "2", "3"}},
		{note: "all present, none matching", input: `{"a": "y", "b": "y", "c": "y"}`, exp: []string{}},
		{note: "only the shallowest matches", input: `{"a": "x", "b": "y", "c": "y"}`, exp: []string{"1"}},
		{note: "only the deepest matches", input: `{"a": "y", "b": "y", "c": "x"}`, exp: []string{"3"}},
		{
			// The refs a rule does not constrain being absent must not exclude it.
			note:  "deepest matches, earlier refs absent",
			input: `{"c": "x"}`,
			exp:   []string{"3"},
		},
		{
			note:  "shallowest matches, later refs absent",
			input: `{"a": "x"}`,
			exp:   []string{"1"},
		},
		{note: "nothing present", input: `{}`, exp: []string{}},
	} {
		t.Run(tc.note, func(t *testing.T) {
			if act := selects(t, index, tc.input); !slices.Equal(act, tc.exp) {
				t.Errorf("expected %v, got %v", tc.exp, act)
			}
		})
	}
}

// TestIndexPathTruncationKeepsTrieLinear is the point of it. n rules that each
// constrain a ref of their own share one spine, where padding each path out to
// the full depth gave every rule a private copy of the tail -- n(n+1)/2 nodes to
// build, and to walk on every lookup.
func TestIndexPathTruncationKeepsTrieLinear(t *testing.T) {
	for _, n := range []int{10, 50, 100} {
		t.Run(fmt.Sprintf("n=%d", n), func(t *testing.T) {
			var src strings.Builder
			src.WriteString("package test\n\n")
			for i := range n {
				fmt.Fprintf(&src, "p := %d if input.f%d == \"x\"\n\n", i, i)
			}

			nodes := trieNodes(t, indexFor(t, src.String()))

			// A spine of one level per ref, each with a value child and an "absent"
			// child, plus the root. Quadratic would be n(n+1)/2 and up.
			if max := 4 * n; nodes > max {
				t.Errorf("expected at most %d trie nodes for %d rules, got %d (quadratic would be ~%d)",
					max, n, nodes, n*(n+1)/2)
			}
		})
	}
}

// TestIndexPathTruncationLeavesSharedRefsAlone checks the shape truncation does
// not apply to: rules constraining the same ref all reach their last level at the
// same depth, so nothing is padded and nothing changes.
func TestIndexPathTruncationLeavesSharedRefsAlone(t *testing.T) {
	var src strings.Builder
	src.WriteString("package test\n\n")
	for i := range 100 {
		fmt.Fprintf(&src, "p := %d if input.f == %d\n\n", i, i)
	}

	index := indexFor(t, src.String())

	if act := selects(t, index, `{"f": 7}`); !slices.Equal(act, []string{"7"}) {
		t.Errorf("expected the index to select rule 7, got %v", act)
	}
	if nodes := trieNodes(t, index); nodes > 110 {
		t.Errorf("expected one level with a child per value, got %d nodes", nodes)
	}
}
