// Copyright 2024 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package distributedtracing

import (
	"context"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/open-policy-agent/opa/v1/runtime"
	"github.com/open-policy-agent/opa/v1/test/e2e"
	colmetricpb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	"google.golang.org/grpc"
)

// mockOTLPGRPCCollector is a lightweight mock OTLP gRPC collector that records
// incoming ExportMetricsServiceRequest messages.
type mockOTLPGRPCCollector struct {
	colmetricpb.UnimplementedMetricsServiceServer
	mu       sync.Mutex
	requests []*colmetricpb.ExportMetricsServiceRequest
	server   *grpc.Server
	addr     string
}

func newMockOTLPGRPCCollector(t *testing.T) *mockOTLPGRPCCollector {
	t.Helper()

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	c := &mockOTLPGRPCCollector{
		server: grpc.NewServer(),
		addr:   lis.Addr().String(),
	}
	colmetricpb.RegisterMetricsServiceServer(c.server, c)

	go func() {
		if err := c.server.Serve(lis); err != nil {
			// Server.Serve returns after GracefulStop; ignore.
		}
	}()

	return c
}

func (c *mockOTLPGRPCCollector) Export(_ context.Context, req *colmetricpb.ExportMetricsServiceRequest) (*colmetricpb.ExportMetricsServiceResponse, error) {
	c.mu.Lock()
	c.requests = append(c.requests, req)
	c.mu.Unlock()
	return &colmetricpb.ExportMetricsServiceResponse{}, nil
}

func (c *mockOTLPGRPCCollector) getRequests() []*colmetricpb.ExportMetricsServiceRequest {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]*colmetricpb.ExportMetricsServiceRequest, len(c.requests))
	copy(out, c.requests)
	return out
}

func (c *mockOTLPGRPCCollector) stop() {
	c.server.GracefulStop()
}

// metricNames extracts all metric names from the collected requests.
func (c *mockOTLPGRPCCollector) metricNames() map[string]bool {
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

// address returns the host:port of the gRPC collector.
func (c *mockOTLPGRPCCollector) address() string {
	return c.addr
}

func TestOTLPMetricsExportGRPC(t *testing.T) {
	collector := newMockOTLPGRPCCollector(t)
	defer collector.stop()

	testServerParams := e2e.NewAPIServerTestParams()
	testServerParams.ConfigOverrides = []string{
		"distributed_tracing.type=grpc",
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
