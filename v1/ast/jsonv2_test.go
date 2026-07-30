// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

//go:build go1.27

package ast

import (
	"bytes"
	"encoding/json/jsontext"
	"testing"
)

// assertJsonEqual fails the test unless the canonical JSON encoding of exp
// and got are equal, meaning that they are compared without regard to
// things like whitespace, key order, etc. For more details, see
// [jsontext.Value.Canonicalize].
func assertJsonEqual[A, B string | []byte](t *testing.T, exp A, got B) {
	t.Helper()

	expVal, gotVal := jsontext.Value(exp), jsontext.Value(got)

	expVal.Canonicalize()
	gotVal.Canonicalize()

	if !bytes.Equal(expVal, gotVal) {
		t.Errorf("expected JSON to be equal:\n%s\n%s", expVal, gotVal)
	}
}
