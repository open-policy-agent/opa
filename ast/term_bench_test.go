// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.
package ast

import (
	"fmt"
	"strings"
	"testing"
)

func BenchmarkObjectLookup(b *testing.B) {
	sizes := []int{5, 50, 500, 5000}
	for _, n := range sizes {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			obj := NewObject()
			for i := 0; i < n; i++ {
				obj.Insert(StringTerm(fmt.Sprint(i)), IntNumberTerm(i))
			}
			key := StringTerm(fmt.Sprint(n - 1))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				value := obj.Get(key)
				if value == nil {
					b.Fatal("expected hit")
				}
			}
		})
	}
}

func BenchmarkSetIntersection(b *testing.B) {
	sizes := []int{5, 50, 500, 5000}
	for _, n := range sizes {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			setA := NewSet()
			setB := NewSet()
			for i := 0; i < n; i++ {
				setA.Add(IntNumberTerm(i))
				setB.Add(IntNumberTerm(i))
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				setC := setA.Intersect(setB)
				if setC.Len() != setA.Len() || setC.Len() != setB.Len() {
					b.Fatal("expected equal")
				}
			}
		})
	}
}

func BenchmarkSetIntersectionDifferentSize(b *testing.B) {
	sizes := []int{4, 50, 500, 5000}
	for _, n := range sizes {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			setA := NewSet()
			setB := NewSet()
			for i := 0; i < n; i++ {
				setA.Add(IntNumberTerm(i))
			}
			for i := 0; i < sizes[0]; i++ {
				setB.Add(IntNumberTerm(i))
			}
			setB.Add(IntNumberTerm(-1))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				setC := setA.Intersect(setB)
				if setC.Len() != sizes[0] {
					b.Fatal("expected size to be equal")
				}
			}
		})
	}
}

func BenchmarkSetMembership(b *testing.B) {
	sizes := []int{5, 50, 500, 5000}
	for _, n := range sizes {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			setA := NewSet()
			for i := 0; i < n; i++ {
				setA.Add(IntNumberTerm(i))
			}
			key := IntNumberTerm(n - 1)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if !setA.Contains(key) {
					b.Fatal("expected hit")
				}
			}
		})
	}
}

func BenchmarkTermHashing(b *testing.B) {
	sizes := []int{10, 100, 1000}
	for _, n := range sizes {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			s := String(strings.Repeat("a", n))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = s.Hash()
			}
		})
	}
}
