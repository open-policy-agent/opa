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

type resettableStruct struct {
	ptr *int
	n   int
}

func (r *resettableStruct) Reset() {
	r.ptr, r.n = nil, 0
}

func TestResettablePool(t *testing.T) {
	pool := NewResettablePool[resettableStruct, *resettableStruct]()

	x := pool.Get()
	v := 42
	x.ptr, x.n = &v, 7
	pool.Put(x)

	if x.ptr != nil || x.n != 0 {
		t.Fatalf("expected fields to be reset on Put, got ptr=%v n=%d", x.ptr, x.n)
	}

	y := pool.Get()
	if y.ptr != nil || y.n != 0 {
		t.Fatalf("expected a fresh/reset value from Get, got ptr=%v n=%d", y.ptr, y.n)
	}
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

func TestAtoi64(t *testing.T) {
	tests := []struct {
		input  string
		expOK  bool
		expInt int64
	}{
		{"", false, 0},
		{"no", false, 0},
		{"02", true, 2},
		{"0", true, 0},
		{"+0.0", true, 0},
		{"-0", true, 0},
		{"123", true, 123},
		{"-123", true, -123},
		{"9223372", true, 9223372},
		{"8223372036854775807", true, 8223372036854775807},
		{"9223372036854775807", true, 9223372036854775807},   // max int64
		{"+9223372036854775807", true, 9223372036854775807},  // max int64, leading '+'
		{"9223372036854775808", false, 0},                    // max int64 + 1
		{"-9223372036854775808", true, -9223372036854775808}, // min int64
		{"-9223372036854775809", false, 0},                   // min int64 - 1
		{"1.0", true, 1},
		{"-123.00000000", true, -123},
		{"1.001", false, 0},
		{"1.1.1", false, 0},
	}

	for _, test := range tests {
		res, ok := Atoi64(test.input)
		if ok != test.expOK || res != test.expInt {
			t.Errorf("Atoi64(%q) = (%d, %v); expected (%d, %v)", test.input, res, ok, test.expInt, test.expOK)
		}
		if strings.Contains(test.input, ".") {
			if res != test.expInt {
				t.Fatalf("Atoi64(%q) = (%d, %v); expected (%d, %v)", test.input, res, ok, test.expInt, test.expOK)
			}
			continue
		}

		strconvRes, err := strconv.Atoi(test.input)
		strconvOK := err == nil

		if strconvOK && test.expOK {
			if strconvRes != int(test.expInt) {
				t.Errorf("strconv.Atoi(%q) = (%d, %v); expected (%d, %v)", test.input, strconvRes, strconvOK, test.expInt, test.expOK)
			}
		} else {
			if strconvOK {
				t.Fatalf("strconv.Atoi(%q) = (%d, %v); expected error: %v", test.input, strconvRes, strconvOK, !test.expOK)
			}
			if test.expOK {
				t.Fatalf("strconv.Atoi(%q) error = %v; expected error: %v", test.input, err, !test.expOK)
			}
		}

	}
}

// See testdata/atoi.txt for a performance comparison between Atoi64 and strconv.Atoi
func BenchmarkAtoi64(b *testing.B) {
	tests := []string{
		"",
		"no",
		"02",
		"0",
		"-0",
		"123",
		"-123",
		"9223372",
		"8223372036854775807",
		"9223372036854775807",  // max int64
		"+9223372036854775807", // max int64, leading '+'
		"9223372036854775808",  // max int64 + 1
		"-9223372036854775808", // min int64
		"-9223372036854775809", // min int64 - 1
	}

	for _, test := range tests {
		b.Run(test, func(b *testing.B) {
			for b.Loop() {
				// replace with strconv.Atoi for comparison
				_, _ = Atoi(test)
			}
		})
	}
}

func mustAtoi(s string) int {
	v, _ := Atoi(s)
	return v
}

func TestStringToByteSlice(t *testing.T) {
	for _, s := range []string{"", "a", "foo/bar"} {
		bs := StringToByteSlice(s)

		// must be equivalent to []byte(s), including being non-nil for ""
		if bs == nil {
			t.Errorf("StringToByteSlice(%q): expected non-nil slice", s)
		}
		if string(bs) != s {
			t.Errorf("StringToByteSlice(%q): got %q", s, string(bs))
		}
	}
}
