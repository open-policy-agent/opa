// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package types

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/open-policy-agent/opa/v1/util"
)

var dynamicPropertyAnyAny = NewDynamicProperty(A, A)

func TestAnySorted(t *testing.T) {
	if Compare(NewAny(S, N)[0], N) != 0 {
		t.Fatal("expected any type to be sorted")
	}
}

func TestAnyMerge(t *testing.T) {
	x := NewAny(S, B)

	if Compare(x.Merge(N)[1], N) != 0 {
		t.Fatal("expected number to be inserted into middle")
	}

	if Compare(x.Merge(Nl)[0], Nl) != 0 {
		t.Fatal("expected null to be inserted at front")
	}

	if Compare(x.Merge(NewArray(nil, A))[2], NewArray(nil, A)) != 0 {
		t.Fatal("expected array to be inserted at back")
	}
}

func TestAnyUnion(t *testing.T) {
	x := NewAny(Nl, N)
	y := NewAny(S, B)
	z := x.Union(y)
	exp := []Type{Nl, B, N, S}
	if len(z) != len(exp) {
		t.Fatalf("expected %v elements in result of union", len(exp))
	}
	for i := range z {
		if Compare(z[i], exp[i]) != 0 {
			t.Fatal("expected", exp[i], "but got", z[i])
		}
	}
}

func TestStrings(t *testing.T) {
	tpe := NewObject([]*StaticProperty{
		{"foo", Nl},
		{"bar", B},
		{"baz", N},
		{"qux", S},
		{"corge", NewArray(
			[]Type{
				A,
				NewAny(Nl, S),
				NewSet(S),
			}, S,
		)},
		{"nil", nil},
	}, NewDynamicProperty(S, N))

	expected := `object<bar: boolean, baz: number, corge: array<any, any<null, string>, set[string]>[string], foo: null, nil: ???, qux: string>[string: number]`

	if tpe.String() != expected {
		t.Fatalf("Expected %v but got: %v", expected, tpe)
	}

	ftpe := NewFunction([]Type{S, S}, N)
	expected = "(string, string) => number"

	if ftpe.String() != expected {
		t.Fatalf("Expected %v but got: %v", expected, ftpe)
	}

	ftpe = NewVariadicFunction([]Type{N}, S, nil)
	expected = "(number, string...) => ???"

	if ftpe.String() != expected {
		t.Fatal("expected", expected, "but got:", ftpe)
	}
}

func TestCompare(t *testing.T) {
	tests := []struct {
		a   Type
		b   Type
		cmp int
	}{
		{Nl, Nl, 0},
		{Nl, B, -1},
		{B, Nl, 1},
		{B, B, 0},
		{B, N, -1},
		{N, N, 0},
		{N, S, -1},
		{S, S, 0},
		{S, NewArray(NewAny(), nil), -1},
		{NewArray(NewAny(), nil), NewArray(NewAny(), A), -1},
		{NewArray(NewAny(), A), NewArray(NewAny(), A), 0},
		{NewArray(NewAny(), A), NewArray(NewAny(), S), 1},
		{NewArray(NewAny(), A), NewArray(NewAny(), nil), 1},
		{NewArray([]Type{S}, nil), NewArray([]Type{N}, nil), 1},
		{NewObject(nil, nil), NewObject(nil, dynamicPropertyAnyAny), -1},
		{NewObject(nil, dynamicPropertyAnyAny), NewObject(nil, nil), 1},
		{NewObject(nil, dynamicPropertyAnyAny), NewObject(nil, dynamicPropertyAnyAny), 0},
		{NewObject(nil, NewDynamicProperty(S, NewAny(S, Nl))), NewObject(nil, dynamicPropertyAnyAny), -1},
		{NewSet(Nl), NewSet(NewAny()), -1},
		{
			NewObject(
				[]*StaticProperty{{"foo", S}},
				nil),
			NewObject(
				[]*StaticProperty{{"foo", S}, {"bar", N}},
				nil),
			1,
		},
		{
			NewObject(
				[]*StaticProperty{{"foo", S}, {"bar", N}},
				nil),
			NewObject(
				[]*StaticProperty{{"foo", S}},
				nil),
			-1,
		},
		{
			NewObject(
				[]*StaticProperty{{"foo", S}},
				nil),
			NewObject(
				[]*StaticProperty{{"foo", Nl}},
				nil),
			1,
		},
		{
			NewObject(
				[]*StaticProperty{{"foo", S}},
				nil),
			NewObject(
				[]*StaticProperty{{"foo", S}, {"foo-2", N}},
				nil),
			-1,
		},
		{
			NewObject(
				[]*StaticProperty{{"foo", S}, {"foo-2", N}},
				nil),
			NewObject(
				[]*StaticProperty{{"foo", S}},
				nil),
			1,
		},
		{NewFunction(nil, nil), A, 1},
		{NewFunction([]Type{B}, N), NewFunction([]Type{S}, N), -1},
		{NewFunction(nil, S), NewFunction(nil, N), 1},
		{NewFunction(nil, S), NewFunction([]Type{N}, S), -1},
		{NewFunction([]Type{S}, N), NewFunction(nil, S), 1},
		{NewFunction([]Type{S}, N), NewFunction([]Type{S}, N), 0},
	}

	for _, tc := range tests {
		result := Compare(tc.a, tc.b)
		if result != tc.cmp {
			t.Fatalf("For Compare(%v, %v) expected %v but got: %v", tc.a, tc.b, tc.cmp, result)
		}
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		a        Type
		b        Type
		expected bool
	}{
		{S, S, true},
		{A, S, true},
		{NewAny(N, B), S, false},
		{N, S, false},
	}

	for _, tc := range tests {
		if Contains(tc.a, tc.b) != tc.expected {
			t.Fatalf("Expected Contains(%v, %v) == %v", tc.a, tc.b, tc.expected)
		}
	}
}

func TestOr(t *testing.T) {
	tests := []struct {
		a        Type
		b        Type
		expected Type
	}{
		{nil, S, S},
		{S, nil, S},
		{Nl, Nl, Nl},
		{S, N, NewAny(N, S)},
		{A, Nl, A},
		{Nl, A, A},
		{Nl, NewAny(S, N), NewAny(S, N, Nl)},
		{A, A, A},
		{NewAny(Nl, N), A, A},
		{NewAny(N, S), NewAny(Nl, B), NewAny(Nl, B, S, N)},
		{NewAny(Nl, N), Nl, NewAny(Nl, N)},
		{NewFunction([]Type{S}, B), NewFunction([]Type{N}, B), NewFunction([]Type{NewAny(S, N)}, B)},
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
		k        any
		expected Type
	}{
		{"static", NewArray([]Type{S}, nil), json.Number("0"), S},
		{"dynamic", NewArray(nil, S), json.Number("100"), S},
		{"out of range", NewArray([]Type{S, N, B}, nil), json.Number("4"), nil},
		{"out of range negative", NewArray([]Type{S, N, B}, nil), json.Number("-4"), nil},
		{"negative", NewArray([]Type{S, N, B}, nil), json.Number("-2"), nil},
		{"non int", NewArray([]Type{S, N, B}, nil), json.Number("1.5"), nil},
		{"non int-2", NewArray([]Type{S, N, B}, nil), 1, nil},
		{"static", NewObject([]*StaticProperty{NewStaticProperty("hello", S)}, nil), "hello", S},
		{"dynamic", NewObject([]*StaticProperty{NewStaticProperty("hello", S)}, NewDynamicProperty(S, N)), "goodbye", N},
		{"dynamic, different key types", NewObject([]*StaticProperty{NewStaticProperty("hello", S)}, NewDynamicProperty(N, N)), json.Number("2"), N},
		{"dynamic, different key types", NewObject([]*StaticProperty{NewStaticProperty("hello", S)}, NewDynamicProperty(N, N)), "hello", S},
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
		{"scalar-4", Nl, "1", nil},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
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
		{"object", NewObject(nil, NewDynamicProperty(S, S)), S},
		{"set", NewSet(N), N},
		{"any", NewAny(NewArray(nil, nil), NewSet(S)), NewAny(S, N)},
		{"any", NewAny(NewArray(nil, nil), S), N},
		{"superset", A, A},
		{"scalar-1", N, nil},
		{"scalar-2", S, nil},
		{"scalar-3", B, nil},
		{"scalar-4", Nl, nil},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
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
		{"object dynamic", NewObject([]*StaticProperty{NewStaticProperty("a", S), NewStaticProperty("b", N)}, NewDynamicProperty(A, B)), NewAny(S, N, B)},
		{"set", NewSet(N), N},
		{"superset", A, A},
		{"any", NewAny(NewArray(nil, N), S), N},
		{"scalar-1", N, nil},
		{"scalar-2", S, nil},
		{"scalar-3", B, nil},
		{"scalar-4", Nl, nil},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			result := Values(tc.tpe)
			if Compare(result, tc.expected) != 0 {
				t.Fatalf("Expected Keys(%v) to be %v but got: %v", tc.tpe, tc.expected, result)
			}
		})
	}
}

func TestTypeOf(t *testing.T) {
	tpe := TypeOf(map[any]any{
		"foo": []any{
			json.Number("1"), true, nil, "hello",
		},
	})

	exp := NewObject([]*StaticProperty{
		NewStaticProperty("foo", NewArray(
			[]Type{
				N, B, Nl, S,
			}, nil,
		)),
	}, nil)

	if Compare(exp, tpe) != 0 {
		t.Fatalf("Expected %v but got: %v", exp, tpe)
	}
}

func TestTypeOfMapOfString(t *testing.T) {
	tpe := TypeOf(map[string]any{
		"foo": "bar",
		"baz": "qux",
	})

	exp := NewObject([]*StaticProperty{
		NewStaticProperty("foo", S),
		NewStaticProperty("baz", S),
	}, nil)

	if Compare(exp, tpe) != 0 {
		t.Fatalf("Expected %v but got: %v", exp, tpe)
	}
}

func TestNil(t *testing.T) {

	tpe := NewObject([]*StaticProperty{
		NewStaticProperty("foo", NewArray(
			[]Type{
				N, B, Nl, S, NewSet(nil),
			}, nil,
		)),
	}, nil)

	if !Nil(tpe) {
		t.Fatalf("Expected %v type to be unknown", tpe)
	}

}

func TestMarshalJSON(t *testing.T) {

	tpe := NewAny(
		NewObject(
			[]*StaticProperty{
				{"foo", N},
				{"func", NewFunction([]Type{S}, N)},
			},
			NewDynamicProperty(S, NewArray([]Type{NewSet(B)}, N)),
		),
	)

	bs, err := json.Marshal(tpe)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expected := util.MustUnmarshalJSON([]byte(`
	{
		"type": "any",
		"of": [
			{
				"type": "object",
				"static": [
					{
						"key": "foo",
						"value": {"type": "number"}
					},
					{
						"key": "func",
						"value": {
							"type": "function",
							"args": [
								{
									"type": "string"
								}
							],
							"result": {
								"type": "number"
							}
						}
					}
				],
				"dynamic": {
					"key": {"type": "string"},
					"value": {
						"type": "array",
						"static": [
							{
								"type": "set",
								"of": {"type": "boolean"}
							}
						],
						"dynamic": {"type": "number"}
					}
				}
			}
		]
	}
	`))

	result := util.MustUnmarshalJSON(bs)

	if !reflect.DeepEqual(expected, result) {
		t.Fatalf("Expected:\n\n%s\n\nGot:\n\n%s", util.MustMarshalJSON(expected), util.MustMarshalJSON(result))
	}

}

func TestRoundtripJSON(t *testing.T) {
	tpe := NewFunction([]Type{
		NewArray([]Type{S, Nl}, N),
		NewObject(
			[]*StaticProperty{
				NewStaticProperty("foo", B),
			},
			NewDynamicProperty(S, NewSet(N))),
		NewObject(
			[]*StaticProperty{
				NewStaticProperty("bar", N),
			},
			nil,
		),
	}, NewAny(S, N))

	bs, err := json.Marshal(tpe)
	if err != nil {
		t.Fatal(err)
	}

	result, err := Unmarshal(bs)
	if err != nil {
		t.Fatal(err)
	}

	if Compare(result, tpe) != 0 {
		t.Fatalf("Got: %v\n\nExpected: %v", result, tpe)
	}
}

func TestRoundtripJSONVariadicFunction(t *testing.T) {
	tpe := NewVariadicFunction([]Type{S}, N, nil)
	bs, err := json.Marshal(tpe)
	if err != nil {
		t.Fatal(err)
	}

	result, err := Unmarshal(bs)
	if err != nil {
		t.Fatal(err)
	}

	if Compare(result, tpe) != 0 {
		t.Fatalf("Got: %v\n\nExpected: %v", result, tpe)
	}
}
