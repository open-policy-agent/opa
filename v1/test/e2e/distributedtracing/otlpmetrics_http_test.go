// Copyright 2024 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package distributedtracing

import (
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/open-policy-agent/opa/v1/runtime"
	"github.com/open-policy-agent/opa/v1/test/e2e"
	colmetricpb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	"google.golang.org/protobuf/proto"
)

// mockOTLPHTTPCollector is a lightweight mock OTLP HTTP collector that records
// incoming ExportMetricsServiceRequest messages.
type mockOTLPHTTPCollector struct {
	mu       sync.Mutex
	requests []*colmetricpb.ExportMetricsServiceRequest
	server   *httptest.Server
}

func newMockOTLPHTTPCollector() *mockOTLPHTTPCollector {
	c := &mockOTLPHTTPCollector{}
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/metrics", c.handleMetrics)
	c.server = httptest.NewServer(mux)
	return c
}

func (c *mockOTLPHTTPCollector) handleMetrics(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	req := &colmetricpb.ExportMetricsServiceRequest{}
	if err := proto.Unmarshal(body, req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	c.mu.Lock()
	c.requests = append(c.requests, req)
	c.mu.Unlock()

	w.WriteHeader(http.StatusOK)
}

func (c *mockOTLPHTTPCollector) getRequests() []*colmetricpb.ExportMetricsServiceRequest {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]*colmetricpb.ExportMetricsServiceRequest, len(c.requests))
	copy(out, c.requests)
	return out
}

func (c *mockOTLPHTTPCollector) stop() {
	c.server.Close()
}

// metricNames extracts all metric names from the collected requests.
func (c *mockOTLPHTTPCollector) metricNames() map[string]bool {
	names := make(map[string]bool)
	for _, req := range c.getRequests() {
		for _, rm := range req.GetResourceMetrics() {
			for _, sm := range rm.GetScopeMetrics() {
				for _, m := range sm.GetMetrics() {
					names[m.GetName()] = true
				}
			}
		}
	}
	return names
}

// address returns the host:port portion of the collector URL (no scheme).
func (c *mockOTLPHTTPCollector) address() string {
	// Strip "http://" prefix
	return c.server.URL[len("http://"):]
}

func TestOTLPMetricsExportHTTP(t *testing.T) {
	collector := newMockOTLPHTTPCollector()
	defer collector.stop()

	testServerParams := e2e.NewAPIServerTestParams()
	testServerParams.ConfigOverrides = []string{
		"distributed_tracing.type=http",
		"distributed_tracing.address=" + collector.address(),
		"distributed_tracing.metrics=true",
		"distributed_tracing.metrics_export_interval_ms=500",
	}
	testServerParams.Logging = runtime.LoggingConfig{Level: "error"}

	e2e.WithRuntime(t, e2e.TestRuntimeOpts{}, testServerParams, func(rt *e2e.TestRuntime) {
		// Make a few requests to OPA to generate HTTP metrics.
		for range 3 {
			resp, err := http.Get(rt.URL() + "/v1/data")
			if err != nil {
				t.Fatal(err)
			}
			resp.Body.Close()
		}

		// Wait for the periodic reader to export. The interval is 500ms,
		// so we poll for up to 5 seconds.
		var names map[string]bool
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			names = collector.metricNames()
			if names["http_request_duration_seconds"] {
				break
			}
			time.Sleep(250 * time.Millisecond)
		}

		if !names["http_request_duration_seconds"] {
			t.Fatalf("expected http_request_duration_seconds metric to be exported, got: %v", names)
		}
	})
}
