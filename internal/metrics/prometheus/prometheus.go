// Copyright 2019 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package prometheus

import (
	"bufio"
	"net"
	"net/http"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// ProviderName is the Prometheus provider name
const ProviderName = "prometheus"

// Provider is the prometheus
type Provider struct {
	registry             *prometheus.Registry
	durationHistogram    *prometheus.HistogramVec
	cancellationCounters *prometheus.CounterVec
}

// NewPrometheusProvider creates new instance of the prometheus provider
func NewPrometheusProvider() *Provider {
	registry := prometheus.NewRegistry()
	registry.MustRegister(prometheus.NewGoCollector())
	durationHistogram := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "http_request_duration_seconds",
			Help: "A histogram of duration for requests.",
		},
		[]string{"code", "handler", "method"},
	)
	registry.MustRegister(durationHistogram)

	cancellationCounters := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_request_cancellations",
			Help: "A count of cancelled requests.",
		},
		[]string{"code", "handler", "method"},
	)

	registry.MustRegister(cancellationCounters)
	return &Provider{
		registry:             registry,
		durationHistogram:    durationHistogram,
		cancellationCounters: cancellationCounters,
	}
}

// RegisterEndpoints registers `/metrics` endpoint
func (p *Provider) RegisterEndpoints(registrar func(path, method string, handler http.Handler)) {
	registrar("/metrics", http.MethodGet, promhttp.HandlerFor(p.registry, promhttp.HandlerOpts{}))
}

// InstrumentHandler returned wrapped HTTP handler with added prometheus instrumentation
func (p *Provider) InstrumentHandler(handler http.Handler, label string) http.Handler {
	durationCollector := p.durationHistogram.MustCurryWith(prometheus.Labels{"handler": label})
	cancellationsCollector := p.cancellationCounters.MustCurryWith(prometheus.Labels{"handler": label})
	return promhttp.InstrumentHandlerDuration(durationCollector, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		csrw := &captureStatusResponseWriter{ResponseWriter: w, status: http.StatusOK}
		var rw http.ResponseWriter
		if h, ok := w.(http.Hijacker); ok {
			rw = &hijacker{ResponseWriter: csrw, hijacker: h}
		} else {
			rw = csrw
		}
		handler.ServeHTTP(rw, r)
		if r.Context().Err() != nil {
			cancellationsCollector.With(prometheus.Labels{"code": strconv.Itoa(csrw.status), "method": r.Method}).Inc()
		}
	}))
}

// Gather collects and returns all registered metrics
func (p *Provider) Gather() (interface{}, error) {
	return p.registry.Gather()
}

// Name returns the provider name
func (p *Provider) Name() string {
	return ProviderName
}

type captureStatusResponseWriter struct {
	http.ResponseWriter
	status int
}

type hijacker struct {
	http.ResponseWriter
	hijacker http.Hijacker
}

func (h *hijacker) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return h.hijacker.Hijack()
}

func (c *captureStatusResponseWriter) WriteHeader(statusCode int) {
	c.ResponseWriter.WriteHeader(statusCode)
	c.status = statusCode
}
