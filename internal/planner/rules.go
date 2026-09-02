package planner

import (
	"fmt"
	"slices"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/util"
)

// funcstack implements a simple map structure used to keep track of virtual
// document => planned function names. The structure supports Push and Pop
// operations so that the planner can shadow planned functions when 'with'
// statements are found.
// The "gen" numbers indicate the "generations"; whenever a 'with' statement
// is planned (a new map is `Push()`ed), it will jump to a previously unused
// number.
type funcstack struct {
	stack []taggedPairs
	next  int
}

type taggedPairs struct {
	pairs  map[string]string
	vars   []ast.Var
	vcount int
	gen    int
}

func newFuncstack() *funcstack {
	return &funcstack{
		stack: []taggedPairs{
			{
				pairs: map[string]string{},
				gen:   0,
				vars: []ast.Var{
					ast.InputRootDocument.Value.(ast.Var),
					ast.DefaultRootDocument.Value.(ast.Var),
				},
				vcount: 2,
			},
		},
		next: 1}
}

func (p funcstack) last() taggedPairs {
	return p.stack[len(p.stack)-1]
}

func (p funcstack) argVars() int {
	return p.last().vcount
}

func (p funcstack) vars() []ast.Var {
	ret := make([]ast.Var, 0, p.last().vcount)
	for i := range p.stack {
		ret = append(ret, p.stack[i].vars...)
	}
	return ret
}

func (p funcstack) Add(key, value string) {
	p.last().pairs[key] = value
}

func (p funcstack) Get(key string) (string, bool) {
	value, ok := p.last().pairs[key]
	return value, ok
}

func (p *funcstack) Push(funcs map[string]string, vars []ast.Var) {
	p.stack = append(p.stack, taggedPairs{
		pairs:  funcs,
		gen:    p.next,
		vars:   vars,
		vcount: p.last().vcount + len(vars),
	})
	p.next++
}

func (p *funcstack) Pop() map[string]string {
	last := p.last()
	p.stack = p.stack[:len(p.stack)-1]
	return last.pairs
}

func (p funcstack) gen() int {
	return p.last().gen
}

// ruletrie implements a simple trie structure for organizing rules that may be
// planned. The trie nodes are keyed by the rule path. The ruletrie supports
// Push and Pop operations that allow the planner to shadow subtrees when 'with'
// statements are found.
type ruletrie struct {
	children map[ast.Value][]*ruletrie
	rules    []*ast.Rule
}

func newRuletrie() *ruletrie {
	return &ruletrie{
		children: map[ast.Value][]*ruletrie{},
	}
}

func (t *ruletrie) Arity() int {
	rules := t.Rules()
	if len(rules) > 0 {
		return len(rules[0].Head.Args)
	}
	return 0
}

func (t *ruletrie) Rules() []*ast.Rule {
	if t != nil {
		if t.rules == nil {
			return nil
		}
		rules := make([]*ast.Rule, len(t.rules), len(t.rules)+len(t.children)) // could be too little
		copy(rules, t.rules)

		// NOTE(sr): We pull in one layer of children: the compiler ensures
		// that these are the only possible, relevant rule sources for a given
		// ref: If the trie is what we get for
		//
		//     a.b.c  = 1 { ... }
		//     a.b[x] = 2 { ... }
		//
		// and we're retrieving a.b, we want Rules() to include the rule body
		// of a.b.c.
		// FIXME: We need to go deeper than just immediate children (?)
		for _, rs := range t.children {
			if r := rs[len(rs)-1].rules; r != nil {
				rules = append(rules, r...)
			}
		}
		return rules
	}
	return nil
}

func (t *ruletrie) Push(key ast.Ref) {
	node := t
	for i := range len(key) - 1 {
		node = node.Get(key[i].Value)
		if node == nil {
			return
		}
	}
	elem := key[len(key)-1]
	node.children[elem.Value] = append(node.children[elem.Value], nil)
}

func (t *ruletrie) Pop(key ast.Ref) {
	node := t
	for i := range len(key) - 1 {
		node = node.Get(key[i].Value)
		if node == nil {
			return
		}
	}
	elem := key[len(key)-1]
	sl := node.children[elem.Value]
	node.children[elem.Value] = sl[:len(sl)-1]
}

func (t *ruletrie) Insert(key ast.Ref) *ruletrie {
	node := t
	for _, elem := range key {
		child := node.Get(elem.Value)
		if child == nil {
			child = newRuletrie()
			node.children[elem.Value] = append(node.children[elem.Value], child)
		}
		node = child
	}
	return node
}

func (t *ruletrie) Lookup(key ast.Ref) *ruletrie {
	node := t
	for _, elem := range key {
		node = node.Get(elem.Value)
		if node == nil {
			return nil
		}
	}
	return node
}

func (t *ruletrie) LookupShallowest(key ast.Ref) *ruletrie {
	node := t
	for _, elem := range key {
		node = node.Get(elem.Value)
		if node == nil {
			return nil
		}
		if len(node.rules) > 0 {
			return node
		}
	}
	return node
}

// TODO: Collapse rules with overlapping extent to same node(?)
func (t *ruletrie) LookupOrInsert(key ast.Ref) *ruletrie {
	if val := t.LookupShallowest(key); val != nil {

		return val
	}
	return t.Insert(key)
}

func (t *ruletrie) DescendantRules() []*ast.Rule {
	if len(t.children) == 0 {
		return t.rules
	}

	rules := make([]*ast.Rule, len(t.rules), len(t.rules)+len(t.children)) // could be too little
	copy(rules, t.rules)

	for _, cs := range t.children {
		for _, c := range cs {
			rules = append(rules, c.DescendantRules()...)
		}
	}

	return rules
}

func (t *ruletrie) ChildrenCount() int {
	return len(t.children)
}

func (t *ruletrie) Children() []ast.Value {
	if t == nil {
		return nil
	}
	sorted := make([]ast.Value, 0, len(t.children))
	for key := range t.children {
		if t.Get(key) != nil {
			sorted = append(sorted, key)
		}
	}
	return util.SortedFunc(sorted, ast.Value.Compare)
}

func (t *ruletrie) Get(k ast.Value) *ruletrie {
	if t == nil {
		return nil
	}
	nodes := t.children[k]
	if len(nodes) == 0 {
		return nil
	}
	return nodes[len(nodes)-1]
}

func (t *ruletrie) DepthFirst(f func(*ruletrie) bool) {
	if f(t) {
		return
	}
	for _, rules := range t.children {
		for i := range rules {
			rules[i].DepthFirst(f)
		}
	}
}

func (t *ruletrie) Depth() int {
	// Avoid Children()'s slice allocation and sort: we only need the max
	// depth over the child nodes. A nil last element is a pushed but
	// not-yet-inserted node (see Push), matching Children()'s filter.
	max := 0
	found := false
	for _, nodes := range t.children {
		last := nodes[len(nodes)-1]
		if last == nil {
			continue
		}
		found = true
		if d := last.Depth(); d > max {
			max = d
		}
	}
	if !found {
		return 0
	}
	return max + 1
}

func (t *ruletrie) String() string {
	return fmt.Sprintf("<ruletrie rules:%v children:%v>", t.rules, t.children)
}

type functionMocksStack struct {
	stack util.GroupStack[frame]
}

type frame map[string]*ast.Term

func newFunctionMocksStack() *functionMocksStack {
	stack := &functionMocksStack{}
	stack.Push()
	return stack
}

func (s *functionMocksStack) Push() {
	s.stack.PushGroup(nil)
}

func (s *functionMocksStack) Pop() {
	s.stack.PopGroup()
}

func (s *functionMocksStack) PushFrame(f frame) {
	s.stack.Push(f)
}

func (s *functionMocksStack) PopFrame() {
	s.stack.Pop()
}

func (s *functionMocksStack) Lookup(f string) *ast.Term {
	current := s.stack.PeekGroup()
	for _, c := range slices.Backward(current) {
		if t, ok := c[f]; ok {
			return t
		}
	}
	return nil
}
