package util

import "testing"

type testStruct struct {
	foo int
}

func BenchmarkNewPtrSlice(b *testing.B) {
	b.ReportAllocs()
	for range b.N {
		s := NewPtrSlice[testStruct](100)
		for j := range 100 {
			s[j].foo = j
		}
	}
}
