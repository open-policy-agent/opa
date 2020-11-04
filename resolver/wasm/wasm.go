// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// +build cgo

package wasm

import (
	"context"
	"fmt"
	"strconv"

	"github.com/open-policy-agent/opa/internal/wasm/sdk/opa"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/resolver"
)

// New creates a new Resolver instance which is using the Wasm module
// policy for the given entrypoint ref.
func New(entrypoints []ast.Ref, policy []byte, data interface{}) (*Resolver, error) {
	o, err := opa.New().
		WithPolicyBytes(policy).
		WithDataJSON(data).
		Init()
	if err != nil {
		return nil, err
	}

	// Construct a quick lookup table of ref -> entrypoint ID
	// for handling evaluations. Only the entrypoints provided
	// by the caller will be constructed, this may be a subset
	// of entrypoints available in the Wasm module, however
	// only the configured ones will be used when Eval() is
	// called.
	entrypointRefToID := ast.NewValueMap()
	epIDs, err := o.Entrypoints(context.Background())
	if err != nil {
		return nil, err
	}
	for path, id := range epIDs {
		for _, ref := range entrypoints {
			refPtr, err := ref.Ptr()
			if err != nil {
				return nil, err
			}
			if refPtr == path {
				entrypointRefToID.Put(ref, ast.Number(strconv.Itoa(int(id))))
			}
		}
	}

	return &Resolver{
		entrypoints:   entrypoints,
		entrypointIDs: entrypointRefToID,
		o:             o,
	}, nil
}

// Resolver implements the resolver.Resolver interface
// using Wasm modules to perform an evaluation.
type Resolver struct {
	entrypoints   []ast.Ref
	entrypointIDs *ast.ValueMap
	o             *opa.OPA
}

// Entrypoints returns a list of entrypoints this resolver is configured to
// perform evaluations on.
func (r *Resolver) Entrypoints() []ast.Ref {
	return r.entrypoints
}

// Close shuts down the resolver.
func (r *Resolver) Close() {
	r.o.Close()
}

// Eval performs an evaluation using the provided input and the Wasm module
// associated with this Resolver instance.
func (r *Resolver) Eval(ctx context.Context, input resolver.Input) (resolver.Result, error) {
	input.Metrics.Timer("wasm_resolver_eval").Start()
	defer input.Metrics.Timer("wasm_resolver_eval").Stop()

	var inp *interface{}

	if input.Input != nil {
		x, err := ast.JSON(input.Input.Value)
		if err != nil {
			return resolver.Result{}, err
		}
		inp = &x
	}

	v := r.entrypointIDs.Get(input.Ref)
	if v == nil {
		return resolver.Result{}, fmt.Errorf("unknown entrypoint %s", input.Ref)
	}

	numValue, ok := v.(ast.Number)
	if !ok {
		return resolver.Result{}, fmt.Errorf("internal error: invalid entrypoint id %s", numValue)
	}

	epID, ok := numValue.Int()
	if !ok {
		return resolver.Result{}, fmt.Errorf("internal error: invalid entrypoint id %s", numValue)
	}

	opts := opa.EvalOpts{
		Input:      inp,
		Entrypoint: opa.EntrypointID(epID),
		Metrics:    input.Metrics,
	}
	out, err := r.o.Eval(ctx, opts)
	if err != nil {
		return resolver.Result{}, err
	}

	result, err := getResult(out)
	if err != nil {
		return resolver.Result{}, err
	} else if result == nil {
		return resolver.Result{}, nil
	}

	v, err = ast.InterfaceToValue(*result)
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
