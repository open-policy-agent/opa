// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"github.com/open-policy-agent/opa/v1/metrics"
	v1 "github.com/open-policy-agent/opa/v1/topdown"
)

// Instrumentation implements helper functions to instrument query evaluation
// to diagnose performance issues. Instrumentation may be expensive in some
// cases, so it is disabled by default.
type Instrumentation = v1.Instrumentation

// NewInstrumentation returns a new Instrumentation object. Performance
// diagnostics recorded on this Instrumentation object will stored in m.
func NewInstrumentation(m metrics.Metrics) *Instrumentation {
	return v1.NewInstrumentation(m)
}
