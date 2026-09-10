// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package pluginset derives the plugins implied by an OPA configuration, split
// into those needing a start and those needing a reconfigure. Shared by the
// discovery plugin and the runtime's configuration file reloading.
package pluginset

import (
	"context"
	"fmt"

	"github.com/open-policy-agent/opa/v1/config"
	"github.com/open-policy-agent/opa/v1/logging"
	"github.com/open-policy-agent/opa/v1/metrics"
	"github.com/open-policy-agent/opa/v1/plugins"
	"github.com/open-policy-agent/opa/v1/plugins/bundle"
	"github.com/open-policy-agent/opa/v1/plugins/logs"
	"github.com/open-policy-agent/opa/v1/plugins/status"
)

// Set holds the plugins Get registered and still need starting, and those
// already registered that need reconfiguring.
type Set struct {
	Start    []plugins.Plugin
	Reconfig []Reconfig
}

// Reconfig pairs an already running plugin with its new configuration.
type Reconfig struct {
	Config any
	Plugin plugins.Plugin
}

// Apply starts the plugins that aren't running, then reconfigures the rest.
func (s *Set) Apply(ctx context.Context) error {
	for _, p := range s.Start {
		if err := p.Start(ctx); err != nil {
			return err
		}
	}

	for _, p := range s.Reconfig {
		p.Plugin.Reconfigure(ctx, p.Config)
	}

	return nil
}

type pluginfactory struct {
	name    string
	factory plugins.Factory
	config  any
}

// Get validates config and registers any plugin it enables that isn't registered
// yet. Plugins config no longer mentions are left running as-is.
func Get(
	factories map[string]plugins.Factory,
	manager *plugins.Manager,
	config *config.Config,
	m metrics.Metrics,
	l logging.Logger,
	trigger *plugins.TriggerMode,
) (*Set, error) {
	// Parse and validate plugin configurations.
	pluginNames := []string{}
	pluginFactories := []pluginfactory{}
	serviceNames := manager.Services()

	for k := range config.Plugins {
		f, ok := factories[k]
		if !ok {
			return nil, fmt.Errorf("plugin %q not registered", k)
		}

		c, err := f.Validate(manager, config.Plugins[k])
		if err != nil {
			return nil, err
		}

		pluginFactories = append(pluginFactories, pluginfactory{
			name:    k,
			factory: f,
			config:  c,
		})

		pluginNames = append(pluginNames, k)
	}

	// Parse and validate bundle/logs/status configurations.

	// If `bundle` was configured use that, otherwise try the new `bundles` option
	bundleConfig, err := bundle.ParseConfig(config.Bundle, serviceNames) //nolint:staticcheck
	if err != nil {
		return nil, err
	}
	if bundleConfig == nil {
		bundleConfig, err = bundle.NewConfigBuilder().WithBytes(config.Bundles).WithServices(serviceNames).
			WithKeyConfigs(manager.PublicKeys()).WithTriggerMode(trigger).Parse()
		if err != nil {
			return nil, err
		}
	} else {
		manager.Logger().Warn("Deprecated 'bundle' configuration specified. Use 'bundles' instead. See https://www.openpolicyagent.org/docs/latest/configuration/#bundles")
	}

	decisionLogsConfig, err := logs.NewConfigBuilder().WithBytes(config.DecisionLogs).WithServices(serviceNames).
		WithPlugins(pluginNames).WithTriggerMode(trigger).WithLogger(l).Parse()
	if err != nil {
		return nil, err
	}

	statusConfig, err := status.NewConfigBuilder().WithBytes(config.Status).WithServices(serviceNames).
		WithPlugins(pluginNames).WithTriggerMode(trigger).Parse()
	if err != nil {
		return nil, err
	}

	// Accumulate plugins to start or reconfigure.
	starts := []plugins.Plugin{}
	reconfigs := []Reconfig{}

	if bundleConfig != nil {
		p, created := getBundlePlugin(manager, bundleConfig)
		if created {
			starts = append(starts, p)
		} else if p != nil {
			reconfigs = append(reconfigs, Reconfig{Config: bundleConfig, Plugin: p})
		}
	}

	if decisionLogsConfig != nil {
		p, created := getDecisionLogsPlugin(manager, decisionLogsConfig, m)
		if created {
			starts = append(starts, p)
		} else if p != nil {
			reconfigs = append(reconfigs, Reconfig{Config: decisionLogsConfig, Plugin: p})
		}
	}

	if statusConfig != nil {
		p, created := getStatusPlugin(manager, statusConfig, m)
		if created {
			starts = append(starts, p)
		} else if p != nil {
			reconfigs = append(reconfigs, Reconfig{Config: statusConfig, Plugin: p})
		}
	}

	result := &Set{Start: starts, Reconfig: reconfigs}

	getCustomPlugins(manager, pluginFactories, result)

	return result, nil
}

func getBundlePlugin(m *plugins.Manager, config *bundle.Config) (plugin *bundle.Plugin, created bool) {
	plugin = bundle.Lookup(m)
	if plugin == nil {
		plugin = bundle.New(config, m)
		m.Register(bundle.Name, plugin)
		registerBundleStatusUpdates(m)
		created = true
	}
	return plugin, created
}

func getDecisionLogsPlugin(m *plugins.Manager, config *logs.Config, metrics metrics.Metrics) (plugin *logs.Plugin, created bool) {
	plugin = logs.Lookup(m)
	if plugin == nil {
		plugin = logs.New(config, m).WithMetrics(metrics)
		m.Register(logs.Name, plugin)
		created = true
	}
	return plugin, created
}

func getStatusPlugin(m *plugins.Manager, config *status.Config, metrics metrics.Metrics) (plugin *status.Plugin, created bool) {
	plugin = status.Lookup(m)

	if plugin == nil {
		plugin = status.New(config, m).WithMetrics(metrics)
		m.Register(status.Name, plugin)
		registerBundleStatusUpdates(m)
		created = true
	}

	return plugin, created
}

func getCustomPlugins(manager *plugins.Manager, factories []pluginfactory, result *Set) {
	for _, pf := range factories {
		if plugin := manager.Plugin(pf.name); plugin != nil {
			result.Reconfig = append(result.Reconfig, Reconfig{Config: pf.config, Plugin: plugin})
		} else {
			plugin := pf.factory.New(manager, pf.config)
			manager.Register(pf.name, plugin)
			result.Start = append(result.Start, plugin)
		}
	}
}

func registerBundleStatusUpdates(m *plugins.Manager) {
	bp := bundle.Lookup(m)
	sp := status.Lookup(m)
	if bp == nil || sp == nil {
		return
	}
	type pluginlistener string

	// Depending on how the plugin was configured we will want to use different listeners
	// for backwards compatibility.
	if !bp.Config().IsMultiBundle() {
		bp.Register(pluginlistener(status.Name), sp.UpdateBundleStatus) //nolint:staticcheck
	} else {
		bp.RegisterBulkListener(pluginlistener(status.Name), sp.BulkUpdateBundleStatus)
	}
}
