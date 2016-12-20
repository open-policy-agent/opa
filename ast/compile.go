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
	Errors Errors

	// Modules contains the compiled modules. The compiled modules are the
	// output of the compilation process. If the compilation process failed,
	// there is no guarantee about the state of the modules.
	Modules map[string]*Module

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

	moduleLoader ModuleLoader
	stages       []stage
}

// QueryContext contains contextual information for running an ad-hoc query.
//
// Ad-hoc queries can be run in the context of a package and imports may be
// included to provide concise access to data.
type QueryContext struct {
	Package *Package
	Imports []*Import
}

// NewQueryContext returns a new QueryContext object.
func NewQueryContext(pkg *Package, imports []*Import) *QueryContext {
	return &QueryContext{
		Package: pkg,
		Imports: imports,
	}
}

// NewQueryContextForModule returns a new QueryContext object based on the
// provided module.
func NewQueryContextForModule(mod *Module) *QueryContext {
	return NewQueryContext(mod.Package, mod.Imports)
}

// Copy returns a deep copy of qc.
func (qc *QueryContext) Copy() *QueryContext {
	if qc == nil {
		return nil
	}
	cpy := *qc
	cpy.Package = qc.Package.Copy()
	cpy.Imports = make([]*Import, len(qc.Imports))
	for i := range qc.Imports {
		cpy.Imports[i] = qc.Imports[i].Copy()
	}
	return &cpy
}

// QueryCompiler defines the interface for compiling ad-hoc queries.
type QueryCompiler interface {

	// Compile should be called to compile ad-hoc queries. The return value is
	// the compiled version of the query.
	Compile(q Body) (Body, error)

	// WithContext sets the QueryContext on the QueryCompiler. Subsequent calls
	// to Compile will take the QUeryContext into account.
	WithContext(qctx *QueryContext) QueryCompiler
}

type stage struct {
	f    func()
	name string
}

// NewCompiler returns a new empty compiler.
func NewCompiler() *Compiler {

	c := &Compiler{
		Modules:    map[string]*Module{},
		RuleGraph:  map[*Rule]map[*Rule]struct{}{},
		ModuleTree: NewModuleTree(nil),
		RuleTree:   NewRuleTree(nil),
	}

	c.stages = []stage{
		stage{c.resolveAllRefs, "resolveAllRefs"},
		stage{c.setModuleTree, "setModuleTree"},
		stage{c.setRuleTree, "setRuleTree"},
		stage{c.setRuleGraph, "setRuleGraph"},
		stage{c.rewriteRefsInHead, "rewriteRefsInHead"},
		stage{c.checkRuleConflicts, "checkRuleConflicts"},
		stage{c.checkBuiltins, "checkBuiltins"},
		stage{c.checkSafetyRuleHeads, "checkSafetyRuleHeads"},
		stage{c.checkSafetyRuleBodies, "checkSafetyRuleBodies"},
		stage{c.checkRecursion, "checkRecursion"},
	}

	return c
}

// QueryCompiler returns a new QueryCompiler object.
func (c *Compiler) QueryCompiler() QueryCompiler {
	return newQueryCompiler(c)
}

// Compile runs the compilation process on the input modules. The compiled
// version of the modules and associated data structures are stored on the
// compiler. If the compilation process fails for any reason, the compiler will
// contain a slice of errors.
func (c *Compiler) Compile(modules map[string]*Module) {
	c.Modules = make(map[string]*Module, len(modules))
	for k, v := range modules {
		c.Modules[k] = v.Copy()
	}
	c.compile()
}

// Failed returns true if a compilation error has been encountered.
func (c *Compiler) Failed() bool {
	return len(c.Errors) > 0
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

// GetRulesWithPrefix returns a slice of rules that share the prefix ref.
//
// E.g., given the following module:
//
//  package a.b.c
//
//  p[x] = y :- ...  # rule1
//  p[k] = v :- ...  # rule2
//  q :- ...         # rule3
//
// The following calls yield the rules on the right.
//
//  GetRulesWithPrefix("data.a.b.c.p")   => [rule1, rule2]
//  GetRulesWithPrefix("data.a.b.c.p.a") => nil
//  GetRulesWithPrefix("data.a.b.c")     => [rule1, rule2, rule3]
func (c *Compiler) GetRulesWithPrefix(ref Ref) (rules []*Rule) {

	node := c.RuleTree

	for _, x := range ref {
		node = node.Children[x.Value]
		if node == nil {
			return nil
		}
	}

	var acc func(node *RuleTreeNode)

	acc = func(node *RuleTreeNode) {
		rules = append(rules, node.Rules...)
		for _, child := range node.Children {
			acc(child)
		}
	}

	acc(node)

	return rules
}

// ModuleLoader defines the interface that callers can implement to enable lazy
// loading of modules during compilation.
type ModuleLoader func(resolved map[string]*Module) (parsed map[string]*Module, err error)

// WithModuleLoader sets f as the ModuleLoader on the compiler.
//
// The compiler will invoke the ModuleLoader after resolving all references in
// the current set of input modules. The ModuleLoader can return a new
// collection of parsed modules that are to be included in the compilation
// process. This process will repeat until the ModuleLoader returns an empty
// collection or an error. If an error is returned, compilation will stop
// immediately.
func (c *Compiler) WithModuleLoader(f ModuleLoader) *Compiler {
	c.moduleLoader = f
	return c
}

// checkBuiltins ensures that built-in functions are specified correctly.
func (c *Compiler) checkBuiltins() {
	for _, mod := range c.Modules {
		bc := newBuiltinChecker()
		for _, err := range bc.Check(mod) {
			c.err(err)
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
			c.err(NewError(RecursionErr, r.Location, "%v: recursive reference: %v (recursion is not allowed)", r.Name, strings.Join(n, " -> ")))
		}
	}
}

// checkRuleConflicts ensures that rules definitions are not in conflict.
func (c *Compiler) checkRuleConflicts() {
	c.RuleTree.DepthFirst(func(node *RuleTreeNode) bool {
		if len(node.Rules) == 0 {
			return false
		}

		kinds := map[DocKind]struct{}{}
		for _, rule := range node.Rules {
			kinds[rule.DocKind()] = struct{}{}
		}

		if len(kinds) > 1 {
			name := Var(node.Key.(String))
			c.err(NewError(CompileErr, node.Rules[0].Loc(), "%v: conflicting rule types (all definitions of %v must have the same type)", name, name))
		}

		return false
	})

	c.ModuleTree.DepthFirst(func(node *ModuleTreeNode) bool {
		for _, mod := range node.Modules {
			for _, rule := range mod.Rules {
				if childNode, ok := node.Children[String(rule.Name)]; ok {
					for _, childMod := range childNode.Modules {
						msg := fmt.Sprintf("%v: package declaration conflicts with rule defined at %v", childMod.Package, rule.Loc())
						c.err(NewError(CompileErr, mod.Package.Loc(), msg))
					}
				}
			}
		}
		return false
	})
}

// checkSafetyRuleBodies ensures that variables appearing in negated expressions or non-target
// positions of built-in expressions will be bound when evaluating the rule from left
// to right, re-ordering as necessary.
func (c *Compiler) checkSafetyRuleBodies() {
	for _, m := range c.Modules {
		safe := ReservedVars.Copy()
		for i := range m.Imports {
			safe.Add(m.Imports[i].Path.Value.(Var))
		}
		for _, r := range m.Rules {
			reordered, unsafe := reorderBodyForSafety(safe, r.Body)
			if len(unsafe) != 0 {
				for v := range unsafe.Vars() {
					c.err(NewError(UnsafeVarErr, r.Location, "%v: %v is unsafe (variable %v must appear in the output position of at least one non-negated expression)", r.Name, v, v))
				}
			} else {
				r.Body = reordered
			}
		}
	}
}

// checkSafetyRuleHeads ensures that variables appearing in the head of a
// rule also appear in the body.
func (c *Compiler) checkSafetyRuleHeads() {
	for _, m := range c.Modules {
		for _, r := range m.Rules {
			unsafe := r.HeadVars().Diff(r.Body.Vars(VarVisitorParams{SkipClosures: true}))
			for v := range unsafe {
				c.err(NewError(UnsafeVarErr, r.Location, "%v: %v is unsafe (variable %v must appear in at least one expression within the body of %v)", r.Name, v, v, r.Name))
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

func (c *Compiler) err(err *Error) {
	c.Errors = append(c.Errors, err)
}

func (c *Compiler) getExports() *util.HashMap {

	exports := util.NewHashMap(func(a, b util.T) bool {
		r1 := a.(Ref)
		r2 := a.(Ref)
		return r1.Equal(r2)
	}, func(v util.T) int {
		return v.(Ref).Hash()
	})

	for _, mod := range c.Modules {
		for _, rule := range mod.Rules {
			v, ok := exports.Get(mod.Package.Path)
			if !ok {
				v = []Var{}
			}
			vars := v.([]Var)
			vars = append(vars, rule.Name)
			exports.Put(mod.Package.Path, vars)
		}
	}

	return exports
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

	exports := c.getExports()

	for _, mod := range c.Modules {

		var exportsForPackage []Var
		if x, ok := exports.Get(mod.Package.Path); ok {
			exportsForPackage = x.([]Var)
		}

		globals := getGlobals(mod.Package, exportsForPackage, mod.Imports)

		for _, rule := range mod.Rules {
			if rule.Key != nil {
				rule.Key = resolveRefsInTerm(globals, rule.Key)
			}
			if rule.Value != nil {
				rule.Value = resolveRefsInTerm(globals, rule.Value)
			}
			rule.Body = resolveRefsInBody(globals, rule.Body)
		}

		// Once imports have been resolved, they are no longer needed.
		mod.Imports = nil
	}

	if c.moduleLoader != nil {

		parsed, err := c.moduleLoader(c.Modules)
		if err != nil {
			c.err(NewError(CompileErr, nil, err.Error()))
			return
		}

		if len(parsed) == 0 {
			return
		}

		for id, module := range parsed {
			c.Modules[id] = module
		}

		c.resolveAllRefs()
	}
}

// rewriteRefsInHead will rewrite rules so that the head does not contain any
// references. If the key or value contains one or more references, that term
// will be moved into the body and assigned to a new variable. The new variable
// will replace the term in the head.
//
// For instance, given the following rule:
//
// p[{"foo": data.foo[i]}] :- i < 100
//
// The rule would be re-written as:
//
// p[__local0__] :- __local0__ = {"foo": data.foo[i]}, i < 100
func (c *Compiler) rewriteRefsInHead() {
	for _, mod := range c.Modules {
		generator := newLocalVarGenerator(mod)
		for _, rule := range mod.Rules {
			if rule.Key != nil {
				found := false
				WalkRefs(rule.Key, func(Ref) bool {
					found = true
					return true
				})
				if found {
					// Replace rule key with generated var
					key := rule.Key
					local := generator.Generate()
					term := &Term{Value: local}
					rule.Key = term
					expr := Equality.Expr(term, key)
					rule.Body = append(rule.Body, expr)
				}
			}
			if rule.Value != nil {
				found := false
				WalkRefs(rule.Value, func(Ref) bool {
					found = true
					return true
				})
				if found {
					// Replace rule value with generated var
					value := rule.Value
					local := generator.Generate()
					term := &Term{Value: local}
					rule.Value = term
					expr := Equality.Expr(term, value)
					rule.Body = append(rule.Body, expr)
				}
			}
		}
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
			edges, ok := c.RuleGraph[r]
			if !ok {
				edges = map[*Rule]struct{}{}
				c.RuleGraph[r] = edges
			}
			vis := &ruleGraphBuilder{
				moduleTree: c.ModuleTree,
				edges:      edges,
			}
			Walk(vis, r)
		}
	}
}

type queryCompiler struct {
	compiler *Compiler
	qctx     *QueryContext
}

func newQueryCompiler(compiler *Compiler) QueryCompiler {
	qc := &queryCompiler{
		compiler: compiler,
		qctx:     nil,
	}
	return qc
}

func (qc *queryCompiler) WithContext(qctx *QueryContext) QueryCompiler {
	qc.qctx = qctx
	return qc
}

func (qc *queryCompiler) Compile(query Body) (Body, error) {

	stages := []func(*QueryContext, Body) (Body, error){
		qc.resolveRefs,
		qc.checkSafety,
		qc.checkBuiltins,
	}

	qctx := qc.qctx.Copy()

	for _, s := range stages {
		var err error
		if query, err = s(qctx, query); err != nil {
			return nil, err
		}
	}

	return query, nil
}

func (qc *queryCompiler) resolveRefs(qctx *QueryContext, body Body) (Body, error) {

	var globals map[Var]Value

	if qctx != nil {
		var exports []Var
		if exist, ok := qc.compiler.getExports().Get(qctx.Package.Path); ok {
			exports = exist.([]Var)
		}
		globals = getGlobals(qctx.Package, exports, qc.qctx.Imports)
		qctx.Imports = nil
	}

	return resolveRefsInBody(globals, body), nil
}

func (qc *queryCompiler) checkSafety(qctx *QueryContext, body Body) (Body, error) {

	safe := ReservedVars.Copy()
	if qctx != nil {
		for i := range qctx.Imports {
			safe.Add(qctx.Imports[i].Path.Value.(Var))
		}
	}

	reordered, unsafe := reorderBodyForSafety(safe, body)

	if len(unsafe) != 0 {
		var err Errors
		for v := range unsafe.Vars() {
			err = append(err, NewError(UnsafeVarErr, body.Loc(), "%v is unsafe (variable %v must appear in the output position of at least one non-negated expression)", v, v))
		}
		return nil, err
	}

	return reordered, nil
}

func (qc *queryCompiler) checkBuiltins(qctx *QueryContext, body Body) (Body, error) {
	bc := newBuiltinChecker()
	if errs := bc.Check(body); len(errs) != 0 {
		return nil, errs
	}
	return body, nil
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

// DepthFirst performs a depth-first traversal of the module tree rooted at n.
// If f returns true, traversal will not continue to the children of n.
func (n *ModuleTreeNode) DepthFirst(f func(node *ModuleTreeNode) bool) {
	if !f(n) {
		for _, node := range n.Children {
			node.DepthFirst(f)
		}
	}
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

// DepthFirst performs a depth-first traversal of the rule tree rooted at n. If
// f returns true, traversal will not continue to the children of n.
func (n *RuleTreeNode) DepthFirst(f func(node *RuleTreeNode) bool) {
	if !f(n) {
		for _, node := range n.Children {
			node.DepthFirst(f)
		}
	}
}

// builtinChecker verifies that built-in functions are called correctly.
type builtinChecker struct {
	errors *Errors
	prefix string
}

func newBuiltinChecker() *builtinChecker {
	return &builtinChecker{
		errors: &Errors{},
	}
}

// Check returns built-in function call errors underneath the AST node x.
func (bc *builtinChecker) Check(x interface{}) Errors {
	Walk(bc, x)
	return *(bc.errors)
}

func (bc *builtinChecker) Visit(x interface{}) Visitor {
	switch x := x.(type) {
	case *Rule:
		cpy := *bc
		cpy.prefix = string(x.Name)
		return &cpy
	case *Expr:
		if ts, ok := x.Terms.([]*Term); ok {
			if bi, ok := BuiltinMap[ts[0].Value.(Var)]; ok {
				if bi.NumArgs != len(ts[1:]) {
					msg := "wrong number of arguments (expression %s must specify %d arguments to built-in function %v)"
					bc.err(CompileErr, x.Location, msg, x.Location.Text, bi.NumArgs, ts[0])
				}
			} else {
				msg := "unknown built-in function %v"
				bc.err(CompileErr, x.Location, msg, ts[0])
			}
		}
	}
	return bc
}

func (bc *builtinChecker) err(code ErrCode, loc *Location, f string, a ...interface{}) {
	if bc.prefix != "" {
		f = bc.prefix + ": " + f
	}
	*(bc.errors) = append(*(bc.errors), NewError(code, loc, f, a...))
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

// findRules returns a slice of rules referred to by ref.
//
// For example, given package a.b.c containing rules p and q:
//
// findRules(a.b.c.p)		=> [p]
// findRules(a.b.c.r)		=> []
// findRules(a.b.c.p[x])	=> [p]
// findRules(a.b.c)			=> [p q]
// findRules(a.b)			=> [p q] # assumes no other rules under a.b
func findRules(node *ModuleTreeNode, ref Ref) []*Rule {

	// Syntactically, reference heads are variables, however, we don't want to
	// treat them the same way as variables in the remainder of the reference
	// here.
	if node, ok := node.Children[ref[0].Value]; ok {
		return findRulesRec(node, ref[1:])
	}

	return nil
}

func findRulesRec(node *ModuleTreeNode, ref Ref) (rs []*Rule) {

	if len(ref) == 0 {
		// Any rules that define documents embedded inside the document referred
		// to by this reference must be included. Accumulate all embedded rules
		// by recursively walking the module tree.
		var acc func(node *ModuleTreeNode)

		acc = func(node *ModuleTreeNode) {
			for _, mod := range node.Modules {
				rs = append(rs, mod.Rules...)
			}
			for _, child := range node.Children {
				acc(child)
			}
		}

		acc(node)

		return rs
	}

	head, tail := ref[0], ref[1:]

	switch head := head.Value.(type) {
	case String:
		if node, ok := node.Children[head]; ok {
			return findRulesRec(node, tail)
		}
		for _, m := range node.Modules {
			for _, r := range m.Rules {
				if String(r.Name).Equal(head) {
					rs = append(rs, r)
				}
			}
		}
	case Var:
		for _, n := range node.Children {
			rs = append(rs, findRulesRec(n, tail)...)
		}
		for _, m := range node.Modules {
			rs = append(rs, m.Rules...)
		}
	}

	return rs
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
		for v := range e.Vars(VarVisitorParams{SkipClosures: true}) {
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
			g.Update(reordered[i-1].Vars(VarVisitorParams{SkipClosures: true}))
		}
		vis := &bodySafetyVisitor{
			current: e,
			globals: g,
			unsafe:  unsafe,
		}
		Walk(vis, e)
	}

	// Need to reset expression indices as re-ordering may have
	// changed them.
	setExprIndices(reordered)

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
	// Check term for safety. This is analogous to the rule head safety check.
	tv := ac.Term.Vars()
	bv := ac.Body.Vars(VarVisitorParams{SkipClosures: true})
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
				vis := &VarVisitor{vars: vs}
				Walk(vis, x)
				return true
			})

			// Compute vars that are closed over from the body but not yet
			// contained in the output position of an expression in the reordered
			// body. These vars are considered unsafe.
			cv := vs.Intersect(body.Vars(VarVisitorParams{SkipClosures: true})).Diff(globals)
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

const localVarFmt = "__local%d__"

type localVarGenerator struct {
	exclude VarSet
}

func newLocalVarGenerator(module *Module) *localVarGenerator {
	exclude := NewVarSet()
	vis := &VarVisitor{
		vars: exclude,
	}
	Walk(vis, module)
	return &localVarGenerator{exclude}
}

func (l *localVarGenerator) Generate() Var {
	name := Var("")
	x := 0
	for len(name) == 0 || l.exclude.Contains(name) {
		name = Var(fmt.Sprintf(localVarFmt, x))
		x++
	}
	l.exclude.Add(name)
	return name
}

func getGlobals(pkg *Package, exports []Var, imports []*Import) map[Var]Value {

	globals := map[Var]Value{}

	// Populate globals with exports within the package.
	for _, v := range exports {
		global := append(Ref{}, pkg.Path...)
		global = append(global, &Term{Value: String(v)})
		globals[v] = global
	}

	// Populate globals with imports.
	for _, i := range imports {
		if len(i.Alias) > 0 {
			path := i.Path.Value.(Ref)
			globals[i.Alias] = path
		} else {
			path := i.Path.Value.(Ref)
			if len(path) == 1 {
				globals[path[0].Value.(Var)] = path
			} else {
				v := path[len(path)-1].Value.(String)
				globals[Var(v)] = path
			}
		}
	}

	return globals
}

func resolveRef(globals map[Var]Value, ref Ref) Ref {

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
			r = append(r, resolveRefsInTerm(globals, x))
		default:
			r = append(r, x)
		}
	}

	return r
}

func resolveRefsInBody(globals map[Var]Value, body Body) Body {
	r := Body{}
	for _, expr := range body {
		r = append(r, resolveRefsInExpr(globals, expr))
	}
	return r
}

func resolveRefsInExpr(globals map[Var]Value, expr *Expr) *Expr {
	cpy := *expr
	switch ts := expr.Terms.(type) {
	case *Term:
		cpy.Terms = resolveRefsInTerm(globals, ts)
	case []*Term:
		buf := []*Term{}
		for _, t := range ts {
			buf = append(buf, resolveRefsInTerm(globals, t))
		}
		cpy.Terms = buf
	}
	return &cpy
}

func resolveRefsInTerm(globals map[Var]Value, term *Term) *Term {
	switch v := term.Value.(type) {
	case Var:
		if r, ok := globals[v]; ok {
			cpy := *term
			cpy.Value = r
			return &cpy
		}
		return term
	case Ref:
		fqn := resolveRef(globals, v)
		cpy := *term
		cpy.Value = fqn
		return &cpy
	case Object:
		o := Object{}
		for _, i := range v {
			k := resolveRefsInTerm(globals, i[0])
			v := resolveRefsInTerm(globals, i[1])
			o = append(o, Item(k, v))
		}
		cpy := *term
		cpy.Value = o
		return &cpy
	case Array:
		a := Array{}
		for _, e := range v {
			x := resolveRefsInTerm(globals, e)
			a = append(a, x)
		}
		cpy := *term
		cpy.Value = a
		return &cpy
	case *Set:
		s := &Set{}
		for _, e := range *v {
			x := resolveRefsInTerm(globals, e)
			s.Add(x)
		}
		cpy := *term
		cpy.Value = s
		return &cpy
	case *ArrayComprehension:
		ac := &ArrayComprehension{}
		ac.Term = resolveRefsInTerm(globals, v.Term)
		ac.Body = resolveRefsInBody(globals, v.Body)
		cpy := *term
		cpy.Value = ac
		return &cpy
	default:
		return term
	}
}
