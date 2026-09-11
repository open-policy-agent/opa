// Copyright 2025 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"slices"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/v1/types"
)

func objType(props ...*types.StaticProperty) *types.Object {
	return types.NewObject(props, nil)
}

func staticProp(key string, tpe types.Type) *types.StaticProperty {
	return types.NewStaticProperty(key, tpe)
}

func TestSprintElided(t *testing.T) {
	t.Parallel()

	user := objType(staticProp("name", types.S), staticProp("roles", types.NewSet(types.S)))

	tests := []struct {
		note  string
		tpe   types.Type
		depth int
		exp   string
	}{
		{"nil", nil, 0, "???"},
		{"scalar", types.S, 0, "string"},
		{"set", types.NewSet(types.S), 0, "set[...]"},
		{"set, one level", types.NewSet(types.S), 1, "set[string]"},
		{"empty any", types.A, 0, "any"},
		{"any", types.NewAny(types.S, types.N), 0, "any<...>"},
		{"any, one level", types.NewAny(types.S, types.N), 1, "any<number, string>"},
		{"static array", types.NewArray([]types.Type{types.S, types.N}, nil), 0, "array<...>"},
		{"dynamic array", types.NewArray(nil, types.S), 0, "array[...]"},
		{"array with tail", types.NewArray([]types.Type{types.S}, types.N), 0, "array<...>[...]"},
		{"array with tail, one level", types.NewArray([]types.Type{types.S}, types.N), 1, "array<string>[number]"},
		{"static object", user, 0, "object<...>"},
		{"static object, one level", user, 1, "object<name: string, roles: set[...]>"},
		{"static object, two levels", user, 2, "object<name: string, roles: set[string]>"},
		{"dynamic object", types.NewObject(nil, types.NewDynamicProperty(types.S, types.A)), 0, "object[...]"},
		{"named", types.Named("collection", types.NewSet(types.S)), 0, "collection: set[...]"},
		{"function", types.NewFunction(types.Args(types.S), types.B), 0, "(...) => ..."},
		{"function, one level", types.NewFunction(types.Args(types.S), types.B), 1, "(string) => boolean"},
		{"void function", types.NewFunction(types.Args(types.S), nil), 0, "(...) => ???"},
		{"nullary function", types.NewFunction(nil, types.B), 0, "() => ..."},
		{"unelided object", user, -1, types.Sprint(user)},
		{"unelided nil", nil, -1, "???"},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			t.Parallel()
			if act := sprintElided(tc.tpe, tc.depth); act != tc.exp {
				t.Errorf("expected %q, got %q", tc.exp, act)
			}
		})
	}
}

func TestSprintElidedMatchesSprint(t *testing.T) {
	t.Parallel()

	for _, tpe := range []types.Type{
		nil,
		types.Nl, types.B, types.N, types.S, types.A,
		types.NewAny(types.S, types.N),
		types.NewSet(types.S),
		types.NewSet(nil),
		types.NewArray([]types.Type{types.S, types.N}, types.B),
		types.NewArray(nil, nil),
		types.NewObject(nil, nil),
		objType(staticProp("a", types.NewSet(types.NewArray(nil, types.S)))),
		types.NewObject([]*types.StaticProperty{staticProp("a", types.S)}, types.NewDynamicProperty(types.S, types.N)),
		types.NewFunction(types.Args(types.S, types.N), types.B),
		types.NewVariadicFunction(types.Args(types.S), types.N, nil),
		types.Named("x", types.NewSet(types.S)),
		types.NewRecursive("#/$defs/node", types.NewObject(nil, nil)),
	} {
		if exp, act := types.Sprint(tpe), sprintElided(tpe, 32); exp != act {
			t.Errorf("expected %q, got %q", exp, act)
		}
	}
}

func TestSprintDiff(t *testing.T) {
	t.Parallel()

	tests := []struct {
		note              string
		a, b              types.Type
		expLeft, expRight string
	}{
		{
			note:     "small operands are kept even when the constructors differ",
			a:        types.NewSet(types.S),
			b:        objType(staticProp("a", types.S)),
			expLeft:  "set[string]",
			expRight: "object<a: string>",
		},
		{
			note:     "large operands with different constructors are collapsed",
			a:        types.NewSet(objType(staticProp("action", types.NewSet(types.S)), staticProp("object", types.S))),
			b:        objType(staticProp("action", types.NewSet(types.S)), staticProp("object", types.S)),
			expLeft:  "set[...]",
			expRight: "object<...>",
		},
		{
			note:     "the differing member of a set is kept",
			a:        types.NewSet(types.S),
			b:        types.NewSet(types.N),
			expLeft:  "set[string]",
			expRight: "set[number]",
		},
		{
			note: "object properties that are equal on both sides are dropped",
			a: objType(staticProp("age", types.N), staticProp("name", types.S),
				staticProp("manager", objType(staticProp("age", types.N), staticProp("name", types.S)))),
			b: objType(staticProp("age", types.N), staticProp("name", types.S),
				staticProp("manager", objType(staticProp("age", types.S), staticProp("name", types.S)))),
			expLeft:  "object<manager: object<age: number, ...>, ...>",
			expRight: "object<manager: object<age: string, ...>, ...>",
		},
		{
			note:     "object properties missing on one side are kept",
			a:        objType(staticProp("age", types.N), staticProp("name", types.S)),
			b:        objType(staticProp("name", types.S)),
			expLeft:  "object<age: number, ...>",
			expRight: "object<...>",
		},
		{
			note:     "dynamic object properties are diffed",
			a:        types.NewObject(nil, types.NewDynamicProperty(types.S, types.N)),
			b:        types.NewObject(nil, types.NewDynamicProperty(types.S, types.B)),
			expLeft:  "object[string: number]",
			expRight: "object[string: boolean]",
		},
		{
			note:     "array elements are kept in place",
			a:        types.NewArray([]types.Type{types.S, types.N}, nil),
			b:        types.NewArray([]types.Type{types.S, types.B}, nil),
			expLeft:  "array<string, number>",
			expRight: "array<string, boolean>",
		},
		{
			note:     "arrays of different length are not diffed element-wise",
			a:        types.NewArray([]types.Type{types.S}, nil),
			b:        types.NewArray([]types.Type{types.S, types.N}, nil),
			expLeft:  "array<string>",
			expRight: "array<string, number>",
		},
		{
			note:     "any members that are common to both sides are dropped",
			a:        types.NewAny(types.S, types.N, types.B),
			b:        types.NewAny(types.S, types.N),
			expLeft:  "any<boolean, ...>",
			expRight: "any<...>",
		},
		{
			note:     "the unknown type is reported as such",
			a:        nil,
			b:        types.NewSet(types.A),
			expLeft:  "???",
			expRight: "set[any]",
		},
		{
			note:     "argument names are kept",
			a:        types.S,
			b:        types.Named("collection", types.NewSet(types.S)),
			expLeft:  "string",
			expRight: "collection: set[string]",
		},
		{
			note:     "equal types are collapsed",
			a:        objType(staticProp("a", types.S)),
			b:        objType(staticProp("a", types.S)),
			expLeft:  "object<a: string>",
			expRight: "object<a: string>",
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			t.Parallel()
			left, right := sprintDiff(tc.a, tc.b)
			if left != tc.expLeft || right != tc.expRight {
				t.Errorf("expected (%q, %q), got (%q, %q)", tc.expLeft, tc.expRight, left, right)
			}
		})
	}
}

func TestSprintDiffTerminates(t *testing.T) {
	t.Parallel()

	node := types.NewRecursive("#/$defs/node", nil)
	node.SetType(objType(staticProp("child", node), staticProp("value", types.S)))

	other := objType(staticProp("child", node), staticProp("value", types.N))

	if left, right := sprintDiff(node.Unwrap(), other); left == "" || right == "" {
		t.Fatalf("expected non-empty renderings, got (%q, %q)", left, right)
	}
}

func TestArgErrDetailLines(t *testing.T) {
	t.Parallel()

	acl := objType(staticProp("action", types.NewSet(types.S)), staticProp("object", types.S))

	tests := []struct {
		note string
		have []types.Type
		want types.FuncArgs
		exp  []string
	}{
		{
			note: "narrow detail is not elided",
			have: []types.Type{types.N, types.S},
			want: types.FuncArgs{Args: []types.Type{types.S, types.S}},
			exp: []string{
				"have: (number, string)",
				"want: (string, string)",
			},
		},
		{
			// https://github.com/open-policy-agent/opa/issues/499
			note: "compatible arguments are collapsed",
			have: []types.Type{types.NewSet(acl), acl, nil},
			want: types.FuncArgs{Args: []types.Type{types.SetOfAny, types.SetOfAny, types.SetOfAny}},
			exp: []string{
				"have: (set[...], object<...>, ???)",
				"want: (set[any], set[any], set[any])",
			},
		},
		{
			note: "arity errors keep unpaired arguments",
			have: []types.Type{types.NewSet(acl), acl, types.S},
			want: types.FuncArgs{Args: []types.Type{types.SetOfAny}},
			exp: []string{
				"have: (set[...], object<...>, string)",
				"want: (set[any])",
			},
		},
		{
			note: "variadic arguments are kept",
			have: []types.Type{types.NewSet(acl), acl},
			want: types.FuncArgs{Args: []types.Type{types.SetOfAny}, Variadic: types.A},
			exp: []string{
				"have: (set[...], object<...>)",
				"want: (set[any], any...)",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			t.Parallel()
			d := &ArgErrDetail{Have: tc.have, Want: tc.want}
			if act := d.Lines(); !slices.Equal(act, tc.exp) {
				t.Errorf("expected\n\n%v\n\ngot\n\n%v", tc.exp, act)
			}
		})
	}
}

func TestUnificationErrDetailLines(t *testing.T) {
	t.Parallel()

	tests := []struct {
		note        string
		left, right types.Type
		exp         []string
	}{
		{
			note:  "narrow detail is not elided",
			left:  objType(staticProp("age", types.N), staticProp("name", types.S)),
			right: objType(staticProp("name", types.S)),
			exp: []string{
				"left  : object<age: number, name: string>",
				"right : object<name: string>",
			},
		},
		{
			note: "only the differing property is kept",
			left: objType(staticProp("age", types.N), staticProp("name", types.S), staticProp("roles", types.NewSet(types.S)),
				staticProp("manager", objType(staticProp("age", types.N), staticProp("name", types.S)))),
			right: objType(staticProp("age", types.N), staticProp("name", types.S), staticProp("roles", types.NewSet(types.S)),
				staticProp("manager", objType(staticProp("age", types.S), staticProp("name", types.S)))),
			exp: []string{
				"left  : object<manager: object<age: number, ...>, ...>",
				"right : object<manager: object<age: string, ...>, ...>",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			t.Parallel()
			d := &UnificationErrDetail{Left: tc.left, Right: tc.right}
			if act := d.Lines(); !slices.Equal(act, tc.exp) {
				t.Errorf("expected\n\n%v\n\ngot\n\n%v", tc.exp, act)
			}
		})
	}
}

func TestTypeErrorElisionCompile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		note   string
		module string
		exp    string
	}{
		{
			note: "union of a set and an object",
			module: `package test

acl := {"action": {"read"}, "object": "doc"}

r := {acl} | acl
`,
			exp: `rego_type_error: or: invalid argument(s)
	have: (set[...], object<...>, ???)
	want: (x: set[any], y: set[any], z: set[any])`,
		},
		{
			note: "wide object passed where a collection of strings is expected",
			module: `package test

x := {"a": 1, "b": 2, "c": 3, "d": 4, "e": 5, "f": 6, "g": 7, "h": 8, "i": 9, "j": 10}

y := concat(", ", x)
`,
			exp: `rego_type_error: concat: invalid argument(s)
	have: (string, object<...>, ???)
	want: (delimiter: string, collection: any<array[string], set[string]>, output: string)`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			t.Parallel()

			c := NewCompiler()
			c.Compile(map[string]*Module{"test.rego": MustParseModule(tc.module)})

			if !c.Failed() {
				t.Fatal("expected compilation to fail")
			}
			if act := c.Errors.Error(); !strings.Contains(act, tc.exp) {
				t.Fatalf("expected error to contain\n\n%s\n\ngot\n\n%s", tc.exp, act)
			}
		})
	}
}
