// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package prefixtrie implements a compressed (radix) trie over string keys that
// answers "which of the keys does this string start with" in one walk of the
// string. It backs both the rule index's `startswith`/`strings.any_prefix_match`
// constraints and the `strings.any_prefix_match` builtin itself.
//
// Testing a string against every key in turn costs O(p) string comparisons for
// p keys. A lookup here walks the string once and costs O(len(s)) byte
// comparisons whatever p is. Compressed rather than one node per byte because
// the node count is then bounded by 2p-1 rather than by the total length of all
// keys -- 10k keys cost thousands of nodes, not hundreds of thousands.
//
// Suffixes are handled by the same structure: a trie holding its keys reversed
// (see Reverse) answers "which keys does s end with" by walking s from its last
// byte back (see SuffixesOf), which needs no reversed copy of s per lookup.
package prefixtrie

import (
	"slices"

	"github.com/open-policy-agent/opa/v1/util"
)

// Trie maps string keys to values of type V. The zero value is an empty trie
// ready to use. Each key has one *V, allocated with new(V) when the key is first
// inserted; callers hang whatever they need off it.
type Trie[V any] struct {
	// edges are sorted by the first byte of their label, which is unique among
	// them, so a step down the trie is a binary search.
	edges []edge[V]
	// child is the value of the key ending exactly here.
	child *V
}

type edge[V any] struct {
	label string
	// node is the trie under this edge; leaf stands in for it when nothing is
	// recorded past the edge's label, which is almost every edge.
	node *Trie[V]
	leaf *V
}

// find locates the edge labelled with first byte b, or the position a new one
// would be inserted at to keep edges sorted. Only that byte is matched; labels
// are compressed, so comparing the rest of one is left to the caller.
func (p *Trie[V]) find(b byte) (int, bool) {
	return slices.BinarySearchFunc(p.edges, b, func(e edge[V], b byte) int {
		return int(e.label[0]) - int(b)
	})
}

// Insert returns the value for key, creating it if this is the first time the
// key is inserted.
func (p *Trie[V]) Insert(key string) *V {
	node := p

	for {
		if key == "" {
			if node.child == nil {
				node.child = new(V)
			}
			return node.child
		}

		pos, found := node.find(key[0])
		if !found {
			leaf := new(V)
			node.edges = slices.Insert(node.edges, pos, edge[V]{label: key, leaf: leaf})
			return leaf
		}

		e := node.edges[pos]
		common := commonPrefixLen(e.label, key)

		switch {
		// The two diverge inside this edge -- "/api/v1" meeting "/api/v2" --
		// so the edge is split where they stop agreeing and what used to hang
		// off it moves down onto the tail, whichever kind it is.
		case common < len(e.label):
			tail := edge[V]{label: e.label[common:], node: e.node, leaf: e.leaf}
			node.edges[pos] = edge[V]{
				label: e.label[:common],
				node:  &Trie[V]{edges: []edge[V]{tail}},
			}

		// The key ends where an edge does with nothing past it, so its value is
		// already the answer.
		case common == len(key) && e.leaf != nil:
			return e.leaf

		// Something is recorded past the edge now, so its value becomes the
		// child of a trie of its own.
		case e.node == nil:
			node.edges[pos] = edge[V]{
				label: e.label,
				node:  &Trie[V]{child: e.leaf},
			}
		}

		node = node.edges[pos].node
		key = key[common:]
	}
}

// PrefixesOf calls visit with the value of every key that s starts with,
// shortest key first. One walk down the trie finds all of them: the keys that
// are prefixes of s are exactly the ends-of-key passed on the way down. It
// stops at the first error visit returns.
func (p *Trie[V]) PrefixesOf(s string, visit func(*V) error) error {
	for node := p; node != nil; {
		if node.child != nil {
			if err := visit(node.child); err != nil {
				return err
			}
		}

		if s == "" {
			return nil
		}

		pos, found := node.find(s[0])
		if !found {
			return nil
		}

		e := node.edges[pos]
		if len(e.label) > len(s) || s[:len(e.label)] != e.label {
			return nil
		}

		s = s[len(e.label):]
		if e.node == nil {
			return visit(e.leaf)
		}
		node = e.node
	}

	return nil
}

// SuffixesOf is PrefixesOf over the end of s, for a trie whose keys were
// inserted reversed (see Reverse).
func (p *Trie[V]) SuffixesOf(s string, visit func(*V) error) error {
	for node := p; node != nil; {
		if node.child != nil {
			if err := visit(node.child); err != nil {
				return err
			}
		}

		if s == "" {
			return nil
		}

		pos, found := node.find(s[len(s)-1])
		if !found {
			return nil
		}

		e := node.edges[pos]
		if len(e.label) > len(s) || !equalReversed(s[len(s)-len(e.label):], e.label) {
			return nil
		}

		s = s[:len(s)-len(e.label)]
		if e.node == nil {
			return visit(e.leaf)
		}
		node = e.node
	}

	return nil
}

// HasPrefixOf reports whether s starts with any key in the trie.
func (p *Trie[V]) HasPrefixOf(s string) bool {
	for node := p; node != nil; {
		if node.child != nil {
			return true
		}

		if s == "" {
			return false
		}

		pos, found := node.find(s[0])
		if !found {
			return false
		}

		e := node.edges[pos]
		if len(e.label) > len(s) || s[:len(e.label)] != e.label {
			return false
		}

		if e.node == nil {
			return true
		}
		s = s[len(e.label):]
		node = e.node
	}

	return false
}

// HasSuffixOf reports whether s ends with any key in a trie whose keys were
// inserted reversed (see Reverse).
func (p *Trie[V]) HasSuffixOf(s string) bool {
	for node := p; node != nil; {
		if node.child != nil {
			return true
		}

		if s == "" {
			return false
		}

		pos, found := node.find(s[len(s)-1])
		if !found {
			return false
		}

		e := node.edges[pos]
		if len(e.label) > len(s) || !equalReversed(s[len(s)-len(e.label):], e.label) {
			return false
		}

		if e.node == nil {
			return true
		}
		s = s[:len(s)-len(e.label)]
		node = e.node
	}

	return false
}

// Values calls visit with every value in the trie, stopping at the first error
// visit returns. A value is visited before the values below it, and siblings in
// the byte order of their labels.
func (p *Trie[V]) Values(visit func(*V) error) error {
	if p == nil {
		return nil
	}

	if p.child != nil {
		if err := visit(p.child); err != nil {
			return err
		}
	}

	for _, e := range p.edges {
		if err := e.node.Values(visit); err != nil {
			return err
		}
		if e.leaf != nil {
			if err := visit(e.leaf); err != nil {
				return err
			}
		}
	}

	return nil
}

// Entry is a key the trie holds, spelled out, with its value.
type Entry[V any] struct {
	Key   string
	Value *V
}

// Entries returns the keys the trie holds, in lexicographic order. Building the
// keys back up costs an allocation per key, so this is meant for debugging and
// tests rather than lookups.
func (p *Trie[V]) Entries() []Entry[V] {
	if p == nil {
		return nil
	}

	var (
		entries []Entry[V]
		collect func(node *Trie[V], prefix string)
	)

	collect = func(node *Trie[V], prefix string) {
		if node.child != nil {
			entries = append(entries, Entry[V]{Key: prefix, Value: node.child})
		}
		for _, e := range node.edges {
			if e.node == nil {
				entries = append(entries, Entry[V]{Key: prefix + e.label, Value: e.leaf})
				continue
			}
			collect(e.node, prefix+e.label)
		}
	}

	collect(p, "")

	return entries
}

// Compact releases the spare capacity in the edge slices. Edges arrive in
// arbitrary order, so they are placed by insertion and grow the way append does
// -- which leaves 60% of the slots unused across a large key set. Call it once
// nothing more will be inserted. slices.Clip only caps the capacity; releasing
// the block means copying out of it.
func (p *Trie[V]) Compact() {
	if p == nil {
		return
	}

	if cap(p.edges) > len(p.edges) {
		exact := make([]edge[V], len(p.edges))
		copy(exact, p.edges)
		p.edges = exact
	}

	for _, e := range p.edges {
		e.node.Compact()
	}
}

// Reverse returns s with its bytes reversed, for inserting a key into a trie
// queried with SuffixesOf or HasSuffixOf. Suffix matching is a byte comparison,
// so reversing bytes rather than runes is what makes a suffix of s a prefix of
// reversed s.
func Reverse(s string) string {
	b := []byte(s)
	slices.Reverse(b)

	// b was made here and is not written to again, so it can be handed over
	// rather than copied a second time.
	return util.ByteSliceToString(b)
}

// equalReversed reports whether tail read backwards is reversed.
func equalReversed(tail, reversed string) bool {
	for i := range reversed {
		if reversed[i] != tail[len(tail)-1-i] {
			return false
		}
	}
	return true
}

func commonPrefixLen(a, b string) int {
	n := min(len(a), len(b))
	i := 0
	for i < n && a[i] == b[i] {
		i++
	}
	return i
}
