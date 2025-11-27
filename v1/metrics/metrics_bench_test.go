// Copyright 2025 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package metrics_test

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/open-policy-agent/opa/v1/metrics"
)

func BenchmarkMetricsMarshaling(b *testing.B) {
	m := metrics.New()

	// Setup a handful of metrics across each type.
	for i := range 10 {
		m.Timer(fmt.Sprintf("rego_timer_example_%d", i)).Start()
	}
	time.Sleep(1 * time.Millisecond)
	for i := range 10 {
		m.Timer(fmt.Sprintf("rego_timer_example_%d", i)).Stop()
	}

	for i := range 10 {
		m.Counter(fmt.Sprintf("rego_counter_example_%d", i)).Add(uint64(i))
	}

	for i := range 10 {
		for j := range 100 {
			m.Histogram(fmt.Sprintf("rego_histogram_example_%d", i)).Update(int64(i + j))
		}
	}

	for b.Loop() {
		bs, err := json.Marshal(m)
		if err != nil {
			b.Fatalf("Unexpected error: %v", err)
		}
		if len(bs) == 0 {
			b.Fatalf("No output")
		}
	}
}

func BenchmarkMetricsTimerStartStopRestart(b *testing.B) {
	m := metrics.New()

	for b.Loop() {
		m.Timer("foo").Start()
		_ = m.Timer("foo").Stop()
		_ = m.Timer("foo").Stop() // Second stop to exercise the sync guard.
		m.Timer("foo").Start()
		_ = m.Timer("foo").Stop()
	}
}
