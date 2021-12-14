// Copyright 2021 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package distributedtracing

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/test/e2e"
	"github.com/open-policy-agent/opa/tracing"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

var testRuntime *e2e.TestRuntime
var spanExporter *tracetest.InMemoryExporter

func TestMain(m *testing.M) {
	spanExporter = tracetest.NewInMemoryExporter()
	options := tracing.NewOptions(
		otelhttp.WithTracerProvider(trace.NewTracerProvider(trace.WithSpanProcessor(trace.NewSimpleSpanProcessor(spanExporter)))),
	)

	flag.Parse()
	testServerParams := e2e.NewAPIServerTestParams()
	testServerParams.DistributedTracingOpts = options

	var err error
	testRuntime, err = e2e.NewTestRuntime(testServerParams)
	if err != nil {
		os.Exit(1)
	}

	os.Exit(testRuntime.RunTests(m))
}

// TestServerSpan exemplarily asserts that the server handlers emit OpenTelemetry spans
// with the correct attributes. It does NOT exercise all handlers, but contains one test
// with a GET and one with a POST.
func TestServerSpan(t *testing.T) {
	spanExporter.Reset()

	t.Run("POST v0/data", func(t *testing.T) {
		t.Cleanup(spanExporter.Reset)

		mr, err := http.Post(testRuntime.URL()+"/v0/data", "application/json", nil)
		if err != nil {
			t.Fatal(err)
		}
		defer mr.Body.Close()

		spans := spanExporter.GetSpans()
		if got, expected := len(spans), 1; got != expected {
			t.Fatalf("got %d span(s), expected %d", got, expected)
		}
		if !spans[0].SpanContext.IsValid() {
			t.Fatalf("invalid span created: %#v", spans[0].SpanContext)
		}
		if got, expected := spans[0].SpanKind.String(), "server"; got != expected {
			t.Fatalf("Expected span kind to be %q but got %q", expected, got)
		}

		expected := []attribute.KeyValue{
			attribute.String("http.host", strings.Replace(testRuntime.URL(), "http://", "", 1)),
			attribute.String("http.method", "POST"),
			attribute.String("http.scheme", "http"),
			attribute.String("http.server_name", "v0/data"),
			attribute.Int("http.status_code", 200),
			attribute.String("http.target", "/v0/data"),
			attribute.String("http.user_agent", "Go-http-client/1.1"),
			attribute.Int("http.wrote_bytes", 2),
			attribute.String("net.transport", "ip_tcp"),
		}
		compareSpanAttributes(t, expected, attribute.NewSet(spans[0].Attributes...))
	})

	t.Run("GET v1/data", func(t *testing.T) {
		t.Cleanup(spanExporter.Reset)

		mr, err := http.Get(testRuntime.URL() + "/v1/data")
		if err != nil {
			t.Fatal(err)
		}
		defer mr.Body.Close()

		spans := spanExporter.GetSpans()
		if got, expected := len(spans), 1; got != expected {
			t.Fatalf("got %d span(s), expected %d", got, expected)
		}
		if !spans[0].SpanContext.IsValid() {
			t.Fatalf("invalid span created: %#v", spans[0].SpanContext)
		}
		if got, expected := spans[0].SpanKind.String(), "server"; got != expected {
			t.Fatalf("Expected span kind to be %q but got %q", expected, got)
		}

		expected := []attribute.KeyValue{
			attribute.String("http.host", strings.Replace(testRuntime.URL(), "http://", "", 1)),
			attribute.String("http.method", "GET"),
			attribute.String("http.scheme", "http"),
			attribute.String("http.server_name", "v1/data"),
			attribute.Int("http.status_code", 200),
			attribute.String("http.target", "/v1/data"),
			attribute.String("http.user_agent", "Go-http-client/1.1"),
			attribute.Int("http.wrote_bytes", 66),
			attribute.String("net.transport", "ip_tcp"),
		}
		compareSpanAttributes(t, expected, attribute.NewSet(spans[0].Attributes...))
	})
}

// TestClientSpan asserts that for all handlers that end up evaluating policies, the
// http.send calls will emit the proper spans related to the incoming requests.
//
// NOTE(sr): `{GET,POST} v1/query` are omitted, http.send is forbidden for ad-hoc queries
func TestClientSpan(t *testing.T) {
	type resp struct {
		DecisionID string `json:"decision_id"`
	}

	policy := `
	package test

	response := http.send({"method": "get", "url": "%s/health"})
	`

	policy = fmt.Sprintf(policy, testRuntime.URL())
	err := testRuntime.UploadPolicy(t.Name(), strings.NewReader(policy))
	if err != nil {
		t.Fatal(err)
	}
	spanExporter.Reset()

	t.Run("POST v0/data", func(t *testing.T) {
		t.Cleanup(spanExporter.Reset)

		mr, err := http.Post(testRuntime.URL()+"/v0/data/test", "application/json", nil)
		if err != nil {
			t.Fatal(err)
		}
		defer mr.Body.Close()

		spans := spanExporter.GetSpans()

		// Ordered by span emission, which is the reverse of the processing
		// code flow:
		// 3 = GET /health (HTTP server handler)
		//     + http.send (HTTP client instrumentation)
		//     + GET /v1/data/test (HTTP server handler)
		if got, expected := len(spans), 3; got != expected {
			t.Fatalf("got %d span(s), expected %d", got, expected)
		}
		if !spans[1].SpanContext.IsValid() {
			t.Fatalf("invalid span created: %#v", spans[1].SpanContext)
		}
		if got, expected := spans[1].SpanKind.String(), "client"; got != expected {
			t.Fatalf("Expected span kind to be %q but got %q", expected, got)
		}

		parentSpanID := spans[2].SpanContext.SpanID()
		if got, expected := spans[1].Parent.SpanID(), parentSpanID; got != expected {
			t.Errorf("expected span to be child of %v, got parent %v", expected, got)
		}

		expected := []attribute.KeyValue{
			attribute.String("http.method", "GET"),
			attribute.String("http.url", testRuntime.URL()+"/health"),
			attribute.String("http.scheme", "http"),
			attribute.String("http.host", strings.Replace(testRuntime.URL(), "http://", "", 1)),
			attribute.String("http.flavor", "1.1"),
			attribute.Int("http.status_code", 200),
		}
		compareSpanAttributes(t, expected, attribute.NewSet(spans[1].Attributes...))
	})

	t.Run("GET v1/data", func(t *testing.T) {
		t.Cleanup(spanExporter.Reset)

		mr, err := http.Get(testRuntime.URL() + "/v1/data/test")
		if err != nil {
			t.Fatal(err)
		}
		defer mr.Body.Close()
		var r resp
		if err := json.NewDecoder(mr.Body).Decode(&r); err != nil {
			t.Fatal(err)
		}
		if r.DecisionID == "" {
			t.Fatal("expected decision id")
		}

		spans := spanExporter.GetSpans()
		if got, expected := len(spans), 3; got != expected {
			t.Fatalf("got %d span(s), expected %d", got, expected)
		}
		if !spans[1].SpanContext.IsValid() {
			t.Fatalf("invalid span created: %#v", spans[1].SpanContext)
		}
		if got, expected := spans[1].SpanKind.String(), "client"; got != expected {
			t.Fatalf("Expected span kind to be %q but got %q", expected, got)
		}

		parentSpanID := spans[2].SpanContext.SpanID()
		if got, expected := spans[1].Parent.SpanID(), parentSpanID; got != expected {
			t.Errorf("expected span to be child of %v, got parent %v", expected, got)
		}

		expected := []attribute.KeyValue{
			attribute.String("http.method", "GET"),
			attribute.String("http.url", testRuntime.URL()+"/health"),
			attribute.String("http.scheme", "http"),
			attribute.String("http.host", strings.Replace(testRuntime.URL(), "http://", "", 1)),
			attribute.String("http.flavor", "1.1"),
			attribute.Int("http.status_code", 200),
		}
		compareSpanAttributes(t, expected, attribute.NewSet(spans[1].Attributes...))

		// The (parent) server span carries the decision ID
		expected = []attribute.KeyValue{
			attribute.String("opa.decision_id", r.DecisionID),
		}
		compareSpanAttributes(t, expected, attribute.NewSet(spans[2].Attributes...))
	})

	t.Run("POST v1/data", func(t *testing.T) {
		t.Cleanup(spanExporter.Reset)

		payload := strings.NewReader(`{"input": "meow"}`)
		mr, err := http.Post(testRuntime.URL()+"/v1/data/test", "application/json", payload)
		if err != nil {
			t.Fatal(err)
		}
		defer mr.Body.Close()
		var r resp
		if err := json.NewDecoder(mr.Body).Decode(&r); err != nil {
			t.Fatal(err)
		}
		if r.DecisionID == "" {
			t.Fatal("expected decision id")
		}

		spans := spanExporter.GetSpans()
		if got, expected := len(spans), 3; got != expected {
			t.Fatalf("got %d span(s), expected %d", got, expected)
		}
		if !spans[1].SpanContext.IsValid() {
			t.Fatalf("invalid span created: %#v", spans[1].SpanContext)
		}
		if got, expected := spans[1].SpanKind.String(), "client"; got != expected {
			t.Fatalf("Expected span kind to be %q but got %q", expected, got)
		}

		parentSpanID := spans[2].SpanContext.SpanID()
		if got, expected := spans[1].Parent.SpanID(), parentSpanID; got != expected {
			t.Errorf("expected span to be child of %v, got parent %v", expected, got)
		}

		expected := []attribute.KeyValue{
			attribute.String("http.method", "GET"),
			attribute.String("http.url", testRuntime.URL()+"/health"),
			attribute.String("http.scheme", "http"),
			attribute.String("http.host", strings.Replace(testRuntime.URL(), "http://", "", 1)),
			attribute.String("http.flavor", "1.1"),
			attribute.Int("http.status_code", 200),
		}
		compareSpanAttributes(t, expected, attribute.NewSet(spans[1].Attributes...))

		// The (parent) server span carries the decision ID
		expected = []attribute.KeyValue{
			attribute.String("opa.decision_id", r.DecisionID),
		}
		compareSpanAttributes(t, expected, attribute.NewSet(spans[2].Attributes...))
	})

	t.Run("POST /", func(t *testing.T) {
		t.Cleanup(spanExporter.Reset)

		main := fmt.Sprintf(`
		package system.main

		response := http.send({"method": "get", "url": "%s/health"})
		`, testRuntime.URL())
		err := testRuntime.UploadPolicy("system.main", strings.NewReader(main))
		if err != nil {
			t.Fatal(err)
		}
		spanExporter.Reset()

		mr, err := http.Post(testRuntime.URL()+"/", "application/json", nil)
		if err != nil {
			t.Fatal(err)
		}
		defer mr.Body.Close()

		spans := spanExporter.GetSpans()
		if got, expected := len(spans), 3; got != expected {
			t.Fatalf("got %d span(s), expected %d", got, expected)
		}
		if !spans[1].SpanContext.IsValid() {
			t.Fatalf("invalid span created: %#v", spans[1].SpanContext)
		}
		if got, expected := spans[1].SpanKind.String(), "client"; got != expected {
			t.Fatalf("Expected span kind to be %q but got %q", expected, got)
		}

		parentSpanID := spans[2].SpanContext.SpanID()
		if got, expected := spans[1].Parent.SpanID(), parentSpanID; got != expected {
			t.Errorf("expected span to be child of %v, got parent %v", expected, got)
		}

		expected := []attribute.KeyValue{
			attribute.String("http.method", "GET"),
			attribute.String("http.url", testRuntime.URL()+"/health"),
			attribute.String("http.scheme", "http"),
			attribute.String("http.host", strings.Replace(testRuntime.URL(), "http://", "", 1)),
			attribute.String("http.flavor", "1.1"),
			attribute.Int("http.status_code", 200),
		}
		compareSpanAttributes(t, expected, attribute.NewSet(spans[1].Attributes...))
	})
}

func compareSpanAttributes(t *testing.T, expectedAttributes []attribute.KeyValue, spanAttributes attribute.Set) {
	t.Helper()
	for _, exp := range expectedAttributes {
		value, exists := spanAttributes.Value(exp.Key)
		if !exists {
			t.Fatalf("Expected span attributes to contain %q key", exp.Key)
		}
		if value != exp.Value {
			t.Fatalf("Expected %q attribute to be %s but got %s", exp.Key, exp.Value.Emit(), value.Emit())
		}
	}
}
