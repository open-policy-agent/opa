// Copyright 2021 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package tracing

import (
	"net/http"

	pkg_tracing "github.com/open-policy-agent/opa/v1/tracing"
	"github.com/open-policy-agent/opa/v1/util"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

func init() {
	pkg_tracing.RegisterHTTPTracing(&factory{})
}

type factory struct{}

func (*factory) NewTransport(tr http.RoundTripper, opts pkg_tracing.Options) http.RoundTripper {
	return otelhttp.NewTransport(tr, util.ToSliceOf[otelhttp.Option](opts)...)
}

func (*factory) NewHandler(f http.Handler, label string, opts pkg_tracing.Options) http.Handler {
	return otelhttp.NewHandler(f, label, util.ToSliceOf[otelhttp.Option](opts)...)
}
