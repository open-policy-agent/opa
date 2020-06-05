// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// +build go1.7

package test

import "testing"

// Subtest executes a sub-test f under test t.
func Subtest(t *testing.T, name string, f func(*testing.T)) {
	t.Helper()
	t.Run(name, f)
}
