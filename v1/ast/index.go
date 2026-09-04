// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"maps"
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
	indices := newrefindices(i.isVirtual)
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
	sorted := indices.Sorted()

	for idx := range rules {
		var prio int
		WalkRules(rules[idx], func(rule *Rule) bool {
			if rule.Default {
				return false
			}
			// Each set of indices the rule can be reached through gets its own
			// path. They share a priority, so a lookup arriving at the rule down
			// several of them still reports it once (see trieTraversalResult.Add).
			if len(indices.disjunctions[rule]) == 0 {
				i.insertPath(sorted, indices.rules[rule], [...]int{idx, prio}, rule)
			} else {
				for _, path := range indices.paths(rule) {
					i.insertPath(sorted, path, [...]int{idx, prio}, rule)
				}
			}
			prio++
			return false
		})
	}

	i.root.compact()

	return true
}

func (i *baseDocEqIndex) insertPath(sorted []Ref, path []*refindex, prio [2]int, rule *Rule) {
	node := i.root

	// The path stops at the last level it constrains. A rule that constrains
	// nothing below has nothing to test there, so walking on would only pad the
	// path with an "absent" node per remaining level -- a copy of the whole tail
	// that no other rule shares, which is what made a trie of n levels cost n^2
	// nodes to build and to walk. The multiple-scalar case below has always
	// attached mid-trie for the same reason.
	remaining := len(path)

	for _, ref := range sorted {
		if remaining == 0 {
			break
		}

		var values []*refindex
		for _, ri := range path {
			if ri.Ref.Equal(ref) {
				values = append(values, ri)
			}
		}
		remaining -= len(values)

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
			} else {
				// When a rule has multiple scalar values (e.g., internal.member_2 with a set),
				// each value should have its own child node, and the rule is appended to each.
				// This creates separate paths for each value so different rules with overlapping
				// values don't interfere with each other.
				for _, val := range values {
					child := val.insertInto(node, ref)
					child.append(prio, rule)
				}
				return
			}
		}
	}

	// Insert rule into trie with (insertion order, priority order)
	// tuple. Retaining the insertion order allows us to return rules
	// in the order they were passed to this function.
	node.append(prio, rule)
}

func (i *baseDocEqIndex) Lookup(resolver ValueResolver) (*IndexResult, error) {
	tr := ttrPool.Get().(*trieTraversalResult)

	defer func() {
		// Note(anderseknert): `clear`ing the map is not good enough here, as it'd mean
		// resetting each of its slice values, costing us new allocations on each append
		// in subsequent lookups
		for i := range tr.unordered {
			tr.unordered[i] = tr.unordered[i][:0]
		}
		tr.ordering = tr.ordering[:0]
		tr.multiple = false
		tr.exist = nil

		ttrPool.Put(tr)
	}()

	err := i.root.Traverse(resolver, tr)
	if err != nil {
		return nil, err
	}

	result := IndexResultPool.Get()

	result.Kind = i.kind
	result.Default = i.defaultRule
	result.OnlyGroundRefs = i.onlyGroundRefs

	if result.Rules == nil {
		result.Rules = make([]*Rule, 0, len(tr.ordering))
	} else {
		result.Rules = result.Rules[:0]
	}

	clear(result.Else)

	for _, pos := range tr.ordering {
		if len(tr.unordered[pos]) == 0 {
			continue
		}

		nodes := util.SortedFunc(tr.unordered[pos], (*ruleNode).prio1Cmp)
		root := nodes[0].rule

		result.Rules = append(result.Rules, root)
		if len(nodes) > 1 {
			if result.Else == nil {
				result.Else = map[*Rule][]*Rule{}
			}

			result.Else[root] = make([]*Rule, len(nodes)-1)
			for i := 1; i < len(nodes); i++ {
				result.Else[root][i-1] = nodes[i].rule
			}
		}
	}

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

func (i *baseDocEqIndex) AllRules(ValueResolver) (*IndexResult, error) {
	tr := newTrieTraversalResult()

	// Walk over the rule trie and accumulate _all_ rules
	rw := &ruleWalker{result: tr}
	i.root.Do(rw)

	result := NewIndexResult(i.kind)
	result.Default = i.defaultRule
	result.OnlyGroundRefs = i.onlyGroundRefs
	result.Rules = make([]*Rule, 0, len(tr.ordering))

	for _, pos := range tr.ordering {
		if len(tr.unordered[pos]) == 0 {
			continue
		}
		slices.SortFunc(tr.unordered[pos], (*ruleNode).prio1Cmp)
		nodes := tr.unordered[pos]
		root := nodes[0].rule
		result.Rules = append(result.Rules, root)
		if len(nodes) > 1 {
			if result.Else == nil {
				result.Else = map[*Rule][]*Rule{}
			}

			result.Else[root] = make([]*Rule, len(nodes)-1)
			for i := 1; i < len(nodes); i++ {
				result.Else[root][i-1] = nodes[i].rule
			}
		}
	}

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

type refindex struct {
	Ref    Ref
	Value  Value
	Mapper *valueMapper
	// Affix says whether Value is a string the value at Ref has to start or end
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
	outer     []*refindex
	frequency *util.HasherMap[Ref, int]
	// alternated holds the refs that some rule constrains with more than one
	// alternative. insertPath stops building that rule's path there -- it hangs
	// the rule off every alternative and returns -- so whatever else the rule
	// constrains is only indexed if it comes first. Sorted puts these refs last
	// to make that so.
	//
	// The alternatives an `or` contributes (see require) are not recorded: they
	// are separate paths, and only meet a second value for the same ref once
	// paths() combines them, which is after Sorted has run. Missing one costs
	// the ordering, never correctness.
	alternated *util.HasherMap[Ref, struct{}]
	sorted     []Ref
}

// maxIndexPaths caps the ways a single rule may be reached: `or` expressions
// multiply out (`{a or b} and {c or d}` is four), and at some point the trie
// nodes cost more than evaluating the rule.
const maxIndexPaths = 32

func newrefindices(isVirtual func(Ref) bool) *refindices {
	return &refindices{
		isVirtual:  isVirtual,
		rules:      map[*Rule][]*refindex{},
		frequency:  util.NewHasherMap[Ref, int](RefEqual),
		alternated: util.NewHasherMap[Ref, struct{}](RefEqual),
	}
}

func valueIsVar(v Value) bool {
	_, ok := v.(Var)
	return ok
}

func (i *refindex) isVar() bool {
	return valueIsVar(i.Value)
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
		i.updateStartsWith(rule, expr, values)

	case op.Equal(Interned.Refs.AnyPrefixMatch) && len(expr.Operands()) == 2:
		i.updateAnyPrefixMatch(rule, expr, values)

	case op.Equal(Interned.Refs.EndsWith) && len(expr.Operands()) == 2:
		i.updateEndsWith(rule, expr, values)

	case op.Equal(Interned.Refs.AnySuffixMatch) && len(expr.Operands()) == 2:
		i.updateAnySuffixMatch(rule, expr, values)
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
				i.count(ri.Ref)
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
	scratch := newrefindices(i.isVirtual)
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
				alt[pos] = &refindex{Ref: ri.Ref, Value: anyValue, Mapper: ri.Mapper}
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

// Sorted returns a sorted list of references that the indices were built from.
// References that appear more frequently in the indexed rules are ordered
// before less frequently appearing references.
func (i *refindices) Sorted() []Ref {
	if i.sorted == nil {
		i.sorted = util.SortedFunc(i.frequency.Keys(), func(a, b Ref) int {
			// A ref that ends some rule's path is worth less as an early level
			// than any ref that does not, however often it was recorded (see
			// refindices.alternated), so it is ranked ahead of frequency.
			if altA, altB := i.isAlternated(a), i.isAlternated(b); altA != altB {
				if altA {
					return 1
				}
				return -1
			}
			countsA, _ := i.frequency.Get(a)
			countsB, _ := i.frequency.Get(b)
			if countsA < countsB { // descending, we want highest-freq first
				return 1
			} else if countsA > countsB {
				return -1
			}
			return a[0].Loc().Compare(b[0].Loc())
		})
	}
	return i.sorted
}

func (i *refindices) Value(rule *Rule, ref Ref) Value {
	if index := i.index(rule, ref); index != nil {
		return index.Value
	}
	return nil
}

func (i *refindices) Mapper(rule *Rule, ref Ref) *valueMapper {
	if index := i.index(rule, ref); index != nil {
		return index.Mapper
	}
	return nil
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

	i.insert(rule, &refindex{Ref: groundPrefix, Value: resolvedValue})
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
			if ref := resolveVarToRef(i.resolvable(rule), args, v); ref != nil {
				i.insert(rule, &refindex{
					Ref:   ref,
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
		lref := resolveVarToRef(i.resolvable(rule), rule.Head.Args, lvar)
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

	i.insert(rule, &refindex{Ref: rref, Value: lval})
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

// insertMembers records the members of the literal collection of an `in`
// statement, each of which is a value the rule may reach ref by. It is
// insertPrefixes again -- see there for why the scan is hoisted out of insert
// -- and stays a copy of it because the two cannot share a dedup set: a base
// string is always a String, so insertPrefixes gets a plain map, while an `in`
// collection holds arbitrary values and needs a HasherMap. Sharing the code
// meant sharing the HasherMap, which cost the prefix path 27% of its build.
func (i *refindices) insertMembers(rule *Rule, ref Ref, members []Value) {
	if len(members) < 2 {
		for _, member := range members {
			i.insert(rule, &refindex{Ref: ref, Value: member})
		}
		return
	}

	// Unlike a prefix, a concrete member takes the place of a "reference is
	// anything" entry (see insert), so the first one goes the ordinary way --
	// the rule's list is short at that point, so the scan it costs is cheap.
	i.insert(rule, &refindex{Ref: ref, Value: members[0]})

	// insert is the only one that may put a value somewhere other than the end
	// of the list, which is what a var needs, so those go in through it and are
	// left out of the block below. A collection holding one is rare, and paying
	// a copy for it keeps the common case a single pass.
	rest := members[1:]
	if slices.ContainsFunc(rest, valueIsVar) {
		for _, member := range rest {
			if valueIsVar(member) {
				i.insert(rule, &refindex{Ref: ref, Value: member})
			}
		}
		rest = slices.DeleteFunc(slices.Clone(rest), valueIsVar)
	}

	concrete := 0
	seen := util.NewHasherMap[Value, struct{}](ValueEqual)

	for _, other := range i.rules[rule] {
		if !other.Ref.Equal(ref) {
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

		*indices[pos] = refindex{Ref: ref, Value: member}
		pos++
	}
	i.rules[rule] = indices[:pos]

	i.countN(ref, len(rest))

	if concrete > 1 {
		i.alternate(ref)
	}
}

func (i *refindices) resolveAndValidateRef(rule *Rule, args []*Term, term *Term) Ref {
	var ref Ref
	switch v := term.Value.(type) {
	case Ref:
		ref = v
	case Var:
		ref = resolveVarToRef(i.resolvable(rule), args, v)
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

	resolved := resolveVarToRef(i.resolvable(rule), args, head)
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
func resolveVarToRef(ri []*refindex, args []*Term, v Var) Ref {
	for _, other := range ri {
		if v.Equal(other.Value) {
			return other.Ref
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
func (i *refindices) count(ref Ref) {
	i.countN(ref, 1)
}

func (i *refindices) countN(ref Ref, n int) {
	count, _ := i.frequency.Get(ref)
	i.frequency.Put(ref, count+n)
}

// alternate records that a rule reaches ref by more than one value, which is
// what insertPath ends the rule's path on (see refindices.alternated). Only the
// values that survive insertPath's var-stripping count: a var entry alongside a
// concrete one is dropped there, so it is not an alternative.
func (i *refindices) alternate(ref Ref) {
	i.alternated.Put(ref, struct{}{})
}

func (i *refindices) isAlternated(ref Ref) bool {
	_, ok := i.alternated.Get(ref)
	return ok
}

func (i *refindices) insert(rule *Rule, index *refindex) {
	i.count(index.Ref)

	indexValueIsVar := index.isVar()

	for pos, other := range i.rules[rule] {
		if other.Ref.Equal(index.Ref) {
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
				i.alternate(index.Ref)
			}
		}
	}

	i.rules[rule] = append(i.rules[rule], index)
}

func (i *refindices) index(rule *Rule, ref Ref) *refindex {
	for _, index := range i.rules[rule] {
		if index.Ref.Equal(ref) {
			return index
		}
	}
	return nil
}

type trieWalker interface {
	Do(any) trieWalker
}

type trieTraversalResult struct {
	unordered map[int][]*ruleNode
	ordering  []int
	exist     *Term
	multiple  bool
}

var ttrPool = &sync.Pool{
	New: func() any {
		return newTrieTraversalResult()
	},
}

func newTrieTraversalResult() *trieTraversalResult {
	return &trieTraversalResult{
		unordered: make(map[int][]*ruleNode, 16),
	}
}

func (tr *trieTraversalResult) Add(t *trieNode) {
	for _, node := range t.rules {
		root := node.prio[0]
		if nodes, ok := tr.unordered[root]; !ok || len(nodes) == 0 {
			tr.ordering = append(tr.ordering, root)
			tr.unordered[root] = append(nodes, node)
		} else if !slices.ContainsFunc(nodes, node.prioEqual) {
			tr.unordered[root] = append(nodes, node)
		}
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
	// detail is what only a level node has; see levelDetail.
	detail   *levelDetail
	next     *trieNode
	rules    []*ruleNode
	value    *Term
	multiple bool
}

func (node *trieNode) append(prio [2]int, rule *Rule) {
	node.rules = append(node.rules, &ruleNode{prio, rule})

	if node.value != nil && rule.Head.Value != nil && !node.value.Equal(rule.Head.Value) {
		node.multiple = true
	}

	if node.value == nil && rule.Head.DocKind() == CompleteDoc {
		node.value = rule.Head.Value
	}
}

type ruleNode struct {
	prio [2]int
	rule *Rule
}

func (a *ruleNode) prio1Cmp(b *ruleNode) int {
	return a.prio[1] - b.prio[1]
}

func (a *ruleNode) prioEqual(b *ruleNode) bool {
	return a.prio == b.prio
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
// fields put trieNode in Go's 144-byte size class; out of line it is 56 bytes,
// which rounds to 64. Moving them out a few at a time buys nothing -- 136 and
// 112 bytes both round up to a class the struct already occupied.
type levelDetail struct {
	ref       Ref
	any       *trieNode
	undefined *trieNode
	array     *trieNode
	scalars   *util.HasherMap[Value, *trieNode]
	mappers   []*valueMapper
	prefixes  *prefixTrie
	suffixes  *prefixTrie
}

func newTrieNodeImpl() *trieNode {
	return &trieNode{}
}

func (node *trieNode) ref() Ref {
	if node.detail == nil {
		return nil
	}
	return node.detail.ref
}

func (node *trieNode) any() *trieNode {
	if node.detail == nil {
		return nil
	}
	return node.detail.any
}

func (node *trieNode) undefined() *trieNode {
	if node.detail == nil {
		return nil
	}
	return node.detail.undefined
}

func (node *trieNode) array() *trieNode {
	if node.detail == nil {
		return nil
	}
	return node.detail.array
}

func (node *trieNode) scalars() *util.HasherMap[Value, *trieNode] {
	if node.detail == nil {
		return nil
	}
	return node.detail.scalars
}

func (node *trieNode) prefixes() *prefixTrie {
	if node.detail == nil {
		return nil
	}
	return node.detail.prefixes
}

func (node *trieNode) suffixes() *prefixTrie {
	if node.detail == nil {
		return nil
	}
	return node.detail.suffixes
}

func (node *trieNode) mappers() []*valueMapper {
	if node.detail == nil {
		return nil
	}
	return node.detail.mappers
}

func (node *trieNode) levelDetail() *levelDetail {
	node.detail = util.Or(node.detail, newLevelDetail)
	return node.detail
}

// affixTrie returns the trie for one end of the value, creating it and the
// detail that holds it on first use.
func (node *trieNode) affixTrie(a affix) *prefixTrie {
	detail := node.levelDetail()

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

	node.any().Do(next)
	node.undefined().Do(next)

	node.scalars().Iter(func(_ Value, child *trieNode) bool {
		child.Do(next)
		return false
	})

	node.prefixes().do(next)
	node.suffixes().do(next)
	node.array().Do(next)
	node.next.Do(next)
}

// compact walks the trie once the index is built and releases what its slices
// grew but do not use.
func (node *trieNode) compact() {
	if node == nil {
		return
	}

	node.prefixes().compact()
	node.suffixes().compact()

	node.any().compact()
	node.undefined().compact()
	node.array().compact()
	node.next.compact()

	node.scalars().Iter(func(_ Value, child *trieNode) bool {
		child.compact()
		return false
	})
}

func (node *trieNode) Insert(ref Ref, value Value, mapper *valueMapper) *trieNode {
	if node.next == nil {
		node.next = newTrieNodeImpl()
		node.next.levelDetail().ref = ref
	}

	if mapper != nil {
		node.next.addMapper(mapper)
	}

	return node.next.insertValue(value)
}

func (node *trieNode) Traverse(resolver ValueResolver, tr *trieTraversalResult) error {
	if node == nil {
		return nil
	}

	tr.Add(node)

	return node.next.traverse(resolver, tr)
}

func (node *trieNode) addMapper(mapper *valueMapper) {
	detail := node.levelDetail()
	for i := range detail.mappers {
		if detail.mappers[i].Key == mapper.Key {
			return
		}
	}
	detail.mappers = append(detail.mappers, mapper)
}

func (node *trieNode) insertValue(value Value) *trieNode {
	detail := node.levelDetail()

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
		detail.array = util.Or(detail.array, newTrieNodeImpl)
		return detail.array.insertArray(value)

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

func (node *trieNode) insertArray(arr *Array) *trieNode {
	if arr.Len() == 0 {
		return node
	}

	detail := node.levelDetail()

	switch head := arr.Elem(0).Value.(type) {
	case Var:
		detail.any = util.Or(detail.any, newTrieNodeImpl)
		return detail.any.insertArray(arr.Slice(1, -1))
	case Null, Boolean, Number, String:
		child, ok := detail.scalars.Get(head)
		if !ok {
			child = newTrieNodeImpl()
			detail.scalars = util.Or(detail.scalars, newScalarChildren)
			detail.scalars.Put(head, child)
		}
		return child.insertArray(arr.Slice(1, -1))

	// Same reasoning as in insertValue above: an array element can itself be
	// a nested array, object, or set, none of which can be indexed precisely
	// at this position, so fall back to "any" and keep indexing the
	// remaining elements.
	case *Array, Object, Set:
		detail.any = util.Or(detail.any, newTrieNodeImpl)
		return detail.any.insertArray(arr.Slice(1, -1))
	}

	panic("illegal value")
}

func (node *trieNode) traverse(resolver ValueResolver, tr *trieTraversalResult) error {
	if node == nil {
		return nil
	}

	v, err := resolver.Resolve(node.ref())
	if err != nil {
		if IsUnknownValueErr(err) {
			return node.traverseUnknown(resolver, tr)
		}
		return err
	}

	if err = node.undefined().Traverse(resolver, tr); err != nil {
		return err
	}

	if v == nil {
		return nil
	}

	if err = node.any().Traverse(resolver, tr); err != nil {
		return err
	}

	if err = node.traverseValue(resolver, tr, v); err != nil {
		return err
	}

	// Prefix constraints are tested against the value as it is, never against
	// what a mapper makes of it: the glob mapper turns a string into the array
	// of its segments, and matching prefixes against those segments would
	// answer a question no rule asked.
	if err = node.traversePrefixes(resolver, tr, v); err != nil {
		return err
	}

	if err = node.traverseSuffixes(resolver, tr, v); err != nil {
		return err
	}

	mappers := node.mappers()
	for i := range mappers {
		mapped := mappers[i].MapValue(v)
		if !ValueEqual(mapped, v) {
			if err := node.traverseValue(resolver, tr, mapped); err != nil {
				return err
			}
		}
	}

	return nil
}

func (node *trieNode) traverseValue(resolver ValueResolver, tr *trieTraversalResult, value Value) error {
	switch value := value.(type) {
	case *Array, Set, Object:
		if node.array() != nil {
			if arr, ok := value.(*Array); ok {
				if err := node.array().traverseArray(resolver, tr, arr); err != nil {
					return err
				}
			}
		}
		if node.scalars().Len() > 0 {
			return node.traverseCollectionMembership(resolver, tr, value)
		}
	case Null, Boolean, Number, String:
		if child, ok := node.scalars().Get(value); ok {
			return child.Traverse(resolver, tr)
		}
	}

	return nil
}

func (node *trieNode) traverseCollectionMembership(resolver ValueResolver, tr *trieTraversalResult, collection Value) error {
	checkMember := func(t *Term) error {
		if IsScalar(t.Value) {
			child, _ := node.scalars().Get(t.Value)
			return child.Traverse(resolver, tr)
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

func (node *trieNode) traverseArray(resolver ValueResolver, tr *trieTraversalResult, arr *Array) (err error) {
	if node == nil {
		return nil
	}

	if arr.Len() == 0 {
		return node.Traverse(resolver, tr)
	}

	if err = node.any().traverseArray(resolver, tr, arr.Slice(1, -1)); err == nil {
		switch head := arr.Elem(0).Value.(type) {
		case Null, Boolean, Number, String:
			child, _ := node.scalars().Get(head)
			return child.traverseArray(resolver, tr, arr.Slice(1, -1))
		}
	}

	return err
}

func (node *trieNode) traverseUnknown(resolver ValueResolver, tr *trieTraversalResult) error {
	if node == nil {
		return nil
	}

	if err := node.Traverse(resolver, tr); err != nil {
		return err
	}

	if err := node.undefined().traverseUnknown(resolver, tr); err != nil {
		return err
	}

	if err := node.any().traverseUnknown(resolver, tr); err != nil {
		return err
	}

	if err := node.array().traverseUnknown(resolver, tr); err != nil {
		return err
	}

	if err := node.prefixes().traverseUnknown(resolver, tr); err != nil {
		return err
	}

	if err := node.suffixes().traverseUnknown(resolver, tr); err != nil {
		return err
	}

	var iterErr error
	node.scalars().Iter(func(_ Value, child *trieNode) bool {
		iterErr = child.traverseUnknown(resolver, tr)
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
		if ref := resolveVarToRef(i.resolvable(rule), args, v); ref != nil {
			i.insert(rule, &refindex{Ref: ref, Value: bval})
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

		i.insert(rule, &refindex{Ref: v, Value: b})
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
