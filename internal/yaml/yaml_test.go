// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package yaml

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestYAMLToJSON(t *testing.T) {
	tests := []struct {
		note   string
		yaml   string
		exp    string
		expErr string
	}{
		{
			note: "YAML 1.1 boolean spellings stay strings (issue 5754)",
			yaml: "on: push\noff: x\n",
			exp:  `{"off":"x","on":"push"}`,
		},
		{
			note: "YAML 1.1 boolean spellings as values",
			yaml: "a: yes\nb: no\nc: y\nd: n\ne: On\nf: OFF\n",
			exp:  `{"a":"yes","b":"no","c":"y","d":"n","e":"On","f":"OFF"}`,
		},
		{
			note: "YAML 1.2 booleans still resolve",
			yaml: "a: true\nb: FALSE\nc: True\n",
			exp:  `{"a":true,"b":false,"c":true}`,
		},
		{
			note: "non-string keys are stringified",
			yaml: "1: a\ntrue: b\n1.5: c\n",
			exp:  `{"1":"a","1.5":"c","true":"b"}`,
		},
		{
			note:   "null keys are rejected",
			yaml:   "null: a\n",
			expErr: "unsupported map key of type",
		},
		{
			note:   "sequence keys are rejected",
			yaml:   "? [1, 2]\n: v\n",
			expErr: "invalid map key",
		},
		{
			note: "nested collections",
			yaml: "a:\n  - on\n  - {off: 1}\n",
			exp:  `{"a":["on",{"off":1}]}`,
		},
		{
			note: "anchors and merge keys",
			yaml: "base: &b\n  on: 1\nderived:\n  <<: *b\n  y: 2\n",
			exp:  `{"base":{"on":1},"derived":{"on":1,"y":2}}`,
		},
		{
			note: "empty document",
			yaml: "",
			exp:  `null`,
		},
		{
			note: "timestamps stay strings",
			yaml: "a: 2023-01-01\nb: 2023-01-01 10:00:00\nc: 2023-01-01T10:00:00Z\n",
			exp:  `{"a":"2023-01-01","b":"2023-01-01 10:00:00","c":"2023-01-01T10:00:00Z"}`,
		},
		{
			note: "timestamps behind an alias stay strings",
			yaml: "a: &t 2023-01-01\nb: *t\n",
			exp:  `{"a":"2023-01-01","b":"2023-01-01"}`,
		},
		{
			note: "duplicate keys resolve last-wins",
			yaml: "a: 1\nb: 9\na: 2\na: 3\n",
			exp:  `{"a":3,"b":9}`,
		},
		{
			note: "duplicate keys in a nested mapping",
			yaml: "x:\n  a: 1\n  a: 2\n",
			exp:  `{"x":{"a":2}}`,
		},
		{
			note: "repeated merge keys are not deduplicated",
			yaml: "p: &p\n  x: 1\nq: &q\n  y: 2\nr:\n  <<: *p\n  <<: *q\n  z: 3\n",
			exp:  `{"p":{"x":1},"q":{"y":2},"r":{"x":1,"y":2,"z":3}}`,
		},
		{
			note: "explicit key overrides a merged one",
			yaml: "p: &p\n  x: 1\nq:\n  <<: *p\n  x: 2\n",
			exp:  `{"p":{"x":1},"q":{"x":2}}`,
		},
		{
			note: "keys colliding only after stringification are last-wins",
			yaml: "1: a\n\"1\": b\n",
			exp:  `{"1":"b"}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			bs, err := YAMLToJSON([]byte(tc.yaml))
			if tc.expErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got %s", tc.expErr, bs)
				}
				if !contains(err.Error(), tc.expErr) {
					t.Fatalf("expected error containing %q, got %v", tc.expErr, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if string(bs) != tc.exp {
				t.Fatalf("expected %s, got %s", tc.exp, bs)
			}
		})
	}
}

func TestUnmarshalUsesJSONTags(t *testing.T) {
	var x struct {
		On string `json:"on"`
	}
	if err := Unmarshal([]byte("on: push\n"), &x); err != nil {
		t.Fatal(err)
	}
	if x.On != "push" {
		t.Fatalf("expected push, got %q", x.On)
	}
}

func TestUnmarshalJSONOpt(t *testing.T) {
	var x any
	useNumber := JSONOpt(func(d *json.Decoder) *json.Decoder {
		d.UseNumber()
		return d
	})
	if err := Unmarshal([]byte("a: 1\n"), &x, useNumber); err != nil {
		t.Fatal(err)
	}
	exp := map[string]any{"a": json.Number("1")}
	if !reflect.DeepEqual(x, exp) {
		t.Fatalf("expected %#v, got %#v", exp, x)
	}
}

func TestMarshalRoundTripsThroughJSON(t *testing.T) {
	in := map[string]any{"n": json.Number("3"), "s": "on"}
	bs, err := Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	if exp := "\"n\": 3\ns: \"on\"\n"; string(bs) != exp {
		t.Fatalf("expected %q, got %q", exp, bs)
	}
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
