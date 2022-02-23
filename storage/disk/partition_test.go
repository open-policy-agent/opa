// Copyright 2021 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package disk

import (
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/storage"
)

func TestPartitionTrie(t *testing.T) {

	// Build simple trie
	root := buildPartitionTrie([]storage.Path{
		storage.MustParsePath("/foo/bar"),
		storage.MustParsePath("/foo/baz/qux"),
		storage.MustParsePath("/corge"),
		storage.MustParsePath("/tenants/*/bindings"), // wildcard in the middle
		storage.MustParsePath("/users/*"),            // wildcard at the end
	})

	// Assert on counts...
	if exp, act := 4, len(root.partitions); exp != act {
		t.Fatalf("expected root to contain %d partitions, got %d", exp, act)
	}

	if len(root.partitions["foo"].partitions) != 2 {
		t.Fatal("expected foo to contain two partitions")
	}

	if len(root.partitions["foo"].partitions["baz"].partitions) != 1 {
		t.Fatal("expected baz to contain one child")
	}

	tests := []struct {
		path    string
		wantIdx int
		wantPtr *partitionTrie
	}{
		{
			path:    "/",
			wantIdx: 0,
			wantPtr: root,
		}, {
			path:    "/foo",
			wantIdx: 1,
			wantPtr: root.partitions["foo"],
		}, {
			path:    "/foo/bar",
			wantIdx: 2,
			wantPtr: root.partitions["foo"].partitions["bar"],
		}, {
			path:    "/foo/bar/baz",
			wantIdx: 3,
			wantPtr: nil,
		}, {
			path:    "/foo/bar/baz/qux",
			wantIdx: 3,
			wantPtr: nil,
		}, {
			path:    "/foo/baz",
			wantIdx: 2,
			wantPtr: root.partitions["foo"].partitions["baz"],
		}, {
			path:    "/foo/baz/deadbeef",
			wantIdx: 3,
			wantPtr: nil,
		}, {
			path:    "/foo/baz/qux",
			wantIdx: 3,
			wantPtr: root.partitions["foo"].partitions["baz"].partitions["qux"],
		}, {
			path:    "/foo/baz/qux/deadbeef",
			wantIdx: 4,
			wantPtr: nil,
		}, {
			path:    "/foo/corge",
			wantIdx: 2,
			wantPtr: nil,
		}, {
			path:    "/deadbeef",
			wantIdx: 1,
			wantPtr: nil,
		}, {
			path:    "/tenants/deadbeef/bindings/user01",
			wantIdx: 4,
			wantPtr: nil,
		}, {
			path:    "/tenants/deadbeef/bindings",
			wantIdx: 3,
			wantPtr: root.partitions["tenants"].partitions["*"].partitions["bindings"],
		}, {
			path:    "/tenants/deadbeef/foo",
			wantIdx: 3,
			wantPtr: nil,
		}, {
			path:    "/users/deadbeef",
			wantIdx: 2,
			wantPtr: root.partitions["users"].partitions["*"],
		},
	}

	for _, tc := range tests {
		t.Run(strings.TrimPrefix(tc.path, "/"), func(t *testing.T) {
			gotIdx, gotPtr := root.Find(storage.MustParsePath(tc.path))
			if gotIdx != tc.wantIdx || gotPtr != tc.wantPtr {
				t.Fatalf("expected (%d, %v) but got (%d, %v)", tc.wantIdx, tc.wantPtr, gotIdx, gotPtr)
			}
		})
	}

}
