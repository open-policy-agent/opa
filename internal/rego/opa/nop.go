// Copyright 2021 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// +build !opa_wasm

package opa

import (
	"context"
	"fmt"
	"os"
)

// OPA is a stub implementation of a opa.OPA.
type OPA struct {
}

// New unimplemented.
func New() *OPA {
	fmt.Fprintf(os.Stderr, `WebAssembly runtime not supported in this build.
----------------------------------------------------------------------------------
Please download an OPA binay with Wasm enabled from
  https://www.openpolicyagent.org/docs/latest/#running-opa
or build it yourself (with Wasm enabled).
----------------------------------------------------------------------------------
`)
	os.Exit(1)
	return nil
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
