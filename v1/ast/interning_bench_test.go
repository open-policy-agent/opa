// Copyright 2024 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"testing"
)

// BenchmarkInternedIntNumberTermFromString tests the performance of
// string-to-int term lookup using unique.Handle[string] keys.
func BenchmarkInternedIntNumberTermFromString(b *testing.B) {
	testCases := []string{
		"-1", "0", "1", "10", "42", "100", "255", "512",
	}

	b.ResetTimer()
	for range b.N {
		for _, s := range testCases {
			_ = InternedIntNumberTermFromString(s)
		}
	}
}

// BenchmarkInternedIntNumberTermFromStringMiss tests the performance
// when the string is NOT in the interned map.
func BenchmarkInternedIntNumberTermFromStringMiss(b *testing.B) {
	testCases := []string{
		"999", "1000", "9999", "-100",
	}

	b.ResetTimer()
	for range b.N {
		for _, s := range testCases {
			_ = InternedIntNumberTermFromString(s)
		}
	}
}
