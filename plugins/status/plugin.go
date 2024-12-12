// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package status implements status reporting.
package status

import (
	"github.com/open-policy-agent/opa/plugins"
	v1 "github.com/open-policy-agent/opa/v1/plugins/status"
)

// Logger defines the interface for status plugins.
type Logger = v1.Logger

// UpdateRequestV1 represents the status update message that OPA sends to
// remote HTTP endpoints.
type UpdateRequestV1 = v1.UpdateRequestV1

// Plugin implements status reporting. Updates can be triggered by the caller.
type Plugin = v1.Plugin

// Config contains configuration for the plugin.
type Config = v1.Config

// BundleLoadDurationNanoseconds represents the configuration for the status.prometheus_config.bundle_loading_duration_ns settings
type BundleLoadDurationNanoseconds = v1.BundleLoadDurationNanoseconds

// ParseConfig validates the config and injects default values.
func ParseConfig(config []byte, services []string, pluginsList []string) (*Config, error) {
	return v1.ParseConfig(config, services, pluginsList)
}

// ConfigBuilder assists in the construction of the plugin configuration.
type ConfigBuilder = v1.ConfigBuilder

// NewConfigBuilder returns a new ConfigBuilder to build and parse the plugin config.
func NewConfigBuilder() *ConfigBuilder {
	return v1.NewConfigBuilder()
}

// New returns a new Plugin with the given config.
func New(parsedConfig *Config, manager *plugins.Manager) *Plugin {
	return v1.New(parsedConfig, manager)
}

// Name identifies the plugin on manager.
const Name = v1.Name

// Lookup returns the status plugin registered with the manager.
func Lookup(manager *plugins.Manager) *Plugin {
	return v1.Lookup(manager)
}
