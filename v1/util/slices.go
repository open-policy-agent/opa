// Copyright 2026 The OPA Authors. All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package util

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
