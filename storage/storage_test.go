// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package storage

import (
	"reflect"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/ast"
)

func TestStorageReadPlugin(t *testing.T) {

	mem1 := NewDataStoreFromReader(strings.NewReader(`
    {
        "foo": {
            "bar": {
                "baz": [1,2,3,4]
            }
        }
    }`))

	mem2 := NewDataStoreFromReader(strings.NewReader(`
	{
		"corge": [5,6,7,8]
	}
	`))

	mountPath := ast.MustParseRef("data.foo.bar.qux")
	mem2.SetMountPath(mountPath)
	store := New(Config{
		Builtin: mem1,
	})
	if err := store.Mount(mem2, mountPath); err != nil {
		t.Fatalf("Unexpected mount error: %v", err)
	}

	txn, err := store.NewTransaction()
	if err != nil {
		panic(err)
	}

	tests := []struct {
		note     string
		path     string
		expected string
	}{
		{"plugin", "data.foo.bar.qux.corge[1]", "6"},
		{"multiple", "data.foo.bar", `{"baz": [1,2,3,4], "qux": {"corge": [5,6,7,8]}}`},
	}

	for i, tc := range tests {

		result, err := store.Read(txn, ast.MustParseRef(tc.path))
		if err != nil {
			t.Errorf("Test #%d (%v): Unexpected read error: %v", i+1, tc.note, err)
		}

		expected := loadExpectedResult(tc.expected)

		if !reflect.DeepEqual(result, expected) {
			t.Fatalf("Test #%d (%v): Expected %v from built-in store but got: %v", i+1, tc.note, expected, result)
		}

	}

}

func TestStorageIndexingBasicUpdate(t *testing.T) {

	refA := ast.MustParseRef("data.a[i]")
	refB := ast.MustParseRef("data.b[x]")
	store, ds := newStorageWithIndices(refA, refB)
	ds.mustPatch(AddOp, path(`a["-"]`), float64(100))

	if store.IndexExists(refA) {
		t.Errorf("Expected index to be removed after patch")
	}
}

func mustBuild(store *Storage, ref ast.Ref) {
	err := store.BuildIndex(invalidTXN, ref)
	if err != nil {
		panic(err)
	}
	if !store.IndexExists(ref) {
		panic(err)
	}
}

func newStorageWithIndices(r ...ast.Ref) (*Storage, *DataStore) {

	data := loadSmallTestData()
	ds := NewDataStoreFromJSONObject(data)
	store := New(Config{
		Builtin: ds,
	})

	for _, x := range r {
		mustBuild(store, x)
	}

	return store, ds
}
