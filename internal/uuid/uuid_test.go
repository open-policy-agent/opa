// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.
package uuid

import (
	"bytes"
	"testing"
)

func TestUUID4(t *testing.T) {
	uuid, err := New(bytes.NewReader(make([]byte, 16)))
	if err != nil {
		t.Fatal(err)
	}

	expect := "00000000-0000-4000-8000-000000000000"
	if uuid != expect {
		t.Errorf("Expected %q, got %q", expect, uuid)
	}
}
