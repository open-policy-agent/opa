package util

import "testing"

type testStruct struct {
	foo int
}

func BenchmarkNewPtrSlice(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		s := NewPtrSlice[testStruct](100)
		for j := 0; j < 100; j++ {
			s[j].foo = j
		}
	}
}
