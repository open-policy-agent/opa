// Copyright 2026 The OPA Authors
// SPDX-License-Identifier: Apache-2.0

package compile

import (
	"slices"

	"github.com/open-policy-agent/opa/v1/ast"
)

// NormalizeMultiValued rewrites comparisons on the elements of array fields, like
// `some p in input.d.f; p == "v"`, into comparisons on the field, `input.d.f = "v"`,
// which is what a Lucene term query means for a multi-valued field. Whether a field
// is an array comes from schema, a JSON Schema of doc, or from how it is used.
func NormalizeMultiValued(queries []ast.Body, doc ast.Ref, schema any) ([]ast.Body, []*ast.Error) {
	if queries == nil { // unconditional NO
		return nil, nil
	}
	shapes, errs := fieldShapes(doc, schema, queries)
	if errs != nil {
		return nil, errs
	}
	n := normalizer{shapes: shapes}
	out := make([]ast.Body, len(queries))
	for i := range queries {
		out[i] = n.body(queries[i])
	}
	return out, n.errs
}

type normalizer struct {
	errs   []*ast.Error
	shapes map[string]shape
}

func (n *normalizer) fail(loc *ast.Location, f string, vs ...any) {
	n.errs = append(n.errs, err(loc, f, vs...))
}

func (n *normalizer) body(b ast.Body) ast.Body {
	out := make(ast.Body, 0, len(b))
	for _, e := range b {
		out = append(out, n.expr(e, b)...)
	}
	return out
}

// expr rewrites one expression; ctx is the body it is part of.
func (n *normalizer) expr(e *ast.Expr, ctx ast.Body) []*ast.Expr {
	if len(e.With) > 0 {
		return []*ast.Expr{e}
	}
	if e.IsEvery() {
		return n.every(e)
	}
	if t, ok := e.Terms.(*ast.Term); ok && n.shape(t) == array {
		n.fail(e.Loc(), "%v is an array, which is always true: try `%v != []`", t, t)
		return nil
	}
	if !e.IsCall() {
		return []*ast.Expr{e}
	}

	if x, ok := n.call(e, ctx); ok {
		return x
	}
	if e.OperatorTerm().Value.(ast.Ref).String() == ast.Equality.Name {
		for _, pair := range [2][2]*ast.Term{{e.Operand(0), e.Operand(1)}, {e.Operand(1), e.Operand(0)}} {
			if _, null := pair[1].Value.(ast.Null); null && fieldRef(pair[0].Value) {
				n.fail(e.Loc(), "a field without a value is missing, not null: try `not %v != null`: %v", pair[0], e)
				return nil
			}
		}
	}
	if !comparison(e) {
		return []*ast.Expr{e}
	}
	// The collection of `x in input.d.f` is the field; it's x that must not be one.
	member := e.OperatorTerm().Value.(ast.Ref).String() == ast.Member.Name
	for i, t := range e.Operands() {
		if n.shape(t) == array && (i == 0 || !member) {
			n.fail(e.Loc(), "%v is an array: compare its elements, as in `\"v\" in %v` or `some x in %v; x > 1`: %v", t, t, t, e)
			return nil
		}
	}
	return []*ast.Expr{e}
}

// call rewrites the calls on fields that the target can express.
func (n *normalizer) call(e *ast.Expr, ctx ast.Body) ([]*ast.Expr, bool) {
	a, b := e.Operand(0), e.Operand(1)
	if a == nil || b == nil {
		return nil, false
	}
	switch op := e.OperatorTerm().Value.(ast.Ref).String(); op {
	case ast.Equality.Name, ast.NotEqual.Name:
		for _, pair := range [2][2]*ast.Term{{a, b}, {b, a}} {
			if field, ok := elementsOf(pair[0]); ok {
				return n.sameValues(e, field, pair[1], op == ast.Equality.Name)
			}
		}
		for i, pair := range [2][2]*ast.Term{{a, b}, {b, a}} {
			if emptyArray(pair[1].Value) && fieldRef(pair[0].Value) {
				return n.onArray(pair[0], e, func() *ast.Expr { return withOperand(e, 1-i, ast.NullTerm()) })
			}
			if op == ast.NotEqual.Name {
				if _, ok := elementRef(pair[0]); ok && ast.IsScalar(pair[1].Value) {
					n.fail(e.Loc(), "comparing an element of a field with `!=` cannot be expressed: it asks "+
						"for some value that differs, not for no value to match: %v", e)
					return nil, true
				}
			} else if base, ok := n.element(pair[0], ctx); ok && fieldValue(pair[1].Value) {
				return n.onArray(base, e, func() *ast.Expr { return withOperand(e, i, base) })
			}
		}
	case ast.GreaterThan.Name, ast.GreaterThanEq.Name, ast.LessThan.Name, ast.LessThanEq.Name:
		for i, pair := range [2][2]*ast.Term{{a, b}, {b, a}} {
			if base, ok := n.element(pair[0], ctx); ok && fieldValue(pair[1].Value) {
				return n.onArray(base, e, func() *ast.Expr { return withOperand(e, i, base) })
			}
		}
	case ast.StartsWith.Name, ast.EndsWith.Name, ast.Contains.Name, ast.AnyPrefixMatch.Name, ast.AnySuffixMatch.Name:
		if !b.IsGround() {
			break
		}
		if base, ok := n.element(a, ctx); ok {
			return n.onArray(base, e, func() *ast.Expr { return withOperand(e, 0, base) })
		}
	case ast.Member.Name:
		if base, ok := n.element(a, ctx); ok && b.IsGround() {
			return n.onArray(base, e, func() *ast.Expr { return withOperand(e, 0, base) })
		}
		if fieldValue(a.Value) && fieldRef(b.Value) {
			return n.onArray(b, e, func() *ast.Expr { return eqExpr(e, b, a) })
		}
	case ast.Count.Name:
		if !fieldRef(a.Value) {
			break
		}
		num, ok := b.Value.(ast.Number)
		if i, isInt := num.Int(); !ok || !isInt || i != 0 {
			n.fail(e.Loc(), "only `count(f) == 0` is supported, a filter cannot count values: %v", e)
			return nil, true
		}
		return n.onArray(a, e, func() *ast.Expr { return eqExpr(e, a, ast.NullTerm()) })
	}
	return nil, false
}

// sameValues is `{v | v = field[_]} == vs`: field holds each of vs, and nothing else.
func (n *normalizer) sameValues(e *ast.Expr, field, vs *ast.Term, eq bool) ([]*ast.Expr, bool) {
	known, ok := knownValues(vs.Value)
	if _, set := vs.Value.(ast.Set); !set || !ok || !eq || e.Negated {
		n.fail(e.Loc(), "the values of a field can only be equal to a known set: %v", e)
		return nil, true
	}
	if n.single(field, e) {
		return nil, true
	}
	if known.Len() == 0 {
		return []*ast.Expr{eqExpr(e, field, ast.NullTerm())}, true
	}
	var out []*ast.Expr
	sorted := known.Sorted()
	for i := range sorted.Len() {
		out = append(out, eqExpr(&ast.Expr{Location: e.Location}, field, sorted.Elem(i)))
	}
	return append(out, n.outside(e, field, known)...), true
}

// onArray is the rewrite of e, which treats field as an array.
func (n *normalizer) onArray(field *ast.Term, e *ast.Expr, rewrite func() *ast.Expr) ([]*ast.Expr, bool) {
	if n.single(field, e) {
		return nil, true
	}
	return []*ast.Expr{rewrite()}, true
}

// single reports, as an error, that e treats field as an array, which it isn't.
func (n *normalizer) single(field *ast.Term, e *ast.Expr) bool {
	if n.shape(field) != single {
		return false
	}
	n.fail(e.Loc(), "%v is a single value, not an array: compare it directly, as in `%v == \"v\"`: %v", field, field, e)
	return true
}

func (n *normalizer) every(e *ast.Expr) []*ast.Expr {
	every := e.Terms.(*ast.Every)
	if e.Negated {
		n.fail(e.Loc(), "a negated `every` is not supported: %v", e)
		return nil
	}

	if fieldRef(every.Domain.Value) {
		if n.single(every.Domain, e) {
			return nil
		}
		return n.overField(every, e)
	}
	n.fail(e.Loc(), "`every` must range over a field: %v", e)
	return nil
}

// overField rewrites `every p in f { ... }`: "no value of f is v" is `not f = "v"`,
// and "every value is one of S" is "no value is outside S".
func (n *normalizer) overField(every *ast.Every, e *ast.Expr) []*ast.Expr {
	fail := func() []*ast.Expr {
		n.fail(e.Loc(), "`every` over a field can only compare the element with known values (`==`, `!=`, `in`, `not in`, `not <`, `not startswith`): %v", e)
		return nil
	}
	val, ok := every.Value.Value.(ast.Var)
	if !ok || len(every.Body) != 1 {
		return fail()
	}
	inner := every.Body[0]
	if len(inner.With) > 0 || !inner.IsCall() {
		return fail()
	}
	a, b := inner.Operand(0), inner.Operand(1)
	if a == nil || b == nil {
		return fail()
	}
	field := every.Domain

	isVal := func(t *ast.Term) bool { v, ok := t.Value.(ast.Var); return ok && v == val }
	switch inner.OperatorTerm().Value.(ast.Ref).String() {
	case ast.NotEqual.Name, ast.Equality.Name:
		all := (inner.OperatorTerm().Value.(ast.Ref).String() == ast.NotEqual.Name) == inner.Negated // `p == v`, `not p != v`
		for _, pair := range [2][2]*ast.Term{{a, b}, {b, a}} {
			if isVal(pair[0]) && fieldValue(pair[1].Value) {
				if all {
					return n.outside(e, field, ast.NewSet(pair[1]))
				}
				x := eqExpr(inner, field, pair[1])
				x.Negated = true
				return []*ast.Expr{x}
			}
		}
	case ast.Member.Name:
		if !isVal(a) || !b.IsGround() {
			break
		}
		if !inner.Negated {
			if vs, ok := knownValues(b.Value); ok {
				return n.outside(e, field, vs)
			}
			break
		}
		x := inner.Copy()
		x.Negated = true
		return []*ast.Expr{withOperand(x, 0, field)}
	case ast.GreaterThan.Name, ast.GreaterThanEq.Name, ast.LessThan.Name, ast.LessThanEq.Name:
		for i, pair := range [2][2]*ast.Term{{a, b}, {b, a}} {
			if inner.Negated && isVal(pair[0]) && fieldValue(pair[1].Value) {
				return []*ast.Expr{withOperand(inner, i, field)}
			}
		}
	case ast.StartsWith.Name, ast.EndsWith.Name, ast.Contains.Name, ast.AnyPrefixMatch.Name, ast.AnySuffixMatch.Name:
		if b.IsGround() && inner.Negated && isVal(a) {
			return []*ast.Expr{withOperand(inner, 0, field)}
		}
	}
	return fail()
}

// outside is `not internal.lucene_outside(field, vs)`: every value is one of vs.
// Lucene says it with the ranges between and around them, so vs must be ordered.
func (n *normalizer) outside(e *ast.Expr, field *ast.Term, vs ast.Set) []*ast.Expr {
	sorted := vs.Sorted()
	for i := range sorted.Len() {
		if ast.TypeName(sorted.Elem(i).Value) != ast.TypeName(sorted.Elem(0).Value) || ast.TypeName(sorted.Elem(0).Value) == "boolean" {
			n.fail(e.Loc(), "comparing all values of a field needs values that are all strings, or all numbers: %v", e)
			return nil
		}
	}
	x := ast.NewExpr([]*ast.Term{ast.NewTerm(luceneOutside), field, ast.NewTerm(sorted)})
	x.Negated = true
	x.Location = e.Location
	return []*ast.Expr{x}
}

var luceneOutside = ast.Ref{ast.VarTerm("internal"), ast.InternedTerm("lucene_outside")}

// knownValues are the elements of a known array or set, if they all are values a
// field can hold.
func knownValues(v ast.Value) (ast.Set, bool) {
	vs := ast.NewSet()
	var elems []*ast.Term
	switch c := v.(type) {
	case *ast.Array:
		for i := range c.Len() {
			elems = append(elems, c.Elem(i))
		}
	case ast.Set:
		elems = c.Slice()
	default:
		return nil, false
	}
	for _, t := range elems {
		if !fieldValue(t.Value) {
			return nil, false
		}
		vs.Add(t)
	}
	return vs, true
}

// elementsOf recognises {v | v = input.d.f[k]}, the set of the values of a field,
// and returns the field.
func elementsOf(t *ast.Term) (*ast.Term, bool) {
	sc, ok := t.Value.(*ast.SetComprehension)
	if !ok || len(sc.Body) != 1 {
		return nil, false
	}
	head, ok := sc.Term.Value.(ast.Var)
	e := sc.Body[0]
	if !ok || e.Negated || len(e.With) > 0 || !e.IsEquality() {
		return nil, false
	}
	for _, pair := range [2][2]*ast.Term{{e.Operand(0), e.Operand(1)}, {e.Operand(1), e.Operand(0)}} {
		if v, ok := pair[0].Value.(ast.Var); !ok || v != head {
			continue
		}
		if ref, ok := elementRef(pair[1]); ok && !ref[len(ref)-1].Value.(ast.Var).Equal(head) {
			return ast.NewTerm(ref[:len(ref)-1]), true
		}
	}
	return nil, false
}

// element recognises input.d.f[<var>] and returns the field. A variable used
// elsewhere ties the element to another (`f[i]` and `g[i]`), so that one is kept.
func (*normalizer) element(t *ast.Term, ctx ast.Body) (*ast.Term, bool) {
	ref, ok := elementRef(t)
	if !ok {
		return nil, false
	}
	v := ref[len(ref)-1].Value.(ast.Var)
	if !v.IsWildcard() {
		uses := 0
		ast.WalkVars(ctx, func(w ast.Var) bool {
			if w == v {
				uses++
			}
			return false
		})
		if uses != 1 {
			return nil, false
		}
	}
	return ast.NewTerm(ref[:len(ref)-1]), true
}

// elementRef reports whether t is input.<path>[<var>], and returns it.
func elementRef(t *ast.Term) (ast.Ref, bool) {
	ref, ok := t.Value.(ast.Ref)
	if !ok || len(ref) < 3 || !ref.HasPrefix(ast.InputRootRef) {
		return nil, false
	}
	if _, isVar := ref[len(ref)-1].Value.(ast.Var); !isVar {
		return nil, false
	}
	return ref, ref[:len(ref)-1].IsGround()
}

// fieldRef reports whether v names a field: a reference into input by plain keys.
func fieldRef(v ast.Value) bool {
	ref, ok := v.(ast.Ref)
	return ok && len(ref) >= 2 && ref.HasPrefix(ast.InputRootRef) && slices.IndexFunc(ref[1:], isNotString) < 0
}

func withOperand(e *ast.Expr, i int, t *ast.Term) *ast.Expr {
	x := e.Copy()
	x.Terms.([]*ast.Term)[i+1] = t
	return x
}

// eqExpr builds `a = b` in place of orig, keeping its negation and location.
func eqExpr(orig *ast.Expr, a, b *ast.Term) *ast.Expr {
	x := ast.Equality.Expr(a, b)
	x.Negated = orig.Negated
	x.Location = orig.Location
	return x
}

// fieldValue reports whether v can be compared with the values of a field: a
// Lucene field has no null value.
func fieldValue(v ast.Value) bool {
	_, null := v.(ast.Null)
	return ast.IsScalar(v) && !null
}

func comparison(e *ast.Expr) bool {
	switch e.OperatorTerm().Value.(ast.Ref).String() {
	case ast.Equality.Name, ast.NotEqual.Name, ast.Member.Name,
		ast.GreaterThan.Name, ast.GreaterThanEq.Name, ast.LessThan.Name, ast.LessThanEq.Name,
		ast.StartsWith.Name, ast.EndsWith.Name, ast.Contains.Name:
		return true
	}
	return false
}

func emptyArray(v ast.Value) bool {
	a, ok := v.(*ast.Array)
	return ok && a.Len() == 0
}
