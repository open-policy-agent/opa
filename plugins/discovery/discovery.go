// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package discovery implements configuration discovery.
//
// Deprecated: This package is intended for older projects transitioning from OPA v0.x and will remain for the lifetime of OPA v1.x, but its use is not recommended.
// For newer features and behaviours, such as defaulting to the Rego v1 syntax, use the corresponding components in the [github.com/open-policy-agent/opa/v1] package instead.
// See https://www.openpolicyagent.org/docs/latest/v0-compatibility/ for more information.
package discovery

import (
	"github.com/open-policy-agent/opa/plugins"
	"github.com/open-policy-agent/opa/v1/hooks"
	"github.com/open-policy-agent/opa/v1/metrics"
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

const (
	// Name is the discovery plugin name that will be registered with the plugin manager.
	Name = v1.Name
)

// Discovery implements configuration discovery for OPA. When discovery is
// started it will periodically download a configuration bundle and try to
// reconfigure the OPA.
type Discovery = v1.Discovery

// Factories provides a set of factory functions to use for
// instantiating custom plugins.
func Factories(fs map[string]plugins.Factory) func(*Discovery) {
	return v1.Factories(fs)
}

// Metrics provides a metrics provider to pass to plugins.
func Metrics(m metrics.Metrics) func(*Discovery) {
	return v1.Metrics(m)
}

func Hooks(hs hooks.Hooks) func(*Discovery) {
	return v1.Hooks(hs)
}

func BootConfig(bootConfig map[string]any) func(*Discovery) {
	return v1.BootConfig(bootConfig)
}

// New returns a new discovery plugin.
func New(manager *plugins.Manager, opts ...func(*Discovery)) (*Discovery, error) {
	return v1.New(manager, opts...)
}

// Lookup returns the discovery plugin registered with the manager.
func Lookup(manager *plugins.Manager) *Discovery {
	return v1.Lookup(manager)
}
