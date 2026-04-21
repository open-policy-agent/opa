// Copyright 2024 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package metricsexport

import (
	"context"
	"crypto/tls"
	"fmt"
	"strings"
	"time"

	prometheus_client "github.com/prometheus/client_golang/prometheus"
	otelprometheus "go.opentelemetry.io/contrib/bridges/prometheus"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.7.0"
	"google.golang.org/grpc/credentials"

	"github.com/open-policy-agent/opa/internal/tlsutil"
	"github.com/open-policy-agent/opa/v1/config"
	"github.com/open-policy-agent/opa/v1/util"
)

const (
	defaultGRPCAddress      = "localhost:4317"
	defaultHTTPAddress      = "localhost:4318"
	defaultExportIntervalMs = 60000
	defaultServiceName      = "opa"
	defaultEncryptionScheme = "off"
)

type metricsExportConfig struct {
	Type                  string `json:"type,omitempty"`
	Address               string `json:"address,omitempty"`
	ExportIntervalMs      *int   `json:"export_interval_ms,omitempty"`
	ServiceName           string `json:"service_name,omitempty"`
	EncryptionScheme      string `json:"encryption,omitempty"`
	EncryptionSkipVerify  *bool  `json:"allow_insecure_tls,omitempty"`
	TLSCertFile           string `json:"tls_cert_file,omitempty"`
	TLSCertPrivateKeyFile string `json:"tls_private_key_file,omitempty"`
	TLSCACertFile         string `json:"tls_ca_cert_file,omitempty"`
}

var supportedEncryptionScheme = map[string]struct{}{
	"off": {}, "tls": {}, "mtls": {},
}

func (c *metricsExportConfig) validateAndInjectDefaults() error {
	switch strings.ToLower(c.Type) {
	case "", "otlp/grpc", "otlp/http": // OK
	default:
		return fmt.Errorf("unknown metrics_export.type %q, must be \"otlp/grpc\", \"otlp/http\" or \"\" (unset)", c.Type)
	}

	if c.Address == "" {
		switch strings.ToLower(c.Type) {
		case "otlp/grpc":
			c.Address = defaultGRPCAddress
		case "otlp/http":
			c.Address = defaultHTTPAddress
		}
	}

	if c.ServiceName == "" {
		c.ServiceName = defaultServiceName
	}

	if c.ExportIntervalMs == nil {
		v := defaultExportIntervalMs
		c.ExportIntervalMs = &v
	}
	if *c.ExportIntervalMs <= 0 {
		return fmt.Errorf("metrics_export.export_interval_ms must be a positive value, got %d", *c.ExportIntervalMs)
	}

	if c.EncryptionScheme == "" {
		c.EncryptionScheme = defaultEncryptionScheme
	}
	if _, ok := supportedEncryptionScheme[c.EncryptionScheme]; !ok {
		return fmt.Errorf("unsupported metrics_export.encryption %q", c.EncryptionScheme)
	}

	if c.EncryptionSkipVerify == nil {
		v := false
		c.EncryptionSkipVerify = &v
	}

	return nil
}

func parseMetricsExportConfig(raw []byte) (*metricsExportConfig, error) {
	if raw == nil {
		return &metricsExportConfig{}, nil
	}
	var cfg metricsExportConfig
	if err := util.Unmarshal(raw, &cfg); err != nil {
		return nil, err
	}
	if err := cfg.validateAndInjectDefaults(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func grpcTLSOption(encryptionScheme string, tlsConfig *tls.Config) otlpmetricgrpc.Option {
	if encryptionScheme == "off" {
		return otlpmetricgrpc.WithInsecure()
	}
	return otlpmetricgrpc.WithTLSCredentials(credentials.NewTLS(tlsConfig))
}

func httpTLSOption(encryptionScheme string, tlsConfig *tls.Config) otlpmetrichttp.Option {
	if encryptionScheme == "off" {
		return otlpmetrichttp.WithInsecure()
	}
	return otlpmetrichttp.WithTLSClientConfig(tlsConfig)
}

// Init initializes metrics export based on the provided configuration.
// If the type is empty or the gatherer is nil, it returns nil.
func Init(ctx context.Context, raw []byte, id string, gatherer prometheus_client.Gatherer) (*metric.MeterProvider, error) {
	parsedConfig, err := config.ParseConfig(raw, id)
	if err != nil {
		return nil, err
	}

	cfg, err := parseMetricsExportConfig(parsedConfig.MetricsExport)
	if err != nil {
		return nil, err
	}

	if cfg.Type == "" || gatherer == nil {
		return nil, nil
	}

	certificate, err := tlsutil.LoadCertificate(cfg.TLSCertFile, cfg.TLSCertPrivateKeyFile)
	if err != nil {
		return nil, err
	}

	certPool, err := tlsutil.LoadCertPool(cfg.TLSCACertFile)
	if err != nil {
		return nil, err
	}

	tlsConfig, err := tlsutil.BuildTLSConfig(cfg.EncryptionScheme, *cfg.EncryptionSkipVerify, certificate, certPool)
	if err != nil {
		return nil, err
	}

	var metricExporter metric.Exporter

	if strings.EqualFold(cfg.Type, "otlp/grpc") {
		opts := []otlpmetricgrpc.Option{
			otlpmetricgrpc.WithEndpoint(cfg.Address),
			grpcTLSOption(cfg.EncryptionScheme, tlsConfig),
		}
		metricExporter, err = otlpmetricgrpc.New(ctx, opts...)
	} else {
		opts := []otlpmetrichttp.Option{
			otlpmetrichttp.WithEndpoint(cfg.Address),
			httpTLSOption(cfg.EncryptionScheme, tlsConfig),
		}
		metricExporter, err = otlpmetrichttp.New(ctx, opts...)
	}
	if err != nil {
		return nil, fmt.Errorf("create OTLP metric exporter: %w", err)
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(cfg.ServiceName),
		),
	)
	if err != nil {
		return nil, err
	}

	interval := time.Duration(*cfg.ExportIntervalMs) * time.Millisecond
	producer := otelprometheus.NewMetricProducer(otelprometheus.WithGatherer(gatherer))

	reader := metric.NewPeriodicReader(metricExporter,
		metric.WithInterval(interval),
		metric.WithProducer(producer),
	)

	mp := metric.NewMeterProvider(
		metric.WithResource(res),
		metric.WithReader(reader),
	)

	return mp, nil
}
