// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
)

func TestVirtualCacheCompositeKey(t *testing.T) {
	t.Parallel()

	cache := NewVirtualCache()
	ref := ast.MustParseRef("data.x.y[[1]].z")
	cache.Put(ref, ast.BooleanTerm(true))
	result, _ := cache.Get(ref)
	if !result.Equal(ast.BooleanTerm(true)) {
		t.Fatalf("Expected true but got %v", result)
	}
}

func TestVirtualCacheInvalidate(t *testing.T) {
	t.Parallel()

	cache := NewVirtualCache()
	cache.Push()
	cache.Put(ast.MustParseRef("data.x.p"), ast.BooleanTerm(true))
	cache.Pop()
	result, _ := cache.Get(ast.MustParseRef("data.x.p"))
	if result != nil {
		t.Fatal("Expected nil result but got:", result)
	}
}

func TestSetAndRetriveUndefined(t *testing.T) {
	t.Parallel()

	cache := NewVirtualCache()
	cache.Put(ast.MustParseRef("data.foo.bar"), nil)
	result, undefined := cache.Get(ast.MustParseRef("data.foo.bar"))
	if result != nil {
		t.Fatal("Expected nil result but got:", result)
	}
	if !undefined {
		t.Fatal("Expected 'undefined' flag to be false got true")
	}
}

func TestBaseCacheGetExactMatch(t *testing.T) {
	t.Parallel()

	cache := newBaseCache()
	cache.Put(ast.MustParseRef("data.x.foo"), ast.StringTerm("bar").Value)
	result := cache.Get(ast.MustParseRef("data.x.foo"))
	if result != ast.StringTerm("bar").Value {
		t.Fatalf("Expected bar but got %v", result)
	}
}

func TestVirtualCacheEmptyFrame(t *testing.T) {
	t.Parallel()

	cache := NewVirtualCache()
	ref := ast.MustParseRef("data.x.p")
	cache.Push()

	if result, undefined := cache.Get(ref); result != nil || undefined {
		t.Fatalf("expected a miss, got %v, %v", result, undefined)
	}
	if keys := cache.Keys(); len(keys) != 0 {
		t.Fatalf("expected no keys, got %v", keys)
	}

	cache.Put(ref, ast.BooleanTerm(true))
	cache.Put(ast.MustParseRef("data.x.q"), ast.BooleanTerm(false))
	if keys := cache.Keys(); len(keys) != 2 {
		t.Fatalf("expected 2 keys, got %v", keys)
	}
}

func TestComprehensionCacheEmptyFrame(t *testing.T) {
	t.Parallel()

	term := ast.MustParseTerm("[x | x = 1]")
	cache := newComprehensionCache()
	cache.Push()

	if _, ok := cache.Elem(term); ok {
		t.Fatal("expected a miss on an empty frame")
	}

	cache.Set(term, newComprehensionCacheElem())
	if _, ok := cache.Elem(term); !ok {
		t.Fatal("expected a hit after Set")
	}

	cache.Pop()
	if _, ok := cache.Elem(term); ok {
		t.Fatal("expected the popped frame's entry to be gone")
	}
}

func TestFunctionMocksStackNoMocks(t *testing.T) {
	t.Parallel()

	s := newFunctionMocksStack()
	ref := ast.MustParseRef("data.f")

	s.PutPairs(nil)
	if _, ok := s.Get(ref); ok {
		t.Fatal("expected no mock")
	}

	s.PutPairs([][2]*ast.Term{{ast.NewTerm(ref), ast.IntNumberTerm(1)}})
	if _, ok := s.Get(ref); !ok {
		t.Fatal("expected a mock")
	}
	s.PopPairs()
	s.PopPairs()
	if _, ok := s.Get(ref); ok {
		t.Fatal("expected no mock after popping")
	}
}
