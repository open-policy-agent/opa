// Copyright 2021 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package tracing

import (
	"net/http"

	pkg_tracing "github.com/open-policy-agent/opa/tracing"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

func init() {
	pkg_tracing.RegisterHTTPTracing(&factory{})
}

type factory struct{}

func (*factory) NewTransport(tr http.RoundTripper, opts pkg_tracing.Options) http.RoundTripper {
	return otelhttp.NewTransport(tr, convertOpts(opts)...)
}

func (*factory) NewHandler(f http.Handler, label string, opts pkg_tracing.Options) http.Handler {
	return otelhttp.NewHandler(f, label, convertOpts(opts)...)
}

func convertOpts(opts pkg_tracing.Options) []otelhttp.Option {
	otelOpts := make([]otelhttp.Option, 0, len(opts))
	for _, opt := range opts {
		otelOpts = append(otelOpts, opt.(otelhttp.Option))
	}
	return otelOpts
}
