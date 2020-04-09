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
		v       ast.Var
		result  *unionFindRoot
		parents map[ast.Var]ast.Var
		roots   map[ast.Var]*unionFindRoot
	}{
		{
			name:   "from empty",
			v:      ast.Var("a"),
			result: &unionFindRoot{key: ast.Var("a")},
			parents: map[ast.Var]ast.Var{
				ast.Var("a"): ast.Var("a"),
			},
			roots: map[ast.Var]*unionFindRoot{
				ast.Var("a"): {key: ast.Var("a")},
			},
		},
		{
			name:   "add another var",
			v:      ast.Var("b"),
			result: &unionFindRoot{key: ast.Var("b")},
			parents: map[ast.Var]ast.Var{
				ast.Var("a"): ast.Var("a"),
				ast.Var("b"): ast.Var("b"),
			},
			roots: map[ast.Var]*unionFindRoot{
				ast.Var("a"): {key: ast.Var("a")},
				ast.Var("b"): {key: ast.Var("b")},
			},
		},
		{
			name:   "add existing",
			v:      ast.Var("b"),
			result: &unionFindRoot{key: ast.Var("b")},
			parents: map[ast.Var]ast.Var{
				ast.Var("a"): ast.Var("a"),
				ast.Var("b"): ast.Var("b"),
			},
			roots: map[ast.Var]*unionFindRoot{
				ast.Var("b"): {key: ast.Var("b")},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actual := uf.MakeSet(tc.v)
			if !reflect.DeepEqual(actual, tc.result) {
				t.Errorf("MakeSet(%v) = %v, expected %v", tc.v, actual, tc.result)
			}
			if !reflect.DeepEqual(tc.parents, uf.parents) {
				t.Errorf("uf.parents = %v, expected %v", uf.parents, tc.parents)
			}
			if !reflect.DeepEqual(tc.parents, uf.parents) {
				t.Errorf("uf.roots = %v, expected %v", uf.roots, tc.roots)
			}
		})
	}
}

func TestUnionFindFind(t *testing.T) {
	tests := []struct {
		name          string
		uf            *unionFind
		v             ast.Var
		expectedRoot  *unionFindRoot
		expectedFound bool
	}{
		{
			name:          "empty uf",
			uf:            newUnionFind(nil),
			v:             ast.Var("a"),
			expectedRoot:  nil,
			expectedFound: false,
		},
		{
			name: "is parent",
			uf: &unionFind{
				parents: map[ast.Var]ast.Var{
					ast.Var("a"): ast.Var("a"),
				},
				roots: map[ast.Var]*unionFindRoot{
					ast.Var("a"): newUnionFindRoot("a"),
				},
			},
			v:             ast.Var("a"),
			expectedRoot:  newUnionFindRoot("a"),
			expectedFound: true,
		},
		{
			name: "find parent",
			uf: &unionFind{
				parents: map[ast.Var]ast.Var{
					ast.Var("a"): ast.Var("b"),
					ast.Var("b"): ast.Var("c"),
					ast.Var("c"): ast.Var("d"),
					ast.Var("d"): ast.Var("d"),
				},
				roots: map[ast.Var]*unionFindRoot{
					ast.Var("d"): newUnionFindRoot("d"),
				},
			},
			v:             ast.Var("a"),
			expectedRoot:  newUnionFindRoot("d"),
			expectedFound: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {

			actualRoot, actualFound := tc.uf.Find(tc.v)
			if !reflect.DeepEqual(actualRoot, tc.expectedRoot) || actualFound != tc.expectedFound {
				t.Errorf("Find(%v) got = (%v, %v), expected (%v, %v)", tc.v, actualRoot, actualFound, tc.expectedRoot, tc.expectedFound)
			}
		})
	}
}

func TestUnionFindMerge(t *testing.T) {
	uf := newUnionFind(func(a *unionFindRoot, b *unionFindRoot) (*unionFindRoot, *unionFindRoot) {
		return a, b
	})

	tests := []struct {
		name    string
		a       ast.Var
		b       ast.Var
		result  *unionFindRoot
		parents map[ast.Var]ast.Var
		roots   map[ast.Var]*unionFindRoot
	}{
		{
			name:   "empty uf",
			a:      ast.Var("a"),
			b:      ast.Var("b"),
			result: newUnionFindRoot("a"),
			parents: map[ast.Var]ast.Var{
				ast.Var("a"): ast.Var("a"),
				ast.Var("b"): ast.Var("a"),
			},
			roots: map[ast.Var]*unionFindRoot{
				ast.Var("a"): {key: ast.Var("a")},
			},
		},
		{
			name:   "same values",
			a:      ast.Var("a"),
			b:      ast.Var("a"),
			result: newUnionFindRoot("a"),
			parents: map[ast.Var]ast.Var{
				ast.Var("a"): ast.Var("a"),
				ast.Var("b"): ast.Var("a"),
			},
			roots: map[ast.Var]*unionFindRoot{
				ast.Var("a"): {key: ast.Var("a")},
			},
		},
		{
			name:   "same values higher rank result",
			a:      ast.Var("b"),
			b:      ast.Var("b"),
			result: newUnionFindRoot("a"),
			parents: map[ast.Var]ast.Var{
				ast.Var("a"): ast.Var("a"),
				ast.Var("b"): ast.Var("a"),
			},
			roots: map[ast.Var]*unionFindRoot{
				ast.Var("a"): {key: ast.Var("a")},
			},
		},
		{
			name:   "transitive",
			a:      ast.Var("b"),
			b:      ast.Var("c"),
			result: newUnionFindRoot("a"),
			parents: map[ast.Var]ast.Var{
				ast.Var("a"): ast.Var("a"),
				ast.Var("b"): ast.Var("a"),
				ast.Var("c"): ast.Var("a"),
			},
			roots: map[ast.Var]*unionFindRoot{
				ast.Var("a"): {key: ast.Var("a")},
			},
		},
		{
			name:   "new roots",
			a:      ast.Var("d"),
			b:      ast.Var("e"),
			result: newUnionFindRoot("d"),
			parents: map[ast.Var]ast.Var{
				ast.Var("a"): ast.Var("a"),
				ast.Var("b"): ast.Var("a"),
				ast.Var("c"): ast.Var("a"),
				ast.Var("d"): ast.Var("d"),
				ast.Var("e"): ast.Var("d"),
			},
			roots: map[ast.Var]*unionFindRoot{
				ast.Var("a"): {key: ast.Var("a")},
				ast.Var("d"): {key: ast.Var("d")},
			},
		},
		{
			name:   "combine roots",
			a:      ast.Var("a"),
			b:      ast.Var("e"),
			result: newUnionFindRoot("a"),
			parents: map[ast.Var]ast.Var{
				ast.Var("a"): ast.Var("a"),
				ast.Var("b"): ast.Var("a"),
				ast.Var("c"): ast.Var("a"),
				ast.Var("d"): ast.Var("a"),
				ast.Var("e"): ast.Var("d"),
			},
			roots: map[ast.Var]*unionFindRoot{
				ast.Var("a"): {key: ast.Var("a")},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actualRoot, canMerge := uf.Merge(tc.a, tc.b)
			if !reflect.DeepEqual(actualRoot, tc.result) || !canMerge {
				t.Errorf("Merge(%v, %v) got = (%v, %v), expected (%v, true)", tc.a, tc.b, actualRoot, canMerge, tc.result)
			}
			if !reflect.DeepEqual(tc.parents, uf.parents) {
				t.Errorf("uf.parents = %v, expected %v", uf.parents, tc.parents)
			}
			if !reflect.DeepEqual(tc.parents, uf.parents) {
				t.Errorf("uf.roots = %v, expected %v", uf.roots, tc.roots)
			}
		})
	}
}
