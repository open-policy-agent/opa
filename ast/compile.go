// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"fmt"
	"sort"
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
		stage{c.checkBuiltinOperators, "checkBuiltinOperators"},
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

// checkBuiltinOperators ensures that all built-in operators are defined in the global
// BuiltinMap.
//
// NOTE(tsandall): in the future this could potentially be replaced with a schema check.
func (c *Compiler) checkBuiltinOperators() {
	for _, m := range c.Modules {
		for _, r := range m.Rules {
			for _, expr := range r.Body {
				ts, ok := expr.Terms.([]*Term)
				if !ok {
					continue
				}
				operator := ts[0].Value.(Var)
				if _, ok := BuiltinMap[operator]; !ok {
					c.err("bad built-in operator in %v: %v", r.Name, operator)
				}
			}
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
			c.err("recursion found in %v: %v", r.Name, strings.Join(n, ", "))
		}
	}
}

// checkSafetyBody ensures that variables appearing in negated expressions or non-target
// positions of built-in expressions will be bound when evaluating the rule from left
// to right. If the variable is bound in an expression that would be processed *after*
// the negated or built-in expression, the expressions will be re-ordered.
func (c *Compiler) checkSafetyBody() {
	for _, m := range c.Modules {
		for _, r := range m.Rules {

			// Build map of expression to variables contained in the expression that
			// are (potentially) unsafe. That is, any variable that is not a global.
			unsafe := map[*Expr]map[Var]struct{}{}
			for _, e := range r.Body {
				cv := &collectAllVarsVisitor{}
				e.Walk(cv.Walk)
				for _, v := range cv.Vars {
					// TODO(tsandall): consider including certain built-in variables
					// in the Globals. E.g., data, =, !=, etc.
					if DefaultRootDocument.Value.Equal(v) {
						continue
					}
					if _, ok := c.Globals[m][v]; !ok {
						if u, ok := unsafe[e]; ok {
							u[v] = struct{}{}
						} else {
							unsafe[e] = map[Var]struct{}{v: struct{}{}}
						}
					}
				}
			}

			safe := map[Var]struct{}{}
			reordered := Body{}

			for _, e := range r.Body {

				// Update "safe" map to include variables in this expression
				// that are safe, i.e., variables in the target positions
				// of built-ins or variables in references.
				if !e.Negated {
					switch ts := e.Terms.(type) {
					case *Term:
						if r, ok := ts.Value.(Ref); ok {
							cv := &collectAllVarsVisitor{}
							for _, t := range r[1:] {
								t.Walk(cv.Walk)
							}
							for _, v := range cv.Vars {
								safe[v] = struct{}{}
							}
						}
					case []*Term:
						b := BuiltinMap[ts[0].Value.(Var)]
						for i, t := range ts[1:] {
							if t.IsGround() {
								continue
							}
							if b.UnifiesRecursively(i) {
								cv := &collectAllVarsVisitor{}
								// If the term is a reference, any variables
								// in the reference (except the head) are safe.
								// Make a copy of the term so that we can exclude
								// the head while walking the rest of the reference.
								tmp := *t
								if r, ok := t.Value.(Ref); ok {
									tmp.Value = r[1:]
								}
								tmp.Walk(cv.Walk)
								for _, v := range cv.Vars {
									safe[v] = struct{}{}
								}
								continue
							}
							if v, ok := t.Value.(Var); ok {
								if b.Unifies(i) {
									safe[v] = struct{}{}
									continue
								}
							}
						}
					}
				}

				// Update the set of unsafe variables for this expression and
				// check if the expression is safe.
				for v := range unsafe[e] {
					if _, ok := safe[v]; ok {
						delete(unsafe[e], v)
					}
				}

				if len(unsafe[e]) == 0 {
					reordered = append(reordered, e)
					delete(unsafe, e)

					// Check if other expressions in the body are considered safe
					// now. If they are considered safe now, they can be added
					// to the end of the re-ordered body.
					for _, e := range r.Body {
						if reordered.Contains(e) {
							continue
						}
						for v := range unsafe[e] {
							if _, ok := safe[v]; ok {
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

			if len(unsafe) != 0 {
				vars := map[Var]struct{}{}
				for _, vs := range unsafe {
					for v := range vs {
						vars[v] = struct{}{}
					}
				}
				unique := []string{}
				for v := range vars {
					unique = append(unique, string(v))
				}
				sort.Strings(unique)
				c.err("unsafe variables in %v: %v", r.Name, strings.Join(unique, ", "))
			} else {
				r.Body = reordered
			}
		}
	}
}

// checkSafetyHead ensures that variables appearing in the head of a
// rule also appear in the body.
func (c *Compiler) checkSafetyHead() {
	for _, m := range c.Modules {
		for _, r := range m.Rules {

			headVars := []*Term{}

			// Accumulate variables appearing in the head of the rule.
			// Stop as soon as the body is encountered.
			r.Walk(func(v interface{}) bool {
				switch v := v.(type) {
				case Body:
					return true
				case *Term:
					if _, ok := v.Value.(Var); ok {
						headVars = append(headVars, v)
					}
				}
				return false
			})

			// Walk over all terms in the body until all variables
			// in the head have been seen or there are no more elements.
			r.Body.Walk(func(v interface{}) bool {
				t, ok := v.(*Term)
				if !ok {
					return false
				}
				for i := range headVars {
					if headVars[i].Equal(t) {
						headVars = append(headVars[:i], headVars[i+1:]...)
						break
					}
				}
				return len(headVars) == 0
			})

			if len(headVars) > 0 {
				for _, v := range headVars {
					c.err("unsafe variable from head of %v: %v", r.Name, v)
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
	for _, m := range c.Modules {
		for _, rule := range m.Rules {
			for _, expr := range rule.Body {
				switch ts := expr.Terms.(type) {
				case *Term:
					expr.Terms = c.resolveRefs(c.Globals[m], ts)
				case []*Term:
					for i, t := range ts {
						ts[i] = c.resolveRefs(c.Globals[m], t)
					}
				}
			}
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
			r.Body.Walk(func(v interface{}) bool {
				if t, ok := v.(*Term); ok {
					if ref, ok := t.Value.(Ref); ok {
						for _, v := range findRules(c.ModuleTree, ref) {
							edges[v] = struct{}{}
						}
					}
				}
				return false
			})
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

type collectAllVarsVisitor struct {
	Vars []Var
}

func (cv *collectAllVarsVisitor) Walk(v interface{}) bool {
	// Handle expressions as special case so that the built-in function
	// name can be excluded from the results.
	if e, ok := v.(*Expr); ok {
		switch ts := e.Terms.(type) {
		case *Term:
			ts.Walk(cv.Walk)
		case []*Term:
			for _, t := range ts[1:] {
				t.Walk(cv.Walk)
			}
		}
		return true
	}
	if v, ok := v.(*Term).Value.(Var); ok {
		cv.Vars = append(cv.Vars, v)
	}
	return false
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
