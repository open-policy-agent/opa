// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package bundle

import (
	v1 "github.com/open-policy-agent/opa/v1/plugins/bundle"
)

// ParseConfig validates the config and injects default values. This is
// for the legacy single bundle configuration. This will add the bundle
// to the `Bundles` map to provide compatibility with newer clients.
// Deprecated: Use `ParseBundlesConfig` with `bundles` OPA config option instead
func ParseConfig(config []byte, services []string) (*Config, error) {
	return v1.ParseConfig(config, services)
}

// ParseBundlesConfig validates the config and injects default values for
// the defined `bundles`. This expects a map of bundle names to resource
// configurations.
func ParseBundlesConfig(config []byte, services []string) (*Config, error) {
	return v1.ParseBundlesConfig(config, services)
}

// NewConfigBuilder returns a new ConfigBuilder to build and parse the bundle config
func NewConfigBuilder() *ConfigBuilder {
	return v1.NewConfigBuilder()
}

// ConfigBuilder assists in the construction of the plugin configuration.
type ConfigBuilder = v1.ConfigBuilder

// Config represents the configuration of the plugin.
// The Config can define a single bundle source or a map of
// `Source` objects defining where/how to download bundles. The
// older single bundle configuration is deprecated and will be
// removed in the future in favor of the `Bundles` map.
type Config = v1.Config

// Source is a configured bundle source to download bundles from
type Source = v1.Source
