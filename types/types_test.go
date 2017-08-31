// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package types

import (
	"encoding/json"
	"testing"

	"github.com/open-policy-agent/opa/util/test"
)

func TestStrings(t *testing.T) {

	tpe := NewObject([]*StaticProperty{
		{"foo", NewNull()},
		{"bar", NewBoolean()},
		{"baz", NewNumber()},
		{"qux", NewString()},
		{"corge", NewArray(
			[]Type{
				NewAny(),
				NewAny(NewNull(), NewString()),
				NewSet(NewString()),
			}, NewString(),
		)},
		{"nil", nil},
	}, NewNumber())

	expected := `object<bar: boolean, baz: number, corge: array<any, any<null, string>, set[string]>[string], foo: null, nil: ???, qux: string>[number]`

	if tpe.String() != expected {
		t.Fatalf("Expected %v but got: %v", expected, tpe)
	}
}

func TestCompare(t *testing.T) {

	tests := []struct {
		a   Type
		b   Type
		cmp int
	}{
		{NewNull(), NewNull(), 0},
		{NewNull(), NewBoolean(), -1},
		{NewBoolean(), NewNull(), 1},
		{NewBoolean(), NewBoolean(), 0},
		{NewBoolean(), NewNumber(), -1},
		{NewNumber(), NewNumber(), 0},
		{NewNumber(), NewString(), -1},
		{NewString(), NewString(), 0},
		{NewString(), NewArray(NewAny(), nil), -1},
		{NewArray(NewAny(), nil), NewArray(NewAny(), NewAny()), -1},
		{NewArray(NewAny(), NewAny()), NewArray(NewAny(), NewAny()), 0},
		{NewArray(NewAny(), NewAny()), NewArray(NewAny(), NewString()), 1},
		{NewArray(NewAny(), NewAny()), NewArray(NewAny(), nil), 1},
		{NewArray([]Type{NewString()}, nil), NewArray([]Type{NewNumber()}, nil), 1},
		{NewObject(nil, nil), NewObject(nil, NewAny()), -1},
		{NewObject(nil, NewAny()), NewObject(nil, nil), 1},
		{NewObject(nil, NewAny()), NewObject(nil, NewAny()), 0},
		{NewObject(nil, NewAny()), NewObject(nil, NewAny(NewString(), NewNull())), -1},
		{NewSet(NewNull()), NewSet(NewAny()), -1},
		{
			NewObject(
				[]*StaticProperty{{"foo", NewString()}},
				nil),
			NewObject(
				[]*StaticProperty{{"foo", NewString()}, {"bar", NewNumber()}},
				nil),
			1,
		},
		{
			NewObject(
				[]*StaticProperty{{"foo", NewString()}, {"bar", NewNumber()}},
				nil),
			NewObject(
				[]*StaticProperty{{"foo", NewString()}},
				nil),
			-1,
		},
		{
			NewObject(
				[]*StaticProperty{{"foo", NewString()}},
				nil),
			NewObject(
				[]*StaticProperty{{"foo", NewNull()}},
				nil),
			1,
		},
		{
			NewObject(
				[]*StaticProperty{{"foo", NewString()}},
				nil),
			NewObject(
				[]*StaticProperty{{"foo", NewString()}, {"foo-2", NewNumber()}},
				nil),
			-1,
		},
		{
			NewObject(
				[]*StaticProperty{{"foo", NewString()}, {"foo-2", NewNumber()}},
				nil),
			NewObject(
				[]*StaticProperty{{"foo", NewString()}},
				nil),
			1,
		},
	}

	for _, tc := range tests {
		result := Compare(tc.a, tc.b)
		if result != tc.cmp {
			t.Fatalf("For Compare(%v, %v) expected %v but got: %v", tc.a, tc.b, tc.cmp, result)
		}
	}
}

func TestOr(t *testing.T) {
	tests := []struct {
		a        Type
		b        Type
		expected Type
	}{
		{nil, NewString(), NewString()},
		{NewString(), nil, NewString()},
		{NewNull(), NewNull(), NewNull()},
		{NewString(), NewNumber(), NewAny(NewNumber(), NewString())},
		{NewAny(), NewNull(), NewAny()},
		{NewNull(), NewAny(), NewAny()},
		{NewNull(), NewAny(NewString(), NewNumber()), NewAny(NewString(), NewNumber(), NewNull())},
		{NewAny(), NewAny(), NewAny()},
		{NewAny(NewNull(), NewNumber()), NewAny(), NewAny()},
		{NewAny(NewNumber(), NewString()), NewAny(NewNull(), NewBoolean()), NewAny(NewNull(), NewBoolean(), NewString(), NewNumber())},
		{NewAny(NewNull(), NewNumber()), NewNull(), NewAny(NewNull(), NewNumber())},
	}

	for _, tc := range tests {
		c := Or(tc.a, tc.b)
		if Compare(c, tc.expected) != 0 {
			t.Fatalf("Expected Or(%v, %v) to be %v but got: %v", tc.a, tc.b, tc.expected, c)
		}
	}

}

func TestSelect(t *testing.T) {

	tests := []struct {
		note     string
		a        Type
		k        interface{}
		expected Type
	}{
		{"static", NewArray([]Type{S}, nil), json.Number("0"), S},
		{"dynamic", NewArray(nil, S), json.Number("100"), S},
		{"out of range", NewArray([]Type{S, N, B}, nil), json.Number("4"), nil},
		{"non int", NewArray([]Type{S, N, B}, nil), json.Number("1.5"), nil},
		{"non int-2", NewArray([]Type{S, N, B}, nil), 1, nil},
		{"static", NewObject([]*StaticProperty{NewStaticProperty("hello", S)}, nil), "hello", S},
		{"dynamic", NewObject([]*StaticProperty{NewStaticProperty("hello", S)}, N), "goodbye", N},
		{"non exist", NewObject([]*StaticProperty{NewStaticProperty("hello", S)}, nil), "deadbeef", nil},
		{"non string", NewObject([]*StaticProperty{NewStaticProperty(json.Number("1"), S), NewStaticProperty(json.Number("2"), N)}, nil), json.Number("2"), N},
		{"member of", NewSet(N), json.Number("2"), N},
		{"non exist", NewSet(N), "foo", nil},
		{"superset", A, A, A},
		{"union", NewAny(NewArray(nil, N), NewArray(nil, S)), json.Number("10"), NewAny(N, S)},
		{"union set", NewSet(NewAny(S, N)), json.Number("1"), N},
		{"scalar", N, "1", nil},
		{"scalar-2", S, "1", nil},
		{"scalar-3", B, "1", nil},
		{"scalar-4", NewNull(), "1", nil},
	}

	for _, tc := range tests {
		test.Subtest(t, tc.note, func(t *testing.T) {
			result := Select(tc.a, tc.k)
			if Compare(result, tc.expected) != 0 {
				t.Fatalf("Expected Select(%v, %v) to be %v but got: %v", tc.a, tc.k, tc.expected, result)
			}
		})
	}
}

func TestKeys(t *testing.T) {
	tests := []struct {
		note     string
		tpe      Type
		expected Type
	}{
		{"array", NewArray(nil, nil), N},
		{"object", NewObject(nil, nil), S},
		{"set", NewSet(N), N},
		{"any", NewAny(NewArray(nil, nil), NewSet(S)), NewAny(S, N)},
		{"any", NewAny(NewArray(nil, nil), S), N},
		{"superset", A, A},
		{"scalar-1", N, nil},
		{"scalar-2", S, nil},
		{"scalar-3", B, nil},
		{"scalar-4", NewNull(), nil},
	}

	for _, tc := range tests {
		test.Subtest(t, tc.note, func(t *testing.T) {
			result := Keys(tc.tpe)
			if Compare(result, tc.expected) != 0 {
				t.Fatalf("Expected Keys(%v) to be %v but got: %v", tc.tpe, tc.expected, result)
			}
		})
	}
}

func TestValues(t *testing.T) {
	tests := []struct {
		note     string
		tpe      Type
		expected Type
	}{
		{"array", NewArray([]Type{N}, nil), N},
		{"array dynamic", NewArray([]Type{N, S}, B), NewAny(S, N, B)},
		{"object", NewObject([]*StaticProperty{NewStaticProperty("a", S), NewStaticProperty("b", N)}, nil), NewAny(S, N)},
		{"object dynamic", NewObject([]*StaticProperty{NewStaticProperty("a", S), NewStaticProperty("b", N)}, B), NewAny(S, N, B)},
		{"set", NewSet(N), N},
		{"superset", A, A},
		{"any", NewAny(NewArray(nil, N), S), N},
		{"scalar-1", N, nil},
		{"scalar-2", S, nil},
		{"scalar-3", B, nil},
		{"scalar-4", NewNull(), nil},
	}

	for _, tc := range tests {
		test.Subtest(t, tc.note, func(t *testing.T) {
			result := Values(tc.tpe)
			if Compare(result, tc.expected) != 0 {
				t.Fatalf("Expected Keys(%v) to be %v but got: %v", tc.tpe, tc.expected, result)
			}
		})
	}
}

func TestTypeOf(t *testing.T) {
	tpe := TypeOf(map[interface{}]interface{}{
		"foo": []interface{}{
			json.Number("1"), true, nil, "hello",
		},
	})

	exp := NewObject([]*StaticProperty{
		NewStaticProperty("foo", NewArray(
			[]Type{
				N, B, NewNull(), S,
			}, nil,
		)),
	}, nil)

	if Compare(exp, tpe) != 0 {
		t.Fatalf("Expected %v but got: %v", exp, tpe)
	}
}

func TestNil(t *testing.T) {

	tpe := NewObject([]*StaticProperty{
		NewStaticProperty("foo", NewArray(
			[]Type{
				N, B, NewNull(), S, NewSet(nil),
			}, nil,
		)),
	}, nil)

	if !Nil(tpe) {
		t.Fatalf("Expected %v type to be unknown", tpe)
	}

}
