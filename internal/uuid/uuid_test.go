// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.
package uuid

import (
	"bytes"
	"fmt"
	"reflect"
	"testing"
)

func TestUUID4(t *testing.T) {
	uuid, err := New(bytes.NewReader(make([]byte, 16)))
	if err != nil {
		t.Fatal(err)
	}
	expect := "00000000-0000-4000-8000-000000000000"
	if uuid != expect {
		t.Errorf("Expected %q, got %q", expect, uuid)
	}
}

func TestParseTrue(t *testing.T) {
	var tests = []struct {
		name  string
		input string
		ans   map[string]interface{}
	}{
		{
			"Test uuid 1",
			"c2fc67c2-47f2-11ee-b67a-9f3619c7493f",
			map[string]interface{}{
				"version":       1,
				"variant":       "RFC4122",
				"nodeid":        "9f-36-19-c7-49-3f",
				"macvariables":  "local:multicast",
				"time":          int64(1693481847404333000),
				"clocksequence": 13946},
		},
		{
			"Test uuid 2",
			"000003e8-48b9-21ee-b200-325096b39f47",
			map[string]interface{}{
				"version":       2,
				"variant":       "RFC4122",
				"nodeid":        "32-50-96-b3-9f-47",
				"macvariables":  "local:unicast",
				"time":          int64(1693566990121469600),
				"clocksequence": 12800,
				"domain":        "Person",
				"id":            1000,
			},
		},
		{
			"Test uuid 3",
			"6bea8ef2-d3d3-3cd1-84e0-9bab06a52ece",
			map[string]interface{}{
				"version": 3,
				"variant": "RFC4122",
			},
		},
		{
			"Test uuid 4",
			"00000000-0000-4000-8000-000000000000",
			map[string]interface{}{"version": 4, "variant": "RFC4122"},
		},
		{
			"Test uuid 5",
			"00000000-0000-5cd1-84e0-9bab06a52ece",
			map[string]interface{}{
				"version": 5,
				"variant": "RFC4122",
			},
		},
		{
			"Test future version and variant",
			"00000000-0000-fcd1-f4e0-9bab06a52ece",
			map[string]interface{}{
				"version": 15,
				"variant": "Future",
			},
		},
		{
			"Test urn format",
			"urn:uuid:c2fc67c2-47f2-11ee-b67a-9f3619c7493f",
			map[string]interface{}{
				"version":       1,
				"variant":       "RFC4122",
				"nodeid":        "9f-36-19-c7-49-3f",
				"macvariables":  "local:multicast",
				"time":          int64(1693481847404333000),
				"clocksequence": 13946},
		},
		{
			"Test uuid with brackets",
			"{000003e8-48b9-21ee-b200-325096b39f47}",
			map[string]interface{}{
				"version":       2,
				"variant":       "RFC4122",
				"nodeid":        "32-50-96-b3-9f-47",
				"macvariables":  "local:unicast",
				"time":          int64(1693566990121469600),
				"clocksequence": 12800,
				"domain":        "Person",
				"id":            1000,
			},
		},
		{
			"Test uuid without dashes",
			"00000000000040008000000000000000",
			map[string]interface{}{"version": 4, "variant": "RFC4122"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := Parse(tt.input)
			exp := tt.ans
			if !reflect.DeepEqual(got, exp) {
				t.Errorf("got: %v, expected: %v", got, exp)
			}
		})
	}
}

func TestParseNegative(t *testing.T) {
	length, runes := "00000000000-123-222-222", "HijVnnS1-GGGG-1234-12345678"
	_, err := Parse(length)
	if err == nil {
		t.Error("got no error, should fail since length of uuid is too long")
	}
	_, err = Parse(runes)
	if err == nil {
		t.Error("got no error, should fail since string contains other characters than hexadecimals")
	}
}

func TestMACVars(t *testing.T) {
	var inp = []byte{byte(0b11111111), byte(0b11111101), byte(0b11111110), byte(0b11111100)}
	var expected = []string{"local:multicast", "global:multicast", "local:unicast", "global:unicast"}
	for i, b := range inp {
		t.Run(fmt.Sprint("Test", i+1), func(t *testing.T) {
			got := macVars(b)
			if got != expected[i] {
				t.Errorf("got %s, expected %s", got, expected[i])
			}
		})
	}
}
