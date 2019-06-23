// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package bundle implements bundle downloading.
package bundle

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"sync"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/bundle"
	"github.com/open-policy-agent/opa/download"
	"github.com/open-policy-agent/opa/internal/manifest"
	"github.com/open-policy-agent/opa/plugins"
	"github.com/open-policy-agent/opa/storage"
	"github.com/sirupsen/logrus"
)

// Plugin implements bundle activation.
type Plugin struct {
	config     Config
	manager    *plugins.Manager             // plugin manager for storage and service clients
	status     *Status                      // current plugin status
	etag       string                       // etag on last successful activation
	listeners  map[interface{}]func(Status) // listeners to send status updates to
	downloader *download.Downloader
	mtx        sync.Mutex
}

// New returns a new Plugin with the given config.
func New(parsedConfig *Config, manager *plugins.Manager) *Plugin {
	p := &Plugin{
		manager: manager,
		config:  *parsedConfig,
		status: &Status{
			Name: parsedConfig.Name,
		},
	}
	p.initDownloader()
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
	p.logInfo("Starting bundle downloader.")
	p.mtx.Lock()
	defer p.mtx.Unlock()
	p.downloader.Start(ctx)
	return nil
}

// Stop stops the plugin.
func (p *Plugin) Stop(ctx context.Context) {
	p.logInfo("Stopping bundle downloader.")
	p.mtx.Lock()
	defer p.mtx.Unlock()
	p.downloader.Stop(ctx)
}

// Reconfigure notifies the plugin that it's configuration has changed.
func (p *Plugin) Reconfigure(ctx context.Context, config interface{}) {
	p.mtx.Lock()
	defer p.mtx.Unlock()

	newConfig := config.(*Config)
	if reflect.DeepEqual(p.config, *newConfig) {
		p.logDebug("Bundle downloader configuration unchanged.")
		return
	}

	p.logInfo("Bundle downloader configuration changed. Restarting bundle downloader.")
	p.config = *config.(*Config)
	p.downloader.Stop(ctx)
	p.initDownloader()
	p.downloader.Start(ctx)
}

// Register a listener to receive status updates. The name must be comparable.
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

func (p *Plugin) initDownloader() {
	client := p.manager.Client(p.config.Service)
	path := p.generateDownloadPath(*(p.config.Prefix), p.config.Name)
	p.downloader = download.New(p.config.Config, client, path).WithCallback(p.oneShot)
}

func (p *Plugin) generateDownloadPath(prefix string, name string) string {
	res := ""
	trimmedPrefix := strings.Trim(prefix, "/")
	if trimmedPrefix != "" {
		res += trimmedPrefix + "/"
	}

	res += strings.Trim(name, "/")

	return res
}

func (p *Plugin) oneShot(ctx context.Context, u download.Update) {
	p.mtx.Lock()
	defer p.mtx.Unlock()

	p.process(ctx, u)
	status := *p.status

	for _, listener := range p.listeners {
		listener(status)
	}
}

func (p *Plugin) process(ctx context.Context, u download.Update) {

	if u.Error != nil {
		p.logError("Bundle download failed: %v", u.Error)
		p.status.SetError(u.Error)
		return
	}

	if u.Bundle != nil {
		p.status.SetDownloadSuccess()

		if err := p.activate(ctx, u.Bundle); err != nil {
			p.logError("Bundle activation failed: %v", err)
			p.status.SetError(err)
			return
		}

		p.status.SetError(nil)
		p.status.SetActivateSuccess(u.Bundle.Manifest.Revision)
		if u.ETag != "" {
			p.logInfo("Bundle downloaded and activated successfully. Etag updated to %v.", u.ETag)
		} else {
			p.logInfo("Bundle downloaded and activated successfully.")
		}
		p.etag = u.ETag
		return
	}

	if u.ETag == p.etag {
		p.logDebug("Bundle download skipped, server replied with not modified.")
		p.status.SetError(nil)
		return
	}
}

func (p *Plugin) activate(ctx context.Context, b *bundle.Bundle) error {
	p.logDebug("Bundle activation in progress. Opening storage transaction.")

	params := storage.WriteParams
	params.Context = storage.NewContext()

	return storage.Txn(ctx, p.manager.Store, params, func(txn storage.Transaction) error {
		p.logDebug("Opened storage transaction (%v).", txn.ID())
		defer p.logDebug("Closing storage transaction (%v).", txn.ID())

		// Build set of roots from old and new bundles. This set of
		// roots should be erased.
		erase := map[string]struct{}{}

		if b.Manifest.Roots != nil {
			for _, root := range *b.Manifest.Roots {
				erase[root] = struct{}{}
			}
		}

		if roots, err := manifest.ReadBundleRoots(ctx, p.manager.Store, txn); err == nil {
			for _, root := range roots {
				erase[root] = struct{}{}
			}
		} else if !storage.IsNotFound(err) {
			return err
		}

		if err := p.eraseData(ctx, txn, erase); err != nil {
			return err
		}

		remaining, err := p.erasePolicies(ctx, txn, erase)
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

		if err := manifest.Write(ctx, p.manager.Store, txn, b.Manifest); err != nil {
			return err
		}

		plugins.SetCompilerOnContext(params.Context, compiler)

		return nil
	})
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

func (p *Plugin) logError(fmt string, a ...interface{}) {
	logrus.WithFields(p.logrusFields()).Errorf(fmt, a...)
}

func (p *Plugin) logInfo(fmt string, a ...interface{}) {
	logrus.WithFields(p.logrusFields()).Infof(fmt, a...)
}

func (p *Plugin) logDebug(fmt string, a ...interface{}) {
	logrus.WithFields(p.logrusFields()).Debugf(fmt, a...)
}

func (p *Plugin) logrusFields() logrus.Fields {
	return logrus.Fields{
		"plugin": Name,
		"name":   p.config.Name,
	}
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
