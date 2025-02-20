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
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"time"

	bundleUtils "github.com/open-policy-agent/opa/internal/bundle"
	"github.com/open-policy-agent/opa/internal/ref"
	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/bundle"
	"github.com/open-policy-agent/opa/v1/download"
	"github.com/open-policy-agent/opa/v1/logging"
	"github.com/open-policy-agent/opa/v1/metrics"
	"github.com/open-policy-agent/opa/v1/plugins"
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
	manager           *plugins.Manager                         // plugin manager for storage and service clients
	status            map[string]*Status                       // current status for each bundle
	etags             map[string]string                        // etag on last successful activation
	listeners         map[interface{}]func(Status)             // listeners to send status updates to
	bulkListeners     map[interface{}]func(map[string]*Status) // listeners to send aggregated status updates to
	downloaders       map[string]Loader
	logger            logging.Logger
	mtx               sync.Mutex
	cfgMtx            sync.RWMutex
	ready             bool
	bundlePersistPath string
	stopped           bool
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
		downloaders: make(map[string]Loader),
		etags:       make(map[string]string),
		ready:       false,
		logger:      manager.Logger(),
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
	for name, dl := range p.downloaders {
		stopDownloaders[name] = dl
	}
	p.downloaders = nil
	p.stopped = true
	p.mtx.Unlock()

	for name, dl := range stopDownloaders {
		p.log(name).Info("Stopping bundle loader.")
		dl.Stop(ctx)
	}
}

// Reconfigure notifies the plugin that it's configuration has changed.
// Any bundle configs that have changed or been added/removed will take
// affect.
func (p *Plugin) Reconfigure(ctx context.Context, config interface{}) {
	// Reconfiguring should not occur in parallel, lock to ensure
	// nothing swaps underneath us with the current p.config and the updated one.
	// Use p.cfgMtx instead of p.mtx to not block any bundle downloads/activations
	// that are in progress. We upgrade to p.mtx locking after stopping downloaders.
	p.cfgMtx.Lock()

	// Look for any bundles that have had their config changed, are new, or have been removed
	newConfig := config.(*Config)
	newBundles, updatedBundles, deletedBundles := p.configDelta(newConfig)
	p.config = *newConfig

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
	for name, dl := range p.downloaders {
		downloaders[name] = dl
	}
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

	delete(p.listeners, name)
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
	p.cfgMtx.RLock()
	defer p.cfgMtx.RUnlock()
	return &Config{
		Name:    p.config.Name,
		Bundles: p.getBundlesCpy(),
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

	for range maxActivationRetry {

		numActivatedBundles := 0
		for name, b := range persistedBundles {
			p.status[name].Metrics = metrics.New()
			p.status[name].Type = b.Type()

			err := p.activate(ctx, name, b, isMultiBundle)
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
	callback := func(ctx context.Context, u download.Update) {
		// wrap the callback to include the name of the bundle that was updated
		p.oneShot(ctx, name, u)
	}
	if strings.ToLower(client.Config().Type) == "oci" {
		ociStorePath := filepath.Join(os.TempDir(), "opa", "oci") // use temporary folder /tmp/opa/oci
		if p.manager.Config.PersistenceDirectory != nil {
			ociStorePath = filepath.Join(*p.manager.Config.PersistenceDirectory, "oci")
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

func (p *Plugin) oneShot(ctx context.Context, name string, u download.Update) {
	p.mtx.Lock()
	defer p.mtx.Unlock()

	p.process(ctx, name, u)

	for _, listener := range p.listeners {
		listener(*p.status[name])
	}

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

func (p *Plugin) process(ctx context.Context, name string, u download.Update) {

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
		return
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

		if err := p.activate(ctx, name, u.Bundle, isMultiBundle); err != nil {
			p.log(name).Error("Bundle activation failed: %v", err)
			p.status[name].SetError(err)
			if !p.stopped {
				etag := p.etags[name]
				p.downloaders[name].SetCache(etag)
			}
			return
		}

		if u.Bundle.Type() == bundle.SnapshotBundleType && p.persistBundle(name, p.getBundlesCpy()) {
			p.log(name).Debug("Persisting bundle to disk in progress.")

			err := p.saveBundleToDisk(name, u.Raw)
			if err != nil {
				p.log(name).Error("Persisting bundle to disk failed: %v", err)
				p.status[name].SetError(err)
				if !p.stopped {
					etag := p.etags[name]
					p.downloaders[name].SetCache(etag)
				}
				return
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

		// If the plugin wasn't ready yet then check if we are now after activating this bundle.
		p.checkPluginReadiness()
		return
	}

	if etag, ok := p.etags[name]; ok && u.ETag == etag {
		p.log(name).Debug("Bundle load skipped, server replied with not modified.")
		p.status[name].SetError(nil)

		// The downloader received a 304 (same etag as saved in local state), update plugin readiness
		p.checkPluginReadiness()
		return
	}
}

func (p *Plugin) checkPluginReadiness() {
	if !p.ready {
		readyNow := true // optimistically
		for _, status := range p.status {
			if len(status.Errors) > 0 || (status.LastSuccessfulActivation == time.Time{}) {
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

func (p *Plugin) activate(ctx context.Context, name string, b *bundle.Bundle, isMultiBundle bool) error {
	p.log(name).Debug("Bundle activation in progress (%v). Opening storage transaction.", b.Manifest.Revision)

	params := storage.WriteParams
	params.Context = storage.NewContext().WithMetrics(p.status[name].Metrics)

	err := storage.Txn(ctx, p.manager.Store, params, func(txn storage.Transaction) error {
		p.log(name).Debug("Opened storage transaction (%v).", txn.ID())
		defer p.log(name).Debug("Closing storage transaction (%v).", txn.ID())

		// Compile the bundle modules with a new compiler and set it on the
		// transaction params for use by onCommit hooks.
		// If activating a delta bundle, use the manager's compiler which should have
		// the polices compiled on it.
		var compiler *ast.Compiler
		if b.Type() == bundle.DeltaBundleType {
			compiler = p.manager.GetCompiler()
		}

		if compiler == nil {
			compiler = ast.NewCompiler()
		}

		compiler = compiler.WithPathConflictsCheck(storage.NonEmpty(ctx, p.manager.Store, txn)).
			WithEnablePrintStatements(p.manager.EnablePrintStatements())

		if b.Manifest.Roots != nil {
			compiler = compiler.WithPathConflictsCheckRoots(*b.Manifest.Roots)
		}

		var activateErr error

		opts := &bundle.ActivateOpts{
			Ctx:           ctx,
			Store:         p.manager.Store,
			Txn:           txn,
			TxnCtx:        params.Context,
			Compiler:      compiler,
			Metrics:       p.status[name].Metrics,
			Bundles:       map[string]*bundle.Bundle{name: b},
			ParserOptions: p.manager.ParserOptions(),
		}

		if p.manager.Info != nil {

			skipKnownSchemaCheck := p.manager.Info.Get(ast.StringTerm("skip_known_schema_check"))
			isAuthzEnabled := p.manager.Info.Get(ast.StringTerm("authorization_enabled"))

			if ast.BooleanTerm(true).Equal(isAuthzEnabled) && ast.BooleanTerm(false).Equal(skipKnownSchemaCheck) {
				authorizationDecisionRef, err := ref.ParseDataPath(*p.manager.Config.DefaultAuthorizationDecision)
				if err != nil {
					return err
				}
				opts.AuthorizationDecisionRef = authorizationDecisionRef
			}
		}

		if isMultiBundle {
			activateErr = bundle.Activate(opts)
		} else {
			activateErr = bundle.ActivateLegacy(opts)
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

func (p *Plugin) persistBundle(name string, bundles map[string]*Source) bool {
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
			if !reflect.DeepEqual(oldSource, source) {
				updatedBundles[name] = source
			}
		}
	}

	return newBundles, updatedBundles, deletedBundles
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
	if p.logger == nil {
		p.logger = logging.Get()
	}
	return p.logger.WithFields(map[string]interface{}{"name": name, "plugin": Name})
}

func (p *Plugin) getBundlePersistPath() (string, error) {
	persistDir, err := p.manager.Config.GetPersistenceDirectory()
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
			sb.WriteString(fmt.Sprintf("\\%c", name[i]))
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
	f                func(context.Context, string, download.Update)
	bundleParserOpts ast.ParserOptions
}

func (fl *fileLoader) Start(ctx context.Context) {
	go func() {
		fl.oneShot(ctx)
	}()
}

func (*fileLoader) Stop(context.Context) {

}

func (*fileLoader) ClearCache() {

}

func (*fileLoader) SetCache(string) {

}

func (fl *fileLoader) Trigger(ctx context.Context) error {
	fl.oneShot(ctx)
	return nil
}

func (fl *fileLoader) oneShot(ctx context.Context) {
	var u download.Update
	u.Metrics = metrics.New()

	info, err := os.Stat(fl.path)
	u.Error = err
	if err != nil {
		fl.f(ctx, fl.name, u)
		return
	}

	var reader *bundle.Reader

	if info.IsDir() {
		reader = bundle.NewCustomReader(bundle.NewDirectoryLoader(fl.path))
	} else {
		f, err := os.Open(fl.path)
		u.Error = err
		if err != nil {
			fl.f(ctx, fl.name, u)
			return
		}
		defer f.Close()
		reader = bundle.NewReader(f)
	}

	b, err := reader.
		WithMetrics(u.Metrics).
		WithBundleVerificationConfig(fl.bvc).
		WithSizeLimitBytes(fl.sizeLimitBytes).
		WithRegoVersion(fl.bundleParserOpts.RegoVersion).
		Read()
	u.Error = err
	if err == nil {
		u.Bundle = &b
	}
	fl.f(ctx, fl.name, u)
}
