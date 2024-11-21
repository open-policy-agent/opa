// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package metrics

import (
	"testing"
	"time"
)

func TestMetricsTimer(t *testing.T) {
	m := New()
	m.Timer("foo").Start()
	time.Sleep(time.Millisecond)
	m.Timer("foo").Stop()
	if m.All()["timer_foo_ns"] == 0 {
		t.Fatalf("Expected foo timer to be non-zero: %v", m.All())
	}
	m.Clear()

	if len(m.All()) > 0 {
		t.Fatalf("Expected metrics to be cleared, but found %v", m.All())
	}
}
