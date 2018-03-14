// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package plugins implements plugin management for the policy engine.
package plugins

import (
	"context"
	"encoding/json"

	"github.com/open-policy-agent/opa/plugins/rest"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/util"
)

// Plugin defines the interface for OPA plugins.
type Plugin interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context)
}

// Manager implements lifecycle management of plugins and gives plugins access
// to engine-wide components like storage.
type Manager struct {
	Store    storage.Store
	services map[string]rest.Client
	plugins  []Plugin
}

// New creates a new Manager using config.
func New(config []byte, store storage.Store) (*Manager, error) {

	var parsedConfig struct {
		Services []json.RawMessage
	}

	if err := util.Unmarshal(config, &parsedConfig); err != nil {
		return nil, err
	}

	services := map[string]rest.Client{}

	for _, s := range parsedConfig.Services {
		client, err := rest.New(s)
		if err != nil {
			return nil, err
		}
		services[client.Service()] = client
	}

	m := &Manager{
		Store:    store,
		services: services,
	}

	return m, nil
}

// Register adds a plugin to the manager. When the manager is started, all of
// the plugins will be started.
func (m *Manager) Register(plugin Plugin) {
	m.plugins = append(m.plugins, plugin)
}

// Start starts the manager.
func (m *Manager) Start(ctx context.Context) error {
	if m == nil {
		return nil
	}
	for _, p := range m.plugins {
		if err := p.Start(ctx); err != nil {
			return err
		}
	}
	return nil
}

// Client returns a client for communicating with a remote service.
func (m *Manager) Client(name string) rest.Client {
	return m.services[name]
}

// Services returns a list of services that m can provide clients for.
func (m *Manager) Services() []string {
	s := make([]string, 0, len(m.services))
	for name := range m.services {
		s = append(s, name)
	}
	return s
}
