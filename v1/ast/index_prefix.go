// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"errors"
	"slices"
)

// prefixTrie holds the string-prefix constraints recorded for one level of the
// rule index: what `startswith(input.x, "/api/")` and
// `strings.any_prefix_match(input.x, [...])` contribute.
//
// A scalar constraint is answered with a map lookup, but a prefix constraint
// has to answer "which of the recorded prefixes does this value start with",
// and the answer is a set, not a single entry. Testing the value against every
// recorded prefix in turn costs O(p) string comparisons per lookup for p
// prefixes -- which is the work strings.any_prefix_match exists to avoid doing
// in the rule body, so doing it in the index instead would be no bargain.
//
// This is a compressed (radix) trie instead: a lookup walks the value once and
// costs O(len(value)) byte comparisons whatever p is. Compressed rather than
// one node per byte because the node count is then bounded by 2p-1 rather than
// by the total length of all prefixes -- 10k prefixes cost thousands of nodes,
// not hundreds of thousands.
type prefixTrie struct {
	// edges are sorted by the first byte of their label, which is unique among
	// them, so a step down the trie is a binary search.
	edges []prefixEdge
	// child is where the rules of the prefixes ending exactly here hang off. It
	// is an ordinary trieNode, so whatever a rule constrains below a prefix
	// constraint indexes as usual.
	child *trieNode
}

type prefixEdge struct {
	label string
	node  *prefixTrie
}

// edge locates the edge labelled with first byte b, or the position a new one
// would be inserted at to keep edges sorted.
func (p *prefixTrie) edge(b byte) (int, bool) {
	return slices.BinarySearchFunc(p.edges, b, func(e prefixEdge, b byte) int {
		return int(e.label[0]) - int(b)
	})
}

// insert returns the node that the rules constrained by prefix hang off,
// creating it if this is the first time the prefix is recorded.
func (p *prefixTrie) insert(prefix string) *trieNode {
	node := p

	for {
		if prefix == "" {
			if node.child == nil {
				node.child = newTrieNodeImpl()
			}
			return node.child
		}

		pos, found := node.edge(prefix[0])
		if !found {
			leaf := &prefixTrie{child: newTrieNodeImpl()}
			node.edges = slices.Insert(node.edges, pos, prefixEdge{label: prefix, node: leaf})
			return leaf.child
		}

		edge := node.edges[pos]
		common := commonPrefixLen(edge.label, prefix)

		// The two diverge inside this edge -- "/api/v1" meeting "/api/v2" --
		// so the edge is split where they stop agreeing and what used to hang
		// off it moves down onto the tail.
		if common < len(edge.label) {
			node.edges[pos] = prefixEdge{
				label: edge.label[:common],
				node:  &prefixTrie{edges: []prefixEdge{{label: edge.label[common:], node: edge.node}}},
			}
		}

		node = node.edges[pos].node
		prefix = prefix[common:]
	}
}

// traverse visits the continuation of every recorded prefix that s starts with.
// One walk down the trie finds all of them: the prefixes of s that are in the
// trie are exactly the ends-of-prefix passed on the way down.
func (p *prefixTrie) traverse(s string, resolver ValueResolver, tr *trieTraversalResult) error {
	for node := p; node != nil; {
		if node.child != nil {
			if err := node.child.Traverse(resolver, tr); err != nil {
				return err
			}
		}

		if s == "" {
			return nil
		}

		pos, found := node.edge(s[0])
		if !found {
			return nil
		}

		edge := node.edges[pos]
		if len(edge.label) > len(s) || s[:len(edge.label)] != edge.label {
			return nil
		}

		s = s[len(edge.label):]
		node = edge.node
	}

	return nil
}

func (p *prefixTrie) do(walker trieWalker) {
	if p == nil {
		return
	}

	p.child.Do(walker)

	for _, edge := range p.edges {
		edge.node.do(walker)
	}
}

func (p *prefixTrie) traverseUnknown(resolver ValueResolver, tr *trieTraversalResult) error {
	if p == nil {
		return nil
	}

	if err := p.child.traverseUnknown(resolver, tr); err != nil {
		return err
	}

	for _, edge := range p.edges {
		if err := edge.node.traverseUnknown(resolver, tr); err != nil {
			return err
		}
	}

	return nil
}

// prefixEntry is a prefix the trie holds, spelled out, with the node its rules
// hang off.
type prefixEntry struct {
	prefix string
	node   *trieNode
}

// walk returns the prefixes the trie holds, in lexicographic order. Only the
// index's debug rendering and its tests have any use for reading back what the
// compression made of them.
func (p *prefixTrie) walk() []prefixEntry {
	if p == nil {
		return nil
	}

	var (
		entries []prefixEntry
		collect func(node *prefixTrie, prefix string)
	)

	collect = func(node *prefixTrie, prefix string) {
		if node.child != nil {
			entries = append(entries, prefixEntry{prefix: prefix, node: node.child})
		}
		for _, edge := range node.edges {
			collect(edge.node, prefix+edge.label)
		}
	}

	collect(p, "")

	return entries
}

func commonPrefixLen(a, b string) int {
	n := min(len(a), len(b))
	i := 0
	for i < n && a[i] == b[i] {
		i++
	}
	return i
}

// InsertPrefix records that the rules below this node require the value at ref
// to be a string starting with prefix.
func (node *trieNode) InsertPrefix(ref Ref, prefix Value) *trieNode {
	if node.next == nil {
		node.next = newTrieNodeImpl()
		node.next.ref = ref
	}

	s, ok := prefix.(String)
	if !ok {
		panic("illegal prefix value")
	}

	if node.next.prefixes == nil {
		node.next.prefixes = &prefixTrie{}
	}

	return node.next.prefixes.insert(string(s))
}

// traversePrefixes visits the rules whose prefix constraints value satisfies.
//
// strings.any_prefix_match takes a collection of search strings as readily as a
// single one, and holds if any of them starts with any of the base strings, so
// a collection is tested element by element -- the same way a scalar constraint
// is matched against the members of a collection (see
// traverseCollectionMembership).
func (node *trieNode) traversePrefixes(resolver ValueResolver, tr *trieTraversalResult, value Value) error {
	if node.prefixes == nil {
		return nil
	}

	if s, ok := value.(String); ok {
		return node.prefixes.traverse(string(s), resolver, tr)
	}

	checkMember := func(t *Term) error {
		if s, ok := t.Value.(String); ok {
			return node.prefixes.traverse(string(s), resolver, tr)
		}
		return nil
	}

	switch col := value.(type) {
	case *Array:
		return col.Iter(checkMember)
	case Set:
		return col.Iter(checkMember)
	case Object:
		return col.Iter(func(_, v *Term) error {
			return checkMember(v)
		})
	}

	return nil
}

// updateStartsWith indexes `startswith(x, "base")`: x has to be a string
// starting with base for the rule to hold.
func (i *refindices) updateStartsWith(rule *Rule, expr *Expr, constants map[Var]Value) {
	ref := i.resolveAndValidateRef(rule, rule.Head.Args, expr.Operand(0))
	if ref == nil {
		return
	}

	prefix, ok := constantString(expr.Operand(1), constants)
	if !ok {
		return
	}

	i.insert(rule, &refindex{Ref: ref, Value: prefix, Prefix: true})
}

// updateAnyPrefixMatch indexes `strings.any_prefix_match(x, base)`, which is
// a disjunction of startswith calls: each base string is recorded as an
// alternative prefix for x, the same way each element of `x in [...]` is
// recorded as an alternative value (see updateMemberRefInValue).
//
// The search operand has to be a single ref: a collection of search strings
// would need every element of one collection tested against the other, which
// is not a constraint on the value at any one ref.
func (i *refindices) updateAnyPrefixMatch(rule *Rule, expr *Expr, constants map[Var]Value) {
	ref := i.resolveAndValidateRef(rule, rule.Head.Args, expr.Operand(0))
	if ref == nil {
		return
	}

	base := expr.Operand(1).Value
	if v, ok := base.(Var); ok {
		resolved, ok := constants[v]
		if !ok {
			return
		}
		base = resolved
	}

	if s, ok := base.(String); ok {
		i.insert(rule, &refindex{Ref: ref, Value: s, Prefix: true})
		return
	}

	// Every base string has to be recorded, or the index would exclude rules
	// the dropped ones would have matched -- so a base that isn't a collection
	// of ground strings throughout leaves the rule unindexed rather than
	// partly indexed.
	prefixes, ok := groundStrings(base)
	if !ok || len(prefixes) == 0 {
		return
	}

	i.insertPrefixes(rule, ref, prefixes)
}

// insertPrefixes records a whole base collection at once. insert() rescans the
// rule's indices on every call, which is fine for the handful an ordinary rule
// contributes but quadratic for the thousands strings.any_prefix_match exists
// to carry, so the scan happens once here instead.
func (i *refindices) insertPrefixes(rule *Rule, ref Ref, prefixes []String) {
	i.countN(ref, len(prefixes))

	// concrete counts the values this rule already reaches ref by that survive
	// insertPath's var-stripping, so that the alternatives the base adds can be
	// weighed against them without a second scan (see refindices.alternate).
	concrete := 0
	seen := make(map[String]struct{}, len(prefixes))
	for _, other := range i.rules[rule] {
		if !other.Ref.Equal(ref) {
			continue
		}
		if !other.isVar() {
			concrete++
		}
		if other.Prefix {
			if s, ok := other.Value.(String); ok {
				seen[s] = struct{}{}
			}
		}
	}

	indices := i.rules[rule]
	for _, prefix := range prefixes {
		if _, ok := seen[prefix]; ok {
			continue
		}
		seen[prefix] = struct{}{}
		concrete++
		indices = append(indices, &refindex{Ref: ref, Value: prefix, Prefix: true})
	}
	i.rules[rule] = indices

	if concrete > 1 {
		i.alternate(ref)
	}
}

// constantString resolves term to a string literal, following one level of
// var binding recorded earlier in the rule body.
func constantString(term *Term, constants map[Var]Value) (String, bool) {
	v := term.Value
	if vr, ok := v.(Var); ok {
		resolved, ok := constants[vr]
		if !ok {
			return "", false
		}
		v = resolved
	}

	s, ok := v.(String)
	return s, ok
}

// groundStrings returns the members of an array or set literal, and reports
// false unless every one of them is a string.
func groundStrings(v Value) ([]String, bool) {
	var (
		iter func(func(*Term) error) error
		n    int
	)

	switch col := v.(type) {
	case *Array:
		iter, n = col.Iter, col.Len()
	case Set:
		iter, n = col.Iter, col.Len()
	default:
		return nil, false
	}

	// The base of a strings.any_prefix_match runs to thousands of strings, so
	// the length is worth taking off the collection rather than growing into.
	out := make([]String, 0, n)
	err := iter(func(t *Term) error {
		s, ok := t.Value.(String)
		if !ok {
			return errNotAString
		}
		out = append(out, s)
		return nil
	})

	if err != nil {
		return nil, false
	}

	return out, true
}

// errNotAString stops the iteration in groundStrings; it never reaches a
// caller.
var errNotAString = errors.New("not a string")
