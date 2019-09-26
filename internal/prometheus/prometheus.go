// Copyright 2019 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package prometheus

import (
	"bufio"
	"encoding/json"
	"net"
	"net/http"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/open-policy-agent/opa/metrics"
)

// Provider wraps a metrics.Metrics provider with a Prometheus registry that can
// instrument the HTTP server's handlers.
type Provider struct {
	registry             *prometheus.Registry
	durationHistogram    *prometheus.HistogramVec
	cancellationCounters *prometheus.CounterVec
	inner                metrics.Metrics
	logger               func(attrs map[string]interface{}, f string, a ...interface{})
}

// New returns a new Provider object.
func New(inner metrics.Metrics, logger func(attrs map[string]interface{}, f string, a ...interface{})) *Provider {
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
		inner:                inner,
		logger:               logger,
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

// Info returns attributes that describe the metric provider.
func (p *Provider) Info() metrics.Info {
	return metrics.Info{
		Name: "prometheus",
	}
}

// All returns the union of the inner metric provider and the underlying
// prometheus registry.
func (p *Provider) All() map[string]interface{} {

	all := p.inner.All()

	families, err := p.registry.Gather()
	if err != nil && p.logger != nil {
		p.logger(map[string]interface{}{
			"err": err,
		}, "Failed to gather metrics from Prometheus registry.")
	}

	for _, f := range families {
		all[f.GetName()] = f
	}

	return all
}

// MarshalJSON returns a JSON representation of the unioned metrics.
func (p *Provider) MarshalJSON() ([]byte, error) {
	return json.Marshal(p.All())
}

// Timer returns a named timer.
func (p *Provider) Timer(name string) metrics.Timer {
	return p.inner.Timer(name)
}

// Counter returns a named counter.
func (p *Provider) Counter(name string) metrics.Counter {
	return p.inner.Counter(name)
}

// Histogram returns a named histogram.
func (p *Provider) Histogram(name string) metrics.Histogram {
	return p.inner.Histogram(name)
}

// Clear resets the inner metric provider. The Prometheus registry does not
// expose an interface to clear the metrics so this call has no affect on
// metrics tracked by Prometheus.
func (p *Provider) Clear() {
	p.inner.Clear()
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
