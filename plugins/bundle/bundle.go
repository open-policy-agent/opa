// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Deprecated: This package is intended for older projects transitioning from OPA v0.x and will remain for the lifetime of OPA v1.x, but its use is not recommended.
// For newer features and behaviours, such as defaulting to the Rego v1 syntax, use the corresponding components in the [github.com/open-policy-agent/opa/v1] package instead.
// See https://www.openpolicyagent.org/docs/latest/v0-compatibility/ for more information.
//
// Package bundle implements bundle loading.
package bundle

import (
	"github.com/open-policy-agent/opa/plugins"
	v1 "github.com/open-policy-agent/opa/v1/plugins/bundle"
)

// ParseConfig validates the config and injects default values. This is
// for the legacy single bundle configuration. This will add the bundle
// to the `Bundles` map to provide compatibility with newer clients.
//
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

// Errors represents a list of errors that occurred during a bundle load enriched by the bundle name.
type Errors = v1.Errors

type Error = v1.Error

func NewBundleError(bundleName string, cause error) Error {
	return v1.NewBundleError(bundleName, cause)
}

// Loader defines the interface that the bundle plugin uses to control bundle
// loading via HTTP, disk, etc.
type Loader = v1.Loader

// Plugin implements bundle activation.
type Plugin = v1.Plugin

// New returns a new Plugin with the given config.
func New(parsedConfig *Config, manager *plugins.Manager) *Plugin {
	return v1.New(parsedConfig, manager)
}

// Name identifies the plugin on manager.
const Name = v1.Name

// Lookup returns the bundle plugin registered with the manager.
func Lookup(manager *plugins.Manager) *Plugin {
	return v1.Lookup(manager)
}

// Status represents the status of processing a bundle.
type Status = v1.Status
