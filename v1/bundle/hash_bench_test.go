// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package bundle

import (
	"fmt"
	"testing"
)

// BenchmarkHashFile hashes documents with many scalar leaves.
func BenchmarkHashFile(b *testing.B) {
	for _, n := range []int{10, 100, 1000, 10000} {
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
