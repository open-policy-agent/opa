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
	"strings"
	"sync"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/bundle"
	"github.com/open-policy-agent/opa/download"
	"github.com/open-policy-agent/opa/plugins"
	"github.com/open-policy-agent/opa/storage"
	"github.com/sirupsen/logrus"
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
		for name := range deletedBundles {
			_, err := p.deactivate(ctx, txn, name, nil)
			if err != nil {
				p.logError(name, "Failed to deactivate bundle: %s", err)
				return err
			}
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

	return storage.Txn(ctx, p.manager.Store, params, func(txn storage.Transaction) error {
		p.logDebug(name, "Opened storage transaction (%v).", txn.ID())
		defer p.logDebug(name, "Closing storage transaction (%v).", txn.ID())

		// Erase data at new roots to prepare for writing the new data
		newRoots := map[string]struct{}{}

		if b.Manifest.Roots != nil {
			for _, root := range *b.Manifest.Roots {
				newRoots[root] = struct{}{}
			}
		}

		// Erase data and policies at new + old roots, and remove the old
		// manifest before activating a new bundle.
		remaining, err := p.deactivate(ctx, txn, name, newRoots)
		if err != nil {
			return err
		}

		// Write data from new bundle into store. Only write under the
		// roots contained in the manifest.
		if err := p.writeData(ctx, txn, *b.Manifest.Roots, b.Data); err != nil {
			return err
		}

		compiler, err := p.writeModules(ctx, txn, b.Modules, remaining)
		if err != nil {
			return err
		}

		// Always write manifests to the named location. If the plugin is in the older style config
		// then also write to the old legacy unnamed location.
		if err := bundle.WriteManifestToStore(ctx, p.manager.Store, txn, name, b.Manifest); err != nil {
			return err
		}
		if !p.config.IsMultiBundle() {
			if err := bundle.LegacyWriteManifestToStore(ctx, p.manager.Store, txn, b.Manifest); err != nil {
				return err
			}
		}

		plugins.SetCompilerOnContext(params.Context, compiler)

		return nil
	})
}

// deactivate a bundle by name. This will clear all policies and data at its roots and remove its
// manifest from storage. If additionalRoots are provided they will be deleted along with the
// roots found in storage for the bundle.
func (p *Plugin) deactivate(ctx context.Context, txn storage.Transaction, name string, additionalRoots map[string]struct{}) (map[string]*ast.Module, error) {
	erase := additionalRoots
	if erase == nil {
		erase = map[string]struct{}{}
	}

	if roots, err := bundle.ReadBundleRootsFromStore(ctx, p.manager.Store, txn, name); err == nil {
		for _, root := range roots {
			erase[root] = struct{}{}
		}
	} else if !storage.IsNotFound(err) {
		return nil, err
	}

	p.logDebug(name, "Erasing data and polices with roots at %+v", erase)

	if err := p.eraseData(ctx, txn, erase); err != nil {
		return nil, err
	}

	remaining, err := p.erasePolicies(ctx, txn, erase)
	if err != nil {
		return nil, err
	}

	if err := bundle.EraseManifestFromStore(ctx, p.manager.Store, txn, name); err != nil && !storage.IsNotFound(err) {
		return nil, err
	}

	if err := bundle.LegacyEraseManifestFromStore(ctx, p.manager.Store, txn); err != nil && !storage.IsNotFound(err) {
		return nil, err
	}

	return remaining, nil
}

func (p *Plugin) eraseData(ctx context.Context, txn storage.Transaction, roots map[string]struct{}) error {
	for root := range roots {
		path, ok := storage.ParsePathEscaped("/" + root)
		if !ok {
			return fmt.Errorf("manifest root path invalid: %v", root)
		}
		if len(path) > 0 {
			if err := p.manager.Store.Write(ctx, txn, storage.RemoveOp, path, nil); err != nil {
				if !storage.IsNotFound(err) {
					return err
				}
			}
		}
	}
	return nil
}

func (p *Plugin) erasePolicies(ctx context.Context, txn storage.Transaction, roots map[string]struct{}) (map[string]*ast.Module, error) {

	ids, err := p.manager.Store.ListPolicies(ctx, txn)
	if err != nil {
		return nil, err
	}

	remaining := map[string]*ast.Module{}

	for _, id := range ids {
		bs, err := p.manager.Store.GetPolicy(ctx, txn, id)
		if err != nil {
			return nil, err
		}
		module, err := ast.ParseModule(id, string(bs))
		if err != nil {
			return nil, err
		}
		path, err := module.Package.Path.Ptr()
		if err != nil {
			return nil, err
		}
		deleted := false
		for root := range roots {
			if strings.HasPrefix(path, root) {
				if err := p.manager.Store.DeletePolicy(ctx, txn, id); err != nil {
					return nil, err
				}
				deleted = true
				break
			}
		}
		if !deleted {
			remaining[id] = module
		}
	}

	return remaining, nil
}

func (p *Plugin) writeData(ctx context.Context, txn storage.Transaction, roots []string, data map[string]interface{}) error {
	for _, root := range roots {
		path, ok := storage.ParsePathEscaped("/" + root)
		if !ok {
			return fmt.Errorf("manifest root path invalid: %v", root)
		}
		if value, ok := lookup(path, data); ok {
			if len(path) > 0 {
				if err := storage.MakeDir(ctx, p.manager.Store, txn, path[:len(path)-1]); err != nil {
					return err
				}
			}
			if err := p.manager.Store.Write(ctx, txn, storage.AddOp, path, value); err != nil {
				return err
			}
		}
	}
	return nil
}

func (p *Plugin) writeModules(ctx context.Context, txn storage.Transaction, files []bundle.ModuleFile, remaining map[string]*ast.Module) (*ast.Compiler, error) {
	modules := map[string]*ast.Module{}
	for name, module := range remaining {
		modules[name] = module
	}
	for _, file := range files {
		modules[file.Path] = file.Parsed
	}
	compiler := ast.NewCompiler().
		WithPathConflictsCheck(storage.NonEmpty(ctx, p.manager.Store, txn))
	if compiler.Compile(modules); compiler.Failed() {
		return nil, compiler.Errors
	}
	for _, file := range files {
		if err := p.manager.Store.UpsertPolicy(ctx, txn, file.Path, file.Raw); err != nil {
			return nil, err
		}
	}
	return compiler, nil
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

func lookup(path storage.Path, data map[string]interface{}) (interface{}, bool) {
	if len(path) == 0 {
		return data, true
	}
	for i := 0; i < len(path)-1; i++ {
		value, ok := data[path[i]]
		if !ok {
			return nil, false
		}
		obj, ok := value.(map[string]interface{})
		if !ok {
			return nil, false
		}
		data = obj
	}
	value, ok := data[path[len(path)-1]]
	return value, ok
}
