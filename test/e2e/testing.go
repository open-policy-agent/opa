// Copyright 2019 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package e2e

import (
	"context"
	"testing"

	"github.com/open-policy-agent/opa/v1/runtime"
	v1 "github.com/open-policy-agent/opa/v1/test/e2e"
)

// NewAPIServerTestParams creates a new set of runtime.Params with enough
// default values filled in to start the server. Options can/should
// be customized for the test case.
func NewAPIServerTestParams() runtime.Params {
	return v1.NewAPIServerTestParams()
}

// TestRuntime holds metadata and provides helper methods
// to interact with the runtime being tested.
type TestRuntime = v1.TestRuntime

// NewTestRuntime returns a new TestRuntime.
func NewTestRuntime(params runtime.Params) (*TestRuntime, error) {
	return v1.NewTestRuntime(params)
}

// NewTestRuntimeWithOpts returns a new TestRuntime.
func NewTestRuntimeWithOpts(opts TestRuntimeOpts, params runtime.Params) (*TestRuntime, error) {
	return v1.NewTestRuntimeWithOpts(opts, params)
}

// WrapRuntime creates a new TestRuntime by wrapping an existing runtime
func WrapRuntime(ctx context.Context, cancel context.CancelFunc, rt *runtime.Runtime) *TestRuntime {
	return v1.WrapRuntime(ctx, cancel, rt)
}

// TestRuntimeOpts contains parameters for the test runtime.
type TestRuntimeOpts = v1.TestRuntimeOpts

// WithRuntime invokes f with a new TestRuntime after waiting for server
// readiness. This function can be called inside of each test that requires a
// runtime as opposed to RunTests which can only be called once.
func WithRuntime(t *testing.T, opts TestRuntimeOpts, params runtime.Params, f func(rt *TestRuntime)) {
	v1.WithRuntime(t, opts, params, f)
}
