// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package precis

import (
	"testing"
)

func BenchmarkUsernameCaseMapped(b *testing.B) {
	for i := 0; i < b.N; i++ {
		UsernameCaseMapped.String("Malvolio")
	}
}

func BenchmarkUsernameCasePreserved(b *testing.B) {
	for i := 0; i < b.N; i++ {
		UsernameCasePreserved.String("Malvolio")
	}
}

func BenchmarkOpaqueString(b *testing.B) {
	for i := 0; i < b.N; i++ {
		OpaqueString.String("Malvolio")
	}
}

func BenchmarkNickname(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Nickname.String("Malvolio")
	}
}
