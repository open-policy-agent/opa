// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package storage

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/open-policy-agent/opa/ast"
)

func TestIndicesBuild(t *testing.T) {

	tests := []struct {
		note     string
		ref      string
		value    interface{}
		expected string
	}{
		{"single var", "data.a[i]", float64(2), `[{"i": 1}]`},
		{"two var", "data.d[x][y]", "baz", `[{"x": "e", "y": 1}]`},
		{"partial ground", `data.c[i]["y"][j]`, nil, `[{"i": 0, "j": 0}]`},
		{"multiple bindings", "data.g[x][y]", float64(0), `[
			{"x": "a", "y": 1},
			{"x": "a", "y": 2},
			{"x": "a", "y": 3},
			{"x": "b", "y": 0},
			{"x": "b", "y": 2},
			{"x": "b", "y": 3},
			{"x": "c", "y": 0},
			{"x": "c", "y": 1},
			{"x": "c", "y": 2}
		]`},
	}

	for i, tc := range tests {
		runIndexBuildTestCase(t, i+1, tc.note, tc.ref, tc.expected, tc.value)
	}

}

func TestIndicesAdd(t *testing.T) {

	indices := newIndices()
	data := loadSmallTestData()
	store := NewDataStoreFromJSONObject(data)

	ref := ast.MustParseRef("data.d[x][y]")

	// TODO(tsandall):
	indices.Build(store, invalidTXN, ref)
	index := indices.Get(ref)

	// new value to add
	var val1 interface{}
	err := json.Unmarshal([]byte(`{"x":[1,true]}`), &val1)
	if err != nil {
		panic(err)
	}
	bindings1 := loadExpectedBindings(`[{"x": "e", "y": 2}]`)[0]
	index.Add(val1, bindings1)
	assertBindingsEqual(t, "new value", index, val1, `[{"x": "e", "y": 2}]`)

	// existing value
	val2 := "baz"
	bindings2 := loadExpectedBindings(`[{"x": "e", "y": 3}]`)[0]
	index.Add(val2, bindings2)
	assertBindingsEqual(t, "existing value", index, val2, `[{"x": "e", "y": 1}, {"x": "e", "y": 3}]`)
	index.Add(val2, bindings2)
	assertBindingsEqual(t, "same value (no change)", index, val2, `[{"x": "e", "y": 1}, {"x": "e", "y": 3}]`)
}

func runIndexBuildTestCase(t *testing.T, i int, note string, refStr string, expectedStr string, value interface{}) {

	indices := newIndices()
	data := loadSmallTestData()
	store := NewDataStoreFromJSONObject(data)
	ref := ast.MustParseRef(refStr)

	if indices.Get(ref) != nil {
		t.Errorf("Test case %d (%v): Did not expect indices to contain %v yet", i, note, ref)
		return
	}

	// TODO(tsandall):
	err := indices.Build(store, invalidTXN, ref)
	if err != nil {
		t.Errorf("Test case %d (%v): Did not expect error from build: %v", i, note, err)
		return
	}

	index := indices.Get(ref)
	if index == nil {
		t.Errorf("Test case %d (%v): Did not expect nil index for %v", i, note, ref)
		return
	}

	assertBindingsEqual(t, fmt.Sprintf("Test case %d (%v)", i, note), index, value, expectedStr)
}

func assertBindingsEqual(t *testing.T, note string, index *bindingIndex, value interface{}, expectedStr string) {

	expected := loadExpectedBindings(expectedStr)

	err := index.Iter(value, func(bindings *Bindings) error {
		for j := range expected {
			if expected[j].Equal(bindings) {
				tmp := expected[:j]
				expected = append(tmp, expected[j+1:]...)
				return nil
			}
		}
		return fmt.Errorf("unexpected bindings: %v", bindings)
	})

	if err != nil {
		t.Errorf("%v: Did not expect error from index iteration: %v", note, err)
		return
	}

	if len(expected) > 0 {
		t.Errorf("%v: Missing expected bindings: %v", note, expected)
		return
	}
}

func loadExpectedBindings(input string) []*Bindings {
	var data []map[string]interface{}
	if err := json.Unmarshal([]byte(input), &data); err != nil {
		panic(err)
	}
	var expected []*Bindings
	for _, bindings := range data {
		buf := NewBindings()
		for k, v := range bindings {
			switch v := v.(type) {
			case string:
				buf.Put(ast.Var(k), ast.String(v))
			case float64:
				buf.Put(ast.Var(k), ast.Number(v))
			default:
				panic("unreachable")
			}
		}
		expected = append(expected, buf)
	}

	return expected
}
