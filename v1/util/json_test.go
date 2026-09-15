// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package util_test

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/util"
)

func TestInvalidJSONInput(t *testing.T) {
	cases := [][]byte{
		[]byte("{ \"k\": 1 }\n{}}"),
		[]byte("{ \"k\": 1 }\n!!!}"),
	}
	for _, tc := range cases {
		var x any
		err := util.UnmarshalJSON(tc, &x)
		if err == nil {
			t.Error("should be an error")
		}
	}
}

func TestRoundTrip(t *testing.T) {
	cases := []any{
		nil,
		1,
		1.1,
		false,
		"string",
		[]int{1},
		[]bool{true},
		[]string{"foo"},
		map[string]string{"foo": "bar"},
		struct {
			F string `json:"foo"`
			B int    `json:"bar"`
		}{"x", 32},
		map[string][]int{
			"ones": {1, 1, 1},
		},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("input %v", tc), func(t *testing.T) {
			err := util.RoundTrip(&tc)
			if err != nil {
				t.Errorf("expected error=nil, got %s", err.Error())
			}
			switch x := tc.(type) {
			// These are the output types we want, nothing else
			case nil, bool, json.Number, string, []any, []string, map[string]any, map[string]string:
			default:
				t.Errorf("unexpected type %T", x)
			}
		})
	}
}

// Regression test for a bug where RoundTrip's fallback path reused an
// existing pointer as its own decode target: opa-envoy-plugin stores a
// shared *ast.object (e.g. request metadata) directly as a value inside an
// otherwise-native map[string]any before handing it to RoundTripFast. Since
// ast.Object.MarshalJSON encodes as a [key,value] array rather than a plain
// JSON object, decoding back into the same ast.object failed with "cannot
// unmarshal array into Go value of type ast.object" -- and, had it not
// errored, would have corrupted the shared value in place.
func TestRoundTripEmbeddedASTValue(t *testing.T) {
	shared := ast.NewObject(
		[2]*ast.Term{ast.StringTerm("ext_authz"), ast.StringTerm("v3")},
		[2]*ast.Term{ast.StringTerm("encoding"), ast.StringTerm("protojson")},
	)
	before := shared.String()

	input := map[string]any{
		"method":  "GET",
		"version": ast.Value(shared),
	}

	var v any = input
	if err := util.RoundTripFast(&v); err != nil {
		t.Fatalf("RoundTripFast: unexpected error: %s", err)
	}

	if shared.String() != before {
		t.Fatalf("shared ast.Object was mutated by RoundTripFast: got %s, want %s", shared.String(), before)
	}

	out, ok := v.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", v)
	}
	if _, ok := out["version"].(ast.Value); ok {
		t.Errorf("expected version to no longer be the original ast.Value pointer, got %T", out["version"])
	}
	if _, err := ast.InterfaceToValue(out); err != nil {
		t.Errorf("result does not convert back to an ast.Value: %s", err)
	}

	// A bare ast.Value at the top level must round-trip too.
	var top any = shared
	if err := util.RoundTrip(&top); err != nil {
		t.Fatalf("RoundTrip: unexpected error: %s", err)
	}
}

func TestRoundTripFastMatchesRoundTrip(t *testing.T) {
	type tagged struct {
		Foo string `json:"baz"`
	}

	cases := []any{
		nil,
		1,
		1.1,
		false,
		"string",
		json.Number("42"),
		[]int{1},
		[]bool{true},
		[]string{"foo"},
		map[string]string{"foo": "bar"},
		map[string][]int{
			"ones": {1, 1, 1},
		},
		tagged{Foo: "bar"},
		map[string]any(nil),
		[]any(nil),
		map[string]any{},
		[]any{},
		map[string]any{
			"str":  "hello",
			"bool": true,
			"num":  json.Number("123"),
			"nil":  nil,
			"arr":  []any{"a", json.Number("1"), map[string]any{"nested": "b"}},
			"obj": map[string]any{
				"deep":                map[string]any{"deeper": []any{json.Number("1"), json.Number("2")}},
				"nil_map":             map[string]any(nil),
				"nil_arr":             []any(nil),
				"raw_ints":            []any{1, 2, 3},
				"tagged_struct":       tagged{Foo: "bar"},
				"tagged_struct_slice": []any{tagged{Foo: "bar"}},
			},
		},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("input %v", tc), func(t *testing.T) {
			want := tc
			if err := util.RoundTrip(&want); err != nil {
				t.Fatalf("RoundTrip: unexpected error: %s", err)
			}

			got := tc
			if err := util.RoundTripFast(&got); err != nil {
				t.Fatalf("RoundTripFast: unexpected error: %s", err)
			}

			if !reflect.DeepEqual(want, got) {
				t.Errorf("RoundTripFast diverged from RoundTrip:\nRoundTrip:     %#v\nRoundTripFast: %#v", want, got)
			}
		})
	}
}

func TestRoundTripFastCyclicMap(t *testing.T) {
	m := map[string]any{}
	m["self"] = m
	v := any(m)

	err := util.RoundTripFast(&v)
	if err == nil {
		t.Fatal("expected an error for a self-referential map, got nil")
	}

	var want any = m
	wantErr := util.RoundTrip(&want)
	if wantErr == nil {
		t.Fatal("expected RoundTrip to also error on a self-referential map")
	}
}

func TestRoundTripFastCyclicSlice(t *testing.T) {
	s := make([]any, 1)
	s[0] = s
	v := any(s)

	err := util.RoundTripFast(&v)
	if err == nil {
		t.Fatal("expected an error for a self-referential slice, got nil")
	}
}

// Deeper than startDetectingCyclesAfter, but not cyclic: must not false-positive.
func TestRoundTripFastDeepNonCyclicTree(t *testing.T) {
	const depth = 2000

	var v any = "leaf"
	for range depth {
		v = map[string]any{"child": v}
	}

	if err := util.RoundTripFast(&v); err != nil {
		t.Fatalf("unexpected error on a deep non-cyclic tree: %s", err)
	}
}

func TestReference(t *testing.T) {
	cases := []any{
		nil,
		func() any { f := any(nil); return &f }(),
		1,
		func() any { f := 1; return &f }(),
		1.1,
		func() any { f := 1.1; return &f }(),
		false,
		func() any { f := false; return &f }(),
		[]int{1},
		&[]int{1},
		func() any { f := &[]int{1}; return &f }(),
		[]bool{true},
		&[]bool{true},
		func() any { f := &[]bool{true}; return &f }(),
		[]string{"foo"},
		&[]string{"foo"},
		func() any { f := &[]string{"foo"}; return &f }(),
		map[string]string{"foo": "bar"},
		&map[string]string{"foo": "bar"},
		func() any { f := &map[string]string{"foo": "bar"}; return &f }(),
		struct {
			F string `json:"foo"`
			B int    `json:"bar"`
		}{"x", 32},
		&struct {
			F string `json:"foo"`
			B int    `json:"bar"`
		}{"x", 32},
		map[string][]int{
			"ones": {1, 1, 1},
		},
		&map[string][]int{
			"ones": {1, 1, 1},
		},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("input %v", tc), func(t *testing.T) {
			ref := util.Reference(tc)
			rv := reflect.ValueOf(ref)
			if rv.Kind() != reflect.Pointer {
				t.Fatalf("expected pointer, got %v", rv.Kind())
			}
			if rv.Elem().Kind() == reflect.Pointer {
				t.Error("expected non-pointer element")
			}
		})
	}
}

// There's valid JSON that doesn't pass through yaml.YAMLToJSON.
// See https://github.com/open-policy-agent/opa/issues/4673
func TestInvalidYAMLValidJSON(t *testing.T) {
	x := []byte{0x22, 0x3a, 0xc2, 0x9a, 0x22}
	y := ""
	if err := util.Unmarshal(x, &y); err != nil {
		t.Fatal(err)
	}
}

func TestUnmarshalJSONUTF8BOM(t *testing.T) {
	bomFail := []byte{0xef, 0xbb, 0xbf, 0x22, 0x5c, 0x2f, 0x22, 0x0a} // "\/" preceded by UTF-8 BOM

	if json.Valid(bomFail) {
		t.Fatal("expected invalid JSON")
	}

	var x any
	err := util.Unmarshal(bomFail, &x)
	if err != nil {
		t.Fatal("expected BOM to be stripped", err)
	}
}

// Costs below include cost of slices.Clone which is needed since we modify in place.
// Without NeedsRoundTrip and json.Number optimizations:
// -----------------------------------------------------
// BenchmarkRoundTrip/zero-allocs-16                     596457      1797 ns/op   12250 B/op      29 allocs/op
// BenchmarkRoundTrip/less-allocs_to_json.Number-16     1000000      1187 ns/op    7398 B/op      22 allocs/op
// BenchmarkRoundTrip/full-allocs_collections-16        1410703       857 ns/op    2473 B/op      28 allocs/op
//
// With NeedsRoundTrip and json.Number optimizations:
// --------------------------------------------------
// BenchmarkRoundTrip/zero-allocs-16                    4078965     27.36 ns/op      80 B/op       1 allocs/op
// BenchmarkRoundTrip/less-allocs_to_json.Number-16    10147891     118.6 ns/op     108 B/op       7 allocs/op
// BenchmarkRoundTrip/full-allocs_collections-16        1475988       813 ns/op    2473 B/op      28 allocs/op
func BenchmarkRoundTrip(b *testing.B) {
	b.Run("zero-allocs", func(b *testing.B) {
		act := []any{nil, false, true, "string", json.Number("1")}
		exp := slices.Clone(act)

		var cpy []any

		for b.Loop() {
			cpy = slices.Clone(act)
			for i := range cpy {
				if err := util.RoundTrip(&cpy[i]); err != nil {
					b.Fatalf("expected error=nil, got %s", err.Error())
				}
			}
		}

		if !slices.Equal(exp, cpy) {
			b.Fatal("expected inputs to be unchanged")
		}
	})

	b.Run("less-allocs with cheap number to json.Number", func(b *testing.B) {
		act := []any{1.1, 1000, -22}
		exp := []any{json.Number("1.1"), json.Number("1000"), json.Number("-22")}

		var cpy []any

		for b.Loop() {
			cpy = slices.Clone(act)
			for i := range cpy {
				if err := util.RoundTrip(&cpy[i]); err != nil {
					b.Fatalf("expected error=nil, got %s", err.Error())
				}
			}
		}

		if !slices.Equal(exp, cpy) {
			b.Fatalf("unexpected: %v", cpy)
		}
	})

	b.Run("full-allocs collections", func(b *testing.B) {
		exp := []any{[]any{json.Number("1"), json.Number("2"), json.Number("3")}, map[string]any{"foo": "bar"}}
		act := []any{[]int{1, 2, 3}, map[string]string{"foo": "bar"}}

		var cpy []any

		for b.Loop() {
			cpy = slices.Clone(act)
			for i := range act {
				if err := util.RoundTrip(&cpy[i]); err != nil {
					b.Fatalf("expected error=nil, got %s", err.Error())
				}
			}
		}

		if !reflect.DeepEqual(exp, cpy) {
			b.Fatalf("expected %v, got %v", exp, cpy)
		}
	})
}

// BenchmarkRoundTripFast mirrors BenchmarkRoundTrip's scenarios, plus an
// already-native-tree case.
//
// RoundTrip on an already-native tree:
// leaves=10-16       5192 ns/op      4776 B/op       93 allocs/op
// leaves=100-16     41931 ns/op     39318 B/op      826 allocs/op
// leaves=1000-16   433611 ns/op    453315 B/op     8047 allocs/op
// leaves=10000-16 4506942 ns/op   4955688 B/op    80121 allocs/op
//
// RoundTripFast on the same tree:
// leaves=10-16       705.9 ns/op    2456 B/op       16 allocs/op
// leaves=100-16      5778 ns/op    20448 B/op      108 allocs/op
// leaves=1000-16    59980 ns/op   217601 B/op     1008 allocs/op
// leaves=10000-16  659289 ns/op  2090347 B/op    10022 allocs/op
func BenchmarkRoundTripFast(b *testing.B) {
	b.Run("zero-allocs", func(b *testing.B) {
		act := []any{nil, false, true, "string", json.Number("1")}
		exp := slices.Clone(act)

		var cpy []any

		for b.Loop() {
			cpy = slices.Clone(act)
			for i := range cpy {
				if err := util.RoundTripFast(&cpy[i]); err != nil {
					b.Fatalf("expected error=nil, got %s", err.Error())
				}
			}
		}

		if !slices.Equal(exp, cpy) {
			b.Fatal("expected inputs to be unchanged")
		}
	})

	b.Run("less-allocs to json.Number", func(b *testing.B) {
		act := []any{1.1, 1000, -22}
		exp := []any{json.Number("1.1"), json.Number("1000"), json.Number("-22")}

		var cpy []any

		for b.Loop() {
			cpy = slices.Clone(act)
			for i := range cpy {
				if err := util.RoundTripFast(&cpy[i]); err != nil {
					b.Fatalf("expected error=nil, got %s", err.Error())
				}
			}
		}

		if !slices.Equal(exp, cpy) {
			b.Fatalf("unexpected: %v", cpy)
		}
	})

	b.Run("full-allocs collections", func(b *testing.B) {
		exp := []any{[]any{json.Number("1"), json.Number("2"), json.Number("3")}, map[string]any{"foo": "bar"}}
		act := []any{[]int{1, 2, 3}, map[string]string{"foo": "bar"}}

		var cpy []any

		for b.Loop() {
			cpy = slices.Clone(act)
			for i := range act {
				if err := util.RoundTripFast(&cpy[i]); err != nil {
					b.Fatalf("expected error=nil, got %s", err.Error())
				}
			}
		}

		if !reflect.DeepEqual(exp, cpy) {
			b.Fatalf("expected %v, got %v", exp, cpy)
		}
	})

	for _, n := range []int{10, 100, 1000, 10000} {
		tree := benchNativeTree(n)

		b.Run(fmt.Sprintf("already-native tree/leaves=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				v := any(tree)
				if err := util.RoundTripFast(&v); err != nil {
					b.Fatalf("expected error=nil, got %s", err.Error())
				}
			}
		})
	}
}

func benchNativeTree(n int) map[string]any {
	arr := make([]any, 0, n/2)
	obj := make(map[string]any, n/2)
	for i := range n {
		if i%2 == 0 {
			arr = append(arr, map[string]any{"i": json.Number(strconv.Itoa(i)), "s": "value"})
		} else {
			obj[fmt.Sprintf("key%d", i)] = json.Number(strconv.Itoa(i))
		}
	}
	return map[string]any{"arr": arr, "obj": obj}
}
