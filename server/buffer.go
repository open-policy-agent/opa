// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package server

import (
	"time"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/metrics"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/topdown"
)

// Info contains information describing a policy decision.
type Info struct {
	Txn        storage.Transaction
	Revision   string // Deprecated: Use `Bundles` instead
	Bundles    map[string]BundleInfo
	DecisionID string
	RemoteAddr string
	Query      string
	Path       string
	Timestamp  time.Time
	Input      *interface{}
	InputAST   ast.Value
	Results    *interface{}
	Error      error
	Metrics    metrics.Metrics
	Trace      []*topdown.Event
}

// BundleInfo contains information describing a bundle.
type BundleInfo struct {
	Revision string
}
