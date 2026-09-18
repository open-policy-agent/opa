// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Curated subset of this package's benchmarks for the nightly benchlab run.
// See build/bench-nightly/set.json.
//
// Every case costs the same fixed slice of wall clock, whatever it measures, so
// the size sweeps are cut to the sizes that say something the others do not.

package bundle

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/open-policy-agent/opa/v1/util/test"
)

// Root-conflict detection runs on every activation. One point per conflict
// shape at the largest size that is not pathological, plus a second disjoint
// point -- the common case -- to show how it scales. chain/500 is left out: it
// is O(N^2) by construction rather than a shape anyone deploys.
func BenchmarkBenchlabHasRootsOverlap(b *testing.B) {
	for _, s := range []struct {
		label   string
		build   func() map[string]*Bundle
		wantErr bool
	}{
		{"disjoint/1000", func() map[string]*Bundle { return makeDisjointBundles(1000) }, false},
		{"disjoint/10000", func() map[string]*Bundle { return makeDisjointBundles(10000) }, false},
		{"identical/1000", func() map[string]*Bundle { return makeIdenticalBundles(1000) }, true},
		{"chain/100", func() map[string]*Bundle { return makeChainBundles(100) }, true},
		{"multi-root/1000x10", func() map[string]*Bundle { return makeMultiRootBundles(1000) }, false},
		{"wide-fanout/1000", func() map[string]*Bundle { return makeWideFanoutBundles(1000) }, true},
	} {
		bundles := s.build()
		b.Run(s.label, func(b *testing.B) {
			benchHasRootsOverlap(b, bundles, s.wantErr)
		})
	}
}

// Activating one bundle into a store that already holds N manifests.
func BenchmarkBenchlabHasRootsOverlapWithStore(b *testing.B) {
	for _, n := range []int{1000, 10000} {
		b.Run(fmt.Sprintf("disjoint/store=%d,new=1", n), func(b *testing.B) {
			benchHasRootsOverlapWithStore(b, n, 1, false)
		})
	}
}

// The .tar.gz ingest path a downloaded bundle takes.
func BenchmarkBenchlabTarballLoader(b *testing.B) {
	for _, n := range []int{1000, 100000} {
		expectedFiles := make(map[string]string, len(benchTestArchiveFiles)+1)
		maps.Copy(expectedFiles, benchTestArchiveFiles)
		expectedFiles["/x/data.json"] = benchTestGetFlatDataJSON(n)

		// The tarball is generated once and reread on every iteration.
		test.WithTempFS(map[string]string{"/archive.tar.gz": ""}, func(rootDir string) {
			tarballFile := filepath.Join(rootDir, "archive.tar.gz")
			benchTestCreateTarballFile(b, rootDir, expectedFiles)

			f, err := os.Open(tarballFile)
			if err != nil {
				b.Fatal(err)
			}
			defer f.Close()

			b.Run(strconv.Itoa(n), func(b *testing.B) {
				for b.Loop() {
					if _, err := f.Seek(0, 0); err != nil {
						b.Fatal(err)
					}
					benchTestLoader(b, NewTarballLoaderWithBaseURL(f, tarballFile))
				}
			})
		})
	}
}

// The --bundle <dir> ingest path.
func BenchmarkBenchlabDirectoryLoader(b *testing.B) {
	for _, n := range []int{10000, 250000} {
		expectedFiles := make(map[string]string, len(benchTestArchiveFiles)+1)
		maps.Copy(expectedFiles, benchTestArchiveFiles)
		expectedFiles["/x/data.json"] = benchTestGetFlatDataJSON(n)

		test.WithTempFS(expectedFiles, func(rootDir string) {
			b.Run(strconv.Itoa(n), func(b *testing.B) {
				for b.Loop() {
					benchTestLoader(b, NewDirectoryLoader(rootDir))
				}
			})
		})
	}
}

// Signature hashing, which signed bundles pay per file.
func BenchmarkBenchlabHashFile(b *testing.B) {
	for _, n := range []int{100, 10000} {
		doc := make(map[string]any, n)
		for i := range n {
			doc[fmt.Sprintf("key%d", i)] = i
		}

		h, err := NewSignatureHasher(SHA256)
		if err != nil {
			b.Fatal(err)
		}

		b.Run(fmt.Sprintf("leaves=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if _, err := h.HashFile(doc); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// The deep copy performed once per activation.
func BenchmarkBenchlabBundleCopy(b *testing.B) {
	for _, n := range []int{100, 10000} {
		bundle := Bundle{Data: benchTestNestedData(n)}
		bundle.Manifest.Init()

		b.Run(fmt.Sprintf("leaves=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				_ = bundle.Copy()
			}
		})
	}
}
