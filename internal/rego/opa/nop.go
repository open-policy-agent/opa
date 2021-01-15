// Copyright 2021 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// +build !opa_wasm

package opa

import (
	"context"

	"github.com/open-policy-agent/opa/metrics"
)

// OPA is a stub implementation of a opa.OPA.
type OPA struct {
}

// Result is a stub implementation of a opa.Result.
type Result struct {
	Result []byte
}

// EvalOpts is a stub implementation of a opa.EvalOpts.
type EvalOpts struct {
	Input   *interface{}
	Metrics metrics.Metrics
}

// New unimplemented.
func New() *OPA {
	panic("WebAssembly runtime not supported in this build")
}

// WithPolicyBytes unimplemented.
func (o *OPA) WithPolicyBytes(policy []byte) *OPA {
	panic("unreachable")
}

// WithDataJSON unimplemented.
func (o *OPA) WithDataJSON(data interface{}) *OPA {
	panic("unreachable")
}

// Init unimplemented.
func (o *OPA) Init() (*OPA, error) {
	panic("unreachable")
}

// Eval unimplemented.
func (o *OPA) Eval(ctx context.Context, opts EvalOpts) (*Result, error) {
	panic("unreachable")
}
