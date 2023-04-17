// Copyright 2021 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package distributedtracing

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"strings"

	"github.com/go-logr/logr"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.7.0"
	"google.golang.org/grpc/credentials"

	"github.com/open-policy-agent/opa/config"
	"github.com/open-policy-agent/opa/logging"
	"github.com/open-policy-agent/opa/util"

	// The import registers opentelemetry with the top-level `tracing` package,
	// so the latter can be used from rego/topdown without an explicit build-time
	// dependency.
	_ "github.com/open-policy-agent/opa/features/tracing"
)

const (
	defaultAddress              = "localhost:4317"
	defaultServiceName          = "opa"
	defaultSampleRatePercentage = int(100)
	defaultEncyrptionScheme     = "off"
	defaultEncryptionSkipVerify = false
)

var supportedEncryptionScheme = map[string]struct{}{
	"off": {}, "tls": {}, "mtls": {},
}

func isSupportedEncryptionScheme(scheme string) bool {
	_, ok := supportedEncryptionScheme[scheme]
	return ok
}

func isSupportedSampleRatePercentage(sampleRate int) bool {
	return sampleRate >= 0 && sampleRate <= 100
}

type distributedTracingConfig struct {
	Type                  string `json:"type,omitempty"`
	Address               string `json:"address,omitempty"`
	ServiceName           string `json:"service_name,omitempty"`
	SampleRatePercentage  *int   `json:"sample_percentage,omitempty"`
	EncryptionScheme      string `json:"encryption,omitempty"`
	EncryptionSkipVerify  *bool  `json:"allow_insecure_tls,omitempty"`
	TLSCertFile           string `json:"tls_cert_file,omitempty"`
	TLSCertPrivateKeyFile string `json:"tls_private_key_file,omitempty"`
	TLSCACertFile         string `json:"tls_ca_cert_file,omitempty"`
}

func Init(ctx context.Context, raw []byte, id string) (*otlptrace.Exporter, *trace.TracerProvider, error) {
	parsedConfig, err := config.ParseConfig(raw, id)
	if err != nil {
		return nil, nil, err
	}

	distributedTracingConfig, err := parseDistributedTracingConfig(parsedConfig.DistributedTracing)
	if err != nil {
		return nil, nil, err
	}

	if strings.ToLower(distributedTracingConfig.Type) != "grpc" {
		return nil, nil, nil
	}

	certificate, err := loadCertificate(distributedTracingConfig.TLSCertFile, distributedTracingConfig.TLSCertPrivateKeyFile)
	if err != nil {
		return nil, nil, err
	}

	certPool, err := loadCertPool(distributedTracingConfig.TLSCACertFile)
	if err != nil {
		return nil, nil, err
	}

	tlsOption, err := tlsOption(distributedTracingConfig.EncryptionScheme, *distributedTracingConfig.EncryptionSkipVerify, certificate, certPool)
	if err != nil {
		return nil, nil, err
	}

	traceExporter := otlptracegrpc.NewUnstarted(
		otlptracegrpc.WithEndpoint(distributedTracingConfig.Address),
		tlsOption,
	)

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(distributedTracingConfig.ServiceName),
		),
	)
	if err != nil {
		return nil, nil, err
	}

	traceProvider := trace.NewTracerProvider(
		trace.WithResource(res),
		trace.WithSampler(trace.ParentBased(trace.TraceIDRatioBased(float64(*distributedTracingConfig.SampleRatePercentage)/float64(100)))),
		trace.WithSpanProcessor(trace.NewBatchSpanProcessor(traceExporter)),
	)

	return traceExporter, traceProvider, nil
}

func SetupLogging(logger logging.Logger) {
	otel.SetErrorHandler(&errorHandler{logger: logger})
	otel.SetLogger(logr.New(&sink{logger: logger}))
}

func parseDistributedTracingConfig(raw []byte) (*distributedTracingConfig, error) {
	if raw == nil {
		encryptionSkipVerify := new(bool)
		sampleRatePercentage := new(int)
		*sampleRatePercentage = defaultSampleRatePercentage
		*encryptionSkipVerify = defaultEncryptionSkipVerify
		return &distributedTracingConfig{
			Address:              defaultAddress,
			ServiceName:          defaultServiceName,
			SampleRatePercentage: sampleRatePercentage,
			EncryptionScheme:     defaultEncyrptionScheme,
			EncryptionSkipVerify: encryptionSkipVerify,
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
	case "", "grpc": // OK
	default:
		return fmt.Errorf("unknown distributed_tracing.type '%s', must be \"grpc\" or \"\" (unset)", c.Type)
	}

	if c.Address == "" {
		c.Address = defaultAddress
	}
	if c.ServiceName == "" {
		c.ServiceName = defaultServiceName
	}
	if c.SampleRatePercentage == nil {
		sampleRatePercentage := new(int)
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
		return nil, fmt.Errorf("distributed_tracing.tls_cert_file and distributed_tracing.tls_private_key_file must be specified together")
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

func tlsOption(encryptionScheme string, encryptionSkipVerify bool, cert *tls.Certificate, certPool *x509.CertPool) (otlptracegrpc.Option, error) {
	if encryptionScheme == "off" {
		return otlptracegrpc.WithInsecure(), nil
	}
	tlsConfig := &tls.Config{
		RootCAs:            certPool,
		InsecureSkipVerify: encryptionSkipVerify,
	}
	if encryptionScheme == "mtls" {
		if cert == nil {
			return nil, fmt.Errorf("distributed_tracing.tls_cert_file required but not supplied")
		}
		tlsConfig.Certificates = []tls.Certificate{*cert}
	}
	return otlptracegrpc.WithTLSCredentials(credentials.NewTLS(tlsConfig)), nil
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

func (s *sink) Info(_ int, msg string, _ ...interface{}) {
	s.logger.Info(msg)
}

func (s *sink) Error(err error, msg string, _ ...interface{}) {
	s.logger.WithFields(map[string]interface{}{"err": err}).Error(msg)
}

func (s *sink) WithName(name string) logr.LogSink {
	return &sink{s.logger.WithFields(map[string]interface{}{"name": name})}
}

func (s *sink) WithValues(...interface{}) logr.LogSink { // ignored
	return s
}
