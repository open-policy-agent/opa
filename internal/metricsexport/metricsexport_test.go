// Copyright 2024 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package metricsexport

import (
	"testing"

	prometheus_client "github.com/prometheus/client_golang/prometheus"
)

func TestParseMetricsExportConfigDefaults(t *testing.T) {
	cfg, err := parseMetricsExportConfig(nil)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Type != "" {
		t.Fatal("expected empty type for nil config")
	}
}

func TestParseMetricsExportConfigOTLPGRPC(t *testing.T) {
	raw := []byte(`{"type": "otlp/grpc"}`)
	cfg, err := parseMetricsExportConfig(raw)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Address != "localhost:4317" {
		t.Fatalf("expected default gRPC address, got %s", cfg.Address)
	}
}

func TestParseMetricsExportConfigOTLPHTTP(t *testing.T) {
	raw := []byte(`{"type": "otlp/http"}`)
	cfg, err := parseMetricsExportConfig(raw)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Address != "localhost:4318" {
		t.Fatalf("expected default HTTP address, got %s", cfg.Address)
	}
}

func TestParseMetricsExportConfigCustomInterval(t *testing.T) {
	raw := []byte(`{"type": "otlp/grpc", "export_interval_ms": 30000}`)
	cfg, err := parseMetricsExportConfig(raw)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ExportIntervalMs == nil || *cfg.ExportIntervalMs != 30000 {
		t.Fatalf("expected export_interval_ms to be 30000, got %v", *cfg.ExportIntervalMs)
	}
}

func TestValidateMetricsExportInvalidType(t *testing.T) {
	raw := []byte(`{"type": "unknown"}`)
	_, err := parseMetricsExportConfig(raw)
	if err == nil {
		t.Fatal("expected error for invalid type")
	}
}

func TestValidateMetricsExportIntervalZero(t *testing.T) {
	raw := []byte(`{"type": "otlp/grpc", "export_interval_ms": 0}`)
	_, err := parseMetricsExportConfig(raw)
	if err == nil {
		t.Fatal("expected error when export_interval_ms is 0")
	}
}

func TestValidateMetricsExportIntervalNegative(t *testing.T) {
	raw := []byte(`{"type": "otlp/grpc", "export_interval_ms": -1}`)
	_, err := parseMetricsExportConfig(raw)
	if err == nil {
		t.Fatal("expected error when export_interval_ms is negative")
	}
}

func TestInitDisabled(t *testing.T) {
	raw := []byte(`{}`)
	mp, err := Init(t.Context(), raw, "test", prometheus_client.NewRegistry())
	if err != nil {
		t.Fatal(err)
	}
	if mp != nil {
		t.Fatal("expected nil MeterProvider when type is empty")
	}
}

func TestInitNilGatherer(t *testing.T) {
	raw := []byte(`{"metrics_export": {"type": "otlp/grpc"}}`)
	mp, err := Init(t.Context(), raw, "test", nil)
	if err != nil {
		t.Fatal(err)
	}
	if mp != nil {
		t.Fatal("expected nil MeterProvider when gatherer is nil")
	}
}

func TestInitGRPCWithGatherer(t *testing.T) {
	registry := prometheus_client.NewRegistry()
	raw := []byte(`{"metrics_export": {"type": "otlp/grpc"}}`)
	mp, err := Init(t.Context(), raw, "test", registry)
	if err != nil {
		t.Fatal(err)
	}
	if mp == nil {
		t.Fatal("expected non-nil MeterProvider")
	}
	_ = mp.Shutdown(t.Context())
}

func TestInitHTTPWithGatherer(t *testing.T) {
	registry := prometheus_client.NewRegistry()
	raw := []byte(`{"metrics_export": {"type": "otlp/http"}}`)
	mp, err := Init(t.Context(), raw, "test", registry)
	if err != nil {
		t.Fatal(err)
	}
	if mp == nil {
		t.Fatal("expected non-nil MeterProvider")
	}
	_ = mp.Shutdown(t.Context())
}
