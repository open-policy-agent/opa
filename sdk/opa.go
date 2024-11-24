// Copyright 2024 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package sdk

import (
	"context"

	"github.com/open-policy-agent/opa/ast"
	v1 "github.com/open-policy-agent/opa/v1/sdk"
)

type OPA = v1.OPA

type Options = v1.Options

type DecisionOptions = v1.DecisionOptions

type DecisionResult = v1.DecisionResult

type PartialQueryMapper = v1.PartialQueryMapper

type PartialOptions = v1.PartialOptions

type PartialResult = v1.PartialResult

type Error = v1.Error

type RawMapper = v1.RawMapper

func New(ctx context.Context, opts Options) (*OPA, error) {
	if opts.RegoVersion == ast.RegoUndefined {
		opts.RegoVersion = ast.DefaultRegoVersion
	}
	return v1.New(ctx, opts)
}

func IsUndefinedErr(err error) bool {
	return v1.IsUndefinedErr(err)
}
