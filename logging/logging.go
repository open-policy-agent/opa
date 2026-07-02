package logging

import (
	"context"

	v1 "github.com/open-policy-agent/opa/v1/logging"
)

// Level log level for Logger
type Level = v1.Level

const (
	// Error error log level
	Error = v1.Error
	// Warn warn log level
	Warn = v1.Warn
	// Info info log level
	Info = v1.Info
	// Debug debug log level
	Debug = v1.Debug
)

// Logger provides interface for OPA logger implementations
type Logger = v1.Logger

// StandardLogger is the default OPA logger implementation.
type StandardLogger = v1.StandardLogger

// New returns a new standard logger.
func New() *StandardLogger {
	return v1.New()
}

// Get returns the standard logger used throughout OPA.
//
// Deprecated. Do not rely on the global logger.
func Get() *StandardLogger {
	return v1.Get()
}

// NoOpLogger logging implementation that does nothing
type NoOpLogger = v1.NoOpLogger

// NewNoOpLogger instantiates new NoOpLogger
func NewNoOpLogger() *NoOpLogger {
	return v1.NewNoOpLogger()
}

// RequestContext represents the request context used to store data
// related to the request that could be used on logs.
type RequestContext = v1.RequestContext

type HTTPRequestContext = v1.HTTPRequestContext

// NewContext returns a copy of parent with an associated RequestContext.
func NewContext(parent context.Context, val *RequestContext) context.Context {
	return v1.NewContext(parent, val)
}

// FromContext returns the RequestContext associated with ctx, if any.
func FromContext(ctx context.Context) (*RequestContext, bool) {
	return v1.FromContext(ctx)
}

func WithHTTPRequestContext(parent context.Context, val *HTTPRequestContext) context.Context {
	return v1.WithHTTPRequestContext(parent, val)
}

func HTTPRequestContextFromContext(ctx context.Context) (*HTTPRequestContext, bool) {
	return v1.HTTPRequestContextFromContext(ctx)
}

func WithDecisionID(parent context.Context, id string) context.Context {
	return v1.WithDecisionID(parent, id)
}

func DecisionIDFromContext(ctx context.Context) (string, bool) {
	return v1.DecisionIDFromContext(ctx)
}
