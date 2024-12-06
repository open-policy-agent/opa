// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package metrics contains helpers for performance metric management inside the policy engine.
package metrics

import (
	v1 "github.com/open-policy-agent/opa/v1/metrics"
)

// Well-known metric names.
const (
	BundleRequest       = v1.BundleRequest
	ServerHandler       = v1.ServerHandler
	ServerQueryCacheHit = v1.ServerQueryCacheHit
	SDKDecisionEval     = v1.SDKDecisionEval
	RegoQueryCompile    = v1.RegoQueryCompile
	RegoQueryEval       = v1.RegoQueryEval
	RegoQueryParse      = v1.RegoQueryParse
	RegoModuleParse     = v1.RegoModuleParse
	RegoDataParse       = v1.RegoDataParse
	RegoModuleCompile   = v1.RegoModuleCompile
	RegoPartialEval     = v1.RegoPartialEval
	RegoInputParse      = v1.RegoInputParse
	RegoLoadFiles       = v1.RegoLoadFiles
	RegoLoadBundles     = v1.RegoLoadBundles
	RegoExternalResolve = v1.RegoExternalResolve
)

// Info contains attributes describing the underlying metrics provider.
type Info = v1.Info

// Metrics defines the interface for a collection of performance metrics in the
// policy engine.
type Metrics = v1.Metrics

type TimerMetrics = v1.TimerMetrics

// New returns a new Metrics object.
func New() Metrics {
	return v1.New()
}

// Timer defines the interface for a restartable timer that accumulates elapsed
// time.
type Timer = v1.Timer

// Histogram defines the interface for a histogram with hardcoded percentiles.
type Histogram = v1.Histogram

// Counter defines the interface for a monotonic increasing counter.
type Counter = v1.Counter

func Statistics(num ...int64) interface{} {
	return v1.Statistics(num...)
}
