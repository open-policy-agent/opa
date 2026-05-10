// Copyright 2021 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package tracing

import (
	"net/http"

	pkg_tracing "github.com/open-policy-agent/opa/v1/tracing"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

func init() {
	pkg_tracing.RegisterHTTPTracing(&factory{})
}

type factory struct{}

func (*factory) NewTransport(tr http.RoundTripper, opts pkg_tracing.Options) http.RoundTripper {
	return otelhttp.NewTransport(tr, withDefaultSpanNameFormatter(opts, clientSpanName)...)
}

func (*factory) NewHandler(f http.Handler, label string, opts pkg_tracing.Options) http.Handler {
	return otelhttp.NewHandler(f, label, withDefaultSpanNameFormatter(opts, serverSpanName)...)
}

// serverSpanName names server-side HTTP spans as "{method} {operation}", where
// `operation` is the route label supplied to NewHandler (for OPA, e.g.
// "v1/data"). This follows the OpenTelemetry HTTP semantic conventions, which
// require the HTTP method as the first token of the span name:
// https://opentelemetry.io/docs/specs/semconv/http/http-spans/#name
func serverSpanName(operation string, r *http.Request) string {
	if r == nil || r.Method == "" {
		return operation
	}
	if operation == "" {
		return r.Method
	}
	return r.Method + " " + operation
}

// clientSpanName names client-side HTTP spans as "{method} {url.path}" where a
// path is available, falling back to "{method}" otherwise. The OpenTelemetry
// HTTP semantic conventions prefer "{method} {target}" over the legacy
// "HTTP {method}" form used by otelhttp's default formatter.
func clientSpanName(_ string, r *http.Request) string {
	if r == nil {
		return "HTTP"
	}
	method := r.Method
	if method == "" {
		method = "HTTP"
	}
	if r.URL != nil && r.URL.Path != "" {
		return method + " " + r.URL.Path
	}
	return method
}

// withDefaultSpanNameFormatter prepends the provided span name formatter to the
// converted otelhttp options. Because otelhttp applies options in order with
// last-write-wins semantics, any user-supplied WithSpanNameFormatter passed
// through pkg_tracing.Options still takes precedence.
func withDefaultSpanNameFormatter(opts pkg_tracing.Options, fn func(string, *http.Request) string) []otelhttp.Option {
	converted := convertOpts(opts)
	out := make([]otelhttp.Option, 0, len(converted)+1)
	out = append(out, otelhttp.WithSpanNameFormatter(fn))
	out = append(out, converted...)
	return out
}

func convertOpts(opts pkg_tracing.Options) []otelhttp.Option {
	otelOpts := make([]otelhttp.Option, 0, len(opts))
	for _, opt := range opts {
		otelOpts = append(otelOpts, opt.(otelhttp.Option))
	}
	return otelOpts
}
