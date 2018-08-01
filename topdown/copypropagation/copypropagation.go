// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package copypropagation

import (
	"sort"

	"github.com/open-policy-agent/opa/ast"
)

// CopyPropagator implements a simple copy propagation optimization to remove
// intermediate variables in partial evaluation results.
//
// For example, given the query: input.x > 1 where 'input' is unknown, the
// compiled query would become input.x = a; a > 1 which would remain in the
// partial evaluation result. The CopyPropagator will remove the variable
// assignment so that partial evaluation simply outputs input.x > 1.
//
// In many cases, copy propagation can remove all variables from the result of
// partial evaluation which simplifies evaluation for non-OPA consumers.
//
// In some cases, copy propagation cannot remove all variables. If the output of
// a built-in call is subsequently used as a ref head, the output variable must
// be kept. For example. sort(input, x); x[0] == 1. In this case, copy
// propagation cannot replace x[0] == 1 with sort(input, x)[0] == 1 as this is
// not legal.
type CopyPropagator struct {
	livevars ast.VarSet // vars that must be preserved in the resulting query
	sorted   []ast.Var  // sorted copy of vars to ensure deterministic result
}

// New returns a new CopyPropagator that optimizes queries while preserving vars
// in the livevars set.
func New(livevars ast.VarSet) *CopyPropagator {

	sorted := make([]ast.Var, 0, len(livevars))
	for v := range livevars {
		sorted = append(sorted, v)
	}

	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Compare(sorted[j]) < 0
	})

	return &CopyPropagator{livevars: livevars, sorted: sorted}
}

// Apply executes the copy propagation optimization and returns a new query.
func (p *CopyPropagator) Apply(query ast.Body) (result ast.Body) {

	uf, ok := p.makeDisjointSets(query)
	if !ok {
		return query
	}

	// Compute set of vars that appear in the head of refs in the query. If a var
	// is dereferenced, we cannot plug it with a constant value so the constant on
	// the union-find root must be unset (e.g., [1][0] is not legal.)
	headvars := ast.NewVarSet()
	ast.WalkRefs(query, func(x ast.Ref) bool {
		if v, ok := x[0].Value.(ast.Var); ok {
			headvars.Add(v)
			if root, ok := uf.Find(v); ok {
				root.constant = nil
			}
		}
		return false
	})

	bindings := map[ast.Var]*binding{}

	for _, expr := range query {

		// Deep copy the expr as it may be mutated below. The caller that is running
		// copy propagation may hold references to the expr.
		expr = expr.Copy()

		p.plugBindings(bindings, uf, expr)

		if keep := p.updateBindings(bindings, uf, headvars, expr); keep {
			result.Append(expr)
		}
	}

	// Run post-processing step on query to ensure that all killed exprs are
	// accounted for. If an expr is killed but the binding is never used, the query
	// must still include the expr. For example, given the query 'input.x = a' and
	// an empty livevar set, the result must include the ref input.x otherwise the
	// query could be satisfied without input.x being defined. When exprs are
	// killed we initialize the binding counter to zero and then increment it each
	// time the binding is substituted. if the binding was never substituted it
	// means the binding value must be added back into the query.
	for _, b := range sortbindings(bindings) {
		if b.n == 0 {
			result.Append(ast.Equality.Expr(ast.NewTerm(b.k), ast.NewTerm(b.v)))
		}
	}

	// Run post-processing step on the query to ensure that all live vars are bound
	// in the result. The plugging that happens above substitutes all vars in the
	// same set with the root.
	for _, v := range p.sorted {
		if root, ok := uf.Find(v); ok {
			if root.constant != nil {
				result.Append(ast.Equality.Expr(ast.NewTerm(v), root.constant))
			} else if b, ok := bindings[root.key]; ok {
				result.Append(ast.Equality.Expr(ast.NewTerm(v), ast.NewTerm(b.v)))
			} else if root.key != v {
				result.Append(ast.Equality.Expr(ast.NewTerm(v), ast.NewTerm(root.key)))
			}
		}
	}

	return result
}

// makeDisjointSets builds the union-find structure for the query. The structure
// is built by processing all of the equality exprs in the query. Sets represent
// vars that must be equal to each other. In addition to vars, each can have at
// most one more . If the query contains expressions that cannot be satisfied
// (e.g., because a set has multiple constants) this function returns false.
func (p *CopyPropagator) makeDisjointSets(query ast.Body) (*unionFind, bool) {
	uf := newUnionFind()
	for _, expr := range query {
		if expr.IsEquality() {
			a, b := expr.Operand(0), expr.Operand(1)
			varA, ok1 := a.Value.(ast.Var)
			varB, ok2 := b.Value.(ast.Var)
			if ok1 && ok2 {
				if _, ok := uf.Merge(varA, varB); !ok {
					return nil, false
				}
			} else if ok1 && ast.IsConstant(b.Value) {
				root := uf.MakeSet(varA)
				if root.constant != nil && !root.constant.Equal(b) {
					return nil, false
				}
				root.constant = b
			} else if ok2 && ast.IsConstant(a.Value) {
				root := uf.MakeSet(varB)
				if root.constant != nil && !root.constant.Equal(a) {
					return nil, false
				}
				root.constant = a
			}
		}
	}

	return uf, true
}

// plugBindings applies the binding list and union-find to x. This process
// removes as many variables as possible.
func (p *CopyPropagator) plugBindings(bindings map[ast.Var]*binding, uf *unionFind, x interface{}) {
	ast.WalkTerms(x, func(t *ast.Term) bool {
		// Apply union-find to remove redundant variables from input.
		switch v := t.Value.(type) {
		case ast.Var:
			if root, ok := uf.Find(v); ok {
				t.Value = root.Value()
			}
		case ast.Ref:
			if root, ok := uf.Find(v[0].Value.(ast.Var)); ok {
				v[0].Value = root.Value()
			}
		}
		// Apply binding list to substitute remaining vars.
		switch v := t.Value.(type) {
		case ast.Var:
			if b, ok := bindings[v]; ok {
				t.Value = b.v
				b.n++
				return true
			}
		case ast.Ref:
			// Refs require special handling. If the head of the ref was killed, then the
			// rest of the ref must be concatenated with the new base.
			//
			// Invariant: ref heads can only be replaced by refs (not calls).
			if b, ok := bindings[v[0].Value.(ast.Var)]; ok {
				t.Value = b.v.(ast.Ref).Concat(v[1:])
				b.n++
			}
			for i := 1; i < len(v); i++ {
				p.plugBindings(bindings, uf, v[i])
			}
			return true
		}
		return false
	})
}

// updateBindings returns false if the expression can be killed. If the
// expression is killed, the binding list is updated to map a var to value.
func (p *CopyPropagator) updateBindings(bindings map[ast.Var]*binding, uf *unionFind, headvars ast.VarSet, expr *ast.Expr) bool {
	if expr.IsEquality() {
		a, b := expr.Operand(0), expr.Operand(1)
		if a.Equal(b) {
			return false
		}
		k, v, keep := p.updateBindingsEq(a, b)
		if !keep {
			if v != nil {
				bindings[k] = newbinding(k, v)
			}
			return false
		}
	} else if expr.IsCall() {
		terms := expr.Terms.([]*ast.Term)
		output := terms[len(terms)-1]
		if k, ok := output.Value.(ast.Var); ok && !p.livevars.Contains(k) && !headvars.Contains(k) {
			bindings[k] = newbinding(k, ast.CallTerm(terms[:len(terms)-1]...).Value)
			return false
		}
	}
	return !isNoop(expr)
}

func (p *CopyPropagator) updateBindingsEq(a, b *ast.Term) (ast.Var, ast.Value, bool) {
	k, v, keep := p.updateBindingsEqAsymmetric(a, b)
	if !keep {
		return k, v, keep
	}
	return p.updateBindingsEqAsymmetric(b, a)
}

func (p *CopyPropagator) updateBindingsEqAsymmetric(a, b *ast.Term) (ast.Var, ast.Value, bool) {
	k, ok := a.Value.(ast.Var)
	if !ok || p.livevars.Contains(k) {
		return "", nil, true
	}

	switch b.Value.(type) {
	case ast.Ref, ast.Call:
		return k, b.Value, false
	}

	return "", nil, true
}

type binding struct {
	k ast.Var
	v ast.Value
	n int // number of times the binding was substituted
}

func newbinding(k ast.Var, v ast.Value) *binding {
	return &binding{k: k, v: v}
}

func sortbindings(bindings map[ast.Var]*binding) []*binding {
	sorted := make([]*binding, 0, len(bindings))
	for _, b := range bindings {
		sorted = append(sorted, b)
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].k.Compare(sorted[j].k) < 0
	})
	return sorted
}

type unionFind struct {
	roots   map[ast.Var]*unionFindRoot
	parents map[ast.Var]ast.Var
}

func newUnionFind() *unionFind {
	return &unionFind{
		roots:   map[ast.Var]*unionFindRoot{},
		parents: map[ast.Var]ast.Var{},
	}
}

func (uf *unionFind) MakeSet(v ast.Var) *unionFindRoot {

	root, ok := uf.Find(v)
	if ok {
		return root
	}

	root = newUnionFindRoot(v)
	uf.parents[v] = v
	uf.roots[v] = root
	return uf.roots[v]
}

func (uf *unionFind) Find(v ast.Var) (*unionFindRoot, bool) {

	parent, ok := uf.parents[v]
	if !ok {
		return nil, false
	}

	if parent == v {
		return uf.roots[v], true
	}

	return uf.Find(parent)
}

func (uf *unionFind) Merge(a, b ast.Var) (*unionFindRoot, bool) {

	r1 := uf.MakeSet(a)
	r2 := uf.MakeSet(b)

	if r1 != r2 {
		uf.parents[r1.key] = r2.key
		delete(uf.roots, r1.key)

		// Sets can have at most one constant value associated with them. When
		// unioning, we must preserve this invariant. If a set has two constants,
		// there will be no way to prove the query.
		if r1.constant != nil && r2.constant != nil && !r1.constant.Equal(r2.constant) {
			return nil, false
		} else if r2.constant == nil {
			r2.constant = r1.constant
		}
	}

	return r2, true
}

type unionFindRoot struct {
	key      ast.Var
	constant *ast.Term
}

func newUnionFindRoot(key ast.Var) *unionFindRoot {
	return &unionFindRoot{
		key: key,
	}
}

func (r *unionFindRoot) Value() ast.Value {
	if r.constant != nil {
		return r.constant.Value
	}
	return r.key
}

func isNoop(expr *ast.Expr) bool {

	if !expr.IsCall() {
		term := expr.Terms.(*ast.Term)
		if !ast.IsConstant(term.Value) {
			return false
		}
		return !ast.Boolean(false).Equal(term.Value)
	}

	// A==A can be ignored
	if expr.Operator().Equal(ast.Equal.Ref()) {
		return expr.Operand(0).Equal(expr.Operand(1))
	}

	return false
}
