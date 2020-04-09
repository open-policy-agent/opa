// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package copypropagation

import "github.com/open-policy-agent/opa/ast"

type rankFunc func(*unionFindRoot, *unionFindRoot) (*unionFindRoot, *unionFindRoot)

type unionFind struct {
	roots   map[ast.Var]*unionFindRoot
	parents map[ast.Var]ast.Var
	rank    rankFunc
}
func newUnionFind(rank rankFunc) *unionFind {
	return &unionFind{
		roots:   map[ast.Var]*unionFindRoot{},
		parents: map[ast.Var]ast.Var{},
		rank:    rank,
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

		r1, r2 = uf.rank(r1, r2)

		uf.parents[r2.key] = r1.key
		delete(uf.roots, r2.key)

		// Sets can have at most one constant value associated with them. When
		// unioning, we must preserve this invariant. If a set has two constants,
		// there will be no way to prove the query.
		if r1.constant != nil && r2.constant != nil && !r1.constant.Equal(r2.constant) {
			return nil, false
		} else if r1.constant == nil {
			r1.constant = r2.constant
		}
	}

	return r1, true
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
