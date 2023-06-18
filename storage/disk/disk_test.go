// Copyright 2021 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package disk

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/internal/file/archive"

	"github.com/open-policy-agent/opa/bundle"

	badger "github.com/dgraph-io/badger/v3"

	"github.com/open-policy-agent/opa/logging"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/util"
	"github.com/open-policy-agent/opa/util/test"
)

type testRead struct {
	path string
	exp  string
}

type testWrite struct {
	op    storage.PatchOp
	path  string
	value string
}

type testWriteError struct {
	op    storage.PatchOp
	path  string
	value string
}

// testCount lets you assert the number of keys under a prefix.
// Note that we don't do exact matches, so the assertions should be
// as exact as possible:
//
//	testCount{"/foo", 1}
//	testCount{"/foo/bar", 1}
//
// both of these would be true for one element under key `/foo/bar`.
type testCount struct {
	key   string
	count int
}

func (tc *testCount) assert(t *testing.T, s *Store) {
	t.Helper()
	key, err := s.pm.DataPath2Key(storage.MustParsePath(tc.key))
	if err != nil {
		t.Fatal(err)
	}

	opt := badger.DefaultIteratorOptions
	opt.PrefetchValues = false
	opt.Prefix = key

	var count int
	if err := s.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(opt)
		defer it.Close()
		for it.Rewind(); it.Valid(); it.Next() {
			count++
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if tc.count != count {
		t.Errorf("key %v: expected %d keys, found %d", tc.key, tc.count, count)
	}
}

type testDump struct{} // for debugging purposes

func (*testDump) do(t *testing.T, s *Store) {
	t.Helper()
	opt := badger.DefaultIteratorOptions
	opt.PrefetchValues = true

	if err := s.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(opt)
		defer it.Close()
		for it.Rewind(); it.Valid(); it.Next() {
			t.Logf("%v -> %v", string(it.Item().Key()), it.Item().ValueSize())
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestPolicies(t *testing.T) {

	test.WithTempFS(map[string]string{}, func(dir string) {
		ctx := context.Background()
		s, err := New(ctx, logging.NewNoOpLogger(), nil, Options{Dir: dir, Partitions: nil})
		if err != nil {
			t.Fatal(err)
		}
		defer s.Close(ctx)

		err = storage.Txn(ctx, s, storage.WriteParams, func(txn storage.Transaction) error {
			ids, err := s.ListPolicies(ctx, txn)
			if err != nil {
				t.Fatal(err)
			} else if len(ids) > 0 {
				t.Fatal("unexpected policies found")
			}

			_, err = s.GetPolicy(ctx, txn, "foo.rego")
			if err == nil {
				t.Fatal("expected error")
			}

			err = s.DeletePolicy(ctx, txn, "foo.rego")
			if err == nil {
				t.Fatal("expected error")
			}

			err = s.UpsertPolicy(ctx, txn, "foo.rego", []byte(`package foo`))
			if err != nil {
				t.Fatal(err)
			}

			ids, err = s.ListPolicies(ctx, txn)
			if err != nil {
				t.Fatal(err)
			} else if len(ids) != 1 || ids[0] != "foo.rego" {
				t.Fatalf("missing or unexpected policies found: %v", ids)
			}

			bs, err := s.GetPolicy(ctx, txn, "foo.rego")
			if err != nil || !bytes.Equal(bs, []byte("package foo")) {
				t.Fatalf("unexpected error or bad result: err=%v, result=%v", err, string(bs))
			}

			err = s.DeletePolicy(ctx, txn, "foo.rego")
			if err != nil {
				t.Fatal(err)
			}

			ids, err = s.ListPolicies(ctx, txn)
			if err != nil {
				t.Fatal(err)
			} else if len(ids) > 0 {
				t.Fatal("unexpected policies found")
			}

			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	})
}

func TestTruncateAbsoluteStoragePath(t *testing.T) {
	test.WithTempFS(map[string]string{}, func(dir string) {
		runTruncateTest(t, dir)
	})
}

func TestTruncateRelativeStoragePath(t *testing.T) {
	dir := "foobar"
	err := os.Mkdir(dir, 0700)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	runTruncateTest(t, dir)
}

func runTruncateTest(t *testing.T, dir string) {
	ctx := context.Background()
	s, err := New(ctx, logging.NewNoOpLogger(), nil, Options{Dir: dir, Partitions: nil})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close(ctx)

	txn := storage.NewTransactionOrDie(ctx, s, storage.WriteParams)

	var archiveFiles = map[string]string{
		"/a/b/c/data.json":   "[1,2,3]",
		"/a/b/d/data.json":   `e: true`,
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
	err = s.Truncate(ctx, txn, params, iterator)
	if err != nil {
		t.Fatalf("Unexpected truncate error: %v", err)
	}

	if err := s.Commit(ctx, txn); err != nil {
		t.Fatalf("Unexpected commit error: %v", err)
	}

	// check symlink not created
	symlink := filepath.Join(dir, symlinkKey)
	_, err = os.Lstat(symlink)
	if err == nil {
		t.Fatal("Expected error but got nil")
	}

	txn = storage.NewTransactionOrDie(ctx, s)

	actual, err := s.Read(ctx, txn, storage.MustParsePath("/"))
	if err != nil {
		t.Fatal(err)
	}

	expected := `
		{
			"a": {
				"b": {
					"c": [1,2,3],
					"d": {
						"e": true
					},
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

	s.Abort(ctx, txn)

	txn = storage.NewTransactionOrDie(ctx, s)
	ids, err := s.ListPolicies(ctx, txn)
	if err != nil {
		t.Fatal(err)
	}

	expectedIds := map[string]struct{}{"policy.rego": {}, "roles/policy.rego": {}}

	for _, id := range ids {
		if _, ok := expectedIds[id]; !ok {
			t.Fatalf("Expected list policies to contain %v but got: %v", expectedIds, id)
		}
	}

	bs, err := s.GetPolicy(ctx, txn, "policy.rego")
	expectedBytes := []byte("package foo\n p = 1")
	if err != nil || !reflect.DeepEqual(expectedBytes, bs) {
		t.Fatalf("Expected get policy to return %v but got: %v (err: %v)", expectedBytes, bs, err)
	}

	bs, err = s.GetPolicy(ctx, txn, "roles/policy.rego")
	expectedBytes = []byte("package bar\n p = 1")
	if err != nil || !reflect.DeepEqual(expectedBytes, bs) {
		t.Fatalf("Expected get policy to return %v but got: %v (err: %v)", expectedBytes, bs, err)
	}

	// Close and re-open store
	if err := s.Close(ctx); err != nil {
		t.Fatalf("store close: %v", err)
	}

	if _, err := New(ctx, logging.NewNoOpLogger(), nil, Options{Dir: dir, Partitions: nil}); err != nil {
		t.Fatalf("store re-open: %v", err)
	}
}

func TestTruncateMultipleTxn(t *testing.T) {
	test.WithTempFS(map[string]string{}, func(dir string) {
		ctx := context.Background()
		s, err := New(ctx, logging.NewNoOpLogger(), nil, Options{Dir: dir, Partitions: nil, Badger: "memtablesize=4000;valuethreshold=600"})
		if err != nil {
			t.Fatal(err)
		}
		defer s.Close(ctx)

		txn := storage.NewTransactionOrDie(ctx, s, storage.WriteParams)

		archiveFiles := map[string]string{}

		for i := 0; i < 20; i++ {

			path := fmt.Sprintf("users/user%d/data.json", i)

			obj := map[string][]byte{}
			obj[fmt.Sprintf("key%d", i)] = bytes.Repeat([]byte("a"), 1<<20) // 1 MB.

			bs, err := json.Marshal(obj)
			if err != nil {
				t.Fatal(err)
			}

			archiveFiles[path] = string(bs)
		}

		// additional data file at root
		archiveFiles["/data.json"] = `{"a": {"b": {"z": true}}}`

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
		err = s.Truncate(ctx, txn, params, iterator)
		if err != nil {
			t.Fatalf("Unexpected truncate error: %v", err)
		}

		if err := s.Commit(ctx, txn); err != nil {
			t.Fatalf("Unexpected commit error: %v", err)
		}

		txn = storage.NewTransactionOrDie(ctx, s)

		_, err = s.Read(ctx, txn, storage.MustParsePath("/users/user19"))
		if err != nil {
			t.Fatal(err)
		}

		s.Abort(ctx, txn)

		txn = storage.NewTransactionOrDie(ctx, s)

		actual, err := s.Read(ctx, txn, storage.MustParsePath("/a"))
		if err != nil {
			t.Fatal(err)
		}

		expected := `
		{
			"b": {
				"z": true
			}
		}
		`
		jsn := util.MustUnmarshalJSON([]byte(expected))

		if !reflect.DeepEqual(jsn, actual) {
			t.Fatalf("Expected reader's read to be %v but got: %v", jsn, actual)
		}
	})
}

func TestDataPartitioningValidation(t *testing.T) {

	closeFn := func(ctx context.Context, s *Store) {
		t.Helper()
		if s == nil {
			return
		}
		if err := s.Close(ctx); err != nil {
			t.Fatal(err)
		}
	}

	test.WithTempFS(map[string]string{}, func(dir string) {

		ctx := context.Background()

		_, err := New(ctx, logging.NewNoOpLogger(), nil, Options{Dir: dir, Partitions: []storage.Path{
			storage.MustParsePath("/foo/bar"),
			storage.MustParsePath("/foo/bar/baz"),
		}})

		if err == nil {
			t.Fatal("expected error")
		} else if sErr, ok := err.(*storage.Error); !ok {
			t.Fatal("expected storage error but got:", err)
		} else if sErr.Code != storage.InternalErr || sErr.Message != "partitions are overlapped: [/foo/bar /foo/bar/baz]" {
			t.Fatal("unexpected code or message, got:", err)
		}

		// set up two partitions
		s, err := New(ctx, logging.NewNoOpLogger(), nil, Options{Dir: dir, Partitions: []storage.Path{
			storage.MustParsePath("/foo/bar"),
			storage.MustParsePath("/foo/baz"),
		}})
		if err != nil {
			t.Fatal(err)
		}

		closeFn(ctx, s)

		// init with same settings: nothing wrong
		s, err = New(ctx, logging.NewNoOpLogger(), nil, Options{Dir: dir, Partitions: []storage.Path{
			storage.MustParsePath("/foo/baz"),
			storage.MustParsePath("/foo/bar"),
		}})
		if err != nil {
			t.Fatal(err)
		}

		closeFn(ctx, s)

		// adding another partition
		s, err = New(ctx, logging.NewNoOpLogger(), nil, Options{Dir: dir, Partitions: []storage.Path{
			storage.MustParsePath("/foo/baz"),
			storage.MustParsePath("/foo/bar"),
			storage.MustParsePath("/foo/qux"),
		}})
		if err != nil {
			t.Fatal(err)
		}

		// We're writing data under the partitions: this affects how
		// some partition changes are treated: if they don't affect existing
		// data, they are accepted.
		err = storage.WriteOne(ctx, s, storage.AddOp, storage.MustParsePath("/foo/corge"), "x")
		if err != nil {
			t.Fatal(err)
		}

		err = storage.WriteOne(ctx, s, storage.AddOp, storage.MustParsePath("/deadbeef"), "x")
		if err != nil {
			t.Fatal(err)
		}

		closeFn(ctx, s)

		s, err = New(ctx, logging.NewNoOpLogger(), nil, Options{Dir: dir, Partitions: []storage.Path{
			storage.MustParsePath("/foo/baz"),
			storage.MustParsePath("/foo/bar"),
			storage.MustParsePath("/foo/qux/corge"),
		}})
		if err == nil || !strings.Contains(err.Error(),
			"partitions are backwards incompatible (old: [/foo/bar /foo/baz /foo/qux /system/*], new: [/foo/bar /foo/baz /foo/qux/corge /system/*], missing: [/foo/qux])") {
			t.Fatal(err)
		}

		closeFn(ctx, s)

		s, err = New(ctx, logging.NewNoOpLogger(), nil, Options{Dir: dir, Partitions: []storage.Path{
			storage.MustParsePath("/foo/baz"),
			storage.MustParsePath("/foo/bar"),
			storage.MustParsePath("/foo/qux"),
			storage.MustParsePath("/foo/corge"),
		}})
		if err == nil || !strings.Contains(err.Error(), "partitions are backwards incompatible (existing data: /foo/corge)") {
			t.Fatal("expected to find existing key but got:", err)
		}

		closeFn(ctx, s)

		s, err = New(ctx, logging.NewNoOpLogger(), nil, Options{Dir: dir, Partitions: []storage.Path{
			storage.MustParsePath("/foo/baz"),
			storage.MustParsePath("/foo/bar"),
			storage.MustParsePath("/foo/qux"),
			storage.MustParsePath("/foo/corge/grault"),
		}})
		if err == nil || !strings.Contains(err.Error(), "partitions are backwards incompatible (existing data: /foo/corge)") {
			t.Fatal("expected to find parent key but got:", err)
		}

		closeFn(ctx, s)

		s, err = New(ctx, logging.NewNoOpLogger(), nil, Options{Dir: dir, Partitions: []storage.Path{
			storage.MustParsePath("/foo/baz"),
			storage.MustParsePath("/foo/bar"),
			storage.MustParsePath("/foo/qux"),
			storage.MustParsePath("/deadbeef"),
		}})
		if err == nil || !strings.Contains(err.Error(), "partitions are backwards incompatible (existing data: /deadbeef)") {
			t.Fatal("expected to find existing key but got:", err)
		}

		closeFn(ctx, s)

		// switching to wildcard partition
		s, err = New(ctx, logging.NewNoOpLogger(), nil, Options{Dir: dir, Partitions: []storage.Path{
			storage.MustParsePath("/foo/*"),
		}})
		if err != nil {
			t.Fatal(err)
		}
		closeFn(ctx, s)

		// adding another partition
		s, err = New(ctx, logging.NewNoOpLogger(), nil, Options{Dir: dir, Partitions: []storage.Path{
			storage.MustParsePath("/fox/in/the/snow/*"),
			storage.MustParsePath("/foo/*"),
		}})
		if err != nil {
			t.Fatal(err)
		}
		closeFn(ctx, s)

		// switching to a partition with multiple wildcards
		s, err = New(ctx, logging.NewNoOpLogger(), nil, Options{Dir: dir, Partitions: []storage.Path{
			storage.MustParsePath("/fox/in/*/*/*"),
			storage.MustParsePath("/foo/*"),
		}})
		if err != nil {
			t.Fatal(err)
		}
		closeFn(ctx, s)

		// there is no going back
		s, err = New(ctx, logging.NewNoOpLogger(), nil, Options{Dir: dir, Partitions: []storage.Path{
			storage.MustParsePath("/fox/in/the/snow/*"),
			storage.MustParsePath("/foo/*"),
		}})
		if err == nil || !strings.Contains(err.Error(),
			"partitions are backwards incompatible (old: [/foo/* /fox/in/*/*/* /system/*], new: [/foo/* /fox/in/the/snow/* /system/*], missing: [/fox/in/*/*/*])",
		) {
			t.Fatal(err)
		}
		closeFn(ctx, s)

		// adding a wildcard partition requires no content on the non-wildcard prefix
		// we open the db with previously used partitions, write another key, and
		// re-open with an extra wildcard partition
		// switching to a partition with multiple wildcards
		s, err = New(ctx, logging.NewNoOpLogger(), nil, Options{Dir: dir, Partitions: []storage.Path{
			storage.MustParsePath("/fox/in/*/*/*"),
			storage.MustParsePath("/foo/*"),
		}})
		if err != nil {
			t.Fatal(err)
		}
		err = storage.WriteOne(ctx, s, storage.AddOp, storage.MustParsePath("/peanutbutter/jelly"), true)
		if err != nil {
			t.Fatal(err)
		}
		closeFn(ctx, s)
		s, err = New(ctx, logging.NewNoOpLogger(), nil, Options{Dir: dir, Partitions: []storage.Path{
			storage.MustParsePath("/fox/in/*/*/*"),
			storage.MustParsePath("/peanutbutter/*"),
			storage.MustParsePath("/foo/*"),
		}})
		if err == nil || !strings.Contains(err.Error(), "partitions are backwards incompatible (existing data: /peanutbutter)") {
			t.Fatal("expected to find existing key but got:", err)
		}
		closeFn(ctx, s)
	})
}

func TestDataPartitioningSystemPartitions(t *testing.T) {
	ctx := context.Background()
	dir := "unused"

	for _, part := range []string{
		"/system",
		"/system/*",
		"/system/a",
		"/system/a/b",
	} {
		_, err := New(ctx, logging.NewNoOpLogger(), nil, Options{Dir: dir, Partitions: []storage.Path{
			storage.MustParsePath(part),
		}})
		if err == nil || !strings.Contains(err.Error(), "system partitions are managed") {
			t.Fatal(err)
		}
	}
}

func TestDataPartitioningReadsAndWrites(t *testing.T) {

	tests := []struct {
		note       string
		partitions []string
		sequence   []interface{}
	}{
		{
			note:       "exact-match: add",
			partitions: []string{"/foo"},
			sequence: []interface{}{
				testWrite{
					op:    storage.AddOp,
					path:  "/foo/bar",
					value: `"x"`,
				},
				testRead{
					path: "/foo/bar",
					exp:  `"x"`,
				},
				testCount{"/foo/bar", 1},
			},
		},
		{
			note:       "exact-match: add: multi-level",
			partitions: []string{"/foo/bar"},
			sequence: []interface{}{
				testWrite{
					op:    storage.AddOp,
					path:  "/foo/bar/baz",
					value: `"x"`,
				},
				testRead{
					path: "/foo/bar/baz",
					exp:  `"x"`,
				},
				testCount{"/foo/bar/baz", 1},
			},
		},
		{
			note:       "exact-match: unpartitioned",
			partitions: []string{"/foo"},
			sequence: []interface{}{
				testWrite{
					op:    storage.AddOp,
					path:  "/deadbeef",
					value: `"x"`,
				},
				testRead{
					path: "/deadbeef",
					exp:  `"x"`,
				},
				testCount{"/foo", 0},
				testCount{"/deadbeef", 1},
			},
		},
		{
			note:       "exact-match: remove",
			partitions: []string{"/foo"},
			sequence: []interface{}{
				testWrite{
					op:    storage.AddOp,
					path:  "/foo/bar",
					value: `7`,
				},
				testWrite{
					op:    storage.AddOp,
					path:  "/foo/baz",
					value: `8`,
				},
				testWrite{
					op:   storage.RemoveOp,
					path: "/foo/bar",
				},
				testRead{
					path: "/foo",
					exp:  `{"baz": 8}`,
				},
			},
		},
		{
			note:       "read: sub-field",
			partitions: []string{"/foo"},
			sequence: []interface{}{
				testWrite{
					op:    storage.AddOp,
					path:  "/foo/bar",
					value: `{"baz": 8}`,
				},
				testRead{
					path: "/foo/bar/baz",
					exp:  `8`,
				},
				testCount{"/foo/bar", 1},
				testCount{"/foo/bar/baz", 0},
			},
		},
		{
			note:       "read-modify-write: add",
			partitions: []string{"/foo"},
			sequence: []interface{}{
				testWrite{
					op:    storage.AddOp,
					path:  "/foo/bar",
					value: `{"baz": 7}`,
				},
				testWrite{
					op:    storage.AddOp,
					path:  "/foo/bar/baz",
					value: `8`,
				},
				testRead{
					path: "/foo/bar",
					exp:  `{"baz": 8}`,
				},
			},
		},
		{
			note:       "read-modify-write: add: unpartitioned",
			partitions: []string{"/foo"},
			sequence: []interface{}{
				testWrite{
					op:    storage.AddOp,
					path:  "/deadbeef",
					value: `{"foo": 7}`,
				},
				testWrite{
					op:    storage.AddOp,
					path:  "/deadbeef/foo",
					value: `8`,
				},
				testRead{
					path: "/deadbeef",
					exp:  `{"foo": 8}`,
				},
			},
		},
		{
			note:       "read-modify-write: add: array append",
			partitions: []string{"/foo"},
			sequence: []interface{}{
				testWrite{
					op:    storage.AddOp,
					path:  "/foo/bar",
					value: `[]`,
				},
				testWrite{
					op:    storage.AddOp,
					path:  "/foo/bar/-",
					value: `8`,
				},
				testRead{
					path: "/foo/bar",
					exp:  `[8]`,
				},
			},
		},
		{
			note:       "read-modify-write: add: array append (via last index)",
			partitions: []string{"/foo"},
			sequence: []interface{}{
				testWrite{
					op:    storage.AddOp,
					path:  "/foo/bar",
					value: `[1]`,
				},
				testWrite{
					op:    storage.AddOp,
					path:  "/foo/bar/1",
					value: `8`,
				},
				testRead{
					path: "/foo/bar",
					exp:  `[1, 8]`,
				},
			},
		},
		{
			note:       "read-modify-write: add: array insert",
			partitions: []string{"/foo"},
			sequence: []interface{}{
				testWrite{
					op:    storage.AddOp,
					path:  "/foo/bar",
					value: `[7]`,
				},
				testWrite{
					op:    storage.AddOp,
					path:  "/foo/bar/0",
					value: `8`,
				},
				testRead{
					path: "/foo/bar",
					exp:  `[8, 7]`,
				},
			},
		},
		{
			note:       "read-modify-write: replace",
			partitions: []string{"/foo"},
			sequence: []interface{}{
				testWrite{
					op:    storage.AddOp,
					path:  "/foo/bar",
					value: `{"baz": 7}`,
				},
				testWrite{
					op:    storage.ReplaceOp,
					path:  "/foo/bar/baz",
					value: `8`,
				},
				testRead{
					path: "/foo/bar",
					exp:  `{"baz": 8}`,
				},
			},
		},
		{
			note:       "read-modify-write: replace: array",
			partitions: []string{"/foo"},
			sequence: []interface{}{
				testWrite{
					op:    storage.AddOp,
					path:  "/foo/bar",
					value: `[7]`,
				},
				testWrite{
					op:    storage.ReplaceOp,
					path:  "/foo/bar/0",
					value: `8`,
				},
				testRead{
					path: "/foo/bar",
					exp:  `[8]`,
				},
			},
		},
		{
			note:       "read-modify-write: remove",
			partitions: []string{"/foo"},
			sequence: []interface{}{
				testWrite{
					op:    storage.AddOp,
					path:  "/foo/bar",
					value: `{"baz": 7}`,
				},
				testWrite{
					op:   storage.RemoveOp,
					path: "/foo/bar/baz",
				},
				testRead{
					path: "/foo/bar",
					exp:  `{}`,
				},
			},
		},
		{
			note:       "read-modify-write: remove: array",
			partitions: []string{"/foo"},
			sequence: []interface{}{
				testWrite{
					op:    storage.AddOp,
					path:  "/foo/bar",
					value: `[7, 8]`,
				},
				testWrite{
					op:   storage.RemoveOp,
					path: "/foo/bar/0",
				},
				testRead{
					path: "/foo/bar",
					exp:  `[8]`,
				},
			},
		},
		{
			note:       "read-modify-write: multi-level: map",
			partitions: []string{"/foo"},
			sequence: []interface{}{
				testWrite{
					op:    storage.AddOp,
					path:  "/foo/bar",
					value: `{"baz": {"qux": {"corge": 7}}}`,
				},
				testWrite{
					op:    storage.ReplaceOp,
					path:  "/foo/bar/baz/qux/corge",
					value: "8",
				},
				testRead{
					path: "/foo/bar",
					exp:  `{"baz": {"qux": {"corge": 8}}}`,
				},
			},
		},
		{
			note:       "read-modify-write: multi-level: array",
			partitions: []string{"/foo"},
			sequence: []interface{}{
				testWrite{
					op:    storage.AddOp,
					path:  "/foo/bar",
					value: `{"baz": [{"qux": {"corge": 7}}]}`,
				},
				testWrite{
					op:    storage.ReplaceOp,
					path:  "/foo/bar/baz/0/qux/corge",
					value: "8",
				},
				testRead{
					path: "/foo/bar",
					exp:  `{"baz": [{"qux": {"corge": 8}}]}`,
				},
			},
		},
		{
			note:       "prefix",
			partitions: []string{"/foo"},
			sequence: []interface{}{
				testWrite{
					op:    storage.AddOp,
					path:  "/foo",
					value: `{"bar": 7, "baz": 8}`,
				},

				testCount{"/foo", 2},
				testRead{
					path: "/foo/bar",
					exp:  `7`,
				},
				testRead{
					path: "/foo/baz",
					exp:  `8`,
				},
				testRead{
					path: "/foo",
					exp:  `{"bar": 7, "baz": 8}`,
				},
			},
		},
		{
			note: "prefix: unpartitioned: root",
			sequence: []interface{}{
				testWrite{
					op:    storage.AddOp,
					path:  "/deadbeef",
					value: `7`,
				},
				testRead{
					path: "/",
					exp:  `{"deadbeef": 7}`,
				},
			},
		},
		{
			note:       "prefix: unpartitioned: mixed",
			partitions: []string{"/foo"},
			sequence: []interface{}{
				testWrite{
					op:    storage.AddOp,
					path:  "/",
					value: `{"foo": {"bar": 7, "baz": 8}, "deadbeef": 9}`,
				},
				testRead{
					path: "/",
					exp:  `{"foo": {"bar": 7, "baz": 8}, "deadbeef": 9}`,
				},
				testRead{
					path: "/foo/bar",
					exp:  `7`,
				},
				testRead{
					path: "/foo/baz",
					exp:  `8`,
				},
				testRead{
					path: "/foo",
					exp:  `{"bar": 7, "baz": 8}`,
				},
				testRead{
					path: "/deadbeef",
					exp:  `9`,
				},
			},
		},
		{
			note:       "prefix: overwrite",
			partitions: []string{"/foo"},
			sequence: []interface{}{
				testWrite{
					op:    storage.AddOp,
					path:  "/",
					value: `{"foo": {"bar": 7, "baz": 8}, "deadbeef": 9}`,
				},
				testWrite{
					op:    storage.AddOp,
					path:  "/foo",
					value: `{"qux": 10, "baz": 8}`,
				},
				testRead{
					path: "/",
					exp:  `{"foo": {"qux": 10, "baz": 8}, "deadbeef": 9}`,
				},
			},
		},
		{
			note:       "prefix: remove",
			partitions: []string{"/foo"},
			sequence: []interface{}{
				testWrite{
					op:   storage.AddOp,
					path: "/",
					value: `{
						"foo": {
							"bar": 7,
							"baz": 8
						},
						"deadbeef": 9
					}`,
				},
				testWrite{
					op:   storage.RemoveOp,
					path: "/foo",
				},
				testRead{
					path: "/",
					exp:  `{"deadbeef": 9}`,
				},
			},
		},
		{
			note: "issue-3711: string-to-number conversion",
			sequence: []interface{}{
				testWrite{
					op:   storage.AddOp,
					path: "/",
					value: `{
						"2": 7
					}`,
				},
				testRead{
					path: "/2",
					exp:  `7`,
				},
			},
		},
		{
			note:       "pattern partitions: middle wildcard: match",
			partitions: []string{"/foo/*/bar"},
			sequence: []interface{}{
				testWrite{
					op:    storage.AddOp,
					path:  "/foo/a/bar",
					value: `{"baz": 7}`,
				},
				testCount{"/foo/a/bar/baz", 1},
				testRead{"/foo/a/bar/baz", `7`},
			},
		},
		{
			note:       "pattern partitions: middle wildcard: no-match",
			partitions: []string{"/foo/*/bar"},
			sequence: []interface{}{
				testWrite{
					op:    storage.AddOp,
					path:  "/foo/b/baz",
					value: `{"quz": 1}`,
				},
				testCount{"/foo/b/baz", 1},
				testCount{"/foo/b/baz/quz", 0},
				testRead{"/foo/b/baz/quz", `1`},
			},
		},
		{
			note:       "pattern partitions: middle wildcard: partial match",
			partitions: []string{"/foo/*/bar"},
			sequence: []interface{}{
				testWrite{
					op:    storage.AddOp,
					path:  "/foo/b",
					value: `{"bar": {"quz": 1}, "x": "y"}`,
				},
				testCount{"/foo/b/bar/quz", 1},
				testRead{"/foo/b/bar/quz", `1`},
				testCount{"/foo/b/x", 1},
				testRead{"/foo/b/x", `"y"`},
			},
		},
		{
			note:       "pattern partitions: 2x middle wildcard: partial match",
			partitions: []string{"/foo/*/*/bar"},
			sequence: []interface{}{
				testWrite{
					op:    storage.AddOp,
					path:  "/foo/b/c",
					value: `{"bar": {"quz": 1}, "x": "y"}`,
				},
				testCount{"/foo/b/c/bar/quz", 1},
				testRead{"/foo/b/c/bar/quz", `1`},
				testCount{"/foo/b/c/x", 1},
				testRead{"/foo/b/c/x", `"y"`},
			},
		},
		{
			note:       "pattern partitions: wildcard at the end",
			partitions: []string{"/users/*"},
			sequence: []interface{}{
				testWrite{
					op:    storage.AddOp,
					path:  "/users",
					value: `{"alice": {"bar": {"quz": 1}, "x": "y"}}`,
				},
				testWrite{
					op:    storage.AddOp,
					path:  "/users/bob",
					value: `{"baz": {"one": 1}, "y": "x"}`,
				},
				testCount{"/users/alice/bar", 1},
				testRead{"/users/alice/bar/quz", `1`},
				testCount{"/users/alice/x", 1},
				testRead{"/users/alice/x", `"y"`},
				testCount{"/users/bob/baz", 1},
				testRead{"/users/bob/baz/one", `1`},
				testCount{"/users/bob/y", 1},
				testRead{"/users/bob/y", `"x"`},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			test.WithTempFS(map[string]string{}, func(dir string) {

				partitions := make([]storage.Path, len(tc.partitions))
				for i := range partitions {
					partitions[i] = storage.MustParsePath(tc.partitions[i])
				}

				ctx := context.Background()
				s, err := New(ctx, logging.NewNoOpLogger(), nil, Options{Dir: dir, Partitions: partitions})
				if err != nil {
					t.Fatal(err)
				}
				defer s.Close(ctx)

				for _, x := range tc.sequence {
					switch x := x.(type) {
					case testCount:
						x.assert(t, s)
					case testWrite:
						executeTestWrite(ctx, t, s, x)
					case testRead:
						result, err := storage.ReadOne(ctx, s, storage.MustParsePath(x.path))
						if err != nil {
							t.Fatal(err)
						}
						var exp interface{}
						if x.exp != "" {
							exp = util.MustUnmarshalJSON([]byte(x.exp))
						}
						if cmp := util.Compare(result, exp); cmp != 0 {
							t.Fatalf("expected %v but got %v", x.exp, result)
						}
					case testDump:
						x.do(t, s)
					default:
						panic("unexpected type")
					}
				}
			})
		})
	}

}

func TestDataPartitioningReadNotFoundErrors(t *testing.T) {
	tests := []struct {
		note       string
		partitions []string
		sequence   []interface{}
	}{
		{
			note:       "unpartitioned: key",
			partitions: []string{"/foo"},
			sequence: []interface{}{
				testWrite{
					op:    storage.AddOp,
					path:  "/foo/bar",
					value: `"x"`,
				},
				testRead{
					path: "/deadbeef",
				},
			},
		},
		{
			note:       "unpartitioned: nested",
			partitions: []string{"/foo"},
			sequence: []interface{}{
				testWrite{
					op:    storage.AddOp,
					path:  "/deadbeef",
					value: `{"x": 7}`,
				},
				testRead{
					path: "/deadbeef/y",
				},
			},
		},
		{
			note:       "unpartitioned: nested: 2-level",
			partitions: []string{"/foo"},
			sequence: []interface{}{
				testWrite{
					op:    storage.AddOp,
					path:  "/deadbeef",
					value: `{"x": 7}`,
				},
				testRead{
					path: "/deadbeef/x/y",
				},
			},
		},
		{
			note:       "partitioned: key",
			partitions: []string{"/foo"},
			sequence: []interface{}{
				testWrite{
					op:    storage.AddOp,
					path:  "/foo/bar",
					value: `"x"`,
				},
				testRead{
					path: "/foo/baz",
				},
			},
		},
		{
			note:       "partitioned: nested",
			partitions: []string{"/foo"},
			sequence: []interface{}{
				testWrite{
					op:    storage.AddOp,
					path:  "/foo/bar",
					value: `{"x": 7}`,
				},
				testRead{
					path: "/foo/bar/y",
				},
			},
		},
		{
			note:       "partitioned: nested: 2-level",
			partitions: []string{"/foo"},
			sequence: []interface{}{
				testWrite{
					op:    storage.AddOp,
					path:  "/foo/bar",
					value: `{"x": 7}`,
				},
				testRead{
					path: "/foo/bar/x/y",
				},
			},
		},
		{
			note:       "partitioned: prefix",
			partitions: []string{"/foo", "/bar"},
			sequence: []interface{}{
				testWrite{
					op:    storage.AddOp,
					path:  "/foo/bar",
					value: `{"x": 7}`,
				},
				testRead{
					path: "/bar",
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			test.WithTempFS(map[string]string{}, func(dir string) {

				partitions := make([]storage.Path, len(tc.partitions))
				for i := range partitions {
					partitions[i] = storage.MustParsePath(tc.partitions[i])
				}

				ctx := context.Background()
				s, err := New(ctx, logging.NewNoOpLogger(), nil, Options{Dir: dir, Partitions: partitions})
				if err != nil {
					t.Fatal(err)
				}
				defer s.Close(ctx)

				for _, x := range tc.sequence {
					switch x := x.(type) {
					case testWrite:
						executeTestWrite(ctx, t, s, x)
					case testRead:
						_, err := storage.ReadOne(ctx, s, storage.MustParsePath(x.path))
						if err == nil {
							t.Fatal("expected error")
						} else if sErr, ok := err.(*storage.Error); !ok {
							t.Fatal("expected storage error but got:", err)
						} else if sErr.Code != storage.NotFoundErr {
							t.Fatal("expected not found error but got:", err)
						}
					default:
						panic("unexpected type")
					}
				}
			})
		})
	}
}

func TestDataPartitioningWriteNotFoundErrors(t *testing.T) {
	tests := []struct {
		note       string
		partitions []string
		sequence   []interface{}
	}{
		{
			note:       "patch: remove: non-existent key",
			partitions: []string{},
			sequence: []interface{}{
				testWrite{
					op:    storage.AddOp,
					path:  "/foo",
					value: `{}`,
				},
				testWriteError{
					op:   storage.RemoveOp,
					path: "/foo/bar",
				},
			},
		},
		{
			note:       "patch: replace: non-existent key",
			partitions: []string{},
			sequence: []interface{}{
				testWrite{
					op:    storage.AddOp,
					path:  "/foo",
					value: `{}`,
				},
				testWriteError{
					op:    storage.ReplaceOp,
					path:  "/foo/bar",
					value: `7`,
				},
			},
		},
		{
			note:       "patch: scalar",
			partitions: []string{},
			sequence: []interface{}{
				testWrite{
					op:    storage.AddOp,
					path:  "/foo",
					value: `{"bar": 7}`,
				},
				testWriteError{
					op:   storage.RemoveOp,
					path: "/foo/bar/baz",
				},
			},
		},
		{
			note:       "patch: array index",
			partitions: []string{},
			sequence: []interface{}{
				testWrite{
					op:    storage.AddOp,
					path:  "/foo",
					value: `[1,2,3]`,
				},
				testWriteError{
					op:   storage.RemoveOp,
					path: "/foo/7",
				},
			},
		},
		{
			note:       "patch: array index: non-leaf",
			partitions: []string{},
			sequence: []interface{}{
				testWrite{
					op:    storage.AddOp,
					path:  "/foo",
					value: `[{"bar": 7}, {"baz": 8}]`,
				},
				testWriteError{
					op:   storage.RemoveOp,
					path: "/foo/7/bar",
				},
			},
		},
		{
			note:       "patch: array: non-existent key",
			partitions: []string{},
			sequence: []interface{}{
				testWrite{
					op:    storage.AddOp,
					path:  "/foo",
					value: `[{"bar": 7}, {"baz": 8}]`,
				},
				testWriteError{
					op:   storage.RemoveOp,
					path: "/foo/1/bar", // index 1 contains baz not bar
				},
			},
		},
		{
			note:       "patch: array: scalar",
			partitions: []string{},
			sequence: []interface{}{
				testWrite{
					op:    storage.AddOp,
					path:  "/foo",
					value: `7`,
				},
				testWriteError{
					op:   storage.RemoveOp,
					path: "/foo/1/bar",
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			test.WithTempFS(map[string]string{}, func(dir string) {

				partitions := make([]storage.Path, len(tc.partitions))
				for i := range partitions {
					partitions[i] = storage.MustParsePath(tc.partitions[i])
				}

				ctx := context.Background()
				s, err := New(ctx, logging.NewNoOpLogger(), nil, Options{Dir: dir, Partitions: partitions})

				if err != nil {
					t.Fatal(err)
				}
				defer s.Close(ctx)

				for _, x := range tc.sequence {
					switch x := x.(type) {
					case testWrite:
						executeTestWrite(ctx, t, s, x)
					case testWriteError:
						var val interface{}
						if x.value != "" {
							val = util.MustUnmarshalJSON([]byte(x.value))
						}
						err := storage.WriteOne(ctx, s, x.op, storage.MustParsePath(x.path), val)
						if err == nil {
							t.Fatal("expected error")
						} else if sErr, ok := err.(*storage.Error); !ok {
							t.Fatal("expected storage error but got:", err)
						} else if sErr.Code != storage.NotFoundErr {
							t.Fatal("expected not found error but got:", err)
						}
					default:
						panic("unexpected type")
					}
				}
			})
		})
	}
}

func TestDataPartitioningWriteInvalidPatchError(t *testing.T) {
	for _, pt := range []string{"/*", "/foo"} {
		t.Run(pt, func(t *testing.T) {
			test.WithTempFS(map[string]string{}, func(dir string) {
				ctx := context.Background()
				s, err := New(ctx, logging.NewNoOpLogger(), nil, Options{Dir: dir, Partitions: []storage.Path{
					storage.MustParsePath("/foo"),
				}})
				if err != nil {
					t.Fatal(err)
				}
				defer s.Close(ctx)

				err = storage.WriteOne(ctx, s, storage.AddOp, storage.MustParsePath("/foo"), util.MustUnmarshalJSON([]byte(`[1,2,3]`)))
				if err == nil {
					t.Fatal("expected error")
				} else if sErr, ok := err.(*storage.Error); !ok {
					t.Fatal("expected storage error but got:", err)
				} else if sErr.Code != storage.InvalidPatchErr {
					t.Fatal("expected invalid patch error but got:", err)
				}

				err = storage.WriteOne(ctx, s, storage.AddOp, storage.MustParsePath("/"), util.MustUnmarshalJSON([]byte(`{"foo": [1,2,3]}`)))
				if err == nil {
					t.Fatal("expected error")
				} else if sErr, ok := err.(*storage.Error); !ok {
					t.Fatal("expected storage error but got:", err)
				} else if sErr.Code != storage.InvalidPatchErr {
					t.Fatal("expected invalid patch error but got:", err)
				}

				err = storage.WriteOne(ctx, s, storage.AddOp, storage.MustParsePath("/"), util.MustUnmarshalJSON([]byte(`[1,2,3]`)))
				if err == nil {
					t.Fatal("expected error")
				} else if sErr, ok := err.(*storage.Error); !ok {
					t.Fatal("expected storage error but got:", err)
				} else if sErr.Code != storage.InvalidPatchErr {
					t.Fatal("expected invalid patch error but got:", err)
				}
			})
		})
	}
}

func executeTestWrite(ctx context.Context, t *testing.T, s storage.Store, x testWrite) {
	t.Helper()
	var val interface{}
	if x.value != "" {
		val = util.MustUnmarshalJSON([]byte(x.value))
	}
	err := storage.WriteOne(ctx, s, x.op, storage.MustParsePath(x.path), val)
	if err != nil {
		t.Fatal(err)
	}
}

func TestDiskTriggers(t *testing.T) {
	test.WithTempFS(map[string]string{}, func(dir string) {
		ctx := context.Background()
		store, err := New(ctx, logging.NewNoOpLogger(), nil, Options{Dir: dir, Partitions: []storage.Path{
			storage.MustParsePath("/foo"),
		}})
		if err != nil {
			t.Fatal(err)
		}
		defer store.Close(ctx)
		writeTxn := storage.NewTransactionOrDie(ctx, store, storage.WriteParams)
		readTxn := storage.NewTransactionOrDie(ctx, store)

		_, err = store.Register(ctx, readTxn, storage.TriggerConfig{
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
	})
}

func TestLookup(t *testing.T) {
	cases := []struct {
		note     string
		input    []byte
		path     string
		expected []byte
	}{
		{
			note:     "empty path",
			input:    []byte(`{"hello": "world"}`),
			path:     "",
			expected: []byte(`{"hello": "world"}`),
		},
		{
			note:     "single path",
			input:    []byte(`{"a": {"b": {"c": "d"}}}`),
			path:     "a",
			expected: []byte(`{"b": {"c": "d"}}`),
		},
		{
			note:     "nested path-1",
			input:    []byte(`{"a": {"b": {"c": "d"}}}`),
			path:     "a/b",
			expected: []byte(`{"c": "d"}`),
		},
		{
			note:     "nested path-2",
			input:    []byte(`{"a": {"b": {"c": {"d": [1,2,3]}}}}`),
			path:     "a/b/c",
			expected: []byte(`{"d": [1,2,3]}`),
		},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {

			path, ok := storage.ParsePathEscaped("/" + tc.path)
			if !ok {
				t.Fatalf("storage path invalid: %v", path)
			}

			result, _, err := lookup(path, tc.input)
			if err != nil {
				t.Fatalf("Unexpected error %v", err)
			}

			switch v := result.(type) {
			case map[string]json.RawMessage:
				var obj map[string]json.RawMessage
				err := util.Unmarshal(tc.expected, &obj)
				if err != nil {
					t.Fatalf("Unexpected error %v", err)
				}

				if !reflect.DeepEqual(v, obj) {
					t.Fatalf("Expected result %v, got %v", obj, result)
				}
			case json.RawMessage:
				if !bytes.Equal(v, tc.expected) {
					t.Fatalf("Expected result %v, got %v", tc.expected, result)
				}
			}
		})
	}
}

func TestDiskDiagnostics(t *testing.T) {
	ctx := context.Background()

	t.Run("no partitions", func(t *testing.T) {
		test.WithTempFS(nil, func(dir string) {
			buf := bytes.Buffer{}
			logger := logging.New()
			logger.SetOutput(&buf)
			logger.SetLevel(logging.Debug)
			store, err := New(ctx, logger, nil, Options{Dir: dir})
			if err != nil {
				t.Fatal(err)
			}
			// store something, won't show up in the logs yet (they're calculated on startup only)
			if err := storage.WriteOne(ctx, store, storage.AddOp, storage.MustParsePath("/foo"), util.MustUnmarshalJSON([]byte(`{"baz": 1000}`))); err != nil {
				t.Fatal(err)
			}
			if err := storage.WriteOne(ctx, store, storage.AddOp, storage.MustParsePath("/bar"), util.MustUnmarshalJSON([]byte(`{"quz": 2000}`))); err != nil {
				t.Fatal(err)
			}
			if err := store.Close(ctx); err != nil {
				t.Fatal(err)
			}

			expected := []string{
				`level=warning msg="no partitions configured"`,
				`level=debug msg="partition /: key count: 0 (estimated size 0 bytes)"`,
			}
			for _, exp := range expected {
				if !strings.Contains(buf.String(), exp) {
					t.Errorf("expected string %q not found in logs", exp)
				}
			}
			if t.Failed() {
				t.Log("log output: ", buf.String())
			}

			// re-open
			buf = bytes.Buffer{}
			logger = logging.New()
			logger.SetOutput(&buf)
			logger.SetLevel(logging.Debug)
			store, err = New(ctx, logger, nil, Options{Dir: dir})
			if err != nil {
				t.Fatal(err)
			}
			if err := store.Close(ctx); err != nil {
				t.Fatal(err)
			}

			expected = []string{
				`level=debug msg="partition /: key count: 2 (estimated size 50 bytes)"`,
			}
			for _, exp := range expected {
				if !strings.Contains(buf.String(), exp) {
					t.Errorf("expected string %q not found in logs", exp)
				}
			}
			if t.Failed() {
				t.Log("log oputput: ", buf.String())
			}
		})
	})

	t.Run("two partitions", func(t *testing.T) {
		test.WithTempFS(nil, func(dir string) {
			opts := Options{
				Dir: dir,
				Partitions: []storage.Path{
					storage.MustParsePath("/foo"),
					storage.MustParsePath("/bar"),
				}}
			buf := bytes.Buffer{}
			logger := logging.New()
			logger.SetOutput(&buf)
			logger.SetLevel(logging.Debug)
			store, err := New(ctx, logger, nil, opts)
			if err != nil {
				t.Fatal(err)
			}

			// store something, won't show up in the logs yet (they're calculated on startup only)
			if err := storage.WriteOne(ctx, store, storage.AddOp, storage.MustParsePath("/foo"), util.MustUnmarshalJSON([]byte(`{"baz": 1000}`))); err != nil {
				t.Fatal(err)
			}

			if err := store.Close(ctx); err != nil {
				t.Fatal(err)
			}

			expected := []string{
				`level=debug msg="partition /bar: key count: 0 (estimated size 0 bytes)"`,
				`level=debug msg="partition /foo: key count: 0 (estimated size 0 bytes)"`,
			}
			for _, exp := range expected {
				if !strings.Contains(buf.String(), exp) {
					t.Errorf("expected string %q not found in logs", exp)
				}
			}
			if t.Failed() {
				t.Log("log oputput: ", buf.String())
			}

			// re-open
			buf = bytes.Buffer{}
			logger = logging.New()
			logger.SetOutput(&buf)
			logger.SetLevel(logging.Debug)
			store, err = New(ctx, logger, nil, opts)
			if err != nil {
				t.Fatal(err)
			}
			if err := store.Close(ctx); err != nil {
				t.Fatal(err)
			}

			expected = []string{
				`level=debug msg="partition /bar: key count: 0 (estimated size 0 bytes)"`,
				`level=debug msg="partition /foo: key count: 1 (estimated size 21 bytes)"`,
			}
			for _, exp := range expected {
				if !strings.Contains(buf.String(), exp) {
					t.Errorf("expected string %q not found in logs", exp)
				}
			}
			if t.Failed() {
				t.Log("log oputput: ", buf.String())
			}
		})
	})

	t.Run("patterned partitions", func(t *testing.T) {
		test.WithTempFS(nil, func(dir string) {
			opts := Options{Dir: dir,
				Partitions: []storage.Path{
					storage.MustParsePath("/foo/*/bar"),
					storage.MustParsePath("/bar"),
				}}
			buf := bytes.Buffer{}
			logger := logging.New()
			logger.SetOutput(&buf)
			logger.SetLevel(logging.Debug)
			store, err := New(ctx, logger, nil, opts)
			if err != nil {
				t.Fatal(err)
			}

			// store something, won't show up in the logs yet (they're calculated on startup only)
			if err := storage.WriteOne(ctx, store, storage.AddOp, storage.MustParsePath("/foo/x/bar"), util.MustUnmarshalJSON([]byte(`{"baz": 1000}`))); err != nil {
				t.Fatal(err)
			}
			if err := storage.WriteOne(ctx, store, storage.AddOp, storage.MustParsePath("/bar"), util.MustUnmarshalJSON([]byte(`{"quz": 1000}`))); err != nil {
				t.Fatal(err)
			}

			if err := store.Close(ctx); err != nil {
				t.Fatal(err)
			}

			expected := []string{
				`level=debug msg="partition /bar: key count: 0 (estimated size 0 bytes)"`,
				`level=debug msg="partition pattern /foo/*/bar: key count: 0 (estimated size 0 bytes)"`,
			}
			for _, exp := range expected {
				if !strings.Contains(buf.String(), exp) {
					t.Errorf("expected string %q not found in logs", exp)
				}
			}
			if t.Failed() {
				t.Log("log oputput: ", buf.String())
			}

			// re-open
			buf = bytes.Buffer{}
			logger = logging.New()
			logger.SetOutput(&buf)
			logger.SetLevel(logging.Debug)
			store, err = New(ctx, logger, nil, opts)
			if err != nil {
				t.Fatal(err)
			}
			if err := store.Close(ctx); err != nil {
				t.Fatal(err)
			}

			expected = []string{
				`level=debug msg="partition /bar: key count: 1 (estimated size 21 bytes)"`,
				`level=debug msg="partition /foo/x/bar (pattern /foo/*/bar): key count: 1 (estimated size 27 bytes)"`,
			}
			for _, exp := range expected {
				if !strings.Contains(buf.String(), exp) {
					t.Errorf("expected string %q not found in logs", exp)
				}
			}
			if t.Failed() {
				t.Log("log oputput: ", buf.String())
			}
		})
	})
}
