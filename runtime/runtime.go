// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package runtime contains the entry point to the policy engine.
//
// Deprecated: This package is intended for older projects transitioning from OPA v0.x and will remain for the lifetime of OPA v1.x, but its use is not recommended.
// For newer features and behaviours, such as defaulting to the Rego v1 syntax, use the corresponding components in the [github.com/open-policy-agent/opa/v1] package instead.
// See https://www.openpolicyagent.org/docs/latest/v0-compatibility/ for more information.
package runtime

import (
	"context"
	"net/http"

	"github.com/open-policy-agent/opa/logging"
	"github.com/open-policy-agent/opa/plugins"
	v1 "github.com/open-policy-agent/opa/v1/runtime"
)

// LoggingHandler returns an http.Handler that will print log messages
// containing the request information as well as response status and latency.
type LoggingHandler = v1.LoggingHandler

// NewLoggingHandler returns a new http.Handler.
func NewLoggingHandler(logger logging.Logger, inner http.Handler) http.Handler {
	return v1.NewLoggingHandler(logger, inner)
}

// RegisterPlugin registers a plugin factory with the runtime
// package. When the runtime is created, the factories are used to parse
// plugin configuration and instantiate plugins. If no configuration is
// provided, plugins are not instantiated. This function is idempotent.
func RegisterPlugin(name string, factory plugins.Factory) {
	v1.RegisterPlugin(name, factory)
}

// Params stores the configuration for an OPA instance.
type Params = v1.Params

// LoggingConfig stores the configuration for OPA's logging behaviour.
type LoggingConfig = v1.LoggingConfig

// NewParams returns a new Params object.
func NewParams() Params {
	return v1.NewParams()
}

// Runtime represents a single OPA instance.
type Runtime = v1.Runtime

// NewRuntime returns a new Runtime object initialized with params. Clients must
// call StartServer() or StartREPL() to start the runtime in either mode.
func NewRuntime(ctx context.Context, params Params) (*Runtime, error) {
	return v1.NewRuntime(ctx, params)
}
