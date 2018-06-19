// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"github.com/open-policy-agent/opa/ast"
)

// copyPropagator implements a simple copy propagation optimization to remove
// intermediate variables in partial evaluation results.
//
// For example, given the query: input.x > 1 where 'input' is unknown, the
// compiled query would become input.x = a; a > 1 which would remain in the
// partial evaluation result. The copyPropagator implementation will remove the
// variable assignment so that partial evaluation simply outputs input.x > 1.
//
// In many cases, copy propagation can remove all variables from the result of
// partial evaluation which simplifies evaluation for non-OPA consumers.
//
// In some cases, copy propagation cannot remove all variables. If the output of
// a built-in call is subsequently used as a ref head, the output variable must
// be kept. For example. sort(input, x); x[0] == 1. In this case, copy
// propagation cannot replace x[0] == 1 with sort(input, x)[0] == 1 as this is
// not legal.
type copyPropagator struct {
	liveSet ast.VarSet // vars that cannot be removed.
}

func newCopyPropagator(callerQuery ast.Body) *copyPropagator {

	liveSet := ast.NewVarSet()

	ast.WalkVars(callerQuery, func(v ast.Var) bool {
		if !v.IsGenerated() {
			liveSet.Add(v)
		}
		return false
	})

	return &copyPropagator{
		liveSet: liveSet,
	}
}

func (p *copyPropagator) Apply(query ast.Body) (result ast.Body) {

	headSet := newHeadSet(query)
	killed := map[ast.Var]*ast.Term{} // vars to substitute with other values.

	for _, expr := range query {
		p.substitute(killed, expr)
		if v, t, kill := p.check(headSet, expr); kill {
			killed[v] = t
		} else {
			result.Append(expr)
		}
	}

	return
}

func (p *copyPropagator) substitute(killed map[ast.Var]*ast.Term, x interface{}) {
	ast.WalkTerms(x, func(t *ast.Term) bool {
		switch v := t.Value.(type) {
		case ast.Var:
			if v2, ok := killed[v]; ok {
				t.Value = v2.Value
				return true
			}
		case ast.Ref:
			// Refs require special handling. if the head var has been killed, the ref
			// head has to be substituted by concatenating the corresponding term with
			// the rest of the ref. For example, given:
			//
			// ref: ref(x, 0) and binding: x/ref(input, "foo")
			//
			// The result should be ref(input, "foo", 0) and not ref(ref(input, "foo"), 0).
			if v2, ok := killed[v[0].Value.(ast.Var)]; ok {
				// Invariant: killed vars are always bound to refs. Vars are never bound to
				// calls if they are subsequently used as ref heads.
				r := v2.Value.(ast.Ref)
				var concat ast.Ref
				for i := 0; i < len(r); i++ {
					concat = append(concat, r[i])
				}
				for i := 1; i < len(v); i++ {
					concat = append(concat, v[i])
				}
				t.Value = concat
			}
			for i := 1; i < len(v); i++ {
				p.substitute(killed, v[i])
			}
			return true
		}
		return false
	})
}

func (p *copyPropagator) check(headSet *headSet, expr *ast.Expr) (v ast.Var, t *ast.Term, kill bool) {

	if expr.IsEquality() {
		a, b := expr.Operand(0), expr.Operand(1)
		v, t, kill = p.checkEq(a, b)
		if kill {
			return
		}
		v, t, kill = p.checkEq(b, a)
		return
	}

	if expr.IsCall() {
		terms := expr.Terms.([]*ast.Term)
		output := terms[len(terms)-1]
		av, isVar := output.Value.(ast.Var)
		if !isVar || p.liveSet.Contains(av) || headSet.Connected(av) {
			return
		}
		return av, ast.CallTerm(terms[:len(terms)-1]...), true
	}

	return
}

func (p *copyPropagator) checkEq(a, b *ast.Term) (v ast.Var, t *ast.Term, kill bool) {

	av, isVar := a.Value.(ast.Var)
	if !isVar || p.liveSet.Contains(av) {
		return
	}

	switch b.Value.(type) {
	case ast.Ref, ast.Call:
		break
	default:
		return
	}
	return av, b, true
}

type headSet struct {
	headVars ast.VarSet
	parents  map[ast.Var]ast.Var
}

func newHeadSet(query ast.Body) *headSet {

	s := &headSet{
		parents:  map[ast.Var]ast.Var{},
		headVars: ast.NewVarSet(),
	}

	for _, expr := range query {
		if expr.IsEquality() {
			a, b := expr.Operand(0), expr.Operand(1)
			va, ok1 := a.Value.(ast.Var)
			vb, ok2 := b.Value.(ast.Var)
			if ok1 && ok2 {
				s.merge(va, vb)
			}
		}
		ast.WalkRefs(expr, func(v ast.Ref) bool {
			s.headVars.Add(v[0].Value.(ast.Var))
			return false
		})
	}

	return s
}

func (s *headSet) Connected(v ast.Var) bool {
	for head := range s.headVars {
		if head == v {
			return true
		}
		r1, ok1 := s.find(v)
		r2, ok2 := s.find(head)
		if ok1 && ok2 && r1 == r2 {
			return true
		}
	}
	return false
}

func (s *headSet) merge(x, y ast.Var) {
	r1 := s.makeSet(x)
	r2 := s.makeSet(y)
	if r1 != r2 {
		s.parents[r1] = r2
	}
}

func (s *headSet) find(x ast.Var) (ast.Var, bool) {
	if parent, ok := s.parents[x]; ok {
		if parent == x {
			return parent, true
		}
		return s.find(parent)
	}
	return "", false
}

func (s *headSet) makeSet(x ast.Var) ast.Var {
	if root, ok := s.find(x); ok {
		return root
	}
	s.parents[x] = x
	return x
}
