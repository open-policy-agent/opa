// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package prefixtrie implements a compressed (radix) trie that finds the keys a
// string starts with in one O(len(s)) walk, however many keys there are. A trie
// of reversed keys (see Reverse) finds the keys a string ends with.
package prefixtrie

import (
	"slices"

	"github.com/open-policy-agent/opa/v1/util"
)

// Trie maps string keys to values of type V. The zero value is an empty trie.
type Trie[V any] struct {
	// edges are sorted by the first byte of their label, which is unique.
	edges []edge[V]
	// child is the value of the key ending exactly here.
	child *V
}

type edge[V any] struct {
	label string
	// node is the trie under this edge, or nil if nothing is below it, in which
	// case leaf is the value of the key ending at the edge.
	node *Trie[V]
	leaf *V
}

// find locates the edge whose label starts with b, or where it would go.
func (p *Trie[V]) find(b byte) (int, bool) {
	return slices.BinarySearchFunc(p.edges, b, func(e edge[V], b byte) int {
		return int(e.label[0]) - int(b)
	})
}

// Insert returns the value for key, creating it with new(V) if key is new.
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
		// The key diverges inside the edge: split it.
		case common < len(e.label):
			tail := edge[V]{label: e.label[common:], node: e.node, leaf: e.leaf}
			node.edges[pos] = edge[V]{
				label: e.label[:common],
				node:  &Trie[V]{edges: []edge[V]{tail}},
			}

		// The key ends with a leaf edge.
		case common == len(key) && e.leaf != nil:
			return e.leaf

		// The key continues past a leaf edge: give it a trie of its own.
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

// PrefixesOf calls visit with the value of every key s starts with, shortest
// first, stopping at the first error.
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

// SuffixesOf is PrefixesOf for the end of s, in a trie of reversed keys.
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

// HasSuffixOf reports whether s ends with any key, in a trie of reversed keys.
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

// Values calls visit with every value in the trie, stopping at the first error.
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

// Entry is a key in the trie and its value.
type Entry[V any] struct {
	Key   string
	Value *V
}

// Entries returns the trie's keys in lexicographic order. It allocates a string
// per key, so is meant for debugging and tests.
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

// Compact releases the edge slices' spare capacity, often more than half of it.
// Call it once nothing more will be inserted.
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

// Reverse returns s with its bytes, not runes, reversed: the form of a key in a
// trie queried with SuffixesOf or HasSuffixOf.
func Reverse(s string) string {
	b := []byte(s)
	slices.Reverse(b)

	return util.ByteSliceToString(b) // b isn't written to again
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
