// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"math/bits"
	"slices"
	"strings"
	"sync"

	"github.com/open-policy-agent/opa/v1/util"
)

var (
	globwildcard = VarTerm("$globwildcard")
	skipIndexing = NewSet(NewTerm(Interned.Refs.InternalPrint), NewTerm(Interned.Refs.InternalTestCase))

	// anyValue is the value of a ref a rule only requires to be defined.
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
		// rules holds one entry per rule and else branch the index holds, groups
		// the position of its ruleset among the rules Build was given. Reading the
		// ids a lookup reached in increasing order groups them and orders each
		// group by priority; see trieTraversalResult and gather.
		rules  []*Rule
		groups []int32
		// required holds, per rule id, the refs it needs defined that are not
		// levels of the counting index, as positions in requiredRefs. See
		// countingIndex.require.
		required map[int32][]int32
		// requiredRefs names those refs, indexed by the positions required holds,
		// so a lookup's memo is read at a position rather than searched.
		requiredRefs []Ref
		// memberships holds, per rule id, the collections gather consults; see
		// membership.
		memberships map[int32][]membership
		// mayEarlyExit decides whether consulting a collection pays; see gather.
		mayEarlyExit bool
		counting     *countingIndex
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
		onlyGroundRefs: true,
	}
}

func (i *baseDocEqIndex) Build(rules []*Rule) bool {
	if len(rules) == 0 {
		return false
	}

	i.kind = rules[0].Head.RuleKind()

	i.buildCounting(rules)
	i.mayEarlyExit = mayEarlyExit(i.rules)

	return true
}

// mayEarlyExit reports whether a caller could stop at the first of these rules
// that holds. Build asks it of every rule, which is what gather can know before it
// has candidates; Lookup asks resultMayEarlyExit of the candidates, which is finer.
func mayEarlyExit(rules []*Rule) bool {
	var value Value
	return agreeOnValue(rules, &value)
}

func (i *baseDocEqIndex) Lookup(resolver ValueResolver) (*IndexResult, error) {
	tr := ttrPool.Get().(*trieTraversalResult)

	defer func() {
		tr.reset()
		ttrPool.Put(tr)
	}()

	tr.grow(len(i.rules))
	tr.counting = util.Or(tr.counting, func() *countingLookup { return &countingLookup{} })
	if err := i.counting.traverse(resolver, tr, tr.counting); err != nil {
		return nil, err
	}
	result := IndexResultPool.Get()

	result.Kind = i.kind
	result.Default = i.defaultRule
	result.OnlyGroundRefs = i.onlyGroundRefs

	result.Rules = result.Rules[:0]

	clear(result.Else)

	if err := i.gather(tr, resolver, result); err != nil {
		IndexResultPool.Put(result)
		return nil, err
	}

	// Decided over the candidates: of `p := 1 if input.foo`, `p := 2 if
	// input.bar` and `p := 1 if input.baz`, the two agreeing on 1 are all a
	// lookup returns when input has no bar.
	tr.multiple = !resultMayEarlyExit(result)

	result.EarlyExit = !tr.multiple

	return result, nil
}

// resultMayEarlyExit reports whether a caller could stop at the first candidate
// that holds, the else branches included: they are values it could stop at too.
func resultMayEarlyExit(result *IndexResult) bool {
	var value Value
	if !agreeOnValue(result.Rules, &value) {
		return false
	}
	for _, branches := range result.Else {
		if !agreeOnValue(branches, &value) {
			return false
		}
	}
	return true
}

// agreeOnValue reports whether rules are complete documents agreeing on value,
// which it carries in so that several sets of rules can be asked as one.
func agreeOnValue(rules []*Rule, value *Value) bool {
	for _, rule := range rules {
		if rule.Head.DocKind() != CompleteDoc {
			return false
		}
		if rule.Head.Value == nil {
			continue
		}
		// A value that is not ground is a different one per binding, so there is
		// no first answer to stop at.
		v := rule.Head.Value.Value
		if !v.IsGround() {
			return false
		}
		if *value != nil && !ValueEqual(*value, v) {
			return false
		}
		*value = v
	}
	return true
}

// key answers for the reference a collection is keyed by, asking the resolver the
// first time only. It is a term, which is what a collection is asked with.
func (c *resolveCache) key(resolver ValueResolver, ref Ref) (*Term, error) {
	if c.keyOK && RefEqual(c.keyRef, ref) {
		return c.keyTerm, nil
	}
	v, err := resolver.Resolve(ref)
	if err != nil {
		return nil, err
	}
	var t *Term
	if v != nil {
		t = NewTerm(v)
	}
	c.keyRef, c.keyTerm, c.keyOK = ref, t, true
	return t, nil
}

// container resolves what holds the collection at ref, returning it and the path
// to the collection from there. It resolves the container once:
// `data.groups.g1.members` and `data.groups.g2.members` share one resolve.
func (c *resolveCache) container(resolver ValueResolver, ref Ref) (Value, Ref, error) {
	const shared = 2 // data.<container>

	if len(ref) <= shared {
		v, err := resolver.Resolve(ref)
		return v, nil, err
	}

	prefix := ref[:shared]
	if !c.prefixOK || !RefEqual(c.prefix, prefix) {
		v, err := resolver.Resolve(prefix)
		if err != nil {
			return nil, nil, err
		}
		c.prefix, c.prefixVal, c.prefixOK = prefix, v, true
	}
	return c.prefixVal, ref[shared:], nil
}

// consultsCollections reports whether excluding a candidate is worth consulting
// its collections for. It is not where the caller stops at the first candidate that
// holds: gather would have to consult all of them to exclude any, which is the work
// evaluation was about to do -- the lookup being what the rule tests.
func (i *baseDocEqIndex) consultsCollections(resolver ValueResolver) bool {
	if !i.mayEarlyExit {
		return true
	}
	every, ok := resolver.(IndexEveryCandidateEvaluated)
	return ok && every.IndexEveryCandidateEvaluated()
}

// inCollections reports whether the rule's collection memberships hold. A
// collection the resolver cannot answer for keeps the rule, as does an array:
// the only thing to ask one is a position, which tells a ruleset's rules apart
// only where their lengths differ.
func (i *baseDocEqIndex) inCollections(resolver ValueResolver, id int32, cache *resolveCache) (bool, error) {
	for _, m := range i.memberships[id] {
		key, err := cache.key(resolver, m.key)
		if err != nil {
			if IsUnknownValueErr(err) {
				continue
			}
			return false, err
		}
		if key == nil {
			return false, nil
		}

		var in bool
		if m.each {
			in, err = anyElementInCollection(resolver, cache, m.collections, key)
		} else {
			in, err = inAnyCollection(resolver, cache, m.collections, key)
		}
		if err != nil || !in {
			return false, err
		}
	}

	return true, nil
}

// anyElementInCollection reports whether an element of coll, the values of an
// object, is in one of collections. Anything other than a collection has no
// elements, and the iteration the rule makes over it yields nothing.
func anyElementInCollection(resolver ValueResolver, cache *resolveCache, collections []Ref, coll *Term) (bool, error) {
	if arr, ok := coll.Value.(*Array); ok {
		for j := range arr.Len() {
			if in, err := inAnyCollection(resolver, cache, collections, arr.Elem(j)); err != nil || in {
				return in, err
			}
		}
		return false, nil
	}

	// Sets and objects are iterated by callback, which escapes: it works on a copy
	// of the cache, so that the caller's, kept on its stack, does not escape too.
	local := *cache
	var in bool
	var err error
	until := func(elem *Term) bool {
		in, err = inAnyCollection(resolver, &local, collections, elem)
		return err != nil || in
	}

	switch c := coll.Value.(type) {
	case Set:
		c.Until(until)
	case Object:
		c.Until(func(_, v *Term) bool { return until(v) })
	}
	*cache = local
	return in, err
}

func inAnyCollection(resolver ValueResolver, cache *resolveCache, collections []Ref, key *Term) (bool, error) {
	for _, ref := range collections {
		if in, err := inCollection(resolver, cache, ref, key); err != nil || in {
			return in, err
		}
	}
	return false, nil
}

func inCollection(resolver ValueResolver, cache *resolveCache, ref Ref, key *Term) (bool, error) {
	base, path, err := cache.container(resolver, ref)
	if err != nil {
		if IsUnknownValueErr(err) {
			return true, nil
		}
		return false, err
	}
	if base == nil {
		return false, nil
	}

	// Asking the store's data as it is converts nothing, where Find and Get
	// would convert every object on the path, and again on every query.
	if lazy, ok := base.(*lazyObj); ok {
		if native, found, ok := lazy.nativeAt(path); ok {
			if !found {
				return false, nil
			}
			switch n := native.(type) {
			case map[string]any:
				s, ok := key.Value.(String)
				if !ok {
					return false, nil
				}
				_, ok = n[string(s)]
				return ok, nil
			case []any:
				return true, nil // as Find's array would below; see inCollections
			}
		}
	}

	coll, err := base.Find(path)
	if err != nil {
		return false, nil // undefined, not an error
	}

	switch c := coll.(type) {
	case nil:
		return false, nil
	case Object:
		return c.Get(key) != nil, nil
	// Base data read from JSON holds no set, but a store keeping ast.Value can,
	// and so can a `with` statement replacing the collection.
	case Set:
		return c.Contains(key), nil
	}
	return true, nil
}

// gather reads the rules a traversal reached into result. Ids ascend with
// priority, so a run of them sharing a group is that ruleset's definitions in
// order, the first being the one to evaluate.
//
// The refs that are not levels of the counting index are checked on the
// candidates here: a bit set for a rule that turns out undefined costs nothing
// to leave set, and asking here means the resolver's error is a return value.
// A nil resolver asks nothing, which is what AllRules wants.
func (i *baseDocEqIndex) gather(tr *trieTraversalResult, resolver ValueResolver, result *IndexResult) error {
	var cache resolveCache
	var root *Rule
	group := int32(-1)
	consults := i.consultsCollections(resolver)

	// Words are marked as they are first written to, in traversal order.
	found := 0
	for _, w := range util.Sorted(tr.touched) {
		found += bits.OnesCount64(tr.hits[w])
	}
	result.Rules = slices.Grow(result.Rules, found)

	// A word holds 64 ids: `w<<6` is the id of its first bit, TrailingZeros64 the
	// offset of the lowest set one, and `word &= word - 1` clears it.
	for _, w := range tr.touched {
		for word := tr.hits[w]; word != 0; word &= word - 1 {
			id := w<<6 | int32(bits.TrailingZeros64(word))

			if resolver != nil && len(i.required) > 0 {
				defined, err := i.defined(resolver, id, &cache)
				if err != nil {
					return err
				}
				if !defined {
					continue
				}
			}

			if consults && resolver != nil && len(i.memberships) > 0 {
				in, err := i.inCollections(resolver, id, &cache)
				if err != nil {
					return err
				}
				if !in {
					continue
				}
			}

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

	return nil
}

func (i *baseDocEqIndex) AllRules(ValueResolver) (*IndexResult, error) {
	tr := newTrieTraversalResult()
	tr.grow(len(i.rules))

	i.countingAll(tr)

	result := NewIndexResult(i.kind)
	result.Default = i.defaultRule
	result.OnlyGroundRefs = i.onlyGroundRefs

	// Every rule, so nothing is asked of the resolver and nothing can fail; see
	// gather.
	_ = i.gather(tr, nil, result)

	result.EarlyExit = !tr.multiple

	return result, nil
}

type valueMapper struct {
	Key      string
	MapValue func(Value) Value
}

// IndexEveryCandidateEvaluated may be implemented by a ValueResolver to answer
// whether every candidate a lookup returns goes on to be evaluated, rather than the
// caller stopping at the first that holds. Where it does, excluding a candidate
// saves evaluating it; see baseDocEqIndex.gather.
type IndexEveryCandidateEvaluated interface {
	IndexEveryCandidateEvaluated() bool
}

// membership is "the value at key has to be a key of one of the collections",
// which a lookup asks the collections rather than a level. There is more than
// one where the lookup is in an `or`: `{data.g.a[k]} or {data.g.b[k]}`.
type membership struct {
	key         Ref
	collections []Ref
	// each says the value at key is a collection, any element of which will do:
	// `x := input.subjects[_]; data.groups.g1.members[x]`.
	each bool
}

// refID identifies one of the references an index is built on.
type refID int32

// refTable numbers the references an index is built on.
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

// shared returns the ref the table holds for ref, so that rules naming the same
// ref keep one copy of it between them, not one each.
func (t *refTable) shared(ref Ref) Ref {
	return t.ref(t.intern(ref))
}

func (t *refTable) ref(id refID) Ref {
	return t.refs[id]
}

// affix is which end of the value at a reference a condition constrains, if it
// constrains an end rather than the whole of it.
type affix uint8

const (
	affixNone affix = iota
	affixPrefix
	affixSuffix
)

// insertLeaf adds a level testing v to the path being built, returning the node
// the rest of the path continues from.
func insertLeaf(node *trieNode, ref Ref, v condValue) *trieNode {
	switch v.affix {
	case affixPrefix:
		return node.InsertPrefix(ref, v.value)
	case affixSuffix:
		return node.InsertSuffix(ref, v.value)
	}
	if v.member {
		return node.InsertMember(ref, v.value)
	}
	return node.Insert(ref, v.value, v.mapper)
}

func oneEnd[T any](values []T, of func(T) (Value, affix)) []T {
	prefix, suffix := -1, -1
	for _, val := range values {
		v, a := of(val)
		s, ok := v.(String)
		if !ok {
			continue
		}
		switch a {
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

	return slices.DeleteFunc(slices.Clone(values), func(val T) bool {
		_, a := of(val)
		return a == drop
	})
}

// maxIndexPaths caps the ways a single rule may be reached: `or` expressions
// multiply out (`{a or b} and {c or d}` is four), and at some point the paths
// cost more than evaluating the rule.
const maxIndexPaths = 32

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

// plugRefs returns expr with each local in its references that is bound to a
// scalar plugged: `x := "foo"; data.policy[x].yz == "v"` then indexes as
// `data.policy.foo.yz == "v"` does. The rule is not modified.
func plugRefs(expr *Expr, constants map[Var]Value) *Expr {
	if len(constants) == 0 {
		return expr
	}

	switch ts := expr.Terms.(type) {
	case *Term: // a bare reference
		if g := plugRef(ts, constants); g != ts {
			cpy := *expr
			cpy.Terms = g
			return &cpy
		}
	case []*Term: // a call: ts[0] is its operator, which the clone keeps as it is
		var out []*Term
		for j := 1; j < len(ts); j++ {
			if g := plugRef(ts[j], constants); g != ts[j] {
				if out == nil {
					out = slices.Clone(ts)
				}
				out[j] = g
			}
		}
		if out != nil {
			cpy := *expr
			cpy.Terms = out
			return &cpy
		}
	}
	return expr
}

func plugRef(t *Term, constants map[Var]Value) *Term {
	ref, ok := t.Value.(Ref)
	if !ok {
		return t
	}

	var out Ref
	for j := 1; j < len(ref); j++ {
		v, ok := ref[j].Value.(Var)
		if !ok {
			continue
		}
		if c, ok := constants[v]; ok && IsScalar(c) {
			if out == nil {
				out = slices.Clone(ref)
			}
			out[j] = NewTerm(c)
		}
	}
	if out == nil {
		return t
	}
	return NewTerm(out)
}

// maxDomainExpansion caps how many times one expression is indexed over the
// literals its locals iterate: once per combination of their positions. Each
// costs about 0.45 us to build, so the cap bounds that at about 2 ms.
const maxDomainExpansion = 4096

// iteration is a literal some locals iterate: `some k, v in {"a": 1, "b": 2}`
// binds k to a key and v to the value under it, position by position. For a set
// a key is its value; for an array it is the position.
type iteration struct {
	keys, values []Value
}

// binding is which iteration a local takes part in, and as which of the two.
type binding struct {
	it  *iteration
	key bool
}

func (b binding) at(pos int) Value {
	if b.key {
		return b.it.keys[pos]
	}
	return b.it.values[pos]
}

func isLocalAlias(expr *Expr) bool {
	if !expr.IsEquality() {
		return false
	}
	_, a := expr.Operand(0).Value.(Var)
	_, b := expr.Operand(1).Value.(Var)
	return a && b
}

type trieWalker interface {
	Do(any) trieWalker
}

// trieSink is what a walk of a level's matcher reports the leaves it arrives at
// to.
type trieSink interface {
	Add(*trieNode)
}

// trieTraversalResult is the rules a lookup, or AllRules, reaches.
//
// They are a bitset over the index's rule ids, so reaching one by several paths
// costs nothing to notice: the second arrival writes a bit that is already
// set. Reading it back in id order is reading it grouped and in priority
// order, ids having been handed out that way, so there is nothing left to sort.
type trieTraversalResult struct {
	hits []uint64
	// touched holds the words of hits that were written to, so that clearing
	// costs what a lookup found rather than what the index holds.
	touched  []int32
	multiple bool
	counting *countingLookup
}

// defined reports whether every ref the rule needs resolves to something. A ref
// that is unknown rather than absent cannot exclude it, the same way traversal
// keeps everything below a level it cannot resolve (see traverseUnknown).
func (i *baseDocEqIndex) defined(resolver ValueResolver, id int32, cache *resolveCache) (bool, error) {
	for _, pos := range i.required[id] {
		defined, err := cache.defined(resolver, i.requiredRefs, pos)
		if err != nil {
			return false, err
		}
		if !defined {
			return false, nil
		}
	}

	return true, nil
}

// resolveCache memoizes, for the length of one lookup, what the resolver answered.
// The refs that are not levels are asked for once each however many rules read
// them, and the collections of a ruleset sit under a shared prefix.
//
// Where topdown's baseCache holds what the store gave, making a resolve cheap, this
// skips making the call.
type resolveCache struct {
	// required is one entry per ref in the index's requiredRefs, so a memo is
	// read at a position rather than searched for.
	required []resolved
	// keyRef and prefix are the last key and the last collection container asked
	// for: a ruleset's rules test the same field, and their collections sit side
	// by side under one prefix.
	keyRef    Ref
	keyTerm   *Term
	keyOK     bool
	prefix    Ref
	prefixVal Value
	prefixOK  bool
}

// resolved is a memo entry: unasked until a lookup asks, and then one of the two
// answers. Zero has to mean unasked, so that the buffer needs no initialising
// beyond being allocated.
type resolved uint8

const (
	unasked resolved = iota
	isDefined
	isAbsent
)

func (c *resolveCache) defined(resolver ValueResolver, refs []Ref, pos int32) (bool, error) {
	if c.required == nil {
		c.required = make([]resolved, len(refs))
	}
	if r := c.required[pos]; r != unasked {
		return r == isDefined, nil
	}

	v, err := resolver.Resolve(refs[pos])
	if err != nil {
		if !IsUnknownValueErr(err) {
			return false, err
		}
		// Unknown rather than absent cannot exclude the rule, the same way
		// traversal keeps everything below a level it cannot resolve.
		v = Boolean(true)
	}

	if v != nil {
		c.required[pos] = isDefined
		return true, nil
	}
	c.required[pos] = isAbsent
	return false, nil
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
}

func (tr *trieTraversalResult) set(id int32) {
	word, bit := id>>6, uint64(1)<<(uint(id)&63)
	if tr.hits[word]&bit != 0 {
		return
	}
	if tr.hits[word] == 0 {
		tr.touched = append(tr.touched, word)
	}
	tr.hits[word] |= bit
}

type trieNode struct {
	// next is the level below this node, set on a matcher's root.
	next *levelDetail
	// rules are, on a leaf, the ids of the counting index's paths whose
	// condition the leaf's value satisfies, in increasing order.
	rules []int32
	// posting is one more than the index of the posting holding the leaf's
	// paths, where that takes less room than listing them in rules; zero
	// otherwise.
	posting int32
}

// levelDetail is the matcher of a level of the counting index: the reference it
// resolves, and the leaves it dispatches the resolved value to, by exact value,
// member, prefix, suffix, glob pattern, or array. The suffix trie holds its base
// strings reversed, so that requiring one at the end of a value is requiring it
// at the start of the value reversed and the same trie answers both (see
// traverseSuffix).
type levelDetail struct {
	ref       Ref
	any       *trieNode
	undefined *trieNode
	array     *arrayTrie
	scalars   *util.HasherMap[Value, *trieNode]
	// strings holds the string-valued scalars, which most are, keyed by the
	// string itself: hashing and comparing an ast.Value costs more than a
	// map[string] does.
	strings  map[string]*trieNode
	members  *util.HasherMap[Value, *trieNode]
	mappers  []*valueMapper
	prefixes *prefixTrie
	suffixes *prefixTrie
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
	for _, child := range d.strings {
		child.Do(walker)
	}
	d.members.Iter(func(_ Value, child *trieNode) bool {
		child.Do(walker)
		return false
	})

	prefixTrieDo(d.prefixes, walker)
	prefixTrieDo(d.suffixes, walker)
	d.array.do(walker)
}

// compact walks a matcher once the index is built and releases what its slices
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

	prefixTrieCompact(d.prefixes)
	prefixTrieCompact(d.suffixes)

	d.any.compact()
	d.undefined.compact()
	d.array.compact()

	d.scalars.Iter(func(_ Value, child *trieNode) bool {
		child.compact()
		return false
	})
	for _, child := range d.strings {
		child.compact()
	}
	d.members.Iter(func(_ Value, child *trieNode) bool {
		child.compact()
		return false
	})

}

// InsertMember returns the node that the rules requiring value as a member of
// the collection at ref continue from.
func (node *trieNode) InsertMember(ref Ref, value Value) *trieNode {
	level := node.level()
	level.ref = ref

	level.members = util.Or(level.members, newScalarChildren)
	child, ok := level.members.Get(value)
	if !ok {
		child = newTrieNodeImpl()
		level.members.Put(value, child)
	}
	return child
}

func (node *trieNode) Insert(ref Ref, value Value, mapper *valueMapper) *trieNode {
	level := node.level()
	level.ref = ref

	if mapper != nil {
		level.addMapper(mapper)
	}

	return level.insertValue(value)
}

func (node *trieNode) Traverse(resolver ValueResolver, tr trieSink) error {
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
	case String:
		child, ok := detail.strings[string(value)]
		if !ok {
			child = newTrieNodeImpl()
			if detail.strings == nil {
				detail.strings = map[string]*trieNode{}
			}
			detail.strings[string(value)] = child
		}
		return child
	case Null, Boolean, Number:
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

	// `x in <collection>` inserts the literal's elements as-is; a ground
	// Object or Set element can't be indexed precisely, so - like Var - it
	// falls back to the "any" node: the rule stays a candidate for every
	// input value. (Ref, comprehensions and Call can't reach here: the
	// compiler binds them to a Var before the index is built.)
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

func (a *arrayTrie) traverse(resolver ValueResolver, tr trieSink, arr *Array) error {
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

func (a *arrayTrie) traverseUnknown(resolver ValueResolver, tr trieSink) error {
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

func (d *levelDetail) traverse(resolver ValueResolver, tr trieSink) error {
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

	// Members are looked for in the value as it is: what a mapper makes of a
	// string is not a collection the rule could have iterated.
	if d.members != nil {
		if err = d.traverseMembers(resolver, tr, v); err != nil {
			return err
		}
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

func (d *levelDetail) traverseValue(resolver ValueResolver, tr trieSink, value Value) error {
	switch value := value.(type) {
	case *Array, Set, Object:
		if d.array != nil {
			if arr, ok := value.(*Array); ok {
				if err := d.array.traverse(resolver, tr, arr); err != nil {
					return err
				}
			}
		}
	case Null, Boolean, Number, String:
		var child *trieNode
		var ok bool
		if s, isString := value.(String); isString {
			child, ok = d.strings[string(s)]
		} else {
			child, ok = d.scalars.Get(value)
		}
		if ok {
			if err := child.Traverse(resolver, tr); err != nil {
				return err
			}
		}
	}

	return nil
}

// traverseMembers visits the nodes of the rules requiring a member of the
// collection at the level's ref, which only a collection can satisfy.
func (d *levelDetail) traverseMembers(resolver ValueResolver, tr trieSink, collection Value) error {
	checkMember := func(t *Term) error {
		if !IsScalar(t.Value) {
			return nil
		}
		child, _ := d.members.Get(t.Value)
		return child.Traverse(resolver, tr)
	}

	switch col := collection.(type) {
	case *Array:
		return col.Iter(checkMember)
	case Set:
		return col.Iter(checkMember)
	case Object:
		for _, node := range objectFromObject(col).sortedKeys() {
			if err := checkMember(node.value); err != nil {
				return err
			}
		}
	}

	return nil
}

// traverseUnknown visits every child of a level whose reference the resolver
// cannot answer for. What each child constrains below resolves as usual: an
// unknown at one level says nothing about the levels under it.
func (d *levelDetail) traverseUnknown(resolver ValueResolver, tr trieSink) error {
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

	if err := prefixTrieTraverseUnknown(d.prefixes, resolver, tr); err != nil {
		return err
	}

	if err := prefixTrieTraverseUnknown(d.suffixes, resolver, tr); err != nil {
		return err
	}

	for _, child := range d.strings {
		if err := child.Traverse(resolver, tr); err != nil {
			return err
		}
	}

	var iterErr error
	d.scalars.Iter(func(_ Value, child *trieNode) bool {
		iterErr = child.Traverse(resolver, tr)
		return iterErr != nil
	})
	if iterErr != nil {
		return iterErr
	}
	d.members.Iter(func(_ Value, child *trieNode) bool {
		iterErr = child.Traverse(resolver, tr)
		return iterErr != nil
	})

	return iterErr
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
