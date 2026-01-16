// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

// CountFunc counts the number of items in a slice S that satisfy predicate function f.
func CountFunc[T any, S ~[]T](items S, f func(T) bool) (n int) {
	for i := range items {
		if f(items[i]) {
			n++
		}
	}
	return n
}
