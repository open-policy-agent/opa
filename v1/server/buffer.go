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
	Timestamp           time.Time
	Txn                 storage.Transaction
	InputAST            ast.Value
	Metrics             metrics.Metrics
	Error               error
	NDBuiltinCache      *any
	Bundles             map[string]BundleInfo
	MappedResults       *any
	HTTPRequestContext  logging.HTTPRequestContext
	IntermediateResults map[string]any
	Results             *any
	Custom              map[string]any
	Input               *any
	TraceID             string
	Path                string
	Query               string
	RemoteAddr          string
	SpanID              string
	BatchDecisionID     string
	DecisionID          string
	Revision            string
	Trace               []*topdown.Event
	RequestID           uint64
}

// BundleInfo contains information describing a bundle.
type BundleInfo struct {
	Revision string
}
