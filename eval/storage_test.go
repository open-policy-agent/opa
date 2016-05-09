// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package eval

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"testing"

	"github.com/open-policy-agent/opa/ast"
)

func TestLoadFromFiles(t *testing.T) {
	tmp1, err := ioutil.TempFile("", "docFile")
	if err != nil {
		panic(err)
	}
	defer os.Remove(tmp1.Name())
	doc1 := `{"foo": "bar", "a": {"b": {"d": [1]}}}`
	if _, err := tmp1.Write([]byte(doc1)); err != nil {
		panic(err)
	}
	if err := tmp1.Close(); err != nil {
		panic(err)
	}

	tmp2, err := ioutil.TempFile("", "policyFile")
	if err != nil {
		panic(err)
	}
	defer os.Remove(tmp2.Name())
	mod1 := `
	package a.b.c
	import data.foo
	p = true :- foo = "bar"
	p = true :- 1 = 2
	`
	if _, err := tmp2.Write([]byte(mod1)); err != nil {
		panic(err)
	}
	if err := tmp2.Close(); err != nil {
		panic(err)
	}

	store, err := NewStorageFromFiles([]string{tmp1.Name(), tmp2.Name()})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
		return
	}

	r, err := store.Get(path("foo"))
	if Compare(r, "bar") != 0 || err != nil {
		t.Errorf("Expected %v but got %v (err: %v)", "bar", r, err)
		return
	}

	r, err = store.Get(path("a.b.c.p"))
	rules, ok := r.([]*ast.Rule)
	if !ok {
		t.Errorf("Expected rules but got: %v", r)
		return
	}
	if !rules[0].Name.Equal(ast.Var("p")) {
		t.Errorf("Expected rule p but got: %v", rules[0])
		return
	}
}

func TestStorageGet(t *testing.T) {

	data := loadSmallTestData()

	var tests = []struct {
		ref      string
		expected interface{}
	}{
		{"a[0]", float64(1)},
		{"a[3]", float64(4)},
		{"b.v1", "hello"},
		{"b.v2", "goodbye"},
		{"c[0].x[1]", false},
		{"c[0].y[0]", nil},
		{"c[0].y[1]", 3.14159},
		{"d.e[1]", "baz"},
		{"d.e", []interface{}{"bar", "baz"}},
		{"c[0].z", map[string]interface{}{"p": true, "q": false}},
		{"d[100]", notFoundError(path("d[100]"), objectKeyTypeMsg(float64(100)))},
		{"dead.beef", notFoundError(path("dead.beef"), doesNotExistMsg)},
		{"a.str", notFoundError(path("a.str"), arrayIndexTypeMsg("str"))},
		{"a[100]", notFoundError(path("a[100]"), outOfRangeMsg)},
		{"a[-1]", notFoundError(path("a[-1]"), outOfRangeMsg)},
		{"b.vdeadbeef", notFoundError(path("b.vdeadbeef"), doesNotExistMsg)},
	}

	store := NewStorageFromJSONObject(data)

	for idx, tc := range tests {
		ref := ast.MustParseRef(tc.ref)
		path, err := ref.Underlying()
		if err != nil {
			panic(err)
		}
		result, err := store.Get(path)
		switch e := tc.expected.(type) {
		case error:
			if err == nil {
				t.Errorf("Test case %d: expected error for %v but got %v", idx+1, ref, result)
			} else if !reflect.DeepEqual(err, tc.expected) {
				t.Errorf("Test case %d: unexpected error for %v: %v, expected: %v", idx+1, ref, err, e)
			}
		default:
			if err != nil {
				t.Errorf("Test case %d: expected success for %v but got %v", idx+1, ref, err)
			}
			if !reflect.DeepEqual(result, tc.expected) {
				t.Errorf("Test case %d: expected %f but got %f", idx+1, tc.expected, result)
			}
		}
	}

}

func TestStoragePatch(t *testing.T) {

	tests := []struct {
		note        string
		op          string
		path        interface{}
		value       string
		expected    error
		getPath     interface{}
		getExpected interface{}
	}{
		{"add root", "add", path("newroot"), `{"a": [[1]]}`, nil, path("newroot"), `{"a": [[1]]}`},
		{"add root/arr", "add", path("a[1]"), `"x"`, nil, path("a"), `[1,"x",2,3,4]`},
		{"add arr/arr", "add", path("h[1][2]"), `"x"`, nil, path("h"), `[[1,2,3], [2,3,"x",4]]`},
		{"add obj/arr", "add", path("d.e[1]"), `"x"`, nil, path("d"), `{"e": ["bar", "x", "baz"]}`},
		{"add obj", "add", path("b.vNew"), `"x"`, nil, path("b"), `{"v1": "hello", "v2": "goodbye", "vNew": "x"}`},
		{"add obj (existing)", "add", path("b.v2"), `"x"`, nil, path("b"), `{"v1": "hello", "v2": "x"}`},

		{"append root/arr", "add", path(`a["-"]`), `"x"`, nil, path("a"), `[1,2,3,4,"x"]`},
		{"append obj/arr", "add", path(`c[0].x["-"]`), `"x"`, nil, path("c[0].x"), `[true,false,"foo","x"]`},
		{"append arr/arr", "add", path(`h[0]["-"]`), `"x"`, nil, path(`h[0][3]`), `"x"`},

		{"remove root", "remove", path("a"), "", nil, path("a"), notFoundError(path("a"), doesNotExistMsg)},
		{"remove root/arr", "remove", path("a[1]"), "", nil, path("a"), "[1,3,4]"},
		{"remove obj/arr", "remove", path("c[0].x[1]"), "", nil, path("c[0].x"), `[true,"foo"]`},
		{"remove arr/arr", "remove", path("h[0][1]"), "", nil, path("h[0]"), "[1,3]"},
		{"remove obj", "remove", path("b.v2"), "", nil, path("b"), `{"v1": "hello"}`},

		{"replace root", "replace", path("a"), "1", nil, path("a"), "1"},
		{"replace obj", "replace", path("b.v1"), "1", nil, path("b"), `{"v1": 1, "v2": "goodbye"}`},
		{"replace array", "replace", path("a[1]"), "999", nil, path("a"), "[1,999,3,4]"},

		{"err: empty path", "add", []interface{}{}, "", notFoundError([]interface{}{}, nonEmptyMsg), nil, nil},
		{"err: non-string head", "add", []interface{}{float64(1)}, "", notFoundError([]interface{}{float64(1)}, stringHeadMsg), nil, nil},
		{"err: add arr (non-integer)", "add", path("a.foo"), "1", notFoundError(path("a.foo"), arrayIndexTypeMsg("xxx")), nil, nil},
		{"err: add arr (non-integer)", "add", path("a[3.14]"), "1", notFoundError(path("a[3.14]"), arrayIndexTypeMsg(3.14)), nil, nil},
		{"err: add arr (out of range)", "add", path("a[5]"), "1", notFoundError(path("a[5]"), outOfRangeMsg), nil, nil},
		{"err: add arr (out of range)", "add", path("a[-1]"), "1", notFoundError(path("a[-1]"), outOfRangeMsg), nil, nil},
		{"err: add arr (missing root)", "add", path("dead.beef[0]"), "1", notFoundError(path("dead.beef"), doesNotExistMsg), nil, nil},
		{"err: add obj (non-string)", "add", path("b[100]"), "1", notFoundError(path("b[100]"), objectKeyTypeMsg(float64(100))), nil, nil},
		{"err: add non-coll", "add", path("a[1][2]"), "1", notFoundError(path("a[1][2]"), nonCollectionMsg(float64(1))), nil, nil},
		{"err: append (missing)", "add", path(`dead.beef["-"]`), "1", notFoundError(path("dead"), doesNotExistMsg), nil, nil},
		{"err: append obj/arr", "add", path(`c[0].deadbeef["-"]`), `"x"`, notFoundError(path("c[0].deadbeef"), doesNotExistMsg), nil, nil},
		{"err: append arr/arr (out of range)", "add", path(`h[9999]["-"]`), `"x"`, notFoundError(path("h[9999]"), outOfRangeMsg), nil, nil},
		{"err: append arr/arr (non-array)", "add", path(`b.v1["-"]`), "1", notFoundError(path("b.v1"), nonArrayMsg("v1")), nil, nil},
		{"err: remove missing", "remove", path("dead.beef[0]"), "", notFoundError(path("dead.beef[0]"), doesNotExistMsg), nil, nil},
		{"err: remove obj (non string)", "remove", path("b[100]"), "", notFoundError(path("b[100]"), objectKeyTypeMsg(float64(100))), nil, nil},
		{"err: remove obj (missing)", "remove", path("b.deadbeef"), "", notFoundError(path("b.deadbeef"), doesNotExistMsg), nil, nil},
		{"err: replace root (missing)", "replace", path("deadbeef"), "1", notFoundError(path("deadbeef"), doesNotExistMsg), nil, nil},
		{"err: replace missing", "replace", "dead.beef[1]", "1", notFoundError(path("dead.beef[1]"), doesNotExistMsg), nil, nil},
	}

	for i, tc := range tests {
		data := loadSmallTestData()
		store := NewStorageFromJSONObject(data)

		// Perform patch and check result
		value := loadExpectedSortedResult(tc.value)

		var op StorageOp
		switch tc.op {
		case "add":
			op = StorageAdd
		case "remove":
			op = StorageRemove
		case "replace":
			op = StorageReplace
		default:
			panic(fmt.Sprintf("illegal value: %v", tc.op))
		}

		err := store.Patch(op, path(tc.path), value)

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

		if tc.getPath == nil {
			continue
		}

		// Perform get and verify result
		result, err := store.Get(path(tc.getPath))
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

func TestStorageIndexingBasicUpdate(t *testing.T) {
	refA := ast.MustParseRef("data.a[i]")
	refB := ast.MustParseRef("data.b[x]")
	store := newStorageWithIndices(refA, refB)

	mustPatch(store, StorageAdd, path(`a["-"]`), float64(100))

	index := store.Indices.Get(refA)
	if index != nil {
		t.Errorf("Expected index to be removed after patch: %v", index)
	}

	index = store.Indices.Get(refB)
	if index == nil {
		t.Errorf("Expected index to be intact after patch: %v", refB)
	}
}

func TestStorageIndexingAddDeepPath(t *testing.T) {
	ref := ast.MustParseRef("data.l[x]")
	refD := ast.MustParseRef("data.l[x].d")
	store := newStorageWithIndices(ref, refD)

	mustPatch(store, StorageAdd, path(`l[0].c["-"]`), float64(5))

	index := store.Indices.Get(ref)
	if index != nil {
		t.Errorf("Expected index to be removed after patch: %v", index)
	}

	index = store.Indices.Get(refD)
	if index == nil {
		t.Errorf("Expected index to be intact after patch: %v", refD)
	}
}

func TestStorageIndexingAddDeepRef(t *testing.T) {
	ref := ast.MustParseRef("data.l[x].a")
	store := newStorageWithIndices(ref)
	var data interface{}
	json.Unmarshal([]byte(`{"a": "eve", "b": 100, "c": [999,999,999]}`), &data)

	mustPatch(store, StorageAdd, path(`l["-"]`), data)

	index := store.Indices.Get(ref)
	if index != nil {
		t.Errorf("Expected index to be removed after patch: %v", index)
	}
}

func newStorageWithIndices(r ...ast.Ref) *Storage {
	data := loadSmallTestData()
	store := NewStorageFromJSONObject(data)
	for _, x := range r {
		mustBuild(store, x)
	}
	return store
}

func mustBuild(store *Storage, ref ast.Ref) {
	err := store.Indices.Build(store, ref)
	if err != nil {
		panic(err)
	}
}

func mustPatch(store *Storage, op StorageOp, path []interface{}, value interface{}) {
	err := store.Patch(op, path, value)
	if err != nil {
		panic(err)
	}
}

func path(input interface{}) []interface{} {
	switch input := input.(type) {
	case []interface{}:
		return input
	case string:
		switch v := ast.MustParseTerm(input).Value.(type) {
		case ast.Var:
			return []interface{}{string(v)}
		case ast.Ref:
			path, err := v.Underlying()
			if err != nil {
				panic(err)
			}
			return path
		}
	}
	panic(fmt.Sprintf("illegal value: %v", input))
}
