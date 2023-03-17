// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.
package logs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/util"
)

func TestNewMaskRule(t *testing.T) {

	tests := []struct {
		note   string
		input  *maskRule
		expErr error
		expPtr *maskRule
	}{
		{
			note: "empty",
			input: &maskRule{
				OP:   maskOPRemove,
				Path: "",
			},
			expErr: fmt.Errorf("mask must be non-empty"),
		},
		{
			note: "missing slash",
			input: &maskRule{
				OP:   maskOPRemove,
				Path: "foo",
			},
			expErr: fmt.Errorf("mask must be slash-prefixed"),
		},
		{
			note: "no prefix",
			input: &maskRule{
				OP:   maskOPRemove,
				Path: "/",
			},
			expErr: fmt.Errorf("mask prefix not allowed"),
		},
		{
			note: "bad prefix key",
			input: &maskRule{
				OP:   maskOPRemove,
				Path: "/labels/foo",
			},
			expErr: fmt.Errorf("mask prefix not allowed"),
		},
		{
			note: "standard",
			input: &maskRule{
				OP:   maskOPRemove,
				Path: "/input/a/b/c",
			},
			expPtr: &maskRule{
				OP:           maskOPRemove,
				Path:         "/input/a/b/c",
				escapedParts: []string{"input", "a", "b", "c"},
			},
		},
		{
			note: "fail with object path undefined",
			input: &maskRule{
				OP:                maskOPRemove,
				Path:              "/input/a/b/c",
				failUndefinedPath: true,
			},
			expPtr: &maskRule{
				OP:                maskOPRemove,
				Path:              "/input/a/b/c",
				failUndefinedPath: true,
				escapedParts:      []string{"input", "a", "b", "c"},
			},
		},
		{
			note: "fail with invalid OP",
			input: &maskRule{
				OP:   maskOP("undefinedOP"),
				Path: "/input/a/b/c",
			},
			expErr: fmt.Errorf("mask op is not supported: undefinedOP"),
		},
		{
			note: "escaping",
			input: &maskRule{
				OP:   maskOPRemove,
				Path: "/input/a/%2F%2F/b",
			},
			expPtr: &maskRule{
				OP:           maskOPRemove,
				Path:         "/input/a/%2F%2F/b",
				escapedParts: []string{"input", "a", "%252F%252F", "b"},
			},
		},
		{
			note: "bad escape",
			input: &maskRule{
				OP:   maskOPRemove,
				Path: "/input/a/%F/b",
			},
			expErr: fmt.Errorf("invalid URL escape"),
		},
		{
			note: "empty component",
			input: &maskRule{
				OP:   maskOPRemove,
				Path: "/input//foo",
			},
			expPtr: &maskRule{
				OP:           maskOPRemove,
				Path:         "/input//foo",
				escapedParts: []string{"input", "", "foo"},
			},
		},
		{
			note: "result",
			input: &maskRule{
				OP:   maskOPRemove,
				Path: "/result/a",
			},
			expPtr: &maskRule{
				OP:           maskOPRemove,
				Path:         "/result/a",
				escapedParts: []string{"result", "a"},
			},
		},
		{
			note: "root",
			input: &maskRule{
				OP:   maskOPRemove,
				Path: "/input",
			},
			expPtr: &maskRule{
				OP:            maskOPRemove,
				Path:          "/input",
				escapedParts:  []string{"input"},
				modifyFullObj: true,
			},
		},
		{
			note: "unsupported mask op",
			input: &maskRule{
				OP:   maskOP("unsupported"),
				Path: "/input",
			},
			expErr: fmt.Errorf("mask op is not supported: unsupported"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			result, err := newMaskRule(tc.input.Path, withOP(tc.input.OP), withValue(tc.input.Value))
			if tc.input.failUndefinedPath {
				_ = withFailUndefinedPath()(result)
			}

			if tc.expErr != nil {
				if err == nil {
					t.Fatalf("Expected error but got: %v", result)
				} else if !strings.Contains(err.Error(), tc.expErr.Error()) {
					t.Fatalf("Expected error: %v, but got error: %v", tc.expErr, err)
				}
			} else {
				if err != nil {
					t.Fatal("Unexpected error:", err)
				}
				if !reflect.DeepEqual(result, tc.expPtr) {
					t.Fatalf("Expected %#+v but got %#+v", tc.expPtr, result)
				}
			}
		})
	}
}

func TestMaskRuleMask(t *testing.T) {

	tests := []struct {
		note   string
		ptr    *maskRule
		event  string
		exp    string
		expErr error
	}{
		{
			note: "erase input",
			ptr: &maskRule{
				OP:   maskOPRemove,
				Path: "/input",
			},
			event: `{"input": {"a": 1}}`,
			exp:   `{"erased": ["/input"]}`,
		},
		{
			note: "upsert input",
			ptr: &maskRule{
				OP:   maskOPUpsert,
				Path: "/input",
				Value: struct {
					RandoString string
				}{RandoString: "foo"},
			},
			event: `{"input": {"a": 1}}`,
			exp:   `{"masked": ["/input"], "input": {"RandoString": "foo"}}`,
		},
		{
			note: "erase result",
			ptr: &maskRule{
				OP:   maskOPRemove,
				Path: "/result",
			},
			event: `{"result": "foo"}`,
			exp:   `{"erased": ["/result"]}`,
		},
		{
			note: "upsert result",
			ptr: &maskRule{
				OP:    maskOPUpsert,
				Path:  "/result",
				Value: "upserted",
			},
			event: `{"result": "foo"}`,
			exp:   `{"masked": ["/result"], "result": "upserted"}`,
		},
		{
			note: "erase undefined input",
			ptr: &maskRule{
				OP:   maskOPRemove,
				Path: "/input/foo",
			},
			event: `{}`,
			exp:   `{}`,
		},
		{
			note: "erase undefined input: fail unknown object path on",
			ptr: &maskRule{
				OP:                maskOPRemove,
				Path:              "/input/foo",
				failUndefinedPath: true,
			},
			event:  `{}`,
			exp:    `{}`,
			expErr: errMaskInvalidObject,
		},
		{
			note: "upsert undefined input",
			ptr: &maskRule{
				OP:   maskOPUpsert,
				Path: "/input/foo",
			},
			event: `{}`,
			exp:   `{}`,
		},
		{
			note: "upsert undefined input: fail unknown object path on",
			ptr: &maskRule{
				OP:                maskOPUpsert,
				Path:              "/input/foo",
				failUndefinedPath: true,
			},
			event:  `{}`,
			exp:    `{}`,
			expErr: errMaskInvalidObject,
		},
		{
			note: "erase undefined result",
			ptr: &maskRule{
				OP:   maskOPRemove,
				Path: "/result/foo",
			},
			event: `{}`,
			exp:   `{}`,
		},
		{
			note: "erase undefined result: fail unknown object path on",
			ptr: &maskRule{
				OP:                maskOPRemove,
				Path:              "/result/foo",
				failUndefinedPath: true,
			},
			event:  `{}`,
			exp:    `{}`,
			expErr: errMaskInvalidObject,
		},
		{
			note: "upsert undefined result",
			ptr: &maskRule{
				OP:   maskOPUpsert,
				Path: "/result/foo",
			},
			event: `{}`,
			exp:   `{}`,
		},
		{
			note: "upsert undefined result: fail unknown object path on",
			ptr: &maskRule{
				OP:                maskOPUpsert,
				Path:              "/result/foo",
				failUndefinedPath: true,
			},
			event:  `{}`,
			exp:    `{}`,
			expErr: errMaskInvalidObject,
		},
		{
			note: "erase undefined node",
			ptr: &maskRule{
				OP:   maskOPRemove,
				Path: "/input/foo",
			},
			event: `{"input": {"bar": 1}}`,
			exp:   `{"input": {"bar": 1}}`,
		},
		{
			note: "erase undefined node: fail unknown object path on",
			ptr: &maskRule{
				OP:                maskOPRemove,
				Path:              "/input/foo",
				failUndefinedPath: true,
			},
			event:  `{"input": {"bar": 1}}`,
			exp:    `{"input": {"bar": 1}}`,
			expErr: errMaskInvalidObject,
		},
		{
			note: "upsert undefined node with nil value",
			ptr: &maskRule{
				OP:   maskOPUpsert,
				Path: "/input/foo",
			},
			event: `{"input": {"bar": 1}}`,
			exp:   `{"input": {"bar": 1, "foo": null}, "masked": ["/input/foo"]}`,
		},
		{
			note: "upsert undefined node with nil value: fail unknown object path on",
			ptr: &maskRule{
				OP:                maskOPUpsert,
				Path:              "/input/foo",
				failUndefinedPath: true,
			},
			event:  `{"input": {"bar": 1}}`,
			exp:    `{"input": {"bar": 1, "foo": null}, "masked": ["/input/foo"]}`,
			expErr: errMaskInvalidObject,
		},
		{
			note: "upsert undefined node with a value",
			ptr: &maskRule{
				OP:    maskOPUpsert,
				Path:  "/input/foo",
				Value: "upserted",
			},
			event: `{"input": {"bar": 1}}`,
			exp:   `{"input": {"bar": 1, "foo": "upserted"}, "masked": ["/input/foo"]}`,
		},
		{
			note: "erase undefined node-2",
			ptr: &maskRule{
				OP:   maskOPRemove,
				Path: "/input/foo/bar",
			},
			event: `{"input": {"foo": 1}}`,
			exp:   `{"input": {"foo": 1}}`,
		},
		{
			note: "upsert unsupported nested object type (json.Number) #1",
			ptr: &maskRule{
				OP:   maskOPUpsert,
				Path: "/input/foo/bar",
			},
			event: `{"input": {"foo": 1}}`,
			exp:   `{"input": {"foo": 1}}`,
		},
		{
			note: "upsert unsupported nested object type (string) #1",
			ptr: &maskRule{
				OP:   maskOPUpsert,
				Path: "/input/foo/bar",
			},
			event: `{"input": {"foo": "bar"}}`,
			exp:   `{"input": {"foo": "bar"}}`,
		},
		{
			note: "erase: undefined object: missing key",
			ptr: &maskRule{
				OP:   maskOPRemove,
				Path: "/input/foo/bar/baz",
			},
			event: `{"input": {"foo": {}}}`,
			exp:   `{"input": {"foo": {}}}`,
		},
		{
			note: "upsert: undefined object: missing key, no value",
			ptr: &maskRule{
				OP:   maskOPUpsert,
				Path: "/input/foo/bar/baz",
			},
			event: `{"input": {"foo": {}}}`,
			exp:   `{"input": {"foo": {"bar": {"baz": null}}}, "masked": ["/input/foo/bar/baz"]}`,
		},
		{
			note: "upsert: undefined object: missing key, provided value",
			ptr: &maskRule{
				OP:    maskOPUpsert,
				Path:  "/input/foo/bar/baz",
				Value: 100,
			},
			event: `{"input": {"foo": {}}}`,
			exp:   `{"input": {"foo": {"bar": {"baz": 100}}}, "masked": ["/input/foo/bar/baz"]}`,
		},
		{
			note: "erase: undefined scalar",
			ptr: &maskRule{
				OP:   maskOPRemove,
				Path: "/input/foo/bar/baz",
			},
			event: `{"input": {"foo": 1}}`,
			exp:   `{"input": {"foo": 1}}`,
		},
		{
			note: "upsert: unsupported nested object type (json.Number) #2",
			ptr: &maskRule{
				OP:   maskOPUpsert,
				Path: "/input/foo/bar/baz",
			},
			event: `{"input": {"foo": 1}}`,
			exp:   `{"input": {"foo": 1}}`,
		},
		{
			note: "erase: undefined array: non-int index",
			ptr: &maskRule{
				OP:   maskOPRemove,
				Path: "/input/foo/bar/baz", // bar is invalid
			},
			event: `{"input": {"foo": [{"baz": 1}]}}`,
			exp:   `{"input": {"foo": [{"baz": 1}]}}`,
		},
		{
			note: "upsert: unsupported type: []interface {}",
			ptr: &maskRule{
				OP:   maskOPUpsert,
				Path: "/input/foo/bar/baz", // foo is []interface
			},
			event: `{"input": {"foo": [{"baz": 1}]}}`,
			exp:   `{"input": {"foo": [{"baz": 1}]}}`,
		},
		{
			note: "erase: undefined array: negative index",
			ptr: &maskRule{
				OP:   maskOPRemove,
				Path: "/input/foo/-1/baz",
			},
			event: `{"input": {"foo": [{"baz": 1}]}}`,
			exp:   `{"input": {"foo": [{"baz": 1}]}}`,
		},
		{
			note: "upsert: undefined array: negative index",
			ptr: &maskRule{
				OP:   maskOPUpsert,
				Path: "/input/foo/-1/baz", // foo is an []interface {}
			},
			event: `{"input": {"foo": [{"baz": 1}]}}`,
			exp:   `{"input": {"foo": [{"baz": 1}]}}`,
		},
		{
			note: "erase: undefined array: index out of range",
			ptr: &maskRule{
				OP:   maskOPRemove,
				Path: "/input/foo/1/baz",
			},
			event: `{"input": {"foo": [{"baz": 1}]}}`,
			exp:   `{"input": {"foo": [{"baz": 1}]}}`,
		},
		{
			note: "upsert: unsupported nested object type (array) #1",
			ptr: &maskRule{
				OP:   maskOPUpsert,
				Path: "/input/foo/1/baz", // foo is an []interface {}
			},
			event: `{"input": {"foo": [{"baz": 1}]}}`,
			exp:   `{"input": {"foo": [{"baz": 1}]}}`,
		},
		{
			note: "erase: undefined array: remove element",
			ptr: &maskRule{
				OP:   maskOPRemove,
				Path: "/input/foo/0",
			},
			event: `{"input": {"foo": [1]}}`,
			exp:   `{"input": {"foo": [1]}}`,
		},
		{
			note: "upsert: unsupported nested object type (array) #2",
			ptr: &maskRule{
				OP:   maskOPUpsert,
				Path: "/input/foo/0",
			},
			event: `{"input": {"foo": [1]}}`,
			exp:   `{"input": {"foo": [1]}}`,
		},
		{
			note: "erase: object key",
			ptr: &maskRule{
				OP:   maskOPRemove,
				Path: "/input/foo",
			},
			event: `{"input": {"bar": 1, "foo": [{"baz": 1}]}}`,
			exp:   `{"input": {"bar": 1}, "erased": ["/input/foo"]}`,
		},
		{
			note: "upsert: object key",
			ptr: &maskRule{
				OP:    maskOPUpsert,
				Path:  "/input/foo",
				Value: []map[string]int{{"nabs": 1}},
			},
			event: `{"input": {"bar": 1, "foo": [{"baz": 1}]}}`,
			exp:   `{"input": {"bar": 1, "foo": [{"nabs": 1}]}, "masked": ["/input/foo"]}`,
		},
		{
			note: "erase: object key (multiple)",
			ptr: &maskRule{
				OP:   maskOPRemove,
				Path: "/input/bar",
			},
			event: `{"input": {"bar": 1}, "erased": ["/input/foo"]}`,
			exp:   `{"input": {}, "erased": ["/input/foo", "/input/bar"]}`,
		},
		{
			note: "erase: object key (nested array)",
			ptr: &maskRule{
				OP:   maskOPRemove,
				Path: "/input/foo/0/bar",
			},
			event: `{"input": {"foo": [{"bar": 1, "baz": 2}]}}`,
			exp:   `{"input": {"foo": [{"baz": 2}]}, "erased": ["/input/foo/0/bar"]}`,
		},
		{
			note: "erase input: special character in path",
			ptr: &maskRule{
				OP:   maskOPRemove,
				Path: "/input/:path",
			},
			event: `{"input": {"bar": 1, ":path": "token"}}`,
			exp:   `{"input": {"bar": 1}, "erased": ["/input/:path"]}`,
		},
		{
			note: "upsert input: special character in path",
			ptr: &maskRule{
				OP:    maskOPUpsert,
				Path:  "/input/:path",
				Value: "upserted",
			},
			event: `{"input": {"bar": 1, ":path": "token"}}`,
			exp:   `{"input": {"bar": 1, ":path": "upserted"}, "masked": ["/input/:path"]}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {

			ptr, err := newMaskRule(tc.ptr.Path, withOP(tc.ptr.OP), withValue(tc.ptr.Value))
			if tc.ptr.failUndefinedPath {
				_ = withFailUndefinedPath()(ptr)
			}

			if err != nil {
				panic(err)
			}

			var exp EventV1
			if err := util.UnmarshalJSON([]byte(tc.exp), &exp); err != nil {
				panic(err)
			}

			var event EventV1
			if err := util.UnmarshalJSON([]byte(tc.event), &event); err != nil {
				panic(err)
			}

			err = ptr.Mask(&event)
			if err != nil {
				if tc.expErr == nil {
					t.Fatalf("no expected error, but received '%s'", err.Error())
				}
				if tc.expErr.Error() != err.Error() {
					t.Fatalf("expected error '%s', got '%s'", tc.expErr.Error(), err.Error())
				}

			}

			// compare via json marshall to map tc input types
			bs1, _ := json.MarshalIndent(exp, "", "  ")
			bs2, _ := json.MarshalIndent(event, "", "  ")
			if !bytes.Equal(bs1, bs2) {
				t.Fatalf("Expected: %s\nGot: %s", string(bs1), string(bs2))
			}
		})
	}
}

func TestNewMaskRuleSet(t *testing.T) {
	tests := []struct {
		note  string
		value interface{}
		exp   *maskRuleSet
		err   error
	}{
		{
			note:  "invalid format: not []interface{}",
			value: map[string]int{"invalid": 1},
			err:   fmt.Errorf("unexpected rule format map[invalid:1] (map[string]int)"),
		},
		{
			note: "invalid format: nested type not string or map[string]interface{}",
			value: []interface{}{
				[]int{1, 2},
			},
			err: fmt.Errorf("invalid mask rule format encountered: []int"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			_, err := newMaskRuleSet(tc.value, func(mRule *maskRule, err error) {})
			if err != nil {
				if exp, act := tc.err.Error(), err.Error(); exp != act {
					t.Fatalf("Expected: %s\nGot: %s", exp, act)
				}
			} else if tc.err != nil {
				t.Errorf("expected error %v, got nil", tc.err)
			}
		})
	}
}

func TestMaskRuleSetMask(t *testing.T) {

	tests := []struct {
		note   string
		rules  []*maskRule
		event  string
		exp    string
		expErr error
	}{
		{
			note: "erase input",
			rules: []*maskRule{
				{
					OP:   maskOPRemove,
					Path: "/input",
				},
			},
			event: `{"input": {"a": 1}}`,
			exp:   `{"erased": ["/input"]}`,
		},
		{
			note: "erase result",
			rules: []*maskRule{
				{
					OP:   maskOPRemove,
					Path: "/result",
				},
			},
			event: `{"result": {"a": 1}}`,
			exp:   `{"erased": ["/result"]}`,
		},
		{
			note: "erase input and result nested",
			rules: []*maskRule{
				{
					OP:   maskOPRemove,
					Path: "/input/a/b",
				},
				{
					OP:   maskOPRemove,
					Path: "/result/c/d",
				},
			},
			event: `{"input":{"a":{"b":"removeme","y":"stillhere"}},"result":{"c":{"d":"removeme","z":"stillhere"}}}`,
			exp:   `{"input":{"a":{"y":"stillhere"}},"result":{"c":{"z":"stillhere"}},"erased":["/input/a/b", "/result/c/d"]}`,
		},
		{
			note: "expected rule error",
			rules: []*maskRule{
				{
					OP:                maskOPRemove,
					Path:              "/result",
					failUndefinedPath: true,
				},
			},
			event:  `{"input":"foo"}`,
			exp:    `{"input":"foo"}`,
			expErr: errMaskInvalidObject,
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			ptr := &maskRuleSet{}
			var ruleErr error
			if tc.expErr != nil {
				ptr.OnRuleError = func(mRule *maskRule, err error) {
					ruleErr = err
				}
			} else {
				ptr.OnRuleError = func(mRule *maskRule, err error) {
					t.Fatalf(fmt.Sprintf("unexpected rule error, rule: %s, error: %s", mRule.String(), err.Error()))
				}
			}
			for _, rule := range tc.rules {
				var mRule *maskRule
				var err error
				if rule.failUndefinedPath {
					mRule, err = newMaskRule(rule.Path, withOP(rule.OP), withValue(rule.Value), withFailUndefinedPath())
				} else {
					mRule, err = newMaskRule(rule.Path, withOP(rule.OP), withValue(rule.Value))
				}
				if err != nil {
					panic(err)
				}
				ptr.Rules = append(ptr.Rules, mRule)
			}

			var exp EventV1
			if err := util.UnmarshalJSON([]byte(tc.exp), &exp); err != nil {
				panic(err)
			}

			var event EventV1
			var origEvent EventV1
			if err := util.UnmarshalJSON([]byte(tc.event), &event); err != nil {
				panic(err)
			}
			origEvent = event

			ptr.Mask(&event)

			// compare via json marshall to map tc input types
			bs1, _ := json.MarshalIndent(exp, "", "  ")
			bs2, _ := json.MarshalIndent(event, "", "  ")
			if !bytes.Equal(bs1, bs2) {
				t.Fatalf("Expected: %s\nGot: %s", string(bs1), string(bs2))
			}

			if origEvent.Result != nil && reflect.DeepEqual(origEvent.Result, event.Result) {
				t.Fatal("Expected event.Result to be deep copied during masking, so that the event's original Result is not modified")
			}

			if tc.expErr != nil {
				if ruleErr == nil {
					t.Fatalf("Expected: %s\nGot:%s", tc.expErr.Error(), "nil")
				}
				if tc.expErr != ruleErr {
					t.Fatalf("Expected: %s\nGot:%s", tc.expErr.Error(), ruleErr.Error())
				}
			}
		})
	}
}
