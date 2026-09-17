// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Deprecated: This package is intended for older projects transitioning from OPA v0.x and will remain for the lifetime of OPA v1.x, but its use is not recommended.
// For newer features and behaviours, such as defaulting to the Rego v1 syntax, use the corresponding components in the [github.com/open-policy-agent/opa/v1] package instead.
// See https://www.openpolicyagent.org/docs/latest/v0-compatibility/ for more information.
package wasm

import (
	"context"

	"github.com/open-policy-agent/opa/ast"
	v1 "github.com/open-policy-agent/opa/v1/resolver/wasm"
)

// New creates a new Resolver instance which is using the Wasm module
// policy for the given entrypoint ref.
func New(entrypoints []ast.Ref, policy []byte, data any) (*Resolver, error) {
	return v1.NewWithContext(context.TODO(), entrypoints, policy, data)
}

// Resolver implements the resolver.Resolver interface
// using Wasm modules to perform an evaluation.
type Resolver = v1.Resolver
