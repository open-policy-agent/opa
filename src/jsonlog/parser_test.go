// Copyright 2015 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package jsonlog

import (
	"testing"
	// "fmt"
)

func TestParser(t *testing.T) {
	_, err := Parse("nonexistent", []byte("2 + 3"))
	if err != nil {
		t.Errorf("Error when parsing: %s", err)
	}
}
