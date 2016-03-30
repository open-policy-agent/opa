// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package opalog

import "testing"

func TestEqualTerms(t *testing.T) {
	assertTermEqual(t, reflectTerm(nil), reflectTerm(nil))
	assertTermEqual(t, reflectTerm(true), reflectTerm(true))
	assertTermEqual(t, reflectTerm(5), reflectTerm(5))
	assertTermEqual(t, reflectTerm("a string"), reflectTerm("a string"))
	assertTermEqual(t, reflectTerm(map[int]int{1: 2}), reflectTerm(map[int]int{1: 2}))
	assertTermEqual(t, reflectTerm(map[int]int{1: 2, 3: 4}), reflectTerm(map[int]int{1: 2, 3: 4}))
	assertTermEqual(t, reflectTerm([]int{1, 2, 3}), reflectTerm([]int{1, 2, 3}))

	assertTermNotEqual(t, reflectTerm(nil), reflectTerm(true))
	assertTermNotEqual(t, reflectTerm(true), reflectTerm(false))
	assertTermNotEqual(t, reflectTerm(5), reflectTerm(7))
	assertTermNotEqual(t, reflectTerm("a string"), reflectTerm("abc"))
	assertTermNotEqual(t, reflectTerm(map[int]int{3: 2}), reflectTerm(map[int]int{1: 2}))
	assertTermNotEqual(t, reflectTerm(map[int]int{1: 2, 3: 7}), reflectTerm(map[int]int{1: 2, 3: 4}))
	assertTermNotEqual(t, reflectTerm(5), reflectTerm("a string"))
	assertTermNotEqual(t, reflectTerm(1), reflectTerm(true))
	assertTermNotEqual(t, reflectTerm(map[int]int{1: 2, 3: 7}), reflectTerm([]int{1, 2, 3, 7}))
	assertTermNotEqual(t, reflectTerm([]int{1, 2, 3}), reflectTerm([]int{1, 2, 4}))

	assertTermEqual(t, reflectTerm(NewVar("foo")), reflectTerm(NewVar("foo")))
	assertTermNotEqual(t, reflectTerm(NewVar("foo")), reflectTerm(NewVar("bar")))
}
