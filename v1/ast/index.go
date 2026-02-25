// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"slices"
	"sort"
	"strings"
	"sync"

	"github.com/open-policy-agent/opa/v1/util"
)

// RuleIndex defines the interface for rule indices.
type RuleIndex interface {

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
type IndexResult struct {
	Rules          []*Rule
	Else           map[*Rule][]*Rule
	Default        *Rule
	Kind           RuleKind
	EarlyExit      bool
	OnlyGroundRefs bool
}

// NewIndexResult returns a new IndexResult object.
func NewIndexResult(kind RuleKind) *IndexResult {
	return &IndexResult{
		Kind: kind,
	}
}

// Empty returns true if there are no rules to evaluate.
func (ir *IndexResult) Empty() bool {
	return len(ir.Rules) == 0 && ir.Default == nil
}

type baseDocEqIndex struct {
	isVirtual      func(Ref) bool
	root           *trieNode
	defaultRule    *Rule
	kind           RuleKind
	onlyGroundRefs bool
}

var (
	equalityRef         = Equality.Ref()
	equalRef            = Equal.Ref()
	globMatchRef        = GlobMatch.Ref()
	internalPrintRef    = InternalPrint.Ref()
	internalTestCaseRef = InternalTestCase.Ref()
	internalMemberRef   = Member.Ref()

	skipIndexing = NewSet(NewTerm(internalPrintRef), NewTerm(internalTestCaseRef))
)

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
			var skip bool
			for i := range rule.Body {
				if op := rule.Body[i].OperatorTerm(); op != nil && skipIndexing.Contains(op) {
					skip = true
					break
				}
			}
			if !skip {
				clear(values)
				for i := range rule.Body {
					indices.Update(rule, rule.Body[i], values)
				}
			}
			return false
		})
	}

	// build trie out of indices.
	for idx := range rules {
		var prio int
		WalkRules(rules[idx], func(rule *Rule) bool {
			if rule.Default {
				return false
			}
			node := i.root
			if indices.Indexed(rule) {
				for _, ref := range indices.Sorted() {
					var values []*refindex
					for _, ri := range indices.rules[rule] {
						if ri.Ref.Equal(ref) {
							values = append(values, ri)
						}
					}
					if len(values) == 0 {
						node = node.Insert(ref, nil, nil)
					} else if len(values) == 1 {
						node = node.Insert(ref, values[0].Value, values[0].Mapper)
					} else {
						var hasVar bool
						for i := range values {
							if _, isVar := values[i].Value.(Var); isVar {
								hasVar = true
								break
							}
						}

						if hasVar {
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
								child := node.Insert(ref, val.Value, val.Mapper)
								child.append([...]int{idx, prio}, rule)
							}
							prio++
							return false
						}
					}
				}
			}
			// Insert rule into trie with (insertion order, priority order)
			// tuple. Retaining the insertion order allows us to return rules
			// in the order they were passed to this function.
			node.append([...]int{idx, prio}, rule)
			prio++
			return false
		})
	}
	return true
}

func (i *baseDocEqIndex) Lookup(resolver ValueResolver) (*IndexResult, error) {
	tr := ttrPool.Get().(*trieTraversalResult)

	defer func() {
		clear(tr.unordered)
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
		slices.SortFunc(tr.unordered[pos], func(a, b *ruleNode) int {
			return a.prio[1] - b.prio[1]
		})
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
		slices.SortFunc(tr.unordered[pos], func(a, b *ruleNode) int {
			return a.prio[1] - b.prio[1]
		})
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
}

type refindices struct {
	isVirtual func(Ref) bool
	rules     map[*Rule][]*refindex
	frequency *util.HasherMap[Ref, int]
	sorted    []Ref
}

func newrefindices(isVirtual func(Ref) bool) *refindices {
	return &refindices{
		isVirtual: isVirtual,
		rules:     map[*Rule][]*refindex{},
		frequency: util.NewHasherMap[Ref, int](RefEqual),
	}
}

// anyValue is a fake variable we used to put "naked ref" expressions
// into the rule index
var anyValue = Var("__any__")

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

	op := expr.Operator()
	if op == nil {
		if ts, ok := expr.Terms.(*Term); ok {
			// NOTE(sr): If we wanted to cover function args, we'd need to also
			// check for type "Var" here. But since it's impossible to call a
			// function with a undefined argument, there's no point to recording
			// "needs to be anything" for function args
			if ref, ok := ts.Value.(Ref); ok { // "naked ref"
				i.updateEq(rule, ref, anyValue, nil)
			}
		}
	}

	equalish := op.Equal(equalityRef) || // unification, no 3-operands version exists
		// NOTE(tsandall): if equal() is called with more than two arguments the
		// output value is being captured in which case the indexer cannot
		// exclude the rule if the equal() call would return false (because the
		// false value must still be produced.)
		(op.Equal(equalRef) && len(expr.Operands()) == 2)

	a, b := expr.Operand(0), expr.Operand(1)
	switch {
	case equalish:
		if !i.updateEqWildcardRef(rule, a.Value, b.Value, values) {
			i.updateEq(rule, a.Value, b.Value, values)
		}

	case op.Equal(globMatchRef) && len(expr.Operands()) == 3:
		// NOTE(sr): Same as with equal() above -- 4 operands means the output
		// of `glob.match` is captured and the rule can thus not be excluded.
		i.updateGlobMatch(rule, expr)

	case op.Equal(internalMemberRef) && len(expr.Operands()) == 2:
		// NOTE(sr): Again, 3 operands means captured output (like above).
		i.updateMember(rule, expr, values)
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
		counts := make([]int, 0, i.frequency.Len())
		i.sorted = make([]Ref, 0, i.frequency.Len())

		i.frequency.Iter(func(k Ref, v int) bool {
			counts = append(counts, v)
			i.sorted = append(i.sorted, k)
			return false
		})

		sort.Slice(i.sorted, func(a, b int) bool {
			if counts[a] > counts[b] {
				return true
			} else if counts[b] > counts[a] {
				return false
			}
			return i.sorted[a][0].Loc().Compare(i.sorted[b][0].Loc()) < 0
		})
	}

	return i.sorted
}

func (i *refindices) Indexed(rule *Rule) bool {
	return len(i.rules[rule]) > 0
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
			if ref := resolveVarToRef(i.rules[rule], args, v); ref != nil {
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
	args := rule.Head.Args
	lhs, rhs := expr.Operand(0), expr.Operand(1)

	lvar, ok := lhs.Value.(Var)
	if ok {
		lref := resolveVarToRef(i.rules[rule], args, lvar)
		if lref != nil {
			i.updateMemberRefInValue(rule, lref, rhs, constants) // `ref in value`
			return
		}
	}

	// `var0 in var1` case (var0 may be constant, var1 ref)
	i.updateMemberValueInRef(rule, args, lhs.Value, rhs, constants)
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

	addRef := func(t *Term) error {
		i.insert(rule, &refindex{Ref: ref, Value: t.Value})
		return nil
	}

	switch rcol := rval.(type) {
	case *Array:
		_ = rcol.Iter(addRef)
	case Set:
		_ = rcol.Iter(addRef)
	case Object:
		_ = rcol.Iter(func(_, v *Term) error {
			return addRef(v)
		})
	}
}

func (i *refindices) resolveAndValidateRef(rule *Rule, args []*Term, term *Term) Ref {
	var ref Ref
	switch v := term.Value.(type) {
	case Ref:
		ref = v
	case Var:
		ref = resolveVarToRef(i.rules[rule], args, v)
	default:
		return nil
	}

	if ref == nil || !i.isValidIndexRef(ref) {
		return nil
	}

	return ref
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
		if ov, ok := other.Value.(Var); ok && ov.Equal(v) {
			return other.Ref
		}
	}
	for j, arg := range args {
		if arg.Value.Compare(v) == 0 {
			return Ref{FunctionArgRootDocument, InternedTerm(j)}
		}
	}

	return nil
}

func (i *refindices) insert(rule *Rule, index *refindex) {
	count, _ := i.frequency.Get(index.Ref)
	i.frequency.Put(index.Ref, count+1)

	_, indexValueIsVar := index.Value.(Var)

	for pos, other := range i.rules[rule] {
		if other.Ref.Equal(index.Ref) {

			if ValueEqual(other.Value, index.Value) {
				return
			}
			_, otherValueIsVar := other.Value.(Var)
			if !indexValueIsVar && otherValueIsVar {
				i.rules[rule][pos] = index
				return
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

var ttrPool = sync.Pool{
	New: func() any {
		return newTrieTraversalResult()
	},
}

func newTrieTraversalResult() *trieTraversalResult {
	return &trieTraversalResult{
		unordered: map[int][]*ruleNode{},
	}
}

func (tr *trieTraversalResult) Add(t *trieNode) {
	for _, node := range t.rules {
		root := node.prio[0]
		nodes, ok := tr.unordered[root]
		if !ok {
			tr.ordering = append(tr.ordering, root)
		}
		// Deduplicate: check if a ruleNode with this priority already exists
		if !slices.ContainsFunc(nodes, func(existing *ruleNode) bool {
			return existing.prio == node.prio
		}) {
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
	ref       Ref
	mappers   []*valueMapper
	next      *trieNode
	any       *trieNode
	undefined *trieNode
	scalars   *util.HasherMap[Value, *trieNode]
	array     *trieNode
	rules     []*ruleNode
	value     *Term
	multiple  bool
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

func newTrieNodeImpl() *trieNode {
	return &trieNode{
		scalars: util.NewHasherMap[Value, *trieNode](ValueEqual),
	}
}

func (node *trieNode) Do(walker trieWalker) {
	if node == nil {
		return
	}
	next := walker.Do(node)
	if next == nil {
		return
	}

	node.any.Do(next)
	node.undefined.Do(next)

	node.scalars.Iter(func(_ Value, child *trieNode) bool {
		child.Do(next)
		return false
	})

	node.array.Do(next)
	node.next.Do(next)
}

func (node *trieNode) Insert(ref Ref, value Value, mapper *valueMapper) *trieNode {

	if node.next == nil {
		node.next = newTrieNodeImpl()
		node.next.ref = ref
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
	for i := range node.mappers {
		if node.mappers[i].Key == mapper.Key {
			return
		}
	}
	node.mappers = append(node.mappers, mapper)
}

func (node *trieNode) insertValue(value Value) *trieNode {

	switch value := value.(type) {
	case nil:
		if node.undefined == nil {
			node.undefined = newTrieNodeImpl()
		}
		return node.undefined
	case Var:
		if node.any == nil {
			node.any = newTrieNodeImpl()
		}
		return node.any
	case Null, Boolean, Number, String:
		child, ok := node.scalars.Get(value)
		if !ok {
			child = newTrieNodeImpl()
			node.scalars.Put(value, child)
		}
		return child
	case *Array:
		if node.array == nil {
			node.array = newTrieNodeImpl()
		}
		return node.array.insertArray(value)
	}

	panic("illegal value")
}

func (node *trieNode) insertArray(arr *Array) *trieNode {

	if arr.Len() == 0 {
		return node
	}

	switch head := arr.Elem(0).Value.(type) {
	case Var:
		if node.any == nil {
			node.any = newTrieNodeImpl()
		}
		return node.any.insertArray(arr.Slice(1, -1))
	case Null, Boolean, Number, String:
		child, ok := node.scalars.Get(head)
		if !ok {
			child = newTrieNodeImpl()
			node.scalars.Put(head, child)
		}
		return child.insertArray(arr.Slice(1, -1))
	}

	panic("illegal value")
}

func (node *trieNode) traverse(resolver ValueResolver, tr *trieTraversalResult) error {
	if node == nil {
		return nil
	}

	v, err := resolver.Resolve(node.ref)
	if err != nil {
		if IsUnknownValueErr(err) {
			return node.traverseUnknown(resolver, tr)
		}
		return err
	}

	err = node.undefined.Traverse(resolver, tr)
	if err != nil {
		return err
	}

	if v == nil {
		return nil
	}

	err = node.any.Traverse(resolver, tr)
	if err != nil {
		return err
	}

	err = node.traverseValue(resolver, tr, v)
	if err != nil {
		return err
	}

	for i := range node.mappers {
		mapped := node.mappers[i].MapValue(v)
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
		if node.array != nil {
			if arr, ok := value.(*Array); ok {
				return node.array.traverseArray(resolver, tr, arr)
			}
			return nil
		}

		if node.scalars.Len() > 0 {
			return node.traverseCollectionMembership(resolver, tr, value)
		}

		return nil

	case Null, Boolean, Number, String:
		child, ok := node.scalars.Get(value)
		if !ok {
			return nil
		}
		return child.Traverse(resolver, tr)
	}

	return nil
}

func (node *trieNode) traverseCollectionMembership(resolver ValueResolver, tr *trieTraversalResult, collection Value) error {
	checkMember := func(t *Term) error {
		if IsScalar(t.Value) {
			child, _ := node.scalars.Get(t.Value)
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

func (node *trieNode) traverseArray(resolver ValueResolver, tr *trieTraversalResult, arr *Array) error {
	if node == nil {
		return nil
	}

	if arr.Len() == 0 {
		return node.Traverse(resolver, tr)
	}

	err := node.any.traverseArray(resolver, tr, arr.Slice(1, -1))
	if err != nil {
		return err
	}

	head := arr.Elem(0).Value

	if !IsScalar(head) {
		return nil
	}

	switch head := head.(type) {
	case Null, Boolean, Number, String:
		child, _ := node.scalars.Get(head)
		return child.traverseArray(resolver, tr, arr.Slice(1, -1))
	}

	panic("illegal value")
}

func (node *trieNode) traverseUnknown(resolver ValueResolver, tr *trieTraversalResult) error {
	if node == nil {
		return nil
	}

	if err := node.Traverse(resolver, tr); err != nil {
		return err
	}

	if err := node.undefined.traverseUnknown(resolver, tr); err != nil {
		return err
	}

	if err := node.any.traverseUnknown(resolver, tr); err != nil {
		return err
	}

	if err := node.array.traverseUnknown(resolver, tr); err != nil {
		return err
	}

	var iterErr error
	node.scalars.Iter(func(_ Value, child *trieNode) bool {
		return child.traverseUnknown(resolver, tr) != nil
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
		if ref := resolveVarToRef(i.rules[rule], args, v); ref != nil {
			i.insert(rule, &refindex{Ref: ref, Value: bval})
			return true
		}

	case Ref:
		if !i.isValidIndexRef(v) {
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

var globwildcard = VarTerm("$globwildcard")

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
		arr[i] = StringTerm(v)
	}
	return NewArray(arr...)
}
