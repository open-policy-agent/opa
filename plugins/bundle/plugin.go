// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package bundle implements bundle downloading.
package bundle

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"

	"github.com/open-policy-agent/opa/metrics"

	"github.com/open-policy-agent/opa/ast"

	"github.com/sirupsen/logrus"

	"github.com/open-policy-agent/opa/bundle"
	"github.com/open-policy-agent/opa/download"
	"github.com/open-policy-agent/opa/plugins"
	"github.com/open-policy-agent/opa/storage"
)

// Plugin implements bundle activation.
type Plugin struct {
	config        Config
	manager       *plugins.Manager                         // plugin manager for storage and service clients
	status        map[string]*Status                       // current status for each bundle
	etags         map[string]string                        // etag on last successful activation
	listeners     map[interface{}]func(Status)             // listeners to send status updates to
	bulkListeners map[interface{}]func(map[string]*Status) // listeners to send aggregated status updates to
	downloaders   map[string]*download.Downloader
	mtx           sync.Mutex
	cfgMtx        sync.Mutex
	legacyConfig  bool
}

// New returns a new Plugin with the given config.
func New(parsedConfig *Config, manager *plugins.Manager) *Plugin {
	initialStatus := map[string]*Status{}
	for name := range parsedConfig.Bundles {
		initialStatus[name] = &Status{
			Name: name,
		}
	}

	p := &Plugin{
		manager:     manager,
		config:      *parsedConfig,
		status:      initialStatus,
		downloaders: make(map[string]*download.Downloader),
		etags:       make(map[string]string),
	}
	p.initDownloaders()
	return p
}

// Name identifies the plugin on manager.
const Name = "bundle"

// Lookup returns the bundle plugin registered with the manager.
func Lookup(manager *plugins.Manager) *Plugin {
	if p := manager.Plugin(Name); p != nil {
		return p.(*Plugin)
	}
	return nil
}

// Start runs the plugin. The plugin will periodically try to download bundles
// from the configured service. When a new bundle is downloaded, the data and
// policies are extracted and inserted into storage.
func (p *Plugin) Start(ctx context.Context) error {
	p.mtx.Lock()
	defer p.mtx.Unlock()
	for name, dl := range p.downloaders {
		p.logInfo(name, "Starting bundle downloader.")
		dl.Start(ctx)
	}
	return nil
}

// Stop stops the plugin.
func (p *Plugin) Stop(ctx context.Context) {
	p.mtx.Lock()
	defer p.mtx.Unlock()
	for name, dl := range p.downloaders {
		p.logInfo(name, "Stopping bundle downloader.")
		dl.Stop(ctx)
	}
}

// Reconfigure notifies the plugin that it's configuration has changed.
// Any bundle configs that have changed or been added/removed will take
// affect.
func (p *Plugin) Reconfigure(ctx context.Context, config interface{}) {
	// Reconfiguring should not occur in parallel, lock to ensure
	// nothing swaps underneath us with the current p.config and the updated one.
	// Use p.cfgMtx instead of p.mtx so as to not block any bundle downloads/activations
	// that are in progress. We upgrade to p.mtx locking after stopping downloaders.
	p.cfgMtx.Lock()
	defer p.cfgMtx.Unlock()

	// Look for any bundles that have had their config changed, are new, or have been removed
	newConfig := config.(*Config)
	newBundles, updatedBundles, deletedBundles := p.configDelta(newConfig)
	p.config = *newConfig

	if len(updatedBundles) == 0 && len(newBundles) == 0 && len(deletedBundles) == 0 {
		// no relevant config changes
		return
	}

	// Stop the downloaders outside p.mtx to allow them to finish handling any in-progress requests.
	for name, dl := range p.downloaders {
		_, updated := updatedBundles[name]
		_, deleted := deletedBundles[name]
		if updated || deleted {
			dl.Stop(ctx)
		}
	}

	// Only lock p.mtx once we start changing the internal maps
	// and downloader configs.
	p.mtx.Lock()
	defer p.mtx.Unlock()

	// Cleanup existing downloaders that are deleted
	for name := range p.downloaders {
		if _, deleted := deletedBundles[name]; deleted {
			p.logInfo(name, "Bundle downloader configuration removed. Stopping bundle downloader.")
			delete(p.downloaders, name)
			delete(p.status, name)
			delete(p.etags, name)
		}
	}

	// Deactivate the bundles that were removed
	params := storage.WriteParams
	params.Context = storage.NewContext()
	err := storage.Txn(ctx, p.manager.Store, params, func(txn storage.Transaction) error {
		opts := &bundle.DeactivateOpts{
			Ctx:         ctx,
			Store:       p.manager.Store,
			Txn:         txn,
			BundleNames: deletedBundles,
		}
		err := bundle.Deactivate(opts)
		if err != nil {
			p.logError(fmt.Sprint(deletedBundles), "Failed to deactivate bundles: %s", err)
			return err
		}
		return nil
	})
	if err != nil {
		// TODO(patrick-east): This probably shouldn't panic.. But OPA shouldn't
		// continue in a potentially inconsistent state.
		panic(errors.New("Unable deactivate bundle: " + err.Error()))
	}

	for name, source := range p.config.Bundles {
		_, updated := updatedBundles[name]
		_, isNew := newBundles[name]

		if isNew || updated {
			if isNew {
				p.status[name] = &Status{Name: name}
				p.logInfo(name, "New bundle downloader configuration added. Starting bundle downloader.")
			} else {
				p.logInfo(name, "Bundle downloader configuration changed. Restarting bundle downloader.")
			}
			p.downloaders[name] = p.newDownloader(name, source)
			p.downloaders[name].Start(ctx)
		}
	}
}

// Register a listener to receive status updates. The name must be comparable.
// The listener will receive a status update for each bundle configured, they are
// not going to be aggregated. For all status updates use `RegisterBulkListener`.
func (p *Plugin) Register(name interface{}, listener func(Status)) {
	p.mtx.Lock()
	defer p.mtx.Unlock()

	if p.listeners == nil {
		p.listeners = map[interface{}]func(Status){}
	}

	p.listeners[name] = listener
}

// Unregister a listener to stop receiving status updates.
func (p *Plugin) Unregister(name interface{}) {
	p.mtx.Lock()
	defer p.mtx.Unlock()

	delete(p.bulkListeners, name)
}

// RegisterBulkListener registers a listener to receive bulk (aggregated) status updates. The name must be comparable.
func (p *Plugin) RegisterBulkListener(name interface{}, listener func(map[string]*Status)) {
	p.mtx.Lock()
	defer p.mtx.Unlock()

	if p.bulkListeners == nil {
		p.bulkListeners = map[interface{}]func(map[string]*Status){}
	}

	p.bulkListeners[name] = listener
}

// UnregisterBulkListener unregisters a listener to stop receiving aggregated status updates.
func (p *Plugin) UnregisterBulkListener(name interface{}) {
	p.mtx.Lock()
	defer p.mtx.Unlock()

	delete(p.bulkListeners, name)
}

// Config returns the plugins current configuration
func (p *Plugin) Config() *Config {
	return &p.config
}

func (p *Plugin) initDownloaders() {
	// Initialize a downloader for each bundle configured.
	for name, source := range p.config.Bundles {
		p.downloaders[name] = p.newDownloader(name, source)
	}
}

func (p *Plugin) newDownloader(name string, source *Source) *download.Downloader {
	conf := source.Config
	client := p.manager.Client(source.Service)
	path := source.Resource
	return download.New(conf, client, path).WithCallback(func(ctx context.Context, u download.Update) {
		// wrap the callback to include the name of the bundle that was updated
		p.oneShot(ctx, name, u)
	})
}

func (p *Plugin) oneShot(ctx context.Context, name string, u download.Update) {
	p.mtx.Lock()
	defer p.mtx.Unlock()

	p.process(ctx, name, u)

	for _, listener := range p.listeners {
		listener(*p.status[name])
	}

	for _, listener := range p.bulkListeners {
		listener(p.status)
	}
}

func (p *Plugin) process(ctx context.Context, name string, u download.Update) {

	if u.Error != nil {
		p.logError(name, "Bundle download failed: %v", u.Error)
		p.status[name].SetError(u.Error)
		return
	}

	if u.Bundle != nil {
		p.status[name].SetDownloadSuccess()

		if err := p.activate(ctx, name, u.Bundle); err != nil {
			p.logError(name, "Bundle activation failed: %v", err)
			p.status[name].SetError(err)
			return
		}

		p.status[name].SetError(nil)
		p.status[name].SetActivateSuccess(u.Bundle.Manifest.Revision)
		if u.ETag != "" {
			p.logInfo(name, "Bundle downloaded and activated successfully. Etag updated to %v.", u.ETag)
		} else {
			p.logInfo(name, "Bundle downloaded and activated successfully.")
		}
		p.etags[name] = u.ETag
		return
	}

	if etag, ok := p.etags[name]; ok && u.ETag == etag {
		p.logDebug(name, "Bundle download skipped, server replied with not modified.")
		p.status[name].SetError(nil)
		return
	}
}

func (p *Plugin) activate(ctx context.Context, name string, b *bundle.Bundle) error {
	p.logDebug(name, "Bundle activation in progress. Opening storage transaction.")

	params := storage.WriteParams
	params.Context = storage.NewContext()

	err := storage.Txn(ctx, p.manager.Store, params, func(txn storage.Transaction) error {
		p.logDebug(name, "Opened storage transaction (%v).", txn.ID())
		defer p.logDebug(name, "Closing storage transaction (%v).", txn.ID())

		// Compile the bundle modules with a new compiler and set it on the
		// transaction params for use by onCommit hooks.
		compiler := ast.NewCompiler().WithPathConflictsCheck(storage.NonEmpty(ctx, p.manager.Store, txn))

		var activateErr error
		m := metrics.New()

		opts := &bundle.ActivateOpts{
			Ctx:      ctx,
			Store:    p.manager.Store,
			Txn:      txn,
			Compiler: compiler,
			Metrics:  m,
			Bundles:  map[string]*bundle.Bundle{name: b},
		}

		if p.config.IsMultiBundle() {
			activateErr = bundle.Activate(opts)
		} else {
			activateErr = bundle.ActivateLegacy(opts)
		}

		plugins.SetCompilerOnContext(params.Context, compiler)

		return activateErr
	})

	return err
}

func (p *Plugin) logError(bundleName string, fmt string, a ...interface{}) {
	logrus.WithFields(p.logrusFields(bundleName)).Errorf(fmt, a...)
}

func (p *Plugin) logInfo(bundleName string, fmt string, a ...interface{}) {
	logrus.WithFields(p.logrusFields(bundleName)).Infof(fmt, a...)
}

func (p *Plugin) logDebug(bundleName string, fmt string, a ...interface{}) {
	logrus.WithFields(p.logrusFields(bundleName)).Debugf(fmt, a...)
}

func (p *Plugin) logrusFields(bundleName string) logrus.Fields {

	f := logrus.Fields{
		"plugin": Name,
		"name":   bundleName,
	}

	return f
}

// configDelta will return a map of new bundle sources, updated bundle sources, and a set of deleted bundle names
func (p *Plugin) configDelta(newConfig *Config) (map[string]*Source, map[string]*Source, map[string]struct{}) {
	deletedBundles := map[string]struct{}{}
	for name := range p.config.Bundles {
		deletedBundles[name] = struct{}{}
	}
	newBundles := map[string]*Source{}
	updatedBundles := map[string]*Source{}
	for name, source := range newConfig.Bundles {
		oldSource, found := p.config.Bundles[name]
		if !found {
			newBundles[name] = source
		} else {
			delete(deletedBundles, name)
			if !reflect.DeepEqual(oldSource, source) {
				updatedBundles[name] = source
			}
		}
	}

	return newBundles, updatedBundles, deletedBundles
}
