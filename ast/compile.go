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

	// RuleTree organizes rules into a tree where each node is keyed by an
	// element in the rule's path. The rule path is the concatenation of the
	// containing package and the stringified rule name. E.g., given the following module:
	//
	//  package ex
	//  p[1] :- true
	//  p[2] :- true
	//  q :- true
	//
	//  root
	//    |
	//    +--- data (no rules)
	//           |
	//           +--- ex (no rules)
	//                |
	//                +--- p (2 rules)
	//                |
	//                +--- q (1 rule)
	RuleTree *RuleTreeNode

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

// CompileModule is a helper function to compile a module represented as a string.
func CompileModule(m string) (*Compiler, *Module, error) {

	mod, err := ParseModule("", m)
	if err != nil {
		return nil, nil, err
	}

	c := NewCompiler()

	key := WildcardPrefix
	mods := map[string]*Module{
		key: mod,
	}

	if c.Compile(mods); c.Failed() {
		return nil, nil, c.Errors[0]
	}

	return c, c.Modules[key], nil
}

// CompileQuery is a helper function to compile a query represented as a string.
func CompileQuery(q string) (*Compiler, Body, error) {

	parsed, err := ParseBody(q)
	if err != nil {
		return nil, nil, err
	}

	key := string(Wildcard.Value.(Var))

	mod := &Module{
		Package: &Package{
			Path:     Ref{DefaultRootDocument},
			Location: parsed.Loc(),
		},
		Rules: []*Rule{
			&Rule{
				Name:     Var(key),
				Body:     parsed,
				Location: parsed.Loc(),
			},
		},
	}
	mods := map[string]*Module{
		key: mod,
	}

	c := NewCompiler()

	if c.Compile(mods); c.Failed() {
		return nil, nil, c.Errors[0]
	}

	return c, c.Modules[key].Rules[0].Body, nil
}

// NewCompiler returns a new empty compiler.
func NewCompiler() *Compiler {

	c := &Compiler{
		Modules:    map[string]*Module{},
		Exports:    newExports(),
		Globals:    map[*Module]map[Var]Value{},
		RuleGraph:  map[*Rule]map[*Rule]struct{}{},
		ModuleTree: NewModuleTree(nil),
		RuleTree:   NewRuleTree(nil),
	}

	c.stages = []stage{
		stage{c.setExports, "setExports"},
		stage{c.setGlobals, "setGlobals"},
		stage{c.setModuleTree, "setModuleTree"},
		stage{c.setRuleTree, "setRuleTree"},
		stage{c.checkBuiltins, "checkBuiltins"},
		stage{c.checkSafetyHead, "checkSafetyHead"},
		stage{c.checkSafetyBody, "checkSafetyBody"},
		stage{c.resolveAllRefs, "resolveAllRefs"},
		stage{c.setRuleGraph, "setRuleGraph"},
		stage{c.checkRecursion, "checkRecursion"},
	}

	return c
}

// Compile runs the compilation process on the input modules. The compiled
// version of the modules and associated data structures are stored on the
// compiler. If the compilation process fails for any reason, the compiler will
// contain a slice of errors.
func (c *Compiler) Compile(modules map[string]*Module) {
	// TODO(tsandall): should the modules be deep copied?
	c.Modules = modules
	c.compile()
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

// GetRulesExact returns a slice of rules referred to by the reference.
//
// E.g., given the following module:
//
//	package a.b.c
//
//	p[k] = v :- ...    # rule1
//  p[k1] = v1 :- ...  # rule2
//
// The following calls yield the rules on the right.
//
//  GetRulesExact("data.a.b.c.p")   => [rule1, rule2]
//  GetRulesExact("data.a.b.c.p.x") => nil
//  GetRulesExact("data.a.b.c")     => nil
func (c *Compiler) GetRulesExact(ref Ref) (rules []*Rule) {
	node := c.RuleTree

	for _, x := range ref {
		node = node.Children[x.Value]
		if node == nil {
			return nil
		}
	}

	return node.Rules
}

// GetRulesForVirtualDocument returns a slice of rules that produce the virtual
// document referred to by the reference.
//
// E.g., given the following module:
//
//	package a.b.c
//
//	p[k] = v :- ...    # rule1
//  p[k1] = v1 :- ...  # rule2
//
// The following calls yield the rules on the right.
//
//  GetRulesForVirtualDocument("data.a.b.c.p")   => [rule1, rule2]
//  GetRulesForVirtualDocument("data.a.b.c.p.x") => [rule1, rule2]
//  GetRulesForVirtualDocument("data.a.b.c")     => nil
func (c *Compiler) GetRulesForVirtualDocument(ref Ref) (rules []*Rule) {

	node := c.RuleTree

	for _, x := range ref {
		node = node.Children[x.Value]
		if node == nil {
			return nil
		}
		if len(node.Rules) > 0 {
			return node.Rules
		}
	}

	return node.Rules
}

// checkBuiltins ensures that built-in functions are specified correctly.
//
// TODO(tsandall): in the future this should be replaced with schema checking.
func (c *Compiler) checkBuiltins() {
	for _, m := range c.Modules {
		for _, r := range m.Rules {
			vis := &GenericVisitor{
				func(x interface{}) bool {
					if expr, ok := x.(*Expr); ok {
						if ts, ok := expr.Terms.([]*Term); ok {
							if bi, ok := BuiltinMap[ts[0].Value.(Var)]; ok {
								if bi.NumArgs != len(ts[1:]) {
									c.err(expr.Location.Errorf("%v: wrong number of arguments (expression %s must specify %d arguments to built-in function %v)", r.Name, expr.Location.Text, bi.NumArgs, ts[0]))
								}
							} else {
								c.err(expr.Location.Errorf("%v: unknown built-in function %v", r.Name, ts[0]))
							}
						}
					}
					return false
				},
			}
			Walk(vis, r.Body)
		}
	}
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
			c.err(r.Location.Errorf("%v: recursive reference: %v (recursion is not allowed)", r.Name, strings.Join(n, " -> ")))
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
				for v := range unsafe.Vars() {
					c.err(r.Location.Errorf("%v: %v is unsafe (variable %v must appear in the output position of at least one non-negated expression)", r.Name, v, v))
				}
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
			unsafe := r.HeadVars().Diff(r.Body.Vars(true))
			for v := range unsafe {
				c.err(r.Location.Errorf("%v: %v is unsafe (variable %v must appear in at least one expression within the body of %v)", r.Name, v, v, r.Name))
			}
		}
	}
}

func (c *Compiler) compile() {
	for _, s := range c.stages {
		if s.f(); c.Failed() {
			return
		}
	}
}

func (c *Compiler) err(err error) {
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
			rule.Body = c.resolveRefsInBody(c.Globals[mod], rule.Body)
		}
		for i := range mod.Imports {
			mod.Imports[i].Alias = Var("")
		}
	}
}

func (c *Compiler) resolveRef(globals map[Var]Value, ref Ref) Ref {

	r := Ref{}
	for i, x := range ref {
		switch v := x.Value.(type) {
		case Var:
			if g, ok := globals[v]; ok {
				switch g := g.(type) {
				case Ref:
					if i == 0 {
						r = append(r, g...)
					} else {
						r = append(r, &Term{Location: x.Location, Value: g[:]})
					}
				case Var:
					r = append(r, &Term{Value: g})
				}
			} else {
				r = append(r, x)
			}
		case Ref:
			r = append(r, c.resolveRefsInTerm(globals, x))
		default:
			r = append(r, x)
		}
	}

	return r
}

func (c *Compiler) resolveRefsInBody(globals map[Var]Value, body Body) Body {
	r := Body{}
	for _, expr := range body {
		r = append(r, c.resolveRefsInExpr(globals, expr))
	}
	return r
}

func (c *Compiler) resolveRefsInExpr(globals map[Var]Value, expr *Expr) *Expr {
	cpy := *expr
	switch ts := expr.Terms.(type) {
	case *Term:
		cpy.Terms = c.resolveRefsInTerm(globals, ts)
	case []*Term:
		buf := []*Term{}
		for _, t := range ts {
			buf = append(buf, c.resolveRefsInTerm(globals, t))
		}
		cpy.Terms = buf
	}
	return &cpy
}

func (c *Compiler) resolveRefsInTerm(globals map[Var]Value, term *Term) *Term {
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
			k := c.resolveRefsInTerm(globals, i[0])
			v := c.resolveRefsInTerm(globals, i[1])
			o = append(o, Item(k, v))
		}
		cpy := *term
		cpy.Value = o
		return &cpy
	case Array:
		a := Array{}
		for _, e := range v {
			x := c.resolveRefsInTerm(globals, e)
			a = append(a, x)
		}
		cpy := *term
		cpy.Value = a
		return &cpy
	case *ArrayComprehension:
		ac := &ArrayComprehension{}
		ac.Term = c.resolveRefsInTerm(globals, v.Term)
		ac.Body = c.resolveRefsInBody(globals, v.Body)
		cpy := *term
		cpy.Value = ac
		return &cpy
	default:
		return term
	}
}

// setExports populates the Exports on the compiler.
// See Compiler for a description of Exports.
func (c *Compiler) setExports() {

	c.Exports = newExports()

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

func newExports() *util.HashMap {
	// TODO(tsandall): replace with ValueMap
	return util.NewHashMap(func(a, b util.T) bool {
		r1 := a.(Ref)
		r2 := a.(Ref)
		return r1.Equal(r2)
	}, func(v util.T) int {
		return v.(Ref).Hash()
	})
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
				}
			} else {
				switch p := i.Path.Value.(type) {
				case Ref:
					v := p[len(p)-1].Value.(String)
					globals[Var(v)] = p
				case Var:
					globals[p] = p
				}
			}
		}

		c.Globals[m] = globals
	}
}

func (c *Compiler) setModuleTree() {
	c.ModuleTree = NewModuleTree(c.Modules)
}

func (c *Compiler) setRuleTree() {
	c.RuleTree = NewRuleTree(c.Modules)
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
	Key      Value
	Modules  []*Module
	Children map[Value]*ModuleTreeNode
}

// NewModuleTree returns a new ModuleTreeNode that represents the root
// of the module tree populated with the given modules.
func NewModuleTree(mods map[string]*Module) *ModuleTreeNode {
	root := &ModuleTreeNode{
		Children: map[Value]*ModuleTreeNode{},
	}
	for _, m := range mods {
		node := root
		for _, x := range m.Package.Path {
			c, ok := node.Children[x.Value]
			if !ok {
				c = &ModuleTreeNode{
					Key:      x.Value,
					Children: map[Value]*ModuleTreeNode{},
				}
				node.Children[x.Value] = c
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

// RuleTreeNode represents a node in the rule tree. The rule tree is keyed by
// rule path.
type RuleTreeNode struct {
	Key      Value
	Rules    []*Rule
	Children map[Value]*RuleTreeNode
}

// NewRuleTree returns a new RuleTreeNode that represents the root
// of the rule tree populated with the given rules.
func NewRuleTree(mods map[string]*Module) *RuleTreeNode {
	root := &RuleTreeNode{
		Children: map[Value]*RuleTreeNode{},
	}
	for _, mod := range mods {
		for _, rule := range mod.Rules {
			node := root
			path := rule.Path(mod.Package.Path)
			for _, x := range path {
				c := node.Children[x.Value]
				if c == nil {
					c = &RuleTreeNode{
						Key:      x.Value,
						Children: map[Value]*RuleTreeNode{},
					}
					node.Children[x.Value] = c
				}
				node = c
			}
			node.Rules = append(node.Rules, rule)
		}
	}
	return root
}

// Size returns the number of rules in the tree.
func (n *RuleTreeNode) Size() int {
	s := len(n.Rules)
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
	return vis
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

func (vs unsafeVars) Set(e *Expr, s VarSet) {
	vs[e] = s
}

func (vs unsafeVars) Update(o unsafeVars) {
	for k, v := range o {
		if _, ok := vs[k]; !ok {
			vs[k] = VarSet{}
		}
		vs[k].Update(v)
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
	if node, ok := node.Children[ref[0].Value]; ok {
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
		if node, ok := node.Children[v]; ok {
			return findRulesRec(node, ref[1:])
		}
		result := []*Rule{}
		for _, m := range node.Modules {
			for _, r := range m.Rules {
				if String(r.Name).Equal(v) {
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

	body, unsafe := reorderBodyForClosures(globals, body)
	if len(unsafe) != 0 {
		return nil, unsafe
	}

	reordered := Body{}
	safe := VarSet{}

	for _, e := range body {
		for v := range e.Vars(true) {
			if globals.Contains(v) {
				safe.Add(v)
			} else {
				unsafe.Add(e, v)
			}
		}
	}

	for {
		n := len(reordered)

		for _, e := range body {
			if reordered.Contains(e) {
				continue
			}

			safe.Update(e.OutputVars(safe))

			for v := range unsafe[e] {
				if safe.Contains(v) {
					delete(unsafe[e], v)
				}
			}

			if len(unsafe[e]) == 0 {
				delete(unsafe, e)
				reordered = append(reordered, e)
			}
		}

		if len(reordered) == n {
			break
		}
	}

	// Recursively visit closures and perform the safety checks on them.
	// Update the globals at each expression to include the variables that could
	// be closed over.
	g := globals.Copy()
	for i, e := range reordered {
		if i > 0 {
			g.Update(reordered[i-1].Vars(true))
		}
		vis := &bodySafetyVisitor{
			current: e,
			globals: g,
			unsafe:  unsafe,
		}
		Walk(vis, e)
	}

	return reordered, unsafe
}

type bodySafetyVisitor struct {
	current *Expr
	globals VarSet
	unsafe  unsafeVars
}

func (vis *bodySafetyVisitor) Visit(x interface{}) Visitor {
	switch x := x.(type) {
	case *Expr:
		cpy := *vis
		cpy.current = x
		return &cpy
	case *ArrayComprehension:
		vis.checkArrayComprehensionSafety(x)
		return nil
	}
	return vis
}

func (vis *bodySafetyVisitor) checkArrayComprehensionSafety(ac *ArrayComprehension) {
	// Check term for safety. This is analagous to the rule head safety check.
	tv := ac.Term.Vars()
	bv := ac.Body.Vars(true)
	bv.Update(vis.globals)
	uv := tv.Diff(bv)
	for v := range uv {
		vis.unsafe.Add(vis.current, v)
	}

	// Check body for safety, reordering as necessary.
	r, u := reorderBodyForSafety(vis.globals, ac.Body)
	if len(u) == 0 {
		ac.Body = r
	} else {
		vis.unsafe.Update(u)
	}
}

// reorderBodyForClosures returns a copy of the body ordered such that
// expressions (such as array comprehensions) that close over variables are ordered
// after other expressions that contain the same variable in an output position.
func reorderBodyForClosures(globals VarSet, body Body) (Body, unsafeVars) {

	reordered := Body{}
	unsafe := unsafeVars{}

	for {
		n := len(reordered)

		for _, e := range body {
			if reordered.Contains(e) {
				continue
			}

			// Collect vars that are contained in closures within this
			// expression.
			vs := VarSet{}
			WalkClosures(e, func(x interface{}) bool {
				vis := &varVisitor{vars: vs}
				Walk(vis, x)
				return true
			})

			// Compute vars that are closed over from the body but not yet
			// contained in the output position of an expression in the reordered
			// body. These vars are considered unsafe.
			cv := vs.Intersect(body.Vars(true)).Diff(globals)
			uv := cv.Diff(reordered.OutputVars(globals))

			if len(uv) == 0 {
				reordered = append(reordered, e)
				delete(unsafe, e)
			} else {
				unsafe.Set(e, uv)
			}
		}

		if len(reordered) == n {
			break
		}
	}

	return reordered, unsafe
}
