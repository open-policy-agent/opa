// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package inmem

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"github.com/open-policy-agent/opa/bundle"
	"github.com/open-policy-agent/opa/internal/file/archive"
	storageerrors "github.com/open-policy-agent/opa/storage/internal/errors"

	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/util"
)

func TestInMemoryRead(t *testing.T) {

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
		{"/a/0/beef", storageerrors.NewNotFoundError(storage.MustParsePath("/a/0/beef"))},
		{"/d/100", storageerrors.NewNotFoundError(storage.MustParsePath("/d/100"))},
		{"/dead/beef", storageerrors.NewNotFoundError(storage.MustParsePath("/dead/beef"))},
		{"/a/str", storageerrors.NewNotFoundErrorWithHint(storage.MustParsePath("/a/str"), storageerrors.ArrayIndexTypeMsg)},
		{"/a/100", storageerrors.NewNotFoundErrorWithHint(storage.MustParsePath("/a/100"), storageerrors.OutOfRangeMsg)},
		{"/a/-1", storageerrors.NewNotFoundErrorWithHint(storage.MustParsePath("/a/-1"), storageerrors.OutOfRangeMsg)},
	}

	store := NewFromObject(data)
	ctx := context.Background()

	for idx, tc := range tests {
		result, err := storage.ReadOne(ctx, store, storage.MustParsePath(tc.path))
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

func TestInMemoryWrite(t *testing.T) {

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
		{"append arr-2", "add", "/a/4", `"x"`, nil, "/a", `[1,2,3,4,"x"]`},
		{"append obj/arr", "add", `/c/0/x/-`, `"x"`, nil, "/c/0/x", `[true,false,"foo","x"]`},
		{"append obj/arr-2", "add", `/c/0/x/3`, `"x"`, nil, "/c/0/x", `[true,false,"foo","x"]`},
		{"append arr/arr", "add", `/h/0/-`, `"x"`, nil, `/h/0/3`, `"x"`},
		{"append arr/arr-2", "add", `/h/0/3`, `"x"`, nil, `/h/0/3`, `"x"`},
		{"append err", "remove", "/c/0/x/-", "", invalidPatchError("/c/0/x/-: invalid patch path"), "", nil},
		{"append err-2", "replace", "/c/0/x/-", "", invalidPatchError("/c/0/x/-: invalid patch path"), "", nil},

		{"remove", "remove", "/a", "", nil, "/a", storageerrors.NewNotFoundError(storage.MustParsePath("/a"))},
		{"remove arr", "remove", "/a/1", "", nil, "/a", "[1,3,4]"},
		{"remove obj/arr", "remove", "/c/0/x/1", "", nil, "/c/0/x", `[true,"foo"]`},
		{"remove arr/arr", "remove", "/h/0/1", "", nil, "/h/0", "[1,3]"},
		{"remove obj", "remove", "/b/v2", "", nil, "/b", `{"v1": "hello"}`},

		{"replace root", "replace", "/", `{"a": [1]}`, nil, "/", `{"a": [1]}`},
		{"replace", "replace", "/a", "1", nil, "/a", "1"},
		{"replace obj", "replace", "/b/v1", "1", nil, "/b", `{"v1": 1, "v2": "goodbye"}`},
		{"replace array", "replace", "/a/1", "999", nil, "/a", "[1,999,3,4]"},

		{"err: bad root type", "add", "/", "[1,2,3]", invalidPatchError(rootMustBeObjectMsg), "", nil},
		{"err: remove root", "remove", "/", "", invalidPatchError(rootCannotBeRemovedMsg), "", nil},
		{"err: add arr (non-integer)", "add", "/a/foo", "1", storageerrors.NewNotFoundErrorWithHint(storage.MustParsePath("/a/foo"), storageerrors.ArrayIndexTypeMsg), "", nil},
		{"err: add arr (non-integer)", "add", "/a/3.14", "1", storageerrors.NewNotFoundErrorWithHint(storage.MustParsePath("/a/3.14"), storageerrors.ArrayIndexTypeMsg), "", nil},
		{"err: add arr (out of range)", "add", "/a/5", "1", storageerrors.NewNotFoundErrorWithHint(storage.MustParsePath("/a/5"), storageerrors.OutOfRangeMsg), "", nil},
		{"err: add arr (out of range)", "add", "/a/-1", "1", storageerrors.NewNotFoundErrorWithHint(storage.MustParsePath("/a/-1"), storageerrors.OutOfRangeMsg), "", nil},
		{"err: add arr (missing root)", "add", "/dead/beef/0", "1", storageerrors.NewNotFoundError(storage.MustParsePath("/dead/beef/0")), "", nil},
		{"err: add non-coll", "add", "/a/1/2", "1", storageerrors.NewNotFoundError(storage.MustParsePath("/a/1/2")), "", nil},
		{"err: append (missing)", "add", `/dead/beef/-`, "1", storageerrors.NewNotFoundError(storage.MustParsePath("/dead/beef/-")), "", nil},
		{"err: append obj/arr", "add", `/c/0/deadbeef/-`, `"x"`, storageerrors.NewNotFoundError(storage.MustParsePath("/c/0/deadbeef/-")), "", nil},
		{"err: append arr/arr (out of range)", "add", `/h/9999/-`, `"x"`, storageerrors.NewNotFoundErrorWithHint(storage.MustParsePath("/h/9999/-"), storageerrors.OutOfRangeMsg), "", nil},
		{"err: append append+add", "add", `/a/-/b/-`, `"x"`, storageerrors.NewNotFoundErrorWithHint(storage.MustParsePath(`/a/-/b/-`), storageerrors.ArrayIndexTypeMsg), "", nil},
		{"err: append arr/arr (non-array)", "add", `/b/v1/-`, "1", storageerrors.NewNotFoundError(storage.MustParsePath("/b/v1/-")), "", nil},
		{"err: remove missing", "remove", "/dead/beef/0", "", storageerrors.NewNotFoundError(storage.MustParsePath("/dead/beef/0")), "", nil},
		{"err: remove obj (missing)", "remove", "/b/deadbeef", "", storageerrors.NewNotFoundError(storage.MustParsePath("/b/deadbeef")), "", nil},
		{"err: replace root (missing)", "replace", "/deadbeef", "1", storageerrors.NewNotFoundError(storage.MustParsePath("/deadbeef")), "", nil},
		{"err: replace missing", "replace", "/dead/beef/1", "1", storageerrors.NewNotFoundError(storage.MustParsePath("/dead/beef/1")), "", nil},
	}

	ctx := context.Background()

	for i, tc := range tests {
		data := loadSmallTestData()
		store := NewFromObject(data)

		// Perform patch and check result
		value := loadExpectedSortedResult(tc.value)

		var op storage.PatchOp
		switch tc.op {
		case "add":
			op = storage.AddOp
		case "remove":
			op = storage.RemoveOp
		case "replace":
			op = storage.ReplaceOp
		default:
			panic(fmt.Sprintf("illegal value: %v", tc.op))
		}

		err := storage.WriteOne(ctx, store, op, storage.MustParsePath(tc.path), value)
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
		result, err := storage.ReadOne(ctx, store, storage.MustParsePath(tc.getPath))
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

func TestInMemoryWriteOfStruct(t *testing.T) {
	type B struct {
		Bar int `json:"bar"`
	}

	type A struct {
		Foo *B `json:"foo"`
	}

	cases := map[string]struct {
		value    interface{}
		expected string
	}{
		"nested struct":            {A{&B{10}}, `{"foo": {"bar": 10 } }`},
		"pointer to nested struct": {&A{&B{10}}, `{"foo": {"bar": 10 } }`},
		"pointer to pointer to nested struct": {
			func() interface{} {
				a := &A{&B{10}}
				return &a
			}(), `{"foo": {"bar": 10 } }`},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			store := New()
			ctx := context.Background()

			err := storage.WriteOne(ctx, store, storage.AddOp, storage.MustParsePath("/x"), tc.value)
			if err != nil {
				t.Fatal(err)
			}

			actual, err := storage.ReadOne(ctx, store, storage.MustParsePath("/x"))
			if err != nil {
				t.Fatal(err)
			}

			expected := loadExpectedSortedResult(tc.expected)
			if !reflect.DeepEqual(expected, actual) {
				t.Errorf("expected %v, got %v", tc.expected, actual)
			}
		})
	}
}

func TestInMemoryTxnMultipleWrites(t *testing.T) {

	ctx := context.Background()
	store := NewFromObject(loadSmallTestData())
	txn := storage.NewTransactionOrDie(ctx, store, storage.WriteParams)

	// Perform a sequence of writes and then verify the read results are the
	// same for the writer during the transaction and the reader after the
	// commit.
	writes := []struct {
		op    storage.PatchOp
		path  string
		value string
	}{
		{storage.AddOp, "/a/-", "[]"},
		{storage.AddOp, "/a/4/-", "1"},
		{storage.AddOp, "/a/4/-", "2"},
		{storage.AddOp, "/a/4/2", "3"},
		{storage.AddOp, "/b/foo", "{}"},
		{storage.AddOp, "/b/foo/bar", "{}"},
		{storage.AddOp, "/b/foo/bar/baz", "1"},
		{storage.AddOp, "/arr", "[]"},
		{storage.AddOp, "/arr/-", "1"},
		{storage.AddOp, "/arr/0", "2"},
		{storage.AddOp, "/arr/2", "3"},
		{storage.AddOp, "/c/0/x/-", "0"},
		{storage.AddOp, "/_", "null"}, // introduce new txn.log head
		{storage.AddOp, "/c/0", `"new c[0]"`},
		{storage.AddOp, "/c/1", `"new c[1]"`},
		{storage.AddOp, "/_head", "1"},
		{storage.AddOp, "/_head", "2"}, // invalidate the txn.log head
		{storage.AddOp, "/d/f", `{"g": {"h": 0}}`},
		{storage.AddOp, "/d/f/g/i", `{"j": 1}`},
	}

	reads := []struct {
		path     string
		expected string
	}{
		{"/a", `[1,2,3,4,[1,2,3]]`},
		{"/b/foo", `{"bar": {"baz": 1}}`},
		{"/arr", `[2,1,3]`},
		{"/c/0", `"new c[0]"`},
		{"/c/1", `"new c[1]"`},
		{"/d/f", `{"g": {"h": 0, "i": {"j": 1}}}`},
		{"/d", `{"e": ["bar", "baz"], "f": {"g":{"h": 0, "i": {"j": 1}}}}`},
		{"/h/1/2", "4"},
	}

	for _, w := range writes {
		var jsn interface{}
		if w.value != "" {
			jsn = util.MustUnmarshalJSON([]byte(w.value))
		}
		if err := store.Write(ctx, txn, w.op, storage.MustParsePath(w.path), jsn); err != nil {
			t.Fatalf("Unexpected write error on %v: %v", w, err)
		}
	}

	for _, r := range reads {
		jsn := util.MustUnmarshalJSON([]byte(r.expected))
		result, err := store.Read(ctx, txn, storage.MustParsePath(r.path))
		if err != nil || !reflect.DeepEqual(jsn, result) {
			t.Fatalf("Expected writer's read %v to be %v but got: %v (err: %v)", r.path, jsn, result, err)
		}
	}

	if err := store.Commit(ctx, txn); err != nil {
		t.Fatalf("Unexpected commit error: %v", err)
	}

	txn = storage.NewTransactionOrDie(ctx, store)

	for _, r := range reads {
		jsn := util.MustUnmarshalJSON([]byte(r.expected))
		result, err := store.Read(ctx, txn, storage.MustParsePath(r.path))
		if err != nil || !reflect.DeepEqual(jsn, result) {
			t.Fatalf("Expected reader's read %v to be %v but got: %v (err: %v)", r.path, jsn, result, err)
		}
	}
}

func TestTruncateNoExistingPath(t *testing.T) {
	ctx := context.Background()
	store := NewFromObject(map[string]interface{}{})
	txn := storage.NewTransactionOrDie(ctx, store, storage.WriteParams)

	var archiveFiles = map[string]string{
		"/a/b/c/data.json": "[1,2,3]",
	}

	files := make([][2]string, 0, len(archiveFiles))
	for name, content := range archiveFiles {
		files = append(files, [2]string{name, content})
	}

	buf := archive.MustWriteTarGz(files)
	b, err := bundle.NewReader(buf).WithLazyLoadingMode(true).Read()
	if err != nil {
		t.Fatal(err)
	}

	iterator := bundle.NewIterator(b.Raw)

	params := storage.WriteParams
	params.BasePaths = []string{""}

	err = store.Truncate(ctx, txn, params, iterator)
	if err != nil {
		t.Fatalf("Unexpected truncate error: %v", err)
	}

	if err := store.Commit(ctx, txn); err != nil {
		t.Fatalf("Unexpected commit error: %v", err)
	}

	txn = storage.NewTransactionOrDie(ctx, store)

	actual, err := store.Read(ctx, txn, storage.MustParsePath("/"))
	if err != nil {
		t.Fatal(err)
	}

	expected := `
{
	"a": {
		"b": {
			"c": [1,2,3]
		}
	}
}
`
	jsn := util.MustUnmarshalJSON([]byte(expected))

	if !reflect.DeepEqual(jsn, actual) {
		t.Fatalf("Expected reader's read to be %v but got: %v", jsn, actual)
	}
}

func TestTruncate(t *testing.T) {
	ctx := context.Background()
	store := NewFromObject(map[string]interface{}{})
	txn := storage.NewTransactionOrDie(ctx, store, storage.WriteParams)

	var archiveFiles = map[string]string{
		"/a/b/c/data.json":   "[1,2,3]",
		"/a/b/d/data.json":   "true",
		"/data.json":         `{"x": {"y": true}, "a": {"b": {"z": true}}}`,
		"/a/b/y/data.yaml":   `foo: 1`,
		"/policy.rego":       "package foo\n p = 1",
		"/roles/policy.rego": "package bar\n p = 1",
	}

	files := make([][2]string, 0, len(archiveFiles))
	for name, content := range archiveFiles {
		files = append(files, [2]string{name, content})
	}

	buf := archive.MustWriteTarGz(files)
	b, err := bundle.NewReader(buf).WithLazyLoadingMode(true).Read()
	if err != nil {
		t.Fatal(err)
	}

	iterator := bundle.NewIterator(b.Raw)

	params := storage.WriteParams
	params.BasePaths = []string{""}

	err = store.Truncate(ctx, txn, params, iterator)
	if err != nil {
		t.Fatalf("Unexpected truncate error: %v", err)
	}

	if err := store.Commit(ctx, txn); err != nil {
		t.Fatalf("Unexpected commit error: %v", err)
	}

	txn = storage.NewTransactionOrDie(ctx, store)

	actual, err := store.Read(ctx, txn, storage.MustParsePath("/"))
	if err != nil {
		t.Fatal(err)
	}

	expected := `
{
	"a": {
		"b": {
			"c": [1,2,3],
			"d": true,
			"y": {
				"foo": 1
			},
			"z": true
		}
	},
	"x": {
		"y": true
	}
}
`
	jsn := util.MustUnmarshalJSON([]byte(expected))

	if !reflect.DeepEqual(jsn, actual) {
		t.Fatalf("Expected reader's read to be %v but got: %v", jsn, actual)
	}

	store.Abort(ctx, txn)

	txn = storage.NewTransactionOrDie(ctx, store)
	ids, err := store.ListPolicies(ctx, txn)
	if err != nil {
		t.Fatal(err)
	}

	expectedIds := map[string]struct{}{"policy.rego": {}, "roles/policy.rego": {}}

	for _, id := range ids {
		if _, ok := expectedIds[id]; !ok {
			t.Fatalf("Expected list policies to contain %v but got: %v", id, expectedIds)
		}
	}

	bs, err := store.GetPolicy(ctx, txn, "policy.rego")
	expectedBytes := []byte("package foo\n p = 1")
	if err != nil || !reflect.DeepEqual(expectedBytes, bs) {
		t.Fatalf("Expected get policy to return %v but got: %v (err: %v)", expectedBytes, bs, err)
	}

	bs, err = store.GetPolicy(ctx, txn, "roles/policy.rego")
	expectedBytes = []byte("package bar\n p = 1")
	if err != nil || !reflect.DeepEqual(expectedBytes, bs) {
		t.Fatalf("Expected get policy to return %v but got: %v (err: %v)", expectedBytes, bs, err)
	}
}

func TestTruncateDataMergeError(t *testing.T) {
	ctx := context.Background()
	store := NewFromObject(map[string]interface{}{})
	txn := storage.NewTransactionOrDie(ctx, store, storage.WriteParams)

	var archiveFiles = map[string]string{
		"/a/b/data.json": `{"c": "foo"}`,
		"/data.json":     `{"a": {"b": {"c": "bar"}}}`,
	}

	files := make([][2]string, 0, len(archiveFiles))
	for name, content := range archiveFiles {
		files = append(files, [2]string{name, content})
	}

	buf := archive.MustWriteTarGz(files)
	b, err := bundle.NewReader(buf).WithLazyLoadingMode(true).Read()
	if err != nil {
		t.Fatal(err)
	}

	iterator := bundle.NewIterator(b.Raw)

	err = store.Truncate(ctx, txn, storage.WriteParams, iterator)
	if err == nil {
		t.Fatal("Expected truncate error but got nil")
	}

	expected := "failed to insert data file from path a/b"
	if err.Error() != expected {
		t.Fatalf("Expected error %v but got %v", expected, err.Error())
	}
}

func TestTruncateBadRootWrite(t *testing.T) {
	ctx := context.Background()
	store := NewFromObject(map[string]interface{}{})
	txn := storage.NewTransactionOrDie(ctx, store, storage.WriteParams)

	var archiveFiles = map[string]string{
		"/a/b/d/data.json":   "true",
		"/data.json":         "[1,2,3]",
		"/roles/policy.rego": "package bar\n p = 1",
	}

	files := make([][2]string, 0, len(archiveFiles))
	for name, content := range archiveFiles {
		files = append(files, [2]string{name, content})
	}

	buf := archive.MustWriteTarGz(files)
	b, err := bundle.NewReader(buf).WithLazyLoadingMode(true).Read()
	if err != nil {
		t.Fatal(err)
	}

	iterator := bundle.NewIterator(b.Raw)

	err = store.Truncate(ctx, txn, storage.WriteParams, iterator)
	if err == nil {
		t.Fatal("Expected truncate error but got nil")
	}

	expected := "storage_invalid_patch_error: root must be object"
	if err.Error() != expected {
		t.Fatalf("Expected error %v but got %v", expected, err.Error())
	}
}

func TestInMemoryTxnWriteFailures(t *testing.T) {

	ctx := context.Background()
	store := NewFromObject(loadSmallTestData())
	txn := storage.NewTransactionOrDie(ctx, store, storage.WriteParams)

	writes := []struct {
		op      storage.PatchOp
		path    string
		value   string
		errCode string
	}{
		{storage.RemoveOp, "/c/0/y", "", ""},
		{storage.RemoveOp, "/c/0/y", "", storage.NotFoundErr},
		{storage.ReplaceOp, "/c/0/y/0", "", storage.NotFoundErr},
		{storage.AddOp, "/new", `{"foo": "bar"}`, ""},
		{storage.AddOp, "/a/0/beef", "", storage.NotFoundErr},
		{storage.AddOp, "/arr", `[1,2,3]`, ""},
		{storage.AddOp, "/arr/0/foo", "", storage.NotFoundErr},
		{storage.AddOp, "/arr/4", "", storage.NotFoundErr},
	}

	for _, w := range writes {
		var jsn interface{}
		if w.value != "" {
			jsn = util.MustUnmarshalJSON([]byte(w.value))
		}
		err := store.Write(ctx, txn, w.op, storage.MustParsePath(w.path), jsn)
		if (w.errCode == "" && err != nil) || (err == nil && w.errCode != "") {
			t.Fatalf("Expected errCode %q but got: %v", w.errCode, err)
		}
	}
}

func TestInMemoryTxnReadFailures(t *testing.T) {

	ctx := context.Background()
	store := NewFromObject(loadSmallTestData())
	txn := storage.NewTransactionOrDie(ctx, store, storage.WriteParams)

	if err := store.Write(ctx, txn, storage.RemoveOp, storage.MustParsePath("/c/0/y"), nil); err != nil {
		t.Fatalf("Unexpected write error: %v", err)
	}

	if result, err := store.Read(ctx, txn, storage.MustParsePath("/c/0/y/0")); !storage.IsNotFound(err) {
		t.Fatalf("Expected NotFoundErr for /c/0/y/0 but got: %v (err: %v)", result, err)
	}

	if result, err := store.Read(ctx, txn, storage.MustParsePath("/c/0/y")); !storage.IsNotFound(err) {
		t.Fatalf("Expected NotFoundErr for /c/0/y but got: %v (err: %v)", result, err)
	}

	if result, err := store.Read(ctx, txn, storage.MustParsePath("/a/0/beef")); !storage.IsNotFound(err) {
		t.Fatalf("Expected NotFoundErr for /c/0/y but got: %v (err: %v)", result, err)
	}

}

func TestInMemoryTxnBadWrite(t *testing.T) {
	ctx := context.Background()
	store := NewFromObject(loadSmallTestData())
	txn := storage.NewTransactionOrDie(ctx, store)
	if err := store.Write(ctx, txn, storage.RemoveOp, storage.MustParsePath("/a"), nil); !storage.IsInvalidTransaction(err) {
		t.Fatalf("Expected InvalidPatchErr but got: %v", err)
	}
}

func TestInMemoryTxnPolicies(t *testing.T) {

	ctx := context.Background()
	store := New()

	txn := storage.NewTransactionOrDie(ctx, store, storage.WriteParams)

	if err := store.UpsertPolicy(ctx, txn, "test", []byte("package test")); err != nil {
		t.Fatalf("Unexpected error on policy insert: %v", err)
	}

	if err := store.Commit(ctx, txn); err != nil {
		t.Fatalf("Unexpected commit error: %v", err)
	}

	txn = storage.NewTransactionOrDie(ctx, store, storage.WriteParams)

	if err := store.UpsertPolicy(ctx, txn, "test", []byte("package test\nimport data.foo")); err != nil {
		t.Fatalf("Unexpected error on policy insert/update: %v", err)
	}

	ids, err := store.ListPolicies(ctx, txn)
	expectedIds := []string{"test"}
	if err != nil || !reflect.DeepEqual(expectedIds, ids) {
		t.Fatalf("Expected list policies to return %v but got: %v (err: %v)", expectedIds, ids, err)
	}

	bs, err := store.GetPolicy(ctx, txn, "test")
	expectedBytes := []byte("package test\nimport data.foo")
	if err != nil || !reflect.DeepEqual(expectedBytes, bs) {
		t.Fatalf("Expected get policy to return %v but got: %v (err: %v)", expectedBytes, bs, err)
	}

	if err := store.DeletePolicy(ctx, txn, "test"); err != nil {
		t.Fatalf("Unexpected delete policy error: %v", err)
	}

	if err := store.UpsertPolicy(ctx, txn, "test2", []byte("package test2")); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	ids, err = store.ListPolicies(ctx, txn)
	expectedIds = []string{"test2"}
	if err != nil || !reflect.DeepEqual(expectedIds, ids) {
		t.Fatalf("Expected list policies to return %v but got: %v (err: %v)", expectedIds, ids, err)
	}

	bs, err = store.GetPolicy(ctx, txn, "test2")
	expectedBytes = []byte("package test2")
	if err != nil || !reflect.DeepEqual(expectedBytes, bs) {
		t.Fatalf("Expected get policy to return %v but got: %v (err: %v)", expectedBytes, bs, err)
	}

	if exist, err := store.GetPolicy(ctx, txn, "test"); !storage.IsNotFound(err) {
		t.Fatalf("Expected NotFoundErr for test but got: %v (err: %v)", exist, err)
	}

	store.Abort(ctx, txn)

	txn = storage.NewTransactionOrDie(ctx, store)
	ids, err = store.ListPolicies(ctx, txn)
	expectedIds = []string{"test"}
	if err != nil || !reflect.DeepEqual(expectedIds, ids) {
		t.Fatalf("Expected list policies to return %v but got: %v (err: %v)", expectedIds, ids, err)
	}

	if exist, err := store.GetPolicy(ctx, txn, "test2"); !storage.IsNotFound(err) {
		t.Fatalf("Expected NotFoundErr for test2 but got: %v (err: %v)", exist, err)
	}

	if err := store.DeletePolicy(ctx, txn, "test"); !storage.IsInvalidTransaction(err) {
		t.Fatalf("Expected InvalidTransactionErr for test but got: %v", err)
	}

	store.Abort(ctx, txn)

	txn = storage.NewTransactionOrDie(ctx, store, storage.WriteParams)

	if err := store.DeletePolicy(ctx, txn, "test"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if err := store.Commit(ctx, txn); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	txn = storage.NewTransactionOrDie(ctx, store)

	if ids, err := store.ListPolicies(ctx, txn); err != nil || len(ids) != 0 {
		t.Fatalf("Expected list policies to be empty but got: %v (err: %v)", ids, err)
	}

}

func TestInMemoryTriggers(t *testing.T) {

	ctx := context.Background()
	store := NewFromObject(loadSmallTestData())
	writeTxn := storage.NewTransactionOrDie(ctx, store, storage.WriteParams)
	readTxn := storage.NewTransactionOrDie(ctx, store)

	_, err := store.Register(ctx, readTxn, storage.TriggerConfig{
		OnCommit: func(context.Context, storage.Transaction, storage.TriggerEvent) {},
	})

	if err == nil || !storage.IsInvalidTransaction(err) {
		t.Fatalf("Expected transaction error: %v", err)
	}

	store.Abort(ctx, readTxn)

	var event storage.TriggerEvent
	modifiedPath := storage.MustParsePath("/a")
	expectedValue := "hello"

	_, err = store.Register(ctx, writeTxn, storage.TriggerConfig{
		OnCommit: func(ctx context.Context, txn storage.Transaction, evt storage.TriggerEvent) {
			result, err := store.Read(ctx, txn, modifiedPath)
			if err != nil || !reflect.DeepEqual(result, expectedValue) {
				t.Fatalf("Expected result to be hello for trigger read but got: %v (err: %v)", result, err)
			}
			event = evt
		},
	})
	if err != nil {
		t.Fatalf("Failed to register callback: %v", err)
	}

	if err := store.Write(ctx, writeTxn, storage.ReplaceOp, modifiedPath, expectedValue); err != nil {
		t.Fatalf("Unexpected write error: %v", err)
	}

	id := "test"
	data := []byte("package abc")
	if err := store.UpsertPolicy(ctx, writeTxn, id, data); err != nil {
		t.Fatalf("Unexpected upsert error: %v", err)
	}

	if err := store.Commit(ctx, writeTxn); err != nil {
		t.Fatalf("Unexpected commit error: %v", err)
	}

	if event.IsZero() || !event.PolicyChanged() || !event.DataChanged() {
		t.Fatalf("Expected policy and data change but got: %v", event)
	}

	expData := storage.DataEvent{Path: modifiedPath, Data: expectedValue, Removed: false}
	if d := event.Data[0]; !reflect.DeepEqual(expData, d) {
		t.Fatalf("Expected data event %v, got %v", expData, d)
	}

	expPolicy := storage.PolicyEvent{ID: id, Data: data, Removed: false}
	if p := event.Policy[0]; !reflect.DeepEqual(expPolicy, p) {
		t.Fatalf("Expected policy event %v, got %v", expPolicy, p)
	}
}

func TestInMemoryTriggersUnregister(t *testing.T) {
	ctx := context.Background()
	store := NewFromObject(loadSmallTestData())
	writeTxn := storage.NewTransactionOrDie(ctx, store, storage.WriteParams)
	modifiedPath := storage.MustParsePath("/a")
	expectedValue := "hello"

	var called bool
	_, err := store.Register(ctx, writeTxn, storage.TriggerConfig{
		OnCommit: func(ctx context.Context, txn storage.Transaction, evt storage.TriggerEvent) {
			if !evt.IsZero() {
				called = true
			}
		},
	})
	if err != nil {
		t.Fatalf("Failed to register callback: %v", err)
	}

	handle, err := store.Register(ctx, writeTxn, storage.TriggerConfig{
		OnCommit: func(ctx context.Context, txn storage.Transaction, evt storage.TriggerEvent) {
			if !evt.IsZero() {
				t.Fatalf("Callback should have been unregistered")
			}
		},
	})
	if err != nil {
		t.Fatalf("Failed to register callback: %v", err)
	}

	if err := store.Commit(ctx, writeTxn); err != nil {
		t.Fatalf("Unexpected commit error: %v", err)
	}

	writeTxn = storage.NewTransactionOrDie(ctx, store, storage.WriteParams)
	if err := store.Write(ctx, writeTxn, storage.AddOp, modifiedPath, expectedValue); err != nil {
		t.Fatalf("Failed to write to store: %v", err)
	}
	handle.Unregister(ctx, writeTxn)

	if err := store.Commit(ctx, writeTxn); err != nil {
		t.Fatalf("Unexpected commit error: %v", err)
	}

	if !called {
		t.Fatal("Registered callback was not called")
	}
}

func TestInMemoryContext(t *testing.T) {

	ctx := context.Background()
	store := New()
	params := storage.WriteParams
	params.Context = storage.NewContext()
	params.Context.Put("foo", "bar")

	txn, err := store.NewTransaction(ctx, params)
	if err != nil {
		t.Fatal(err)
	}

	_, err = store.Register(ctx, txn, storage.TriggerConfig{
		OnCommit: func(_ context.Context, _ storage.Transaction, event storage.TriggerEvent) {
			if event.Context.Get("foo") != "bar" {
				t.Fatalf("Expected foo/bar in context but got: %+v", event.Context)
			} else if event.Context.Get("deadbeef") != nil {
				t.Fatalf("Got unexpected deadbeef value in context: %+v", event.Context)
			}
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := store.Commit(ctx, txn); err != nil {
		t.Fatal(err)
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
		]
    }`), &data)
	if err != nil {
		panic(err)
	}
	return data
}

func TestOptRoundTripOnWrite(t *testing.T) {
	validObject := map[string]string{"foo": "bar"}

	// self-referential objects are not serializable to JSON.
	invalidObject := map[string]interface{}{}
	invalidObject["foo"] = invalidObject

	tests := []struct {
		name    string
		opts    []Opt
		obj     interface{}
		wantErr bool
	}{{
		name:    "success on valid object no Opts",
		opts:    nil,
		obj:     validObject,
		wantErr: false,
	}, {
		name:    "success on valid object round trip enabled",
		opts:    []Opt{OptRoundTripOnWrite(true)},
		obj:     validObject,
		wantErr: false,
	}, {
		name:    "success on valid object round trip disabled",
		opts:    []Opt{OptRoundTripOnWrite(false)},
		obj:     validObject,
		wantErr: false,
	}, {
		// Ensure the setting defaults to "true".
		name:    "failure on invalid object no Opts",
		opts:    nil,
		obj:     invalidObject,
		wantErr: true,
	}, {
		name:    "failure on invalid object round trip enabled",
		opts:    []Opt{OptRoundTripOnWrite(true)},
		obj:     invalidObject,
		wantErr: true,
	}, {
		// While this represents a bad use case, it's how we know the round-tripping
		// has been disabled.
		name:    "success on invalid object round trip disabled",
		opts:    []Opt{OptRoundTripOnWrite(false)},
		obj:     invalidObject,
		wantErr: false,
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := NewWithOpts(tt.opts...)
			ctx := context.Background()

			txn, err := db.NewTransaction(ctx, storage.WriteParams)
			if err != nil {
				t.Fatal(err)
			}

			err = db.Write(ctx, txn, storage.AddOp, []string{"data"}, tt.obj)
			if tt.wantErr && err == nil {
				t.Fatal("got Write error = nil, want error")
			} else if !tt.wantErr && err != nil {
				t.Fatalf("got Write error, want nil")
			}
		})
	}
}
