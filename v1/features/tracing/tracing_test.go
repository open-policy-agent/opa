// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package tracing

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	pkg_tracing "github.com/open-policy-agent/opa/v1/tracing"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestServerSpanNameFormatter(t *testing.T) {
	cases := []struct {
		name      string
		method    string
		operation string
		want      string
	}{
		{"GET v1/data", "GET", "v1/data", "GET v1/data"},
		{"POST v1/data", "POST", "v1/data", "POST v1/data"},
		{"DELETE v1/policies", "DELETE", "v1/policies", "DELETE v1/policies"},
		{"empty operation", "GET", "", "GET"},
		{"empty method", "", "v1/data", "v1/data"},
		{"nil request", "", "v1/data", "v1/data"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var r *http.Request
			if tc.name != "nil request" {
				r = &http.Request{Method: tc.method, URL: &url.URL{Path: "/whatever"}}
			}
			if got := serverSpanName(tc.operation, r); got != tc.want {
				t.Fatalf("serverSpanName(%q, %+v) = %q, want %q", tc.operation, r, got, tc.want)
			}
		})
	}
}

func TestClientSpanNameFormatter(t *testing.T) {
	cases := []struct {
		name   string
		method string
		path   string
		want   string
	}{
		{"GET with path", "GET", "/v1/data", "GET /v1/data"},
		{"POST with path", "POST", "/v1/logs", "POST /v1/logs"},
		{"GET no path", "GET", "", "GET"},
		{"empty method with path", "", "/health", "HTTP /health"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := &http.Request{Method: tc.method, URL: &url.URL{Path: tc.path}}
			if got := clientSpanName("", r); got != tc.want {
				t.Fatalf("clientSpanName(%q, path=%q) = %q, want %q", tc.method, tc.path, got, tc.want)
			}
		})
	}

	t.Run("nil request", func(t *testing.T) {
		if got := clientSpanName("", nil); got != "HTTP" {
			t.Fatalf("clientSpanName(nil) = %q, want %q", got, "HTTP")
		}
	})
}

// TestHandlerSpanNameEndToEnd asserts that the handler factory wraps
// otelhttp.NewHandler such that the recorded server span is named
// "{method} {operation}" per OpenTelemetry HTTP semantic conventions.
func TestHandlerSpanNameEndToEnd(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := trace.NewTracerProvider(trace.WithSpanProcessor(trace.NewSimpleSpanProcessor(exporter)))
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })

	opts := pkg_tracing.NewOptions(otelhttp.WithTracerProvider(tp))

	f := &factory{}
	h := f.NewHandler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}), "v1/data", opts)

	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	resp, err := http.Post(srv.URL+"/v1/data/foo", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()

	spans := exporter.GetSpans()
	if got, want := len(spans), 1; got != want {
		t.Fatalf("got %d span(s), want %d", got, want)
	}
	if got, want := spans[0].Name, "POST v1/data"; got != want {
		t.Fatalf("span name = %q, want %q", got, want)
	}
}

// TestUserSpanNameFormatterOverrides asserts that any user-supplied
// WithSpanNameFormatter in the Options still wins over OPA's default.
func TestUserSpanNameFormatterOverrides(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := trace.NewTracerProvider(trace.WithSpanProcessor(trace.NewSimpleSpanProcessor(exporter)))
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })

	custom := func(string, *http.Request) string { return "custom-name" }
	opts := pkg_tracing.NewOptions(
		otelhttp.WithTracerProvider(tp),
		otelhttp.WithSpanNameFormatter(custom),
	)

	f := &factory{}
	h := f.NewHandler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}), "v1/data", opts)

	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	resp, err := http.Get(srv.URL + "/anything")
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()

	spans := exporter.GetSpans()
	if got, want := len(spans), 1; got != want {
		t.Fatalf("got %d span(s), want %d", got, want)
	}
	if got, want := spans[0].Name, "custom-name"; got != want {
		t.Fatalf("span name = %q, want %q", got, want)
	}
}

// TestTransportSpanNameEndToEnd asserts that the transport factory names
// outbound HTTP client spans as "{method} {path}".
func TestTransportSpanNameEndToEnd(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := trace.NewTracerProvider(trace.WithSpanProcessor(trace.NewSimpleSpanProcessor(exporter)))
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(upstream.Close)

	opts := pkg_tracing.NewOptions(otelhttp.WithTracerProvider(tp))

	f := &factory{}
	client := &http.Client{Transport: f.NewTransport(http.DefaultTransport, opts)}

	req, err := http.NewRequest(http.MethodPost, upstream.URL+"/v1/logs", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()

	spans := exporter.GetSpans()
	if got, want := len(spans), 1; got != want {
		t.Fatalf("got %d span(s), want %d", got, want)
	}
	if got, want := spans[0].Name, "POST /v1/logs"; got != want {
		t.Fatalf("span name = %q, want %q", got, want)
	}
}
