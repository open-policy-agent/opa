// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package wasm

import (
	"context"
	"fmt"

	"github.com/open-policy-agent/opa/internal/wasm/sdk/opa"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/resolver"
)

// New creates a new Resolver instance which is using the Wasm module
// policy for the given entrypoint ref.
func New(entrypoint ast.Ref, policy []byte, data interface{}) (*Resolver, error) {
	o, err := opa.New().
		WithPolicyBytes(policy).
		WithDataJSON(data).
		Init()
	if err != nil {
		return nil, err
	}
	return &Resolver{
		Entrypoint: entrypoint,
		o:          o,
	}, nil
}

// Resolver implements the resolver.Resolver interface
// using Wasm modules to perform an evaluation.
type Resolver struct {
	Entrypoint ast.Ref
	o          *opa.OPA
}

// Close shuts down the resolver.
func (r *Resolver) Close() {
	r.o.Close()
}

// Eval performs an evalution using the provided input and the Wasm module
// associated with this Resolver instance.
func (r *Resolver) Eval(ctx context.Context, input resolver.Input) (resolver.Result, error) {

	var inp *interface{}

	if input.Input != nil {
		x, err := ast.JSON(input.Input.Value)
		if err != nil {
			return resolver.Result{}, err
		}
		inp = &x
	}

	out, err := r.o.Eval(ctx, 0, inp)
	if err != nil {
		return resolver.Result{}, err
	}

	result, err := getResult(out)
	if err != nil {
		return resolver.Result{}, err
	} else if result == nil {
		return resolver.Result{}, nil
	}

	v, err := ast.InterfaceToValue(*result)
	if err != nil {
		return resolver.Result{}, err
	}

	return resolver.Result{Value: v}, nil
}

// SetData will update the external data for the Wasm instance.
func (r *Resolver) SetData(data interface{}) error {
	return r.o.SetData(data)
}

func getResult(rs *opa.Result) (*interface{}, error) {

	r, ok := rs.Result.([]interface{})
	if !ok {
		return nil, fmt.Errorf("illegal result set type")
	}

	if len(r) == 0 {
		return nil, nil
	}

	m, ok := r[0].(map[string]interface{})
	if !ok || len(m) != 1 {
		return nil, fmt.Errorf("illegal result type")
	}

	result, ok := m["result"]
	if !ok {
		return nil, fmt.Errorf("missing value")
	}

	return &result, nil
}
