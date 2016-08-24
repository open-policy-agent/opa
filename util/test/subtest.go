// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// +build !go1.7

package test

import "testing"

// Subtest provides pre-1.7 backwards compatibility for sub-tests.
func Subtest(t *testing.T, name string, f func(*testing.T)) {
	f(t)
}
