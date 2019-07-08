package logs

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/util"
)

func TestParsePtr(t *testing.T) {

	tests := []struct {
		note   string
		input  string
		expErr error
		expPtr ptr
	}{
		{
			note:   "empty",
			input:  "",
			expErr: fmt.Errorf("mask must be non-empty"),
		},
		{
			note:   "missing slash",
			input:  "foo/bar",
			expErr: fmt.Errorf("mask must be slash-prefixed"),
		},
		{
			note:   "no prefix",
			input:  "/",
			expErr: fmt.Errorf("mask prefix not allowed"),
		},
		{
			note:   "bad prefix key",
			input:  "/labels/foo",
			expErr: fmt.Errorf("mask prefix not allowed"),
		},
		{
			note:   "standard",
			input:  "/input/a/b/c",
			expPtr: ptr{"input", "a", "b", "c"},
		},
		{
			note:   "escaping",
			input:  "/input/a/%2F%2F/b",
			expPtr: ptr{"input", "a", "//", "b"},
		},
		{
			note:   "bad escape",
			input:  "/input/a/%F/b",
			expErr: fmt.Errorf("invalid URL escape"),
		},
		{
			note:   "empty component",
			input:  "/input//foo",
			expPtr: ptr{"input", "", "foo"},
		},
		{
			note:   "result",
			input:  "/result/a",
			expPtr: ptr{"result", "a"},
		},
		{
			note:   "root",
			input:  "/input",
			expPtr: ptr{"input"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			result, err := parsePtr(tc.input)
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
					t.Fatalf("Expected %v but got %v", tc.expPtr, result)
				}
			}
		})
	}
}

func TestPtrString(t *testing.T) {

	result := ptr{"input", "foo/bar", "baz", ""}.String()
	exp := "/input/foo%2Fbar/baz/"

	if result != exp {
		t.Fatalf("Expected %q but got %q", exp, result)
	}
}

func TestPtrErase(t *testing.T) {

	tests := []struct {
		note  string
		ptr   string
		event string
		exp   string
	}{
		{
			note:  "erase input",
			ptr:   "/input",
			event: `{"input": {"a": 1}}`,
			exp:   `{"erased": ["/input"]}`,
		},
		{
			note:  "erase result",
			ptr:   "/result",
			event: `{"result": "foo"}`,
			exp:   `{"erased": ["/result"]}`,
		},
		{
			note:  "erase: undefined input",
			ptr:   "/input/foo",
			event: `{}`,
			exp:   `{}`,
		},
		{
			note:  "erase: undefined result",
			ptr:   "/result/foo",
			event: `{}`,
			exp:   `{}`,
		},
		{
			note:  "erase: undefined node",
			ptr:   "/input/foo",
			event: `{"input": {"bar": 1}}`,
			exp:   `{"input": {"bar": 1}}`,
		},
		{
			note:  "erase: undefined node-2",
			ptr:   "/input/foo/bar",
			event: `{"input": {"foo": 1}}`,
			exp:   `{"input": {"foo": 1}}`,
		},
		{
			note:  "erase: undefined object: missing key",
			ptr:   "/input/foo/bar/baz",
			event: `{"input": {"foo": {}}}`,
			exp:   `{"input": {"foo": {}}}`,
		},
		{
			note:  "erase: undefined scalar",
			ptr:   "/input/foo/bar/baz",
			event: `{"input": {"foo": 1}}`,
			exp:   `{"input": {"foo": 1}}`,
		},
		{
			note:  "erase: undefined array: non-int index",
			ptr:   "/input/foo/bar/baz", // bar is invalid
			event: `{"input": {"foo": [{"baz": 1}]}}`,
			exp:   `{"input": {"foo": [{"baz": 1}]}}`,
		},
		{
			note:  "erase: undefined array: negative index",
			ptr:   "/input/foo/-1/baz",
			event: `{"input": {"foo": [{"baz": 1}]}}`,
			exp:   `{"input": {"foo": [{"baz": 1}]}}`,
		},
		{
			note:  "erase: undefined array: index out of range",
			ptr:   "/input/foo/1/baz",
			event: `{"input": {"foo": [{"baz": 1}]}}`,
			exp:   `{"input": {"foo": [{"baz": 1}]}}`,
		},
		{
			note:  "erase: undefined array: remove element",
			ptr:   "/input/foo/0",
			event: `{"input": {"foo": [1]}}`,
			exp:   `{"input": {"foo": [1]}}`,
		},
		{
			note:  "erase: object key",
			ptr:   "/input/foo",
			event: `{"input": {"bar": 1, "foo": [{"baz": 1}]}}`,
			exp:   `{"input": {"bar": 1}, "erased": ["/input/foo"]}`,
		},
		{
			note:  "erase: object key (multiple)",
			ptr:   "/input/bar",
			event: `{"input": {"bar": 1}, "erased": ["/input/foo"]}`,
			exp:   `{"input": {}, "erased": ["/input/foo", "/input/bar"]}`,
		},
		{
			note:  "erase: object key (nested array)",
			ptr:   "/input/foo/0/bar",
			event: `{"input": {"foo": [{"bar": 1, "baz": 2}]}}`,
			exp:   `{"input": {"foo": [{"baz": 2}]}, "erased": ["/input/foo/0/bar"]}`,
		},
		{
			note:  "erase: result key",
			ptr:   "/result/foo/bar/baz",
			event: `{"result": {"foo": {"bar": {"baz": 1}}}}`,
			exp:   `{"result": {"foo": {"bar": {}}}, "erased": ["/result/foo/bar/baz"]}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {

			ptr, err := parsePtr(tc.ptr)
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

			ptr.Erase(&event)

			if !reflect.DeepEqual(event, exp) {
				bs1, _ := json.MarshalIndent(exp, "", "  ")
				bs2, _ := json.MarshalIndent(event, "", "  ")
				t.Fatalf("Expected: %s\nGot: %s", bs1, bs2)
			}
		})
	}
}
