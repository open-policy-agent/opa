// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package discovery

import (
	v1 "github.com/open-policy-agent/opa/v1/plugins/discovery"
)

// Config represents the configuration for the discovery feature.
type Config = v1.Config

// ConfigBuilder assists in the construction of the plugin configuration.
type ConfigBuilder = v1.ConfigBuilder

// NewConfigBuilder returns a new ConfigBuilder to build and parse the discovery config
func NewConfigBuilder() *ConfigBuilder {
	return v1.NewConfigBuilder()
}

// ParseConfig returns a valid Config object with defaults injected.
func ParseConfig(bs []byte, services []string) (*Config, error) {
	return v1.ParseConfig(bs, services)
}
