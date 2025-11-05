package util

import (
	"math"
	"slices"
	"strings"
	"sync"
	"unsafe"
)

// NewPtrSlice returns a slice of pointers to T with length n,
// with only 2 allocations performed no matter the size of n.
// See:
// https://gist.github.com/CAFxX/e96e8a5c3841d152f16d266a1fe7f8bd#slices-of-pointers
func NewPtrSlice[T any](n int) []*T {
	return GrowPtrSlice[T](nil, n)
}

// GrowPtrSlice appends n elements to the slice, each pointing to
// a newly-allocated T. The resulting slice has length equal to len(s)+n.
//
// It performs at most 2 allocations, regardless of n.
func GrowPtrSlice[T any](s []*T, n int) []*T {
	s = slices.Grow(s, n)
	p := make([]T, n)
	for i := range n {
		s = append(s, &p[i])
	}
	return s
}

// Allocation free conversion from []byte to string (unsafe)
// Note that the byte slice must not be modified after conversion
func ByteSliceToString(bs []byte) string {
	return unsafe.String(unsafe.SliceData(bs), len(bs))
}

// Allocation free conversion from ~string to []byte (unsafe)
// Note that the byte slice must not be modified after conversion
func StringToByteSlice[T ~string](s T) []byte {
	return unsafe.Slice(unsafe.StringData(string(s)), len(s))
}

// NumDigitsInt returns the number of digits in n.
// This is useful for pre-allocating buffers for string conversion.
func NumDigitsInt(n int) int {
	if n == 0 {
		return 1
	}

	if n < 0 {
		n = -n
	}

	return int(math.Log10(float64(n))) + 1
}

// NumDigitsUint returns the number of digits in n.
// This is useful for pre-allocating buffers for string conversion.
func NumDigitsUint(n uint64) int {
	if n == 0 {
		return 1
	}

	return int(math.Log10(float64(n))) + 1
}

// KeysCount returns the number of keys in m that satisfy predicate p.
func KeysCount[K comparable, V any](m map[K]V, p func(K) bool) int {
	count := 0
	for k := range m {
		if p(k) {
			count++
		}
	}
	return count
}

// SplitMap calls fn for each delim-separated part of text and returns a slice of the results.
// Cheaper than calling fn on strings.Split(text, delim), as it avoids allocating an intermediate slice of strings.
func SplitMap[T any](text string, delim string, fn func(string) T) []T {
	sl := make([]T, 0, strings.Count(text, delim)+1)
	for s := range strings.SplitSeq(text, delim) {
		sl = append(sl, fn(s))
	}
	return sl
}

// SlicePool is a pool for (pointers to) slices of type T.
// It uses sync.Pool to pool the slices, and grows them as needed.
type SlicePool[T any] struct {
	pool sync.Pool
}

// NewSlicePool creates a new SlicePool for slices of type T with the given initial length.
// This number is only a hint, as the slices will grow as needed. For best performance, store
// slices of similar lengths in the same pool.
func NewSlicePool[T any](length int) *SlicePool[T] {
	return &SlicePool[T]{
		pool: sync.Pool{
			New: func() any {
				s := make([]T, length)
				return &s
			},
		},
	}
}

// Get returns a pointer to a slice of type T with the given length
// from the pool. The slice capacity will grow as needed to accommodate
// the requested length. The returned slice will have all its elements
// set to the zero value of T. Returns a pointer to avoid allocating.
func (sp *SlicePool[T]) Get(length int) *[]T {
	s := sp.pool.Get().(*[]T)
	d := *s

	if cap(d) < length {
		d = slices.Grow(d, length)
	}

	d = d[:length] // reslice to requested length, while keeping capacity

	clear(d)

	*s = d

	return s
}

// Put returns a pointer to a slice of type T to the pool.
func (sp *SlicePool[T]) Put(s *[]T) {
	sp.pool.Put(s)
}
