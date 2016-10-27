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

func TestStorageReadNonGroundRef(t *testing.T) {
	store := New(InMemoryConfig())
	txn := NewTransactionOrDie(store)
	defer store.Close(txn)
	ref := ast.MustParseRef("data.foo[i]")
	_, e := store.Read(txn, ref)
	err, ok := e.(*Error)
	if !ok {
		t.Fatalf("Expected storage error but got: %v", err)
	}
	if err.Code != InternalErr {
		t.Fatalf("Expected internal error but got: %v", err)
	}
}

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

func TestStorageTransactionManagement(t *testing.T) {

	store := New(Config{
		Builtin: NewDataStoreFromReader(strings.NewReader(`
			{
				"foo": {
					"bar": {
						"baz": [1,2,3,4]
					}
				}
			}`)),
	})

	mock := mockStore{}

	if err := store.Mount(mock, ast.MustParseRef("data.foo.bar.qux")); err != nil {
		t.Fatalf("Unexpected mount error: %v", err)
	}

	txn, err := store.NewTransaction(ast.MustParseRef("data.foo.bar.qux.corge[x]"))

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if !reflect.DeepEqual(store.active, map[string]struct{}{mock.ID(): struct{}{}}) {
		t.Fatalf("Expected active to contain exactly one element but got: %v", store.active)
	}

	store.Close(txn)

	if len(store.active) != 0 {
		t.Fatalf("Expected active to be reset but got: %v", store.active)
	}

}

type mockStore struct {
	WritesNotSupported
	TriggersNotSupported
	id string
}

func (mockStore) ID() string {
	return "mock-store"
}

func (mockStore) Read(txn Transaction, ref ast.Ref) (interface{}, error) {
	return nil, nil
}

func (mockStore) Begin(txn Transaction, refs []ast.Ref) error {
	return nil
}

func (mockStore) Close(txn Transaction) {

}

func TestGroupStoresByRef(t *testing.T) {

	mounts := map[string]ast.Ref{
		"mount-1": ast.MustParseRef("data.foo.bar.qux"),
		"mount-2": ast.MustParseRef("data.foo.baz"),
		"mount-3": ast.MustParseRef("data.corge"),
	}

	result := groupRefsByStore("built-in", mounts, []ast.Ref{
		ast.MustParseRef("data[x]"),
		ast.MustParseRef("data.foo.bar.qux.grault"),
		ast.MustParseRef("data.foo[x][y][z]"),
	})

	expected := map[string][]ast.Ref{
		"built-in": []ast.Ref{
			ast.MustParseRef("data[x]"),
			ast.MustParseRef("data.foo[x][y][z]"),
		},
		"mount-1": []ast.Ref{
			ast.MustParseRef("data.foo.bar.qux"),
			ast.MustParseRef("data.foo.bar.qux.grault"),
			ast.MustParseRef("data.foo.bar.qux[z]"),
		},
		"mount-2": []ast.Ref{
			ast.MustParseRef("data.foo.baz"),
			ast.MustParseRef("data.foo.baz[y][z]"),
		},
		"mount-3": []ast.Ref{
			ast.MustParseRef("data.corge"),
		},
	}

	if len(result) != len(expected) {
		t.Fatalf("Expected %v but got: %v", expected, result)
	}

	for id := range result {
		if len(result[id]) != len(expected[id]) {
			t.Fatalf("Expected %v but got: %v", expected[id], result[id])
		}
		for i := range result[id] {
			if !result[id][i].Equal(expected[id][i]) {
				t.Fatalf("Expected %v but got: %v", expected[id], result[id])
			}
		}
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
