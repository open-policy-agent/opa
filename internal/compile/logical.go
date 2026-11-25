// Copyright 2026 The OPA Authors
// SPDX-License-Identifier: Apache-2.0

package compile

import (
	"github.com/open-policy-agent/opa/v1/ast"
)

// FlattenLogical rewrites the logical expressions that Rego v2 (and the
// corresponding future keywords) leaves in partial evaluation results —
// `a and b`, `a or b`, `not <body>` — into the flat fragment that the filter
// targets express: plain and negated expressions, one query per disjunct. The
// result is what partial evaluation produces for the equivalent Rego v1 policy.
//
// Partial evaluation saves some of those negations in support rules, which a
// negation then refers to: their bodies are what is negated.
//
// The number of disjuncts grows exponentially with the `or` expressions in a
// body, and the input can set that number through an `every` over it: a query
// that would expand into more than maxDisjuncts is an error.
func FlattenLogical(queries []ast.Body, support []*ast.Module) ([]ast.Body, *ast.Error) {
	f := flattener{support: negatedSupport(support)}
	var out []ast.Body
	for _, q := range queries {
		out = append(out, f.dnf(q)...)
		if f.tooBig {
			return nil, err(f.loc, "expands into more than %d alternatives", maxDisjuncts)
		}
	}
	if len(out) == 0 { // no disjunct is satisfiable: an unconditional NO
		return nil, nil
	}
	return out, nil
}

const maxDisjuncts = 1024

// flattener records whether the disjuncts exceeded maxDisjuncts, and where.
type flattener struct {
	support map[string][]ast.Body
	tooBig  bool
	loc     *ast.Location
}

// negatedSupport returns the bodies of the support rules that are true or
// undefined, by their path: what partial evaluation saves a negation in.
func negatedSupport(modules []*ast.Module) map[string][]ast.Body {
	bodies := map[string][]ast.Body{}
	unfit := map[string]bool{}
	for _, m := range modules {
		for _, r := range m.Rules {
			path := r.Ref().String()
			if len(r.Head.Args) > 0 || r.Default || r.Else != nil || !ast.InternedTerm(true).Equal(r.Head.Value) {
				unfit[path] = true
			}
			bodies[path] = append(bodies[path], r.Body)
		}
	}
	for path := range unfit {
		delete(bodies, path)
	}
	return bodies
}

// limit returns bodies, or nothing once there are too many of them.
func (f *flattener) limit(expr *ast.Expr, bodies []ast.Body) []ast.Body {
	if f.over(expr, len(bodies)) {
		return nil
	}
	return bodies
}

// over records, and reports, when expr has more than maxDisjuncts.
func (f *flattener) over(expr *ast.Expr, n int) bool {
	if !f.tooBig && n > maxDisjuncts {
		f.tooBig, f.loc = true, expr.Loc()
	}
	return f.tooBig
}

// dnf returns one body per disjunct that the conjunction body is equivalent to.
func (f *flattener) dnf(body ast.Body) []ast.Body {
	out := []ast.Body{{}}
	for _, expr := range body {
		out = f.product(expr, out, f.dnfExpr(expr))
	}
	return out
}

// dnfExpr returns one body per disjunct that expr is equivalent to. A negation
// that can't be taken apart, and anything with `with`, are kept for the checks
// to reject.
func (f *flattener) dnfExpr(expr *ast.Expr) []ast.Body {
	if len(expr.With) > 0 {
		return []ast.Body{{expr}}
	}
	switch terms := expr.Terms.(type) {
	case *ast.LogicalOr:
		return f.limit(expr, append(f.dnf(terms.Lhs), f.dnf(terms.Rhs)...))
	case *ast.LogicalAnd:
		return f.product(expr, f.dnf(terms.Lhs), f.dnf(terms.Rhs))
	case *ast.Not:
		if bodies, ok := f.complementBody(expr, terms.Body); ok {
			return bodies
		}
	}
	return []ast.Body{{expr}}
}

// complementBody returns the disjunction that the negation of the conjunction
// body is equivalent to: by De Morgan, one disjunct per expression. It reports
// false for a body whose expressions share a variable, which can't be taken apart.
func (f *flattener) complementBody(not *ast.Expr, body ast.Body) ([]ast.Body, bool) {
	if sharesVar(body) {
		return nil, false
	}
	var out []ast.Body
	for _, expr := range body {
		bodies, ok := f.complementExpr(expr)
		if !ok {
			return nil, false
		}
		out = f.limit(not, append(out, bodies...))
	}
	return out, true
}

// complementExpr returns one body per disjunct that the negation of expr is
// equivalent to, or false if it has none.
func (f *flattener) complementExpr(expr *ast.Expr) ([]ast.Body, bool) {
	if len(expr.With) > 0 {
		return nil, false
	}
	switch terms := expr.Terms.(type) {
	case *ast.LogicalOr:
		l, r, ok := f.complementOperands(expr, terms.Lhs, terms.Rhs)
		return f.product(expr, l, r), ok
	case *ast.LogicalAnd:
		l, r, ok := f.complementOperands(expr, terms.Lhs, terms.Rhs)
		return f.limit(expr, append(l, r...)), ok
	case *ast.Not:
		return f.dnf(terms.Body), true
	case *ast.Term:
		if bodies, ok := f.support[terms.String()]; ok && !expr.Negated {
			out := []ast.Body{{}}
			for _, b := range bodies {
				c, ok := f.complementBody(expr, b)
				if !ok {
					return nil, false
				}
				out = f.product(expr, out, c)
			}
			return out, true
		}
	}
	return []ast.Body{{expr.Complement()}}, true
}

func (f *flattener) complementOperands(expr *ast.Expr, lhs, rhs ast.Body) ([]ast.Body, []ast.Body, bool) {
	l, ok := f.complementBody(expr, lhs)
	if !ok {
		return nil, nil, false
	}
	r, ok := f.complementBody(expr, rhs)
	return l, r, ok
}

// sharesVar reports whether two expressions of body use the same variable.
func sharesVar(body ast.Body) bool {
	seen := ast.NewVarSet()
	for _, expr := range body {
		vars := exprVars(expr)
		if len(vars.Intersect(seen)) > 0 {
			return true
		}
		seen.Update(vars)
	}
	return false
}

// exprVars returns the variables of x, the heads of refs included: in `x.a`,
// x is one. input and data aren't.
func exprVars(x any) ast.VarSet {
	vis := ast.NewVarVisitor().WithParams(ast.VarVisitorParams{SkipRefCallHead: true, SkipClosures: true})
	vis.Walk(x)
	return vis.Vars().Diff(rootDocuments)
}

var rootDocuments = ast.NewVarSet(ast.InputRootDocument.Value.(ast.Var), ast.DefaultRootDocument.Value.(ast.Var))

// product returns the conjunctions of every body in as with every body in bs.
func (f *flattener) product(expr *ast.Expr, as, bs []ast.Body) []ast.Body {
	if len(as) == 0 || len(bs) == 0 || f.over(expr, len(as)*len(bs)) {
		return nil
	}
	out := make([]ast.Body, 0, len(as)*len(bs))
	for _, a := range as {
		for _, b := range bs {
			out = append(out, concatBody(a, b))
		}
	}
	return out
}

func concatBody(a, b ast.Body) ast.Body {
	out := make(ast.Body, 0, len(a)+len(b))
	out = append(out, a...)
	return append(out, b...)
}
