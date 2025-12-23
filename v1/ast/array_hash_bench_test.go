// Copyright 2024 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"strconv"
	"testing"
)

// BenchmarkArrayCreation benchmarks array creation with different sizes.
// This measures the impact of lazy hash computation vs eager hash computation.
func BenchmarkArrayCreation(b *testing.B) {
	sizes := []int{10, 100, 1000, 10000}
	for _, n := range sizes {
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			terms := make([]*Term, n)
			for i := range n {
				terms[i] = IntNumberTerm(i)
			}
			b.ResetTimer()
			b.ReportAllocs()
			for b.Loop() {
				_ = NewArray(terms...)
			}
		})
	}
}

// BenchmarkArrayHash benchmarks hash computation on arrays.
// With lazy evaluation, first hash access triggers computation.
func BenchmarkArrayHash(b *testing.B) {
	sizes := []int{10, 100, 1000, 10000}
	for _, n := range sizes {
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			terms := make([]*Term, n)
			for i := range n {
				terms[i] = IntNumberTerm(i)
			}
			arr := NewArray(terms...)
			b.ResetTimer()
			b.ReportAllocs()
			for b.Loop() {
				_ = arr.Hash()
			}
		})
	}
}

// BenchmarkArrayHashRepeated benchmarks repeated hash access.
// With lazy evaluation and caching, subsequent accesses should be faster.
func BenchmarkArrayHashRepeated(b *testing.B) {
	sizes := []int{10, 100, 1000, 10000}
	for _, n := range sizes {
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			terms := make([]*Term, n)
			for i := range n {
				terms[i] = IntNumberTerm(i)
			}
			arr := NewArray(terms...)
			// Trigger hash computation once
			_ = arr.Hash()
			b.ResetTimer()
			b.ReportAllocs()
			for b.Loop() {
				_ = arr.Hash()
			}
		})
	}
}

// BenchmarkArrayAppend benchmarks appending elements to arrays.
// This measures the impact of incremental hash updates.
func BenchmarkArrayAppend(b *testing.B) {
	sizes := []int{10, 100, 1000}
	for _, n := range sizes {
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			terms := make([]*Term, n)
			for i := range n {
				terms[i] = IntNumberTerm(i)
			}
			newTerm := IntNumberTerm(9999)
			b.ResetTimer()
			b.ReportAllocs()
			for b.Loop() {
				arr := NewArray(terms...)
				_ = arr.Append(newTerm)
			}
		})
	}
}

// BenchmarkArrayAppendWithHash benchmarks appending when hash is already computed.
// This tests the incremental hash update optimization.
func BenchmarkArrayAppendWithHash(b *testing.B) {
	sizes := []int{10, 100, 1000}
	for _, n := range sizes {
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			terms := make([]*Term, n)
			for i := range n {
				terms[i] = IntNumberTerm(i)
			}
			newTerm := IntNumberTerm(9999)
			b.ResetTimer()
			b.ReportAllocs()
			for b.Loop() {
				arr := NewArray(terms...)
				_ = arr.Hash() // Force hash computation
				_ = arr.Append(newTerm)
			}
		})
	}
}

// BenchmarkArrayCopy benchmarks copying arrays.
// This measures memory allocation savings from not copying hashs slice.
func BenchmarkArrayCopy(b *testing.B) {
	sizes := []int{10, 100, 1000, 10000}
	for _, n := range sizes {
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			terms := make([]*Term, n)
			for i := range n {
				terms[i] = IntNumberTerm(i)
			}
			arr := NewArray(terms...)
			b.ResetTimer()
			b.ReportAllocs()
			for b.Loop() {
				_ = arr.Copy()
			}
		})
	}
}

// BenchmarkArraySlice benchmarks slicing arrays.
// This measures the impact of not copying hashs slice during slicing.
func BenchmarkArraySlice(b *testing.B) {
	sizes := []int{100, 1000, 10000}
	for _, n := range sizes {
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			terms := make([]*Term, n)
			for i := range n {
				terms[i] = IntNumberTerm(i)
			}
			arr := NewArray(terms...)
			b.ResetTimer()
			b.ReportAllocs()
			for b.Loop() {
				_ = arr.Slice(0, n/2)
			}
		})
	}
}

// BenchmarkArraySet benchmarks setting array elements.
// This tests hash invalidation performance.
func BenchmarkArraySet(b *testing.B) {
	sizes := []int{10, 100, 1000}
	for _, n := range sizes {
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			terms := make([]*Term, n)
			for i := range n {
				terms[i] = IntNumberTerm(i)
			}
			arr := NewArray(terms...)
			newTerm := IntNumberTerm(9999)
			b.ResetTimer()
			b.ReportAllocs()
			for b.Loop() {
				arr.Set(0, newTerm)
			}
		})
	}
}

// BenchmarkArraySorted benchmarks sorting arrays.
// With lazy evaluation, sorted arrays can preserve computed hash.
func BenchmarkArraySorted(b *testing.B) {
	sizes := []int{10, 100, 1000}
	for _, n := range sizes {
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			terms := make([]*Term, n)
			for i := range n {
				terms[i] = IntNumberTerm(n - i) // reverse order
			}
			arr := NewArray(terms...)
			// Trigger hash computation
			_ = arr.Hash()
			b.ResetTimer()
			b.ReportAllocs()
			for b.Loop() {
				_ = arr.Sorted()
			}
		})
	}
}

// BenchmarkArrayNoHashAccess benchmarks operations that don't access hash.
// This shows the benefit of lazy evaluation when hash is never needed.
func BenchmarkArrayNoHashAccess(b *testing.B) {
	sizes := []int{10, 100, 1000, 10000}
	for _, n := range sizes {
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			terms := make([]*Term, n)
			for i := range n {
				terms[i] = IntNumberTerm(i)
			}
			b.ResetTimer()
			b.ReportAllocs()
			for b.Loop() {
				arr := NewArray(terms...)
				_ = arr.Len()
				_ = arr.Elem(0)
			}
		})
	}
}
