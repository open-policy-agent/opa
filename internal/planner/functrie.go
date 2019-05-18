package planner

import (
	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/internal/ir"
)

// functrie implements a simple trie structure for organizing planned functions.
// The functions are organized to facilitate access when planning references
// against the data document.
type functrie struct {
	children map[ast.Value]*functrie
	val      *functrieValue
}

type functrieValue struct {
	Fn    *ir.Func
	Rules []*ast.Rule
}

func (val *functrieValue) Arity() int {
	return len(val.Rules[0].Head.Args)
}

func newFunctrie() *functrie {
	return &functrie{
		children: map[ast.Value]*functrie{},
	}
}

func (t *functrie) Insert(key ast.Ref, val *functrieValue) {
	node := t
	for _, elem := range key {
		child, ok := node.children[elem.Value]
		if !ok {
			child = newFunctrie()
			node.children[elem.Value] = child
		}
		node = child
	}
	node.val = val
}

func (t *functrie) Lookup(key ast.Ref) *functrieValue {
	node := t
	for _, elem := range key {
		var ok bool
		if node == nil {
			return nil
		} else if node, ok = node.children[elem.Value]; !ok {
			return nil
		}
	}
	return node.val
}

func (t *functrie) LookupOrInsert(key ast.Ref, orElse *functrieValue) *functrieValue {
	if val := t.Lookup(key); val != nil {
		return val
	}
	t.Insert(key, orElse)
	return orElse
}

func (t *functrie) FuncMap() map[string]*ir.Func {
	result := map[string]*ir.Func{}
	t.toMap(result)
	return result
}

func (t *functrie) toMap(result map[string]*ir.Func) {
	if t.val != nil {
		result[t.val.Fn.Name] = t.val.Fn
		return
	}
	for _, node := range t.children {
		node.toMap(result)
	}
}
