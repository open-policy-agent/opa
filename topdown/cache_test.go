// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"testing"

	"github.com/open-policy-agent/opa/ast"
)

func TestVirtualCacheInvalidate(t *testing.T) {
	cache := newVirtualCache()
	cache.Push()
	cache.Put(ast.MustParseRef("data.x.p"), ast.BooleanTerm(true))
	cache.Pop()
	result := cache.Get(ast.MustParseRef("data.x.p"))
	if result != nil {
		t.Fatal("Expected nil result but got:", result)
	}
}

func TestBaseCacheGetExactMatch(t *testing.T) {
	cache := newBaseCache()
	cache.Put(ast.MustParseRef("data.x.foo"), ast.StringTerm("bar").Value)
	result, _ := cache.Get(ast.MustParseRef("data.x.foo"))
	if result != ast.StringTerm("bar").Value {
		t.Fatalf("Expected bar but got %v", result)
	}
}

func TestBaseCacheGetPartialMatchSingleLevel(t *testing.T) {
	cache := newBaseCache()
	cache.Put(ast.MustParseRef("data.x.foo"), ast.StringTerm("bar").Value)
	result, _ := cache.Get(ast.MustParseRef("data.x"))

	expected := map[string]interface{}{}
	expected["foo"] = "bar"
	exp, err := ast.InterfaceToValue(expected)

	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}

	if exp.Compare(result) != 0 {
		t.Fatalf("Expected %v but got %v", expected, result)
	}
}

func TestBaseCacheGetPartialMatchMultiLevel(t *testing.T) {
	cache := newBaseCache()
	cache.Put(ast.MustParseRef("data.x.foo1.bar1"), ast.StringTerm("baz1").Value)
	cache.Put(ast.MustParseRef("data.x.foo2.bar2.bar21"), ast.ArrayTerm(ast.NumberTerm("1"), ast.NumberTerm("2"), ast.NumberTerm("3")).Value)

	result, _ := cache.Get(ast.MustParseRef("data.x"))

	expected := map[string]interface{}{}
	expected["foo1"] = map[string]interface{}{"bar1": "baz1"}
	expected["foo2"] = map[string]interface{}{"bar2": map[string]interface{}{"bar21": []interface{}{1, 2, 3}}}

	exp, err := ast.InterfaceToValue(expected)

	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}

	if exp.Compare(result) != 0 {
		t.Fatalf("Expected %v but got %v", expected, result)
	}
}

func TestBaseCacheRemoveNoChildren(t *testing.T) {
	cache := newBaseCache()
	ref := "data.x.foo1.bar1"
	cache.Put(ast.MustParseRef(ref), ast.StringTerm("baz1").Value)

	result, found := cache.Get(ast.MustParseRef(ref))
	if !found {
		t.Fatalf("Ref %v should be present in cache", ref)
	}

	if result != ast.StringTerm("baz1").Value {
		t.Fatalf("Expected baz1 but got %v", result)
	}

	cache.Remove(ast.MustParseRef(ref))

	result, found = cache.Get(ast.MustParseRef(ref))
	if found {
		t.Fatalf("Ref %v should not be present in cache", ref)
	}

	if result != nil {
		t.Fatalf("Expected nil but got %v", result)
	}
}

func TestBaseCacheRemoveWithChildren(t *testing.T) {
	cache := newBaseCache()
	ref := "data.x.foo1.bar1"
	cache.Put(ast.MustParseRef(ref), ast.StringTerm("baz1").Value)
	cache.Put(ast.MustParseRef("data.x.foo2"), ast.StringTerm("baz2").Value)

	result, found := cache.Get(ast.MustParseRef(ref))
	if !found {
		t.Fatalf("Ref %v should be present in cache", ref)
	}

	if result != ast.StringTerm("baz1").Value {
		t.Fatalf("Expected baz1 but got %v", result)
	}

	cache.Remove(ast.MustParseRef(ref))

	result, found = cache.Get(ast.MustParseRef(ref))
	if found {
		t.Fatalf("Ref %v should not be present in cache", ref)
	}

	if result != nil {
		t.Fatalf("Expected nil but got %v", result)
	}

	result, _ = cache.Get(ast.MustParseRef("data.x"))
	expected := map[string]interface{}{}
	expected["foo2"] = "baz2"

	exp, err := ast.InterfaceToValue(expected)

	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}

	if exp.Compare(result) != 0 {
		t.Fatalf("Expected %v but got %v", expected, result)
	}
}

func TestBaseCacheRemoveWithChildrenMulti(t *testing.T) {
	cache := newBaseCache()
	cache.Put(ast.MustParseRef("data.x.foo1.bar1.baz1"), ast.StringTerm("blah1").Value)
	cache.Put(ast.MustParseRef("data.x.foo1.bar1.baz2"), ast.StringTerm("blah2").Value)
	cache.Put(ast.MustParseRef("data.x.foo2"), ast.StringTerm("baz2").Value)

	cache.Remove(ast.MustParseRef("data.x.foo1.bar1.baz1"))

	result, found := cache.Get(ast.MustParseRef("data.x.foo1.bar1.baz1"))
	if found {
		t.Fatalf("Ref %v should not be present in cache", "data.x.foo1.bar1.baz1")
	}

	if result != nil {
		t.Fatalf("Expected nil but got %v", result)
	}

	result, _ = cache.Get(ast.MustParseRef("data.x"))
	expected := map[string]interface{}{}
	expected["foo1"] = map[string]interface{}{"bar1": map[string]interface{}{"baz2": "blah2"}}
	expected["foo2"] = "baz2"

	exp, err := ast.InterfaceToValue(expected)

	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}

	if exp.Compare(result) != 0 {
		t.Fatalf("Expected %v but got %v", expected, result)
	}
}
