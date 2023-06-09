// Copyright 2023 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"encoding/json"
	"testing"

	"github.com/open-policy-agent/opa/types"
)

func TestInsertIntoObject(t *testing.T) {
	tests := []struct {
		note     string
		obj      *types.Object
		path     Ref
		tpe      types.Type
		expected types.Type
	}{
		{
			note: "adding to empty object",
			obj:  types.NewObject(nil, nil),
			path: Ref{NewTerm(String("a"))},
			tpe:  types.S,
			expected: types.NewObject(
				[]*types.StaticProperty{types.NewStaticProperty("a", types.S)},
				nil),
		},
		{
			note: "empty path",
			obj: types.NewObject(
				[]*types.StaticProperty{types.NewStaticProperty("a", types.S)},
				nil),
			path: nil,
			tpe:  types.S,
			expected: types.NewObject(
				[]*types.StaticProperty{types.NewStaticProperty("a", types.S)},
				nil),
		},
		{
			note: "adding to populated object",
			obj: types.NewObject(
				[]*types.StaticProperty{types.NewStaticProperty("a", types.S)},
				nil),
			path: Ref{NewTerm(String("b"))},
			tpe:  types.S,
			expected: types.NewObject(
				[]*types.StaticProperty{
					types.NewStaticProperty("a", types.S),
					types.NewStaticProperty("b", types.S),
				},
				nil),
		},
		{
			note: "number key",
			obj: types.NewObject(
				[]*types.StaticProperty{types.NewStaticProperty("a", types.S)},
				nil),
			path: Ref{NewTerm(Number("2"))},
			tpe:  types.S,
			expected: types.NewObject(
				[]*types.StaticProperty{
					types.NewStaticProperty("a", types.S),
					types.NewStaticProperty(json.Number("2"), types.S),
				},
				nil),
		},
		{
			note: "same path, same type",
			obj: types.NewObject(
				[]*types.StaticProperty{types.NewStaticProperty("a", types.S)},
				nil),
			path: Ref{NewTerm(String("a"))},
			tpe:  types.S,
			expected: types.NewObject(
				[]*types.StaticProperty{types.NewStaticProperty("a", types.S)},
				nil),
		},
		{
			note: "same path, different type",
			obj: types.NewObject(
				[]*types.StaticProperty{types.NewStaticProperty("a", types.S)},
				nil),
			path: Ref{NewTerm(String("a"))},
			tpe:  types.B,
			expected: types.NewObject(
				[]*types.StaticProperty{types.NewStaticProperty("a", types.Or(types.S, types.B))},
				nil),
		},
		{
			note: "same path, any type inserted",
			obj: types.NewObject(
				[]*types.StaticProperty{types.NewStaticProperty("a", types.S)},
				nil),
			path: Ref{NewTerm(String("a"))},
			tpe:  types.A,
			expected: types.NewObject(
				[]*types.StaticProperty{types.NewStaticProperty("a", types.A)},
				nil),
		},
		{
			note: "same path, any type present",
			obj: types.NewObject(
				[]*types.StaticProperty{types.NewStaticProperty("a", types.A)},
				nil),
			path: Ref{NewTerm(String("a"))},
			tpe:  types.S,
			expected: types.NewObject(
				[]*types.StaticProperty{types.NewStaticProperty("a", types.A)},
				nil),
		},
		{
			note: "long path",
			obj: types.NewObject(
				[]*types.StaticProperty{types.NewStaticProperty("a", types.S)},
				nil),
			path: Ref{NewTerm(String("b")), NewTerm(String("c")), NewTerm(String("d"))},
			tpe:  types.S,
			expected: types.NewObject(
				[]*types.StaticProperty{
					types.NewStaticProperty("a", types.S),
					types.NewStaticProperty("b", types.NewObject([]*types.StaticProperty{
						types.NewStaticProperty("c", types.NewObject([]*types.StaticProperty{
							types.NewStaticProperty("d", types.S),
						}, nil)),
					}, nil)),
				},
				nil),
		},
		{
			note: "long path, full match",
			obj: types.NewObject(
				[]*types.StaticProperty{
					types.NewStaticProperty("a", types.S),
					types.NewStaticProperty("b", types.NewObject([]*types.StaticProperty{
						types.NewStaticProperty("c", types.NewObject([]*types.StaticProperty{
							types.NewStaticProperty("d", types.S),
						}, nil)),
					}, nil)),
				},
				nil),
			path: Ref{NewTerm(String("b")), NewTerm(String("c")), NewTerm(String("d"))},
			tpe:  types.S,
			expected: types.NewObject(
				[]*types.StaticProperty{
					types.NewStaticProperty("a", types.S),
					types.NewStaticProperty("b", types.NewObject([]*types.StaticProperty{
						types.NewStaticProperty("c", types.NewObject([]*types.StaticProperty{
							types.NewStaticProperty("d", types.S),
						}, nil)),
					}, nil)),
				},
				nil),
		},
		{
			note: "long path, full match, different type",
			obj: types.NewObject(
				[]*types.StaticProperty{
					types.NewStaticProperty("a", types.S),
					types.NewStaticProperty("b", types.NewObject([]*types.StaticProperty{
						types.NewStaticProperty("c", types.NewObject([]*types.StaticProperty{
							types.NewStaticProperty("d", types.S),
						}, nil)),
					}, nil)),
				},
				nil),
			path: Ref{NewTerm(String("b")), NewTerm(String("c")), NewTerm(String("d"))},
			tpe:  types.B,
			expected: types.NewObject(
				[]*types.StaticProperty{
					types.NewStaticProperty("a", types.S),
					types.NewStaticProperty("b", types.NewObject([]*types.StaticProperty{
						types.NewStaticProperty("c", types.NewObject([]*types.StaticProperty{
							types.NewStaticProperty("d", types.Or(types.S, types.B)),
						}, nil)),
					}, nil)),
				},
				nil),
		},
		{
			note: "long path, partial match",
			obj: types.NewObject(
				[]*types.StaticProperty{
					types.NewStaticProperty("a", types.S),
					types.NewStaticProperty("b", types.NewObject([]*types.StaticProperty{
						types.NewStaticProperty("c", types.NewObject([]*types.StaticProperty{
							types.NewStaticProperty("d", types.S),
						}, nil)),
					}, nil)),
				},
				nil),
			path: Ref{NewTerm(String("b")), NewTerm(String("x")), NewTerm(String("d"))},
			tpe:  types.S,
			expected: types.NewObject(
				[]*types.StaticProperty{
					types.NewStaticProperty("a", types.S),
					types.NewStaticProperty("b", types.NewObject([]*types.StaticProperty{
						types.NewStaticProperty("c", types.NewObject([]*types.StaticProperty{
							types.NewStaticProperty("d", types.S),
						}, nil)),
						types.NewStaticProperty("x", types.NewObject([]*types.StaticProperty{
							types.NewStaticProperty("d", types.S),
						}, nil)),
					}, nil)),
				},
				nil),
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			result, err := insertIntoObject(tc.obj, tc.path, tc.tpe)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if types.Compare(result, tc.expected) != 0 {
				t.Fatalf("Expected %v but got %v", tc.expected, result)
			}
		})
	}
}

func TestTypeTreeInsert(t *testing.T) {
	n := newTypeTree()

	abcRef := Ref{NewTerm(String("a")), NewTerm(String("b")), NewTerm(String("c"))}
	n.Put(abcRef, types.B)
	actual := n.Get(abcRef)
	if types.Compare(actual, types.B) != 0 {
		t.Fatalf("Expected %v but got %v", types.B, actual)
	}

	abdeRef := Ref{NewTerm(String("a")), NewTerm(String("b")), NewTerm(String("d")), NewTerm(String("e"))}
	n.Put(abdeRef, types.N)
	actual = n.Get(abdeRef)
	if types.Compare(actual, types.N) != 0 {
		t.Fatalf("Expected %v but got %v", types.N, actual)
	}

	// existing "child" leafs should be added to new intermediate object leaf

	abRef := Ref{NewTerm(String("a")), NewTerm(String("b"))}
	n.Insert(abRef, types.NewObject(nil, &types.DynamicProperty{Key: types.N, Value: types.S}))

	actual = n.Get(abRef)
	expected := types.NewObject(
		[]*types.StaticProperty{
			types.NewStaticProperty("c", types.B),
			types.NewStaticProperty("d", types.NewObject(
				[]*types.StaticProperty{types.NewStaticProperty("e", types.N)},
				nil)),
		},
		&types.DynamicProperty{Key: types.N, Value: types.S},
	)
	if types.Compare(actual, expected) != 0 {
		t.Fatalf("Expected %v but got %v", expected, actual)
	}

	// new "child" leafs should be added to new intermediate object leaf

	abfRef := Ref{NewTerm(String("a")), NewTerm(String("b")), NewTerm(String("f"))}
	n.Insert(abfRef, types.S)

	actual = n.Get(abfRef)
	if types.Compare(actual, types.S) != 0 {
		t.Fatalf("Expected %v but got %v", types.S, actual)
	}

	actual = n.Get(abRef)
	expected = types.NewObject(
		[]*types.StaticProperty{
			types.NewStaticProperty("c", types.B),
			types.NewStaticProperty("f", types.S),
			types.NewStaticProperty("d", types.NewObject(
				[]*types.StaticProperty{types.NewStaticProperty("e", types.N)},
				nil)),
		},
		&types.DynamicProperty{Key: types.N, Value: types.S},
	)
	if types.Compare(actual, expected) != 0 {
		t.Fatalf("Expected %v but got %v", expected, actual)
	}
}
