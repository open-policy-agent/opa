// Copyright 2024 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package distributedtracing

import (
	"testing"

	prometheus_client "github.com/prometheus/client_golang/prometheus"
)

func TestParseDistributedTracingConfigMetricsDefaults(t *testing.T) {
	cfg, err := parseDistributedTracingConfig(nil)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Metrics == nil || *cfg.Metrics != false {
		t.Fatal("expected metrics to default to false")
	}
	if cfg.MetricsExportIntervalMs == nil || *cfg.MetricsExportIntervalMs != 60000 {
		t.Fatalf("expected metrics_export_interval_ms to default to 60000, got %v", *cfg.MetricsExportIntervalMs)
	}
}

func TestParseDistributedTracingConfigMetricsEnabled(t *testing.T) {
	raw := []byte(`{"type": "grpc", "metrics": true}`)
	cfg, err := parseDistributedTracingConfig(raw)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Metrics == nil || !*cfg.Metrics {
		t.Fatal("expected metrics to be true")
	}
	if cfg.MetricsExportIntervalMs == nil || *cfg.MetricsExportIntervalMs != 60000 {
		t.Fatal("expected default metrics_export_interval_ms")
	}
}

func TestParseDistributedTracingConfigMetricsDisabled(t *testing.T) {
	raw := []byte(`{"type": "grpc", "metrics": false}`)
	cfg, err := parseDistributedTracingConfig(raw)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Metrics == nil || *cfg.Metrics {
		t.Fatal("expected metrics to be false")
	}
}

func TestParseDistributedTracingConfigMetricsCustomInterval(t *testing.T) {
	raw := []byte(`{"type": "grpc", "metrics": true, "metrics_export_interval_ms": 30000}`)
	cfg, err := parseDistributedTracingConfig(raw)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MetricsExportIntervalMs == nil || *cfg.MetricsExportIntervalMs != 30000 {
		t.Fatalf("expected metrics_export_interval_ms to be 30000, got %v", *cfg.MetricsExportIntervalMs)
	}
}

func TestValidateMetricsWithoutTypeReturnsError(t *testing.T) {
	raw := []byte(`{"metrics": true}`)
	_, err := parseDistributedTracingConfig(raw)
	if err == nil {
		t.Fatal("expected error when metrics is true but type is not set")
	}
	expected := "distributed_tracing.type must be set when distributed_tracing.metrics is enabled"
	if err.Error() != expected {
		t.Fatalf("expected error %q, got %q", expected, err.Error())
	}
}

func TestInitMetricsEnabledNilGathererReturnsNilMeterProvider(t *testing.T) {
	raw := []byte(`{"distributed_tracing": {"type": "grpc", "metrics": true}}`)
	_, _, _, mp, err := Init(t.Context(), raw, "test", nil)
	if err != nil {
		t.Fatal(err)
	}
	if mp != nil {
		t.Fatal("expected nil MeterProvider when gatherer is nil")
	}
}

func TestInitMetricsDisabledReturnsNilMeterProvider(t *testing.T) {
	raw := []byte(`{"distributed_tracing": {"type": "grpc", "metrics": false}}`)
	_, _, _, mp, err := Init(t.Context(), raw, "test", nil)
	if err != nil {
		t.Fatal(err)
	}
	if mp != nil {
		t.Fatal("expected nil MeterProvider when metrics is disabled")
	}
}

func TestInitNoTypeReturnsAllNil(t *testing.T) {
	raw := []byte(`{"distributed_tracing": {}}`)
	exp, tp, res, mp, err := Init(t.Context(), raw, "test", nil)
	if err != nil {
		t.Fatal(err)
	}
	if exp != nil || tp != nil || res != nil || mp != nil {
		t.Fatal("expected all nil when type is not set")
	}
}

func TestInitMetricsEnabledWithGathererReturnsNonNilMeterProvider(t *testing.T) {
	registry := prometheus_client.NewRegistry()
	raw := []byte(`{"distributed_tracing": {"type": "grpc", "metrics": true}}`)
	_, _, _, mp, err := Init(t.Context(), raw, "test", registry)
	if err != nil {
		t.Fatal(err)
	}
	if mp == nil {
		t.Fatal("expected non-nil MeterProvider when metrics is enabled with a gatherer")
	}
	// Shutdown will fail because no collector is running, but that's expected.
	_ = mp.Shutdown(t.Context())
}

func TestInitMetricsEnabledHTTPWithGathererReturnsNonNilMeterProvider(t *testing.T) {
	registry := prometheus_client.NewRegistry()
	raw := []byte(`{"distributed_tracing": {"type": "http", "metrics": true}}`)
	_, _, _, mp, err := Init(t.Context(), raw, "test", registry)
	if err != nil {
		t.Fatal(err)
	}
	if mp == nil {
		t.Fatal("expected non-nil MeterProvider when metrics is enabled with HTTP type and a gatherer")
	}
	_ = mp.Shutdown(t.Context())
}

func TestValidateMetricsExportIntervalMsZero(t *testing.T) {
	raw := []byte(`{"type": "grpc", "metrics": true, "metrics_export_interval_ms": 0}`)
	_, err := parseDistributedTracingConfig(raw)
	if err == nil {
		t.Fatal("expected error when metrics_export_interval_ms is 0")
	}
}

func TestValidateMetricsExportIntervalMsNegative(t *testing.T) {
	raw := []byte(`{"type": "grpc", "metrics": true, "metrics_export_interval_ms": -1}`)
	_, err := parseDistributedTracingConfig(raw)
	if err == nil {
		t.Fatal("expected error when metrics_export_interval_ms is negative")
	}
}

func TestBuildTLSConfigOff(t *testing.T) {
	tlsConfig, err := buildTLSConfig("off", false, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if tlsConfig != nil {
		t.Fatal("expected nil tls config for 'off' encryption")
	}
}

func TestBuildTLSConfigMTLSNoCert(t *testing.T) {
	_, err := buildTLSConfig("mtls", false, nil, nil)
	if err == nil {
		t.Fatal("expected error for mtls without cert")
	}
}
