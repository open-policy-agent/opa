// Copyright 2026 The OPA Authors. All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package util

import "slices"

// Map applies f to each element of s and returns a new slice containing the results.
func Map[T any, U any](s []T, f func(T) U) []U {
	if s == nil {
		return nil
	}
	r := make([]U, len(s))
	for i, v := range s {
		r[i] = f(v)
	}
	return r
}

// MapAppend applies f to each element of src and appends the results to dst, returning the resulting slice.
func MapAppend[T any, U any](dst []U, src []T, f func(T) U) []U {
	if len(src) > 0 {
		dst = slices.Grow(dst, len(src))
		for _, v := range src {
			dst = append(dst, f(v))
		}
	}
	return dst
}

// Every returns true if pred is true for every element of a, otherwise false.
// Returns true for empty / nil slices.
func Every[T any, S ~[]T](a S, pred func(T) bool) bool {
	for _, v := range a {
		if !pred(v) {
			return false
		}
	}
	return true
}

// TryMap returns a new slice of type U by applying the function f to each element of the input slice s.
// If f returns an error for any element, the function aborts and returns the error.
func TryMap[T any, U any](s []T, f func(T) (U, error)) (r []U, err error) {
	if s == nil {
		return nil, nil
	}
	r = make([]U, len(s))
	for i, v := range s {
		if r[i], err = f(v); err != nil {
			return nil, err
		}
	}
	return r, nil
}

// ToSliceOfAny converts a slice of any type T to []any.
func ToSliceOfAny[T any](s []T) []any {
	if s == nil {
		return nil
	}
	r := make([]any, len(s))
	for i, v := range s {
		r[i] = v
	}
	return r
}

// Count returns the number of elements in items that satisfy pred.
func Count[T any](pred func(T) bool, items ...T) (c int) {
	for i := range items {
		if pred(items[i]) {
			c++
		}
	}
	return c
}

// Not returns a new predicate function that negates the result of pred.
func Not[T any](pred func(T) bool) func(T) bool {
	return func(v T) bool {
		return !pred(v)
	}
}

// Identity returns the input value unchanged.
func Identity[T any](v T) T {
	return v
}
