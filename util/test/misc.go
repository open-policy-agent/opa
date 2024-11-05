// Copyright 2024 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package test

import "testing"

func FatalMismatch(t *testing.T, act, exp any) {
	t.Helper()
	t.Fatalf("expected:\n\n%v\n\nbut got:\n\n%v", exp, act)
}
