// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package config implements OPA configuration file parsing and validation.
//
// Deprecated: This package is intended for older projects transitioning from OPA v0.x and will remain for the lifetime of OPA v1.x, but its use is not recommended.
// For newer features and behaviours, such as defaulting to the Rego v1 syntax, use the corresponding components in the [github.com/open-policy-agent/opa/v1] package instead.
// See https://www.openpolicyagent.org/docs/latest/v0-compatibility/ for more information.
package config

import (
	v1 "github.com/open-policy-agent/opa/v1/config"
)

// Config represents the configuration file that OPA can be started with.
type Config = v1.Config

// ParseConfig returns a valid Config object with defaults injected. The id
// and version parameters will be set in the labels map.
func ParseConfig(raw []byte, id string) (*Config, error) {
	return v1.ParseConfig(raw, id)
}
