// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package bundle

import (
	"fmt"
	"testing"

	"github.com/open-policy-agent/opa/internal/storage/mock"
	"github.com/open-policy-agent/opa/v1/storage"
)

// BenchmarkHasRootsOverlap covers a range of scenarios, each one
// stressing a different aspect of the algorithm:
//   - disjoint: best case, no conflicts
//   - identical: all bundles share one root (N same-root conflicts)
//   - chain: deep prefix chain, O(N^2) reported conflicts
//   - multi-root: N bundles with M roots each, all disjoint
//   - wide-fanout: one umbrella + N descendants, O(N) reported conflicts
//
// We use closures to build the bundle sets, so that we don't have
// inputs for other benchmarks interfering with the current one in the
// form of increased GC overheads.
func BenchmarkHasRootsOverlap(b *testing.B) {
	scenarios := []struct {
		label   string
		build   func() map[string]*Bundle
		wantErr bool
	}{
		{"disjoint/10", func() map[string]*Bundle { return makeDisjointBundles(10) }, false},
		{"disjoint/100", func() map[string]*Bundle { return makeDisjointBundles(100) }, false},
		{"disjoint/1000", func() map[string]*Bundle { return makeDisjointBundles(1000) }, false},
		{"disjoint/10000", func() map[string]*Bundle { return makeDisjointBundles(10000) }, false},

		{"identical/10", func() map[string]*Bundle { return makeIdenticalBundles(10) }, true},
		{"identical/100", func() map[string]*Bundle { return makeIdenticalBundles(100) }, true},
		{"identical/1000", func() map[string]*Bundle { return makeIdenticalBundles(1000) }, true},

		{"chain/10", func() map[string]*Bundle { return makeChainBundles(10) }, true},
		{"chain/100", func() map[string]*Bundle { return makeChainBundles(100) }, true},
		{"chain/500", func() map[string]*Bundle { return makeChainBundles(500) }, true},

		{"multi-root/10x10", func() map[string]*Bundle { return makeMultiRootBundles(10, 10) }, false},
		{"multi-root/100x10", func() map[string]*Bundle { return makeMultiRootBundles(100, 10) }, false},
		{"multi-root/1000x10", func() map[string]*Bundle { return makeMultiRootBundles(1000, 10) }, false},

		{"wide-fanout/10", func() map[string]*Bundle { return makeWideFanoutBundles(10) }, true},
		{"wide-fanout/100", func() map[string]*Bundle { return makeWideFanoutBundles(100) }, true},
		{"wide-fanout/1000", func() map[string]*Bundle { return makeWideFanoutBundles(1000) }, true},
	}

	for _, s := range scenarios {
		bundles := s.build()
		b.Run(s.label, func(b *testing.B) {
			benchHasRootsOverlap(b, bundles, s.wantErr)
		})
	}
}

func benchHasRootsOverlap(b *testing.B, bundles map[string]*Bundle, wantErr bool) {
	b.Helper()
	ctx := b.Context()
	store := mock.New()

	b.ResetTimer()
	for b.Loop() {
		txn := storage.NewTransactionOrDie(ctx, store, storage.WriteParams)
		err := hasRootsOverlap(ctx, store, txn, bundles)
		if (err != nil) != wantErr {
			b.Fatalf("unexpected err=%v (wantErr=%v)", err, wantErr)
		}
		store.Abort(ctx, txn)
	}
}

// BenchmarkHasRootsOverlapWithStore exercises the case where a store
// already has a set of N existing bundles plus a small number of
// new bundles being activated.
func BenchmarkHasRootsOverlapWithStore(b *testing.B) {
	for _, n := range []int{100, 1000, 10000} {
		b.Run(fmt.Sprintf("disjoint/store=%d,new=1", n), func(b *testing.B) {
			benchHasRootsOverlapWithStore(b, n, 1, false)
		})
	}
}

func benchHasRootsOverlapWithStore(b *testing.B, existingCount, newCount int, wantErr bool) {
	b.Helper()
	ctx := b.Context()
	store := mock.New()

	// Pre-populate the store with N disjoint, well-formed manifests.
	setupTxn := storage.NewTransactionOrDie(ctx, store, storage.WriteParams)
	for i := range existingCount {
		roots := []string{fmt.Sprintf("ex%d", i)}
		manifest := Manifest{Roots: &roots}
		if err := WriteManifestToStore(ctx, store, setupTxn, fmt.Sprintf("existing%d", i), manifest); err != nil {
			b.Fatalf("setup: %v", err)
		}
	}
	if err := store.Commit(ctx, setupTxn); err != nil {
		b.Fatalf("setup commit: %v", err)
	}

	// Build the new-bundle map with disjoint roots that don't collide
	// with the store population (so the algorithm scans the full sorted
	// list without short-circuiting).
	newBundles := map[string]*Bundle{}
	for i := range newCount {
		roots := []string{fmt.Sprintf("nu%d", i)}
		newBundles[fmt.Sprintf("new%d", i)] = &Bundle{Manifest: Manifest{Roots: &roots}}
	}

	b.ResetTimer()
	for b.Loop() {
		txn := storage.NewTransactionOrDie(ctx, store, storage.WriteParams)
		err := hasRootsOverlap(ctx, store, txn, newBundles)
		if (err != nil) != wantErr {
			b.Fatalf("unexpected err=%v (wantErr=%v)", err, wantErr)
		}
		store.Abort(ctx, txn)
	}
}

// makeDisjointBundles builds N bundles that each have a single, unique root.
func makeDisjointBundles(n int) map[string]*Bundle {
	out := make(map[string]*Bundle, n)
	for i := range n {
		roots := []string{fmt.Sprintf("root%d", i)}
		name := fmt.Sprintf("b%d", i)
		out[name] = &Bundle{Manifest: Manifest{Roots: &roots}}
	}
	return out
}

// makeIdenticalBundles builds N bundles all declaring the same root.
// This is a pathological same-root conflict (N-way collision reported
// as a single message).
func makeIdenticalBundles(n int) map[string]*Bundle {
	out := make(map[string]*Bundle, n)
	for i := range n {
		roots := []string{"shared"}
		name := fmt.Sprintf("b%d", i)
		out[name] = &Bundle{Manifest: Manifest{Roots: &roots}}
	}
	return out
}

// makeChainBundles builds N bundles where each bundle's root is the
// previous bundle's root with "/x" appended: "a", "a/x", "a/x/x", ...
// Every pair is an ancestor/descendant conflict, yielding O(N^2)
// reported conflicts total. This is the worst case scenario for any
// bundle roots conflict detection algorithm.
func makeChainBundles(n int) map[string]*Bundle {
	out := make(map[string]*Bundle, n)
	path := "a"
	for i := range n {
		roots := []string{path}
		name := fmt.Sprintf("b%d", i)
		out[name] = &Bundle{Manifest: Manifest{Roots: &roots}}
		path += "/x"
	}
	return out
}

// makeMultiRootBundles builds N bundles each declaring M disjoint
// roots. This exercises per-root scaling independent of bundle count.
func makeMultiRootBundles(bundleCount, rootsPerBundle int) map[string]*Bundle {
	out := make(map[string]*Bundle, bundleCount)
	for i := range bundleCount {
		roots := make([]string, rootsPerBundle)
		for j := range rootsPerBundle {
			roots[j] = fmt.Sprintf("bundle%d/root%d", i, j)
		}
		name := fmt.Sprintf("b%d", i)
		out[name] = &Bundle{Manifest: Manifest{Roots: &roots}}
	}
	return out
}

// makeWideFanoutBundles builds one "umbrella" bundle at root "a" as
// well as N sibling bundles rooted at "a/xI".
// Should produce N ancestor/descendant conflicts.
func makeWideFanoutBundles(n int) map[string]*Bundle {
	out := make(map[string]*Bundle, n+1)
	umbrellaRoots := []string{"a"}
	out["umbrella"] = &Bundle{Manifest: Manifest{Roots: &umbrellaRoots}}
	for i := range n {
		roots := []string{fmt.Sprintf("a/x%d", i)}
		name := fmt.Sprintf("b%d", i)
		out[name] = &Bundle{Manifest: Manifest{Roots: &roots}}
	}
	return out
}
