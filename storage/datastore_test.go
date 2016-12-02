// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package storage

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"github.com/open-policy-agent/opa/util"
)

func TestDataStoreGet(t *testing.T) {

	data := loadSmallTestData()

	var tests = []struct {
		path     string
		expected interface{}
	}{
		{"/a/0", json.Number("1")},
		{"/a/3", json.Number("4")},
		{"/b/v1", "hello"},
		{"/b/v2", "goodbye"},
		{"/c/0/x/1", false},
		{"/c/0/y/0", nil},
		{"/c/0/y/1", json.Number("3.14159")},
		{"/d/e/1", "baz"},
		{"/d/e", []interface{}{"bar", "baz"}},
		{"/c/0/z", map[string]interface{}{"p": true, "q": false}},
		{"/d/100", notFoundError(MustParsePath("/d/100"), doesNotExistMsg)},
		{"/dead/beef", notFoundError(MustParsePath("/dead/beef"), doesNotExistMsg)},
		{"/a/str", notFoundError(MustParsePath("/a/str"), arrayIndexTypeMsg)},
		{"/a/100", notFoundError(MustParsePath("/a/100"), outOfRangeMsg)},
		{"/a/-1", notFoundError(MustParsePath("/a/-1"), outOfRangeMsg)},
		{"/b/vdeadbeef", notFoundError(MustParsePath("/b/vdeadbeef"), doesNotExistMsg)},
	}

	ds := NewDataStoreFromJSONObject(data)

	for idx, tc := range tests {
		result, err := ds.Read(nil, MustParsePath(tc.path))
		switch e := tc.expected.(type) {
		case error:
			if err == nil {
				t.Errorf("Test case %d: expected error for %v but got %v", idx+1, tc.path, result)
			} else if !reflect.DeepEqual(err, tc.expected) {
				t.Errorf("Test case %d: unexpected error for %v: %v, expected: %v", idx+1, tc.path, err, e)
			}
		default:
			if err != nil {
				t.Errorf("Test case %d: expected success for %v but got %v", idx+1, tc.path, err)
			}
			if !reflect.DeepEqual(result, tc.expected) {
				t.Errorf("Test case %d: expected %f but got %f", idx+1, tc.expected, result)
			}
		}
	}

}

func TestDataStorePatch(t *testing.T) {

	tests := []struct {
		note        string
		op          string
		path        string
		value       string
		expected    error
		getPath     string
		getExpected interface{}
	}{
		{"add root", "add", "/", `{"a": [1]}`, nil, "/", `{"a": [1]}`},
		{"add", "add", "/newroot", `{"a": [[1]]}`, nil, "/newroot", `{"a": [[1]]}`},
		{"add arr", "add", "/a/1", `"x"`, nil, "/a", `[1,"x",2,3,4]`},
		{"add arr/arr", "add", "/h/1/2", `"x"`, nil, "/h", `[[1,2,3], [2,3,"x",4]]`},
		{"add obj/arr", "add", "/d/e/1", `"x"`, nil, "/d", `{"e": ["bar", "x", "baz"]}`},
		{"add obj", "add", "/b/vNew", `"x"`, nil, "/b", `{"v1": "hello", "v2": "goodbye", "vNew": "x"}`},
		{"add obj (existing)", "add", "/b/v2", `"x"`, nil, "/b", `{"v1": "hello", "v2": "x"}`},

		{"append arr", "add", "/a/-", `"x"`, nil, "/a", `[1,2,3,4,"x"]`},
		{"append obj/arr", "add", `/c/0/x/-`, `"x"`, nil, "/c/0/x", `[true,false,"foo","x"]`},
		{"append arr/arr", "add", `/h/0/-`, `"x"`, nil, `/h/0/3`, `"x"`},

		{"remove", "remove", "/a", "", nil, "/a", notFoundError(MustParsePath("/a"), doesNotExistMsg)},
		{"remove arr", "remove", "/a/1", "", nil, "/a", "[1,3,4]"},
		{"remove obj/arr", "remove", "/c/0/x/1", "", nil, "/c/0/x", `[true,"foo"]`},
		{"remove arr/arr", "remove", "/h/0/1", "", nil, "/h/0", "[1,3]"},
		{"remove obj", "remove", "/b/v2", "", nil, "/b", `{"v1": "hello"}`},

		{"replace root", "replace", "/", `{"a": [1]}`, nil, "/", `{"a": [1]}`},
		{"replace", "replace", "/a", "1", nil, "/a", "1"},
		{"replace obj", "replace", "/b/v1", "1", nil, "/b", `{"v1": 1, "v2": "goodbye"}`},
		{"replace array", "replace", "/a/1", "999", nil, "/a", "[1,999,3,4]"},

		{"err: bad root type", "add", "/", "[1,2,3]", invalidPatchErr(rootMustBeObjectMsg), "", nil},
		{"err: remove root", "remove", "/", "", invalidPatchErr(rootCannotBeRemovedMsg), "", nil},
		{"err: add arr (non-integer)", "add", "/a/foo", "1", notFoundError(MustParsePath("/a/foo"), arrayIndexTypeMsg), "", nil},
		{"err: add arr (non-integer)", "add", "/a/3.14", "1", notFoundError(MustParsePath("/a/3.14"), arrayIndexTypeMsg), "", nil},
		{"err: add arr (out of range)", "add", "/a/5", "1", notFoundError(MustParsePath("/a/5"), outOfRangeMsg), "", nil},
		{"err: add arr (out of range)", "add", "/a/-1", "1", notFoundError(MustParsePath("/a/-1"), outOfRangeMsg), "", nil},
		{"err: add arr (missing root)", "add", "/dead/beef/0", "1", notFoundError(MustParsePath("/dead/beef"), doesNotExistMsg), "", nil},
		{"err: add non-coll", "add", "/a/1/2", "1", notFoundError(MustParsePath("/a/1/2"), doesNotExistMsg), "", nil},
		{"err: append (missing)", "add", `/dead/beef/-`, "1", notFoundError(MustParsePath("/dead"), doesNotExistMsg), "", nil},
		{"err: append obj/arr", "add", `/c/0/deadbeef/-`, `"x"`, notFoundError(MustParsePath("/c/0/deadbeef"), doesNotExistMsg), "", nil},
		{"err: append arr/arr (out of range)", "add", `/h/9999/-`, `"x"`, notFoundError(MustParsePath("/h/9999"), outOfRangeMsg), "", nil},
		{"err: append append+add", "add", `/a/-/b/-`, `"x"`, notFoundError(MustParsePath(`/a/-`), arrayIndexTypeMsg), "", nil},
		{"err: append arr/arr (non-array)", "add", `/b/v1/-`, "1", notFoundError(MustParsePath("/b/v1"), doesNotExistMsg), "", nil},
		{"err: remove missing", "remove", "/dead/beef/0", "", notFoundError(MustParsePath("/dead/beef/0"), doesNotExistMsg), "", nil},
		{"err: remove obj (non string)", "remove", "/b/100", "", notFoundError(MustParsePath("/b/100"), doesNotExistMsg), "", nil},
		{"err: remove obj (missing)", "remove", "/b/deadbeef", "", notFoundError(MustParsePath("/b/deadbeef"), doesNotExistMsg), "", nil},
		{"err: replace root (missing)", "replace", "/deadbeef", "1", notFoundError(MustParsePath("/deadbeef"), doesNotExistMsg), "", nil},
		{"err: replace missing", "replace", "/dead/beef/1", "1", notFoundError(MustParsePath("/dead/beef/1"), doesNotExistMsg), "", nil},
	}

	for i, tc := range tests {
		data := loadSmallTestData()
		ds := NewDataStoreFromJSONObject(data)

		// Perform patch and check result
		value := loadExpectedSortedResult(tc.value)

		var op PatchOp
		switch tc.op {
		case "add":
			op = AddOp
		case "remove":
			op = RemoveOp
		case "replace":
			op = ReplaceOp
		default:
			panic(fmt.Sprintf("illegal value: %v", tc.op))
		}

		err := ds.patch(op, MustParsePath(tc.path), value)

		if tc.expected == nil {
			if err != nil {
				t.Errorf("Test case %d (%v): unexpected patch error: %v", i+1, tc.note, err)
				continue
			}
		} else {
			if err == nil {
				t.Errorf("Test case %d (%v): expected patch error, but got nil instead", i+1, tc.note)
				continue
			}
			if !reflect.DeepEqual(err, tc.expected) {
				t.Errorf("Test case %d (%v): expected patch error %v but got: %v", i+1, tc.note, tc.expected, err)
				continue
			}
		}

		if tc.getPath == "" {
			continue
		}

		// Perform get and verify result
		result, err := ds.Read(nil, MustParsePath(tc.getPath))
		switch expected := tc.getExpected.(type) {
		case error:
			if err == nil {
				t.Errorf("Test case %d (%v): expected get error but got: %v", i+1, tc.note, result)
				continue
			}
			if !reflect.DeepEqual(err, expected) {
				t.Errorf("Test case %d (%v): expected get error %v but got: %v", i+1, tc.note, expected, err)
				continue
			}
		case string:
			if err != nil {
				t.Errorf("Test case %d (%v): unexpected get error: %v", i+1, tc.note, err)
				continue
			}

			e := loadExpectedResult(expected)

			if !reflect.DeepEqual(result, e) {
				t.Errorf("Test case %d (%v): expected get result %v but got: %v", i+1, tc.note, e, result)
			}
		}

	}

}

func loadExpectedResult(input string) interface{} {
	if len(input) == 0 {
		return nil
	}
	var data interface{}
	if err := util.UnmarshalJSON([]byte(input), &data); err != nil {
		panic(err)
	}
	return data
}

func loadExpectedSortedResult(input string) interface{} {
	data := loadExpectedResult(input)
	switch data := data.(type) {
	case []interface{}:
		return data
	default:
		return data
	}
}

func loadSmallTestData() map[string]interface{} {
	var data map[string]interface{}
	err := util.UnmarshalJSON([]byte(`{
        "a": [1,2,3,4],
        "b": {
            "v1": "hello",
            "v2": "goodbye"
        },
        "c": [{
            "x": [true, false, "foo"],
            "y": [null, 3.14159],
            "z": {"p": true, "q": false}
        }],
        "d": {
            "e": ["bar", "baz"]
        },
		"g": {
			"a": [1, 0, 0, 0],
			"b": [0, 2, 0, 0],
			"c": [0, 0, 0, 4]
		},
		"h": [
			[1,2,3],
			[2,3,4]
		],
		"l": [
			{
				"a": "bob",
				"b": -1,
				"c": [1,2,3,4]
			},
			{
				"a": "alice",
				"b": 1,
				"c": [2,3,4,5],
				"d": null
			}
		]
    }`), &data)
	if err != nil {
		panic(err)
	}
	return data
}
