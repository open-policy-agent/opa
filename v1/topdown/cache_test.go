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
