// Copyright 2024 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"testing"
)

// BenchmarkValueMapMarshalJSON tests the performance of ValueMap.MarshalJSON
// with slice pooling.
func BenchmarkValueMapMarshalJSON(b *testing.B) {
	testCases := []struct {
		name string
		size int
	}{
		{"small/5", 5},
		{"medium/50", 50},
		{"large/500", 500},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			vm := NewValueMap()
			for i := 0; i < tc.size; i++ {
				vm.Put(IntNumberTerm(i).Value, StringTerm("value").Value)
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				_, err := vm.MarshalJSON()
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkObjectMarshalJSON tests the performance of object.MarshalJSON
// with slice pooling.
func BenchmarkObjectMarshalJSON(b *testing.B) {
	testCases := []struct {
		name string
		size int
	}{
		{"small/5", 5},
		{"medium/50", 50},
		{"large/500", 500},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			items := make([][2]*Term, tc.size)
			for i := 0; i < tc.size; i++ {
				items[i] = Item(IntNumberTerm(i), StringTerm("value"))
			}
			term := ObjectTerm(items...)

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				_, err := term.MarshalJSON()
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
