// Copyright 2021 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// +build opa_wasm

package opa

import (
	"context"
	wopa "github.com/open-policy-agent/opa/internal/wasm/sdk/opa"
	"github.com/open-policy-agent/opa/metrics"
)

// OPA is an implementation of the OPA SDK.
type OPA struct {
	opa *wopa.OPA
}

// Result holds the evaluation result.
type Result struct {
	Result []byte
}

// EvalOpts define options for performing an evaluation.
type EvalOpts struct {
	Input   *interface{}
	Metrics metrics.Metrics
}

// New constructs a new OPA instance.
func New() *OPA {
	return &OPA{opa: wopa.New()}
}

// WithPolicyBytes configures the compiled policy to load.
func (o *OPA) WithPolicyBytes(policy []byte) *OPA {
	o.opa = o.opa.WithPolicyBytes(policy)
	return o
}

// WithDataJSON configures the JSON data to load.
func (o *OPA) WithDataJSON(data interface{}) *OPA {
	o.opa = o.opa.WithDataJSON(data)
	return o
}

// Init initializes the OPA instance.
func (o *OPA) Init() (*OPA, error) {
	i, err := o.opa.Init()
	if err != nil {
		return nil, err
	}
	o.opa = i
	return o, nil
}

// Eval evaluates the policy.
func (o *OPA) Eval(ctx context.Context, opts EvalOpts) (*Result, error) {
	evalOptions := wopa.EvalOpts{
		Input:   opts.Input,
		Metrics: opts.Metrics,
	}

	res, err := o.opa.Eval(ctx, evalOptions)
	if err != nil {
		return nil, err
	}

	return &Result{Result: res.Result}, nil
}
