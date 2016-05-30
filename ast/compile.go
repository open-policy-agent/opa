// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"fmt"
	"strings"

	"github.com/open-policy-agent/opa/util"
)

// Compiler contains the state of a compilation process.
type Compiler struct {

	// Errors contains errors that occurred during the compilation process.
	// If there are one or more errors, the compilation process is considered
	// "failed".
	Errors []error

	// Modules contains the compiled modules. The compiled modules are the
	// output of the compilation process. If the compilation process failed,
	// there is no guarantee about the state of the modules.
	Modules map[string]*Module

	// Exports contains a mapping of package paths to variables. The variables
	// represent externally accessible symbols. For now the only type of
	// externally visible symbol is a rule. For example:
	//
	// package a.b.c
	//
	// import data.e.f
	//
	// p = true :- q[x] = 1         # "p" is an export
	// q[x] :- f.r[x], not f.m[x]   # "q" is an export
	//
	// In this case, the mapping would be:
	//
	// {
	//   a.b.c: [p, q]
	// }
	Exports *util.HashMap

	// Globals contains a mapping of modules to globally accessible variables
	// within each module. Each variable is mapped to the value which represents
	// the fully qualified name of the variable. For example:
	//
	// package a.b.c
	//
	// import data.e.f
	// import y as z
	//
	// p = true :- q[x] = 1
	// q[x] :- f.r[x], not f.m[x]
	//
	// In this case, the mapping would be
	//
	// {
	//  <modptr>: {q: data.a.b.c.q, f: data.e.f, p: data.a.b.c.p, z: y}
	// }
	Globals map[*Module]map[Var]Value

	// ModuleTree organizes the modules into a tree where each node is keyed
	// by an element in the module's package path. E.g., given modules containg
	// the following package directives: "a", "a.b", "a.c", and "a.b", the
	// resulting module tree would be:
	//
	//  root
	//    |
	//    +--- data (no modules)
	//           |
	//           +--- a (1 module)
	//                |
	//                +--- b (2 modules)
	//                |
	//                +--- c (1 module)
	//
	ModuleTree *ModuleTreeNode

	// RuleGraph represents the rule dependencies.
	// An edge (u, v) is added to the graph if rule "u" depends on rule "v".
	// A rule depends on another rule if it refers to it.
	RuleGraph map[*Rule]map[*Rule]struct{}

	stages []stage
}

type stage struct {
	f    func()
	name string
}

// NewCompiler returns a new empty compiler.
func NewCompiler() *Compiler {

	c := &Compiler{
		Globals:   map[*Module]map[Var]Value{},
		RuleGraph: map[*Rule]map[*Rule]struct{}{},
	}

	c.stages = []stage{
		stage{c.setExports, "setExports"},
		stage{c.setGlobals, "setGlobals"},
		stage{c.setModuleTree, "setModuleTree"},
		stage{c.checkSafetyHead, "checkSafetyHead"},
		stage{c.checkSafetyBody, "checkSafetyBody"},
		stage{c.resolveAllRefs, "resolveAllRefs"},
		stage{c.setRuleGraph, "setRuleGraph"},
		stage{c.checkRecursion, "checkRecursion"},
	}

	return c
}

// Compile runs the compilation process on the input modules.
// The output of the compilation process can be obtained from
// the Errors or Modules attributes of the Compiler.
func (c *Compiler) Compile(mods map[string]*Module) {

	// TODO(tsandall): need to revisit the error messages. E.g.,
	// errors local to a rule should include rule name, package path,
	// and potentially a snippet of text identifying the source of the
	// the problem. In some cases a useful tip could be provided, e.g.,
	// "Did you mean to assign 'u' to something?"
	//
	// TODO(tsandall): should the modules be deep copied?

	c.Modules = mods

	for _, s := range c.stages {
		if s.f(); c.Failed() {
			return
		}
	}
}

// Failed returns true if a compilation error has been encountered.
func (c *Compiler) Failed() bool {
	return len(c.Errors) > 0
}

// FlattenErrors returns a single message that contains a flattened version of the compiler error messages.
// This must only be called when the compilation process has failed.
func (c *Compiler) FlattenErrors() string {

	if len(c.Errors) == 0 {
		panic(fmt.Sprintf("illegal call: %v", c))
	}

	if len(c.Errors) == 1 {
		return fmt.Sprintf("1 error occurred: %v", c.Errors[0].Error())
	}

	b := []string{}
	for _, err := range c.Errors {
		b = append(b, err.Error())
	}

	return fmt.Sprintf("%d errors occurred:\n%s", len(c.Errors), strings.Join(b, "\n"))
}

// checkRecursion ensures that there are no recursive rule definitions, i.e., there are
// no cycles in the RuleGraph.
func (c *Compiler) checkRecursion() {
	for r := range c.RuleGraph {
		t := &ruleGraphTraveral{
			graph:   c.RuleGraph,
			visited: map[*Rule]struct{}{},
		}
		if p := util.DFS(t, r, r); len(p) > 0 {
			n := []string{}
			for _, x := range p {
				n = append(n, string(x.(*Rule).Name))
			}
			c.err("recursion found in %v: %v", r.Name, strings.Join(n, ", "))
		}
	}
}

// checkSafetyBody ensures that variables appearing in negated expressions or non-target
// positions of built-in expressions will be bound when evaluating the rule from left
// to right, re-ordering as necessary.
func (c *Compiler) checkSafetyBody() {
	for _, m := range c.Modules {
		globals := ReservedVars.Copy()
		for v := range c.Globals[m] {
			globals.Add(v)
		}
		for _, r := range m.Rules {
			reordered, unsafe := reorderBodyForSafety(globals, r.Body)
			if len(unsafe) != 0 {
				c.err("unsafe variables in %v: %v", r.Name, unsafe.Vars())
			} else {
				r.Body = reordered
			}
		}
	}
}

// checkSafetyHeads ensures that variables appearing in the head of a
// rule also appear in the body.
func (c *Compiler) checkSafetyHead() {
	for _, m := range c.Modules {
		for _, r := range m.Rules {
			headVars := r.HeadVars()
			bodyVars := r.Body.Vars()
			for headVar := range headVars {
				if _, ok := bodyVars[headVar]; !ok {
					c.err("unsafe variable from head of %v: %v", r.Name, headVar)
				}
			}
		}
	}
}

func (c *Compiler) err(f string, a ...interface{}) {
	err := fmt.Errorf(f, a...)
	c.Errors = append(c.Errors, err)
}

// resolveAllRefs resolves references in expressions to their fully qualified values.
//
// For instance, given the following module:
//
// package a.b
// import data.foo.bar
// p[x] :- bar[_] = x
//
// The reference "bar[_]" would be resolved to "data.foo.bar[_]".
func (c *Compiler) resolveAllRefs() {
	for _, mod := range c.Modules {
		for _, rule := range mod.Rules {
			for _, expr := range rule.Body {
				switch ts := expr.Terms.(type) {
				case *Term:
					expr.Terms = c.resolveRefs(c.Globals[mod], ts)
				case []*Term:
					for i, t := range ts {
						ts[i] = c.resolveRefs(c.Globals[mod], t)
					}
				}
			}
		}
		for i := range mod.Imports {
			mod.Imports[i].Alias = Var("")
		}
	}
}

func (c *Compiler) resolveRef(globals map[Var]Value, ref Ref) Ref {

	global := globals[ref[0].Value.(Var)]
	if global == nil {
		return ref
	}
	fqn := Ref{}
	switch global := global.(type) {
	case Ref:
		fqn = append(fqn, global...)
		for _, p := range ref[1:] {
			switch v := p.Value.(type) {
			case Var:
				global := globals[v]
				if global != nil {
					_, isRef := global.(Ref)
					if isRef {
						c.err("nested references in %v: %v => %v", ref, v, global)
						return ref
					}
					fqn = append(fqn, &Term{Location: p.Location, Value: global})
				} else {
					fqn = append(fqn, p)
				}
			default:
				fqn = append(fqn, p)
			}
		}
	case Var:
		fqn = append(fqn, &Term{Value: global})
		fqn = append(fqn, ref[1:]...)
	default:
		c.err("unexpected %T: %v", global, global)
		return ref
	}

	return fqn
}

func (c *Compiler) resolveRefs(globals map[Var]Value, term *Term) *Term {
	switch v := term.Value.(type) {
	case Var:
		if r, ok := globals[v]; ok {
			cpy := *term
			cpy.Value = r
			return &cpy
		}
		return term
	case Ref:
		fqn := c.resolveRef(globals, v)
		cpy := *term
		cpy.Value = fqn
		return &cpy
	case Object:
		o := Object{}
		for _, i := range v {
			k := c.resolveRefs(globals, i[0])
			v := c.resolveRefs(globals, i[1])
			o = append(o, Item(k, v))
		}
		cpy := *term
		cpy.Value = o
		return &cpy
	case Array:
		a := Array{}
		for _, e := range v {
			x := c.resolveRefs(globals, e)
			a = append(a, x)
		}
		cpy := *term
		cpy.Value = a
		return &cpy
	default:
		return term
	}
}

// setExports populates the Exports on the compiler.
// See Compiler for a description of Exports.
func (c *Compiler) setExports() {

	c.Exports = util.NewHashMap(func(a, b util.T) bool {
		r1 := a.(Ref)
		r2 := a.(Ref)
		return r1.Equal(r2)
	}, func(v util.T) int {
		return v.(Ref).Hash()
	})

	for _, mod := range c.Modules {
		for _, rule := range mod.Rules {
			v, ok := c.Exports.Get(mod.Package.Path)
			if !ok {
				v = []Var{}
			}
			vars := v.([]Var)
			vars = append(vars, rule.Name)
			c.Exports.Put(mod.Package.Path, vars)
		}
	}

}

// setGlobals populates the Globals on the compiler.
// See Compiler for a description of Globals.
func (c *Compiler) setGlobals() {

	for _, m := range c.Modules {

		p := m.Package.Path
		v, ok := c.Exports.Get(p)
		if !ok {
			continue
		}

		exports := v.([]Var)
		globals := map[Var]Value{}

		// Populate globals with exports within the package.
		for _, v := range exports {
			global := append(Ref{}, p...)
			global = append(global, &Term{Value: String(v)})
			globals[v] = global
		}

		// Populate globals with imports within this module.
		for _, i := range m.Imports {
			if len(i.Alias) > 0 {
				switch p := i.Path.Value.(type) {
				case Ref:
					globals[i.Alias] = p
				case Var:
					globals[i.Alias] = p
				default:
					c.err("unexpected %T: %v", p, i)
				}
			} else {
				switch p := i.Path.Value.(type) {
				case Ref:
					switch v := p[len(p)-1].Value.(type) {
					case String:
						globals[Var(v)] = p
					default:
						c.err("unexpected %T: %v", v, i)
					}
				case Var:
					globals[p] = p
				default:
					c.err("unexpected %T: %v", i.Path, i.Path)
				}
			}
		}

		c.Globals[m] = globals
	}
}

func (c *Compiler) setModuleTree() {
	c.ModuleTree = NewModuleTree(c.Modules)
}

func (c *Compiler) setRuleGraph() {
	for _, m := range c.Modules {
		for _, r := range m.Rules {
			// Walk over all of the references in the rule body and
			// lookup the rules they may refer to. These rules are
			// the dependencies of the current rule. Add these dependencies
			// to the graph.
			edges, ok := c.RuleGraph[r]

			if !ok {
				edges = map[*Rule]struct{}{}
				c.RuleGraph[r] = edges
			}

			vis := &ruleGraphBuilder{
				moduleTree: c.ModuleTree,
				edges:      edges,
			}

			Walk(vis, r.Body)
		}
	}
}

// ModuleTreeNode represents a node in the module tree. The module
// tree is keyed by the package path.
type ModuleTreeNode struct {
	Key      string
	Modules  []*Module
	Children map[string]*ModuleTreeNode
}

// NewModuleTree returns a new ModuleTreeNode that represents the root
// of the module tree populated with the given modules.
func NewModuleTree(mods map[string]*Module) *ModuleTreeNode {
	root := &ModuleTreeNode{
		Children: map[string]*ModuleTreeNode{},
	}
	for _, m := range mods {
		node := root
		for _, x := range m.Package.Path {
			var s string
			switch v := x.Value.(type) {
			case Var:
				s = string(v)
			case String:
				s = string(v)
			}
			c, ok := node.Children[s]
			if !ok {
				c = &ModuleTreeNode{
					Key:      s,
					Children: map[string]*ModuleTreeNode{},
				}
				node.Children[s] = c
			}
			node = c
		}
		node.Modules = append(node.Modules, m)
	}
	return root
}

// Size returns the number of modules in the tree.
func (n *ModuleTreeNode) Size() int {
	s := len(n.Modules)
	for _, c := range n.Children {
		s += c.Size()
	}
	return s
}

type ruleGraphBuilder struct {
	moduleTree *ModuleTreeNode
	edges      map[*Rule]struct{}
}

func (vis *ruleGraphBuilder) Visit(v interface{}) Visitor {
	ref, ok := v.(Ref)
	if !ok {
		return vis
	}
	for _, v := range findRules(vis.moduleTree, ref) {
		vis.edges[v] = struct{}{}
	}
	return nil
}

type ruleGraphTraveral struct {
	graph   map[*Rule]map[*Rule]struct{}
	visited map[*Rule]struct{}
}

func (g *ruleGraphTraveral) Edges(x util.T) []util.T {
	u := x.(*Rule)
	edges := g.graph[u]
	r := []util.T{}
	for v := range edges {
		r = append(r, v)
	}
	return r
}

func (g *ruleGraphTraveral) Visited(x util.T) bool {
	u := x.(*Rule)
	_, ok := g.visited[u]
	g.visited[u] = struct{}{}
	return ok
}

func (g *ruleGraphTraveral) Equals(a, b util.T) bool {
	return a.(*Rule) == b.(*Rule)
}

type unsafeVars map[*Expr]VarSet

func (vs unsafeVars) Add(e *Expr, v Var) {
	if u, ok := vs[e]; ok {
		u[v] = struct{}{}
	} else {
		vs[e] = VarSet{v: struct{}{}}
	}
}

func (vs unsafeVars) Vars() VarSet {
	r := VarSet{}
	for _, s := range vs {
		r.Update(s)
	}
	return r
}

// findRules returns a slice of rules that are referred to
// by the reference "ref". For example, suppose a package
// "a.b.c" contains rules "p" and "q". If this function
// is called with a ref "a.b.c.p" (or a.b.c.p[x] or ...) the
// result would contain a single value: rule p. If this function
// is called with "a.b.c", the result would be empty. Lastly,
// if this function is called with a reference containing
// variables, such as: "a.b.c[x]", the result will contain
// rule "p" and rule "q".
func findRules(node *ModuleTreeNode, ref Ref) []*Rule {
	k := string(ref[0].Value.(Var))
	if node, ok := node.Children[k]; ok {
		return findRulesRec(node, ref[1:])
	}
	return nil
}

func findRulesRec(node *ModuleTreeNode, ref Ref) []*Rule {
	if len(ref) == 0 {
		return nil
	}
	switch v := ref[0].Value.(type) {
	case Var:
		result := []*Rule{}
		tail := ref[1:]
		for _, n := range node.Children {
			result = append(result, findRulesRec(n, tail)...)
		}
		for _, m := range node.Modules {
			result = append(result, m.Rules...)
		}
		return result
	case String:
		k := string(v)
		if node, ok := node.Children[k]; ok {
			return findRulesRec(node, ref[1:])
		}
		result := []*Rule{}
		for _, m := range node.Modules {
			for _, r := range m.Rules {
				if string(r.Name) == k {
					result = append(result, r)
				}
			}
		}
		return result
	default:
		return nil
	}
}

// reorderBodyForSafety returns a copy of the body ordered such that
// left to right evaluation of the body will not encounter unbound variables
// in input positions or negated expressions.
//
// Expressions are added to the re-ordered body as soon as they are considered
// safe. If multiple expressions become safe in the same pass, they are added
// in their original order. This results in minimal re-ordering of the body.
//
// If the body cannot be reordered to ensure safety, the second return value
// contains a mapping of expressions to unsafe variables in those expressions.
func reorderBodyForSafety(globals VarSet, body Body) (Body, unsafeVars) {

	reordered := Body{}
	unsafe := unsafeVars{}

	for _, e := range body {
		for v := range e.Vars() {
			if !globals.Contains(v) {
				unsafe.Add(e, v)
			}
		}
	}

	safe := VarSet{}

	for _, e := range body {

		safe.Update(e.OutputVars())

		for v := range unsafe[e] {
			if safe.Contains(v) {
				delete(unsafe[e], v)
			}
		}

		if len(unsafe[e]) == 0 {
			reordered = append(reordered, e)
			delete(unsafe, e)

			// Check if other expressions in the body are considered safe
			// now. If they are considered safe now, they can be added
			// to the end of the re-ordered body.
			for _, e := range body {
				if reordered.Contains(e) {
					continue
				}
				for v := range unsafe[e] {
					if safe.Contains(v) {
						delete(unsafe[e], v)
					}
				}
				if len(unsafe[e]) == 0 {
					reordered = append(reordered, e)
					delete(unsafe, e)
				}
			}
		}
	}

	return reordered, unsafe
}
