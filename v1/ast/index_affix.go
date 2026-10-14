// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"github.com/open-policy-agent/opa/internal/prefixtrie"
	"github.com/open-policy-agent/opa/v1/util"
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

func prefixTrieTraverseUnknown(p *prefixTrie, resolver ValueResolver, tr *trieTraversalResult) error {
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
// a collection is tested element by element -- the same way a scalar constraint
// is matched against the members of a collection (see
// traverseCollectionMembership).
func (d *levelDetail) traversePrefixes(resolver ValueResolver, tr *trieTraversalResult, value Value) error {
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
		if o, ok := col.(*object); ok {
			return o.Iter(func(_, v *Term) error {
				return checkMember(v)
			})
		}
		return col.Iter(func(_, v *Term) error {
			return checkMember(v)
		})
	}

	return nil
}

// traverseSuffixes visits the rules whose suffix constraints value satisfies.
// A collection is tested element by element, as for prefixes.
func (d *levelDetail) traverseSuffixes(resolver ValueResolver, tr *trieTraversalResult, value Value) error {
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
		if o, ok := col.(*object); ok {
			// doesn't allocate / escape
			for _, node := range o.sortedKeys() {
				if err := checkMember(node.value); err != nil {
					return err
				}
			}
			return nil
		}
		// allocates / escapes
		return col.Iter(func(_, v *Term) error {
			return checkMember(v)
		})
	}

	return nil
}

// updateStartsWith indexes `startswith(x, "base")`: x has to be a string
// starting with base for the rule to hold.
func (i *refindices) updateAffix(rule *Rule, expr *Expr, constants map[Var]Value, a affix) {
	ref := i.resolveAndValidateRef(rule, rule.Head.Args, expr.Operand(0))
	if ref == nil {
		return
	}

	base, ok := constantString(expr.Operand(1), constants)
	if !ok {
		return
	}

	i.insert(rule, &refindex{ref: i.table.intern(ref), Value: base, Affix: a})
}

// updateAnyPrefixMatch indexes `strings.any_prefix_match(x, base)`, which is
// a disjunction of startswith calls: each base string is recorded as an
// alternative prefix for x, the same way each element of `x in [...]` is
// recorded as an alternative value (see updateMemberRefInValue).
//
// The search operand has to be a single ref: a collection of search strings
// would need every element of one collection tested against the other, which
// is not a constraint on the value at any one ref.
func (i *refindices) updateAnyAffixMatch(rule *Rule, expr *Expr, constants map[Var]Value, a affix) {
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
		i.insert(rule, &refindex{ref: i.table.intern(ref), Value: s, Affix: a})
		return
	}

	// Every base string has to be recorded, or the index would exclude rules
	// the dropped ones would have matched -- so a base that isn't a collection
	// of ground strings throughout leaves the rule unindexed rather than
	// partly indexed.
	bases, ok := groundStrings(base)
	if !ok || len(bases) == 0 {
		return
	}

	i.insertAffixes(rule, ref, bases, a)
}

// insertAffixes records a whole base collection at once, for either end of the
// value. insert() rescans the rule's indices on every call, which is quadratic
// over the thousands strings.any_prefix_match carries, so the scan happens once
// here instead. insertMembers is the same for `in`; the two dedup on different
// key types.
func (i *refindices) insertAffixes(rule *Rule, ref Ref, bases []Value, a affix) {
	id := i.table.intern(ref)

	// concrete counts the values this rule already reaches ref by that survive
	// insertPath's var-stripping, so that the alternatives the base adds can be
	// weighed against them without a second scan (see refindices.alternate).
	concrete := 0
	known := false
	seen := make(map[String]struct{}, len(bases))
	for _, other := range i.rules[rule] {
		if other.ref != id {
			continue
		}
		known = true
		if !other.isVar() {
			concrete++
		}
		if other.Affix == a {
			if s, ok := other.Value.(String); ok {
				seen[s] = struct{}{}
			}
		}
	}
	n := len(bases)
	if known && n > 0 {
		n--
	}
	i.countN(id, n)

	// One refindex per base, laid down in a single block rather than allocated
	// one at a time: a base collection runs to thousands of them. Duplicates
	// leave slack at the end of the block, which the reslice below drops.
	pos := len(i.rules[rule])
	indices := util.GrowPtrSlice(i.rules[rule], len(bases))

	for _, base := range bases {
		// groundStrings has established that every base is a String, and hands
		// the Term's own Value over so that refindex.Value costs no second box.
		key := base.(String)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		concrete++

		*indices[pos] = refindex{ref: id, Value: base, Affix: a}
		pos++
	}
	i.rules[rule] = indices[:pos]

	if concrete > 1 {
		i.alternate(id, alternationTerminal)
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
// false unless every one of them is a string. The member's own Value is what
// comes back, not the String inside it: every caller puts it straight into a
// refindex, and a Term is already holding it boxed.
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
