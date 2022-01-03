// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package storage

import (
	"math"
	"reflect"
	"testing"

	"fmt"

	"github.com/open-policy-agent/opa/ast"
)

func TestNewPathForString(t *testing.T) {

	tests := []struct {
		input  string
		result Path
		ok     bool
	}{
		{"", nil, false},
		{"foo", nil, false},
		{"/", Path{}, true},
		{"/", nil, true},
		{"/foo", Path{"foo"}, true},
		{"/foo/bar", Path{"foo", "bar"}, true},
	}

	for _, tc := range tests {
		result, ok := ParsePath(tc.input)
		if (tc.ok != ok) || !tc.result.Equal(result) {
			t.Errorf("For %v wanted (%v, %v) but got (%v, %v)", tc.input, tc.result, tc.ok, result, ok)
		}
	}
}

func TestNewPathForRef(t *testing.T) {
	tests := []struct {
		input  ast.Ref
		result Path
		err    error
	}{
		{ast.Ref{}, nil, fmt.Errorf("empty reference (indicates error in caller)")},
		{ast.MustParseRef("data.foo[x]"), nil, fmt.Errorf("unresolved reference (indicates error in caller): data.foo[x]")},
		{ast.MustParseRef("data.foo[true]"), nil, &Error{
			Code:    NotFoundErr,
			Message: fmt.Sprintf("%v: does not exist", ast.MustParseRef("data.foo[true]")),
		}},
		{ast.MustParseRef("data.foo[[1, 2]]"), nil, fmt.Errorf("composites cannot be base document keys: %v", ast.MustParseRef("data.foo[[1, 2]]"))},
		{ast.MustParseRef("data.foo[{1, 2}]"), nil, fmt.Errorf("composites cannot be base document keys: %v", ast.MustParseRef("data.foo[{1, 2}]"))},
		{ast.MustParseRef(`data.foo[{"foo": 2}]`), nil, fmt.Errorf("composites cannot be base document keys: %v", ast.MustParseRef(`data.foo[{"foo": 2}]`))},

		{ast.MustParseRef("data"), Path{}, nil},
		{ast.MustParseRef("data.foo"), Path{"foo"}, nil},
		{ast.MustParseRef("data.foo[1]"), Path{"foo", "1"}, nil},
		{ast.MustParseRef("data.foo.bar"), Path{"foo", "bar"}, nil},
	}

	for _, tc := range tests {
		result, err := NewPathForRef(tc.input)
		if tc.err != nil && !reflect.DeepEqual(tc.err, err) {
			t.Errorf("For %v expected %v but got %v", tc.input, tc.err, err)
		} else if !result.Equal(tc.result) {
			t.Errorf("For %v expected %v but got %v", tc.input, tc.result, result)
		}
	}
}

func TestNewPathForStringEscaped(t *testing.T) {

	tests := []struct {
		input  string
		result Path
		ok     bool
	}{
		{
			input:  "/foo/bar", // no escaping
			result: Path{"foo", "bar"},
			ok:     true,
		},
		{
			input:  "/foo%2Fbar/baz", // single escape
			result: Path{"foo/bar", "baz"},
			ok:     true,
		},
		{
			input:  "/foo%2F%2Fbar/baz", // double escape
			result: Path{"foo//bar", "baz"},
			ok:     true,
		},
	}

	for _, tc := range tests {
		result, ok := ParsePathEscaped(tc.input)
		if (tc.ok != ok) || !tc.result.Equal(result) {
			t.Errorf("For %v wanted (%v, %v) but got (%v, %v)", tc.input, tc.result, tc.ok, result, ok)
		}
	}
}

func TestPathCompare(t *testing.T) {
	tests := []struct {
		a      Path
		b      Path
		result int
	}{
		{Path{}, Path{}, 0},
		{Path{}, Path{"x"}, -1},
		{Path{"x"}, Path{}, 1},
		{Path{"x"}, Path{"x"}, 0},
		{Path{"x"}, Path{"y"}, -1},
		{Path{"x"}, Path{"w"}, 1},
		{Path{"x"}, Path{"wz"}, 1},
		{Path{"x"}, Path{"xx"}, -1},
		{Path{"xx"}, Path{"x"}, 1},
		{Path{"xx"}, Path{"xx"}, 0},
		{Path{"xy"}, Path{"xx"}, 1},
	}
	for _, tc := range tests {
		result := tc.a.Compare(tc.b)
		if result != tc.result {
			t.Errorf("For %v.Compare(%v) expected %v but got %v", tc.a, tc.b, tc.result, result)
		}
	}
}

func TestPathEqual(t *testing.T) {
	tests := []struct {
		a      Path
		b      Path
		result bool
	}{
		{Path{}, Path{}, true},
		{Path{}, Path{"foo"}, false},
		{Path{"foo"}, Path{}, false},
		{Path{"foo", "bar"}, Path{"foo"}, false},
		{Path{"foo", "bar"}, Path{"foo", "bar"}, true},
	}
	for _, tc := range tests {
		result := tc.a.Equal(tc.b)
		if result != tc.result {
			t.Errorf("For %v.HasPrefix(%v) expected %v but got %v", tc.a, tc.b, tc.result, result)
		}
	}
}

func TestPathHasPrefix(t *testing.T) {
	tests := []struct {
		a      Path
		b      Path
		result bool
	}{
		{Path{}, Path{}, true},
		{Path{}, Path{"foo"}, false},
		{Path{"foo"}, Path{}, true},
		{Path{"foo"}, Path{"bar"}, false},
		{Path{"bar"}, Path{"foo"}, false},
		{Path{"foo", "bar"}, Path{"foo"}, true},
		{Path{"foo", "bar"}, Path{"foo", "bar"}, true},
		{Path{"foo", "bar"}, Path{"foo", "bar", "baz"}, false},
		{Path{"foo", "bar", "baz"}, Path{}, true},
	}
	for _, tc := range tests {
		result := tc.a.HasPrefix(tc.b)
		if result != tc.result {
			t.Errorf("For %v.HasPrefix(%v) expected %v but got %v", tc.a, tc.b, tc.result, result)
		}
	}
}

func TestPathRef(t *testing.T) {
	tests := []struct {
		path string
		head string
		ref  string
	}{
		{"/", "data", "data"},
		{"/foo/bar", "data", "data.foo.bar"},
		{"/foo/bar/3", "data", "data.foo.bar[3]"},
		{fmt.Sprintf("/foo/bar/%d", math.MaxInt64), "data", fmt.Sprintf("data.foo.bar[%d]", math.MaxInt64)},
	}
	for _, tc := range tests {
		path := MustParsePath(tc.path)
		head := ast.VarTerm(tc.head)
		ref := ast.MustParseRef(tc.ref)
		result := path.Ref(head)
		if !result.Equal(ref) {
			t.Errorf("Expected %v but got %v", ref, result)
		}
	}
}
