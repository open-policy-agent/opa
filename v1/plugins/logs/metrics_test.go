// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

//go:build slow

package logs

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/v1/logging/test"
	"github.com/open-policy-agent/opa/v1/metrics"
	"github.com/open-policy-agent/opa/v1/plugins"
	"github.com/open-policy-agent/opa/v1/plugins/status"
	"github.com/open-policy-agent/opa/v1/server"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestPluginBufferSize(t *testing.T) {
	t.Parallel()

	for _, bufferType := range []string{eventBufferType, sizeBufferType} {
		t.Run(bufferType, func(t *testing.T) {
			t.Parallel()

			ctx := t.Context()
			fixture := newTestFixture(t, testFixtureOptions{
				ReportingBufferType: bufferType,
				TriggerMode:         plugins.TriggerManual,
			})
			defer fixture.server.stop()
			fixture.server.ch = make(chan []EventV1, 10)

			for i := range 20 {
				if err := fixture.plugin.Log(ctx, &server.Info{DecisionID: strconv.Itoa(i)}); err != nil {
					t.Fatal(err)
				}
			}
			if bufferType == eventBufferType {
				assertBufferSize(t, fixture.plugin, bufferType, 20)
			}

			// A failed upload requeues the chunk, which is what the size should show.
			fixture.server.expCode = 500
			if err := fixture.plugin.Trigger(ctx); err == nil {
				t.Fatal("expected upload to fail")
			}
			<-fixture.server.ch

			gotType, size := fixture.plugin.BufferSize()
			if gotType != bufferType {
				t.Fatalf("expected buffer type %q, got %q", bufferType, gotType)
			}
			switch {
			case bufferType == eventBufferType && size != 1:
				t.Fatalf("expected the requeued chunk as the only item, got %d", size)
			case size == 0:
				t.Fatal("expected the requeued chunk in the buffer")
			}

			fixture.server.expCode = 200
			if err := fixture.plugin.Trigger(ctx); err != nil {
				t.Fatal(err)
			}
			<-fixture.server.ch
			assertBufferSize(t, fixture.plugin, bufferType, 0)
		})
	}
}

func TestStatusPrometheusExportsBufferSize(t *testing.T) {
	t.Parallel()

	for _, enabled := range []bool{true, false} {
		t.Run(fmt.Sprintf("prometheus=%t", enabled), func(t *testing.T) {
			t.Parallel()

			ctx := t.Context()
			reg := prometheus.NewRegistry()
			fixture := newTestFixture(t, testFixtureOptions{
				ConsoleLogger:       test.New(),
				ReportingBufferType: eventBufferType,
				PrometheusRegister:  reg,
				TriggerMode:         plugins.TriggerManual,
			})
			defer fixture.server.stop()

			config, err := status.ParseConfig(fmt.Appendf(nil, `{"console": true, "prometheus": %t}`, enabled), fixture.manager.Services(), nil)
			if err != nil {
				t.Fatal(err)
			}
			fixture.manager.Register(Name, fixture.plugin)
			fixture.manager.Register(status.Name, status.New(config, fixture.manager))
			if err := fixture.manager.Start(ctx); err != nil {
				t.Fatal(err)
			}
			defer fixture.manager.Stop(ctx)

			for i := range 3 {
				if err := fixture.plugin.Log(ctx, &server.Info{DecisionID: strconv.Itoa(i)}); err != nil {
					t.Fatal(err)
				}
			}

			exp := ""
			if enabled {
				exp = `
# HELP decision_logs_buffer_size_events Number of items waiting in the decision log event buffer.
# TYPE decision_logs_buffer_size_events gauge
decision_logs_buffer_size_events 3
`
			}
			if err := testutil.GatherAndCompare(reg, strings.NewReader(exp), "decision_logs_buffer_size_events"); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestPluginChunkUploadCounters(t *testing.T) {
	t.Parallel()

	for _, bufferType := range []string{eventBufferType, sizeBufferType} {
		t.Run(bufferType, func(t *testing.T) {
			t.Parallel()

			ctx := t.Context()
			m := metrics.New()
			fixture := newTestFixture(t, testFixtureOptions{
				ReportingBufferType: bufferType,
				TriggerMode:         plugins.TriggerManual,
			})
			defer fixture.server.stop()
			fixture.server.ch = make(chan []EventV1, 10)
			fixture.plugin = fixture.plugin.WithMetrics(m)

			if err := fixture.plugin.Start(ctx); err != nil {
				t.Fatal(err)
			}
			defer fixture.plugin.Stop(ctx)

			if err := fixture.plugin.Log(ctx, &server.Info{DecisionID: "1"}); err != nil {
				t.Fatal(err)
			}
			if err := fixture.plugin.Trigger(ctx); err != nil {
				t.Fatal(err)
			}
			<-fixture.server.ch
			assertCounter(t, m, logChunkUploadedCounterName, 1)
			assertCounter(t, m, logChunkUploadFailedCounterName, 0)

			fixture.server.expCode = 500
			if err := fixture.plugin.Log(ctx, &server.Info{DecisionID: "2"}); err != nil {
				t.Fatal(err)
			}
			if err := fixture.plugin.Trigger(ctx); err == nil {
				t.Fatal("expected upload to fail")
			}
			<-fixture.server.ch
			assertCounter(t, m, logChunkUploadedCounterName, 1)
			assertCounter(t, m, logChunkUploadFailedCounterName, 1)
		})
	}
}

func assertBufferSize(t *testing.T, p *Plugin, expType string, exp int64) {
	t.Helper()
	if gotType, got := p.BufferSize(); gotType != expType || got != exp {
		t.Fatalf("expected %s buffer size %d, got %s buffer size %d", expType, exp, gotType, got)
	}
}

func assertCounter(t *testing.T, m metrics.Metrics, name string, exp uint64) {
	t.Helper()
	if got := m.Counter(name).Value().(uint64); got != exp {
		t.Fatalf("expected %s to be %d, got %d", name, exp, got)
	}
}
