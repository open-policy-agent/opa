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

func TestMetricsTimerDoubleStop(t *testing.T) {
	m := New()
	m.Timer("foo").Start()

	time.Sleep(time.Millisecond)
	m.Timer("foo").Stop()
	t1 := m.Timer("foo").Int64()

	time.Sleep(time.Millisecond)
	m.Timer("foo").Stop()
	t2 := m.Timer("foo").Int64()

	if t1 != t2 {
		t.Fatalf("Unexpected difference in stopped timer values: %v, %v", t1, t2)
	}
}

func TestMetricsTimerRestart(t *testing.T) {
	m := New()
	m.Timer("foo").Start()

	time.Sleep(time.Millisecond)
	m.Timer("foo").Stop()
	t1 := m.Timer("foo").Int64()

	// Restart the timer.
	m.Timer("foo").Start()
	time.Sleep(time.Millisecond)
	m.Timer("foo").Stop()
	t2 := m.Timer("foo").Int64()

	if t1 >= t2 {
		t.Fatalf("Expected restarted timer to advance, but got same value.: %v, %v", t1, t2)
	}
}
