// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// +build !cgo

package wasm

import (
	"context"
	"errors"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/resolver"
)

type Resolver struct {
}

func (r *Resolver) Entrypoints() []ast.Ref {
	panic("unreachable")
}

func (r *Resolver) Close() {
	panic("unreachable")
}

func (r *Resolver) Eval(ctx context.Context, input resolver.Input) (resolver.Result, error) {

	panic("unreachable")
}

func (r *Resolver) SetData(data interface{}) error {
	panic("unreachable")
}

func New(entrypoints []ast.Ref, policy []byte, data interface{}) (*Resolver, error) {
	return nil, errors.New("WebAssembly runtime not supported in this build")
}
