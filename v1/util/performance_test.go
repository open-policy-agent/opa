package util

import (
	"slices"
	"strconv"
	"strings"
	"testing"
)

type testStruct struct {
	foo int
}

func BenchmarkNewPtrSlice(b *testing.B) {
	for b.Loop() {
		s := NewPtrSlice[testStruct](100)
		for j := range 100 {
			s[j].foo = j
		}
	}
}

func TestSplitMap(t *testing.T) {
	if res := SplitMap("0.1.2", ".", mustAtoi); !slices.Equal(res, []int{0, 1, 2}) {
		t.Fatalf("Expected [0 1 2], got: %v", res)
	}

	if res := SplitMap("0", ".", mustAtoi); !slices.Equal(res, []int{0}) {
		t.Fatalf("Expected [0], got: %v", res)
	}
}

// BenchmarkMapDelimited/map_delimited-16         2419126        28.98 ns/op     24 B/op       1 allocs/op
// BenchmarkMapDelimited/split_and_convert-16    30683016        39.70 ns/op     72 B/op       2 allocs/op
func BenchmarkSplitMap(b *testing.B) {
	b.Run("split map", func(b *testing.B) {
		var res []int
		for b.Loop() {
			res = SplitMap("0.1.2", ".", mustAtoi)
		}
		if !slices.Equal(res, []int{0, 1, 2}) {
			b.Fatalf("Expected [0 1 2], got: %v", res)
		}
	})

	b.Run("split and convert", func(b *testing.B) {
		var res []int
		for b.Loop() {
			parts := strings.Split("0.1.2", ".")
			res = make([]int, len(parts))
			for i := range parts {
				res[i] = mustAtoi(parts[i])
			}
		}
		if !slices.Equal(res, []int{0, 1, 2}) {
			b.Fatalf("Expected [0 1 2], got: %v", res)
		}
	})
}

// Zero allocations
func BenchmarkSlicePoolGetPut(b *testing.B) {
	sp := NewSlicePool[int](4)
	for b.Loop() {
		s := sp.Get(4)
		sp.Put(s)
	}
}

func mustAtoi(s string) int {
	v, _ := strconv.Atoi(s)
	return v
}
