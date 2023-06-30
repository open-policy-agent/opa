// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package discovery implements configuration discovery.
package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/open-policy-agent/opa/ast"
	bundleApi "github.com/open-policy-agent/opa/bundle"
	"github.com/open-policy-agent/opa/config"
	"github.com/open-policy-agent/opa/download"
	"github.com/open-policy-agent/opa/hooks"
	bundleUtils "github.com/open-policy-agent/opa/internal/bundle"
	cfg "github.com/open-policy-agent/opa/internal/config"
	"github.com/open-policy-agent/opa/internal/errors"
	"github.com/open-policy-agent/opa/keys"
	"github.com/open-policy-agent/opa/logging"
	"github.com/open-policy-agent/opa/metrics"
	"github.com/open-policy-agent/opa/plugins"
	"github.com/open-policy-agent/opa/plugins/bundle"
	"github.com/open-policy-agent/opa/plugins/logs"
	"github.com/open-policy-agent/opa/plugins/status"
	"github.com/open-policy-agent/opa/rego"
	"github.com/open-policy-agent/opa/storage/inmem"
)

const (
	// Name is the discovery plugin name that will be registered with the plugin manager.
	Name = "discovery"

	// maxActivationRetry represents the maximum number of attempts
	// to activate a persisted discovery bundle. The value chosen for
	// maxActivationRetry ensures that too much time is not spent to activate
	// a bundle that will never successfully activate.
	maxActivationRetry = 10
)

// Discovery implements configuration discovery for OPA. When discovery is
// started it will periodically download a configuration bundle and try to
// reconfigure the OPA.
type Discovery struct {
	manager           *plugins.Manager
	config            *Config
	factories         map[string]plugins.Factory
	downloader        bundle.Loader                       // discovery bundle downloader
	status            *bundle.Status                      // discovery status
	listenersMtx      sync.Mutex                          // lock for listener map
	listeners         map[interface{}]func(bundle.Status) // listeners for discovery update events
	etag              string                              // discovery bundle etag for caching purposes
	metrics           metrics.Metrics
	readyOnce         sync.Once
	logger            logging.Logger
	bundlePersistPath string
	hooks             hooks.Hooks
}

// Factories provides a set of factory functions to use for
// instantiating custom plugins.
func Factories(fs map[string]plugins.Factory) func(*Discovery) {
	return func(d *Discovery) {
		d.factories = fs
	}
}

// Metrics provides a metrics provider to pass to plugins.
func Metrics(m metrics.Metrics) func(*Discovery) {
	return func(d *Discovery) {
		d.metrics = m
	}
}

func Hooks(hs hooks.Hooks) func(*Discovery) {
	return func(d *Discovery) {
		d.hooks = hs
	}
}

// New returns a new discovery plugin.
func New(manager *plugins.Manager, opts ...func(*Discovery)) (*Discovery, error) {
	result := &Discovery{
		manager: manager,
	}

	for _, f := range opts {
		f(result)
	}

	config, err := NewConfigBuilder().WithBytes(manager.Config.Discovery).WithServices(manager.Services()).
		WithKeyConfigs(manager.PublicKeys()).Parse()

	if err != nil {
		return nil, err
	} else if config == nil {
		if _, err := getPluginSet(result.factories, manager, manager.Config, result.metrics, nil); err != nil {
			return nil, err
		}
		return result, nil
	}

	if names := manager.Config.PluginNames(); len(names) > 0 {
		return nil, fmt.Errorf("discovery prohibits manual configuration of %v", strings.Join(names, " and "))
	}

	result.config = config
	restClient := manager.Client(config.service)
	if strings.ToLower(restClient.Config().Type) == "oci" {
		ociStorePath := filepath.Join(os.TempDir(), "opa", "oci") // use temporary folder /tmp/opa/oci
		if manager.Config.PersistenceDirectory != nil {
			ociStorePath = filepath.Join(*manager.Config.PersistenceDirectory, "oci")
		}
		result.downloader = download.NewOCI(config.Config, restClient, config.path, ociStorePath).
			WithCallback(result.oneShot).
			WithBundleVerificationConfig(config.Signing).
			WithBundlePersistence(config.Persist)
	} else {
		result.downloader = download.New(config.Config, restClient, config.path).
			WithCallback(result.oneShot).
			WithBundleVerificationConfig(config.Signing).
			WithBundlePersistence(config.Persist)
	}
	result.status = &bundle.Status{
		Name: Name,
	}

	result.logger = manager.Logger().WithFields(map[string]interface{}{"plugin": Name})

	manager.UpdatePluginStatus(Name, &plugins.Status{State: plugins.StateNotReady})
	return result, nil
}

// Start starts the dynamic discovery process if configured.
func (c *Discovery) Start(ctx context.Context) error {

	bundlePersistPath, err := c.getBundlePersistPath()
	if err != nil {
		return err
	}
	c.bundlePersistPath = bundlePersistPath

	c.loadAndActivateBundleFromDisk(ctx)

	if c.downloader != nil {
		c.downloader.Start(ctx)
	} else {
		// If there is no dynamic discovery then update the status to OK.
		c.manager.UpdatePluginStatus(Name, &plugins.Status{State: plugins.StateOK})
	}
	return nil
}

// Stop stops the dynamic discovery process if configured.
func (c *Discovery) Stop(ctx context.Context) {
	if c.downloader != nil {
		c.downloader.Stop(ctx)
	}

	c.manager.UpdatePluginStatus(Name, &plugins.Status{State: plugins.StateNotReady})
}

// Reconfigure is a no-op on discovery.
func (*Discovery) Reconfigure(context.Context, interface{}) {
}

// Lookup returns the discovery plugin registered with the manager.
func Lookup(manager *plugins.Manager) *Discovery {
	if p := manager.Plugin(Name); p != nil {
		return p.(*Discovery)
	}
	return nil
}

func (c *Discovery) TriggerMode() *plugins.TriggerMode {
	if c.config == nil {
		return nil
	}
	return c.config.Trigger
}

func (c *Discovery) Trigger(ctx context.Context) error {
	if c.downloader == nil {
		return nil
	}
	return c.downloader.Trigger(ctx)
}

func (c *Discovery) RegisterListener(name interface{}, f func(bundle.Status)) {
	c.listenersMtx.Lock()
	defer c.listenersMtx.Unlock()

	if c.listeners == nil {
		c.listeners = map[interface{}]func(bundle.Status){}
	}

	c.listeners[name] = f
}

func (c *Discovery) getBundlePersistPath() (string, error) {
	persistDir, err := c.manager.Config.GetPersistenceDirectory()
	if err != nil {
		return "", err
	}

	return filepath.Join(persistDir, "bundles"), nil
}

func (c *Discovery) loadAndActivateBundleFromDisk(ctx context.Context) {

	if c.config != nil && c.config.Persist {
		b, err := c.loadBundleFromDisk()
		if err != nil {
			c.logger.Error("Failed to load discovery bundle from disk: %v", err)
			c.status.SetError(err)
			return
		}

		if b == nil {
			return
		}

		for retry := 0; retry < maxActivationRetry; retry++ {

			ps, err := c.processBundle(ctx, b)
			if err != nil {
				c.logger.Error("Discovery bundle processing error occurred: %v", err)
				c.status.SetError(err)
				continue
			}

			for _, p := range ps.Start {
				if err := p.Start(ctx); err != nil {
					c.logger.Error("Failed to start configured plugins: %v", err)
					c.status.SetError(err)
					return
				}
			}

			for _, p := range ps.Reconfig {
				p.Plugin.Reconfigure(ctx, p.Config)
			}

			c.status.SetError(nil)
			c.status.SetActivateSuccess(b.Manifest.Revision)

			// On the first activation success mark the plugin as being in OK state
			c.readyOnce.Do(func() {
				c.manager.UpdatePluginStatus(Name, &plugins.Status{State: plugins.StateOK})
			})

			c.logger.Debug("Discovery bundle loaded from disk and activated successfully.")
		}
	}
}

func (c *Discovery) loadBundleFromDisk() (*bundleApi.Bundle, error) {
	return bundleUtils.LoadBundleFromDisk(c.bundlePersistPath, c.discoveryBundleDirName(), c.config.Signing)
}

func (c *Discovery) saveBundleToDisk(raw io.Reader) error {

	bundleDir := filepath.Join(c.bundlePersistPath, c.discoveryBundleDirName())
	bundleFile := filepath.Join(bundleDir, "bundle.tar.gz")

	tmpFile, saveErr := saveCurrentBundleToDisk(bundleDir, raw)
	if saveErr != nil {
		c.logger.Error("Failed to save new discovery bundle to disk: %v", saveErr)

		if err := os.Remove(tmpFile); err != nil {
			c.logger.Warn("Failed to remove temp file ('%s'): %v", tmpFile, err)
		}

		if _, err := os.Stat(bundleFile); err == nil {
			c.logger.Warn("Older version of activated discovery bundle persisted, ignoring error")
			return nil
		}
		return saveErr
	}

	return os.Rename(tmpFile, bundleFile)
}

func saveCurrentBundleToDisk(path string, raw io.Reader) (string, error) {
	return bundleUtils.SaveBundleToDisk(path, raw)
}

func (c *Discovery) oneShot(ctx context.Context, u download.Update) {

	c.processUpdate(ctx, u)

	if p := status.Lookup(c.manager); p != nil {
		p.UpdateDiscoveryStatus(*c.status)
	}

	c.listenersMtx.Lock()
	defer c.listenersMtx.Unlock()

	for _, f := range c.listeners {
		f(*c.status)
	}
}

func (c *Discovery) processUpdate(ctx context.Context, u download.Update) {
	c.status.SetRequest()

	if u.Error != nil {
		c.logger.Error("Discovery download failed: %v", u.Error)
		c.status.SetError(u.Error)
		c.downloader.ClearCache()
		return
	}

	c.status.LastSuccessfulRequest = c.status.LastRequest

	if u.Bundle != nil {
		c.status.Type = u.Bundle.Type()
		c.status.LastSuccessfulDownload = c.status.LastSuccessfulRequest
		c.status.SetBundleSize(u.Size)

		if err := c.reconfigure(ctx, u); err != nil {
			c.logger.Error("Discovery reconfiguration error occurred: %v", err)
			c.status.SetError(err)
			c.downloader.ClearCache()
			return
		}

		if c.config != nil && c.config.Persist {
			c.logger.Debug("Persisting discovery bundle to disk in progress.")

			err := c.saveBundleToDisk(u.Raw)
			if err != nil {
				c.logger.Error("Persisting discovery bundle to disk failed: %v", err)
				c.status.SetError(err)
				c.downloader.SetCache("")
				return
			}
			c.logger.Debug("Discovery bundle persisted to disk successfully at path %v.", filepath.Join(c.bundlePersistPath, c.discoveryBundleDirName()))
		}

		c.status.SetError(nil)
		c.status.SetActivateSuccess(u.Bundle.Manifest.Revision)

		// On the first activation success mark the plugin as being in OK state
		c.readyOnce.Do(func() {
			c.manager.UpdatePluginStatus(Name, &plugins.Status{State: plugins.StateOK})
		})

		if u.ETag != "" {
			c.logger.Info("Discovery update processed successfully. Etag updated to %v.", u.ETag)
		} else {
			c.logger.Info("Discovery update processed successfully.")
		}
		c.etag = u.ETag
		return
	}

	if u.ETag == c.etag {
		c.logger.Debug("Discovery update skipped, server replied with not modified.")
		c.status.SetError(nil)
		return
	}
}

func (c *Discovery) reconfigure(ctx context.Context, u download.Update) error {

	ps, err := c.processBundle(ctx, u.Bundle)
	if err != nil {
		return err
	}

	for _, p := range ps.Start {
		if err := p.Start(ctx); err != nil {
			return err
		}
	}

	for _, p := range ps.Reconfig {
		p.Plugin.Reconfigure(ctx, p.Config)
	}

	return nil
}

func (c *Discovery) processBundle(ctx context.Context, b *bundleApi.Bundle) (*pluginSet, error) {

	config, err := evaluateBundle(ctx, c.manager.ID, c.manager.Info, b, c.config.query)
	if err != nil {
		return nil, err
	}

	c.hooks.Each(func(h hooks.Hook) {
		if f, ok := h.(hooks.ConfigDiscoveryHook); ok {
			if c, e := f.OnConfigDiscovery(ctx, config); e != nil {
				err = errors.Join(err, e)
			} else {
				config = c
			}
		}
	})
	if err != nil {
		return nil, err
	}

	// Note: We don't currently support changes to the discovery
	// configuration. These changes are risky because errors would be
	// unrecoverable (without keeping track of changes and rolling back...)
	config.Discovery = c.manager.Config.Discovery

	// check for updates to the discovery service
	opts := cfg.ServiceOptions{
		Raw:        config.Services,
		AuthPlugin: c.manager.AuthPlugin,
		Keys:       c.manager.PublicKeys(),
		Logger:     c.logger.WithFields(c.manager.Client(c.config.service).LoggerFields()),
	}
	services, err := cfg.ParseServicesConfig(opts)
	if err != nil {
		return nil, err
	}

	if client, ok := services[c.config.service]; ok {
		dClient := c.manager.Client(c.config.service)
		if !client.Config().Equal(dClient.Config()) {
			return nil, fmt.Errorf("updates to the discovery service are not allowed")
		}
	}

	// check for updates to the keys provided in the boot config
	keys, err := keys.ParseKeysConfig(config.Keys)
	if err != nil {
		return nil, err
	}

	if c.config.Signing != nil {
		for key, kc := range keys {
			if curr, ok := c.config.Signing.PublicKeys[key]; ok {
				if !curr.Equal(kc) {
					return nil, fmt.Errorf("updates to keys specified in the boot configuration are not allowed")
				}
			}
		}
	}

	if err := c.manager.Reconfigure(config); err != nil {
		return nil, err
	}

	return getPluginSet(c.factories, c.manager, config, c.metrics, c.config.Trigger)
}

// discoveryBundleDirName returns the name of the directory where the discovery bundle will be persisted.
// It wraps the deprecated config.Name and uses Name as a default.
func (c *Discovery) discoveryBundleDirName() string {
	if c.config.Name != nil {
		return *c.config.Name
	}
	return Name
}

func evaluateBundle(ctx context.Context, id string, info *ast.Term, b *bundleApi.Bundle, query string) (*config.Config, error) {

	modules := b.ParsedModules("discovery")

	compiler := ast.NewCompiler()

	if compiler.Compile(modules); compiler.Failed() {
		return nil, compiler.Errors
	}

	store := inmem.NewFromObjectWithOpts(b.Data, inmem.OptRoundTripOnWrite(false))

	rego := rego.New(
		rego.Query(query),
		rego.Compiler(compiler),
		rego.Store(store),
		rego.Runtime(info),
	)

	rs, err := rego.Eval(ctx)
	if err != nil {
		return nil, err
	}

	if len(rs) == 0 {
		return nil, fmt.Errorf("undefined configuration")
	}

	bs, err := json.Marshal(rs[0].Expressions[0].Value)
	if err != nil {
		return nil, err
	}

	return config.ParseConfig(bs, id)
}

type pluginSet struct {
	Start    []plugins.Plugin
	Reconfig []pluginreconfig
}

type pluginreconfig struct {
	Config interface{}
	Plugin plugins.Plugin
}

type pluginfactory struct {
	name    string
	factory plugins.Factory
	config  interface{}
}

func getPluginSet(factories map[string]plugins.Factory, manager *plugins.Manager, config *config.Config, m metrics.Metrics, trigger *plugins.TriggerMode) (*pluginSet, error) {

	// Parse and validate plugin configurations.
	pluginNames := []string{}
	pluginFactories := []pluginfactory{}

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
	bundleConfig, err := bundle.ParseConfig(config.Bundle, manager.Services())
	if err != nil {
		return nil, err
	}
	if bundleConfig == nil {
		bundleConfig, err = bundle.NewConfigBuilder().WithBytes(config.Bundles).WithServices(manager.Services()).
			WithKeyConfigs(manager.PublicKeys()).WithTriggerMode(trigger).Parse()
		if err != nil {
			return nil, err
		}
	} else {
		manager.Logger().Warn("Deprecated 'bundle' configuration specified. Use 'bundles' instead. See https://www.openpolicyagent.org/docs/latest/configuration/#bundles")
	}

	decisionLogsConfig, err := logs.NewConfigBuilder().WithBytes(config.DecisionLogs).WithServices(manager.Services()).
		WithPlugins(pluginNames).WithTriggerMode(trigger).Parse()
	if err != nil {
		return nil, err
	}

	statusConfig, err := status.NewConfigBuilder().WithBytes(config.Status).WithServices(manager.Services()).
		WithPlugins(pluginNames).WithTriggerMode(trigger).Parse()
	if err != nil {
		return nil, err
	}

	// Accumulate plugins to start or reconfigure.
	starts := []plugins.Plugin{}
	reconfigs := []pluginreconfig{}

	if bundleConfig != nil {
		p, created := getBundlePlugin(manager, bundleConfig)
		if created {
			starts = append(starts, p)
		} else if p != nil {
			reconfigs = append(reconfigs, pluginreconfig{bundleConfig, p})
		}
	}

	if decisionLogsConfig != nil {
		p, created := getDecisionLogsPlugin(manager, decisionLogsConfig, m)
		if created {
			starts = append(starts, p)
		} else if p != nil {
			reconfigs = append(reconfigs, pluginreconfig{decisionLogsConfig, p})
		}
	}

	if statusConfig != nil {
		p, created := getStatusPlugin(manager, statusConfig, m)
		if created {
			starts = append(starts, p)
		} else if p != nil {
			reconfigs = append(reconfigs, pluginreconfig{statusConfig, p})
		}
	}

	result := &pluginSet{starts, reconfigs}

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

func getCustomPlugins(manager *plugins.Manager, factories []pluginfactory, result *pluginSet) {
	for _, pf := range factories {
		if plugin := manager.Plugin(pf.name); plugin != nil {
			result.Reconfig = append(result.Reconfig, pluginreconfig{pf.config, plugin})
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
		bp.Register(pluginlistener(status.Name), sp.UpdateBundleStatus)
	} else {
		bp.RegisterBulkListener(pluginlistener(status.Name), sp.BulkUpdateBundleStatus)
	}
}
