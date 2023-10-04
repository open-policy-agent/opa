// Copyright 2023 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
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
				nil,
				types.NewDynamicProperty(types.S, types.S)),
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
				},
				types.NewDynamicProperty(types.S, types.S)),
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
				},
				types.NewDynamicProperty(types.N, types.S)),
		},
		{
			note: "other type value inserted",
			obj: types.NewObject(
				nil,
				types.NewDynamicProperty(types.S, types.S)),
			path: Ref{NewTerm(String("a"))},
			tpe:  types.B,
			expected: types.NewObject(
				nil,
				types.NewDynamicProperty(types.S, types.Any{types.B, types.S})),
		},
		{
			note: "any type value inserted",
			obj: types.NewObject(
				nil,
				types.NewDynamicProperty(types.S, types.S)),
			path: Ref{NewTerm(String("a"))},
			tpe:  types.A,
			expected: types.NewObject(
				nil,
				types.NewDynamicProperty(types.S, types.A)),
		},
		{
			note: "other type key inserted",
			obj: types.NewObject(
				nil,
				types.NewDynamicProperty(types.S, types.S)),
			path: Ref{NewTerm(Number("42"))},
			tpe:  types.S,
			expected: types.NewObject(
				nil,
				types.NewDynamicProperty(types.Any{types.N, types.S}, types.S)),
		},
		{
			note: "other type key and value inserted",
			obj: types.NewObject(
				nil,
				types.NewDynamicProperty(types.S, types.S)),
			path: Ref{NewTerm(Number("42"))},
			tpe:  types.B,
			expected: types.NewObject(
				nil,
				types.NewDynamicProperty(types.Any{types.N, types.S}, types.Any{types.B, types.S})),
		},
		{
			note: "any type value present, string inserted",
			obj: types.NewObject(
				nil,
				types.NewDynamicProperty(types.S, types.A)),
			path: Ref{NewTerm(String("a"))},
			tpe:  types.S,
			expected: types.NewObject(
				nil,
				types.NewDynamicProperty(types.S, types.A)),
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
				},
				types.NewDynamicProperty(types.S, // b
					types.NewObject(nil, types.NewDynamicProperty(types.S, // c
						types.NewObject(nil, types.NewDynamicProperty(types.S, types.S)))))), // d
		},
		{
			note: "long path, dynamic overlap with different key type",
			obj: types.NewObject(
				nil,
				types.NewDynamicProperty(types.N, types.S)),
			path: Ref{NewTerm(String("b")), NewTerm(String("c")), NewTerm(String("d"))},
			tpe:  types.S,
			expected: types.NewObject(
				nil,
				types.NewDynamicProperty(types.Any{types.N, types.S}, // b
					types.Any{types.S,
						types.NewObject(nil, types.NewDynamicProperty(types.S, // c
							types.NewObject(nil, types.NewDynamicProperty(types.S, types.S))))})), // d
		},
		{
			note: "long path, dynamic overlap with object",
			obj: types.NewObject(
				nil,
				types.NewDynamicProperty(types.S,
					types.NewObject(nil, types.NewDynamicProperty(types.S, types.N)))),
			path: Ref{NewTerm(String("b")), NewTerm(String("c")), NewTerm(String("d"))},
			tpe:  types.S,
			expected: types.NewObject(
				nil,
				types.NewDynamicProperty(types.S, // b
					types.Any{
						types.NewObject(nil, types.NewDynamicProperty(types.S, types.N)),
						types.NewObject(nil, types.NewDynamicProperty(types.S, // c
							types.NewObject(nil, types.NewDynamicProperty(types.S, types.S)))), // d
					})),
		},
		{
			note: "long path, dynamic overlap with object (2)",
			obj: types.NewObject(
				nil,
				types.NewDynamicProperty(types.S,
					types.NewObject(nil, types.NewDynamicProperty(types.S,
						types.NewObject(nil, types.NewDynamicProperty(types.S, types.N)))))),
			path: Ref{NewTerm(String("b")), NewTerm(String("c")), NewTerm(String("d"))},
			tpe:  types.S,
			expected: types.NewObject(
				nil,
				types.NewDynamicProperty(types.S,
					types.Any{ // Objects aren't merged, as that would become very complicated if they contain static components
						types.NewObject(nil, types.NewDynamicProperty(types.S,
							types.NewObject(nil, types.NewDynamicProperty(types.S, types.N)))),
						types.NewObject(nil, types.NewDynamicProperty(types.S,
							types.NewObject(nil, types.NewDynamicProperty(types.S, types.S)))),
					})),
		},
		{
			note: "long path, dynamic overlap with different value type",
			obj: types.NewObject(
				nil,
				types.NewDynamicProperty(types.S, types.S)),
			path: Ref{NewTerm(String("b")), NewTerm(String("c")), NewTerm(String("d"))},
			tpe:  types.S,
			expected: types.NewObject(
				nil,
				types.NewDynamicProperty(types.S, // b
					types.Any{types.S,
						types.NewObject(nil, types.NewDynamicProperty(types.S, // c
							types.NewObject(nil, types.NewDynamicProperty(types.S, types.S))))})), // d
		},
	}

	env := TypeEnv{}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			result, err := insertIntoObject(tc.obj, tc.path, tc.tpe, &env)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if types.Compare(result, tc.expected) != 0 {
				t.Fatalf("Expected %v but got %v", tc.expected, result)
			}
		})
	}
}

type pathAndType struct {
	path Ref
	tpe  types.Type
}

func TestTypeTreeNode_Insert(t *testing.T) {
	cases := []struct {
		note       string
		insertions []pathAndType
		expected   []pathAndType
	}{
		{
			note: "only primitives",
			insertions: []pathAndType{
				{
					path: MustParseRef("data.a.b.c"),
					tpe:  types.N,
				},
				{
					path: MustParseRef("data.a.b.c2"),
					tpe:  types.S,
				},
				{
					path: MustParseRef("data.a.b[42]"),
					tpe:  types.B,
				},
			},
			expected: []pathAndType{
				{
					path: MustParseRef("data.a.b.c"),
					tpe:  types.N,
				},
				{
					path: MustParseRef("data.a.b.c2"),
					tpe:  types.S,
				},
				{
					path: MustParseRef("data.a.b[42]"),
					tpe:  types.B,
				},
				{
					path: MustParseRef("data.a.b"),
					tpe:  nil,
				},
			},
		},
		{
			note: "primitive leafs inserted into object",
			insertions: []pathAndType{
				{
					path: MustParseRef("data.a.b"),
					tpe:  types.NewObject(nil, types.NewDynamicProperty(types.N, types.B)),
				},
				{
					path: MustParseRef("data.a.b.c"),
					tpe:  types.S,
				},
				{
					path: MustParseRef("data.a.b[true]"),
					tpe:  types.S,
				},
			},
			expected: []pathAndType{
				{
					path: MustParseRef("data.a.b"),
					tpe: types.NewObject(nil, types.NewDynamicProperty(
						types.Any{types.B, types.N, types.S}, types.Any{types.B, types.S})),
				},
			},
		},
		{
			note: "primitive leafs first, then object",
			insertions: []pathAndType{
				{
					path: MustParseRef("data.a.b.c"),
					tpe:  types.S,
				},
				{
					path: MustParseRef("data.a.b[true]"),
					tpe:  types.S,
				},
				{
					path: MustParseRef("data.a.b"),
					tpe:  types.NewObject(nil, types.NewDynamicProperty(types.N, types.B)),
				},
			},
			expected: []pathAndType{
				{
					path: MustParseRef("data.a.b"),
					tpe: types.NewObject(nil, types.NewDynamicProperty(
						types.Any{types.B, types.N, types.S},
						types.Any{types.B, types.S},
					)),
				},
			},
		},
		{
			note: "object beside object",
			insertions: []pathAndType{
				{
					path: MustParseRef("data.a.b"),
					tpe:  types.NewObject(nil, types.NewDynamicProperty(types.N, types.B)),
				},
				{
					path: MustParseRef("data.a.b"),
					tpe:  types.NewObject(nil, types.NewDynamicProperty(types.S, types.S)),
				},
			},
			expected: []pathAndType{
				{
					path: MustParseRef("data.a.b"),
					tpe: types.NewObject(nil, types.NewDynamicProperty(
						types.Any{types.N, types.S},
						types.Any{types.B, types.S},
					)),
				},
			},
		},
		{
			note: "object beside object with static types",
			insertions: []pathAndType{
				{
					path: MustParseRef("data.a.b"),
					tpe: types.NewObject(
						[]*types.StaticProperty{types.NewStaticProperty("foo", types.N)},
						types.NewDynamicProperty(types.N, types.B)),
				},
				{
					path: MustParseRef("data.a.b"),
					tpe: types.NewObject(
						[]*types.StaticProperty{types.NewStaticProperty("bar", types.S)},
						types.NewDynamicProperty(types.S, types.S)),
				},
			},
			expected: []pathAndType{
				{
					path: MustParseRef("data.a.b"),
					tpe: types.Any{
						types.NewObject(
							[]*types.StaticProperty{types.NewStaticProperty("foo", types.N)},
							types.NewDynamicProperty(types.N, types.B)),
						types.NewObject(
							[]*types.StaticProperty{types.NewStaticProperty("bar", types.S)},
							types.NewDynamicProperty(types.S, types.S)),
					},
				},
			},
		},
		{
			note: "object beside object with static types (2)",
			insertions: []pathAndType{
				{
					path: MustParseRef("data.a.b"),
					tpe: types.NewObject(
						nil,
						types.NewDynamicProperty(types.N, types.B)),
				},
				{
					path: MustParseRef("data.a.b"),
					tpe: types.NewObject(
						[]*types.StaticProperty{types.NewStaticProperty("bar", types.S)},
						types.NewDynamicProperty(types.S, types.S)),
				},
			},
			expected: []pathAndType{
				{
					path: MustParseRef("data.a.b"),
					tpe: types.Any{
						types.NewObject(
							nil,
							types.NewDynamicProperty(types.N, types.B)),
						types.NewObject(
							[]*types.StaticProperty{types.NewStaticProperty("bar", types.S)},
							types.NewDynamicProperty(types.S, types.S)),
					},
				},
			},
		},
		{
			note: "object beside object with static types (3)",
			insertions: []pathAndType{
				{
					path: MustParseRef("data.a.b"),
					tpe: types.NewObject(
						[]*types.StaticProperty{types.NewStaticProperty("foo", types.N)},
						types.NewDynamicProperty(types.N, types.B)),
				},
				{
					path: MustParseRef("data.a.b"),
					tpe: types.NewObject(
						nil,
						types.NewDynamicProperty(types.S, types.S)),
				},
			},
			expected: []pathAndType{
				{
					path: MustParseRef("data.a.b"),
					tpe: types.Any{
						types.NewObject(
							[]*types.StaticProperty{types.NewStaticProperty("foo", types.N)},
							types.NewDynamicProperty(types.N, types.B)),
						types.NewObject(
							nil,
							types.NewDynamicProperty(types.S, types.S)),
					},
				},
			},
		},
		{
			note: "object beside object with static types (4)",
			insertions: []pathAndType{
				{
					path: MustParseRef("data.a.b"),
					tpe: types.NewObject(
						[]*types.StaticProperty{types.NewStaticProperty("foo", types.N)},
						types.NewDynamicProperty(types.N, types.B)),
				},
				{
					path: MustParseRef("data.a.b"),
					tpe: types.NewObject(
						nil,
						types.NewDynamicProperty(types.S, types.S)),
				},
				{
					path: MustParseRef("data.a.b"),
					tpe: types.NewObject(
						nil,
						types.NewDynamicProperty(types.N, types.B)),
				},
			},
			expected: []pathAndType{
				{
					path: MustParseRef("data.a.b"),
					tpe: types.Any{
						types.NewObject(
							[]*types.StaticProperty{types.NewStaticProperty("foo", types.N)},
							types.NewDynamicProperty(types.N, types.B)),
						types.NewObject(
							nil,
							types.NewDynamicProperty(types.Any{types.N, types.S}, types.Any{types.B, types.S})),
					},
				},
			},
		},
		{
			note: "object into object",
			insertions: []pathAndType{
				{
					path: MustParseRef("data.a.b"),
					tpe:  types.NewObject(nil, types.NewDynamicProperty(types.N, types.B)),
				},
				{
					path: MustParseRef("data.a.b.c"),
					tpe:  types.NewObject(nil, types.NewDynamicProperty(types.B, types.N)),
				},
			},
			expected: []pathAndType{
				{
					path: MustParseRef("data.a.b"),
					tpe: types.NewObject(nil, types.NewDynamicProperty(
						types.Any{types.N, types.S},
						types.Any{types.B, types.NewObject(nil, types.NewDynamicProperty(types.B, types.N))},
					)),
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			root := newTypeTree()
			env := TypeEnv{tree: root}

			for _, insertion := range tc.insertions {
				root.Insert(insertion.path, insertion.tpe, &env)
			}

			for _, expected := range tc.expected {
				actual := root.Get(expected.path)
				if types.Compare(actual, expected.tpe) != 0 {
					t.Fatalf("Expected %v but got %v", expected.tpe, actual)
				}
			}
		})
	}
}

func TestTypeTreeInsert(t *testing.T) {
	env := TypeEnv{}
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
	n.Insert(abRef, types.NewObject(nil, &types.DynamicProperty{Key: types.N, Value: types.S}), &env)

	actual = n.Get(abRef)
	expected := types.NewObject(
		nil,
		types.NewDynamicProperty(
			types.Any{types.N, types.S},
			types.Any{types.B, types.S, types.NewObject(nil, types.NewDynamicProperty(types.S, types.N))}),
	)
	if types.Compare(actual, expected) != 0 {
		t.Fatalf("Expected %v but got %v", expected, actual)
	}

	// new "child" leafs should be added to new intermediate object leaf

	abfRef := Ref{NewTerm(String("a")), NewTerm(String("b")), NewTerm(Boolean(true))}
	n.Insert(abfRef, types.S, &env)

	actual = n.Get(abfRef)
	if types.Compare(actual, types.S) != 0 {
		t.Fatalf("Expected %v but got %v", types.S, actual)
	}

	actual = n.Get(abRef)
	expected = types.NewObject(
		nil,
		types.NewDynamicProperty(
			types.Any{types.B, types.N, types.S},
			types.Any{types.B, types.S, types.NewObject(nil, types.NewDynamicProperty(types.S, types.N))}),
	)
	if types.Compare(actual, expected) != 0 {
		t.Fatalf("Expected %v but got %v", expected, actual)
	}
}
