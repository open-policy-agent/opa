// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package server

import (
	"time"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/logging"
	"github.com/open-policy-agent/opa/v1/metrics"
	"github.com/open-policy-agent/opa/v1/storage"
	"github.com/open-policy-agent/opa/v1/topdown"
)

// Info contains information describing a policy decision.
type Info struct {
	Txn                 storage.Transaction
	Revision            string // Deprecated: Use `Bundles` instead
	Bundles             map[string]BundleInfo
	DecisionID          string
	BatchDecisionID     string
	TraceID             string
	SpanID              string
	RemoteAddr          string
	HTTPRequestContext  logging.HTTPRequestContext
	Query               string
	Path                string
	Timestamp           time.Time
	Input               *any
	InputAST            ast.Value
	Results             *any
	IntermediateResults map[string]any
	MappedResults       *any
	NDBuiltinCache      *any
	Error               error
	Metrics             metrics.Metrics
	Trace               []*topdown.Event
	RequestID           uint64
	Custom              map[string]any
}

// BundleInfo contains information describing a bundle.
type BundleInfo struct {
	Revision string
}
