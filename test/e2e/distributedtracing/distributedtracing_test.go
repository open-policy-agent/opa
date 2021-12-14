// Copyright 2021 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package distributedtracing

import (
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

func TestServerSpan(t *testing.T) {
	spanExporter.Reset()

	mr, err := http.Get(testRuntime.URL() + "/v1/data/test")
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

	attributes := attribute.NewSet(spans[0].Attributes...)

	expected := []attribute.KeyValue{
		attribute.String("http.host", strings.Replace(testRuntime.URL(), "http://", "", 1)),
		attribute.String("http.method", "GET"),
		attribute.String("http.scheme", "http"),
		attribute.String("http.server_name", "v1/data"),
		attribute.Int("http.status_code", 200),
		attribute.String("http.target", "/v1/data/test"),
		attribute.String("http.user_agent", "Go-http-client/1.1"),
		attribute.Int("http.wrote_bytes", 2),
		attribute.String("net.transport", "ip_tcp"),
	}
	compareSpanAttributes(t, expected, attributes)
}

func TestClientSpan(t *testing.T) {
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

	mr, err := http.Get(testRuntime.URL() + "/v1/data/test")
	if err != nil {
		t.Fatal(err)
	}

	defer mr.Body.Close()

	spans := spanExporter.GetSpans()

	// 3 = GET /v1/data/test (HTTP server handler)
	//     + http.send (HTTP client instrumentation)
	//     + GET /health (HTTP server handler)
	if got, expected := len(spans), 3; got != expected {
		t.Fatalf("got %d span(s), expected %d", got, expected)
	}
	if !spans[1].SpanContext.IsValid() {
		t.Fatalf("invalid span created: %#v", spans[1].SpanContext)
	}
	if got, expected := spans[1].SpanKind.String(), "client"; got != expected {
		t.Fatalf("Expected span kind to be %q but got %q", expected, got)
	}

	attributes := attribute.NewSet(spans[1].Attributes...)

	expected := []attribute.KeyValue{
		attribute.String("http.method", "GET"),
		attribute.String("http.url", testRuntime.URL()+"/health"),
		attribute.String("http.scheme", "http"),
		attribute.String("http.host", strings.Replace(testRuntime.URL(), "http://", "", 1)),
		attribute.String("http.flavor", "1.1"),
		attribute.Int("http.status_code", 200),
	}
	compareSpanAttributes(t, expected, attributes)
}

func compareSpanAttributes(t *testing.T, expectedAttributes []attribute.KeyValue, spanAttributes attribute.Set) {
	for _, exp := range expectedAttributes {
		value, exists := spanAttributes.Value(exp.Key)
		if !exists {
			t.Fatalf("Expected span attributes to contain %q key", exp.Key)
		}
		if value != exp.Value {
			t.Fatalf("Expected %q attribute to be %q but got %q", exp.Key, exp.Value.AsString(), value.AsString())
		}
	}
}
