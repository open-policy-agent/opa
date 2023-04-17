// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package dependencies

import (
	"fmt"
	"sort"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/util"
)

// All returns the list of data ast.Refs that the given AST element depends on.
func All(x interface{}) (resolved []ast.Ref, err error) {
	var rawResolved []ast.Ref
	switch x := x.(type) {
	case *ast.Module, *ast.Package, *ast.Import, *ast.Rule, *ast.Head, ast.Body, *ast.Expr, *ast.With, *ast.Term, ast.Ref, ast.Object, *ast.Array, ast.Set, *ast.ArrayComprehension:
	default:
		return nil, fmt.Errorf("not an ast element: %v", x)
	}

	visitor := ast.NewGenericVisitor(func(x interface{}) bool {
		switch x := x.(type) {
		case *ast.Package, *ast.Import:
			return true
		case *ast.Module, *ast.Head, *ast.Expr, *ast.With, *ast.Term, ast.Object, *ast.Array, *ast.Set, *ast.ArrayComprehension:
		case *ast.Rule:
			rawResolved = append(rawResolved, ruleDeps(x)...)
			return true
		case ast.Body:
			vars := ast.NewVarVisitor()
			vars.Walk(x)

			arr := ast.NewArray()
			for v := range vars.Vars() {
				if v.IsWildcard() {
					continue
				}
				arr = arr.Append(ast.NewTerm(v))
			}

			// The analysis will discard variables that are not used in
			// direct comparisons or in the output. Since lone Bodies are
			// often queries, we want all the variables to be in the output.
			r := &ast.Rule{
				Head: &ast.Head{Name: ast.Var("_"), Value: ast.NewTerm(arr)},
				Body: x,
			}
			rawResolved = append(rawResolved, ruleDeps(r)...)
			return true
		case ast.Ref:
			rawResolved = append(rawResolved, x)
		}
		return false
	})
	visitor.Walk(x)
	if len(rawResolved) == 0 {
		return nil, nil
	}

	return dedup(rawResolved), nil
}

// Minimal returns the list of data ast.Refs that the given AST element depends on.
// If an AST element depends on a ast.Ref that is a prefix of another dependency, the
// ast.Ref that is the prefix of the other will be the only one in the returned list.
//
// As an example, if an element depends on data.x and data.x.y, only data.x will
// be in the returned list.
func Minimal(x interface{}) (resolved []ast.Ref, err error) {
	rawResolved, err := All(x)
	if err != nil {
		return nil, err
	}

	if len(rawResolved) == 0 {
		return nil, nil
	}

	return filter(rawResolved, func(a, b ast.Ref) bool {
		return b.HasPrefix(a)
	}), nil
}

// Base returns the list of base data documents that the given AST element depends on.
//
// The returned refs are always constant and are truncated at any point where they become
// dynamic. That is, a ref like data.a.b[x] will be truncated to data.a.b.
func Base(compiler *ast.Compiler, x interface{}) ([]ast.Ref, error) {
	baseRefs, err := base(compiler, x)
	if err != nil {
		return nil, err
	}

	return dedup(baseRefs), nil
}

func base(compiler *ast.Compiler, x interface{}) ([]ast.Ref, error) {
	refs, err := Minimal(x)
	if err != nil {
		return nil, err
	}

	var baseRefs []ast.Ref
	for _, r := range refs {
		r = r.ConstantPrefix()
		if rules := compiler.GetRules(r); len(rules) > 0 {
			for _, rule := range rules {
				bases, err := base(compiler, rule)
				if err != nil {
					panic("not reached")
				}

				baseRefs = append(baseRefs, bases...)
			}
		} else {
			baseRefs = append(baseRefs, r)
		}
	}

	return baseRefs, nil
}

// Virtual returns the list of virtual data documents that the given AST element depends
// on.
//
// The returned refs are always constant and are truncated at any point where they become
// dynamic. That is, a ref like data.a.b[x] will be truncated to data.a.b.
func Virtual(compiler *ast.Compiler, x interface{}) ([]ast.Ref, error) {
	virtualRefs, err := virtual(compiler, x)
	if err != nil {
		return nil, err
	}

	return dedup(virtualRefs), nil
}

func virtual(compiler *ast.Compiler, x interface{}) ([]ast.Ref, error) {
	refs, err := Minimal(x)
	if err != nil {
		return nil, err
	}

	var virtualRefs []ast.Ref
	for _, r := range refs {
		r = r.ConstantPrefix()
		if rules := compiler.GetRules(r); len(rules) > 0 {
			for _, rule := range rules {
				virtuals, err := virtual(compiler, rule)
				if err != nil {
					panic("not reached")
				}

				virtualRefs = append(virtualRefs, rule.Path())
				virtualRefs = append(virtualRefs, virtuals...)
			}
		}
	}

	return virtualRefs, nil
}

func dedup(refs []ast.Ref) []ast.Ref {
	sort.Slice(refs, func(i, j int) bool {
		return refs[i].Compare(refs[j]) < 0
	})

	return filter(refs, func(a, b ast.Ref) bool {
		return a.Compare(b) == 0
	})
}

// filter removes all items from the list that cause pref to return true. It is
// called on adjacent pairs of elements, and the one passed as the second argument
// to pref is considered the current one being examined. The first argument will
// be the element immediately preceding it.
func filter(rs []ast.Ref, pred func(ast.Ref, ast.Ref) bool) (filtered []ast.Ref) {
	if len(rs) == 0 {
		return nil
	}

	last := rs[0]
	filtered = append(filtered, last)
	for i := 1; i < len(rs); i++ {
		cur := rs[i]
		if pred(last, cur) {
			continue
		}

		filtered = append(filtered, cur)
		last = cur
	}

	return filtered
}

// FIXME(tsandall): this logic should be revisited as it seems overly
// complicated. It should be possible to compute all dependencies in two
// passes:
//
//  1. perform syntactic unification on vars
//  2. gather all refs rooted at data after plugging the head with substitution
//     from (1)
func ruleDeps(rule *ast.Rule) (resolved []ast.Ref) {
	vars, others := extractEq(rule.Body)
	joined := joinVarRefs(vars)

	headVars := rule.Head.Vars()
	headRefs, others := resolveOthers(others, headVars, joined)

	resolveRef := func(r ast.Ref) bool {
		resolved = append(resolved, expandRef(r, joined)...)
		return false
	}

	varVisitor := ast.NewVarVisitor().WithParams(ast.VarVisitorParams{SkipRefHead: true})
	// Clean up whatever refs are remaining among the other expressions.
	for _, expr := range others {
		ast.WalkRefs(expr, resolveRef)
		varVisitor.Walk(expr)
	}

	// If a reference ending in a header variable is a prefix of an already
	// resolved reference, skip it and simply walk the nodes below it.
	visitor := &skipVisitor{fn: resolveRef}
	for _, r := range headRefs {
		if !containsPrefix(resolved, r) {
			resolved = append(resolved, r.Copy())
		}
		visitor.skipped = false
		ast.NewGenericVisitor(visitor.Visit).Walk(r)
	}

	usedVars := varVisitor.Vars()

	// Vars included in refs must be counted as used.
	ast.WalkRefs(rule.Body, func(r ast.Ref) bool {
		for i := 1; i < len(r); i++ {
			if v, ok := r[i].Value.(ast.Var); ok {
				usedVars.Add(v)
			}
		}
		return false
	})

	resolveRemainingVars(joined, visitor, usedVars, headVars)
	return resolved
}

// Extract the equality expressions from each rule, they contain
// the potential split references. In order to be considered for
// joining, an equality must have a variable on one side and a
// reference on the other. Any other construct is thrown into
// the others list to be resolved later.
func extractEq(exprs ast.Body) (vars map[ast.Var][]ast.Ref, others []*ast.Expr) {
	vars = map[ast.Var][]ast.Ref{}
	for v := range exprs.Vars(ast.VarVisitorParams{}) {
		vars[v] = nil
	}

	for _, expr := range exprs {
		if !expr.IsEquality() {
			others = append(others, expr)
			continue
		}

		terms := expr.Terms.([]*ast.Term)
		left, right := terms[1], terms[2]
		if l, ok := left.Value.(ast.Var); ok {
			if r, ok := right.Value.(ast.Ref); ok {
				vars[l] = append(vars[l], r)
				continue
			}
		} else if r, ok := right.Value.(ast.Var); ok {
			if l, ok := left.Value.(ast.Ref); ok {
				vars[r] = append(vars[r], l)
				continue
			}
		}

		others = append(others, expr)
	}
	return vars, others
}

func expandRef(r ast.Ref, vars map[ast.Var]*util.HashMap) []ast.Ref {
	head, rest := r[0], r[1:]
	if ast.RootDocumentNames.Contains(head) {
		return []ast.Ref{r}
	}

	h := head.Value.(ast.Var)
	rs, ok := vars[h]
	if !ok {
		return nil
	}

	var expanded []ast.Ref
	rs.Iter(func(a, _ util.T) bool {
		ref := a.(ast.Ref)
		expanded = append(expanded, append(ref.Copy(), rest...))
		return false
	})
	return expanded
}

func joinVarRefs(vars map[ast.Var][]ast.Ref) map[ast.Var]*util.HashMap {
	joined := map[ast.Var]*util.HashMap{}
	for v := range vars {
		joined[v] = util.NewHashMap(refEq, refHash)
	}

	done := false
	for !done {
		done = true
		for v, rs := range vars {
			for _, r := range rs {
				head, rest := r[0], r[1:]
				if ast.RootDocumentNames.Contains(head) {
					if _, ok := joined[v].Get(r); !ok {
						joined[v].Put(r, struct{}{})
						done = false
					}
					continue
				}

				h, ok := head.Value.(ast.Var)
				if !ok {
					panic("not reached")
				}

				joined[h].Iter(func(a, _ util.T) bool {
					jr := a.(ast.Ref)
					join := append(jr.Copy(), rest...)
					if _, ok := joined[v].Get(join); !ok {
						joined[v].Put(join, struct{}{})
						done = false
					}
					return false
				})
			}
		}
	}

	return joined
}

func resolveOthers(others []*ast.Expr, headVars ast.VarSet, joined map[ast.Var]*util.HashMap) (headRefs []ast.Ref, leftover []*ast.Expr) {
	for _, expr := range others {
		if term, ok := expr.Terms.(*ast.Term); ok {
			if r, ok := term.Value.(ast.Ref); ok {
				end := r[len(r)-1]
				v, ok := end.Value.(ast.Var)
				if ok && headVars.Contains(v) {
					headRefs = append(headRefs, expandRef(r, joined)...)
					continue
				}
			}
		}

		leftover = append(leftover, expr)
	}

	return headRefs, leftover
}

func resolveRemainingVars(joined map[ast.Var]*util.HashMap, visitor *skipVisitor, usedVars ast.VarSet, headVars ast.VarSet) {
	for v, refs := range joined {
		skipped := false

		if headVars.Contains(v) || refs.Len() > 1 || usedVars.Contains(v) {
			skipped = true
		}

		refs.Iter(func(a, _ util.T) bool {
			visitor.skipped = skipped
			r := a.(ast.Ref)
			ast.NewGenericVisitor(visitor.Visit).Walk(r)
			return false
		})
	}
}

func containsPrefix(refs []ast.Ref, r ast.Ref) bool {
	for _, ref := range refs {
		if ref.HasPrefix(r) {
			return true
		}
	}
	return false
}

func refEq(a, b util.T) bool {
	ar, aok := a.(ast.Ref)
	br, bok := b.(ast.Ref)
	return aok && bok && ar.Equal(br)
}

func refHash(a util.T) int {
	return a.(ast.Ref).Hash()
}

type skipVisitor struct {
	fn      func(ast.Ref) bool
	skipped bool
}

func (sv *skipVisitor) Visit(v interface{}) bool {
	if sv.skipped {
		if r, ok := v.(ast.Ref); ok {
			return sv.fn(r)
		}
	}

	sv.skipped = true
	return false
}
