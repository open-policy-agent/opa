// Copyright 2026 The OPA Authors. All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package util

// Map returns a new slice of type U by applying the function f to each element of the input slice s.
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

func Not[T any](f func(T) bool) func(T) bool {
	return func(v T) bool {
		return !f(v)
	}
}
