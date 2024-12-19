package util

import "slices"

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
	for i := 0; i < n; i++ {
		s = append(s, &p[i])
	}
	return s
}
