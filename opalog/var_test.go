// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package opalog

import "testing"

func TestEqualVarTerms(t *testing.T) {
	assertTermEqual(t, reflectTerm(NewVar("foo")), reflectTerm(NewVar("foo")))
	assertTermNotEqual(t, reflectTerm(NewVar("foo")), reflectTerm(NewVar("foobar")))
}
