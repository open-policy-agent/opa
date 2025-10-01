// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package util_test

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"testing"

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
			t.Errorf("should be an error")
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
			if rv.Kind() != reflect.Ptr {
				t.Fatalf("expected pointer, got %v", rv.Kind())
			}
			if rv.Elem().Kind() == reflect.Ptr {
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
			b.Fatalf("expected inputs to be unchanged")
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
