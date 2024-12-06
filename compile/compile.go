// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package compile implements bundles compilation and linking.
package compile

import (
	"github.com/open-policy-agent/opa/ast"
	v1 "github.com/open-policy-agent/opa/v1/compile"
)

const (
	// TargetRego is the default target. The source rego is copied (potentially
	// rewritten for optimization purpsoes) into the bundle. The target supports
	// base documents.
	TargetRego = v1.TargetRego

	// TargetWasm is an alternative target that compiles the policy into a wasm
	// module instead of Rego. The target supports base documents.
	TargetWasm = v1.TargetWasm

	// TargetPlan is an altertive target that compiles the policy into an
	// imperative query plan that can be further transpiled or interpreted.
	TargetPlan = v1.TargetPlan
)

// Targets contains the list of targets supported by the compiler.
var Targets = v1.Targets

// Compiler implements bundle compilation and linking.
type Compiler = v1.Compiler

// New returns a new compiler instance that can be invoked.
func New() *Compiler {
	return v1.New().WithRegoVersion(ast.DefaultRegoVersion)
}
