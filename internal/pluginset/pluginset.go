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
	"slices"

	"github.com/open-policy-agent/opa/v1/config"
	"github.com/open-policy-agent/opa/v1/logging"
	"github.com/open-policy-agent/opa/v1/metrics"
	"github.com/open-policy-agent/opa/v1/plugins"
	"github.com/open-policy-agent/opa/v1/plugins/bundle"
	"github.com/open-policy-agent/opa/v1/plugins/logs"
	"github.com/open-policy-agent/opa/v1/plugins/status"
)

// Set holds the plugins Configs.Set registered and that still need starting, and
// those already registered that need reconfiguring.
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

// Configs holds the plugin configurations a configuration enables. Deriving them
// registers nothing, so a caller can inspect Orphaned and reject a configuration
// before any plugin has been created.
type Configs struct {
	bundle  *bundle.Config
	logs    *logs.Config
	status  *status.Config
	custom  []pluginfactory
	metrics metrics.Metrics

	// Orphaned names the configuration sections of plugins that are running but
	// that these configurations no longer enable. Building the set leaves them
	// running unchanged -- the manager cannot unregister a plugin -- so a caller
	// that needs the running plugins to match the configuration has to reject it
	// instead. Discovery ignores this.
	Orphaned []string
}

// New validates config and registers any plugin it enables that isn't registered
// yet. Plugins config no longer enables are left running as-is.
func New(
	factories map[string]plugins.Factory,
	manager *plugins.Manager,
	config *config.Config,
	m metrics.Metrics,
	l logging.Logger,
	trigger *plugins.TriggerMode,
) (*Set, error) {
	configs, err := Parse(factories, manager, config, m, l, trigger)
	if err != nil {
		return nil, err
	}

	return configs.Set(manager), nil
}

// Parse validates config and derives the plugin configurations it enables,
// without touching the manager.
func Parse(
	factories map[string]plugins.Factory,
	manager *plugins.Manager,
	config *config.Config,
	m metrics.Metrics,
	l logging.Logger,
	trigger *plugins.TriggerMode,
) (*Configs, error) {
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

	configs := &Configs{
		bundle:  bundleConfig,
		logs:    decisionLogsConfig,
		status:  statusConfig,
		custom:  pluginFactories,
		metrics: m,
	}

	if bundleConfig == nil && bundle.Lookup(manager) != nil {
		configs.Orphaned = append(configs.Orphaned, "bundles")
	}
	if decisionLogsConfig == nil && logs.Lookup(manager) != nil {
		configs.Orphaned = append(configs.Orphaned, "decision_logs")
	}
	if statusConfig == nil && status.Lookup(manager) != nil {
		configs.Orphaned = append(configs.Orphaned, "status")
	}
	// Only the plugins this package manages: the manager holds others, such as
	// discovery itself, that no configuration section here enables.
	for name := range factories {
		if _, ok := config.Plugins[name]; !ok && manager.Plugin(name) != nil {
			configs.Orphaned = append(configs.Orphaned, "plugins."+name)
		}
	}
	slices.Sort(configs.Orphaned)

	return configs, nil
}

// Set registers any plugin these configurations enable that isn't registered
// yet, and returns the plugins to start and reconfigure.
func (c *Configs) Set(manager *plugins.Manager) *Set {
	// Accumulate plugins to start or reconfigure.
	starts := []plugins.Plugin{}
	reconfigs := []Reconfig{}

	if c.bundle != nil {
		p, created := getBundlePlugin(manager, c.bundle)
		if created {
			starts = append(starts, p)
		} else if p != nil {
			reconfigs = append(reconfigs, Reconfig{Config: c.bundle, Plugin: p})
		}
	}

	if c.logs != nil {
		p, created := getDecisionLogsPlugin(manager, c.logs, c.metrics)
		if created {
			starts = append(starts, p)
		} else if p != nil {
			reconfigs = append(reconfigs, Reconfig{Config: c.logs, Plugin: p})
		}
	}

	if c.status != nil {
		p, created := getStatusPlugin(manager, c.status, c.metrics)
		if created {
			starts = append(starts, p)
		} else if p != nil {
			reconfigs = append(reconfigs, Reconfig{Config: c.status, Plugin: p})
		}
	}

	result := &Set{Start: starts, Reconfig: reconfigs}

	getCustomPlugins(manager, c.custom, result)

	return result
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
