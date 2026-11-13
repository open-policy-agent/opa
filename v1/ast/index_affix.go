// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"github.com/open-policy-agent/opa/internal/prefixtrie"
)

// This file holds the indexing of both ends of a string: `startswith` and
// `strings.any_prefix_match`, and `endswith` and `strings.any_suffix_match`. A
// suffix trie is a prefix trie over the base strings reversed.
//
// prefixTrie holds one level's prefix constraints, each prefix's value being the
// trieNode its rules hang off.
type prefixTrie = prefixtrie.Trie[trieNode]

func prefixTrieDo(p *prefixTrie, walker trieWalker) {
	_ = p.Values(func(node *trieNode) error {
		node.Do(walker)
		return nil
	})
}

func prefixTrieCompact(p *prefixTrie) {
	p.Compact()
	_ = p.Values(func(node *trieNode) error {
		node.compact()
		return nil
	})
}

func prefixTrieTraverseUnknown(p *prefixTrie, resolver ValueResolver, tr trieSink) error {
	return p.Values(func(node *trieNode) error {
		return node.Traverse(resolver, tr)
	})
}

// InsertPrefix records that the rules below this node require the value at ref
// to be a string starting with prefix.
func (node *trieNode) InsertPrefix(ref Ref, prefix Value) *trieNode {
	level := node.level()
	level.ref = ref

	s, ok := prefix.(String)
	if !ok {
		panic("illegal prefix value")
	}

	return level.affixTrie(affixPrefix).Insert(string(s))
}

// InsertSuffix records that the rules below this node require the value at ref
// to be a string ending with suffix.
func (node *trieNode) InsertSuffix(ref Ref, suffix Value) *trieNode {
	level := node.level()
	level.ref = ref

	s, ok := suffix.(String)
	if !ok {
		panic("illegal suffix value")
	}

	return level.affixTrie(affixSuffix).Insert(prefixtrie.Reverse(string(s)))
}

// traversePrefixes visits the rules whose prefix constraints value satisfies.
//
// strings.any_prefix_match takes a collection of search strings as readily as a
// single one, and holds if any of them starts with any of the base strings, so
// a collection is tested element by element, as traverseMembers does.
func (d *levelDetail) traversePrefixes(resolver ValueResolver, tr trieSink, value Value) error {
	prefixes := d.prefixes
	if prefixes == nil {
		return nil
	}

	// A closure shared with checkMember would escape, allocating per lookup.
	if s, ok := value.(String); ok {
		return prefixes.PrefixesOf(string(s), func(node *trieNode) error {
			return node.Traverse(resolver, tr)
		})
	}

	checkMember := func(t *Term) error {
		if s, ok := t.Value.(String); ok {
			return prefixes.PrefixesOf(string(s), func(node *trieNode) error {
				return node.Traverse(resolver, tr)
			})
		}
		return nil
	}

	switch col := value.(type) {
	case *Array:
		return col.Iter(checkMember)
	case Set:
		return col.Iter(checkMember)
	case Object:
		obj := objectFromObject(col)
		for _, node := range obj.sortedKeys() {
			if err := checkMember(node.value); err != nil {
				return err
			}
		}
	}

	return nil
}

// traverseSuffixes visits the rules whose suffix constraints value satisfies.
// A collection is tested element by element, as for prefixes.
func (d *levelDetail) traverseSuffixes(resolver ValueResolver, tr trieSink, value Value) error {
	suffixes := d.suffixes
	if suffixes == nil {
		return nil
	}

	// A closure shared with checkMember would escape, allocating per lookup.
	if s, ok := value.(String); ok {
		return suffixes.SuffixesOf(string(s), func(node *trieNode) error {
			return node.Traverse(resolver, tr)
		})
	}

	checkMember := func(t *Term) error {
		if s, ok := t.Value.(String); ok {
			return suffixes.SuffixesOf(string(s), func(node *trieNode) error {
				return node.Traverse(resolver, tr)
			})
		}
		return nil
	}

	switch col := value.(type) {
	case *Array:
		return col.Iter(checkMember)
	case Set:
		return col.Iter(checkMember)
	case Object:
		obj := objectFromObject(col)
		for _, node := range obj.sortedKeys() {
			if err := checkMember(node.value); err != nil {
				return err
			}
		}
	}

	return nil
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
// false unless every one of them is a string. The member's own Value is what
// comes back, not the String inside it: every caller puts it straight into a
// condition, and a Term is already holding it boxed.
func groundStrings(v Value) ([]Value, bool) {
	var (
		until func(func(*Term) bool) bool
		n     int
	)

	switch col := v.(type) {
	case *Array:
		until, n = col.Until, col.Len()
	case Set:
		until, n = col.Until, col.Len()
	default:
		return nil, false
	}

	// The base of a strings.any_prefix_match runs to thousands of strings, so
	// the length is worth taking off the collection rather than growing into.
	out := make([]Value, 0, n)

	// Until stops on the first member that is not a string, and reports having
	// stopped -- which is the whole of "unless every one of them is a string".
	if until(func(t *Term) bool {
		_, ok := t.Value.(String)
		if ok {
			out = append(out, t.Value)
		}
		return !ok
	}) {
		return nil, false
	}

	return out, true
}
