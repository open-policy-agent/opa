// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package runtime

import (
	"context"

	"github.com/open-policy-agent/opa/plugins"
	v1 "github.com/open-policy-agent/opa/v1/runtime"
)

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
