// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

//go:build !go1.27

package ast

import (
	"bytes"
	"testing"
)

// assertJsonEqual fails the test unless exp and got are byte-for-byte equal.
func assertJsonEqual[A, B string | []byte](t *testing.T, exp A, got B) {
	t.Helper()

	if !bytes.Equal([]byte(exp), []byte(got)) {
		t.Errorf("expected JSON to be equal:\n%s\n%s", exp, got)
	}
}
