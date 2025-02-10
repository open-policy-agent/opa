// Copyright 2025 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package tester

import "github.com/open-policy-agent/opa/v1/topdown"

type TestQueryTracer struct {
	topdown.BufferTracer
}

func NewTestQueryTracer() *TestQueryTracer {
	return &TestQueryTracer{}
}

func (t *TestQueryTracer) TraceEvent(e topdown.Event) {
	if e.Op == topdown.TestCaseOp {
		t.BufferTracer.TraceEvent(e)
	}
}

func (t *TestQueryTracer) Events() []*topdown.Event {
	if t == nil {
		return nil
	}
	return t.BufferTracer
}
