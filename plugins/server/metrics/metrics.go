// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Deprecated: This package is intended for older projects transitioning from OPA v0.x and will remain for the lifetime of OPA v1.x, but its use is not recommended.
// For newer features and behaviours, such as defaulting to the Rego v1 syntax, use the corresponding components in the [github.com/open-policy-agent/opa/v1] package instead.
// See https://www.openpolicyagent.org/docs/latest/v0-compatibility/ for more information.
package metrics

import (
	v1 "github.com/open-policy-agent/opa/v1/plugins/server/metrics"
)

// Config represents the configuration for the Server.Metrics settings
type Config = v1.Config

// Prom represents the configuration for the Server.Metrics.Prom settings
type Prom = v1.Prom

// HTTPRequestDurationSeconds represents the configuration for the Server.Metrics.Prom.HTTPRequestDurationSeconds settings
type HTTPRequestDurationSeconds = v1.HTTPRequestDurationSeconds

// ConfigBuilder assists in the construction of the plugin configuration.
type ConfigBuilder = v1.ConfigBuilder

// NewConfigBuilder returns a new ConfigBuilder to build and parse the server config
func NewConfigBuilder() *ConfigBuilder {
	return v1.NewConfigBuilder()
}
