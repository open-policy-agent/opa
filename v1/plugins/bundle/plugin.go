// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package bundle implements bundle loading.
package bundle

import (
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"

	bundleUtils "github.com/open-policy-agent/opa/internal/bundle"
	"github.com/open-policy-agent/opa/internal/ref"
	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/bundle"
	"github.com/open-policy-agent/opa/v1/download"
	"github.com/open-policy-agent/opa/v1/hooks"
	"github.com/open-policy-agent/opa/v1/logging"
	"github.com/open-policy-agent/opa/v1/metrics"
	"github.com/open-policy-agent/opa/v1/plugins"
	"github.com/open-policy-agent/opa/v1/plugins/rest"
	"github.com/open-policy-agent/opa/v1/storage"
)

// maxActivationRetry represents the maximum number of attempts
// to activate persisted bundles. Activation retries are useful
// in scenarios where a persisted bundle may have a dependency on some
// other persisted bundle. As there are no ordering guarantees for which
// bundle loads first, retries could help in the bundle activation process.
// Typically, multiple bundles are not encouraged. The value chosen for
// maxActivationRetry allows upto 10 bundles to successfully activate
// in the worst case that they depend on each other. At the same time, it also
// ensures that too much time is not spent to activate bundles that will never
// successfully activate.
const maxActivationRetry = 10

var goos = runtime.GOOS

// Loader defines the interface that the bundle plugin uses to control bundle
// loading via HTTP, disk, etc.
type Loader interface {
	Start(context.Context)
	Stop(context.Context)
	Trigger(context.Context) error
	SetCache(string)
	ClearCache()
}

// Plugin implements bundle activation.
type Plugin struct {
	config            Config
	clientConfigs     map[string]*rest.Config          // service client each downloader was built with, guarded by cfgMtx
	manager           *plugins.Manager                 // plugin manager for storage and service clients
	status            map[string]*Status               // current status for each bundle
	etags             map[string]string                // etag on last successful activation
	listeners         map[any]func(Status)             // listeners to send status updates to
	bulkListeners     map[any]func(map[string]*Status) // listeners to send aggregated status updates to
	downloaders       map[string]Loader
	logger            logging.Logger
	mtx               sync.Mutex
	cfgMtx            sync.RWMutex
	ready             bool
	bundlePersistPath string
	stopped           bool
	batching          bool                       // collecting the initial load's downloads, guarded by mtx
	batchedBundles    map[string]download.Update // downloads collected during the initial load, by name, guarded by mtx
	batchedReported   map[string]struct{}        // bundles that have reported during the initial load, guarded by mtx
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
		manager:       manager,
		config:        *parsedConfig,
		clientConfigs: clientConfigs(manager, parsedConfig.Bundles),
		status:        initialStatus,
		downloaders:   make(map[string]Loader),
		etags:         make(map[string]string),
		ready:         false,
		logger:        manager.Logger(),
	}

	manager.UpdatePluginStatus(Name, &plugins.Status{State: plugins.StateNotReady})
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

	var err error

	p.bundlePersistPath, err = p.getBundlePersistPath()
	if err != nil {
		return err
	}

	p.loadAndActivateBundlesFromDisk(ctx)

	// Bundles that arrive from here on are held back until every configured
	// bundle has reported, so the initial load compiles them once instead of
	// once per bundle. A load that already made the plugin ready has nothing
	// left to batch.
	p.cfgMtx.RLock()
	batchBundleActivation := p.config.BatchBundleActivation
	p.cfgMtx.RUnlock()

	p.batching = batchBundleActivation && !p.ready
	if p.batching {
		p.batchedBundles = map[string]download.Update{}
		p.batchedReported = map[string]struct{}{}
	}

	p.initDownloaders(ctx)
	for name, dl := range p.downloaders {
		p.log(name).Info("Starting bundle loader.")
		dl.Start(ctx)
	}
	return nil
}

// Stop stops the plugin.
func (p *Plugin) Stop(ctx context.Context) {
	p.mtx.Lock()
	stopDownloaders := map[string]Loader{}
	maps.Copy(stopDownloaders, p.downloaders)
	p.downloaders = nil
	p.stopped = true
	p.batching = false
	p.batchedBundles = nil
	p.batchedReported = nil
	p.mtx.Unlock()

	for name, dl := range stopDownloaders {
		p.log(name).Info("Stopping bundle loader.")
		dl.Stop(ctx)
	}
}

// Reconfigure notifies the plugin that it's configuration has changed.
// Any bundle configs that have changed or been added/removed will take
// effect.
func (p *Plugin) Reconfigure(ctx context.Context, config any) {
	// Reconfiguring should not occur in parallel, lock to ensure
	// nothing swaps underneath us with the current p.config and the updated one.
	// Use p.cfgMtx instead of p.mtx to not block any bundle downloads/activations
	// that are in progress. We upgrade to p.mtx locking after stopping downloaders.
	p.cfgMtx.Lock()

	// Look for any bundles that have had their config changed, are new, or have been removed
	newConfig := config.(*Config)

	for name, source := range newConfig.Bundles {
		err := source.ValidateAndInjectDefaults()
		if err != nil {
			p.log(name).Error("Failed to validate bundle configuration: %s", err)
			p.cfgMtx.Unlock()
			return
		}
	}
	newBundles, updatedBundles, deletedBundles := p.configDelta(newConfig)
	p.config = *newConfig
	p.clientConfigs = clientConfigs(p.manager, newConfig.Bundles)
	p.cfgMtx.Unlock()

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
			p.log(name).Info("Bundle loader configuration removed. Stopping bundle loader.")
			delete(p.downloaders, name)
			delete(p.status, name)
			delete(p.etags, name)
			p.dropFromBatch(name)
		}
	}

	// Deactivate the bundles that were removed
	params := storage.WriteParams
	params.Context = storage.NewContext() // TODO(sr): metrics?
	err := storage.Txn(ctx, p.manager.Store, params, func(txn storage.Transaction) error {
		opts := &bundle.DeactivateOpts{
			Ctx:           ctx,
			Store:         p.manager.Store,
			Txn:           txn,
			BundleNames:   deletedBundles,
			ParserOptions: p.manager.ParserOptions(),
		}
		err := bundle.Deactivate(opts)
		if err != nil {
			p.manager.Logger().Error(fmt.Sprint(deletedBundles), "Failed to deactivate bundles: %s", err)
			return err
		}
		return nil
	})
	if err != nil {
		// TODO(patrick-east): This probably shouldn't panic.. But OPA shouldn't
		// continue in a potentially inconsistent state.
		panic(errors.New("Unable deactivate bundle: " + err.Error()))
	}

	readyNow := p.ready

	bundles := p.getBundlesCpy()
	for name, source := range bundles {
		_, updated := updatedBundles[name]
		_, isNew := newBundles[name]

		if isNew || updated {
			// A restarted loader has to report before the initial load can
			// complete, because what the load is holding for this name, if
			// anything, was downloaded for the configuration it just replaced.
			p.dropFromBatch(name)

			if isNew {
				p.status[name] = &Status{Name: name}
				p.log(name).Info("New bundle loader configuration added. Starting bundle loader.")
			} else {
				p.log(name).Info("Bundle loader configuration changed. Restarting bundle loader.")
			}

			downloader := p.newDownloader(name, source, bundles)

			etag := p.readBundleEtagFromStore(ctx, name)
			downloader.SetCache(etag)

			p.downloaders[name] = downloader
			p.etags[name] = etag
			p.downloaders[name].Start(ctx)

			readyNow = false
		}
	}

	if !readyNow {
		p.ready = false
		p.manager.UpdatePluginStatus(Name, &plugins.Status{State: plugins.StateNotReady})
	}

	// Last, because a removal can complete the initial load: the activation
	// decides for itself whether the plugin is ready once it has run.
	if p.batching {
		if err := p.activateBatchIfComplete(ctx); err != nil {
			p.pluginLog().Debug("Batched bundle activation failed after a configuration change: %v", err)
		}

		// A download reports the status it produced; the initial load was
		// completed by the configuration change here, so the status goes out
		// without one.
		p.notifyBulkListeners()
	}
}

// Loaders returns the map of bundle loaders configured on this plugin.
func (p *Plugin) Loaders() map[string]Loader {
	return p.downloaders
}

// Trigger triggers a bundle download on all configured bundles.
func (p *Plugin) Trigger(ctx context.Context) error {
	var errs Errors

	p.mtx.Lock()
	downloaders := map[string]Loader{}
	maps.Copy(downloaders, p.downloaders)
	p.mtx.Unlock()

	for name, d := range downloaders {
		// plugin callback will also log the trigger error and include it in the bundle status
		err := d.Trigger(ctx)
		// only return errors for TriggerMode manual as periodic bundles will be retried
		if err != nil {
			trigger := p.Config().Bundles[name].Trigger
			if trigger != nil && *trigger == plugins.TriggerManual {
				errs = append(errs, NewBundleError(name, err))
			}
		}
	}
	if len(errs) > 0 {
		return errs
	}
	return nil
}

// Register a listener to receive status updates. The name must be comparable.
// The listener will receive a status update for each bundle configured, they are
// not going to be aggregated. For all status updates use `RegisterBulkListener`.
func (p *Plugin) Register(name any, listener func(Status)) {
	p.mtx.Lock()
	defer p.mtx.Unlock()

	if p.listeners == nil {
		p.listeners = map[any]func(Status){}
	}

	p.listeners[name] = listener
}

// Unregister a listener to stop receiving status updates.
func (p *Plugin) Unregister(name any) {
	p.mtx.Lock()
	defer p.mtx.Unlock()

	delete(p.listeners, name)
}

// RegisterBulkListener registers a listener to receive bulk (aggregated) status updates. The name must be comparable.
func (p *Plugin) RegisterBulkListener(name any, listener func(map[string]*Status)) {
	p.mtx.Lock()
	defer p.mtx.Unlock()

	if p.bulkListeners == nil {
		p.bulkListeners = map[any]func(map[string]*Status){}
	}

	p.bulkListeners[name] = listener
}

// UnregisterBulkListener unregisters a listener to stop receiving aggregated status updates.
func (p *Plugin) UnregisterBulkListener(name any) {
	p.mtx.Lock()
	defer p.mtx.Unlock()

	delete(p.bulkListeners, name)
}

// Config returns the plugins current configuration
func (p *Plugin) Config() *Config {
	p.cfgMtx.RLock()
	defer p.cfgMtx.RUnlock()
	return &Config{
		Name:                  p.config.Name,
		Bundles:               p.getBundlesCpy(),
		BatchBundleActivation: p.config.BatchBundleActivation,
	}
}

func (p *Plugin) initDownloaders(ctx context.Context) {
	bundles := p.getBundlesCpy()

	// Initialize a downloader for each bundle configured.
	for name, source := range bundles {
		downloader := p.newDownloader(name, source, bundles)

		etag := p.readBundleEtagFromStore(ctx, name)
		downloader.SetCache(etag)

		p.downloaders[name] = downloader
		p.etags[name] = etag
	}
}

func (p *Plugin) readBundleEtagFromStore(ctx context.Context, name string) string {
	var etag string
	err := storage.Txn(ctx, p.manager.Store, storage.TransactionParams{}, func(txn storage.Transaction) error {
		var loadErr error
		etag, loadErr = bundle.ReadBundleEtagFromStore(ctx, p.manager.Store, txn, name)
		if loadErr != nil && !storage.IsNotFound(loadErr) {
			p.log(name).Error("Failed to load bundle etag from store: %v", loadErr)
			return loadErr
		}
		return nil
	})
	if err != nil {
		// TODO: This probably shouldn't panic. But OPA shouldn't
		// continue in a potentially inconsistent state.
		panic(errors.New("Unable to load bundle etag from store: " + err.Error()))
	}

	return etag
}

func (p *Plugin) loadAndActivateBundlesFromDisk(ctx context.Context) {
	persistedBundles := map[string]*bundle.Bundle{}

	bundles := p.getBundlesCpy()

	p.cfgMtx.RLock()
	isMultiBundle := p.config.IsMultiBundle()
	p.cfgMtx.RUnlock()

	for name, src := range bundles {
		if p.persistBundle(name, bundles) {
			b, err := p.loadBundleFromDisk(p.bundlePersistPath, name, src)
			if err != nil {
				p.log(name).Error("Failed to load bundle from disk: %v", err)
				p.status[name].SetError(err)
				continue
			}

			if b == nil {
				continue
			}

			persistedBundles[name] = b
		}
	}

	if len(persistedBundles) == 0 {
		return
	}

	p.cfgMtx.RLock()
	batchBundleActivation := p.config.BatchBundleActivation
	p.cfgMtx.RUnlock()

	// Every persisted bundle is in hand, so they can be activated together. If
	// that fails, the loop below activates them one at a time, which reports the
	// error against the bundle that caused it and retries around any bundle
	// ordering.
	if batchBundleActivation {
		for name, b := range persistedBundles {
			p.status[name].Metrics = metrics.New()
			p.status[name].Type = b.Type()
		}

		err := p.activate(ctx, persistedBundles, isMultiBundle)
		if err == nil {
			for name, b := range persistedBundles {
				p.status[name].SetError(nil)
				p.status[name].SetActivateSuccess(b.Manifest.Revision)
				p.log(name).Debug("Bundle loaded from disk and activated successfully.")
			}
			p.checkPluginReadiness()
			return
		}

		p.pluginLog().Info("Batched bundle activation failed, activating bundles individually: %v", err)
	}

	for range maxActivationRetry {

		numActivatedBundles := 0
		for name, b := range persistedBundles {
			p.status[name].Metrics = metrics.New()
			p.status[name].Type = b.Type()

			err := p.activate(ctx, map[string]*bundle.Bundle{name: b}, isMultiBundle)
			if err != nil {
				p.log(name).Error("Bundle activation failed: %v", err)
				p.status[name].SetError(err)
				continue
			}

			p.status[name].SetError(nil)
			p.status[name].SetActivateSuccess(b.Manifest.Revision)

			p.checkPluginReadiness()

			p.log(name).Debug("Bundle loaded from disk and activated successfully.")
			numActivatedBundles++
		}

		if numActivatedBundles == len(persistedBundles) {
			return
		}
	}
}

func (p *Plugin) newDownloader(name string, source *Source, bundles map[string]*Source) Loader {
	if u, err := url.Parse(source.Resource); err == nil && u.Scheme == "file" {
		return &fileLoader{
			name:             name,
			path:             u.Path,
			bvc:              source.Signing,
			sizeLimitBytes:   source.SizeLimitBytes,
			f:                p.oneShot,
			bundleParserOpts: p.manager.ParserOptions(),
		}
	}

	conf := source.Config
	client := p.manager.Client(source.Service)
	path := source.Resource
	callback := func(ctx context.Context, u download.Update) error {
		// wrap the callback to include the name of the bundle that was updated
		return p.oneShot(ctx, name, u)
	}
	if strings.ToLower(client.Config().Type) == "oci" {
		ociStorePath := ""
		if cfg := p.manager.GetConfig(); cfg.PersistenceDirectory != nil {
			ociStorePath = filepath.Join(*cfg.PersistenceDirectory, "oci")
		}
		return download.NewOCI(conf, client, path, ociStorePath).
			WithCallback(callback).
			WithBundleVerificationConfig(source.Signing).
			WithSizeLimitBytes(source.SizeLimitBytes).
			WithBundlePersistence(p.persistBundle(name, bundles)).
			WithBundleParserOpts(p.manager.ParserOptions())
	}
	return download.New(conf, client, path).
		WithCallback(callback).
		WithBundleVerificationConfig(source.Signing).
		WithSizeLimitBytes(source.SizeLimitBytes).
		WithBundlePersistence(p.persistBundle(name, bundles)).
		WithLazyLoadingMode(true).
		WithBundleName(name).
		WithBundleParserOpts(p.manager.ParserOptions())
}

func (p *Plugin) oneShot(ctx context.Context, name string, u download.Update) error {
	p.mtx.Lock()
	defer p.mtx.Unlock()

	err := p.process(ctx, name, u)

	for _, listener := range p.listeners {
		listener(*p.status[name])
	}

	p.notifyBulkListeners()

	return err
}

// notifyBulkListeners sends the current status of every bundle to the bulk
// listeners, which is the listener a multi-bundle configuration registers, and
// so the one an activation has to be reported through.
func (p *Plugin) notifyBulkListeners() {
	for _, listener := range p.bulkListeners {
		// Send a copy of the full status map to the bulk listeners.
		// They shouldn't have access to the original underlying
		// map, primarily for thread safety issues with modifications
		// made to it.
		statusCpy := map[string]*Status{}
		for k, v := range p.status {
			v := *v
			statusCpy[k] = &v
		}
		listener(statusCpy)
	}
}

func (p *Plugin) process(ctx context.Context, name string, u download.Update) error {
	if u.Metrics != nil {
		p.status[name].Metrics = u.Metrics
	} else {
		p.status[name].Metrics = metrics.New()
	}

	p.status[name].SetRequest()

	if u.Error != nil {
		p.log(name).Error("Bundle load failed: %v", u.Error)
		p.status[name].SetError(u.Error)
		if !p.stopped {
			etag := p.etags[name]
			p.downloaders[name].SetCache(etag)
		}

		if err := p.settleBatch(ctx, name, u); err != nil {
			return errors.Join(u.Error, err)
		}
		return u.Error
	}

	p.status[name].LastSuccessfulRequest = p.status[name].LastRequest

	if u.Bundle != nil {
		p.status[name].Type = u.Bundle.Type()
		p.status[name].LastSuccessfulDownload = p.status[name].LastSuccessfulRequest

		p.status[name].Metrics.Timer(metrics.RegoLoadBundles).Start()
		defer p.status[name].Metrics.Timer(metrics.RegoLoadBundles).Stop()

		p.cfgMtx.RLock()
		isMultiBundle := p.config.IsMultiBundle()
		p.cfgMtx.RUnlock()

		if p.batching {
			return p.settleBatch(ctx, name, u)
		}

		if err := p.activate(ctx, map[string]*bundle.Bundle{name: u.Bundle}, isMultiBundle); err != nil {
			p.log(name).Error("Bundle activation failed: %v", err)
			p.status[name].SetError(err)
			if !p.stopped {
				etag := p.etags[name]
				p.downloaders[name].SetCache(etag)
			}
			return err
		}

		if err := p.recordActivation(name, u); err != nil {
			return err
		}

		// If the plugin wasn't ready yet then check if we are now after activating this bundle.
		p.checkPluginReadiness()
		return nil
	}

	if etag, ok := p.etags[name]; ok && u.ETag == etag {
		p.log(name).Debug("Bundle load skipped, server replied with not modified.")
		p.status[name].SetError(nil)

		// The downloader received a 304 (same etag as saved in local state), update plugin readiness
		p.checkPluginReadiness()
	}

	return p.settleBatch(ctx, name, u)
}

func (p *Plugin) checkPluginReadiness() {
	if !p.ready {
		readyNow := true // optimistically
		for _, status := range p.status {
			if len(status.Errors) > 0 || status.LastSuccessfulActivation.IsZero() {
				readyNow = false // Not ready yet, check again on next bundle activation.
				break
			}
		}

		if readyNow {
			p.ready = true
			p.manager.UpdatePluginStatus(Name, &plugins.Status{State: plugins.StateOK})
		}
	}
}

// activate compiles and activates the given bundles in a single storage
// transaction, so that their modules are compiled together instead of once per
// bundle.
func (p *Plugin) activate(ctx context.Context, bundles map[string]*bundle.Bundle, isMultiBundle bool) error {
	for name, b := range bundles {
		p.log(name).Debug("Bundle activation in progress (%v). Opening storage transaction.", b.Manifest.Revision)
	}

	m := p.activationMetrics(bundles)

	params := storage.WriteParams
	params.Context = storage.NewContext().WithMetrics(m)

	err := storage.Txn(ctx, p.manager.Store, params, func(txn storage.Transaction) error {
		p.pluginLog().Debug("Opened storage transaction (%v).", txn.ID())
		defer p.pluginLog().Debug("Closing storage transaction (%v).", txn.ID())

		// Compile the bundle modules with a new compiler and set it on the
		// transaction params for use by onCommit hooks.
		// If activating a delta bundle, use the manager's compiler which should have
		// the polices compiled on it.
		var compiler *ast.Compiler
		if hasDeltaBundle(bundles) {
			compiler = p.manager.GetCompiler()
		}

		if compiler == nil {
			compiler = ast.NewCompiler()
		}

		compiler = compiler.WithPathConflictsCheck(storage.NonEmpty(ctx, p.manager.Store, txn)).
			WithEnablePrintStatements(p.manager.EnablePrintStatements())

		var roots []string
		for _, b := range bundles {
			if b.Manifest.Roots != nil {
				roots = append(roots, *b.Manifest.Roots...)
			}
		}
		if len(roots) > 0 {
			compiler = compiler.WithPathConflictsCheckRoots(roots)
		}

		var activateErr error

		// Call pre-activation hooks so plugins can inspect the bundle manifest
		// and register external sources before compilation.
		p.manager.Hooks().Each(func(h hooks.Hook) {
			if f, ok := h.(hooks.BundlePreActivateHook); ok {
				for name, b := range bundles {
					if err := f.OnBundlePreActivate(ctx, name, b.Manifest); err != nil {
						p.log(name).Warn("Pre-activation hook failed: %v", err)
					}
				}
			}
		})

		opts := &bundle.ActivateOpts{
			Ctx:             ctx,
			Store:           p.manager.Store,
			Txn:             txn,
			TxnCtx:          params.Context,
			Compiler:        compiler,
			Metrics:         m,
			Bundles:         bundles,
			ExternalSources: p.manager.GetExternalSources(),
			ParserOptions:   p.manager.ParserOptions(),
		}

		if p.manager.Info != nil {

			skipKnownSchemaCheck := p.manager.Info.Get(ast.StringTerm("skip_known_schema_check"))
			isAuthzEnabled := p.manager.Info.Get(ast.StringTerm("authorization_enabled"))

			if ast.BooleanTerm(true).Equal(isAuthzEnabled) && ast.BooleanTerm(false).Equal(skipKnownSchemaCheck) {
				authorizationDecisionRef, err := ref.ParseDataPath(*p.manager.GetConfig().DefaultAuthorizationDecision)
				if err != nil {
					return err
				}
				opts.AuthorizationDecisionRef = authorizationDecisionRef
			}
		}

		if isMultiBundle {
			activateErr = bundle.Activate(opts)
		} else {
			activateErr = bundle.ActivateLegacy(opts) //nolint:staticcheck
		}

		plugins.SetCompilerOnContext(params.Context, compiler)

		resolvers, err := bundleUtils.LoadWasmResolversFromStore(ctx, p.manager.Store, txn, nil)
		if err != nil {
			return err
		}

		plugins.SetWasmResolversOnContext(params.Context, resolvers)

		return activateErr
	})

	return err
}

// activationMetrics returns the metrics an activation is recorded against. A
// single bundle is recorded against its own status, which is what activating one
// bundle has always done. A batch spans several bundles, so it gets a set of its
// own rather than being charged to one of them arbitrarily.
func (p *Plugin) activationMetrics(bundles map[string]*bundle.Bundle) metrics.Metrics {
	if len(bundles) == 1 {
		for name := range bundles {
			return p.status[name].Metrics
		}
	}
	return metrics.New()
}

// hasDeltaBundle reports whether any of the bundles is a delta bundle. A delta
// bundle patches the modules already in the store, so it is compiled against the
// manager's compiler instead of a fresh one.
func hasDeltaBundle(bundles map[string]*bundle.Bundle) bool {
	for _, b := range bundles {
		if b.Type() == bundle.DeltaBundleType {
			return true
		}
	}
	return false
}

// settleBatch records that the bundle has produced a download result and
// activates the collected bundles once the initial load has nothing left to
// wait for. Any result settles a bundle: a failed download reports an error of
// its own rather than holding the others back, and a bundle the server has no
// new revision for has nothing to activate in the first place.
func (p *Plugin) settleBatch(ctx context.Context, name string, u download.Update) error {
	if !p.batching {
		return nil
	}

	p.batchedReported[name] = struct{}{}
	if u.Bundle != nil {
		p.batchedBundles[name] = u
	}

	return p.activateBatchIfComplete(ctx)
}

// activateBatchIfComplete activates the collected bundles once the initial load
// has nothing left to wait for. Taking a bundle out of the configuration can be
// the last thing the load was waiting for, so this is not only reached from a
// download.
func (p *Plugin) activateBatchIfComplete(ctx context.Context) error {
	if !p.batching || !p.batchComplete() {
		return nil
	}

	p.cfgMtx.RLock()
	isMultiBundle := p.config.IsMultiBundle()
	p.cfgMtx.RUnlock()

	return p.activateBatch(ctx, isMultiBundle)
}

// dropFromBatch forgets a bundle the initial load is holding. Its configuration
// changed or it is gone, so a download taken before that no longer describes
// what the bundle is, and it must not settle the load on its own.
func (p *Plugin) dropFromBatch(name string) {
	delete(p.batchedBundles, name)
	delete(p.batchedReported, name)
}

// batchComplete reports whether the initial load has nothing left to wait for.
// A bundle is done with once it has reported a download result, or once it has
// already activated from disk and so has nothing left to contribute. It reads
// p.status rather than counting, because configuring a bundle adds it to the
// load and removing one takes it out again while the load is running.
func (p *Plugin) batchComplete() bool {
	for name, status := range p.status {
		if _, ok := p.batchedReported[name]; ok {
			continue
		}
		if !status.LastSuccessfulActivation.IsZero() {
			continue
		}
		return false
	}
	return true
}

// activateBatch activates the bundles collected during the initial load. The
// plugin is ready to serve once every configured bundle has activated, so they
// are activated together and their modules compiled once instead of once per
// bundle.
func (p *Plugin) activateBatch(ctx context.Context, isMultiBundle bool) error {
	updates := p.batchedBundles
	p.batching = false
	p.batchedBundles = nil
	p.batchedReported = nil

	// A bundle that is no longer configured is not part of the load, and
	// activating it would put back what removing it erased.
	bundles := make(map[string]*bundle.Bundle, len(updates))
	for name, u := range updates {
		if _, ok := p.status[name]; ok {
			bundles[name] = u.Bundle
		}
	}

	if len(bundles) == 0 {
		p.checkPluginReadiness()
		return nil
	}

	// bundle.Activate is all or nothing: one invalid bundle fails the batch and
	// no bundle reaches the store. A delta bundle patches what is already there
	// rather than replacing it. Both are activated one bundle at a time instead,
	// which is what the initial load does when it is not batched, so that a bad
	// bundle only keeps itself out of the store and reports its own error.
	individually := hasDeltaBundle(bundles)
	if !individually {
		if err := p.activate(ctx, bundles, isMultiBundle); err != nil {
			p.pluginLog().Info("Batched bundle activation failed, activating bundles individually: %v", err)
			individually = true
		}
	}

	var errs Errors

	for name, u := range updates {
		if _, ok := p.status[name]; !ok {
			continue
		}

		if individually {
			if err := p.activate(ctx, map[string]*bundle.Bundle{name: u.Bundle}, isMultiBundle); err != nil {
				p.log(name).Error("Bundle activation failed: %v", err)
				p.status[name].SetError(err)
				if !p.stopped {
					etag := p.etags[name]
					p.downloaders[name].SetCache(etag)
				}
				errs = append(errs, NewBundleError(name, err))
				continue
			}
		}

		if err := p.recordActivation(name, u); err != nil {
			errs = append(errs, NewBundleError(name, err))
			continue
		}
		p.log(name).Debug("Bundle activated as part of the initial load.")
	}

	p.checkPluginReadiness()

	if len(errs) == 0 {
		return nil
	}
	return errs
}

// recordActivation persists the bundle to disk when its source asks for it, and
// records the successful activation against the bundle's status. Activation
// itself has already happened by the time this runs.
func (p *Plugin) recordActivation(name string, u download.Update) error {
	if u.Bundle.Type() == bundle.SnapshotBundleType && p.persistBundle(name, p.getBundlesCpy()) {
		p.log(name).Debug("Persisting bundle to disk in progress.")

		if err := p.saveBundleToDisk(name, u.Raw); err != nil {
			p.log(name).Error("Persisting bundle to disk failed: %v", err)
			p.status[name].SetError(err)
			if !p.stopped {
				etag := p.etags[name]
				p.downloaders[name].SetCache(etag)
			}
			return err
		}
		p.log(name).Debug("Bundle persisted to disk successfully at path %v.", filepath.Join(p.bundlePersistPath, name))
	}

	p.status[name].SetError(nil)
	p.status[name].SetActivateSuccess(u.Bundle.Manifest.Revision)
	p.status[name].SetBundleSize(u.Size)

	if u.ETag != "" {
		p.log(name).Info("Bundle loaded and activated successfully. Etag updated to %v.", u.ETag)
	} else {
		p.log(name).Info("Bundle loaded and activated successfully.")
	}
	p.etags[name] = u.ETag

	return nil
}

func (*Plugin) persistBundle(name string, bundles map[string]*Source) bool {
	bundleSrc := bundles[name]

	if bundleSrc == nil {
		return false
	}
	return bundleSrc.Persist
}

// configDelta will return a map of new bundle sources, updated bundle sources, and a set of deleted bundle names
func (p *Plugin) configDelta(newConfig *Config) (map[string]*Source, map[string]*Source, map[string]struct{}) {
	deletedBundles := map[string]struct{}{}

	// p.cfgMtx lock held at calling site, so we don't need
	// to get a copy of the bundles map here
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
			// The downloader holds the client it was built with, so a service
			// re-registered under the same name counts as a change even when the
			// bundle's own configuration is untouched.
			if !reflect.DeepEqual(oldSource, source) || !p.clientChanged(name, source) {
				updatedBundles[name] = source
			}
		}
	}

	return newBundles, updatedBundles, deletedBundles
}

// clientChanged reports whether the service client a bundle would be downloaded
// with now matches the one its downloader was built with.
func (p *Plugin) clientChanged(name string, source *Source) bool {
	old, ok := p.clientConfigs[name]
	return ok && old.Equal(p.manager.Client(source.Service).Config())
}

// clientConfigs snapshots the service client configuration behind each bundle.
func clientConfigs(manager *plugins.Manager, bundles map[string]*Source) map[string]*rest.Config {
	configs := make(map[string]*rest.Config, len(bundles))
	for name, source := range bundles {
		if source != nil {
			configs[name] = manager.Client(source.Service).Config()
		}
	}
	return configs
}

func (p *Plugin) saveBundleToDisk(name string, raw io.Reader) error {
	bundleName := getNormalizedBundleName(name)

	bundleDir := filepath.Join(p.bundlePersistPath, bundleName)
	bundleFile := filepath.Join(bundleDir, "bundle.tar.gz")

	tmpFile, saveErr := saveCurrentBundleToDisk(bundleDir, raw)
	if saveErr != nil {
		p.log(name).Error("Failed to save new bundle to disk: %v", saveErr)

		if err := os.Remove(tmpFile); err != nil {
			p.log(name).Warn("Failed to remove temp file ('%s'): %v", tmpFile, err)
		}

		if _, err := os.Stat(bundleFile); err == nil {
			p.log(name).Warn("Older version of activated bundle persisted, ignoring error")
			return nil
		}
		return saveErr
	}

	return os.Rename(tmpFile, bundleFile)
}

func saveCurrentBundleToDisk(path string, raw io.Reader) (string, error) {
	return bundleUtils.SaveBundleToDisk(path, raw)
}

func (p *Plugin) loadBundleFromDisk(path, name string, src *Source) (*bundle.Bundle, error) {
	bundleName := getNormalizedBundleName(name)

	if src != nil {
		return bundleUtils.LoadBundleFromDiskForRegoVersion(p.manager.ParserOptions().RegoVersion, path, bundleName, src.Signing)
	}
	return bundleUtils.LoadBundleFromDiskForRegoVersion(p.manager.ParserOptions().RegoVersion, path, bundleName, nil)
}

func (p *Plugin) log(name string) logging.Logger {
	return p.pluginLog().WithFields(map[string]any{"name": name, "plugin": Name})
}

// pluginLog returns the logger of the plugin itself, for the messages that are
// not about a single bundle. It falls back to the global logger for the plugins
// that were not built by New.
func (p *Plugin) pluginLog() logging.Logger {
	if p.logger == nil {
		p.logger = logging.Get()
	}
	return p.logger
}

func (p *Plugin) getBundlePersistPath() (string, error) {
	persistDir, err := p.manager.GetConfig().GetPersistenceDirectory()
	if err != nil {
		return "", err
	}

	return filepath.Join(persistDir, "bundles"), nil
}

func (p *Plugin) getBundlesCpy() map[string]*Source {
	p.cfgMtx.RLock()
	defer p.cfgMtx.RUnlock()
	bundlesCpy := map[string]*Source{}
	for k, v := range p.config.Bundles {
		v := *v
		bundlesCpy[k] = &v
	}
	return bundlesCpy
}

// getNormalizedBundleName returns a version of the input with
// invalid file and directory name characters on Windows escaped.
// It returns the input as-is for non-Windows systems.
func getNormalizedBundleName(name string) string {
	if goos != "windows" {
		return name
	}

	sb := new(strings.Builder)
	for i := range len(name) {
		if isReservedCharacter(rune(name[i])) {
			fmt.Fprintf(sb, "\\%c", name[i])
		} else {
			sb.WriteByte(name[i])
		}
	}

	return sb.String()
}

// isReservedCharacter checks if the input is a reserved character on Windows that should not be
// used in file and directory names
// For details, see https://learn.microsoft.com/en-us/windows/win32/fileio/naming-a-file#naming-conventions.
func isReservedCharacter(r rune) bool {
	return r == '<' || r == '>' || r == ':' || r == '"' || r == '/' || r == '\\' || r == '|' || r == '?' || r == '*'
}

type fileLoader struct {
	name             string
	path             string
	bvc              *bundle.VerificationConfig
	sizeLimitBytes   int64
	f                func(context.Context, string, download.Update) error
	bundleParserOpts ast.ParserOptions
}

func (fl *fileLoader) Start(ctx context.Context) {
	go func() {
		_ = fl.oneShot(ctx)
	}()
}

func (*fileLoader) Stop(context.Context) {
}

func (*fileLoader) ClearCache() {
}

func (*fileLoader) SetCache(string) {
}

func (fl *fileLoader) Trigger(ctx context.Context) error {
	return fl.oneShot(ctx)
}

func (fl *fileLoader) oneShot(ctx context.Context) (err error) {
	var u download.Update
	u.Metrics = metrics.New()

	info, err := os.Stat(fl.path)
	u.Error = err
	if err != nil {
		return fl.f(ctx, fl.name, u)
	}

	var reader *bundle.Reader

	if info.IsDir() {
		reader = bundle.NewCustomReader(bundle.NewDirectoryLoader(fl.path))
	} else {
		var f *os.File
		f, err = os.Open(fl.path)
		u.Error = err
		if err != nil {
			return fl.f(ctx, fl.name, u)
		}
		defer func(f *os.File) {
			err = errors.Join(err, f.Close())
		}(f)
		reader = bundle.NewReader(f)
	}

	b, err := reader.
		WithMetrics(u.Metrics).
		WithBundleVerificationConfig(fl.bvc).
		WithLazyLoadingMode(bundle.HasExtension()).
		WithSizeLimitBytes(fl.sizeLimitBytes).
		WithRegoVersion(fl.bundleParserOpts.RegoVersion).
		WithProcessAnnotations(fl.bundleParserOpts.ProcessAnnotation).
		Read()
	u.Error = err
	if err == nil {
		u.Bundle = &b
	}
	return fl.f(ctx, fl.name, u)
}
