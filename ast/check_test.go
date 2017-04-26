// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"testing"

	"github.com/open-policy-agent/opa/types"
	"github.com/open-policy-agent/opa/util/test"
)

func TestCheckInference(t *testing.T) {

	// fake_builtin_1([str1,str2])
	RegisterBuiltin(&Builtin{
		Name: Var("fake_builtin_1"),
		Args: []types.Type{
			types.NewArray(
				[]types.Type{types.S, types.S}, nil,
			),
		},
		TargetPos: []int{0},
	})

	// fake_builtin_2({"a":str1,"b":str2})
	RegisterBuiltin(&Builtin{
		Name: Var("fake_builtin_2"),
		Args: []types.Type{
			types.NewObject(
				[]*types.Property{
					{"a", types.S},
					{"b", types.S},
				}, nil,
			),
		},
		TargetPos: []int{0},
	})

	// fake_builtin_3({str1,str2,...})
	RegisterBuiltin(&Builtin{
		Name: Var("fake_builtin_3"),
		Args: []types.Type{
			types.NewSet(types.S),
		},
		TargetPos: []int{0},
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
		{"empty-composites", `
				obj = {};
				arr = [];
				set = set();
				obj[i] = v1;
				arr[j] = v2;
				set[v3];
				obj = {"foo": "bar"}
				arr = [1];
				set = {1,2,3}
				`, map[Var]types.Type{
			Var("obj"): types.NewObject(nil, types.A),
			Var("i"):   types.S,
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
			Var("i"): types.NewAny(types.S, types.N),
		}},
		{"local-reference-nested", `
			a = [["foo"], 0, {"bar": "baz"}, 2];
			b = [0,1,2,3];
			a[b[_]][k] = v
			`, map[Var]types.Type{
			Var("k"): types.NewAny(types.S, types.N),
		}},
		{"simple-built-in", "x = 1 + 2", map[Var]types.Type{
			Var("x"): types.N,
		}},
		{"simple-built-in-exists", "x = 1 + 2; y = x + 2", map[Var]types.Type{
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
	}

	for _, tc := range tests {
		test.Subtest(t, tc.note, func(t *testing.T) {
			body := MustParseBody(tc.query)
			checker := newTypeChecker()
			env, err := checker.CheckBody(nil, body)
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

	tests := []struct {
		note     string
		rules    [][2]string
		ref      string
		expected types.Type
	}{
		{"trivial", ruleset1, `data.a.trivial`, types.B},

		{"complete-doc", ruleset1, `data.a.complete`, types.NewArray(
			[]types.Type{types.NewObject(
				[]*types.Property{{
					"foo", types.N,
				}},
				nil,
			)},
			nil,
		)},

		{"complete-doc-suffix", ruleset1, `data.a.complete[0].foo`, types.N},

		{"partial-set-doc", ruleset1, `data.a.partialset`, types.NewSet(
			types.NewObject(
				[]*types.Property{{
					"foo", types.S,
				}},
				nil,
			),
		)},

		{"partial-object-doc", ruleset1, "data.a.partialobj", types.NewObject(
			nil,
			types.NewObject(
				[]*types.Property{{
					"foo", types.S,
				}},
				nil,
			),
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
			types.NewAny(
				types.S,
				types.N),
		)},

		{"ref", ruleset1, "data.b.trivial_ref", types.B},

		{"ref-transitive", ruleset1, "data.b.transitive_ref", types.NewArray(
			[]types.Type{
				types.B,
			},
			nil,
		)},

		{"prefix", ruleset1, `data.prefix.a.b`, types.NewObject(
			[]*types.Property{{
				"c", types.NewObject(
					[]*types.Property{{
						"d", types.B,
					}},
					types.A,
				),
			}},
			types.A,
		)},

		// Check that prefixes that iterate fallback to any.
		{"prefix-iter", ruleset1, `data.prefix.i.j[k]`, types.A},

		// Check that iteration targetting a rule (but nonetheless prefixed) falls back to any.
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
	}

	for _, tc := range tests {
		test.Subtest(t, tc.note, func(t *testing.T) {
			rules := make([]*Rule, len(tc.rules))

			// Convert test rules into rule slice for call.
			for i := range tc.rules {
				pkg := MustParsePackage(`package ` + tc.rules[i][0])
				rule := MustParseRule(tc.rules[i][1])
				module := &Module{
					Package: pkg,
					Rules:   []*Rule{rule},
				}
				rule.Module = module
				rules[i] = rule
			}

			ref := MustParseRef(tc.ref)
			checker := newTypeChecker()
			env, err := checker.CheckRules(nil, rules)

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

func TestCheckBadBuiltin(t *testing.T) {
	body := MustParseBody(`bad_builtin()`)
	tc := newTypeChecker()
	_, err := tc.CheckBody(nil, body)
	if len(err) != 1 || err[0].Code != TypeErr {
		t.Fatalf("Expected type error from %v but got: %v", body, err)
	}
}

func TestCheckBadCardinality(t *testing.T) {
	body := MustParseBody("plus(1,2); plus(1,2,3,4)")
	tc := newTypeChecker()
	_, err := tc.CheckBody(nil, body)
	if len(err) != 2 || err[0].Code != TypeErr || err[1].Code != TypeErr {
		t.Fatalf("Expected 2 type errors from %v but got: %v", body, err)
	}
	detail, ok := err[1].Details.(*ArgErrDetail)
	if !ok {
		t.Fatalf("Expected argument error details but got: %v", err)
	}
	expected := []types.Type{
		types.N,
		types.N,
		types.N,
		types.N,
	}
	if len(expected) != len(detail.Have) {
		t.Fatalf("Expected arg types %v but got: %v", expected, detail.Have)
	}
	for i := range expected {
		if types.Compare(expected[i], detail.Have[i]) != 0 {
			t.Fatalf("Expected types for %v to be %v but got: %v", body[0], expected, detail.Have)
		}
	}
}

func TestCheckMatchErrors(t *testing.T) {
	tests := []struct {
		note  string
		query string
	}{
		{"null", "{ null = true }"},
		{"boolean", "{ true = null }"},
		{"number", "{ 1 = null }"},
		{"string", `{ "hello" = null }`},
		{"array", "{[1,2,3] = null}"},
		{"array-nested", `{[1,2,3] = [1,2,"3"]}`},
		{"array-nested-2", `{[1,2] = [1,2,3]}`},
		{"array-dynamic", `{ [ true | true ] = [x | a = [1, "foo"]; x = a[_]] }`},
		{"object", `{{"a": 1, "b": 2} = null}`},
		{"object-nested", `{ {"a": 1, "b": "2"} = {"a": 1, "b": 2} }`},
		{"object-nested-2", `{ {"a": 1} = {"a": 1, "b": "2"} }`},
		{"object-dynamic", `{ obj2 = obj1 }`},
		{"set", "{{1,2,3} = null}"},
	}

	for _, tc := range tests {
		test.Subtest(t, tc.note, func(t *testing.T) {
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
		Name: Var("fake_builtin_2"),
		Args: []types.Type{
			types.NewAny(types.NewObject(
				[]*types.Property{
					{"a", types.S},
					{"b", types.S},
				}, nil,
			), types.NewObject(
				[]*types.Property{
					{"b", types.S},
					{"c", types.S},
				}, nil,
			)),
		},
		TargetPos: []int{0},
	})

	tests := []struct {
		note  string
		query string
	}{
		{"trivial", "x = true + 1"},
		{"refs", "x = [null]; y = x[0] + 1"},
		{"array comprehensions", `sum([null | true], x)`},
		{"arrays-any", `sum([1,2,"3",4], x)`},
		{"arrays-bad-input", `contains([1,2,3], "x")`},
		{"objects-any", `fake_builtin_2({"a": a, "c": c})`},
		{"objects-bad-input", `sum({"a": 1, "b": 2}, x)`},
		{"sets-any", `sum({1,2,"3",4}, x)`},
	}

	for _, tc := range tests {
		test.Subtest(t, tc.note, func(t *testing.T) {
			body := MustParseBody(tc.query)
			checker := newTypeChecker()
			_, err := checker.CheckBody(nil, body)
			if len(err) != 1 {
				t.Fatalf("Expected exactly one error from %v but got:\n%v", body, err)
			}
		})
	}
}

func TestCheckRefErrors(t *testing.T) {

	module := MustParseModule(`
		package test
	`)

	rs := []string{
		`scalar = 1 { true }`,
		`obj = {"foo": 1, "bar": 2} { true }`,
		`nested = {"foo": [1,2,3]} { true }`,
		`set[1] { true }`,
	}

	rules := make([]*Rule, len(rs))
	for i := range rules {
		rules[i] = MustParseRule(rs[i])
		rules[i].Module = module
	}

	env, err := newTypeChecker().CheckRules(nil, rules)
	if len(err) > 0 {
		t.Fatalf("Unexpected error in rules: %v", err)
	}

	tests := []struct {
		note  string
		input string
	}{
		{"trivial", "{ data.test.obj.deadbeef }"},
		{"trivial-2", "{ data.test.obj.foo.deadbeef }"},
		{"non-leaf-var", `{ x = null; data.test[x] }`},
		{"non-leaf-ref", `{ x = [1]; data.test[x[0]] }`},
		{"non-leaf-nested-ref", `{ x = ["a"]; data.test[x[1]] }`},
		{"non-leaf-invalid", `{ data.test[100] }`},
		{"leaf-ref", `{ x = {"foo": 1}; data.test.obj[x.foo] }`},
		{"leaf-var", `{ x = 1; data.test.obj[x] }`},
		{"repeat", `{ data.test.nested[x][x] }`},
		{"scalar", `{ data.test.scalar[x] }`},
	}

	for _, tc := range tests {
		test.Subtest(t, tc.note, func(t *testing.T) {
			body := MustParseBody(tc.input)
			_, err := newTypeChecker().CheckBody(env, body)
			if len(err) != 1 {
				t.Fatalf("Expected exactly one error but got: %v", err)
			}
		})
	}
}
