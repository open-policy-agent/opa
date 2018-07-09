// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package util_test

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"github.com/open-policy-agent/opa/util"
)

func TestRoundTrip(t *testing.T) {
	cases := []interface{}{
		nil,
		1,
		1.1,
		false,
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
			case nil, bool, json.Number, int64, float64, int, string, []interface{},
				[]string, map[string]interface{}, map[string]string:
			default:
				t.Errorf("unexpected type %T", x)
			}
		})
	}
}

func TestReference(t *testing.T) {
	cases := []interface{}{
		nil,
		func() interface{} { f := interface{}(nil); return &f }(),
		1,
		func() interface{} { f := 1; return &f }(),
		1.1,
		func() interface{} { f := 1.1; return &f }(),
		false,
		func() interface{} { f := false; return &f }(),
		[]int{1},
		&[]int{1},
		func() interface{} { f := &[]int{1}; return &f }(),
		[]bool{true},
		&[]bool{true},
		func() interface{} { f := &[]bool{true}; return &f }(),
		[]string{"foo"},
		&[]string{"foo"},
		func() interface{} { f := &[]string{"foo"}; return &f }(),
		map[string]string{"foo": "bar"},
		&map[string]string{"foo": "bar"},
		func() interface{} { f := &map[string]string{"foo": "bar"}; return &f }(),
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

func TestAreEqualJSON(t *testing.T) {
	cases := []struct {
		note     string
		arg1     interface{}
		arg2     interface{}
		expected bool
	}{
		{"integer success", 1, 1, true},
		{"int fail", 1, 2, false},
		{"string success", "hi", "hi", true},
		{"string fail", "hi", "bye", false},
		{"map success", map[string]interface{}{
			"order":     4,
			"different": 1,
		}, map[string]interface{}{
			"different": 1,
			"order":     4,
		}, true},
		{"array order preserved", []int{1, 2, 3}, []int{3, 2, 1}, false},
	}

	for _, c := range cases {
		t.Run(c.note, func(t *testing.T) {
			if res := util.AreEqualJSON(c.arg1, c.arg2); res != c.expected {
				t.Fatalf("Expected: %v\nGot: %v", c.expected, res)
			}
		})
	}

}
