// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import "testing"

func TestQueryIDFactory(t *testing.T) {
	f := &queryIDFactory{}
	for i := 0; i < 10; i++ {
		if n := f.Next(); n != uint64(i) {
			t.Errorf("expected %d, got %d", i, n)
		}
	}
}
