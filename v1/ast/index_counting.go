// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"cmp"
	"maps"
	"slices"
)

// countingIndex answers which rules a lookup reaches by following, per path to a
// rule, whether every condition on it holds (Whang et al., "Indexing Boolean
// Expressions", VLDB 2009). A path is one conjunction of conditions, at most one
// per indexed ref; its rule is reached when all of them hold.
//
// Picture a table with a row per path and a column per indexed ref, each cell
// holding the path's condition on that ref, if it has one:
//
//	        input.method   input.path       input.user
//	path 0  == "GET"       startswith "/a"
//	path 1  == "POST"                       defined
//
// A column is a level: a matcher (see levelDetail) whose leaves each name a
// posting, the set of rows whose cell there the leaf's value satisfies. A lookup
// visits the columns in turn and keeps the set of rows still alive:
//
//   - a level adds the paths that start on it, their first condition being
//     there, if the value satisfies them;
//   - and removes the paths continuing on it that the value does not satisfy;
//   - a level none of whose continuing paths is alive, and on which none start,
//     is not resolved at all.
//
// Levels go in order of the share of their paths a value can be expected to
// satisfy (see finish), so that the first ones leave few paths alive and the
// later ones only have to look at those.
type countingIndex struct {
	levels []countingLevel
	// rule is the rule each path reaches.
	rule []int32
	// always are the rules a path without conditions reaches.
	always []int32

	// What the build collects before finish: the conditions on each ref,
	// interned by table, their values, and how many paths each rule has.
	table       *refTable
	pending     [][]pendingCond
	pendingVals []condValue
	pathsOf     []int32
}

// countingLevel holds what deciding whether to visit a level takes, which a
// lookup asks of every level; the rest, needed once it is visited, is behind
// one pointer.
type countingLevel struct {
	// continuing are the paths with a condition on a level before this one
	// too; starts is whether some path has its first one here.
	continuing pathSet
	starts     bool
	*levelParts
}

type levelParts struct {
	// matcher's leaves list their paths in rules, or name a posting holding
	// them where that takes less room; see trieNode.posting.
	matcher  levelDetail
	postings postings
	// first are the paths starting on the level.
	first pathSet
}

type pendingCond struct {
	lo, hi int32
	path   int32
}

// buildCounting builds i's rules into a counting index.
func (i *baseDocEqIndex) buildCounting(rules []*Rule) {
	c := &countingIndex{table: newRefTable()}
	t := translator{isVirtual: i.isVirtual}
	// Only Build needs it; a reused index mustn't keep its compiler alive.
	i.isVirtual = nil

	i.rules = make([]*Rule, 0, len(rules))
	i.groups = make([]int32, 0, len(rules))
	c.pathsOf = make([]int32, 0, len(rules))
	for idx := range rules {
		WalkRules(rules[idx], func(rule *Rule) bool {
			if rule.Default {
				i.defaultRule = rule
				return false
			}
			if i.onlyGroundRefs {
				i.onlyGroundRefs = rule.Head.Reference.IsGround()
			}
			// Ids ascend with priority within a ruleset; see baseDocEqIndex.rules.
			id := int32(len(i.rules))
			i.rules = append(i.rules, rule)
			i.groups = append(i.groups, int32(idx))

			var p projection
			p.body(t.translate(rule))
			if len(p.memberships) > 0 {
				if i.memberships == nil {
					i.memberships = make(map[int32][]membership, len(rules))
				}
				for j := range p.memberships {
					m := &p.memberships[j]
					m.key = c.table.shared(m.key)
					for k, coll := range m.collections {
						m.collections[k] = c.table.shared(coll)
					}
				}
				i.memberships[id] = p.memberships
			}

			paths := p.paths()
			for _, path := range paths {
				c.insert(path, id)
			}
			c.pathsOf = append(c.pathsOf, int32(len(paths)))
			return false
		})
	}

	c.finish(i)
	i.counting = c
}

// insert adds a path to rule id: one condition per ref it constrains, which
// holds if any of the values the path names for the ref does. A value that can
// be anything gives way to concrete ones: everything on a path has to hold.
func (c *countingIndex) insert(path []projected, id int32) {
	p := int32(len(c.rule))
	c.rule = append(c.rule, id)

	// A path names few refs, however many values: group by going through them.
	var refsArr [8]Ref
	refs := refsArr[:0]
	for _, v := range path {
		if !slices.ContainsFunc(refs, func(r Ref) bool { return RefEqual(r, v.ref) }) {
			refs = append(refs, v.ref)
		}
	}
	c.pendingVals = slices.Grow(c.pendingVals, len(path))
	for _, r := range refs {
		lo := len(c.pendingVals)
		concrete := false
		for _, v := range path {
			if RefEqual(v.ref, r) {
				c.pendingVals = append(c.pendingVals, v.condValue)
				concrete = concrete || !valueIsVar(v.value)
			}
		}
		if values := c.pendingVals[lo:]; len(values) > 1 && concrete {
			c.pendingVals = append(c.pendingVals[:lo], slices.DeleteFunc(values, func(v condValue) bool { return valueIsVar(v.value) })...)
		}
		ref := c.table.intern(r)
		c.pending = growTo(c.pending, int(ref)+1)
		c.pending[ref] = append(c.pending[ref], pendingCond{lo: int32(lo), hi: int32(len(c.pendingVals)), path: p})
	}
}

// unvalued reports whether every condition on the ref only asks for it to be
// defined, which a level cannot narrow a lookup by: the compiler leaves those
// behind when it hoists a term into a local, `__local1__ = data.groups.g0.members`,
// and gather checks them on the candidates instead.
func (c *countingIndex) unvalued(conds []pendingCond) bool {
	for _, pc := range conds {
		if slices.ContainsFunc(c.pendingVals[pc.lo:pc.hi], func(v condValue) bool { return !valueIsVar(v.value) }) {
			return false
		}
	}
	return true
}

// require records, for each rule whose every path reads a ref that is only
// ever asked to be defined, that the ref has to be: Lookup checks that on the
// candidates.
func (c *countingIndex) require(i *baseDocEqIndex, ref refID, conds []pendingCond) {
	reading := map[int32]int32{}
	for _, pc := range conds {
		reading[c.rule[pc.path]]++
	}
	pos := int32(-1)
	for _, id := range slices.Sorted(maps.Keys(reading)) {
		if reading[id] != c.pathsOf[id] {
			continue
		}
		if pos < 0 {
			pos = int32(len(i.requiredRefs))
			i.requiredRefs = append(i.requiredRefs, c.table.ref(ref))
			if i.required == nil {
				i.required = map[int32][]int32{}
			}
		}
		i.required[id] = append(i.required[id], pos)
	}
}

// finish builds the levels, ordered by the share of their paths a value can be
// expected to satisfy, smallest first. A level that one path reaches rules out
// at most that path, so it goes last however selective its value is.
//
// The estimate only knows the rules; it mis-orders where lookups' values come
// up unlike the rules name them. Re-ordering by sampled lookups, and ranking
// levels by paths ruled out rather than by share, measured 453 -> 131 ns/op on
// that shape against ~1% on uniform ones: parked.
func (c *countingIndex) finish(i *baseDocEqIndex) {
	type built struct {
		parts *levelParts
		conds []pendingCond
		share float64
	}
	words := (len(c.rule) + 63) / 64
	// Small indexes, which most are, keep their scratch on the stack.
	var bsArr [4]built
	var bufArr, leavesArr [8]*trieNode
	bs, buf, leaves := bsArr[:0], bufArr[:0], leavesArr[:0]
	tests := make([]bool, len(c.rule))
	for ref, conds := range c.pending {
		if len(conds) == 0 {
			continue
		}
		if c.unvalued(conds) {
			c.require(i, refID(ref), conds)
			continue
		}
		for _, pc := range conds {
			tests[pc.path] = true
		}
		parts := &levelParts{matcher: levelDetail{ref: c.table.ref(refID(ref))}}
		root := &trieNode{next: &parts.matcher}
		leaves = leaves[:0]
		for _, pc := range conds {
			buf = appendLeaves(buf[:0], root, c.pendingVals[pc.lo:pc.hi])
			for _, leaf := range buf {
				if len(leaf.rules) == 0 {
					leaves = append(leaves, leaf)
				}
				leaf.rules = append(leaf.rules, pc.path)
			}
		}
		parts.matcher.compact()
		share := expectedHits(&parts.matcher, leaves) / float64(len(conds))
		for _, leaf := range leaves {
			if k := parts.postings.add(leaf.rules, words); k >= 0 {
				leaf.posting, leaf.rules = k+1, nil
			}
		}
		parts.postings.clip()
		bs = append(bs, built{parts: parts, conds: conds, share: share})
	}
	slices.SortStableFunc(bs, func(a, b built) int {
		if c := cmp.Compare(a.share, b.share); c != 0 {
			return c
		}
		return cmp.Compare(len(b.conds), len(a.conds))
	})

	started := make([]bool, len(c.rule))
	c.levels = make([]countingLevel, 0, len(bs))
	var firstArr, continuingArr [16]int32
	first, continuing := firstArr[:0], continuingArr[:0]
	for _, b := range bs {
		first, continuing = first[:0], continuing[:0]
		for _, pc := range b.conds {
			if started[pc.path] {
				continuing = append(continuing, pc.path)
			} else {
				first = append(first, pc.path)
				started[pc.path] = true
			}
		}
		b.parts.first = newPathSet(first, words)
		c.levels = append(c.levels, countingLevel{
			continuing: newPathSet(continuing, words),
			starts:     len(first) > 0,
			levelParts: b.parts,
		})
	}
	for p, t := range tests {
		if !t {
			c.always = append(c.always, c.rule[p])
		}
	}
	c.table, c.pending, c.pendingVals, c.pathsOf = nil, nil, nil, nil
}

// appendLeaves appends the leaves below root a condition on these values hangs
// off: one leaf for one value, the "any" leaf when one of them can be anything,
// and a leaf per value otherwise.
func appendLeaves(leaves []*trieNode, root *trieNode, values []condValue) []*trieNode {
	d := root.next
	switch {
	case len(values) == 1:
		return append(leaves, insertLeaf(root, d.ref, values[0]))
	case slices.ContainsFunc(values, func(v condValue) bool { return valueIsVar(v.value) }):
		for _, v := range values {
			if v.mapper != nil {
				d.addMapper(v.mapper)
			}
		}
		return append(leaves, root.Insert(d.ref, anyValue, values[0].mapper))
	}
	for _, v := range oneEnd(values, func(v condValue) (Value, affix) { return v.value, v.affix }) {
		leaves = append(leaves, insertLeaf(root, d.ref, v))
	}
	return leaves
}

// expectedHits estimates how many paths a lookup's value satisfies on the level
// whose leaves these are: all of those that accept any value, and of the
// others, as many as a leaf holds on average when values come up as often as
// the rules name them.
func expectedHits(d *levelDetail, leaves []*trieNode) float64 {
	var anyHits, sum, squares float64
	for _, leaf := range leaves {
		n := float64(len(leaf.rules))
		if leaf == d.any {
			anyHits = n
			continue
		}
		sum += n
		squares += n * n
	}
	if sum == 0 {
		return anyHits
	}
	return anyHits + squares/sum
}

// countingLookup is one lookup's scratch: the paths the value of the level being
// walked satisfies, and the paths alive.
type countingLookup struct {
	held  idSet
	alive idSet
	sink  levelSink
}

func (s *countingLookup) reset() {
	s.held.reset()
	s.alive.reset()
}

// levelSink collects into held the paths of the leaves a level's walk arrives
// at, only those in within where it is set.
type levelSink struct {
	held, within *idSet
	postings     *postings
}

func (k *levelSink) Add(leaf *trieNode) {
	if leaf.posting > 0 {
		k.held.orIn(k.postings.get(leaf.posting-1), k.within)
		return
	}
	// Where few paths are alive and the leaf lists many, it is the leaf that is
	// looked up, its ids being in increasing order.
	if k.within != nil && 8*len(k.within.touched) < len(leaf.rules) {
		k.within.each(func(p int32) {
			if _, ok := slices.BinarySearch(leaf.rules, p); ok {
				k.held.set(p)
			}
		})
		return
	}
	for _, p := range leaf.rules {
		if k.within == nil || k.within.has(p) {
			k.held.set(p)
		}
	}
}

// traverse sets in tr the rules the resolver's values reach.
func (c *countingIndex) traverse(resolver ValueResolver, tr *trieTraversalResult, s *countingLookup) error {
	for _, id := range c.always {
		tr.set(id)
	}

	s.held.grow(len(c.rule))
	s.alive.grow(len(c.rule))
	defer s.reset()

	for j := range c.levels {
		l := &c.levels[j]
		starts := l.starts
		if !starts && !s.alive.meets(&l.continuing) {
			continue
		}

		// Where no path starts, only the paths alive can still come to hold.
		s.sink = levelSink{held: &s.held, postings: &l.postings}
		if !starts {
			s.sink.within = &s.alive
		}
		if err := l.matcher.traverse(resolver, &s.sink); err != nil {
			return err
		}

		if starts {
			s.alive.orIn(l.first, &s.held)
		}
		s.alive.removeUnless(l.continuing, &s.held)
		s.held.reset()
	}

	s.alive.each(func(p int32) { tr.set(c.rule[p]) })
	return nil
}

// countingAll sets every rule in tr, and decides early exit over the values of
// complete documents; a value of another kind differing from theirs counts
// against it too.
func (i *baseDocEqIndex) countingAll(tr *trieTraversalResult) {
	var value Value
	for id, rule := range i.rules {
		tr.set(int32(id))
		if rule.Head.DocKind() != CompleteDoc || rule.Head.Value == nil {
			continue
		}
		v := rule.Head.Value.Value
		if !v.IsGround() || value != nil && !ValueEqual(value, v) {
			tr.multiple = true
		}
		value = v
	}
	if value == nil || tr.multiple {
		return
	}
	for _, rule := range i.rules {
		if rule.Head.DocKind() != CompleteDoc && rule.Head.Value != nil && !ValueEqual(value, rule.Head.Value.Value) {
			tr.multiple = true
			return
		}
	}
}
