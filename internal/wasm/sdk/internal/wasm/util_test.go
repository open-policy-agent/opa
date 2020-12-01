// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.
package wasm

import (
	"testing"
)

func TestUtilPages(t *testing.T) {
	if Pages(wasmPageSize-1) != 1 ||
		Pages(wasmPageSize) != 1 ||
		Pages(wasmPageSize+1) != 2 {
		t.Errorf("pages not rounded correctly")
	}
}
