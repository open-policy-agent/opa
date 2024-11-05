// Copyright 2021 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package sdk contains a high-level API for embedding OPA inside of Go programs.
package sdk

import (
	"context"

	"github.com/open-policy-agent/opa/ast"
	v1 "github.com/open-policy-agent/opa/v1/sdk"
)

// OPA represents an instance of the policy engine. OPA can be started with
// several options that control configuration, logging, and lifecycle.
//
// Deprecated: use [v1.OPA] instead.
type OPA = v1.OPA

// New returns a new OPA object. This function should minimally be called with
// options that specify an OPA configuration file.
//
// The returned OPA instance expects modules to use the v0 Rego syntax.
// To use the v1 Rego syntax, set the RegoVersion field in the options,
// or use [github.com/open-policy-agent/opa/v1/sdk.New].
//
// Deprecated: use [github.com/open-policy-agent/opa/v1/sdk.New] instead.
func New(ctx context.Context, opts Options) (*OPA, error) {

	if opts.RegoVersion == ast.RegoUndefined {
		opts.RegoVersion = ast.RegoV0
	}

	return v1.New(ctx, opts)
}

// DecisionOptions contains parameters for query evaluation.
//
// Deprecated: use [github.com/open-policy-agent/opa/v1/sdk.] instead.
type DecisionOptions = v1.DecisionOptions

// DecisionResult contains the output of query evaluation.
//
// Deprecated: use [github.com/open-policy-agent/opa/v1/sdk.DecisionResult] instead.
type DecisionResult = v1.DecisionResult

// PartialQueryMapper
//
// Deprecated: use [github.com/open-policy-agent/opa/v1/sdk.PartialQueryMapper] instead.
type PartialQueryMapper = v1.PartialQueryMapper

// PartialOptions contains parameters for partial query evaluation.
//
// Deprecated: use [github.com/open-policy-agent/opa/v1/sdk.PartialOptions] instead.
type PartialOptions = v1.PartialOptions

// PartialResult
//
// Deprecated: use [github.com/open-policy-agent/opa/v1/sdk.PartialResult] instead.
type PartialResult = v1.PartialResult

// Error represents an internal error in the SDK.
//
// Deprecated: use [github.com/open-policy-agent/opa/v1/sdk.Error] instead.
type Error = v1.Error

const UndefinedErr = v1.UndefinedErr

// IsUndefinedErr returns true of the err represents an undefined decision error.
//
// Deprecated: use [github.com/open-policy-agent/opa/v1/sdk.IsUndefinedErr] instead.
func IsUndefinedErr(err error) bool {
	return v1.IsUndefinedErr(err)
}
