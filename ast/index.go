// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/open-policy-agent/opa/util"
)

// RuleIndex defines the interface for rule indices.
type RuleIndex interface {

	// Build tries to construct an index for the given rules. If the index was
	// constructed, ok is true, otherwise false.
	Build(rules []*Rule) (ok bool)

	// Index returns a set of rules to evaluate and a default rule if one was
	// present when the index was built.
	Index(resolver ValueResolver) (rules []*Rule, defaultRule *Rule, err error)
}

type baseDocEqIndex struct {
	isVirtual   func(Ref) bool
	root        *trieNode
	defaultRule *Rule
}

func newBaseDocEqIndex(isVirtual func(Ref) bool) *baseDocEqIndex {
	return &baseDocEqIndex{
		isVirtual: isVirtual,
		root:      newTrieNodeImpl(),
	}
}

func (i *baseDocEqIndex) Build(rules []*Rule) bool {

	refs := make(refValueIndex, len(rules))

	// freq is map[ref]int where the values represent the frequency of the
	// ref/key.
	freq := util.NewHashMap(func(a, b util.T) bool {
		r1, r2 := a.(Ref), b.(Ref)
		return r1.Equal(r2)
	}, func(x util.T) int {
		return x.(Ref).Hash()
	})

	// Build refs and freq maps
	for _, rule := range rules {

		if rule.Default {
			// Compiler guarantees that only one default will be defined per path.
			i.defaultRule = rule
			continue
		}

		for _, expr := range rule.Body {
			ref, value, ok := i.getRefAndValue(expr)
			if ok {
				refs.Insert(rule, ref, value)
				count, ok := freq.Get(ref)
				if !ok {
					count = 0
				}
				count = count.(int) + 1
				freq.Put(ref, count)
			}
		}
	}

	// Sort by frequency
	type refCountPair struct {
		ref   Ref
		count int
	}

	sorted := make([]refCountPair, 0, freq.Len())
	freq.Iter(func(k, v util.T) bool {
		ref, count := k.(Ref), v.(int)
		sorted = append(sorted, refCountPair{ref, count})
		return false
	})

	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].count > sorted[j].count {
			return true
		}
		return false
	})

	// Build trie
	for _, rule := range rules {

		if rule.Default {
			continue
		}

		node := i.root

		if refs := refs[rule]; refs != nil {
			for _, pair := range sorted {
				value := refs.Get(pair.ref)
				node = node.Insert(pair.ref, value)
			}
		}

		node.rules = append(node.rules, rule)
	}

	return true
}

func (i *baseDocEqIndex) Index(resolver ValueResolver) ([]*Rule, *Rule, error) {
	rules, err := i.traverse(i.root, resolver)
	return rules, i.defaultRule, err
}

func (i *baseDocEqIndex) traverse(node *trieNode, resolver ValueResolver) ([]*Rule, error) {

	if node == nil {
		return nil, nil
	}

	result := make([]*Rule, len(node.rules))
	copy(result, node.rules)
	next := node.next

	if next == nil {
		return result, nil
	}

	children, err := next.Resolve(resolver)
	if err != nil {
		return nil, err
	}

	for _, child := range children {
		rules, err := i.traverse(child, resolver)
		if err != nil {
			return nil, err
		}
		result = append(result, rules...)
	}

	return result, nil
}

func (i *baseDocEqIndex) getRefAndValue(expr *Expr) (Ref, Value, bool) {

	if !expr.IsEquality() || expr.Negated {
		return nil, nil, false
	}

	a, b := expr.Operand(0), expr.Operand(1)

	if ref, value, ok := i.getRefAndValueFromTerms(a, b); ok {
		return ref, value, true
	}

	return i.getRefAndValueFromTerms(b, a)
}

func (i *baseDocEqIndex) getRefAndValueFromTerms(a, b *Term) (Ref, Value, bool) {

	ref, ok := a.Value.(Ref)
	if !ok {
		return nil, nil, false
	}

	if !RootDocumentNames.Contains(ref[0]) {
		return nil, nil, false
	}

	if i.isVirtual(ref) {
		return nil, nil, false
	}

	if ref.IsNested() || !ref.IsGround() {
		return nil, nil, false
	}

	switch b := b.Value.(type) {
	case Null, Boolean, Number, String, Var:
		return ref, b, true
	case Array:
		stop := false
		first := true
		vis := NewGenericVisitor(func(x interface{}) bool {
			if first {
				first = false
				return false
			}
			switch x.(type) {
			// No nested structures or values that require evaluation (other than var).
			case Array, Object, *Set, *ArrayComprehension, Ref:
				stop = true
			}
			return stop
		})
		Walk(vis, b)
		if !stop {
			return ref, b, true
		}
	}

	return nil, nil, false
}

type refValueIndex map[*Rule]*ValueMap

func (m refValueIndex) Insert(rule *Rule, ref Ref, value Value) {
	vm, ok := m[rule]
	if !ok {
		vm = NewValueMap()
		m[rule] = vm
	}
	vm.Put(ref, value)
}

type trieWalker interface {
	Do(x interface{}) trieWalker
}

type trieNode struct {
	ref       Ref
	next      *trieNode
	any       *trieNode
	undefined *trieNode
	scalars   map[Value]*trieNode
	array     *trieNode
	rules     []*Rule
}

func newTrieNodeImpl() *trieNode {
	return &trieNode{
		scalars: map[Value]*trieNode{},
	}
}

func (node *trieNode) Do(walker trieWalker) {
	next := walker.Do(node)
	if next == nil {
		return
	}
	if node.next != nil {
		node.next.Do(next)
		return
	}
	if node.any != nil {
		node.any.Do(next)
	}
	if node.undefined != nil {
		node.undefined.Do(next)
	}
	for _, child := range node.scalars {
		child.Do(next)
	}
	if node.array != nil {
		node.array.Do(next)
	}
}

func (node *trieNode) Insert(ref Ref, value Value) *trieNode {

	if node.next == nil {
		node.next = newTrieNodeImpl()
		node.next.ref = ref
	}

	return node.next.insertValue(value)
}

func (node *trieNode) Resolve(resolver ValueResolver) ([]*trieNode, error) {

	v, err := resolver.Resolve(node.ref)
	if err != nil {
		return nil, err
	}

	result := []*trieNode{}

	if node.undefined != nil {
		result = append(result, node.undefined)
	}

	if v == nil {
		return result, nil
	}

	if node.any != nil {
		result = append(result, node.any)
	}

	result = append(result, node.resolveValue(v)...)
	return result, nil
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
		child, ok := node.scalars[value]
		if !ok {
			child = newTrieNodeImpl()
			node.scalars[value] = child
		}
		return child
	case Array:
		if node.array == nil {
			node.array = newTrieNodeImpl()
		}
		return node.array.insertArray(value)
	}

	panic("illegal value")
}

func (node *trieNode) insertArray(arr Array) *trieNode {

	if len(arr) == 0 {
		return node
	}

	switch head := arr[0].Value.(type) {
	case Var:
		if node.any == nil {
			node.any = newTrieNodeImpl()
		}
		return node.any.insertArray(arr[1:])
	case Null, Boolean, Number, String:
		child, ok := node.scalars[head]
		if !ok {
			child = newTrieNodeImpl()
			node.scalars[head] = child
		}
		return child.insertArray(arr[1:])
	}

	panic("illegal value")
}

func (node *trieNode) resolveValue(value Value) []*trieNode {

	switch value := value.(type) {
	case Array:
		if node.array == nil {
			return nil
		}
		return node.array.resolveArray(value)

	case Null, Boolean, Number, String:
		child, ok := node.scalars[value]
		if !ok {
			return nil
		}
		return []*trieNode{child}
	}

	return nil
}

func (node *trieNode) resolveArray(arr Array) []*trieNode {

	if len(arr) == 0 {
		if node.next != nil || len(node.rules) > 0 {
			return []*trieNode{node}
		}
		return nil
	}

	head := arr[0].Value

	if !IsScalar(head) {
		return nil
	}

	var result []*trieNode

	if node.any != nil {
		result = append(result, node.any.resolveArray(arr[1:])...)
	}

	child, ok := node.scalars[head]
	if !ok {
		return result
	}

	return append(result, child.resolveArray(arr[1:])...)
}

type triePrinter struct {
	depth int
	w     io.Writer
}

func (p triePrinter) Do(x interface{}) trieWalker {
	padding := strings.Repeat(" ", p.depth)
	fmt.Fprintf(p.w, "%v%v\n", padding, x)
	p.depth++
	return p
}

func printTrie(w io.Writer, trie *trieNode) {
	pp := triePrinter{0, w}
	trie.Do(pp)
}
