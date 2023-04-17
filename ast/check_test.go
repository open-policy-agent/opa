// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/types"
	"github.com/open-policy-agent/opa/util"
)

func TestCheckInference(t *testing.T) {

	// fake_builtin_1([str1,str2])
	RegisterBuiltin(&Builtin{
		Name: "fake_builtin_1",
		Decl: types.NewFunction(
			nil,
			types.NewArray(
				[]types.Type{types.S, types.S}, nil,
			),
		),
	})

	// fake_builtin_2({"a":str1,"b":str2})
	RegisterBuiltin(&Builtin{
		Name: "fake_builtin_2",
		Decl: types.NewFunction(
			nil,
			types.NewObject(
				[]*types.StaticProperty{
					{Key: "a", Value: types.S},
					{Key: "b", Value: types.S},
				}, nil,
			),
		),
	})

	// fake_builtin_3({str1,str2,...})
	RegisterBuiltin(&Builtin{
		Name: "fake_builtin_3",
		Decl: types.NewFunction(
			nil,
			types.NewSet(types.S),
		),
	})

	tests := []struct {
		note     string
		query    string
		expected map[Var]types.Type
	}{
		{"trivial", `x = 1`, map[Var]types.Type{
			Var("x"): types.N,
		}},
		{"one-level", "y = 1; x = y", map[Var]types.Type{
			Var("x"): types.N,
			Var("y"): types.N,
		}},
		{"two-level", "z = 1; y = z; x = y", map[Var]types.Type{
			Var("x"): types.N,
			Var("y"): types.N,
			Var("z"): types.N,
		}},
		{"array-nested", "[x, 1] = [true, y]", map[Var]types.Type{
			Var("x"): types.B,
			Var("y"): types.N,
		}},
		{"array-transitive", "y = [[2], 1]; [[x], 1] = y", map[Var]types.Type{
			Var("x"): types.N,
			Var("y"): types.NewArray(
				[]types.Type{
					types.NewArray([]types.Type{types.N}, nil),
					types.N,
				}, nil),
		}},
		{"array-embedded", `[1, "2", x] = data.foo`, map[Var]types.Type{
			Var("x"): types.A,
		}},
		{"object-nested", `{"a": "foo", "b": {"c": x}} = {"a": y, "b": {"c": 2}}`, map[Var]types.Type{
			Var("x"): types.N,
			Var("y"): types.S,
		}},
		{"object-transitive", `y = {"a": "foo", "b": 2}; {"a": z, "b": x} = y`, map[Var]types.Type{
			Var("x"): types.N,
			Var("z"): types.S,
		}},
		{"object-embedded", `{"1": "2", "2": x} = data.foo`, map[Var]types.Type{
			Var("x"): types.A,
		}},
		{"object-numeric-key", `x = {1: 2}; y = 1; x[y]`, map[Var]types.Type{
			Var("x"): types.NewObject([]*types.StaticProperty{{Key: json.Number("1"), Value: types.N}}, nil),
			Var("y"): types.N,
		}},
		{"object-object-key", `x = {{{}: 1}: 1}`, map[Var]types.Type{
			Var("x"): types.NewObject(
				[]*types.StaticProperty{types.NewStaticProperty(
					map[string]interface{}{
						"{}": json.Number("1"),
					},
					types.N,
				)},
				nil,
			),
		}},
		{"object-composite-ref-operand", `x = {{}: 1}; x[{}] = y`, map[Var]types.Type{
			Var("x"): types.NewObject(
				[]*types.StaticProperty{types.NewStaticProperty(
					map[string]interface{}{},
					types.N,
				)},
				nil,
			),
			Var("y"): types.N,
		}},
		{"sets", `x = {1, 2}; y = {{"foo", 1}, x}`, map[Var]types.Type{
			Var("x"): types.NewSet(types.N),
			Var("y"): types.NewSet(
				types.NewAny(
					types.NewSet(
						types.NewAny(types.N, types.S),
					),
					types.NewSet(
						types.N,
					),
				),
			),
		}},
		{"sets-nested", `{"a", 1, 2} = {1,2,3}`, nil},
		{"sets-composite-ref-operand", `s = {[1, 2], [3, 4]}; s[[x, y]]`, map[Var]types.Type{
			Var("x"): types.N,
			Var("y"): types.N,
			Var("s"): types.NewSet(types.NewArray([]types.Type{types.N, types.N}, nil)),
		}},
		{"empty-composites", `
				obj = {};
				arr = [];
				set = set();
				obj[i] = v1;
				arr[j] = v2;
				set[v3];
				obj = {"foo": "bar"};
				arr = [1];
				set = {1,2,3}
				`, map[Var]types.Type{
			Var("obj"): types.NewObject(nil, types.NewDynamicProperty(types.A, types.A)),
			Var("i"):   types.A,
			Var("v1"):  types.A,
			Var("arr"): types.NewArray(nil, types.A),
			Var("j"):   types.N,
			Var("v2"):  types.A,
			Var("set"): types.NewSet(types.A),
			Var("v3"):  types.A,
		}},
		{"empty-composite-property", `
			obj = {};
			obj.foo = x;
			obj[i].foo = y
		`, map[Var]types.Type{
			Var("x"): types.A,
			Var("y"): types.A,
		}},
		{"local-reference", `
			a = [
				1,
				{
					"foo": [
						{"bar": null},
						-1,
						{"bar": true}
					]
				},
				3];

			x = a[1].foo[_].bar`, map[Var]types.Type{
			Var("x"): types.NewAny(types.NewNull(), types.B),
		}},
		{"local-reference-var", `

			a = [
				{
					"a": null,
					"b": {
						"foo": {
							"c": {1,},
						},
						"bar": {
							"c": {"hello",},
						},
					},
				},
				{
					"a": null,
					"b": {
						"foo": {
							"c": {1,},
						},
						"bar": {
							"c": {true,},
						},
					},
				},
			];
			x = a[i].b[j].c[k]
			`, map[Var]types.Type{
			Var("i"): types.N,
			Var("j"): types.S,
			Var("k"): types.NewAny(types.S, types.N, types.B),
			Var("x"): types.NewAny(types.S, types.N, types.B),
		}},
		{"local-reference-var-any", `
			a = [[], {}];
			a[_][i]
		`, map[Var]types.Type{
			Var("i"): types.A,
		}},
		{"local-reference-nested", `
			a = [["foo"], 0, {"bar": "baz"}, 2];
			b = [0,1,2,3];
			a[b[_]][k] = v
			`, map[Var]types.Type{
			Var("k"): types.NewAny(types.S, types.N),
		}},
		{"simple-built-in", "plus(1,2,x)", map[Var]types.Type{
			Var("x"): types.N,
		}},
		{"simple-built-in-exists", "plus(1,2,x); plus(x,2,y)", map[Var]types.Type{
			Var("x"): types.N,
			Var("y"): types.N,
		}},
		{"array-builtin", `fake_builtin_1([x,"foo"])`, map[Var]types.Type{
			Var("x"): types.S,
		}},
		{"object-builtin", `fake_builtin_2({"a": "foo", "b": x})`, map[Var]types.Type{
			Var("x"): types.S,
		}},
		{"set-builtin", `fake_builtin_3({"foo", x})`, map[Var]types.Type{
			Var("x"): types.S,
		}},
		{"array-comprehension-ref-closure", `a = [1,"foo",3]; x = [ i | a[_] = i ]`, map[Var]types.Type{
			Var("x"): types.NewArray(nil, types.NewAny(types.N, types.S)),
		}},
		{"array-comprehension-var-closure", `x = 1; y = [ i | x = i ]`, map[Var]types.Type{
			Var("y"): types.NewArray(nil, types.N),
		}},
		{"dynamic-object-value", `q = {"a": "b", "c": "d"}; {k: [v]} = {k: [q[k]]}`, map[Var]types.Type{
			Var("k"): types.S,
			Var("v"): types.A,
		}},
		{
			note:  "type unioning: arrays",
			query: `x = [[1], ["foo"]]; x[_] = [y]`,
			expected: map[Var]types.Type{
				Var("y"): types.NewAny(
					types.N, types.S,
				),
			},
		},
		{
			note:  "type unioning: sets",
			query: `x = {[1], ["foo"]}; x[[y]]`,
			expected: map[Var]types.Type{
				Var("y"): types.NewAny(
					types.N, types.S,
				),
			},
		},
		{
			note:  "type unioning: object values",
			query: `x = {"a": [1], "b": ["foo"]}; x[_] = [y]`,
			expected: map[Var]types.Type{
				Var("y"): types.NewAny(
					types.N, types.S,
				),
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			body := MustParseBody(tc.query)
			checker := newTypeChecker()
			env := checker.Env(BuiltinMap)
			env, err := checker.CheckBody(env, body)
			if len(err) != 0 {
				t.Fatalf("Unexpected error: %v", err)
			}
			for k, tpe := range tc.expected {
				result := env.Get(k)
				if tpe == nil {
					if result != nil {
						t.Errorf("Expected %v type to be unset but got: %v", k, result)
					}
				} else {
					if result == nil {
						t.Errorf("Expected to infer %v => %v but got nil", k, tpe)
					} else if types.Compare(tpe, result) != 0 {
						t.Errorf("Expected to infer %v => %v but got %v", k, tpe, result)
					}
				}
			}
		})
	}
}

func TestCheckInferenceRules(t *testing.T) {

	// Rules must have refs resolved, safe ordering, etc. Each pair is a
	// (package path, rule) tuple. The test constructs the Rule objects to
	// run the inference on from these inputs.
	ruleset1 := [][2]string{
		{`a`, `trivial = true { true }`},
		{`a`, `complete = [{"foo": x}] { x = 1 }`},
		{`a`, `partialset[{"foo": x}] { y = "bar"; x = y }`},
		{`a`, `partialobj[x] = {"foo": y} { y = "bar"; x = y }`},
		{`b`, `trivial_ref = x { x = data.a.trivial }`},
		{`b`, `transitive_ref = [x] { y = data.b.trivial_ref; x = y }`},
		{`c`, `else_kw = null { false } else = 100 { true } else = "foo" { true }`},
		{`iteration`, `arr = [[1], ["two"], {"x": true}, ["four"]] { true }`},
		{`iteration`, `values[x] { data.iteration.arr[_][_] = x } `},
		{`iteration`, `keys[i] { data.iteration.arr[_][i] = _ } `},
		{`disjunction`, `partialset[1] { true }`},
		{`disjunction`, `partialset[x] { x = "foo" }`},
		{`disjunction`, `partialset[3] { true }`},
		{`disjunction`, `partialobj[x] = y { y = "bar"; x = "foo" }`},
		{`disjunction`, `partialobj[x] = y { y = 100; x = "foo" }`},
		{`disjunction`, `complete = 1 { true }`},
		{`disjunction`, `complete = x { x = "foo" }`},
		{`prefix.a.b.c`, `d = true { true }`},
		{`prefix.i.j.k`, `p = 1 { true }`},
		{`prefix.i.j.k`, `p = "foo" { true }`},
		{`default_rule`, `default x = 1`},
		{`default_rule`, `x = "foo" { true }`},
		{`unknown_type`, `p = [x] { x = data.deadbeef }`},
		{`nested_ref`, `inner = {"a": 0, "b": "1"} { true }`},
		{`nested_ref`, `middle = [[1, true], ["foo", false]] { true }`},
		{`nested_ref`, `p = x { data.nested_ref.middle[data.nested_ref.inner.a][0] = x }`},
		{`number_key`, `q[x] = y { a = ["a", "b"]; y = a[x] }`},
		{`non_leaf`, `p[x] { data.prefix.i[x][_] }`},
	}
	ruleset2 := [][2]string{
		{`ref_rule_single`, `p.q.r { true }`},
		{`ref_rule_single_with_number_key`, `p.q[3] { true }`},
		{`ref_regression_array_key`,
			`walker[[p, v]] = o { l = input; walk(l, k); [p, v] = k; o = {} }`},
	}

	tests := []struct {
		note     string
		rules    [][2]string
		ref      string
		expected types.Type
	}{
		{"trivial", ruleset1, `data.a.trivial`, types.B},

		{"complete-doc", ruleset1, `data.a.complete`, types.NewArray(
			[]types.Type{types.NewObject(
				[]*types.StaticProperty{{
					Key: "foo", Value: types.N,
				}},
				nil,
			)},
			nil,
		)},

		{"complete-doc-suffix", ruleset1, `data.a.complete[0].foo`, types.N},

		{"else-kw", ruleset1, "data.c.else_kw", types.NewAny(types.NewNull(), types.N, types.S)},

		{"partial-set-doc", ruleset1, `data.a.partialset`, types.NewSet(
			types.NewObject(
				[]*types.StaticProperty{{
					Key: "foo", Value: types.S,
				}},
				nil,
			),
		)},

		{"partial-object-doc", ruleset1, "data.a.partialobj", types.NewObject(
			nil,
			types.NewDynamicProperty(types.S, types.NewObject(
				[]*types.StaticProperty{{
					Key: "foo", Value: types.S,
				}},
				nil,
			)),
		)},

		{"partial-object-doc-suffix", ruleset1, `data.a.partialobj.somekey.foo`, types.S},

		{"partial-object-doc-number-suffix", ruleset1, "data.number_key.q[1]", types.S},

		{"iteration", ruleset1, "data.iteration.values", types.NewSet(
			types.NewAny(
				types.S,
				types.N,
				types.B),
		)},

		{"iteration-keys", ruleset1, "data.iteration.keys", types.NewSet(
			types.NewAny(
				types.S,
				types.N,
			),
		)},

		{"disj-complete-doc", ruleset1, "data.disjunction.complete", types.NewAny(
			types.S,
			types.N,
		)},

		{"disj-partial-set-doc", ruleset1, "data.disjunction.partialset", types.NewSet(
			types.NewAny(
				types.S,
				types.N),
		)},

		{"disj-partial-obj-doc", ruleset1, "data.disjunction.partialobj", types.NewObject(
			nil,
			types.NewDynamicProperty(types.S, types.NewAny(types.S, types.N)),
		)},

		{"ref", ruleset1, "data.b.trivial_ref", types.B},

		{"ref-transitive", ruleset1, "data.b.transitive_ref", types.NewArray(
			[]types.Type{
				types.B,
			},
			nil,
		)},

		{"prefix", ruleset1, `data.prefix.a.b`, types.NewObject(
			[]*types.StaticProperty{{
				Key: "c", Value: types.NewObject(
					[]*types.StaticProperty{{Key: "d", Value: types.B}},
					types.NewDynamicProperty(types.S, types.A),
				),
			}},
			types.NewDynamicProperty(types.S, types.A),
		)},

		// Check that prefixes that iterate fallback to any.
		{"prefix-iter", ruleset1, `data.prefix.i.j[k]`, types.A},

		// Check that iteration targeting a rule (but nonetheless prefixed) falls back to any.
		{"prefix-iter-2", ruleset1, `data.prefix.i.j[k].p`, types.A},

		{"default-rule", ruleset1, "data.default_rule.x", types.NewAny(
			types.S,
			types.N,
		)},

		{"unknown-type", ruleset1, "data.unknown_type.p", types.NewArray(
			[]types.Type{
				types.A,
			},
			nil,
		)},

		{"nested-ref", ruleset1, "data.nested_ref.p", types.NewAny(
			types.S,
			types.N,
		)},

		{"non-leaf", ruleset1, "data.non_leaf.p", types.NewSet(
			types.S,
		)},

		{"ref-rules single value, full ref", ruleset2, "data.ref_rule_single.p.q.r", types.B},
		{"ref-rules single value, prefix", ruleset2, "data.ref_rule_single.p",
			types.NewObject(
				[]*types.StaticProperty{{
					Key: "q", Value: types.NewObject(
						[]*types.StaticProperty{{Key: "r", Value: types.B}},
						types.NewDynamicProperty(types.S, types.A),
					),
				}},
				types.NewDynamicProperty(types.S, types.A),
			)},

		{"ref-rules single value, number key, full ref", ruleset2, "data.ref_rule_single_with_number_key.p.q[3]", types.B},
		{"ref-rules single value, number key, prefix", ruleset2, "data.ref_rule_single_with_number_key.p",
			types.NewObject(
				[]*types.StaticProperty{{
					Key: "q", Value: types.NewObject(
						[]*types.StaticProperty{{Key: json.Number("3"), Value: types.B}},
						types.NewDynamicProperty(types.S, types.A),
					),
				}},
				types.NewDynamicProperty(types.S, types.A),
			)},

		{"ref_regression_array_key", ruleset2, "data.ref_regression_array_key.walker",
			types.NewObject(
				nil,
				types.NewDynamicProperty(types.NewArray([]types.Type{types.NewArray(types.A, types.A), types.A}, nil),
					types.NewObject(nil, types.NewDynamicProperty(types.A, types.A))),
			)},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			var elems []util.T

			// Convert test rules into rule slice for call.
			for i := range tc.rules {
				pkg := MustParsePackage(`package ` + tc.rules[i][0])
				rule := MustParseRule(tc.rules[i][1])
				module := &Module{
					Package: pkg,
					Rules:   []*Rule{rule},
				}
				rule.Module = module
				elems = append(elems, rule)
				for next := rule.Else; next != nil; next = next.Else {
					next.Module = module
					elems = append(elems, next)
				}
			}

			ref := MustParseRef(tc.ref)
			checker := newTypeChecker()
			env, err := checker.CheckTypes(newTypeChecker().Env(map[string]*Builtin{"walk": BuiltinMap["walk"]}), elems, nil)

			if err != nil {
				t.Fatalf("Unexpected error %v:", err)
			}

			result := env.Get(ref)
			if tc.expected == nil {
				if result != nil {
					t.Errorf("Expected %v type to be unset but got: %v", ref, result)
				}
			} else {
				if result == nil {
					t.Errorf("Expected to infer %v => %v but got nil", ref, tc.expected)
				} else if types.Compare(tc.expected, result) != 0 {
					t.Errorf("Expected to infer %v => %v but got %v", ref, tc.expected, result)
				}
			}
		})
	}

}

func TestCheckInferenceOverlapWithRules(t *testing.T) {
	ruleset1 := [][2]string{
		{`prefix.i.j.k`, `p = 1 { true }`},
		{`prefix.i.j.k`, `p = "foo" { true }`},
	}
	tests := []struct {
		note     string
		rules    [][2]string
		ref      string
		expected types.Type // ref's type
		query    string
		extra    map[Var]types.Type
	}{
		{
			note:     "non-leaf, extra vars",
			rules:    ruleset1,
			ref:      "data.prefix.i.j[k]",
			expected: types.A,
			query:    "data.prefix.i.j[k][b]",
			extra: map[Var]types.Type{
				Var("k"): types.S,
				Var("b"): types.S,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			var elems []util.T

			// Convert test rules into rule slice for "warmup" call.
			for i := range tc.rules {
				pkg := MustParsePackage(`package ` + tc.rules[i][0])
				rule := MustParseRule(tc.rules[i][1])
				module := &Module{
					Package: pkg,
					Rules:   []*Rule{rule},
				}
				rule.Module = module
				elems = append(elems, rule)
				for next := rule.Else; next != nil; next = next.Else {
					next.Module = module
					elems = append(elems, next)
				}
			}

			ref := MustParseRef(tc.ref)
			checker := newTypeChecker()
			env, err := checker.CheckTypes(nil, elems, nil)
			if err != nil {
				t.Fatalf("Unexpected error %v:", err)
			}

			result := env.Get(ref)
			if tc.expected == nil {
				if result != nil {
					t.Errorf("Expected %v type to be unset but got: %v", ref, result)
				}
			} else {
				if result == nil {
					t.Errorf("Expected to infer %v => %v but got nil", ref, tc.expected)
				} else if types.Compare(tc.expected, result) != 0 {
					t.Errorf("Expected to infer %v => %v but got %v", ref, tc.expected, result)
				}
			}

			body := MustParseBody(tc.query)
			env, err = checker.CheckBody(env, body)
			if len(err) != 0 {
				t.Fatalf("Unexpected error: %v", err)
			}
			for ex, exp := range tc.extra {
				act := env.Get(ex)
				if types.Compare(act, exp) != 0 {
					t.Errorf("Expected to infer extra %v => %v but got %v", ex, exp, act)
				}
			}
		})
	}
}

func TestCheckErrorSuppression(t *testing.T) {

	query := `arr = [1,2,3]; arr[0].deadbeef = 1`

	_, errs := newTypeChecker().CheckBody(nil, MustParseBody(query))
	if len(errs) != 1 {
		t.Fatalf("Expected exactly one error but got: %v", errs)
	}

	_, ok := errs[0].Details.(*RefErrUnsupportedDetail)
	if !ok {
		t.Fatalf("Expected ref error but got: %v", errs)
	}

	query = `_ = [true | count(1)]`

	_, errs = newTypeChecker().CheckBody(newTypeChecker().Env(BuiltinMap), MustParseBody(query))
	if len(errs) != 1 {
		t.Fatalf("Expected exactly one error but got: %v", errs)
	}

	_, ok = errs[0].Details.(*ArgErrDetail)
	if !ok {
		t.Fatalf("Expected arg error but got: %v", errs)
	}

}

func TestCheckBadCardinality(t *testing.T) {
	tests := []struct {
		body string
		exp  []types.Type
	}{
		{
			body: "plus(1)",
			exp:  []types.Type{types.N},
		},
		{
			body: "plus(1, 2, 3, 4)",
			exp:  []types.Type{types.N, types.N, types.N, types.N},
		},
	}
	for _, test := range tests {
		body := MustParseBody(test.body)
		tc := newTypeChecker()
		env := tc.Env(BuiltinMap)
		_, err := tc.CheckBody(env, body)
		if len(err) != 1 || err[0].Code != TypeErr {
			t.Fatalf("Expected 1 type error from %v but got: %v", body, err)
		}
		detail, ok := err[0].Details.(*ArgErrDetail)
		if !ok {
			t.Fatalf("Expected argument error details but got: %v", err)
		}
		if len(test.exp) != len(detail.Have) {
			t.Fatalf("Expected arg types %v but got: %v", test.exp, detail.Have)
		}
		for i := range test.exp {
			if types.Compare(test.exp[i], detail.Have[i]) != 0 {
				t.Fatalf("Expected types for %v to be %v but got: %v", body[0], test.exp, detail.Have)
			}
		}
	}
}

func TestCheckMatchErrors(t *testing.T) {
	tests := []struct {
		note  string
		query string
	}{
		{"null", "null = true"},
		{"boolean", "true = null"},
		{"number", "1 = null"},
		{"string", `"hello" = null`},
		{"array", "[1,2,3] = null"},
		{"array-nested", `[1,2,3] = [1,2,"3"]`},
		{"array-nested-2", `[1,2] = [1,2,3]`},
		{"array-dynamic", `[ true | true ] = [x | a = [1, "foo"]; x = a[_]]`},
		{"object", `{"a": 1, "b": 2} = null`},
		{"object-nested", `{"a": 1, "b": "2"} = {"a": 1, "b": 2}`},
		{"object-nested-2", `{"a": 1} = {"a": 1, "b": "2"}`},
		{"set", "{1,2,3} = null"},
		{"any", `x = ["str", 1]; x[_] = null`},
	}
	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			body := MustParseBody(tc.query)
			checker := newTypeChecker()
			_, err := checker.CheckBody(nil, body)
			if len(err) != 1 {
				t.Fatalf("Expected exactly one error from %v, but got:\n%v", body, err)
			}
		})
	}
}

func TestCheckBuiltinErrors(t *testing.T) {

	RegisterBuiltin(&Builtin{
		Name: "fake_builtin_2",
		Decl: types.NewFunction(
			types.Args(
				types.NewAny(types.NewObject(
					[]*types.StaticProperty{
						{Key: "a", Value: types.S},
						{Key: "b", Value: types.S},
					}, nil),
				),
			),
			types.NewObject(
				[]*types.StaticProperty{
					{Key: "b", Value: types.S},
					{Key: "c", Value: types.S},
				}, nil,
			),
		),
	})

	tests := []struct {
		note  string
		query string
	}{
		{"trivial", "plus(true, 1, x)"},
		{"refs", "x = [null]; plus(x[0], 1, y)"},
		{"array comprehensions", `sum([null | true], x)`},
		{"arrays-any", `sum([1,2,"3",4], x)`},
		{"arrays-bad-input", `contains([1,2,3], "x")`},
		{"objects-any", `fake_builtin_2({"a": a, "c": c})`},
		{"objects-bad-input", `sum({"a": 1, "b": 2}, x)`},
		{"sets-any", `sum({1,2,"3",4}, x)`},
		{"virtual-ref", `plus(data.test.p, data.coffee, 0)`},
	}

	env := newTestEnv([]string{
		`p = "foo" { true }`,
		`f(x) = x { true }`,
	})

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			body := MustParseBody(tc.query)
			checker := newTypeChecker()
			_, err := checker.CheckBody(env, body)
			if len(err) != 1 {
				t.Fatalf("Expected exactly one error from %v but got:\n%v", body, err)
			}
		})
	}
}

func TestVoidBuiltins(t *testing.T) {

	// Void builtins are used in test cases.
	RegisterBuiltin(&Builtin{
		Name: "fake_void_builtin",
		Decl: types.NewFunction(
			types.Args(types.N),
			nil,
		),
	})

	tests := []struct {
		query   string
		wantErr bool
	}{
		{"fake_void_builtin(1)", false},
		{"fake_void_builtin()", true},
		{"fake_void_builtin(1,2)", true},
		{"fake_void_builtin(true)", true},
	}

	for _, tc := range tests {
		body := MustParseBody(tc.query)
		checker := newTypeChecker()
		_, errs := checker.CheckBody(newTestEnv(nil), body)
		if len(errs) != 0 && !tc.wantErr {
			t.Fatal(errs)
		} else if len(errs) == 0 && tc.wantErr {
			t.Fatal("Expected error")
		}
	}
}

func TestVariadicBuiltins(t *testing.T) {

	// Ensure type checking allows variadic arguments.
	env := newTypeChecker().Env(map[string]*Builtin{
		"println": {
			Name: "println",
			Decl: types.NewVariadicFunction([]types.Type{}, types.A, nil),
		},
	})

	_, errs := newTypeChecker().CheckBody(env, MustParseBody(`println("hello", "world")`))
	if len(errs) != 0 {
		t.Fatal(errs)
	}

	// Test error checking on positional arguments on a variadic function.
	env = newTypeChecker().Env(map[string]*Builtin{
		"println": {
			Name: "println",
			Decl: types.NewVariadicFunction([]types.Type{types.N}, types.A, nil),
		},
	})

	_, errs = newTypeChecker().CheckBody(env, MustParseBody(`println("hello", 7)`))
	if len(errs) != 1 {
		t.Fatal("expected one error but got:", errs)
	}

	detail := errs[0].Details.(*ArgErrDetail)

	if len(detail.Have) != 2 || len(detail.Want.Args) != 1 || types.Compare(detail.Want.Args[0], types.N) != 0 {
		t.Fatal("unexpected detail:", detail)
	}

	// Test error checking on variadic arguments.
	env = newTypeChecker().Env(map[string]*Builtin{
		"println": {
			Name: "println",
			Decl: types.NewVariadicFunction([]types.Type{types.N}, types.N, nil),
		},
	})

	_, errs = newTypeChecker().CheckBody(env, MustParseBody(`println(7, "world")`))
	if len(errs) != 1 {
		t.Fatal("expected one error but got:", errs)
	}

	detail = errs[0].Details.(*ArgErrDetail)

	if len(detail.Have) != 2 || len(detail.Want.Args) != 1 || types.Compare(detail.Want.Args[0], types.N) != 0 || types.Compare(detail.Want.Variadic, types.N) != 0 {
		t.Fatal("unexpected detail:", detail)
	}

}

func TestCheckRefErrUnsupported(t *testing.T) {

	query := `arr = [[1,2],[3,4]]; arr[1][0].deadbeef`

	_, errs := newTypeChecker().CheckBody(nil, MustParseBody(query))
	if len(errs) != 1 {
		t.Fatalf("Expected exactly one error but got: %v", errs)
	}

	details, ok := errs[0].Details.(*RefErrUnsupportedDetail)
	if !ok {
		t.Fatalf("Expected ref err unsupported but got: %v", errs)
	}

	wantRef := MustParseRef(`arr[1][0].deadbeef`)
	wantPos := 2
	wantHave := types.N

	if !wantRef.Equal(details.Ref) ||
		wantPos != details.Pos ||
		types.Compare(wantHave, details.Have) != 0 {
		t.Fatalf("Expected (%v, %v, %v) but got: (%v, %v, %v)", wantRef, wantPos, wantHave, details.Ref, details.Pos, details.Have)
	}

}

func TestCheckRefErrInvalid(t *testing.T) {

	env := newTestEnv([]string{
		`p { true }`,
		`q = {"foo": 1, "bar": 2} { true }`,
		`a.b.c[3] = x { x = {"x": {"y": 2}} }`,
	})

	tests := []struct {
		note  string
		query string
		ref   string
		pos   int
		have  types.Type
		want  types.Type
		oneOf []Value
	}{
		{
			note:  "bad non-leaf var",
			query: `x = 1; data.test[x]`,
			ref:   `data.test[x]`,
			pos:   2,
			have:  types.N,
			want:  types.S,
			oneOf: []Value{String("a"), String("p"), String("q")},
		},
		{
			note:  "bad non-leaf ref",
			query: `arr = [1]; data.test[arr[0]]`,
			ref:   `data.test[arr[0]]`,
			pos:   2,
			have:  types.N,
			want:  types.S,
			oneOf: []Value{String("a"), String("p"), String("q")},
		},
		{
			note:  "bad leaf ref",
			query: `arr = [1]; data.test.q[arr[0]]`,
			ref:   `data.test.q[arr[0]]`,
			pos:   3,
			have:  types.N,
			want:  types.S,
			oneOf: []Value{String("bar"), String("foo")},
		},
		{
			note:  "bad ref hitting last term",
			query: `x = true; data.test.a.b.c[x][_]`,
			ref:   `data.test.a.b.c[x][_]`,
			pos:   5,
			have:  types.B,
			want:  types.Any{types.N, types.S},
			oneOf: []Value{Number("3")},
		},
		{
			note:  "bad ref hitting dynamic part",
			query: `s = true; data.test.a.b.c[3].x[s][_] = _`,
			ref:   `data.test.a.b.c[3].x[s][_]`,
			pos:   7,
			have:  types.B,
			want:  types.S,
			oneOf: []Value{String("y")},
		},
		{
			note:  "bad leaf var",
			query: `x = 1; data.test.q[x]`,
			ref:   `data.test.q[x]`,
			pos:   3,
			have:  types.N,
			want:  types.S,
			oneOf: []Value{String("bar"), String("foo")},
		},
		{
			note:  "bad array index value",
			query: "arr = [[1,2],[3],[4]]; arr[0].dead.beef = x",
			ref:   "arr[0].dead.beef",
			pos:   2,
			want:  types.N,
		},
		{
			note:  "bad set element value",
			query: `s = {{1,2},{3,4}}; x = {1,2}; s[x].deadbeef`,
			ref:   "s[x].deadbeef",
			pos:   2,
			want:  types.N,
		},
		{
			note:  "bad object key value",
			query: `arr = [{"a": 1, "c": 3}, {"b": 2}]; arr[0].b`,
			ref:   "arr[0].b",
			pos:   2,
			want:  types.S,
			oneOf: []Value{String("a"), String("c")},
		},
		{
			// NOTE(sr): Thins one and the next are special: it cannot work with ref heads, either, since we need at
			// least ONE string term after data.test: a module needs a package line, and the shortest head ref
			// possible is thus data.x.y.
			note:  "bad non-leaf value",
			query: `data.test[1]`,
			ref:   "data.test[1]",
			pos:   2,
			have:  types.N,
			want:  types.S,
			oneOf: []Value{String("a"), String("p"), String("q")},
		},
		{
			note:  "bad non-leaf value (package)", // See note above ^^
			query: `data[1]`,
			ref:   "data[1]",
			pos:   1,
			have:  types.N,
			want:  types.S,
			oneOf: []Value{String("test")},
		},
		{
			note:  "composite ref operand",
			query: `data.test.q[[1, 2]]`,
			ref:   "data.test.q[[1, 2]]",
			pos:   3,
			have:  types.NewArray([]types.Type{types.N, types.N}, nil),
			want:  types.S,
		},
		{
			note:  "composite ref type error 1",
			query: `a = {[1], [2], [3]}; a[["foo"]]`,
			ref:   `a[["foo"]]`,
			pos:   1,
			have:  types.NewArray([]types.Type{types.S}, nil),
			want:  types.NewArray([]types.Type{types.N}, nil),
		},
		{
			note:  "composite ref type error 2",
			query: `a = {{"a": 2}}; a[{"a": "foo"}]`,
			ref:   `a[{"a": "foo"}]`,
			pos:   1,
			have:  types.NewObject([]*types.StaticProperty{types.NewStaticProperty("a", types.S)}, nil),
			want:  types.NewObject([]*types.StaticProperty{types.NewStaticProperty("a", types.N)}, nil),
		},
		{
			note:  "composite ref type error 3 - array",
			query: `a = [1,2,3]; a[{}] = b`,
			ref:   `a[{}]`,
			pos:   1,
			have:  types.NewObject(nil, types.NewDynamicProperty(types.A, types.A)),
			want:  types.N,
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {

			_, errs := newTypeChecker().CheckBody(env, MustParseBody(tc.query))
			if len(errs) != 1 {
				t.Fatalf("Expected exactly one error but got: %v", errs)
			}

			details, ok := errs[0].Details.(*RefErrInvalidDetail)
			if !ok {
				t.Fatalf("Expected ref error invalid but got: %v", errs)
			}

			wantRef := MustParseRef(tc.ref)

			if details.Pos != tc.pos ||
				!details.Ref.Equal(wantRef) ||
				types.Compare(details.Want, tc.want) != 0 ||
				types.Compare(details.Have, tc.have) != 0 ||
				!reflect.DeepEqual(details.OneOf, tc.oneOf) {
				t.Fatalf("Expected (%v, %v, %v, %v, %v) but got: (%v, %v, %v, %v, %v)", wantRef, tc.pos, tc.have, tc.want, tc.oneOf, details.Ref, details.Pos, details.Have, details.Want, details.OneOf)
			}
		})
	}
}

func TestFunctionsTypeInference(t *testing.T) {
	functions := []string{
		`foo([a, b]) = y { split(a, b, y) }`,
		`bar(x) = y { count(x, y) }`,
		`baz([x, y]) = z { sprintf("%s%s", [x, y], z) }`,
		`qux({"bar": x, "foo": y}) = {a: b} { upper(y, a); json.unmarshal(x, b) }`,
		`corge(x) = y { qux({"bar": x, "foo": x}, a); baz([a["{5: true}"], "BUZ"], y) }`,
	}
	body := strings.Join(functions, "\n")
	base := fmt.Sprintf("package base\n%s", body)

	c := NewCompiler()
	if c.Compile(map[string]*Module{"base": MustParseModule(base)}); c.Failed() {
		t.Fatalf("Failed to compile base module: %v", c.Errors)
	}

	tests := []struct {
		body    string
		wantErr bool
	}{
		{
			`fn(_) = y { data.base.foo(["hello", 5], y) }`,
			true,
		},
		{
			`fn(_) = y { data.base.foo(["hello", "ll"], y) }`,
			false,
		},
		{
			`fn(_) = y { data.base.baz(["hello", "ll"], y) }`,
			false,
		},
		{
			`fn(_) = y { data.base.baz([5, ["foo", "bar", true]], y) }`,
			false,
		},
		{
			`fn(_) = y { data.base.baz(["hello", {"a": "b", "c": 3}], y) }`,
			false,
		},
		{
			`fn(_) = y { data.base.corge("this is not json", y) }`,
			false,
		},
		{
			`fn(x) = y { data.non_existent(x, a); y = a[0] }`,
			true,
		},
		{
			`fn(x) = y { y = [x] }`,
			false,
		},
		{
			`f(x) = y { [x] = y }`,
			false,
		},
		{
			`fn(x) = y { y = {"k": x} }`,
			false,
		},
		{
			`f(x) = y { {"k": x} = y }`,
			false,
		},
		{
			`p { [data.base.foo] }`,
			true,
		},
		{
			`p { x = data.base.foo }`,
			true,
		},
		{
			`p { data.base.foo(data.base.bar) }`,
			true,
		},
	}

	for n, test := range tests {
		t.Run(fmt.Sprintf("Test Case %d", n), func(t *testing.T) {
			mod := MustParseModule(fmt.Sprintf("package test\n%s", test.body))
			c := NewCompiler()
			c.Compile(map[string]*Module{"base": MustParseModule(base), "mod": mod})
			if test.wantErr && !c.Failed() {
				t.Errorf("Expected error but got success")
			} else if !test.wantErr && c.Failed() {
				t.Errorf("Expected success but got error: %v", c.Errors)
			}
		})
	}
}

func TestFunctionTypeInferenceUnappliedWithObjectVarKey(t *testing.T) {

	// Run type inference on a function that constructs an object with a key
	// from args in the head.
	module := MustParseModule(`
		package test

		f(x) = y { y = {x: 1} }
	`)

	elems := []util.T{
		module.Rules[0],
	}

	env, err := newTypeChecker().CheckTypes(newTypeChecker().Env(BuiltinMap), elems, nil)

	if len(err) > 0 {
		t.Fatal(err)
	}

	// Check inferred type for reference to function.
	tpe := env.Get(MustParseRef("data.test.f"))
	exp := types.NewFunction([]types.Type{types.A}, types.NewObject(nil, types.NewDynamicProperty(types.A, types.N)))

	if types.Compare(tpe, exp) != 0 {
		t.Fatalf("Expected %v but got %v", exp, tpe)
	}
}

func TestCheckValidErrors(t *testing.T) {

	module := MustParseModule(`
		package test

		p {
			concat("", 1)  # type error
		}

		q {
			r(1)
		}

		r(x) = x`)

	module2 := MustParseModule(`
		package test

		b {
			a(1)		# call erroneous function
		}

		a(x) {
			max("foo")  # max requires an array
		}

		m {
			1 / "foo"	# type error
		}

		n {
			m			# call erroneous rule
		}`)

	module3 := MustParseModule(`
		package test

		x := {"a" : 1}

		y {
			z
		}

		z {
			x[1] == 1	# undefined reference error
		}`)

	tests := map[string]struct {
		module *Module
		numErr int
		query  []string
	}{
		"single_type_error":         {module: module, numErr: 1, query: []string{`data.test.p`}},
		"multiple_type_error":       {module: module2, numErr: 2, query: []string{`data.test.a`, `data.test.m`}},
		"undefined_reference_error": {module: module3, numErr: 1, query: []string{`data.test.z`}},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			c := NewCompiler()
			c.Compile(map[string]*Module{"test": tc.module})

			if !c.Failed() {
				t.Errorf("Expected error but got success")
			}

			if len(c.Errors) != tc.numErr {
				t.Fatalf("Expected %v error(s) but got: %v", tc.numErr, c.Errors)
			}

			// check type of the rule/function that contains an error
			for _, q := range tc.query {
				tpe := c.TypeEnv.Get(MustParseRef(q))

				if types.Compare(tpe, types.NewAny()) != 0 {
					t.Fatalf("Expected Any type but got %v", tpe)
				}
			}
		})
	}
}

func TestCheckErrorDetails(t *testing.T) {

	tests := []struct {
		detail   ErrorDetails
		expected []string
	}{
		{
			detail: &RefErrUnsupportedDetail{
				Ref:  MustParseRef("data.foo[x]"),
				Pos:  1,
				Have: types.N,
			},
			expected: []string{
				"data.foo[x]",
				"^^^^^^^^",
				"have: number",
			},
		},
		{
			detail: &RefErrInvalidDetail{
				Ref:  MustParseRef("data.foo[x]"),
				Pos:  2,
				Have: types.N,
				Want: types.S,
			},
			expected: []string{
				"data.foo[x]",
				"         ^",
				"         have (type): number",
				"         want (type): string",
			},
		},
		{
			detail: &RefErrInvalidDetail{
				Ref:  MustParseRef("data.foo[100]"),
				Pos:  2,
				Want: types.S,
				OneOf: []Value{
					String("a"),
					String("b"),
				},
			},
			expected: []string{
				"data.foo[100]",
				"         ^",
				"         have: 100",
				`         want (one of): ["a" "b"]`,
			},
		},
		{
			detail: &ArgErrDetail{
				Have: []types.Type{
					types.N,
					types.S,
				},
				Want: types.FuncArgs{
					Args: []types.Type{
						types.S,
						types.S,
					},
				},
			},
			expected: []string{
				"have: (number, string)",
				"want: (string, string)",
			},
		},
	}

	for _, tc := range tests {
		if !reflect.DeepEqual(tc.detail.Lines(), tc.expected) {
			t.Errorf("Expected %v for %v but got: %v", tc.expected, tc.detail, tc.detail.Lines())
		}
	}
}

func TestCheckErrorOrdering(t *testing.T) {

	mod := MustParseModule(`
		package test

		q = true

		p { data.test.q = 1 }  # type error: bool = number
		p { data.test.q = 2 }  # type error: bool = number
	`)

	input := make([]util.T, len(mod.Rules))
	inputReversed := make([]util.T, len(mod.Rules))

	for i := range input {
		input[i] = mod.Rules[i]
		inputReversed[i] = mod.Rules[i]
	}

	tmp := inputReversed[1]
	inputReversed[1] = inputReversed[2]
	inputReversed[2] = tmp

	_, errs1 := newTypeChecker().CheckTypes(nil, input, nil)
	_, errs2 := newTypeChecker().CheckTypes(nil, inputReversed, nil)

	if errs1.Error() != errs2.Error() {
		t.Fatalf("Expected error slices to be equal. errs1:\n\n%v\n\nerrs2:\n\n%v\n\n", errs1, errs2)
	}
}

func TestRewrittenVarsInErrors(t *testing.T) {

	_, errs := newTypeChecker().WithVarRewriter(rewriteVarsInRef(map[Var]Var{
		"__local0__": "foo",
		"__local1__": "bar",
	})).CheckBody(nil, MustParseBody(`__local0__ = [[1]]; __local1__ = "bar"; __local0__[0][__local1__]`))

	if len(errs) != 1 {
		t.Fatal("expected exactly one error but got:", len(errs))
	}

	detail, ok := errs[0].Details.(*RefErrInvalidDetail)
	if !ok {
		t.Fatal("expected invalid ref detail but got:", errs[0].Details)
	}

	if !detail.Ref.Equal(MustParseRef("foo[0][bar]")) {
		t.Fatal("expected ref to be foo[0][bar] but got:", detail.Ref)
	}

}

func newTestEnv(rs []string) *TypeEnv {
	module := MustParseModule(`
		package test
	`)

	// We preallocate enough for at least the base rules.
	// Else cases will cause reallocs, but that's okay.
	elems := make([]util.T, 0, len(rs))

	for i := range rs {
		rule := MustParseRule(rs[i])
		rule.Module = module
		elems = append(elems, rule)
		for next := rule.Else; next != nil; next = next.Else {
			next.Module = module
			elems = append(elems, next)
		}
	}

	env, err := newTypeChecker().CheckTypes(newTypeChecker().Env(BuiltinMap), elems, nil)
	if len(err) > 0 {
		panic(err)
	}

	return env
}

const inputSchema = `{
	"$schema": "http://json-schema.org/draft-07/schema",
	"$id": "http://example.com/example.json",
	"type": "object",
	"title": "The root schema",
	"description": "The root schema comprises the entire JSON document.",
	"default": {},
	"examples": [
		{
			"user": "alice",
			"operation": "write"
		}
	],
	"required": [
		"user",
		"operation"
	],
	"properties": {
		"user": {
			"$id": "#/properties/user",
			"type": "string",
			"title": "The user schema",
			"description": "An explanation about the purpose of this instance.",
			"default": "",
			"examples": [
				"alice"
			]
		},
		"operation": {
			"$id": "#/properties/operation",
			"type": "string",
			"title": "The operation schema",
			"description": "An explanation about the purpose of this instance.",
			"default": "",
			"examples": [
				"write"
			]
		}
	},
	"additionalProperties": true
}`

const inputSchema2 = `{
    "$schema": "http://json-schema.org/draft-07/schema",
    "$id": "http://example.com/example.json",
    "type": "object",
    "title": "The root schema",
    "description": "The root schema comprises the entire JSON document.",
    "default": {},
    "examples": [
        {
            "operation": "read"
        }
    ],
    "required": [
        "operation"
    ],
    "properties": {
        "operation": {
            "$id": "#/properties/operation",
            "type": "string",
            "title": "The operation schema",
            "description": "An explanation about the purpose of this instance.",
            "default": "",
            "examples": [
                "read"
            ]
        }
    },
    "additionalProperties": true
}`

const dataSchema = `{
    "$schema": "http://json-schema.org/draft-07/schema",
    "$id": "http://example.com/example.json",
    "type": "object",
    "title": "The root schema",
    "description": "The root schema comprises the entire JSON document.",
    "default": {},
    "examples": [
        {
            "alice": [
                "read",
                "write"
            ],
            "bob": [
                "read"
            ]
        }
    ],
    "required": [
        "alice",
        "bob"
    ],
    "properties": {
        "alice": {
            "$id": "#/properties/alice",
            "type": "array",
            "title": "The alice schema",
            "description": "An explanation about the purpose of this instance.",
            "default": [],
            "examples": [
                [
                    "read",
                    "write"
                ]
            ],
            "additionalItems": false,
            "items": {
                "$id": "#/properties/alice/items",
                "type": "string",
                "title": "The items schema",
                "description": "An explanation about the purpose of this instance.",
                "default": "",
                "examples": [
                    [
                        "read",
                        "write"
                    ]
                ]
            }
        },
        "bob": {
            "$id": "#/properties/bob",
            "type": "array",
            "title": "The bob schema",
            "description": "An explanation about the purpose of this instance.",
            "default": [],
            "examples": [
                [
                    "read"
                ]
            ],
            "additionalItems": false,
            "items": {
                "$id": "#/properties/bob/items",
                "type": "string",
                "title": "The items schema",
                "description": "An explanation about the purpose of this instance.",
                "default": "",
                "examples": [
                    [
                        "read"
                    ]
                ]
            }
        }
    },
    "additionalProperties": true
}`

func TestCheckAnnotationRules(t *testing.T) {

	ischema := util.MustUnmarshalJSON([]byte(inputSchema))
	ischema2 := util.MustUnmarshalJSON([]byte(inputSchema2))
	dschema := util.MustUnmarshalJSON([]byte(dataSchema))
	numberSchema := util.MustUnmarshalJSON([]byte(`{"type": "number"}`))
	stringSchema := util.MustUnmarshalJSON([]byte(`{"type": "string"}`))

	module1 := `
package policy

import data.acl
import input

default allow = false

# METADATA
# scope: rule
# schemas:
#   - input: schema["input"]
#   - data.acl: schema["acl-schema"]
allow {
		access = data.acl[input.user]
		access[_] == input.operation
}`

	module2 := `
package policy

import data.acl
import input

default allow = false

# METADATA
# scope: rule
# schemas:
#   - input: schema["input"]
#   - input: schema["whocan-input-schema"]
#   - data.acl: schema["acl-schema"]
whocan[user] {
		access = acl[user]
		access[_] == input.operation
}`

	module3 := `
package policy

import data.acl
import input

default allow = false

# METADATA
# scope: rule
# schemas:
#   - input: schema["input"]
#   - input: schema["whocan-input-schema"]
#   - data.acl: schema["acl-schema"]
allow {
	access = data.acl[input.user]
	access[_] == input.operation
}`

	module4 := `
package policy

import data.acl
import input

default allow = false

# METADATA
# scope: rule
# schemas:
#   - input: schema.missing
whocan[user] {
		access = acl[user]
		access[_] == input.operation
}`

	module8 := `
package policy

import data.acl
import input

default allow = false

# METADATA
# scope: rule
# schemas:
#   - input: schema["input"]
#   - data.acl: schema["acl-schema"]
#   - input.apple.orange: schema["input"]
allow {
		access = data.acl[input.user]
		access[_] == input.operation
		input.apple.banana
}`

	module9 := `
package policy

import data.acl
import input

default allow = false

# METADATA
# scope: rule
# schemas:
#   - input: schema["input"]
#   - data.acl: schema["acl-schema"]
#   - input.apple.orange: schema["input"]
#   - input.apple.orange.banana: schema["acl-schema"]
allow {
		access = data.acl[input.user]
		access[_] == input.operation
		input.apple.orange.banana
}`

	module10 := `
package policy

import data.acl
import input

default allow = false

# METADATA
# scope: rule
# schemas:
#   - input: schema["input"]
#   - input.apple.orange: schema["input"]
#   - input.apple.orange.banana: schema["acl-schema"]
allow {
		access = data.acl[input.user]
		access[_] == input.operation
		input.apple.orange.banana.fruit
}`

	module11 := `
package policy

import data.acl
import input

default allow = false

# METADATA
# scope: rule
# schemas:
#   - input: schema["input"]
#   - input.apple.orange: schema["input"]
#   - input.apple.orange: schema["acl-schema"]
allow {
		access = data.acl[input.user]
		access[_] == input.operation
		input.apple.orange.bob
		input.apple.orange.user
}`

	module12 := `
package policy

import data.acl
import input

default allow = false

# METADATA
# scope: rule
# schemas:
#   - input: schema["input"]
#   - input: schema["acl-schema"]
allow {
		access = data.acl[input.user]
}`

	module13 := `
package policy

import data.acl
import input

default allow = false

# METADATA
# scope: rule
# schemas:
#   - input: schema["input"]
#   - input.apple["orange"]: schema["input"]
allow {
		access = data.acl[input.user]
		input.apple.orange.fruit
}`

	module14 := `
package policy

import data.acl
import input

# METADATA
# scope: rule
# schemas:
#   - input.request.object: schema["acl-schema"]
deny[msg] {
	input.request.kind.kind == "Pod"
	image := input.request.object.spec.typo.containers[_].image
	not startswith(image, "hooli.com/")
}`

	module15 := `
package policy

import data.acl
import input

default allow = false

# METADATA
# scope: rule
# schemas:
#   -  data.acl: schema["acl-schema"]
allow {
		access = data.acl[input.user]
		access[_] == input.operation
		input.apple.orange.banana
}`

	module16 := `
package policy

import data.acl
import input

# METADATA
# scope: rule
# schemas:
#   - data.acl: schema["acl-schema"]
deny[msg] {
	input.request.kind.kinds == "Pod"
	image := input.request.object.spec.containers[_].image
	not startswith(image, "hooli.com/")
	data.blah
}`

	module17 := `
package policy

import data.acl
import input

default allow = false

# METADATA
# scope: rule
# schemas:
#   - input: schema["acl-schema"]
allow {
		input.alice
}

deny[msg] {
	input.foo
}`

	module18 := `
package policy

import data.acl
import input

# METADATA
# scope: rule
# schemas:
#   - input: schema["input"]
#   - input.apple.banana: schema["input"]
deny[msg] {
	input.apple.banana
}

deny1[msg] {
	input.apple.banana.foo
}`

	module19 := `
package policy

import data.acl
import input

default allow = false

# METADATA
# scope: rule
# schemas:
#   - data.acl: schema["acl-schema"]
#   - data.acl.foo: schema["input"]
allow {
		access = data.acl[input.user]
		access[_] == input.operation
		input.apple.orange.banana
		data.acl.foo.blah
}`

	module20 := `
package policy

import data.acl
import input

default allow = false

# METADATA
# scope: rule
# schemas:
#   - input: schema["input"]
#   - data.acl: schema["acl-schema"]
allow {
		access = data.acl[input.user]
		access[_] == input.operation
		data.acl.foo
}`

	module21 := `
package policy

import data.acl
import input

default allow = false

# METADATA
# scope: rule
# schemas:
#   - input: schema["input"]
#   - data.acl: schema["acl-schema"]
allow {
		access = data.acl[input.user]
		access[_] == input.operation
}

# METADATA for whocan rule
# scope: rule
# schemas:
#   - input: schema["whocan-input-schema"]
#   - data.acl: schema["acl-schema"]
whocan[user] {
		access = acl[user]
		access[_] == input.operation
}`

	module22 := `
package policy

import data.acl
import input

default allow = false

# METADATA
# scope: rule
# schemas:
#   - input: schema["input"]
#   - data.acl: schema["acl-schema"]
allow {
		access = data.acl[input.user]
		access[_] == input.operation
		data.foo
		data.acl.foo
}

# METADATA for whocan rule
# scope: rule
# schemas:
#   - input: schema["whocan-input-schema"]
#   - data.acl: schema["acl-schema"]
whocan[user] {
		access = acl[user]
		access[_] == input.operation.foo

}`

	schemaSet := NewSchemaSet()
	schemaSet.Put(MustParseRef("schema.number"), numberSchema)
	schemaSet.Put(MustParseRef("schema.string"), stringSchema)
	schemaSet.Put(MustParseRef("schema.input"), ischema)
	schemaSet.Put(MustParseRef(`schema["whocan-input-schema"]`), ischema2)
	schemaSet.Put(MustParseRef(`schema["acl-schema"]`), dschema)

	tests := []struct {
		note    string
		modules []string
		err     string
	}{
		{note: "data and input annotations", modules: []string{module1}},
		{note: "correct data override", modules: []string{module2}},
		{note: "incorrect data override", modules: []string{module3}, err: "undefined ref: input.user"},
		{note: "missing schema", modules: []string{module4}, err: "undefined schema: schema.missing"},
		{note: "overriding ref with length greater than one and not existing", modules: []string{module8}, err: "undefined ref: input.apple.banana"},
		{note: "overriding ref with length greater than one and existing prefix", modules: []string{module9}},
		{note: "overriding ref with length greater than one and existing prefix with type error", modules: []string{module10}, err: "undefined ref: input.apple.orange.banana.fruit"},
		{note: "overriding ref with length greater than one and existing ref", modules: []string{module11}, err: "undefined ref: input.apple.orange.user"},
		{note: "overriding ref of size one", modules: []string{module12}, err: "undefined ref: input.user"},
		{note: "overriding annotation written with brackets", modules: []string{module13}, err: "undefined ref: input.apple.orange.fruit"},
		{note: "overriding strict", modules: []string{module14}, err: "undefined ref: input.request.object.spec.typo"},
		{note: "data annotation but no input schema", modules: []string{module15}},
		{note: "data schema annotation does not overly restrict data expression", modules: []string{module16}},
		{note: "correct defer annotation on another rule has no effect base case", modules: []string{module17}},
		{note: "correct defer annotation on another rule has no effect", modules: []string{module18}},
		{note: "overriding ref with data prefix", modules: []string{module19}, err: "data.acl.foo.blah"},
		{note: "data annotation type error", modules: []string{module20}, err: "data.acl.foo"},
		{note: "more than one rule with metadata", modules: []string{module21}},
		{note: "more than one rule with metadata with type error", modules: []string{module22}, err: "undefined ref"},
		{note: "document scope", err: "test1.rego:8: rego_type_error: match error", modules: []string{`package test
# METADATA
# scope: document
# schemas:
# - input.foo: schema.number
p { input.foo = 7 }

p { input.foo = [] }`}},

		{note: "rule scope overrides document scope", modules: []string{`package test

# METADATA
# scope: document
# schemas:
# - input.foo: schema.number
p { input.foo = 7 }

# METADATA
# scope: rule
# schemas:
# - input.foo: schema.string
p { input.foo = "str" }`}},

		{note: "rule scope merges with document scope", err: "test1.rego:15: rego_type_error: match error", modules: []string{`package test

# METADATA
# scope: document
# schemas:
# - input.bar: schema.number
p { input.bar = 7 }

# METADATA
# scope: rule
# schemas:
# - input.foo: schema.string
p {
	input.foo = "str"
	input.bar = "str"
}`}},

		{note: "document scope conflict", err: "test1.rego:9: rego_type_error: document annotation redeclared: test1.rego:3", modules: []string{`package test

# METADATA
# scope: document
# schemas:
# - input.foo: schema.number
p { input.foo = 7 }

# METADATA
# scope: document
# schemas:
# - input.foo: schema.string
p { input.foo = "str" }`}},

		{note: "package scope in other module", modules: []string{`# METADATA
# scope: package
# schemas:
# - input.foo: schema.number
package test`, `package test

p { input.foo = 7 }`}},

		{note: "package scope in other module type conflict", err: "test2.rego:3: rego_type_error: match error", modules: []string{`# METADATA
# scope: package
# schemas:
# - input.foo: schema.string
package test`, `package test

p { input.foo = 7 }`}},

		{note: "package scope conflict", err: "test2.rego:1: rego_type_error: package annotation redeclared: test1.rego:1", modules: []string{`# METADATA
# scope: package
# schemas:
# - input.foo: schema.string
package test`, `# METADATA
# scope: package
# schemas:
# - input.foo: schema.number
package test

p { input.foo = 7 }`}},

		{note: "subpackages scope", err: "test1.rego:7: rego_type_error: match error", modules: []string{`# METADATA
# scope: subpackages
# schemas:
# - input: schema.number
package test

p { input = "str" }`}},

		{note: "document scope overrides subpackages scope", modules: []string{`# METADATA
# scope: subpackages
# schemas:
# - input: schema.number
package test

# METADATA
# scope: document
# schemas:
# - input: schema.string
p { input = "str" }`}},

		{note: "document scope overrides subpackages scope and finds error", err: "test1.rego:11: rego_type_error: match error", modules: []string{`# METADATA
# scope: subpackages
# schemas:
# - input: schema.string
package test

# METADATA
# scope: rule
# schemas:
# - input: schema.number
p { input = "str" }`}},

		{note: "package scope", err: "test1.rego:7: rego_type_error: match error", modules: []string{`# METADATA
# scope: package
# schemas:
# - input: schema.string
package test

p { input = 7 }`}},

		{note: "rule scope overrides package scope", modules: []string{`# METADATA
# scope: package
# schemas:
# - input: schema.string
package test

# METADATA
# scope: rule
# schemas:
# - input: schema.number
p { input = 7 }`}},

		{note: "inline definition", err: "test1.rego:7: rego_type_error: match error", modules: []string{`package test

# METADATA
# scope: rule
# schemas:
# - input: {"type": "string"}
p { input = 7 }`}},
		{note: "document scope is unordered", err: "test1.rego:3: rego_type_error: match error", modules: []string{`package test

p { input = 7 }

# METADATA
# scope: document
# schemas:
# - input: schema.string
p { input = "foo" }`}},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			var modules []*Module
			var elems []util.T

			for i, module := range tc.modules {
				mod, err := ParseModuleWithOpts(fmt.Sprintf("test%d.rego", i+1), module, ParserOptions{
					ProcessAnnotation: true,
				})
				if err != nil {
					t.Fatal(err)
				}
				modules = append(modules, mod)

				for _, rule := range mod.Rules {
					elems = append(elems, rule)
					for next := rule.Else; next != nil; next = next.Else {
						elems = append(elems, next)
					}
				}
			}

			oldTypeEnv := newTypeChecker().WithSchemaSet(schemaSet).Env(BuiltinMap)
			as, errors := BuildAnnotationSet(modules)
			typeenv, checkErrors := newTypeChecker().WithSchemaSet(schemaSet).CheckTypes(oldTypeEnv, elems, as)
			errors = append(errors, checkErrors...)
			if len(errors) > 0 {
				for _, e := range errors {
					if tc.err == "" || !strings.Contains(e.Error(), tc.err) {
						t.Fatalf("Unexpected check rule error when processing annotations: %v", e)
					}
				}
				return
			} else if tc.err != "" {
				t.Fatalf("Expected error %q but got success", tc.err)
			}

			if oldTypeEnv.tree.children != nil && typeenv.next.tree.children != nil && (typeenv.next.tree.children.Len() != oldTypeEnv.tree.children.Len()) {
				t.Fatalf("Unexpected type env")
			}

		})
	}
}

func TestCheckAnnotationInference(t *testing.T) {

	tests := []struct {
		note    string
		modules map[string]string
		schemas map[string]string
		exp     map[string]types.Type
	}{
		{
			note: "rule scope",
			modules: map[string]string{
				"test.rego": `
package test

# METADATA
# scope: rule
# schemas:
# - input: schema.foo
p = x { input = x }

q = p`,
			},
			schemas: map[string]string{
				"schema.foo": `{"type": "number"}`,
			},
			exp: map[string]types.Type{
				"data.test.p": types.N,
				"data.test.q": types.N,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {

			modules := map[string]*Module{}
			for k, v := range tc.modules {
				var err error
				modules[k], err = ParseModuleWithOpts(k, v, ParserOptions{ProcessAnnotation: true})
				if err != nil {
					t.Fatal(err)
				}

				ss := NewSchemaSet()
				for k, v := range tc.schemas {

					ref := MustParseRef(k)
					var schema interface{}
					err = util.Unmarshal([]byte(v), &schema)
					if err != nil {
						t.Fatal(err)
					}

					ss.Put(ref, schema)
				}

				compiler := NewCompiler().
					WithSchemas(ss).
					WithUseTypeCheckAnnotations(true)
				compiler.Compile(modules)
				if compiler.Failed() {
					t.Fatal("unexpected error:", compiler.Errors)
				}

				for k, v := range tc.exp {
					ref := MustParseRef(k)
					result := compiler.TypeEnv.Get(ref)
					if types.Compare(result, v) != 0 {
						t.Errorf("expected %v => %v but got %v", ref, v, result)
					}
				}
			}

		})
	}

}

func TestRemoteSchema(t *testing.T) {
	schema := `{"type": "boolean"}`

	schemaCalled := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		schemaCalled = true
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(schema))
	}))
	defer server.Close()

	policy := fmt.Sprintf(`
package test

# METADATA
# schemas:
# - input: {$ref: "%s"}
p {
	input == 42
}`, server.URL)

	module, err := ParseModuleWithOpts("policy.rego", policy, ParserOptions{ProcessAnnotation: true})
	if err != nil {
		t.Fatal(err)
	}
	modules := map[string]*Module{"policy.rego": module}

	compiler := NewCompiler().
		WithUseTypeCheckAnnotations(true)
	compiler.Compile(modules)

	if !compiler.Failed() {
		t.Fatal("expected error, got none")
	}

	expectedTypeError := "rego_type_error: match error"
	if !strings.Contains(compiler.Errors.Error(), expectedTypeError) {
		t.Fatalf("expected error:\n\n%s\n\ngot:\n\n%s",
			expectedTypeError, compiler.Errors.Error())
	}

	if !schemaCalled {
		t.Fatal("expected schema server to be called, was not")
	}
}

func TestRemoteSchemaHostNotAllowed(t *testing.T) {
	capabilities := CapabilitiesForThisVersion()
	capabilities.AllowNet = []string{}
	schema := `{"type": "boolean"}`

	schemaCalled := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		schemaCalled = true
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(schema))
	}))
	defer server.Close()

	policy := fmt.Sprintf(`
package test

# METADATA
# schemas:
# - input: {$ref: "%s"}
p {
	input == 42
}`, server.URL)

	module, err := ParseModuleWithOpts("policy.rego", policy, ParserOptions{ProcessAnnotation: true})
	if err != nil {
		t.Fatal(err)
	}
	modules := map[string]*Module{"policy.rego": module}

	compiler := NewCompiler().
		WithUseTypeCheckAnnotations(true).
		WithCapabilities(capabilities)
	compiler.Compile(modules)

	if !compiler.Failed() {
		t.Fatal("expected error, got none")
	}

	expectedTypeError := "rego_type_error: unable to compile the schema: remote reference loading disabled"
	if !strings.Contains(compiler.Errors.Error(), expectedTypeError) {
		t.Fatalf("expected error:\n\n%s\n\ngot:\n\n%s",
			expectedTypeError, compiler.Errors.Error())
	}

	if schemaCalled {
		t.Fatal("expected schema server to not be called, was")
	}
}
