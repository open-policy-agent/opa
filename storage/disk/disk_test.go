// Copyright 2021 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package disk

import (
	"bytes"
	"context"
	"strings"
	"testing"

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

func TestPolicies(t *testing.T) {

	test.WithTempFS(map[string]string{}, func(dir string) {
		ctx := context.Background()
		s, err := New(ctx, Options{Dir: dir, Partitions: nil})
		if err != nil {
			t.Fatal(err)
		}

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

func TestDataPartitioningValidation(t *testing.T) {

	closeFn := func(ctx context.Context, s *Store) {
		t.Helper()
		if err := s.Close(ctx); err != nil {
			t.Fatal(err)
		}
	}

	test.WithTempFS(map[string]string{}, func(dir string) {

		ctx := context.Background()

		_, err := New(ctx, Options{Dir: dir, Partitions: []storage.Path{
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

		s, err := New(ctx, Options{Dir: dir, Partitions: []storage.Path{
			storage.MustParsePath("/foo/bar"),
			storage.MustParsePath("/foo/baz"),
		}})
		if err != nil {
			t.Fatal(err)
		}

		closeFn(ctx, s)

		s, err = New(ctx, Options{Dir: dir, Partitions: []storage.Path{
			storage.MustParsePath("/foo/baz"),
			storage.MustParsePath("/foo/bar"),
		}})
		if err != nil {
			t.Fatal(err)
		}

		closeFn(ctx, s)

		s, err = New(ctx, Options{Dir: dir, Partitions: []storage.Path{
			storage.MustParsePath("/foo/baz"),
			storage.MustParsePath("/foo/bar"),
			storage.MustParsePath("/foo/qux"),
		}})
		if err != nil {
			t.Fatal(err)
		}

		err = storage.WriteOne(ctx, s, storage.AddOp, storage.MustParsePath("/foo/corge"), "x")
		if err != nil {
			t.Fatal(err)
		}

		err = storage.WriteOne(ctx, s, storage.AddOp, storage.MustParsePath("/deadbeef"), "x")
		if err != nil {
			t.Fatal(err)
		}

		closeFn(ctx, s)

		s, err = New(ctx, Options{Dir: dir, Partitions: []storage.Path{
			storage.MustParsePath("/foo/baz"),
			storage.MustParsePath("/foo/bar"),
			storage.MustParsePath("/foo/qux/corge"),
		}})
		if err == nil || !strings.Contains(err.Error(), "partitions are backwards incompatible (old: [/foo/bar /foo/baz /foo/qux], new: [/foo/bar /foo/baz /foo/qux/corge], missing: [/foo/qux])") {
			t.Fatal(err)
		}

		closeFn(ctx, s)

		s, err = New(ctx, Options{Dir: dir, Partitions: []storage.Path{
			storage.MustParsePath("/foo/baz"),
			storage.MustParsePath("/foo/bar"),
			storage.MustParsePath("/foo/qux"),
			storage.MustParsePath("/foo/corge"),
		}})
		if err == nil || !strings.Contains(err.Error(), "partitions are backwards incompatible (existing data: /foo/corge)") {
			t.Fatal("expected to find existing key but got:", err)
		}

		closeFn(ctx, s)

		s, err = New(ctx, Options{Dir: dir, Partitions: []storage.Path{
			storage.MustParsePath("/foo/baz"),
			storage.MustParsePath("/foo/bar"),
			storage.MustParsePath("/foo/qux"),
			storage.MustParsePath("/foo/corge/grault"),
		}})
		if err == nil || !strings.Contains(err.Error(), "partitions are backwards incompatible (existing data: /foo/corge)") {
			t.Fatal("expected to find parent key but got:", err)
		}

		closeFn(ctx, s)

		s, err = New(ctx, Options{Dir: dir, Partitions: []storage.Path{
			storage.MustParsePath("/foo/baz"),
			storage.MustParsePath("/foo/bar"),
			storage.MustParsePath("/foo/qux"),
			storage.MustParsePath("/deadbeef"),
		}})
		if err == nil || !strings.Contains(err.Error(), "partitions are backwards incompatible (existing data: /deadbeef)") {
			t.Fatal("expected to find existing key but got:", err)
		}

		closeFn(ctx, s)
	})
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
			note:       "read-modify-write: add: array overwrite",
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
					exp:  `[8]`,
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
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			test.WithTempFS(map[string]string{}, func(dir string) {

				partitions := make([]storage.Path, len(tc.partitions))
				for i := range partitions {
					partitions[i] = storage.MustParsePath(tc.partitions[i])
				}

				ctx := context.Background()
				s, err := New(ctx, Options{Dir: dir, Partitions: partitions})

				if err != nil {
					t.Fatal(err)
				}

				for _, x := range tc.sequence {
					switch x := x.(type) {
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
				s, err := New(ctx, Options{Dir: dir, Partitions: partitions})

				if err != nil {
					t.Fatal(err)
				}

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
			note:       "unpartitioned",
			partitions: []string{"/foo"},
			sequence: []interface{}{
				testWriteError{
					op:    storage.AddOp,
					path:  "/does/notexist",
					value: `7`,
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
					value: `{}`,
				},
				testWriteError{
					op:    storage.AddOp,
					path:  "/deadbeef/x/y",
					value: `{}`,
				},
			},
		},
		{
			note:       "partitioned: nested",
			partitions: []string{"/foo"},
			sequence: []interface{}{
				testWriteError{
					op:    storage.AddOp,
					path:  "/foo/bar/baz",
					value: `{}`,
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
					value: `{}`,
				},
				testWriteError{
					op:    storage.AddOp,
					path:  "/foo/bar/baz/qux",
					value: `7`,
				},
			},
		},
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
				s, err := New(ctx, Options{Dir: dir, Partitions: partitions})

				if err != nil {
					t.Fatal(err)
				}

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
	test.WithTempFS(map[string]string{}, func(dir string) {
		ctx := context.Background()
		s, err := New(ctx, Options{Dir: dir, Partitions: []storage.Path{
			storage.MustParsePath("/foo"),
		}})
		if err != nil {
			t.Fatal(err)
		}

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
