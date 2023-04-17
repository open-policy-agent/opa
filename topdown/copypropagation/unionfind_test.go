// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package copypropagation

import (
	"reflect"
	"testing"

	"github.com/open-policy-agent/opa/ast"
)

func TestUnionFindRootValue(t *testing.T) {
	tests := []struct {
		name     string
		root     unionFindRoot
		expected ast.Value
	}{
		{
			name:     "var only",
			root:     unionFindRoot{key: ast.Var("foo")},
			expected: ast.Var("foo"),
		},
		{
			name:     "const only",
			root:     unionFindRoot{constant: ast.StringTerm("foo")},
			expected: ast.String("foo"),
		},
		{
			name:     "const and var",
			root:     unionFindRoot{key: ast.Var("foo"), constant: ast.StringTerm("bar")},
			expected: ast.String("bar"),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := &unionFindRoot{
				key:      tc.root.key,
				constant: tc.root.constant,
			}
			if got := r.Value(); tc.expected.Compare(got) != 0 {
				t.Errorf("Value() = %v, expected %v", got, tc.expected)
			}
		})
	}
}

func TestUnionFindMakeSet(t *testing.T) {

	uf := newUnionFind(nil)

	tests := []struct {
		name    string
		v       ast.Value
		result  *unionFindRoot
		parents map[ast.Value]ast.Value
		roots   map[ast.Value]*unionFindRoot
	}{
		{
			name:   "from empty",
			v:      ast.Var("a"),
			result: &unionFindRoot{key: ast.Var("a")},
		},
		{
			name:   "add another var",
			v:      ast.Var("b"),
			result: &unionFindRoot{key: ast.Var("b")},
		},
		{
			name:   "add existing",
			v:      ast.Var("b"),
			result: &unionFindRoot{key: ast.Var("b")},
		},
		{
			name:   "add ref",
			v:      ast.Ref{ast.StringTerm("foo")},
			result: &unionFindRoot{key: ast.Ref{ast.StringTerm("foo")}},
		},
		{
			name:   "add ref existing",
			v:      ast.Ref{ast.StringTerm("foo")},
			result: &unionFindRoot{key: ast.Ref{ast.StringTerm("foo")}},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actual := uf.MakeSet(tc.v)
			if !reflect.DeepEqual(actual, tc.result) {
				t.Errorf("MakeSet(%v) = %v, expected %v", tc.v, actual, tc.result)
			}
		})
	}
}

func TestUnionFindFindEmptyUF(t *testing.T) {
	uf := newUnionFind(noopUnionFindRank)
	actual, found := uf.Find(ast.Var("a"))
	if found || actual != nil {
		t.Error("Expected Find() to return (nil, false)")
	}
}

func TestUnionFindFindIsParent(t *testing.T) {
	uf := newUnionFind(noopUnionFindRank)

	uf.MakeSet(ast.Var("a")) // "a" will have a parent "a"

	actual, found := uf.Find(ast.Var("a"))

	expected := newUnionFindRoot(ast.Var("a"))
	if !found || actual.Value().Compare(expected.Value()) != 0 {
		t.Errorf("Expected Find() to return (true, %+v)", expected)
	}
}

func TestUnionFindFindParent(t *testing.T) {
	fooBarRef := ast.Ref{ast.StringTerm("foo"), ast.StringTerm("bar"), ast.VarTerm("x")}
	call := ast.Call{ast.RefTerm(ast.VarTerm("gt")), ast.NumberTerm("1"), ast.VarTerm("x")}

	uf := newUnionFind(noopUnionFindRank)
	uf.Merge(ast.Var("a"), ast.Var("b"))
	uf.Merge(ast.Var("b"), ast.Var("c"))
	uf.Merge(ast.Var("c"), fooBarRef)
	uf.Merge(fooBarRef, ast.Var("d"))
	uf.Merge(ast.Var("d"), call)
	uf.Merge(call, ast.Var("e"))

	actual, found := uf.Find(ast.Var("e"))

	expected := newUnionFindRoot(ast.Var("a"))
	if !found || actual.Value().Compare(expected.Value()) != 0 {
		t.Errorf("Expected Find() to return (true, %+v)", expected)
	}
}

func TestUnionFindMerge(t *testing.T) {
	uf := newUnionFind(noopUnionFindRank)

	tests := []struct {
		name    string
		a       ast.Value
		b       ast.Value
		result  *unionFindRoot
		parents map[ast.Value]ast.Value
		roots   map[ast.Value]*unionFindRoot
	}{
		{
			name:   "empty uf",
			a:      ast.Var("a"),
			b:      ast.Var("b"),
			result: newUnionFindRoot(ast.Var("a")),
		},
		{
			name:   "same values",
			a:      ast.Var("a"),
			b:      ast.Var("a"),
			result: newUnionFindRoot(ast.Var("a")),
		},
		{
			name:   "same values higher rank result",
			a:      ast.Var("b"),
			b:      ast.Var("b"),
			result: newUnionFindRoot(ast.Var("a")),
		},
		{
			name:   "transitive",
			a:      ast.Var("b"),
			b:      ast.Var("c"),
			result: newUnionFindRoot(ast.Var("a")),
		},
		{
			name:   "new roots",
			a:      ast.Var("d"),
			b:      ast.Var("e"),
			result: newUnionFindRoot(ast.Var("d")),
		},
		{
			name:   "combine roots",
			a:      ast.Var("a"),
			b:      ast.Var("e"),
			result: newUnionFindRoot(ast.Var("a")),
		},
		{
			name:   "new ref roots",
			a:      ast.Ref{ast.StringTerm("foo"), ast.StringTerm("bar")},
			b:      ast.Var("x"),
			result: newUnionFindRoot(ast.Ref{ast.StringTerm("foo"), ast.StringTerm("bar")}),
		},
		{
			name:   "combine ref roots",
			a:      ast.Var("b"),
			b:      ast.Ref{ast.StringTerm("foo"), ast.StringTerm("bar")},
			result: newUnionFindRoot(ast.Var("a")),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actualRoot, canMerge := uf.Merge(tc.a, tc.b)
			if !reflect.DeepEqual(actualRoot, tc.result) || !canMerge {
				t.Errorf("Merge(%v, %v) got = (%v, %v), expected (%v, true)", tc.a, tc.b, actualRoot, canMerge, tc.result)
			}
		})
	}
}

var noopUnionFindRank = func(a *unionFindRoot, b *unionFindRoot) (*unionFindRoot, *unionFindRoot) {
	return a, b
}
