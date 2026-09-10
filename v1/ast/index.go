// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"cmp"
	"maps"
	"math/bits"
	"slices"
	"strings"
	"sync"

	"github.com/open-policy-agent/opa/v1/util"
)

var (
	globwildcard = VarTerm("$globwildcard")
	skipIndexing = NewSet(NewTerm(Interned.Refs.InternalPrint), NewTerm(Interned.Refs.InternalTestCase))

	// anyValue is a fake variable we used to put "naked ref" expressions
	// into the rule index
	anyValue Value = Var("__any__")
)

type (
	// RuleIndex defines the interface for rule indices.
	RuleIndex interface {
		// Build tries to construct an index for the given rules. If the index was
		// constructed, it returns true, otherwise false.
		Build(rules []*Rule) bool

		// Lookup searches the index for rules that will match the provided
		// resolver. If the resolver returns an error, it is returned via err.
		Lookup(resolver ValueResolver) (*IndexResult, error)

		// AllRules traverses the index and returns all rules that will match
		// the provided resolver without any optimizations (effectively with
		// indexing disabled). If the resolver returns an error, it is returned
		// via err.
		AllRules(resolver ValueResolver) (*IndexResult, error)
	}
	// IndexResult contains the result of an index lookup.
	IndexResult struct {
		Rules          []*Rule
		Else           map[*Rule][]*Rule
		Default        *Rule
		Kind           RuleKind
		EarlyExit      bool
		OnlyGroundRefs bool
	}
	baseDocEqIndex struct {
		isVirtual      func(Ref) bool
		root           *trieNode
		defaultRule    *Rule
		kind           RuleKind
		onlyGroundRefs bool
		// rules holds one entry per rule and else branch the trie carries, groups
		// the position of its ruleset among the rules Build was given. Reading the
		// ids a lookup reached in increasing order groups them and orders each
		// group by priority; see trieTraversalResult and gather.
		rules  []*Rule
		groups []int32
	}
)

// NewIndexResult returns a new IndexResult object.
func NewIndexResult(kind RuleKind) *IndexResult {
	return &IndexResult{Kind: kind}
}

// Empty returns true if there are no rules to evaluate.
func (ir *IndexResult) Empty() bool {
	return len(ir.Rules) == 0 && ir.Default == nil
}

func newBaseDocEqIndex(isVirtual func(Ref) bool) *baseDocEqIndex {
	return &baseDocEqIndex{
		isVirtual:      isVirtual,
		root:           newTrieNodeImpl(),
		onlyGroundRefs: true,
	}
}

func (i *baseDocEqIndex) Build(rules []*Rule) bool {
	if len(rules) == 0 {
		return false
	}

	i.kind = rules[0].Head.RuleKind()
	indices := newrefindices(i.isVirtual, newRefTable())
	values := make(map[Var]Value)

	// build indices for each rule.
	for idx := range rules {
		WalkRules(rules[idx], func(rule *Rule) bool {
			if rule.Default {
				i.defaultRule = rule
				return false
			}
			if i.onlyGroundRefs {
				i.onlyGroundRefs = rule.Head.Reference.IsGround()
			}
			if !bodySkipsIndexing(rule.Body) {
				clear(values)
				for i := range rule.Body {
					indices.Update(rule, rule.Body[i], values)
				}
			}
			return false
		})
	}

	// build trie out of indices.
	levels := indices.Sorted()

	for idx := range rules {
		WalkRules(rules[idx], func(rule *Rule) bool {
			if rule.Default {
				return false
			}

			// Ids are minted in WalkRules' order, so they ascend with priority
			// within a ruleset -- the (insertion, priority) pair a node used to
			// carry, in one integer:
			//
			//	f(x) := 1 if x == "a"  # group 0, id 0
			//	else := 2 if x == "b"  #          id 1
			//	f(x) := 3 if x == "c"  # group 1, id 2
			id := int32(len(i.rules))
			i.rules = append(i.rules, rule)
			i.groups = append(i.groups, int32(idx))

			// Each set of indices the rule can be reached through gets its own
			// path. They share an id, so a lookup arriving at the rule down
			// several of them still reports it once (see trieTraversalResult.Add).
			if len(indices.disjunctions[rule]) == 0 {
				i.insertPath(indices.table, levels, indices.rules[rule], id, rule)
			} else {
				for _, path := range indices.paths(rule) {
					i.insertPath(indices.table, levels, path, id, rule)
				}
			}
			return false
		})
	}

	i.root.compact()

	return true
}

func (i *baseDocEqIndex) insertPath(table *refTable, levels []refID, path []*refindex, id int32, rule *Rule) {
	node := i.root

	// The path stops at the last level it constrains. A rule that constrains
	// nothing below has nothing to test there, so walking on would only pad the
	// path with an "absent" node per remaining level -- a copy of the whole tail
	// that no other rule shares, which is what made a trie of n levels cost n^2
	// nodes to build and to walk. The multiple-scalar case below has always
	// attached mid-trie for the same reason.
	remaining := len(path)

	// One scratch slice for every level, not one per level: a rule's path crosses
	// every level above the last it constrains, most of them constraining nothing.
	var values []*refindex

	for _, level := range levels {
		if remaining == 0 {
			break
		}

		values = values[:0]
		for _, ri := range path {
			if ri.ref == level {
				values = append(values, ri)
			}
		}
		remaining -= len(values)

		ref := table.ref(level)

		// A var value records "this ref can be anything", which a concrete value
		// for the same ref supersedes: everything on one path has to hold, so the
		// concrete value is the stronger of the two constraints. A chain of
		// assignments, `x := input.a; y := x`, leaves one var entry per local
		// behind, and only the first of them is replaced when the concrete value
		// is inserted. Keeping the rest would index the rule under anyValue below
		// and give up all the discrimination the concrete value buys us.
		if len(values) > 1 {
			if concrete := slices.DeleteFunc(slices.Clone(values), (*refindex).isVar); len(concrete) > 0 {
				values = concrete
			}
		}

		if len(values) == 0 {
			node = node.Insert(ref, nil, nil)
		} else if len(values) == 1 {
			node = values[0].insertInto(node, ref)
		} else {
			if slices.ContainsFunc(values, (*refindex).isVar) {
				child := node.Insert(ref, anyValue, values[0].Mapper)
				for i := range values {
					if values[i].Mapper != nil {
						node.next.addMapper(values[i].Mapper)
					}
				}
				node = child
			} else if remaining == 0 || slices.ContainsFunc(values, (*refindex).isAffix) ||
				slices.ContainsFunc(values, (*refindex).isComposite) {
				// Nothing below to continue a path with, so the rule hangs off
				// every alternative -- which rules reaching the same values
				// share. Affixes always take this route; see alternation, and
				// so does anything a converging level could not be keyed on:
				// insertValue sends an object or a set to the "anything" node
				// and an array into the array trie, which is where a lookup
				// goes looking for them.
				for _, val := range oneAffixEnd(values) {
					child := val.insertInto(node, ref)
					child.append(id, rule)
				}
				return
			} else {
				// The alternatives meet again on one node, and the rest of the
				// path is built from there rather than under each of them.
				node = node.insertAlternatives(ref, values)
			}
		}
	}

	node.append(id, rule)
}

func (i *baseDocEqIndex) Lookup(resolver ValueResolver) (*IndexResult, error) {
	tr := ttrPool.Get().(*trieTraversalResult)

	defer func() {
		tr.reset()
		ttrPool.Put(tr)
	}()

	tr.grow(len(i.rules))

	err := i.root.Traverse(resolver, tr)
	if err != nil {
		return nil, err
	}

	result := IndexResultPool.Get()

	result.Kind = i.kind
	result.Default = i.defaultRule
	result.OnlyGroundRefs = i.onlyGroundRefs

	result.Rules = result.Rules[:0]

	clear(result.Else)

	i.gather(tr, result)

	if !tr.multiple {
		// even when the indexer hasn't seen multiple values, the rule itself could be one
		// where early exit shouldn't be applied.
		var lastValue Value
		for i := range result.Rules {
			if result.Rules[i].Head.DocKind() != CompleteDoc {
				tr.multiple = true
				break
			}
			if result.Rules[i].Head.Value != nil {
				if lastValue != nil && !ValueEqual(lastValue, result.Rules[i].Head.Value.Value) {
					tr.multiple = true
					break
				}
				lastValue = result.Rules[i].Head.Value.Value
			}
		}
	}

	result.EarlyExit = !tr.multiple

	return result, nil
}

// gather reads the rules a traversal reached into result. Ids ascend with
// priority, so a run of them sharing a group is that ruleset's definitions in
// order, the first being the one to evaluate.
func (i *baseDocEqIndex) gather(tr *trieTraversalResult, result *IndexResult) {
	var root *Rule
	group := int32(-1)

	// Words are marked as they are first written to, in traversal order.
	slices.Sort(tr.touched)

	found := 0
	for _, w := range tr.touched {
		found += bits.OnesCount64(tr.hits[w])
	}
	result.Rules = slices.Grow(result.Rules, found)

	// A word holds 64 ids: `w<<6` is the id of its first bit, TrailingZeros64 the
	// offset of the lowest set one, and `word &= word - 1` clears it.
	for _, w := range tr.touched {
		for word := tr.hits[w]; word != 0; word &= word - 1 {
			id := w<<6 | int32(bits.TrailingZeros64(word))
			rule := i.rules[id]

			if g := i.groups[id]; g != group {
				group, root = g, rule
				result.Rules = append(result.Rules, rule)
				continue
			}

			if result.Else == nil {
				result.Else = map[*Rule][]*Rule{}
			}
			result.Else[root] = append(result.Else[root], rule)
		}
	}
}

func (i *baseDocEqIndex) AllRules(ValueResolver) (*IndexResult, error) {
	tr := newTrieTraversalResult()
	tr.grow(len(i.rules))

	// Walk over the rule trie and accumulate _all_ rules
	rw := &ruleWalker{result: tr}
	i.root.Do(rw)

	result := NewIndexResult(i.kind)
	result.Default = i.defaultRule
	result.OnlyGroundRefs = i.onlyGroundRefs
	i.gather(tr, result)

	result.EarlyExit = !tr.multiple

	return result, nil
}

type ruleWalker struct {
	result *trieTraversalResult
}

func (r *ruleWalker) Do(x any) trieWalker {
	tn := x.(*trieNode)
	r.result.Add(tn)
	return r
}

type valueMapper struct {
	Key      string
	MapValue func(Value) Value
}

// refID identifies one of the references an index is built on.
type refID int32

// refTable numbers the references an index is built on. One table is shared by
// every refindices of a build, the scratch ones an `and`/`or` operand is
// indexed into included, so that an id means the same thing wherever it turns
// up.
type refTable struct {
	// refs are the references in id order; ids answers the other direction, and
	// is only built past refTableScan entries.
	refs []Ref
	ids  *util.HasherMap[Ref, refID]
}

// refTableScan is how many references a table holds before it builds a map:
// below that, comparing a ref to the few already here beats hashing it, and most
// rulesets are indexed on a handful.
const refTableScan = 8

func newRefTable() *refTable {
	return &refTable{}
}

func (t *refTable) intern(ref Ref) refID {
	if t.ids == nil {
		for id, other := range t.refs {
			if RefEqual(other, ref) {
				return refID(id)
			}
		}
		if len(t.refs) < refTableScan {
			t.refs = append(t.refs, ref)
			return refID(len(t.refs) - 1)
		}
		t.ids = util.NewHasherMap[Ref, refID](RefEqual)
		for id, other := range t.refs {
			t.ids.Put(other, refID(id))
		}
	}

	if id, ok := t.ids.Get(ref); ok {
		return id
	}
	id := refID(len(t.refs))
	t.refs = append(t.refs, ref)
	t.ids.Put(ref, id)
	return id
}

func (t *refTable) ref(id refID) Ref {
	return t.refs[id]
}

type refindex struct {
	Value  Value
	Mapper *valueMapper
	// ref is the reference this constrains, as numbered by the build's table.
	ref refID
	// Affix says whether Value is a string the value at ref has to start or end
	// with, rather than one it has to equal -- what startswith, endswith and
	// their strings.any_*_match forms contribute. Several of them for one ref
	// are alternatives, as for `in`.
	Affix affix
}

// affix is which end of the value at a reference a refindex constrains, if it
// constrains an end rather than the whole of it.
type affix uint8

const (
	affixNone affix = iota
	affixPrefix
	affixSuffix
)

// insertInto adds the level this index constrains to the path being built,
// returning the node the rest of the path continues from.
func (i *refindex) insertInto(node *trieNode, ref Ref) *trieNode {
	switch i.Affix {
	case affixPrefix:
		return node.InsertPrefix(ref, i.Value)
	case affixSuffix:
		return node.InsertSuffix(ref, i.Value)
	}
	return node.Insert(ref, i.Value, i.Mapper)
}

// oneAffixEnd keeps the affixes of one end of the value where values constrain
// both, and everything that is not an affix.
//
// A rule hung off the leaves of both the prefix and the suffix trie is admitted
// by either, which is the disjunction of what it wrote where it wrote a
// conjunction:
//
//	p if {
//		strings.any_prefix_match(input.path, ["/a", "/b"])
//		strings.any_suffix_match(input.path, [".go", ".rego"])
//	}
//
// admits "/c/x.go" on the suffix alone. Testing one end and leaving the other to
// evaluation admits a subset of that -- what one end admits, both admit -- so
// one end is kept. A level cannot test both: the tries hold leaves, and a leaf
// cannot be made to depend on another trie's answer.
//
// Which end is kept is decided by the shortest base string of each, since a set
// admits a value that matches any one of its bases and the shortest of them
// admits the most. Counting them instead would keep ["/"] over [".go",
// ".rego"], and every absolute path matches "/".
func oneAffixEnd(values []*refindex) []*refindex {
	prefix, suffix := -1, -1
	for _, val := range values {
		s, ok := val.Value.(String)
		if !ok {
			continue
		}
		switch val.Affix {
		case affixPrefix:
			if prefix < 0 || len(s) < prefix {
				prefix = len(s)
			}
		case affixSuffix:
			if suffix < 0 || len(s) < suffix {
				suffix = len(s)
			}
		}
	}

	if prefix < 0 || suffix < 0 {
		return values
	}

	drop := affixSuffix
	if suffix > prefix {
		drop = affixPrefix
	}

	return slices.DeleteFunc(slices.Clone(values), func(val *refindex) bool {
		return val.Affix == drop
	})
}

// alternatives are sets of indices, any one of which is enough to reach a rule.
type alternatives = [][]*refindex

type refindices struct {
	isVirtual func(Ref) bool
	rules     map[*Rule][]*refindex
	// disjunctions holds the alternatives contributed by each `or` in the rule;
	// every combination of them is a way to reach it.
	disjunctions map[*Rule][]alternatives
	// outer holds the enclosing scope's indices when this is the scratch for an
	// operand body: resolvable from inside, but not the operand's own.
	outer []*refindex
	table *refTable
	// stats holds what Sorted ranks the references by, indexed by ref id.
	stats  []refStats
	sorted []refID
}

// refStats is what one reference accumulated over a build, which is what decides
// the order of the trie's levels. Dropped once the trie is built.
type refStats struct {
	// count is how often the ref took part in indexing a rule. Sorted passes
	// over the ids that never counted: a scratch interns the refs of an operand
	// that may turn out unindexable, and then nothing records them.
	count int32
	// alternated is whether some rule reaches the ref by more than one value,
	// and what that costs insertPath. An `or` is not recorded: its alternatives
	// are separate paths, and only meet a second value for one ref once paths()
	// combines them, after Sorted has run.
	alternated alternation
}

// maxIndexPaths caps the ways a single rule may be reached: `or` expressions
// multiply out (`{a or b} and {c or d}` is four), and at some point the trie
// nodes cost more than evaluating the rule.
const maxIndexPaths = 32

func newrefindices(isVirtual func(Ref) bool, table *refTable) *refindices {
	return &refindices{
		isVirtual: isVirtual,
		table:     table,
		rules:     map[*Rule][]*refindex{},
	}
}

// growTo extends s so that it can be indexed by every id below n, leaving what
// it already holds in place.
func growTo[T any](s []T, n int) []T {
	if len(s) >= n {
		return s
	}
	return append(s, make([]T, n-len(s))...)
}

func valueIsVar(v Value) bool {
	_, ok := v.(Var)
	return ok
}

func (i *refindex) isVar() bool {
	return valueIsVar(i.Value)
}

func (i *refindex) isAffix() bool {
	return i.Affix != affixNone
}

// isComposite reports whether a lookup could not find this value among a
// level's alternatives, which are keyed on the value as it stands. insertValue
// sends an object or a set to the "anything" node and an array into the array
// trie, which is where a lookup goes looking for them instead.
func (i *refindex) isComposite() bool {
	return !IsScalar(i.Value)
}

// Update attempts to update the refindices for the given expression in the
// given rule. If the expression cannot be indexed the update does not affect
// the indices.
func (i *refindices) Update(rule *Rule, expr *Expr, values map[Var]Value) {
	if len(expr.With) > 0 {
		// NOTE(tsandall): In the future, we may need to consider expressions
		// that have with statements applied to them.
		return
	}

	if expr.Negated {
		// NOTE(sr): We could try to cover simple expressions, like
		// not input.funky => input.funky == false or undefined (two refindex?)
		return
	}

	switch terms := expr.Terms.(type) {
	case *LogicalAnd:
		i.updateLogicalAnd(rule, terms, values)
		return
	case *LogicalOr:
		i.updateLogicalOr(rule, terms, values)
		return
	}

	op := expr.Operator()
	if op == nil {
		if ts, ok := expr.Terms.(*Term); ok {
			// NOTE(sr): If we wanted to cover function args, we'd need to also
			// check for type "Var" here. But since it's impossible to call a
			// function with a undefined argument, there's no point to recording
			// "needs to be anything" for function args
			if _, ok := ts.Value.(Ref); ok { // "naked ref"
				i.updateEq(rule, ts.Value, anyValue, nil)
			}
		}
	}

	equalish := op.Equal(Interned.Refs.Equality) || // unification, no 3-operands version exists
		// NOTE(tsandall): if equal() is called with more than two arguments the
		// output value is being captured in which case the indexer cannot
		// exclude the rule if the equal() call would return false (because the
		// false value must still be produced.)
		(op.Equal(Interned.Refs.Equal) && len(expr.Operands()) == 2)

	a, b := expr.Operand(0), expr.Operand(1)
	switch {
	case equalish:
		if !i.updateEqWildcardRef(rule, a.Value, b.Value, values) {
			i.updateEq(rule, a.Value, b.Value, values)
		}

	case op.Equal(Interned.Refs.GlobMatch) && len(expr.Operands()) == 3:
		// NOTE(sr): Same as with equal() above -- 4 operands means the output
		// of `glob.match` is captured and the rule can thus not be excluded.
		i.updateGlobMatch(rule, expr)

	case op.Equal(Interned.Refs.Member) && len(expr.Operands()) == 2:
		// NOTE(sr): Again, 3 operands means captured output (like above).
		i.updateMember(rule, expr, values)

	case op.Equal(Interned.Refs.StartsWith) && len(expr.Operands()) == 2:
		// As with equal() above: a third operand captures the result, and a
		// rule producing `false` still has to be evaluated.
		i.updateAffix(rule, expr, values, affixPrefix)

	case op.Equal(Interned.Refs.AnyPrefixMatch) && len(expr.Operands()) == 2:
		i.updateAnyAffixMatch(rule, expr, values, affixPrefix)

	case op.Equal(Interned.Refs.EndsWith) && len(expr.Operands()) == 2:
		i.updateAffix(rule, expr, values, affixSuffix)

	case op.Equal(Interned.Refs.AnySuffixMatch) && len(expr.Operands()) == 2:
		i.updateAnyAffixMatch(rule, expr, values, affixSuffix)
	}
}

// updateLogicalAnd folds both operands of a conjunction into the rule's
// indices: `lhs and rhs` only succeeds if both operands do, so whatever either
// operand requires of the input, the rule requires.
//
// Each operand is indexed against the indices the rule has so far, not against
// what its sibling contributes: operand bodies are separate scopes, so the same
// var in each is a different var, and resolveVarToRef must not connect them.
func (i *refindices) updateLogicalAnd(rule *Rule, and *LogicalAnd, values map[Var]Value) {
	lhs := i.operandAlternatives(rule, and.Lhs, values)
	rhs := i.operandAlternatives(rule, and.Rhs, values)

	i.require(rule, lhs)
	i.require(rule, rhs)
}

// require records that the rule is only defined if one of the alternatives
// holds. A lone alternative is unconditional, so its indices join the rule's
// own; several are kept apart for Build to turn into separate paths.
func (i *refindices) require(rule *Rule, alts alternatives) {
	switch len(alts) {
	case 0:
		return
	case 1:
		for _, ri := range alts[0] {
			i.insert(rule, ri)
		}
	default:
		for _, alt := range alts {
			for _, ri := range alt {
				i.count(ri.ref)
			}
		}
		if i.disjunctions == nil {
			i.disjunctions = map[*Rule][]alternatives{}
		}
		i.disjunctions[rule] = append(i.disjunctions[rule], alts)
	}
}

// updateLogicalOr records the operands of a disjunction as alternative ways to
// reach the rule, `lhs or rhs` holding if either operand does. An operand
// nothing can be indexed on could be satisfied by any input at all, which
// leaves the disjunction saying nothing about the rule.
func (i *refindices) updateLogicalOr(rule *Rule, or *LogicalOr, values map[Var]Value) {
	lhs := i.operandAlternatives(rule, or.Lhs, values)
	if len(lhs) == 0 {
		return
	}

	rhs := i.operandAlternatives(rule, or.Rhs, values)
	if len(rhs) == 0 {
		return
	}

	i.require(rule, slices.Concat(lhs, rhs))
}

// operandAlternatives returns the ways the body of an `and`/`or` operand can be
// satisfied; an operand with an `or` of its own has one per branch, and none at
// all means nothing about it could be indexed. It is indexed into a scratch, so
// that what it requires reaches the rule only through require().
func (i *refindices) operandAlternatives(rule *Rule, body Body, values map[Var]Value) alternatives {
	scratch := newrefindices(i.isVirtual, i.table)
	scratch.outer = append(slices.Clone(i.rules[rule]), i.outer...)
	scratch.updateOperand(rule, body, values)

	alts := scratch.paths(rule)
	if len(alts) == 1 && len(alts[0]) == 0 {
		return nil
	}

	for _, alt := range alts {
		for pos, ri := range alt {
			// The var is scoped to the operand body and must not become
			// resolvable from the outside (see resolveVarToRef); that the ref
			// has to be defined still holds.
			if ri.isVar() {
				alt[pos] = &refindex{ref: ri.ref, Value: anyValue, Mapper: ri.Mapper}
			}
		}
	}

	return alts
}

// paths returns every set of indices that can lead to the rule: the ones that
// always hold, combined with one branch from each disjunction. Past
// maxIndexPaths the disjunctions are dropped -- fewer constraints only widen
// what the index admits, so the result stays correct.
func (i *refindices) paths(rule *Rule) alternatives {
	unconditional := i.rules[rule]
	paths := alternatives{unconditional}

	for _, alts := range i.disjunctions[rule] {
		if len(paths)*len(alts) > maxIndexPaths {
			return alternatives{unconditional}
		}

		combined := make(alternatives, 0, len(paths)*len(alts))
		for _, path := range paths {
			for _, alt := range alts {
				combined = append(combined, append(slices.Clone(path), alt...))
			}
		}
		paths = combined
	}

	return paths
}

// updateOperand folds the expressions of an `and`/`or` operand body into the
// rule's indices. An operand body is a closed scope -- bindings made inside it
// reach neither the enclosing body nor the sibling operand (see
// evalLogicalOperand in topdown) -- so its constants are copied in and dropped
// on return.
func (i *refindices) updateOperand(rule *Rule, body Body, values map[Var]Value) {
	scoped := make(map[Var]Value, len(values))
	maps.Copy(scoped, values)

	for _, expr := range body {
		i.Update(rule, expr, scoped)
	}
}

func (i *refindices) isValidIndexRef(ref Ref) bool {
	// NB(sr): the ordering is intentional, cheapest-first
	return RootDocumentNames.Contains(ref[0]) &&
		!ref.IsNested() &&
		ref.IsGround() &&
		!i.isVirtual(ref)
}

// Sorted returns the references the indices were built from, ordered so that
// the ones appearing in more of the indexed rules come first.
func (i *refindices) Sorted() []refID {
	if i.sorted != nil {
		return i.sorted
	}

	for id, stats := range i.stats {
		if stats.count > 0 {
			i.sorted = append(i.sorted, refID(id))
		}
	}

	slices.SortFunc(i.sorted, func(a, b refID) int {
		// A ref reached by several values is worth less as an early level,
		// and one that ends the rule's path less again, however often
		// either was recorded -- so both outrank frequency.
		if c := cmp.Compare(i.stats[a].alternated, i.stats[b].alternated); c != 0 {
			return c
		}
		if c := cmp.Compare(i.stats[b].count, i.stats[a].count); c != 0 { // descending
			return c
		}
		if c := i.table.ref(a)[0].Loc().Compare(i.table.ref(b)[0].Loc()); c != 0 {
			return c
		}
		// Refs built rather than parsed -- a function's args[n] -- share a
		// location, so fall back on the order they were first seen in.
		return cmp.Compare(a, b)
	})

	return i.sorted
}

func (i *refindices) updateEq(rule *Rule, a, b Value, constants map[Var]Value) {
	args := rule.Head.Args
	if !i.eqOperandsToRefAndValue(rule, args, a, b, constants) {
		i.eqOperandsToRefAndValue(rule, args, b, a, constants)
	}
}

func (i *refindices) updateEqWildcardRef(rule *Rule, a, b Value, constants map[Var]Value) bool {
	return i.tryIndexWildcardRef(rule, a, b, constants) ||
		i.tryIndexWildcardRef(rule, b, a, constants)
}

func (i *refindices) tryIndexWildcardRef(rule *Rule, a, b Value, constants map[Var]Value) bool {
	ref, ok := a.(Ref)
	if !ok {
		return false
	}

	ref = i.resolveRefHead(rule, rule.Head.Args, ref)
	if ref == nil {
		return false
	}

	groundPrefix := ref.GroundPrefix()
	if len(groundPrefix) != len(ref)-1 || !i.isValidIndexRef(groundPrefix) {
		return false
	}

	resolvedValue := b
	if bvar, ok := b.(Var); ok {
		if resolved, ok := constants[bvar]; ok {
			resolvedValue = resolved
		}
	} else if val, ok := indexValue(b); ok {
		resolvedValue = val
	} else {
		return false
	}

	if !IsScalar(resolvedValue) {
		return false
	}

	i.insert(rule, &refindex{ref: i.table.intern(groundPrefix), Value: resolvedValue})
	return true
}

func (i *refindices) updateGlobMatch(rule *Rule, expr *Expr) {
	args := rule.Head.Args

	delim, ok := globDelimiterToString(expr.Operand(1))
	if !ok {
		return
	}

	if arr := globPatternToArray(expr.Operand(0), delim); arr != nil {
		// The 3rd operand of glob.match is the value to match. We assume the
		// 3rd operand was a reference that has been rewritten and bound to a
		// variable earlier in the query OR a function argument variable.
		match := expr.Operand(2)
		if v, ok := match.Value.(Var); ok {
			if ref := i.resolveVarToRef(i.resolvable(rule), args, v); ref != nil {
				i.insert(rule, &refindex{
					ref:   i.table.intern(ref),
					Value: arr.Value,
					Mapper: &valueMapper{
						Key: delim,
						MapValue: func(v Value) Value {
							if s, ok := v.(String); ok {
								return stringSliceToArray(splitStringEscaped(string(s), delim))
							}
							return v
						},
					},
				})
			}
		}
	}
}

func (i *refindices) updateMember(rule *Rule, expr *Expr, constants map[Var]Value) {
	lhs, rhs := expr.Operand(0), expr.Operand(1)
	lvar, ok := lhs.Value.(Var)
	if ok {
		lref := i.resolveVarToRef(i.resolvable(rule), rule.Head.Args, lvar)
		if lref != nil {
			i.updateMemberRefInValue(rule, lref, rhs, constants) // `ref in value`
			return
		}
	}

	// `var0 in var1` case (var0 may be constant, var1 ref)
	i.updateMemberValueInRef(rule, rule.Head.Args, lhs.Value, rhs, constants)
}

func (i *refindices) updateMemberValueInRef(rule *Rule, args []*Term, lval Value, rhs *Term, constants map[Var]Value) {
	if lvar, ok := lval.(Var); ok {
		val, ok := constants[lvar]
		if ok {
			lval = val
		}
	} else if !IsScalar(lval) {
		return
	}

	rref := i.resolveAndValidateRef(rule, args, rhs)
	if rref == nil {
		return
	}

	i.insert(rule, &refindex{ref: i.table.intern(rref), Value: lval})
}

func (i *refindices) updateMemberRefInValue(rule *Rule, ref Ref, rhs *Term, constants map[Var]Value) {
	rval := rhs.Value
	if rvar, ok := rval.(Var); ok { // rhs is var, try to resolve
		if resolved, ok := constants[rvar]; ok {
			rval = resolved
		}
	}

	var (
		forEach func(func(*Term))
		n       int
	)

	switch rcol := rval.(type) {
	case *Array:
		forEach, n = rcol.Foreach, rcol.Len()
	case Set:
		forEach, n = rcol.Foreach, rcol.Len()
	case Object:
		n = rcol.Len()
		forEach = func(f func(*Term)) {
			rcol.Foreach(func(_, v *Term) { f(v) })
		}
	default:
		return
	}

	members := make([]Value, 0, n)
	forEach(func(t *Term) {
		members = append(members, t.Value)
	})

	i.insertMembers(rule, ref, members)
}

// insertMembers records the members of an `in` collection, each a value the
// rule may reach ref by, hoisting insert's scan out of the loop. insertAffixes
// is the same for base strings; the two dedup on different key types.
func (i *refindices) insertMembers(rule *Rule, ref Ref, members []Value) {
	id := i.table.intern(ref)

	if len(members) < 2 {
		for _, member := range members {
			i.insert(rule, &refindex{ref: id, Value: member})
		}
		return
	}

	// Unlike a prefix, a concrete member takes the place of a "reference is
	// anything" entry (see insert), so the first one goes the ordinary way --
	// the rule's list is short at that point, so the scan it costs is cheap.
	i.insert(rule, &refindex{ref: id, Value: members[0]})

	// insert is the only one that may put a value somewhere other than the end
	// of the list, which is what a var needs, so those go in through it and are
	// left out of the block below. A collection holding one is rare, and paying
	// a copy for it keeps the common case a single pass.
	rest := members[1:]
	if slices.ContainsFunc(rest, valueIsVar) {
		for _, member := range rest {
			if valueIsVar(member) {
				i.insert(rule, &refindex{ref: id, Value: member})
			}
		}
		rest = slices.DeleteFunc(slices.Clone(rest), valueIsVar)
	}

	concrete := 0
	seen := util.NewHasherMap[Value, struct{}](ValueEqual)

	for _, other := range i.rules[rule] {
		if other.ref != id {
			continue
		}
		if !other.isVar() {
			concrete++
		}
		if other.Affix == affixNone {
			seen.Put(other.Value, struct{}{})
		}
	}

	// One refindex per member, laid down in a single block rather than
	// allocated one at a time, as in insertAffixes. Duplicates leave slack at
	// the end of the block, which the reslice drops.
	pos := len(i.rules[rule])
	indices := util.GrowPtrSlice(i.rules[rule], len(rest))

	for _, member := range rest {
		if _, ok := seen.Get(member); ok {
			continue
		}
		seen.Put(member, struct{}{})
		concrete++

		*indices[pos] = refindex{ref: id, Value: member}
		pos++
	}
	i.rules[rule] = indices[:pos]

	i.countN(id, len(rest))

	if concrete > 1 {
		i.alternate(id, alternationConverging)
	}
}

func (i *refindices) resolveAndValidateRef(rule *Rule, args []*Term, term *Term) Ref {
	var ref Ref
	switch v := term.Value.(type) {
	case Ref:
		ref = v
	case Var:
		ref = i.resolveVarToRef(i.resolvable(rule), args, v)
	default:
		return nil
	}

	if ref == nil || !i.isValidIndexRef(ref) {
		return nil
	}

	return ref
}

// resolveRefHead resolves a ref rooted at a local variable -- what
//
//	x := input
//	x.foo == "bar"
//
// gets compiled to -- into the ref that local aliases, splicing the remainder of
// the ref onto it: `input.foo`. Refs that are already rooted at a root document
// are returned unchanged; a head that does not resolve yields nil.
func (i *refindices) resolveRefHead(rule *Rule, args []*Term, ref Ref) Ref {
	head, isVar := ref[0].Value.(Var)
	if !isVar || RootDocumentNames.Contains(ref[0]) {
		return ref
	}

	resolved := i.resolveVarToRef(i.resolvable(rule), args, head)
	if resolved == nil {
		return nil
	}

	return resolved.Concat(ref[1:])
}

// resolveVarToRef checks the previously prepared `*refindex` slice for
// occurrences of the var `v`. Since we store `ref = var` expressions for
// "any" lookups (i.e. "return the rule if ref is anything"), we can
// resolve vars to refs in these simple cases:
//
//	__local2__ = input.foo
//	__local2__ = <something>
//
// This what builtin calls involving refs are rewritten to, so it is used
// for var -> ref lookup when buiding the RI for glob.match or `v in col`.
//
// For convenience, we also resolve function arg vars here.
//
// NB: This also covers explicit var assignments, like `role := input.rule`,
// but it is no help with chains of assignments, like
//
//	x := input.role
//	y := x
//	<something with x>
//
// as we're not capturing `var = var` expressions in the index.
func (i *refindices) resolveVarToRef(ri []*refindex, args []*Term, v Var) Ref {
	for _, other := range ri {
		if v.Equal(other.Value) {
			return i.table.ref(other.ref)
		}
	}
	for j, arg := range args {
		if v.Equal(arg.Value) {
			return Ref{FunctionArgRootDocument, InternedTerm(j)}
		}
	}

	return nil
}

// resolvable returns the indices a var here can be resolved against: the rule's
// own, plus those of any scope enclosing an operand body.
func (i *refindices) resolvable(rule *Rule) []*refindex {
	if len(i.outer) == 0 {
		return i.rules[rule]
	}
	return append(slices.Clone(i.rules[rule]), i.outer...)
}

// count records that ref took part in indexing a rule, which is what orders the
// trie levels (see Sorted).
func (i *refindices) count(ref refID) {
	i.countN(ref, 1)
}

func (i *refindices) countN(ref refID, n int) {
	i.stat(ref).count += int32(n)
}

// stat returns the reference's statistics, making room for them if this is the
// first thing recorded about it.
func (i *refindices) stat(ref refID) *refStats {
	i.stats = growTo(i.stats, int(ref)+1)
	return &i.stats[ref]
}

// alternation is what a ref reached by several values costs the rest of the
// rule's path, and what Sorted ranks such refs by.
type alternation uint8

const (
	// alternationNone: no rule reaches the ref by more than one value.
	alternationNone alternation = iota

	// alternationConverging: the alternatives meet again on one node, so the
	// path continues from there. Still ranked after the plain refs, since the
	// rule gets a node of its own and stops sharing what is below.
	alternationConverging

	// alternationTerminal: the alternatives cannot meet again, so the rule
	// hangs off each and whatever it constrains below goes unindexed. Affixes
	// are these -- a prefix trie cannot point several leaves at one node.
	alternationTerminal
)

// alternate records that a rule reaches ref by more than one value, and what
// that costs. The worse kind recorded for a ref wins. Only the values
// surviving insertPath's var-stripping count.
func (i *refindices) alternate(ref refID, kind alternation) {
	if kind == alternationNone {
		return
	}

	stats := i.stat(ref)
	stats.alternated = max(stats.alternated, kind)
}

func (i *refindices) insert(rule *Rule, index *refindex) {
	i.count(index.ref)

	indexValueIsVar := index.isVar()

	for pos, other := range i.rules[rule] {
		if other.ref == index.ref {
			if other.Affix == index.Affix && ValueEqual(other.Value, index.Value) {
				return
			}
			otherValueIsVar := other.isVar()
			// An affix constraint does not take the place of the "ref is
			// anything" entry the way a concrete value does: that entry is what
			// lets a later expression resolve the same local back to this ref
			// (see resolveVarToRef), and insertPath drops it anyway once the
			// ref has a concrete value on the path.
			if !indexValueIsVar && index.Affix == affixNone && otherValueIsVar {
				i.rules[rule][pos] = index
				return
			}
			if !indexValueIsVar && !otherValueIsVar {
				// insertPath cannot converge a level that any affix reaches,
				// so one on either side makes this pair a terminal one.
				kind := alternationConverging
				if index.Affix != affixNone || other.Affix != affixNone {
					kind = alternationTerminal
				}
				i.alternate(index.ref, kind)
			}
		}
	}

	i.rules[rule] = append(i.rules[rule], index)
}

type trieWalker interface {
	Do(any) trieWalker
}

// trieTraversalResult is what a walk of the trie -- a lookup, or the whole of it
// for AllRules -- collects.
//
// The rules reached are a bitset over the index's rule ids, so reaching one down
// several paths costs nothing to notice: the second arrival writes a bit that is
// already set. Reading it back in id order is reading it grouped and in priority
// order, ids having been handed out that way, so there is nothing left to sort.
type trieTraversalResult struct {
	hits []uint64
	// touched holds the words of hits that were written to, so that clearing
	// costs what a lookup found rather than what the index holds.
	touched  []int32
	exist    *Term
	multiple bool
}

var ttrPool = &sync.Pool{
	New: func() any {
		return newTrieTraversalResult()
	},
}

func newTrieTraversalResult() *trieTraversalResult {
	return &trieTraversalResult{}
}

// grow makes room for an index holding n rules. The pool never sizes back down,
// which is a byte per eight rules against an index costing hundreds per rule.
func (tr *trieTraversalResult) grow(n int) {
	tr.hits = growTo(tr.hits, (n+63)/64)
}

func (tr *trieTraversalResult) reset() {
	for _, w := range tr.touched {
		tr.hits[w] = 0
	}
	tr.touched = tr.touched[:0]
	tr.multiple = false
	tr.exist = nil
}

func (tr *trieTraversalResult) Add(t *trieNode) {
	for _, id := range t.rules {
		word, bit := id>>6, uint64(1)<<(uint(id)&63)
		if tr.hits[word]&bit != 0 {
			continue
		}
		if tr.hits[word] == 0 {
			tr.touched = append(tr.touched, word)
		}
		tr.hits[word] |= bit
	}
	if t.multiple {
		tr.multiple = true
	}
	if tr.multiple || t.value == nil {
		return
	}
	if t.value.IsGround() && tr.exist == nil || tr.exist.Equal(t.value) {
		tr.exist = t.value
		return
	}
	tr.multiple = true
}

type trieNode struct {
	// next is the level below this node, nil where the paths under it end.
	next *levelDetail
	// rules are the ids of the rules whose path ends here, see
	// baseDocEqIndex.rules.
	rules    []int32
	value    *Term
	multiple bool
}

func (node *trieNode) append(id int32, rule *Rule) {
	node.rules = append(node.rules, id)

	if node.value != nil && rule.Head.Value != nil && !node.value.Equal(rule.Head.Value) {
		node.multiple = true
	}

	if node.value == nil && rule.Head.DocKind() == CompleteDoc {
		node.value = rule.Head.Value
	}
}

// levelDetail is everything a trieNode has by virtue of being a *level* -- the
// reference it resolves, the children it dispatches the resolved value to, and
// the constraints that are not exact values. The suffix trie holds its base
// strings reversed, so that requiring one at the end of a value is requiring it
// at the start of the value reversed and the same trie answers both (see
// traverseSuffix).
//
// It is held behind one pointer because a trieNode is allocated per indexed
// value and almost none of them are levels: half a million prefixes make one
// level and half a million nodes that only carry rules. Measured over such an
// index, every field here is set on 0 or 1 of the 500002 nodes.
//
// Where the boundaries fall decides how much that is worth. Inline, these
// fields put trieNode in Go's 160-byte size class; out of line it is 56 bytes,
// which rounds to 64. Moving them out a few at a time buys nothing -- 136 and
// 112 bytes both round up to a class the struct already occupied.
//
// The same reasoning applies once more within levelDetail: alternatives is set
// on the few levels some rule reaches by more than one value, so it costs 8
// bytes here rather than the 32 its two fields would inline.
type levelDetail struct {
	ref          Ref
	any          *trieNode
	undefined    *trieNode
	array        *arrayTrie
	scalars      *util.HasherMap[Value, *trieNode]
	mappers      []*valueMapper
	prefixes     *prefixTrie
	suffixes     *prefixTrie
	alternatives *alternativeChildren
}

// alternativeChildren are the nodes that rules reaching a level by several
// values continue from. The two fields hold the same nodes for two different
// jobs, and neither does the other's:
//
// members answers "which nodes does this value reach", which is what a lookup
// asks. A node is in it under every one of the values that reaches it, so a
// rule with a thousand-member collection puts its one node under a thousand
// keys, and several rules sharing a value put several nodes under that one.
//
// converged answers "which nodes are below this level", which is what the
// walks over the whole trie ask -- traverseUnknown, Do and compact. Reading
// that off members would visit a node once per value that reaches it: correct,
// since trieTraversalResult.Add folds a rule reached twice into one, but a
// thousand times the work for the collection above. So the nodes are listed
// once each here as they are created.
type alternativeChildren struct {
	members   *util.HasherMap[Value, []*trieNode]
	converged []*trieNode
}

func newTrieNodeImpl() *trieNode {
	return &trieNode{}
}

// level returns the level below node, creating it if this is the first rule to
// be discriminated there.
func (node *trieNode) level() *levelDetail {
	node.next = util.Or(node.next, newLevelDetail)
	return node.next
}

func (d *levelDetail) converged() []*trieNode {
	if d != nil && d.alternatives != nil {
		return d.alternatives.converged
	}
	return nil
}

// affixTrie returns the trie for one end of the value, creating it and the
// detail that holds it on first use.
func (d *levelDetail) affixTrie(a affix) *prefixTrie {
	detail := d

	switch a {
	case affixSuffix:
		detail.suffixes = util.Or(detail.suffixes, newPrefixTrie)
		return detail.suffixes
	default:
		detail.prefixes = util.Or(detail.prefixes, newPrefixTrie)
		return detail.prefixes
	}
}

func newLevelDetail() *levelDetail {
	return &levelDetail{}
}

func newScalarChildren() *util.HasherMap[Value, *trieNode] {
	return util.NewHasherMap[Value, *trieNode](ValueEqual)
}

func newPrefixTrie() *prefixTrie {
	return &prefixTrie{}
}

func (node *trieNode) Do(walker trieWalker) {
	if node == nil {
		return
	}
	next := walker.Do(node)
	if next == nil {
		return
	}

	node.next.do(next)
}

func (d *levelDetail) do(walker trieWalker) {
	if d == nil {
		return
	}

	d.any.Do(walker)
	d.undefined.Do(walker)

	d.scalars.Iter(func(_ Value, child *trieNode) bool {
		child.Do(walker)
		return false
	})

	for _, child := range d.converged() {
		child.Do(walker)
	}

	d.prefixes.do(walker)
	d.suffixes.do(walker)
	d.array.do(walker)
}

// compact walks the trie once the index is built and releases what its slices
// grew but do not use.
func (node *trieNode) compact() {
	if node == nil {
		return
	}

	node.next.compact()
}

func (d *levelDetail) compact() {
	if d == nil {
		return
	}

	d.prefixes.compact()
	d.suffixes.compact()

	d.any.compact()
	d.undefined.compact()
	d.array.compact()

	for _, child := range d.converged() {
		child.compact()
	}

	d.scalars.Iter(func(_ Value, child *trieNode) bool {
		child.compact()
		return false
	})

	if d.alternatives != nil {
		d.alternatives.converged = slices.Clip(d.alternatives.converged)
	}
}

// insertAlternatives adds a level a rule reaches by any one of several values,
// and returns the one node the rest of its path continues from. Every value
// keys to that node, so what the rule constrains below is built once instead of
// repeated under each alternative.
func (node *trieNode) insertAlternatives(ref Ref, values []*refindex) *trieNode {
	level := node.level()
	level.ref = ref
	level.alternatives = util.Or(level.alternatives, newAlternativeChildren)
	alt := level.alternatives

	converge := newTrieNodeImpl()
	alt.converged = append(alt.converged, converge)

	for _, val := range values {
		if val.Mapper != nil {
			level.addMapper(val.Mapper)
		}
		nodes, _ := alt.members.Get(val.Value)
		alt.members.Put(val.Value, append(nodes, converge))
	}

	return converge
}

func newAlternativeChildren() *alternativeChildren {
	return &alternativeChildren{
		members: util.NewHasherMap[Value, []*trieNode](ValueEqual),
	}
}

func (node *trieNode) Insert(ref Ref, value Value, mapper *valueMapper) *trieNode {
	level := node.level()
	level.ref = ref

	if mapper != nil {
		level.addMapper(mapper)
	}

	return level.insertValue(value)
}

func (node *trieNode) Traverse(resolver ValueResolver, tr *trieTraversalResult) error {
	if node == nil {
		return nil
	}

	tr.Add(node)

	return node.next.traverse(resolver, tr)
}

func (d *levelDetail) addMapper(mapper *valueMapper) {
	detail := d
	for i := range detail.mappers {
		if detail.mappers[i].Key == mapper.Key {
			return
		}
	}
	detail.mappers = append(detail.mappers, mapper)
}

func (d *levelDetail) insertValue(value Value) *trieNode {
	detail := d

	switch value := value.(type) {
	case nil:
		detail.undefined = util.Or(detail.undefined, newTrieNodeImpl)
		return detail.undefined
	case Var:
		detail.any = util.Or(detail.any, newTrieNodeImpl)
		return detail.any
	case Null, Boolean, Number, String:
		child, ok := detail.scalars.Get(value)
		if !ok {
			child = newTrieNodeImpl()
			detail.scalars = util.Or(detail.scalars, newScalarChildren)
			detail.scalars.Put(value, child)
		}
		return child
	case *Array:
		detail.array = util.Or(detail.array, newArrayTrie)
		return detail.array.insert(value)

	// `x in <collection>` (see updateMemberRefInValue) inserts each element of
	// the literal collection as-is, without restricting it to scalars/arrays
	// like the equality-based indexing does (see indexValue). A ground
	// Object or Set element can't be indexed precisely, so - like Var - it
	// falls back to the "any" node: the rule stays a candidate for every
	// input value. (The other composite Value types - Ref, comprehensions,
	// Call - can't actually reach here: the compiler rewrites them into
	// separate statements, bound to a Var, before the index is built.)
	case Object, Set:
		detail.any = util.Or(detail.any, newTrieNodeImpl)
		return detail.any
	}

	panic("illegal value")
}

// arrayTrie dispatches on the elements of an array, one node per position. A
// position dispatches the element after it and ends a rule's array, which a
// trieNode cannot hold at once.
type arrayTrie struct {
	any     *arrayTrie
	scalars *util.HasherMap[Value, *arrayTrie]
	// end is where a rule whose array ends at this position continues.
	end *trieNode
}

func newArrayTrie() *arrayTrie {
	return &arrayTrie{}
}

func newArrayChildren() *util.HasherMap[Value, *arrayTrie] {
	return util.NewHasherMap[Value, *arrayTrie](ValueEqual)
}

// insert returns the node the rule's path continues from once arr is consumed.
func (a *arrayTrie) insert(arr *Array) *trieNode {
	if arr.Len() == 0 {
		a.end = util.Or(a.end, newTrieNodeImpl)
		return a.end
	}

	switch head := arr.Elem(0).Value.(type) {
	case Null, Boolean, Number, String:
		child, ok := a.scalars.Get(head)
		if !ok {
			child = newArrayTrie()
			a.scalars = util.Or(a.scalars, newArrayChildren)
			a.scalars.Put(head, child)
		}
		return child.insert(arr.Slice(1, -1))

	// An element that is itself an array, object or set cannot be indexed
	// precisely at this position, so -- as for a var -- it falls back to any,
	// and the elements after it go on being indexed.
	case Var, *Array, Object, Set:
		a.any = util.Or(a.any, newArrayTrie)
		return a.any.insert(arr.Slice(1, -1))
	}

	panic("illegal value")
}

func (a *arrayTrie) traverse(resolver ValueResolver, tr *trieTraversalResult, arr *Array) error {
	if a == nil {
		return nil
	}

	if arr.Len() == 0 {
		return a.end.Traverse(resolver, tr)
	}

	if err := a.any.traverse(resolver, tr, arr.Slice(1, -1)); err != nil {
		return err
	}

	switch head := arr.Elem(0).Value.(type) {
	case Null, Boolean, Number, String:
		child, _ := a.scalars.Get(head)
		return child.traverse(resolver, tr, arr.Slice(1, -1))
	}

	return nil
}

func (a *arrayTrie) traverseUnknown(resolver ValueResolver, tr *trieTraversalResult) error {
	if a == nil {
		return nil
	}

	if err := a.end.Traverse(resolver, tr); err != nil {
		return err
	}

	if err := a.any.traverseUnknown(resolver, tr); err != nil {
		return err
	}

	var iterErr error
	a.scalars.Iter(func(_ Value, child *arrayTrie) bool {
		iterErr = child.traverseUnknown(resolver, tr)
		return iterErr != nil
	})

	return iterErr
}

func (a *arrayTrie) do(walker trieWalker) {
	if a == nil {
		return
	}

	a.end.Do(walker)
	a.any.do(walker)
	a.scalars.Iter(func(_ Value, child *arrayTrie) bool {
		child.do(walker)
		return false
	})
}

func (a *arrayTrie) compact() {
	if a == nil {
		return
	}

	a.end.compact()
	a.any.compact()
	a.scalars.Iter(func(_ Value, child *arrayTrie) bool {
		child.compact()
		return false
	})
}

func (d *levelDetail) traverse(resolver ValueResolver, tr *trieTraversalResult) error {
	if d == nil {
		return nil
	}

	v, err := resolver.Resolve(d.ref)
	if err != nil {
		if IsUnknownValueErr(err) {
			return d.traverseUnknown(resolver, tr)
		}
		return err
	}

	// Which order the branches below are taken in does not decide the order the
	// candidates come back in -- gather reads them by id. Only undefined coming
	// before the nil return is load-bearing: a ref that resolved to nothing
	// admits the rules wanting it undefined and no others.
	if err = d.undefined.Traverse(resolver, tr); err != nil {
		return err
	}

	if v == nil {
		return nil
	}

	if err = d.any.Traverse(resolver, tr); err != nil {
		return err
	}

	if err = d.traverseValue(resolver, tr, v); err != nil {
		return err
	}

	// Prefix constraints are tested against the value as it is, never against
	// what a mapper makes of it: the glob mapper turns a string into the array
	// of its segments, and matching prefixes against those segments would
	// answer a question no rule asked.
	if err = d.traversePrefixes(resolver, tr, v); err != nil {
		return err
	}

	if err = d.traverseSuffixes(resolver, tr, v); err != nil {
		return err
	}

	for i := range d.mappers {
		mapped := d.mappers[i].MapValue(v)
		if !ValueEqual(mapped, v) {
			if err := d.traverseValue(resolver, tr, mapped); err != nil {
				return err
			}
		}
	}

	return nil
}

func (d *levelDetail) traverseValue(resolver ValueResolver, tr *trieTraversalResult, value Value) error {
	switch value := value.(type) {
	case *Array, Set, Object:
		if d.array != nil {
			if arr, ok := value.(*Array); ok {
				if err := d.array.traverse(resolver, tr, arr); err != nil {
					return err
				}
			}
		}
		// Alternatives as well as scalars: a level every rule reaches by
		// several values has its children under alternatives and none under
		// scalars, and a collection at the reference still has to be tested
		// against them.
		if d.scalars.Len() > 0 || d.alternatives != nil {
			return d.traverseCollectionMembership(resolver, tr, value)
		}
	case Null, Boolean, Number, String:
		if child, ok := d.scalars.Get(value); ok {
			if err := child.Traverse(resolver, tr); err != nil {
				return err
			}
		}
		// A level with no alternatives -- almost all of them -- pays a branch
		// and nothing more.
		if d.alternatives != nil {
			return d.alternatives.traverse(resolver, tr, value)
		}
	}

	return nil
}

// traverse visits the nodes that the rules reaching this level by value
// continue from.
func (alt *alternativeChildren) traverse(resolver ValueResolver, tr *trieTraversalResult, value Value) error {
	nodes, ok := alt.members.Get(value)
	if !ok {
		return nil
	}

	for _, child := range nodes {
		if err := child.Traverse(resolver, tr); err != nil {
			return err
		}
	}

	return nil
}

func (d *levelDetail) traverseCollectionMembership(resolver ValueResolver, tr *trieTraversalResult, collection Value) error {
	alt := d.alternatives
	checkMember := func(t *Term) error {
		if IsScalar(t.Value) {
			child, _ := d.scalars.Get(t.Value)
			if err := child.Traverse(resolver, tr); err != nil {
				return err
			}
			if alt != nil {
				return alt.traverse(resolver, tr, t.Value)
			}
		}
		return nil
	}

	switch col := collection.(type) {
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

// traverseUnknown visits every child of a level whose reference the resolver
// cannot answer for. What each child constrains below resolves as usual: an
// unknown at one level says nothing about the levels under it.
func (d *levelDetail) traverseUnknown(resolver ValueResolver, tr *trieTraversalResult) error {
	if d == nil {
		return nil
	}

	if err := d.undefined.Traverse(resolver, tr); err != nil {
		return err
	}

	if err := d.any.Traverse(resolver, tr); err != nil {
		return err
	}

	if err := d.array.traverseUnknown(resolver, tr); err != nil {
		return err
	}

	if err := d.prefixes.traverseUnknown(resolver, tr); err != nil {
		return err
	}

	if err := d.suffixes.traverseUnknown(resolver, tr); err != nil {
		return err
	}

	for _, child := range d.converged() {
		if err := child.Traverse(resolver, tr); err != nil {
			return err
		}
	}

	var iterErr error
	d.scalars.Iter(func(_ Value, child *trieNode) bool {
		iterErr = child.Traverse(resolver, tr)
		return iterErr != nil
	})

	return iterErr
}

// If term `a` is one of the function's operands, we store a Ref: `args[0]`
// for the argument number. So for `f(x, y) { x = 10; y = 12 }`, we'll
// bind `args[0]` and `args[1]` to this rule when called for (x=10) and
// (y=12) respectively.
func (i *refindices) eqOperandsToRefAndValue(rule *Rule, args []*Term, a, b Value, constants map[Var]Value) bool {
	switch v := a.(type) {
	case Var:
		// a is a var, but we have not been able to resolve it to a ref, save for later
		if IsConstant(b) {
			constants[v] = b
		}

		bval, ok := indexValue(b)
		if !ok {
			return false
		}
		if ref := i.resolveVarToRef(i.resolvable(rule), args, v); ref != nil {
			i.insert(rule, &refindex{ref: i.table.intern(ref), Value: bval})
			return true
		}

	case Ref:
		// A ref rooted at a local -- `x := input; x.foo == "bar"` -- indexes the
		// same as the ref that local aliases, so long as the local resolves.
		v = i.resolveRefHead(rule, args, v)
		if v == nil || !i.isValidIndexRef(v) {
			return false
		}

		if bvar, ok := b.(Var); ok { // cheaper lookup first: constants
			if resolved, ok := constants[bvar]; ok {
				b = resolved
			}
		} else if bval, ok := indexValue(b); ok {
			b = bval
		} else {
			return false
		}

		i.insert(rule, &refindex{ref: i.table.intern(v), Value: b})
		return true
	}
	return false
}

func indexValue(b Value) (Value, bool) {
	switch b := b.(type) {
	case Null, Boolean, Number, String, Var:
		return b, true
	case *Array:
		stop := false
		first := true
		vis := NewGenericVisitor(func(x any) bool {
			if first {
				first = false
				return false
			}
			switch x.(type) {
			// No nested structures or values that require evaluation (other than var).
			case *Array, Object, Set, *ArrayComprehension, *ObjectComprehension, *SetComprehension, Ref:
				stop = true
			}
			return stop
		})
		vis.Walk(b)
		if !stop {
			return b, true
		}
	}

	return nil, false
}

func globDelimiterToString(delim *Term) (string, bool) {
	arr, ok := delim.Value.(*Array)
	if !ok {
		return "", false
	}

	var result string

	if arr.Len() == 0 {
		result = "."
	} else {
		sb := strings.Builder{}
		for i := range arr.Len() {
			term := arr.Elem(i)
			s, ok := term.Value.(String)
			if !ok {
				return "", false
			}
			sb.WriteString(string(s))
		}
		result = sb.String()
	}

	return result, true
}

func globPatternToArray(pattern *Term, delim string) *Term {
	s, ok := pattern.Value.(String)
	if !ok {
		return nil
	}

	parts := splitStringEscaped(string(s), delim)
	arr := make([]*Term, len(parts))

	for i := range parts {
		if parts[i] == "*" {
			arr[i] = globwildcard
		} else {
			var escaped bool
			for _, c := range parts[i] {
				if c == '\\' {
					escaped = !escaped
					continue
				}
				if !escaped {
					switch c {
					case '[', '?', '{', '*':
						// TODO(tsandall): super glob and character pattern
						// matching not supported yet.
						return nil
					}
				}
				escaped = false
			}
			arr[i] = StringTerm(parts[i])
		}
	}

	return ArrayTerm(arr...)
}

// splits s on characters in delim except if delim characters have been escaped
// with reverse solidus.
func splitStringEscaped(s string, delim string) []string {
	var last, curr int
	var escaped bool
	var result []string

	for ; curr < len(s); curr++ {
		if s[curr] == '\\' || escaped {
			escaped = !escaped
			continue
		}
		if strings.ContainsRune(delim, rune(s[curr])) {
			result = append(result, s[last:curr])
			last = curr + 1
		}
	}

	result = append(result, s[last:])

	return result
}

func stringSliceToArray(s []string) *Array {
	arr := make([]*Term, len(s))
	for i, v := range s {
		arr[i] = InternedTerm(v)
	}
	return NewArray(arr...)
}

func skipIndexingOperator(expr *Expr) bool {
	op := expr.OperatorTerm()
	return op != nil && skipIndexing.Contains(op)
}

// bodySkipsIndexing reports whether body contains an expression that must not
// be indexed away, either at the top level or inside a nested body. The nested
// bodies matter: a rule holding a `print` call inside an `and`, `or`, `not` or
// `every` body is still a rule whose side effects are lost if the indexer
// excludes it from evaluation.
func bodySkipsIndexing(body Body) bool {
	if slices.ContainsFunc(body, skipIndexingOperator) {
		return true
	}
	for _, expr := range body {
		if !exprHasNestedBody(expr) {
			continue
		}
		found := false
		WalkBodies(expr, func(b Body) bool {
			if !found && slices.ContainsFunc(b, skipIndexingOperator) {
				found = true
			}
			return found
		})
		if found {
			return true
		}
	}
	return false
}

// exprHasNestedBody is a cheap pre-check for bodySkipsIndexing: only these
// expression shapes hold a body directly, so only these are worth the cost of
// a full walk.
func exprHasNestedBody(expr *Expr) bool {
	switch expr.Terms.(type) {
	case *Every, *Not, *LogicalAnd, *LogicalOr:
		return true
	}
	return false
}
