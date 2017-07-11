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

	// ModuleTree organizes the modules into a tree where each node is keyed by
	// an element in the module's package path. E.g., given modules containing
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
	// containing package and the stringified rule name. E.g., given the
	// following module:
	//
	//  package ex
	//  p[1] { true }
	//  p[2] { true }
	//  q = true
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
	RuleTree *TreeNode

	// FuncTree organizes user functions into a tree where each node is keyed
	// by an element in the logical path to the function. The logical path is
	// the concatenations of the containing package and the stringified function
	// name. Functions are only located at the lead nodes, each of which have
	// exactly 1 function. E.g., given the following module:
	//
	//  package a.b
	//  p(x) = y { y = x }
	//  q(x) = y { y = 2*x }
	//
	//  root
	//    |
	//    +--- a (no functions)
	//         |
	//         +--- b (no functions)
	//              |
	//              +--- p
	//              |
	//              +--- q
	FuncTree *TreeNode

	// FunctionMap is a map containing the user defined functions of this
	// compiler's modules.
	FuncMap map[String][]*Func

	// Graph represents the dependencies between rules and funcs (lets call
	// them targets). An edge (u,v) is added to the graph if target "u"
	// depends on target "v". A target "u" depends on target "v" if target
	// "u" refers to the virtual document (or function) defined by target "v".
	Graph *Graph

	// TypeEnv holds type information for values inferred by the compiler.
	TypeEnv *TypeEnv

	generatedVars map[*Module]VarSet
	moduleLoader  ModuleLoader
	ruleIndices   *util.HashMap
	stages        []func()
}

// QueryContext contains contextual information for running an ad-hoc query.
//
// Ad-hoc queries can be run in the context of a package and imports may be
// included to provide concise access to data.
type QueryContext struct {
	Package *Package
	Imports []*Import
	Input   Value
}

// NewQueryContext returns a new QueryContext object.
func NewQueryContext() *QueryContext {
	return &QueryContext{}
}

// InputDefined returns true if the input document is defined in qc.
func (qc *QueryContext) InputDefined() bool {
	return qc != nil && qc.Input != nil
}

// WithPackage sets the pkg on qc.
func (qc *QueryContext) WithPackage(pkg *Package) *QueryContext {
	if qc == nil {
		qc = NewQueryContext()
	}
	qc.Package = pkg
	return qc
}

// WithImports sets the imports on qc.
func (qc *QueryContext) WithImports(imports []*Import) *QueryContext {
	if qc == nil {
		qc = NewQueryContext()
	}
	qc.Imports = imports
	return qc
}

// WithInput sets the input on qc.
func (qc *QueryContext) WithInput(input Value) *QueryContext {
	if qc == nil {
		qc = NewQueryContext()
	}
	qc.Input = input
	return qc
}

// Copy returns a deep copy of qc.
func (qc *QueryContext) Copy() *QueryContext {
	if qc == nil {
		return nil
	}
	cpy := *qc
	if cpy.Package != nil {
		cpy.Package = qc.Package.Copy()
	}
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

	// TypeEnv returns the type environment built after running type checking
	// on the query.
	TypeEnv() *TypeEnv

	// WithContext sets the QueryContext on the QueryCompiler. Subsequent calls
	// to Compile will take the QueryContext into account.
	WithContext(qctx *QueryContext) QueryCompiler
}

// NewCompiler returns a new empty compiler.
func NewCompiler() *Compiler {

	c := &Compiler{
		Modules:       map[string]*Module{},
		TypeEnv:       NewTypeEnv(),
		FuncMap:       map[String][]*Func{},
		generatedVars: map[*Module]VarSet{},
		ruleIndices: util.NewHashMap(func(a, b util.T) bool {
			r1, r2 := a.(Ref), b.(Ref)
			return r1.Equal(r2)
		}, func(x util.T) int {
			return x.(Ref).Hash()
		}),
	}

	c.ModuleTree = NewModuleTree(nil)
	c.RuleTree = NewRuleTree(c.ModuleTree)
	c.FuncTree = NewFuncTree(c.ModuleTree)

	checker := newTypeChecker()
	c.TypeEnv = checker.checkLanguageBuiltins()

	c.stages = []func(){
		c.resolveAllRefs,
		c.setModuleTree,
		c.setRuleTree,
		c.setFuncTree,
		c.setGraph,
		c.rewriteRefsInHead,
		c.checkWithModifiers,
		c.checkRuleConflicts,
		c.checkSafetyFuncHeads,
		c.checkSafetyFuncBodies,
		c.checkSafetyRuleHeads,
		c.checkSafetyRuleBodies,
		c.checkRecursion,
		c.checkTypes,
		c.buildRuleIndices,
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
//	p[k] = v { ... }    # rule1
//  p[k1] = v1 { ... }  # rule2
//
// The following calls yield the rules on the right.
//
//  GetRulesExact("data.a.b.c.p")   => [rule1, rule2]
//  GetRulesExact("data.a.b.c.p.x") => nil
//  GetRulesExact("data.a.b.c")     => nil
func (c *Compiler) GetRulesExact(ref Ref) (rules []*Rule) {
	node := c.RuleTree

	for _, x := range ref {
		if node = node.Child(x.Value); node == nil {
			return nil
		}
	}

	return extractRules(node.Values)
}

// GetRulesForVirtualDocument returns a slice of rules that produce the virtual
// document referred to by the reference.
//
// E.g., given the following module:
//
//	package a.b.c
//
//	p[k] = v { ... }    # rule1
//  p[k1] = v1 { ... }  # rule2
//
// The following calls yield the rules on the right.
//
//  GetRulesForVirtualDocument("data.a.b.c.p")   => [rule1, rule2]
//  GetRulesForVirtualDocument("data.a.b.c.p.x") => [rule1, rule2]
//  GetRulesForVirtualDocument("data.a.b.c")     => nil
func (c *Compiler) GetRulesForVirtualDocument(ref Ref) (rules []*Rule) {

	node := c.RuleTree

	for _, x := range ref {
		if node = node.Child(x.Value); node == nil {
			return nil
		}
		if len(node.Values) > 0 {
			return extractRules(node.Values)
		}
	}

	return extractRules(node.Values)
}

// GetRulesWithPrefix returns a slice of rules that share the prefix ref.
//
// E.g., given the following module:
//
//  package a.b.c
//
//  p[x] = y { ... }  # rule1
//  p[k] = v { ... }  # rule2
//  q { ... }         # rule3
//
// The following calls yield the rules on the right.
//
//  GetRulesWithPrefix("data.a.b.c.p")   => [rule1, rule2]
//  GetRulesWithPrefix("data.a.b.c.p.a") => nil
//  GetRulesWithPrefix("data.a.b.c")     => [rule1, rule2, rule3]
func (c *Compiler) GetRulesWithPrefix(ref Ref) (rules []*Rule) {

	node := c.RuleTree

	for _, x := range ref {
		if node = node.Child(x.Value); node == nil {
			return nil
		}
	}

	var acc func(node *TreeNode)

	acc = func(node *TreeNode) {
		rules = append(rules, extractRules(node.Values)...)
		for _, child := range node.Children {
			if child.Hide {
				continue
			}
			acc(child)
		}
	}

	acc(node)

	return rules
}

func extractRules(s []util.T) (rules []*Rule) {
	for _, r := range s {
		rules = append(rules, r.(*Rule))
	}
	return rules
}

// GetRules returns a slice of rules that are referred to by ref.
//
// E.g., given the following module:
//
//  package a.b.c
//
//  p[x] = y { q[x] = y; ... } # rule1
//  q[x] = y { ... }           # rule2
//
// The following calls yield the rules on the right.
//
//  GetRules("data.a.b.c.p")	=> [rule1]
//  GetRules("data.a.b.c.p.x")	=> [rule1]
//  GetRules("data.a.b.c.q")	=> [rule2]
//  GetRules("data.a.b.c")		=> [rule1, rule2]
//  GetRules("data.a.b.d")		=> nil
func (c *Compiler) GetRules(ref Ref) (rules []*Rule) {

	set := map[*Rule]struct{}{}

	for _, rule := range c.GetRulesForVirtualDocument(ref) {
		set[rule] = struct{}{}
	}

	for _, rule := range c.GetRulesWithPrefix(ref) {
		set[rule] = struct{}{}
	}

	for rule := range set {
		rules = append(rules, rule)
	}

	return rules
}

// GetFunc returns the function referred to by name.
func (c *Compiler) GetFunc(name String) []*Func {
	if fn, ok := c.FuncMap[name]; ok {
		return fn
	}
	return nil
}

// GetAllFuncs returns a map of functions that this compiler has discovered.
func (c *Compiler) GetAllFuncs() map[String][]*Func {
	cpy := map[String][]*Func{}
	for _, fn := range c.FuncMap {
		var fns []*Func
		for _, f := range fn {
			fns = append(fns, f.Copy())
		}
		cpy[fn[0].PathString()] = fns
	}
	return cpy
}

// RuleIndex returns a RuleIndex built for the rule set referred to by path.
// The path must refer to the rule set exactly, i.e., given a rule set at path
// data.a.b.c.p, refs data.a.b.c.p.x and data.a.b.c would not return a
// RuleIndex built for the rule.
func (c *Compiler) RuleIndex(path Ref) RuleIndex {
	r, ok := c.ruleIndices.Get(path)
	if !ok {
		return nil
	}
	return r.(RuleIndex)
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

// buildRuleIndices constructs indices for rules.
func (c *Compiler) buildRuleIndices() {

	c.RuleTree.DepthFirst(func(node *TreeNode) bool {
		if len(node.Values) == 0 {
			return false
		}
		index := newBaseDocEqIndex(func(ref Ref) bool {
			return len(c.GetRules(ref.GroundPrefix())) > 0
		})
		if rules := extractRules(node.Values); index.Build(rules) {
			c.ruleIndices.Put(rules[0].Path(), index)
		}
		return false
	})

}

// checkRecursion ensures that there are no recursive definitions, i.e., there are
// no cycles in the Graph.
func (c *Compiler) checkRecursion() {
	eq := func(a, b util.T) bool {
		ar, aok := a.(*Rule)
		br, bok := b.(*Rule)
		if aok && bok {
			return ar == br
		}

		af, aok := a.(*Func)
		bf, bok := b.(*Func)
		return aok && bok && af == bf
	}

	c.RuleTree.DepthFirst(func(node *TreeNode) bool {
		for _, rule := range node.Values {
			r := rule.(*Rule)
			c.checkSelfPath(RuleTypeName, r.Loc(), eq, r, r)
		}
		return false
	})

	c.FuncTree.DepthFirst(func(node *TreeNode) bool {
		for _, fn := range node.Values {
			f := fn.(*Func)
			c.checkSelfPath(FuncTypeName, f.Loc(), eq, f, f)
		}
		return false
	})
}

func (c *Compiler) checkSelfPath(t string, loc *Location, eq func(a, b util.T) bool, a, b util.T) {
	tr := newgraphTraversal(c.Graph)
	if p := util.DFSPath(tr, eq, a, b); len(p) > 0 {
		n := []string{}
		for _, x := range p {
			n = append(n, astNodeToString(x))
		}
		c.err(NewError(RecursionErr, loc, "%v %v is recursive: %v", t, astNodeToString(a), strings.Join(n, " -> ")))
	}
}

func astNodeToString(x interface{}) string {
	switch x := x.(type) {
	case *Rule:
		return string(x.Head.Name)
	case *Func:
		return string(x.Head.Name)
	default:
		panic("not reached")
	}
}

// checkRuleConflicts ensures that rules definitions are not in conflict.
func (c *Compiler) checkRuleConflicts() {
	c.RuleTree.DepthFirst(func(node *TreeNode) bool {
		if len(node.Values) == 0 {
			return false
		}

		kinds := map[DocKind]struct{}{}
		defaultRules := 0

		for _, rule := range node.Values {
			r := rule.(*Rule)
			kinds[r.Head.DocKind()] = struct{}{}
			if r.Default {
				defaultRules++
			}
		}

		name := Var(node.Key.(String))

		if len(kinds) > 1 {
			c.err(NewError(TypeErr, node.Values[0].(*Rule).Loc(), "conflicting rules named %v found", name))
		}

		if defaultRules > 1 {
			c.err(NewError(TypeErr, node.Values[0].(*Rule).Loc(), "multiple default rules named %s found", name))
		}

		return false
	})

	c.ModuleTree.DepthFirst(func(node *ModuleTreeNode) bool {
		for _, mod := range node.Modules {
			for _, rule := range mod.Rules {
				for _, fn := range mod.Funcs {
					if rule.Head.Name.Equal(fn.Head.Name) {
						msg := fmt.Sprintf("rule defined at %v conflicts with function defined at %v", rule.Loc(), fn.Loc())
						c.err(NewError(CompileErr, mod.Package.Loc(), msg))
					}
				}

				if childNode, ok := node.Children[String(rule.Head.Name)]; ok {
					for _, childMod := range childNode.Modules {
						msg := fmt.Sprintf("%v conflicts with rule defined at %v", childMod.Package, rule.Loc())
						c.err(NewError(TypeErr, mod.Package.Loc(), msg))
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
		WalkRules(m, func(r *Rule) bool {
			r.Body = c.checkBodySafety(safe, m, r.Body, r.Loc())
			return false
		})
	}
}

func (c *Compiler) checkSafetyFuncBodies() {
	for _, m := range c.Modules {
		safe := ReservedVars.Copy()
		WalkFuncs(m, func(f *Func) bool {
			s := safe.Copy()
			s.Update(f.Head.ArgVars())
			f.Body = c.checkBodySafety(s, m, f.Body, f.Loc())
			return false
		})
	}
}

func (c *Compiler) checkBodySafety(safe VarSet, m *Module, b Body, l *Location) Body {
	reordered, unsafe := reorderBodyForSafety(safe, b)
	if len(unsafe) != 0 {
		for v := range unsafe.Vars() {
			if !c.generatedVars[m].Contains(v) {
				c.err(NewError(UnsafeVarErr, l, "%v %v is unsafe", VarTypeName, v))
			}
		}
		return b
	}
	return reordered
}

var safetyCheckVarVisitorParams = VarVisitorParams{
	SkipClosures: true,
}

// checkSafetyRuleHeads ensures that variables appearing in the head of a
// rule also appear in the body.
func (c *Compiler) checkSafetyRuleHeads() {
	for _, m := range c.Modules {
		WalkRules(m, func(r *Rule) bool {
			unsafe := r.Head.Vars().Diff(r.Body.Vars(safetyCheckVarVisitorParams))
			for v := range unsafe {
				if !c.generatedVars[m].Contains(v) {
					c.err(NewError(UnsafeVarErr, r.Loc(), "%v %v is unsafe", VarTypeName, v))
				}
			}
			return false
		})
	}
}

func (c *Compiler) checkSafetyFuncHeads() {
	for _, m := range c.Modules {
		WalkFuncs(m, func(f *Func) bool {
			vars := f.Body.Vars(safetyCheckVarVisitorParams)
			vars.Update(f.Head.ArgVars())
			unsafe := f.Head.OutVars().Diff(vars)
			for v := range unsafe {
				if !c.generatedVars[m].Contains(v) {
					c.err(NewError(UnsafeVarErr, f.Loc(), "%v %v is unsafe", VarTypeName, v))
				}
			}
			return false
		})
	}
}

// checkTypes runs the type checker on all rules and user functions. The type
// checker builds a TypeEnv that is stored on the compiler.
func (c *Compiler) checkTypes() {
	// Recursion is caught in earlier step, so this cannot fail.
	sorted, _ := c.Graph.Sort()
	checker := newTypeChecker()
	env, errs := checker.CheckTypes(c.TypeEnv, sorted)
	for _, err := range errs {
		c.err(err)
	}
	c.TypeEnv = env
}

// checkWithModifiers ensures that with modifier values do not contain
// references or closures.
func (c *Compiler) checkWithModifiers() {
	for _, m := range c.Modules {
		wc := newWithModifierChecker()
		for _, err := range wc.Check(m) {
			c.err(err)
		}
	}
}

func (c *Compiler) compile() {
	for _, fn := range c.stages {
		if fn(); c.Failed() {
			return
		}
	}
}

func (c *Compiler) err(err *Error) {
	c.Errors = append(c.Errors, err)
}

func (c *Compiler) getExports() (*util.HashMap, *util.HashMap) {

	rules := util.NewHashMap(func(a, b util.T) bool {
		r1 := a.(Ref)
		r2 := a.(Ref)
		return r1.Equal(r2)
	}, func(v util.T) int {
		return v.(Ref).Hash()
	})
	funcs := rules.Copy()

	for _, mod := range c.Modules {
		rv, ok := rules.Get(mod.Package.Path)
		if !ok {
			rv = []Var{}
		}
		rvs := rv.([]Var)

		fv, ok := funcs.Get(mod.Package.Path)
		if !ok {
			fv = []*Func{}
		}
		fvs := fv.([]*Func)

		for _, rule := range mod.Rules {
			rvs = append(rvs, rule.Head.Name)
		}
		for _, fn := range mod.Funcs {
			fvs = append(fvs, fn)
		}
		rules.Put(mod.Package.Path, rvs)
		funcs.Put(mod.Package.Path, fvs)
	}

	return rules, funcs
}

// resolveAllRefs resolves references in expressions to their fully qualified values.
//
// For instance, given the following module:
//
// package a.b
// import data.foo.bar
// p[x] { bar[_] = x }
//
// The reference "bar[_]" would be resolved to "data.foo.bar[_]".
func (c *Compiler) resolveAllRefs() {

	rules, funcs := c.getExports()

	for _, mod := range c.Modules {

		var ruleExports []Var
		if x, ok := rules.Get(mod.Package.Path); ok {
			ruleExports = x.([]Var)
		}

		var funcExports []*Func
		if x, ok := funcs.Get(mod.Package.Path); ok {
			funcExports = x.([]*Func)
		}

		globals := getGlobals(mod.Package, ruleExports, funcExports, mod.Imports)
		WalkRules(mod, func(rule *Rule) bool {
			resolveRefsInRule(globals, rule)
			return false
		})
		WalkFuncs(mod, func(fn *Func) bool {
			resolveRefsInFunc(globals, fn)
			path := fn.PathString()
			c.FuncMap[path] = append(c.FuncMap[path], fn)

			return false
		})

		// Once imports have been resolved, they are no longer needed.
		mod.Imports = nil
	}

	for _, mod := range c.Modules {
		visitor := NewGenericVisitor(func(x interface{}) bool {
			// Walk terms in order to provide more detailed location
			// information.
			switch x := x.(type) {
			case *Term:
				switch v := x.Value.(type) {
				case Ref:
					if _, ok := c.FuncMap[String(v.String())]; ok {
						c.err(&Error{
							Code:    CompileErr,
							Message: x.Location.Format("%v refers to a known builtin but does not call it", string(x.Location.Text)),
						})
					}
				}
			}
			return false
		})

		Walk(visitor, mod)
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

// rewriteTermsInHead will rewrite rules so that the head does not contain any
// terms that require evaluation (e.g., refs or comprehensions). If the key or
// value contains or more of these terms, the key or value will be moved into
// the body and assigned to a new variable. The new variable will replace the
// key or value in the head.
//
// For instance, given the following rule:
//
// p[{"foo": data.foo[i]}] { i < 100 }
//
// The rule would be re-written as:
//
// p[__local0__] { i < 100; __local0__ = {"foo": data.foo[i]} }
func (c *Compiler) rewriteRefsInHead() {
	for _, mod := range c.Modules {
		generator := newLocalVarGenerator(mod)
		WalkRules(mod, func(rule *Rule) bool {
			if rule.Head.Key != nil {
				found := false
				vis := NewGenericVisitor(func(x interface{}) bool {
					if found {
						return true
					}
					switch x.(type) {
					case Ref, *ArrayComprehension, *ObjectComprehension, *SetComprehension:
						found = true
						return true
					}
					return false
				})
				Walk(vis, rule.Head.Key)
				if found {
					// Replace rule key with generated var
					key := rule.Head.Key
					local := generator.Generate()
					term := &Term{Value: local}
					rule.Head.Key = term
					expr := Equality.Expr(term, key)
					expr.Location = rule.Loc()
					rule.Body.Append(expr)
				}
			}
			if rule.Head.Value != nil {
				found := false
				vis := NewGenericVisitor(func(x interface{}) bool {
					if found {
						return true
					}
					switch x.(type) {
					case Ref, *ArrayComprehension, *ObjectComprehension, *SetComprehension:
						found = true
						return true
					}
					return false
				})
				Walk(vis, rule.Head.Value)
				if found {
					// Replace rule value with generated var
					value := rule.Head.Value
					local := generator.Generate()
					term := &Term{Value: local}
					rule.Head.Value = term
					expr := Equality.Expr(term, value)
					expr.Location = rule.Loc()
					rule.Body.Append(expr)
				}
			}
			return false
		})
		c.generatedVars[mod] = generator.Generated()
	}
}

func (c *Compiler) setModuleTree() {
	c.ModuleTree = NewModuleTree(c.Modules)
}

func (c *Compiler) setRuleTree() {
	c.RuleTree = NewRuleTree(c.ModuleTree)
}

func (c *Compiler) setFuncTree() {
	c.FuncTree = NewFuncTree(c.ModuleTree)
}

func (c *Compiler) setGraph() {
	c.Graph = NewGraph(c.Modules, c.GetRules, c.GetFunc)
}

type queryCompiler struct {
	compiler *Compiler
	qctx     *QueryContext
	typeEnv  *TypeEnv
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
		qc.checkWithModifiers,
		qc.checkSafety,
		qc.checkTypes,
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

func (qc *queryCompiler) TypeEnv() *TypeEnv {
	return qc.typeEnv
}

func (qc *queryCompiler) resolveRefs(qctx *QueryContext, body Body) (Body, error) {

	var globals map[Var]Ref

	if qctx != nil && qctx.Package != nil {
		var ruleExports []Var
		rules, funcs := qc.compiler.getExports()
		if exist, ok := rules.Get(qctx.Package.Path); ok {
			ruleExports = exist.([]Var)
		}

		var funcExports []*Func
		if exist, ok := funcs.Get(qctx.Package.Path); ok {
			funcExports = exist.([]*Func)
		}

		globals = getGlobals(qctx.Package, ruleExports, funcExports, qc.qctx.Imports)
		qctx.Imports = nil
	}

	return resolveRefsInBody(globals, body), nil
}

func (qc *queryCompiler) checkSafety(_ *QueryContext, body Body) (Body, error) {

	safe := ReservedVars.Copy()
	reordered, unsafe := reorderBodyForSafety(safe, body)

	if len(unsafe) != 0 {
		var err Errors
		for v := range unsafe.Vars() {
			err = append(err, NewError(UnsafeVarErr, body.Loc(), "%v %v is unsafe", VarTypeName, v))
		}
		return nil, err
	}

	return reordered, nil
}

// referencesInput returns true if expr refers to the input document. This
// function will not visit closures.
func referencesInput(expr *Expr) bool {

	found := false

	vis := NewGenericVisitor(func(x interface{}) bool {
		if found {
			return found
		}
		switch x := x.(type) {
		case *ArrayComprehension, *ObjectComprehension, *SetComprehension: // skip closures
			return true
		case Ref:
			if x.HasPrefix(InputRootRef) {
				found = true
				return found
			}
		}
		return false
	})

	Walk(vis, expr)

	return found
}

// definesInput returns true if expr defines the input document using the with
// modifier.
func definesInput(expr *Expr) bool {
	for _, w := range expr.With {
		if ref, ok := w.Target.Value.(Ref); ok {
			if ref.HasPrefix(InputRootRef) {
				return true
			}
		}
	}
	return false
}

func (qc *queryCompiler) checkTypes(qctx *QueryContext, body Body) (Body, error) {
	var errs Errors
	checker := newTypeChecker()
	qc.typeEnv, errs = checker.CheckBody(qc.compiler.TypeEnv, body)
	if len(errs) > 0 {
		return nil, errs
	}
	return body, nil
}

func (qc *queryCompiler) checkWithModifiers(qctx *QueryContext, body Body) (Body, error) {
	wc := newWithModifierChecker()
	if errs := wc.Check(body); len(errs) != 0 {
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
	Hide     bool
}

// NewModuleTree returns a new ModuleTreeNode that represents the root
// of the module tree populated with the given modules.
func NewModuleTree(mods map[string]*Module) *ModuleTreeNode {
	root := &ModuleTreeNode{
		Children: map[Value]*ModuleTreeNode{},
	}
	for _, m := range mods {
		node := root
		for i, x := range m.Package.Path {
			c, ok := node.Children[x.Value]
			if !ok {
				var hide bool
				if i == 1 && x.Value.Compare(SystemDocumentKey) == 0 {
					hide = true
				}
				c = &ModuleTreeNode{
					Key:      x.Value,
					Children: map[Value]*ModuleTreeNode{},
					Hide:     hide,
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

// TreeNode represents a node in the rule tree. The rule tree is keyed by
// rule path.
type TreeNode struct {
	Key      Value
	Values   []util.T
	Children map[Value]*TreeNode
	Hide     bool
}

// NewRuleTree returns a new TreeNode that represents the root
// of the rule tree populated with the given rules.
func NewRuleTree(mtree *ModuleTreeNode) *TreeNode {

	ruleSets := map[String][]util.T{}

	// Build rule sets for this package.
	for _, mod := range mtree.Modules {
		for _, rule := range mod.Rules {
			key := String(rule.Head.Name)
			ruleSets[key] = append(ruleSets[key], rule)
		}
	}

	// Each rule set becomes a leaf node.
	children := map[Value]*TreeNode{}

	for key, rules := range ruleSets {
		children[key] = &TreeNode{
			Key:      key,
			Children: nil,
			Values:   rules,
		}
	}

	// Each module in subpackage becomes child node.
	for _, child := range mtree.Children {
		children[child.Key] = NewRuleTree(child)
	}

	return &TreeNode{
		Key:      mtree.Key,
		Values:   nil,
		Children: children,
		Hide:     mtree.Hide,
	}
}

// Size returns the number of rules in the tree.
func (n *TreeNode) Size() int {
	s := len(n.Values)
	for _, c := range n.Children {
		s += c.Size()
	}
	return s
}

// Child returns n's child with key k.
func (n *TreeNode) Child(k Value) *TreeNode {
	switch k.(type) {
	case String, Var:
		return n.Children[k]
	}
	return nil
}

// DepthFirst performs a depth-first traversal of the rule tree rooted at n. If
// f returns true, traversal will not continue to the children of n.
func (n *TreeNode) DepthFirst(f func(node *TreeNode) bool) {
	if !f(n) {
		for _, node := range n.Children {
			node.DepthFirst(f)
		}
	}
}

// NewFuncTree returns a new TreeNode that represents the root
// of the function tree populated with the given functions.
func NewFuncTree(mtree *ModuleTreeNode) *TreeNode {
	funcSet := map[String]*Func{}

	// Build function sets for this package.
	for _, mod := range mtree.Modules {
		for _, fn := range mod.Funcs {
			key := String(fn.Head.Name)
			funcSet[key] = fn
		}
	}

	// Each function becomes a leaf node.
	children := map[Value]*TreeNode{}
	for key, fn := range funcSet {
		children[key] = &TreeNode{
			Key:      key,
			Children: nil,
			Values:   []util.T{fn},
		}
	}

	// Each module in subpackage becomes child node.
	for _, child := range mtree.Children {
		children[child.Key] = NewFuncTree(child)
	}

	return &TreeNode{
		Key:      mtree.Key,
		Values:   nil,
		Children: children,
		Hide:     mtree.Hide,
	}
}

type withModifierChecker struct {
	errors Errors
	expr   *Expr
	prefix string
}

func newWithModifierChecker() *withModifierChecker {
	return &withModifierChecker{}
}

func (wc *withModifierChecker) Check(x interface{}) Errors {
	Walk(wc, x)
	return wc.errors
}

func (wc *withModifierChecker) Visit(x interface{}) Visitor {
	switch x := x.(type) {
	case *Rule:
		wc.prefix = string(x.Head.Name)
	case *Expr:
		wc.expr = x
	case *With:
		wc.checkTarget(x)
		wc.checkValue(x)
		return nil
	}
	return wc
}

func (wc *withModifierChecker) checkTarget(w *With) {

	if ref, ok := w.Target.Value.(Ref); ok {
		if !ref.HasPrefix(InputRootRef) {
			wc.err(TypeErr, w.Location, "with keyword target must be %v", InputRootDocument)
		}
	}

	// TODO(tsandall): could validate that target is in fact referred to by
	// evaluation of the expression.
}

func (wc *withModifierChecker) checkValue(w *With) {
	WalkClosures(w.Value, func(c interface{}) bool {
		wc.err(TypeErr, w.Location, "with keyword value must not contain closures")
		return true
	})

	if len(wc.errors) > 0 {
		return
	}

	WalkRefs(w.Value, func(r Ref) bool {
		wc.err(TypeErr, w.Location, "with keyword value must not contain %vs", RefTypeName)
		return true
	})
}

func (wc *withModifierChecker) err(code string, loc *Location, f string, a ...interface{}) {
	if wc.prefix != "" {
		f = wc.prefix + ": " + f
	}
	wc.errors = append(wc.errors, NewError(code, loc, f, a...))
}

// Graph represents the graph of dependencies between ast Rules and Funcs.
type Graph struct {
	adj    map[util.T]map[util.T]struct{}
	nodes  map[util.T]struct{}
	sorted []util.T
}

// NewGraph returns a new Graph based on modules. The list function
// must return the rules or user functions referred to directly by the ref.
func NewGraph(modules map[string]*Module, list func(Ref) []*Rule, resolve func(String) []*Func) *Graph {

	graph := &Graph{
		adj:    map[util.T]map[util.T]struct{}{},
		nodes:  map[util.T]struct{}{},
		sorted: nil,
	}

	// Walk over all rules and functions, add them to graph, and build adjencency lists.
	for _, module := range modules {
		addRefDeps := func(a util.T) func(ref Ref) bool {
			return func(ref Ref) bool {
				for _, b := range list(ref.GroundPrefix()) {
					graph.addDependency(a, b)
				}
				return false
			}
		}
		addFuncDeps := func(a util.T) func(expr *Expr) bool {
			return func(expr *Expr) bool {
				if expr.IsBuiltin() {
					name := expr.Terms.([]*Term)[0].Value.(String)

					// Language builtins won't be resolved.
					if b := resolve(name); b != nil {
						for _, c := range b {
							graph.addDependency(a, c)
						}
					}
				}
				return false
			}
		}

		WalkRules(module, func(a *Rule) bool {
			graph.addNode(a)
			WalkRefs(a, addRefDeps(a))
			WalkExprs(a, addFuncDeps(a))
			return false
		})
		WalkFuncs(module, func(a *Func) bool {
			graph.addNode(a)
			WalkRefs(a, addRefDeps(a))
			WalkExprs(a, addFuncDeps(a))
			return false
		})
	}

	return graph
}

// Dependencies returns the set of rules and funcs that x depends on.
func (g *Graph) Dependencies(x util.T) map[util.T]struct{} {
	return g.adj[x]
}

// Sort returns a slice of rules and functions sorted by dependencies. If a cycle
// is found, ok is set to false.
func (g *Graph) Sort() (sorted []util.T, ok bool) {
	if g.sorted != nil {
		return g.sorted, true
	}

	sort := &graphSort{
		sorted: make([]util.T, 0, len(g.nodes)),
		deps:   g.Dependencies,
		marked: map[util.T]struct{}{},
		temp:   map[util.T]struct{}{},
	}

	for node := range g.nodes {
		if !sort.Visit(node) {
			return nil, false
		}
	}

	g.sorted = sort.sorted
	return g.sorted, true
}

func (g *Graph) addDependency(u util.T, v util.T) {

	if _, ok := g.nodes[u]; !ok {
		g.addNode(u)
	}

	if _, ok := g.nodes[v]; !ok {
		g.addNode(v)
	}

	edges, ok := g.adj[u]
	if !ok {
		edges = map[util.T]struct{}{}
		g.adj[u] = edges
	}

	edges[v] = struct{}{}
}

func (g *Graph) addNode(n util.T) {
	g.nodes[n] = struct{}{}
}

type graphSort struct {
	sorted []util.T
	deps   func(util.T) map[util.T]struct{}
	marked map[util.T]struct{}
	temp   map[util.T]struct{}
}

func (sort *graphSort) Marked(node util.T) bool {
	_, marked := sort.marked[node]
	return marked
}

func (sort *graphSort) Visit(node util.T) (ok bool) {
	if _, ok := sort.temp[node]; ok {
		return false
	}
	if sort.Marked(node) {
		return true
	}
	sort.temp[node] = struct{}{}
	for other := range sort.deps(node) {
		if !sort.Visit(other) {
			return false
		}
	}
	sort.marked[node] = struct{}{}
	delete(sort.temp, node)
	sort.sorted = append(sort.sorted, node)
	return true
}

type graphTraversal struct {
	graph   *Graph
	visited map[util.T]struct{}
}

func newgraphTraversal(graph *Graph) *graphTraversal {
	return &graphTraversal{
		graph:   graph,
		visited: map[util.T]struct{}{},
	}
}

func (g *graphTraversal) Edges(x util.T) []util.T {
	r := []util.T{}
	for v := range g.graph.Dependencies(x) {
		r = append(r, v)
	}
	return r
}

func (g *graphTraversal) Visited(u util.T) bool {
	_, ok := g.visited[u]
	g.visited[u] = struct{}{}
	return ok
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
		for v := range e.Vars(safetyCheckVarVisitorParams) {
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
			g.Update(reordered[i-1].Vars(safetyCheckVarVisitorParams))
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
	case *ObjectComprehension:
		vis.checkObjectComprehensionSafety(x)
		return nil
	case *SetComprehension:
		vis.checkSetComprehensionSafety(x)
		return nil
	}
	return vis
}

// Check term for safety. This is analogous to the rule head safety check.
func (vis *bodySafetyVisitor) checkComprehensionSafety(tv VarSet, body Body) Body {
	bv := body.Vars(safetyCheckVarVisitorParams)
	bv.Update(vis.globals)
	uv := tv.Diff(bv)
	for v := range uv {
		vis.unsafe.Add(vis.current, v)
	}

	// Check body for safety, reordering as necessary.
	r, u := reorderBodyForSafety(vis.globals, body)
	if len(u) == 0 {
		return r
	}

	vis.unsafe.Update(u)
	return body
}

func (vis *bodySafetyVisitor) checkArrayComprehensionSafety(ac *ArrayComprehension) {
	ac.Body = vis.checkComprehensionSafety(ac.Term.Vars(), ac.Body)
}

func (vis *bodySafetyVisitor) checkObjectComprehensionSafety(oc *ObjectComprehension) {
	tv := oc.Key.Vars()
	tv.Update(oc.Value.Vars())
	oc.Body = vis.checkComprehensionSafety(tv, oc.Body)
}

func (vis *bodySafetyVisitor) checkSetComprehensionSafety(sc *SetComprehension) {
	sc.Body = vis.checkComprehensionSafety(sc.Term.Vars(), sc.Body)
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
			cv := vs.Intersect(body.Vars(safetyCheckVarVisitorParams)).Diff(globals)
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
	exclude   VarSet
	generated VarSet
}

func newLocalVarGenerator(module *Module) *localVarGenerator {
	exclude := NewVarSet()
	vis := &VarVisitor{
		vars: exclude,
	}
	Walk(vis, module)
	return &localVarGenerator{exclude, NewVarSet()}
}

func (l *localVarGenerator) Generated() VarSet {
	return l.generated
}

func (l *localVarGenerator) Generate() Var {
	name := Var("")
	x := 0
	for len(name) == 0 || l.generated.Contains(name) || l.exclude.Contains(name) {
		name = Var(fmt.Sprintf(localVarFmt, x))
		x++
	}
	l.generated.Add(name)
	return name
}

func getGlobals(pkg *Package, rules []Var, funcs []*Func, imports []*Import) map[Var]Ref {

	globals := map[Var]Ref{}

	// Populate globals with exports within the package.
	for _, v := range rules {
		global := append(Ref{}, pkg.Path...)
		global = append(global, &Term{Value: String(v)})
		globals[v] = global
	}
	for _, fn := range funcs {
		globals[fn.Head.Name] = fn.Path()
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

func resolveRef(globals map[Var]Ref, ref Ref) Ref {

	r := Ref{}
	for i, x := range ref {
		switch v := x.Value.(type) {
		case Var:
			if g, ok := globals[v]; ok {
				cpy := g.Copy()
				for i := range cpy {
					cpy[i].SetLocation(x.Location)
				}
				if i == 0 {
					r = cpy
				} else {
					r = append(r, NewTerm(cpy).SetLocation(x.Location))
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

func resolveRefsInRule(globals map[Var]Ref, rule *Rule) {
	if rule.Head.Key != nil {
		rule.Head.Key = resolveRefsInTerm(globals, rule.Head.Key)
	}
	if rule.Head.Value != nil {
		rule.Head.Value = resolveRefsInTerm(globals, rule.Head.Value)
	}
	rule.Body = resolveRefsInBody(globals, rule.Body)
}

func resolveRefsInFunc(globals map[Var]Ref, fn *Func) {
	for i := range fn.Head.Args {
		fn.Head.Args[i] = resolveRefsInTerm(globals, fn.Head.Args[i])
	}
	fn.Head.Output = resolveRefsInTerm(globals, fn.Head.Output)
	fn.Body = resolveRefsInBody(globals, fn.Body)
}

func resolveRefsInBody(globals map[Var]Ref, body Body) Body {
	r := Body{}
	for _, expr := range body {
		r = append(r, resolveRefsInExpr(globals, expr))
	}
	return r
}

func resolveRefsInExpr(globals map[Var]Ref, expr *Expr) *Expr {
	cpy := *expr
	switch ts := expr.Terms.(type) {
	case *Term:
		cpy.Terms = resolveRefsInTerm(globals, ts)
	case []*Term:
		buf := []*Term{}

		// Resolve user defined functions.
		v := Var(ts[0].Value.(String))
		if r, ok := globals[v]; ok {
			tcpy := *ts[0]
			tcpy.Value = String(r.String())
			buf = append(buf, &tcpy)
			ts = ts[1:]
		}

		for _, t := range ts {
			buf = append(buf, resolveRefsInTerm(globals, t))
		}
		cpy.Terms = buf
	}
	for _, w := range cpy.With {
		w.Target = resolveRefsInTerm(globals, w.Target)
		w.Value = resolveRefsInTerm(globals, w.Value)
	}
	return &cpy
}

func resolveRefsInTerm(globals map[Var]Ref, term *Term) *Term {
	switch v := term.Value.(type) {
	case Var:
		if g, ok := globals[v]; ok {
			cpy := g.Copy()
			for i := range cpy {
				cpy[i].SetLocation(term.Location)
			}
			return NewTerm(cpy).SetLocation(term.Location)
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
	case *ObjectComprehension:
		oc := &ObjectComprehension{}
		oc.Key = resolveRefsInTerm(globals, v.Key)
		oc.Value = resolveRefsInTerm(globals, v.Value)
		oc.Body = resolveRefsInBody(globals, v.Body)
		cpy := *term
		cpy.Value = oc
		return &cpy
	case *SetComprehension:
		sc := &SetComprehension{}
		sc.Term = resolveRefsInTerm(globals, v.Term)
		sc.Body = resolveRefsInBody(globals, v.Body)
		cpy := *term
		cpy.Value = sc
		return &cpy
	default:
		return term
	}
}
