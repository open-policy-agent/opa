// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"testing"

	"github.com/open-policy-agent/opa/ast"
)

func TestQueryIDFactory(t *testing.T) {
	f := &queryIDFactory{}
	for i := 0; i < 10; i++ {
		if n := f.Next(); n != uint64(i) {
			t.Errorf("expected %d, got %d", i, n)
		}
	}
}

func TestMergeNonOverlappingKeys(t *testing.T) {
	realData := ast.MustParseTerm(`{"foo": "bar"}`).Value.(ast.Object)
	mockData := ast.MustParseTerm(`{"baz": "blah"}`).Value.(ast.Object)

	result, ok := merge(mockData, realData)
	if !ok {
		t.Fatal("Unexpected error occurred")
	}

	expected := ast.MustParseTerm(`{"foo": "bar", "baz": "blah"}`).Value

	if expected.Compare(result) != 0 {
		t.Fatalf("Expected %v but got %v", expected, result)
	}
}

func TestMergeOverlappingKeys(t *testing.T) {
	realData := ast.MustParseTerm(`{"foo": "bar"}`).Value.(ast.Object)
	mockData := ast.MustParseTerm(`{"foo": "blah"}`).Value.(ast.Object)

	result, ok := merge(mockData, realData)
	if !ok {
		t.Fatal("Unexpected error occurred")
	}

	expected := ast.MustParseTerm(`{"foo": "blah"}`).Value
	if expected.Compare(result) != 0 {
		t.Fatalf("Expected %v but got %v", expected, result)
	}

	realData = ast.MustParseTerm(`{"foo": {"foo1": {"foo11": [1,2,3], "foo12": "hello"}}, "bar": "baz"}`).Value.(ast.Object)
	mockData = ast.MustParseTerm(`{"foo": {"foo1": [1,2,3], "foo12": "world", "foo13": 123}, "baz": "bar"}`).Value.(ast.Object)

	result, ok = merge(mockData, realData)
	if !ok {
		t.Fatal("Unexpected error occurred")
	}

	expected = ast.MustParseTerm(`{"foo": {"foo1": [1,2,3], "foo12": "world", "foo13": 123}, "bar": "baz", "baz": "bar"}`).Value
	if expected.Compare(result) != 0 {
		t.Fatalf("Expected %v but got %v", expected, result)
	}

}

func TestMergeError(t *testing.T) {
	realData := ast.MustParseTerm(`{"foo": "bar"}`).Value.(ast.Object)
	mockData := ast.StringTerm("baz").Value

	_, ok := merge(mockData, realData)
	if ok {
		t.Fatal("Expected error")
	}
}
