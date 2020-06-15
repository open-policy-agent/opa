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

func TestRefContainsNonScalar(t *testing.T) {
	cases := []struct {
		note     string
		ref      ast.Ref
		expected bool
	}{
		{
			note:     "empty ref",
			ref:      ast.MustParseRef("data"),
			expected: false,
		},
		{
			note:     "string ref",
			ref:      ast.MustParseRef(`data.foo["bar"]`),
			expected: false,
		},
		{
			note:     "number ref",
			ref:      ast.MustParseRef("data.foo[1]"),
			expected: false,
		},
		{
			note:     "set ref",
			ref:      ast.MustParseRef("data.foo[{0}]"),
			expected: true,
		},
		{
			note:     "array ref",
			ref:      ast.MustParseRef(`data.foo[["bar"]]`),
			expected: true,
		},
		{
			note:     "object ref",
			ref:      ast.MustParseRef(`data.foo[{"bar": 1}]`),
			expected: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			actual := refContainsNonScalar(tc.ref)

			if actual != tc.expected {
				t.Errorf("Expected %t for %s", tc.expected, tc.ref)
			}
		})
	}

}

func TestContainsNestedRefOrCall(t *testing.T) {

	tests := []struct {
		note  string
		input string
		want  bool
	}{
		{
			note:  "single term - negative",
			input: "p[x]",
			want:  false,
		},
		{
			note:  "single term - positive ref",
			input: "p[q[x]]",
			want:  true,
		},
		{
			note:  "single term - positive composite ref",
			input: "[q[x]]",
			want:  true,
		},
		{
			note:  "single term - positive composite call",
			input: "[f(x)]",
			want:  true,
		},
		{
			note:  "call expr - negative",
			input: "f(x)",
			want:  false,
		},
		{
			note:  "call expr - positive ref",
			input: "f(p[x])",
			want:  true,
		},
		{
			note:  "call expr - positive call",
			input: "f(g(x))",
			want:  true,
		},
		{
			note:  "call expr - positive composite",
			input: "f([g(x)])",
			want:  true,
		},
		{
			note:  "unify expr - negative",
			input: "p[x] = q[y]",
			want:  false,
		},
		{
			note:  "unify expr - positive ref",
			input: "p[x] = q[r[y]]",
			want:  true,
		},
		{
			note:  "unify expr - positive call",
			input: "f(x) = g(h(y))",
			want:  true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			vis := newNestedCheckVisitor()
			expr := ast.MustParseExpr(tc.input)
			result := containsNestedRefOrCall(vis, expr)
			if result != tc.want {
				t.Fatal("Expected", tc.want, "but got", result)
			}
		})
	}
}
