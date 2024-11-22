// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package wasm

import (
	"github.com/open-policy-agent/opa/ast"
	v1 "github.com/open-policy-agent/opa/v1/resolver/wasm"
)

// New creates a new Resolver instance which is using the Wasm module
// policy for the given entrypoint ref.
func New(entrypoints []ast.Ref, policy []byte, data interface{}) (*Resolver, error) {
	return v1.New(entrypoints, policy, data)
}

// Resolver implements the resolver.Resolver interface
// using Wasm modules to perform an evaluation.
type Resolver = v1.Resolver
