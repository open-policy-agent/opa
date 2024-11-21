// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package discovery implements configuration discovery.
package discovery

import (
	"github.com/open-policy-agent/opa/plugins"
	"github.com/open-policy-agent/opa/v1/hooks"
	"github.com/open-policy-agent/opa/v1/metrics"
	v1 "github.com/open-policy-agent/opa/v1/plugins/discovery"
)

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

func BootConfig(bootConfig map[string]interface{}) func(*Discovery) {
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
