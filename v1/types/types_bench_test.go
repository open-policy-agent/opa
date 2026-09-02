// Copyright 2021 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package types

import (
	"encoding/json"
	"fmt"
	"strconv"
	"testing"
)

func BenchmarkSelect(b *testing.B) {
	sizes := []int{1000}
	for _, size := range sizes {
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
}

func generateTypes(n int, prefix ...string) Any {
	types := make([]Type, 0, n)
	if len(prefix) > 0 {
		for i := range n {
			types = append(types, generateTypeWithPrefix(i, prefix[0]))
		}
	} else {
		for i := range n {
			types = append(types, generateType(i))
		}
	}
	return types
}

func generateType(n int) Type {
	static := make([]*StaticProperty, n)
	for i := range n {
		static[i] = NewStaticProperty(json.Number(strconv.Itoa(i)), N)
	}
	return NewObject(static, nil)
}

func generateTypeWithPrefix(n int, prefix string) Type {
	static := make([]*StaticProperty, n)
	for i := range n {
		static[i] = NewStaticProperty(prefix+strconv.Itoa(i), S)
	}
	return NewObject(static, nil)
}

func BenchmarkAnyMergeOne(b *testing.B) {
	sizes := []int{100, 500, 1000, 5000, 10000}
	for _, size := range sizes {
		anyA := generateTypes(size)
		tpeB := N
		b.Run(strconv.Itoa(size), func(b *testing.B) {
			for b.Loop() {
				result := anyA.Merge(tpeB)
				if len(result) != len(anyA)+1 {
					b.Fatalf("Expected length of merged result to be: %d, got: %d", len(anyA)+1, len(result))
				}
			}
		})
	}
}

// Build up 2x Any type lists of unique and different types, then Union merge.
func BenchmarkAnyUnionAllUniqueTypes(b *testing.B) {
	sizes := []int{100, 250, 500, 1000, 2500}
	for _, sizeA := range sizes {
		for _, sizeB := range sizes {
			anyA := generateTypes(sizeA)
			anyB := generateTypes(sizeB, "B-")
			b.Run(fmt.Sprintf("%dx%d", sizeA, sizeB), func(b *testing.B) {
				for b.Loop() {
					resultA2B := anyA.Union(anyB)
					// Expect length to be A + B - 1, because the `object` type is present in both Any type sets.
					if len(resultA2B) != (len(anyA) + len(anyB) - 1) {
						b.Fatalf("Expected length of unioned result to be: %d, got: %d", len(anyA)+len(anyB)-1, len(resultA2B))
					}
				}
			})
		}
	}
}
