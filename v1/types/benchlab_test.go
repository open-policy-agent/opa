// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Curated subset of this package's benchmarks for the nightly benchlab run.
// See build/bench-nightly/set.json.

package types

import (
	"encoding/json"
	"fmt"
	"strconv"
	"testing"
)

func BenchmarkBenchlabSelect(b *testing.B) {
	const size = 1000
	b.Run(strconv.Itoa(size), func(b *testing.B) {
		tpe := generateType(size)
		var num any = json.Number(strconv.Itoa(size - 1))
		for b.Loop() {
			if result := Select(tpe, num); result != nil {
				if Compare(result, N) != 0 {
					b.Fatal("expected number type")
				}
			}
		}
	})
}

func BenchmarkBenchlabAnyMergeOne(b *testing.B) {
	for _, size := range []int{100, 1000, 10000} {
		anyA := generateTypes(size)
		b.Run(strconv.Itoa(size), func(b *testing.B) {
			for b.Loop() {
				if result := anyA.Merge(N); len(result) != len(anyA)+1 {
					b.Fatalf("expected length %d, got %d", len(anyA)+1, len(result))
				}
			}
		})
	}
}

func BenchmarkBenchlabAnyUnionAllUniqueTypes(b *testing.B) {
	for _, sizes := range []struct{ a, b int }{
		{100, 100},
		{500, 500},
		{1000, 2500},
		{2500, 2500},
	} {
		anyA := generateTypes(sizes.a)
		anyB := generateTypes(sizes.b, "B-")
		b.Run(fmt.Sprintf("%dx%d", sizes.a, sizes.b), func(b *testing.B) {
			// A + B - 1: the object type is present in both sets.
			want := len(anyA) + len(anyB) - 1
			for b.Loop() {
				if result := anyA.Union(anyB); len(result) != want {
					b.Fatalf("expected length %d, got %d", want, len(result))
				}
			}
		})
	}
}
