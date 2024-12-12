// Copyright 2024 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package bundle

import (
	"context"
	"testing"

	"github.com/open-policy-agent/opa/internal/storage/mock"
	"github.com/open-policy-agent/opa/storage"
)

func TestHasRootsOverlap(t *testing.T) {
	ctx := context.Background()

	cases := []struct {
		note        string
		storeRoots  map[string]*[]string
		bundleRoots map[string]*[]string
		overlaps    bool
	}{
		{
			note:        "no overlap with existing roots",
			storeRoots:  map[string]*[]string{"bundle1": {"a", "b"}},
			bundleRoots: map[string]*[]string{"bundle2": {"c"}},
			overlaps:    false,
		},
		{
			note:        "no overlap with existing roots multiple bundles",
			storeRoots:  map[string]*[]string{"bundle1": {"a", "b"}},
			bundleRoots: map[string]*[]string{"bundle2": {"c"}, "bundle3": {"d"}},
			overlaps:    false,
		},
		{
			note:        "no overlap no existing roots",
			storeRoots:  map[string]*[]string{},
			bundleRoots: map[string]*[]string{"bundle1": {"a", "b"}},
			overlaps:    false,
		},
		{
			note:        "no overlap without existing roots multiple bundles",
			storeRoots:  map[string]*[]string{},
			bundleRoots: map[string]*[]string{"bundle1": {"a", "b"}, "bundle2": {"c"}},
			overlaps:    false,
		},
		{
			note:        "overlap without existing roots multiple bundles",
			storeRoots:  map[string]*[]string{},
			bundleRoots: map[string]*[]string{"bundle1": {"a", "b"}, "bundle2": {"a", "c"}},
			overlaps:    true,
		},
		{
			note:        "overlap with existing roots",
			storeRoots:  map[string]*[]string{"bundle1": {"a", "b"}},
			bundleRoots: map[string]*[]string{"bundle2": {"c", "a"}},
			overlaps:    true,
		},
		{
			note:        "overlap with existing roots multiple bundles",
			storeRoots:  map[string]*[]string{"bundle1": {"a", "b"}},
			bundleRoots: map[string]*[]string{"bundle2": {"c", "a"}, "bundle3": {"a"}},
			overlaps:    true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			mockStore := mock.New()
			txn := storage.NewTransactionOrDie(ctx, mockStore, storage.WriteParams)

			for name, roots := range tc.storeRoots {
				err := WriteManifestToStore(ctx, mockStore, txn, name, Manifest{Roots: roots})
				if err != nil {
					t.Fatalf("unexpected error: %s", err)
				}
			}

			bundles := map[string]*Bundle{}
			for name, roots := range tc.bundleRoots {
				bundles[name] = &Bundle{
					Manifest: Manifest{
						Roots: roots,
					},
				}
			}

			//err := hasRootsOverlap(ctx, mockStore, txn, bundles)
			//if !tc.overlaps && err != nil {
			//	t.Fatalf("unepected error: %s", err)
			//} else if tc.overlaps && (err == nil || !strings.Contains(err.Error(), "detected overlapping roots in bundle manifest")) {
			//	t.Fatalf("expected overlapping roots error, got: %s", err)
			//}

			//err = mockStore.Commit(ctx, txn)
			//if err != nil {
			//	t.Fatalf("unexpected error: %s", err)
			//}

			mockStore.AssertValid(t)
		})
	}
}
