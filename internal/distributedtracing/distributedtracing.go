// Copyright 2021 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package distributedtracing

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/go-logr/logr"
	prometheus_client "github.com/prometheus/client_golang/prometheus"
	otelprometheus "go.opentelemetry.io/contrib/bridges/prometheus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.7.0"
	"google.golang.org/grpc/credentials"

	"github.com/open-policy-agent/opa/v1/config"
	"github.com/open-policy-agent/opa/v1/logging"
	"github.com/open-policy-agent/opa/v1/util"

	// The import registers opentelemetry with the top-level `tracing` package,
	// so the latter can be used from rego/topdown without an explicit build-time
	// dependency.
	_ "github.com/open-policy-agent/opa/v1/features/tracing"
)

const (
	// default gRPC port defined in https://opentelemetry.io/docs/specs/otlp/#otlpgrpc-default-port
	defaultGRPCAddress = "localhost:4317"
	// default HTTP port defined in https://opentelemetry.io/docs/specs/otlp/#otlphttp-default-port
	defaultHTTPAddress = "localhost:4318"

	defaultServiceName          = "opa"
	defaultSampleRatePercentage = float64(100)
	defaultEncyrptionScheme     = "off"
	defaultEncryptionSkipVerify = false

	// the following default values are from the OpenTelemetry specs:
	// https://opentelemetry.io/docs/specs/otel/trace/sdk/#batching-processor
	defaultBatchSpanProcessorBlocking           = false
	defaultBatchSpanProcessorBatchTimeoutMs     = 5000
	defaultBatchSpanProcessorExportTimeoutMs    = 30000
	defaultBatchSpanProcessorMaxExportBatchSize = 512
	defaultBatchSpanProcessorMaxQueueSize       = 2048

	defaultMetricsEnabled          = false
	defaultMetricsExportIntervalMs = 60000 // 60 seconds
)

var supportedEncryptionScheme = map[string]struct{}{
	"off": {}, "tls": {}, "mtls": {},
}

func isSupportedEncryptionScheme(scheme string) bool {
	_, ok := supportedEncryptionScheme[scheme]
	return ok
}

func isSupportedSampleRatePercentage(sampleRate float64) bool {
	return sampleRate >= 0 && sampleRate <= 100
}

type resourceConfig struct {
	ServiceVersion        string `json:"service_version,omitempty"`
	ServiceInstanceID     string `json:"service_instance_id,omitempty"`
	ServiceNamespace      string `json:"service_namespace,omitempty"`
	DeploymentEnvironment string `json:"deployment_environment,omitempty"`
}

type batchSpanProcessorConfig struct {
	Blocking           *bool `json:"blocking,omitempty"`
	BatchTimeoutMs     *int  `json:"batch_timeout_ms,omitempty"`
	ExportTimeoutMs    *int  `json:"export_timeout_ms,omitempty"`
	MaxExportBatchSize *int  `json:"max_export_batch_size,omitempty"`
	MaxQueueSize       *int  `json:"max_queue_size,omitempty"`
}

type distributedTracingConfig struct {
	Type                      string                   `json:"type,omitempty"`
	Address                   string                   `json:"address,omitempty"`
	ServiceName               string                   `json:"service_name,omitempty"`
	SampleRatePercentage      *float64                 `json:"sample_percentage,omitempty"`
	EncryptionScheme          string                   `json:"encryption,omitempty"`
	EncryptionSkipVerify      *bool                    `json:"allow_insecure_tls,omitempty"`
	TLSCertFile               string                   `json:"tls_cert_file,omitempty"`
	TLSCertPrivateKeyFile     string                   `json:"tls_private_key_file,omitempty"`
	TLSCACertFile             string                   `json:"tls_ca_cert_file,omitempty"`
	Resource                  resourceConfig           `json:"resource"`
	BatchSpanProcessorOptions batchSpanProcessorConfig `json:"batch_span_processor_options"`
	Metrics                   *bool                    `json:"metrics,omitempty"`
	MetricsExportIntervalMs   *int                     `json:"metrics_export_interval_ms,omitempty"`
}

func Init(ctx context.Context, raw []byte, id string, gatherer prometheus_client.Gatherer) (*otlptrace.Exporter, *trace.TracerProvider, *resource.Resource, *metric.MeterProvider, error) {
	parsedConfig, err := config.ParseConfig(raw, id)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	distributedTracingConfig, err := parseDistributedTracingConfig(parsedConfig.DistributedTracing)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	if !strings.EqualFold(distributedTracingConfig.Type, "grpc") && !strings.EqualFold(distributedTracingConfig.Type, "http") {
		return nil, nil, nil, nil, nil
	}

	certificate, err := loadCertificate(distributedTracingConfig.TLSCertFile, distributedTracingConfig.TLSCertPrivateKeyFile)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	certPool, err := loadCertPool(distributedTracingConfig.TLSCACertFile)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	tlsConfig, err := buildTLSConfig(distributedTracingConfig.EncryptionScheme, *distributedTracingConfig.EncryptionSkipVerify, certificate, certPool)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	var traceExporter *otlptrace.Exporter
	if strings.EqualFold(distributedTracingConfig.Type, "grpc") {
		traceExporter = otlptracegrpc.NewUnstarted(
			otlptracegrpc.WithEndpoint(distributedTracingConfig.Address),
			grpcTLSOption(distributedTracingConfig.EncryptionScheme, tlsConfig),
		)
	} else if strings.EqualFold(distributedTracingConfig.Type, "http") {
		traceExporter = otlptracehttp.NewUnstarted(
			otlptracehttp.WithEndpoint(distributedTracingConfig.Address),
			httpTLSOption(distributedTracingConfig.EncryptionScheme, tlsConfig),
		)
	}

	var resourceAttributes []attribute.KeyValue
	if distributedTracingConfig.Resource.ServiceVersion != "" {
		resourceAttributes = append(resourceAttributes, semconv.ServiceVersionKey.String(distributedTracingConfig.Resource.ServiceVersion))
	}
	if distributedTracingConfig.Resource.ServiceInstanceID != "" {
		resourceAttributes = append(resourceAttributes, semconv.ServiceInstanceIDKey.String(distributedTracingConfig.Resource.ServiceInstanceID))
	}
	if distributedTracingConfig.Resource.ServiceNamespace != "" {
		resourceAttributes = append(resourceAttributes, semconv.ServiceNamespaceKey.String(distributedTracingConfig.Resource.ServiceNamespace))
	}

	// NOTE: this is currently using the `deployment.environment` setting which is being deprecated
	// in favour of `deployment.environment.name` in future versions of the OpenTelemetry schema.
	// This will need to be taken into account when upgrading the library version in the future.
	if distributedTracingConfig.Resource.DeploymentEnvironment != "" {
		resourceAttributes = append(resourceAttributes, semconv.DeploymentEnvironmentKey.String(distributedTracingConfig.Resource.DeploymentEnvironment))
	}
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(distributedTracingConfig.ServiceName),
		),
		resource.WithAttributes(resourceAttributes...),
	)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	var batchSpanProcessorOptions []trace.BatchSpanProcessorOption
	if distributedTracingConfig.BatchSpanProcessorOptions.Blocking != nil && *distributedTracingConfig.BatchSpanProcessorOptions.Blocking {
		batchSpanProcessorOptions = append(batchSpanProcessorOptions, trace.WithBlocking())
	}
	if distributedTracingConfig.BatchSpanProcessorOptions.BatchTimeoutMs != nil {
		batchSpanProcessorOptions = append(batchSpanProcessorOptions, trace.WithBatchTimeout(time.Duration(*distributedTracingConfig.BatchSpanProcessorOptions.BatchTimeoutMs)*time.Millisecond))
	}
	if distributedTracingConfig.BatchSpanProcessorOptions.ExportTimeoutMs != nil {
		batchSpanProcessorOptions = append(batchSpanProcessorOptions, trace.WithExportTimeout(time.Duration(*distributedTracingConfig.BatchSpanProcessorOptions.ExportTimeoutMs)*time.Millisecond))
	}
	if distributedTracingConfig.BatchSpanProcessorOptions.MaxExportBatchSize != nil {
		batchSpanProcessorOptions = append(batchSpanProcessorOptions, trace.WithMaxExportBatchSize(*distributedTracingConfig.BatchSpanProcessorOptions.MaxExportBatchSize))
	}
	if distributedTracingConfig.BatchSpanProcessorOptions.MaxQueueSize != nil {
		batchSpanProcessorOptions = append(batchSpanProcessorOptions, trace.WithMaxQueueSize(*distributedTracingConfig.BatchSpanProcessorOptions.MaxQueueSize))
	}

	traceProvider := trace.NewTracerProvider(
		trace.WithResource(res),
		trace.WithSampler(trace.ParentBased(trace.TraceIDRatioBased(*distributedTracingConfig.SampleRatePercentage/float64(100)))),
		trace.WithSpanProcessor(trace.NewBatchSpanProcessor(traceExporter, batchSpanProcessorOptions...)),
	)

	var meterProvider *metric.MeterProvider
	if distributedTracingConfig.Metrics != nil && *distributedTracingConfig.Metrics && gatherer != nil {
		meterProvider, err = createMeterProvider(ctx, distributedTracingConfig, tlsConfig, res, gatherer)
		if err != nil {
			return nil, nil, nil, nil, err
		}
	}

	return traceExporter, traceProvider, res, meterProvider, nil
}

func createMeterProvider(ctx context.Context, cfg *distributedTracingConfig, tlsConfig *tls.Config, res *resource.Resource, gatherer prometheus_client.Gatherer) (*metric.MeterProvider, error) {
	var metricExporter metric.Exporter
	var err error

	if strings.EqualFold(cfg.Type, "grpc") {
		opts := []otlpmetricgrpc.Option{
			otlpmetricgrpc.WithEndpoint(cfg.Address),
		}
		if cfg.EncryptionScheme == "off" {
			opts = append(opts, otlpmetricgrpc.WithInsecure())
		} else if tlsConfig != nil {
			opts = append(opts, otlpmetricgrpc.WithTLSCredentials(credentials.NewTLS(tlsConfig)))
		}
		metricExporter, err = otlpmetricgrpc.New(ctx, opts...)
	} else {
		opts := []otlpmetrichttp.Option{
			otlpmetrichttp.WithEndpoint(cfg.Address),
		}
		if cfg.EncryptionScheme == "off" {
			opts = append(opts, otlpmetrichttp.WithInsecure())
		} else if tlsConfig != nil {
			opts = append(opts, otlpmetrichttp.WithTLSClientConfig(tlsConfig))
		}
		metricExporter, err = otlpmetrichttp.New(ctx, opts...)
	}
	if err != nil {
		return nil, fmt.Errorf("create OTLP metric exporter: %w", err)
	}

	interval := time.Duration(*cfg.MetricsExportIntervalMs) * time.Millisecond
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

func SetupLogging(logger logging.Logger) {
	otel.SetErrorHandler(&errorHandler{logger: logger})
	otel.SetLogger(logr.New(&sink{logger: logger}))
}

func parseDistributedTracingConfig(raw []byte) (*distributedTracingConfig, error) {
	if raw == nil {
		encryptionSkipVerify := new(bool)
		sampleRatePercentage := new(float64)
		metricsEnabled := new(bool)
		metricsExportIntervalMs := new(int)
		*sampleRatePercentage = defaultSampleRatePercentage
		*encryptionSkipVerify = defaultEncryptionSkipVerify
		*metricsEnabled = defaultMetricsEnabled
		*metricsExportIntervalMs = defaultMetricsExportIntervalMs
		return &distributedTracingConfig{
			Address:                 defaultGRPCAddress,
			ServiceName:             defaultServiceName,
			SampleRatePercentage:    sampleRatePercentage,
			EncryptionScheme:        defaultEncyrptionScheme,
			EncryptionSkipVerify:    encryptionSkipVerify,
			Metrics:                 metricsEnabled,
			MetricsExportIntervalMs: metricsExportIntervalMs,
		}, nil
	}
	var config distributedTracingConfig

	if err := util.Unmarshal(raw, &config); err != nil {
		return nil, err
	}
	if err := config.validateAndInjectDefaults(); err != nil {
		return nil, err
	}

	return &config, nil
}

func (c *distributedTracingConfig) validateAndInjectDefaults() error {
	switch c.Type {
	case "", "grpc", "http": // OK
	default:
		return fmt.Errorf("unknown distributed_tracing.type '%s', must be \"grpc\", \"http\" or \"\" (unset)", c.Type)
	}

	if c.Address == "" {
		if c.Type == "grpc" {
			c.Address = defaultGRPCAddress
		} else if c.Type == "http" {
			c.Address = defaultHTTPAddress
		}
	}
	if c.ServiceName == "" {
		c.ServiceName = defaultServiceName
	}
	if c.SampleRatePercentage == nil {
		sampleRatePercentage := new(float64)
		*sampleRatePercentage = defaultSampleRatePercentage
		c.SampleRatePercentage = sampleRatePercentage
	}
	if c.EncryptionScheme == "" {
		c.EncryptionScheme = defaultEncyrptionScheme
	}
	if c.EncryptionSkipVerify == nil {
		encryptionSkipVerify := new(bool)
		*encryptionSkipVerify = defaultEncryptionSkipVerify
		c.EncryptionSkipVerify = encryptionSkipVerify
	}

	if c.BatchSpanProcessorOptions.Blocking == nil {
		blocking := new(bool)
		*blocking = defaultBatchSpanProcessorBlocking
		c.BatchSpanProcessorOptions.Blocking = blocking
	}

	if c.BatchSpanProcessorOptions.BatchTimeoutMs == nil {
		batchTimeoutMs := new(int)
		*batchTimeoutMs = defaultBatchSpanProcessorBatchTimeoutMs
		c.BatchSpanProcessorOptions.BatchTimeoutMs = batchTimeoutMs
	}

	if c.BatchSpanProcessorOptions.ExportTimeoutMs == nil {
		exportTimeoutMs := new(int)
		*exportTimeoutMs = defaultBatchSpanProcessorExportTimeoutMs
		c.BatchSpanProcessorOptions.ExportTimeoutMs = exportTimeoutMs
	}

	if c.BatchSpanProcessorOptions.MaxExportBatchSize == nil {
		maxExportBatchSize := new(int)
		*maxExportBatchSize = defaultBatchSpanProcessorMaxExportBatchSize
		c.BatchSpanProcessorOptions.MaxExportBatchSize = maxExportBatchSize
	}

	if c.BatchSpanProcessorOptions.MaxQueueSize == nil {
		maxQueueSize := new(int)
		*maxQueueSize = defaultBatchSpanProcessorMaxQueueSize
		c.BatchSpanProcessorOptions.MaxQueueSize = maxQueueSize
	}

	if c.Metrics == nil {
		metricsEnabled := new(bool)
		*metricsEnabled = defaultMetricsEnabled
		c.Metrics = metricsEnabled
	}
	if c.MetricsExportIntervalMs == nil {
		metricsExportIntervalMs := new(int)
		*metricsExportIntervalMs = defaultMetricsExportIntervalMs
		c.MetricsExportIntervalMs = metricsExportIntervalMs
	}
	if *c.MetricsExportIntervalMs <= 0 {
		return fmt.Errorf("distributed_tracing.metrics_export_interval_ms must be a positive value, got %d", *c.MetricsExportIntervalMs)
	}

	if *c.Metrics && c.Type == "" {
		return errors.New("distributed_tracing.type must be set when distributed_tracing.metrics is enabled")
	}

	if !isSupportedEncryptionScheme(c.EncryptionScheme) {
		return fmt.Errorf("unsupported distributed_tracing.encryption_scheme '%s'", c.EncryptionScheme)
	}

	if !isSupportedSampleRatePercentage(*c.SampleRatePercentage) {
		return fmt.Errorf("unsupported distributed_tracing.sample_percentage '%v'", *c.SampleRatePercentage)
	}

	return nil
}

func loadCertificate(tlsCertFile, tlsPrivateKeyFile string) (*tls.Certificate, error) {

	if tlsCertFile != "" && tlsPrivateKeyFile != "" {
		cert, err := tls.LoadX509KeyPair(tlsCertFile, tlsPrivateKeyFile)
		if err != nil {
			return nil, err
		}
		return &cert, nil
	}

	if tlsCertFile != "" || tlsPrivateKeyFile != "" {
		return nil, errors.New("distributed_tracing.tls_cert_file and distributed_tracing.tls_private_key_file must be specified together")
	}

	return nil, nil
}

func loadCertPool(tlsCACertFile string) (*x509.CertPool, error) {
	if tlsCACertFile == "" {
		return nil, nil
	}

	caCertPEM, err := os.ReadFile(tlsCACertFile)
	if err != nil {
		return nil, fmt.Errorf("read CA cert file: %v", err)
	}
	pool := x509.NewCertPool()
	if ok := pool.AppendCertsFromPEM(caCertPEM); !ok {
		return nil, fmt.Errorf("failed to parse CA cert %q", tlsCACertFile)
	}
	return pool, nil
}

func buildTLSConfig(encryptionScheme string, encryptionSkipVerify bool, cert *tls.Certificate, certPool *x509.CertPool) (*tls.Config, error) {
	if encryptionScheme == "off" {
		return nil, nil
	}
	tlsConfig := &tls.Config{
		RootCAs:            certPool,
		InsecureSkipVerify: encryptionSkipVerify,
	}
	if encryptionScheme == "mtls" {
		if cert == nil {
			return nil, errors.New("distributed_tracing.tls_cert_file required but not supplied")
		}
		tlsConfig.Certificates = []tls.Certificate{*cert}
	}
	return tlsConfig, nil
}

func grpcTLSOption(encryptionScheme string, tlsConfig *tls.Config) otlptracegrpc.Option {
	if encryptionScheme == "off" {
		return otlptracegrpc.WithInsecure()
	}
	return otlptracegrpc.WithTLSCredentials(credentials.NewTLS(tlsConfig))
}

func httpTLSOption(encryptionScheme string, tlsConfig *tls.Config) otlptracehttp.Option {
	if encryptionScheme == "off" {
		return otlptracehttp.WithInsecure()
	}
	return otlptracehttp.WithTLSClientConfig(tlsConfig)
}

type errorHandler struct {
	logger logging.Logger
}

func (e *errorHandler) Handle(err error) {
	e.logger.Warn("Distributed tracing: " + err.Error())
}

// NOTE(sr): This adapter code is used to ensure that whatever otel logs, now or
// in the future, will end up in "our" logs, and not go through whatever defaults
// it has set up with its global logger. As such, it's to a full-featured
// implementation fo the logr.LogSink interface, but a rather minimal one. Notably,
// fields are no supported, the initial runtime time info is ignored, and there is
// no support for different verbosity level is "info" logs: they're all printed
// as-is.

type sink struct {
	logger logging.Logger
}

func (s *sink) Enabled(level int) bool {
	return int(s.logger.GetLevel()) >= level
}

func (*sink) Init(logr.RuntimeInfo) {} // ignored

func (s *sink) Info(_ int, msg string, _ ...any) {
	s.logger.Info(msg)
}

func (s *sink) Error(err error, msg string, _ ...any) {
	s.logger.WithFields(map[string]any{"err": err}).Error(msg)
}

func (s *sink) WithName(name string) logr.LogSink {
	return &sink{s.logger.WithFields(map[string]any{"name": name})}
}

func (s *sink) WithValues(...any) logr.LogSink { // ignored
	return s
}
