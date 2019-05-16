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
	fn       *ir.Func
}

func newFunctrie() *functrie {
	return &functrie{
		children: map[ast.Value]*functrie{},
	}
}

func (t *functrie) Insert(key ast.Ref, fn *ir.Func) {
	node := t
	for _, elem := range key {
		child, ok := node.children[elem.Value]
		if !ok {
			child = newFunctrie()
			node.children[elem.Value] = child
		}
		node = child
	}
	node.fn = fn
}

func (t *functrie) Lookup(key ast.Ref) *ir.Func {
	node := t
	for _, elem := range key {
		var ok bool
		if node == nil {
			return nil
		} else if node, ok = node.children[elem.Value]; !ok {
			return nil
		}
	}
	return node.fn
}

func (t *functrie) Map() map[string]*ir.Func {
	result := map[string]*ir.Func{}
	t.toMap(result)
	return result
}

func (t *functrie) toMap(result map[string]*ir.Func) {
	if t.fn != nil {
		result[t.fn.Name] = t.fn
		return
	}
	for _, node := range t.children {
		node.toMap(result)
	}
}
