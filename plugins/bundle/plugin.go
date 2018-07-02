// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package bundle implements bundle downloading.
package bundle

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/bundle"
	"github.com/open-policy-agent/opa/plugins"
	"github.com/open-policy-agent/opa/server/types"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/util"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const (
	// min amount of time to wait following a failure
	minRetryDelay          = time.Millisecond * 100
	defaultMinDelaySeconds = int64(60)
	defaultMaxDelaySeconds = int64(120)
)

// PollingConfig represents configuration for the plugin's polling behaviour.
type PollingConfig struct {
	MinDelaySeconds *int64 `json:"min_delay_seconds,omitempty"` // min amount of time to wait between successful poll attempts
	MaxDelaySeconds *int64 `json:"max_delay_seconds,omitempty"` // max amount of time to wait between poll attempts
}

// Config represents configuration the plguin.
type Config struct {
	Name    string        `json:"name"`
	Service string        `json:"service"`
	Polling PollingConfig `json:"polling"`
}

func (c *Config) validateAndInjectDefaults(services []string) error {

	if c.Name == "" {
		return fmt.Errorf("invalid bundle name %q", c.Name)
	}

	found := false

	for _, svc := range services {
		if svc == c.Service {
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("invalid service name %q in bundle %q", c.Service, c.Name)
	}

	min := defaultMinDelaySeconds
	max := defaultMaxDelaySeconds

	// reject bad min/max values
	if c.Polling.MaxDelaySeconds != nil && c.Polling.MinDelaySeconds != nil {
		if *c.Polling.MaxDelaySeconds < *c.Polling.MinDelaySeconds {
			return fmt.Errorf("max polling delay must be >= min polling delay in bundle %q", c.Name)
		}
		min = *c.Polling.MinDelaySeconds
		max = *c.Polling.MaxDelaySeconds
	} else if c.Polling.MaxDelaySeconds == nil && c.Polling.MinDelaySeconds != nil {
		return fmt.Errorf("polling configuration missing 'max_delay_seconds' in bundle %q", c.Name)
	} else if c.Polling.MinDelaySeconds == nil && c.Polling.MaxDelaySeconds != nil {
		return fmt.Errorf("polling configuration missing 'min_delay_seconds' in bundle %q", c.Name)
	}

	// scale to seconds
	minSeconds := int64(time.Duration(min) * time.Second)
	c.Polling.MinDelaySeconds = &minSeconds

	maxSeconds := int64(time.Duration(max) * time.Second)
	c.Polling.MaxDelaySeconds = &maxSeconds

	return nil
}

const (
	errCode = "bundle_error"
)

// Status represents the status of the plugin.
type Status struct {
	Name                     string    `json:"name"`
	ActiveRevision           string    `json:"active_revision,omitempty"`
	LastSuccessfulActivation time.Time `json:"last_successful_activation,omitempty"`
	LastSuccessfulDownload   time.Time `json:"last_successful_download,omitempty"`
	Code                     string    `json:"code,omitempty"`
	Message                  string    `json:"message,omitempty"`
	Errors                   []error   `json:"errors,omitempty"`
}

// Plugin implements bundle downloading and activation.
type Plugin struct {
	manager   *plugins.Manager             // plugin manager for storage and service clients
	config    Config                       // plugin config
	stop      chan chan struct{}           // used to signal plugin to stop running
	etag      string                       // last ETag header for caching purposes
	status    *Status                      // current plugin status
	listeners map[interface{}]func(Status) // listeners to send status updates to
	mtx       sync.Mutex
}

// New returns a new Plugin with the given config.
func New(config []byte, manager *plugins.Manager) (*Plugin, error) {

	var parsedConfig Config

	if err := util.Unmarshal(config, &parsedConfig); err != nil {
		return nil, err
	}

	if err := parsedConfig.validateAndInjectDefaults(manager.Services()); err != nil {
		return nil, err
	}

	plugin := &Plugin{
		manager: manager,
		config:  parsedConfig,
		stop:    make(chan chan struct{}),
		status: &Status{
			Name: parsedConfig.Name,
		},
		listeners: map[interface{}]func(Status){},
	}

	return plugin, nil
}

// Start runs the plugin. The plugin will periodically try to download bundles
// from the configured service. When a new bundle is downloaded, the data and
// policies are extracted and inserted into storage.
func (p *Plugin) Start(ctx context.Context) error {
	go p.loop()
	return nil
}

// Stop stops the plugin.
func (p *Plugin) Stop(ctx context.Context) {
	done := make(chan struct{})
	p.stop <- done
	_ = <-done
}

// Register a lisetner to receive status updates. The name must be comparable.
func (p *Plugin) Register(name interface{}, listener func(Status)) {
	p.mtx.Lock()
	defer p.mtx.Unlock()
	p.listeners[name] = listener
}

// Unregister a listener to stop receiving status updates.
func (p *Plugin) Unregister(name interface{}) {
	p.mtx.Lock()
	defer p.mtx.Unlock()
	delete(p.listeners, name)
}

func (p *Plugin) loop() {

	ctx, cancel := context.WithCancel(context.Background())

	var retry int

	for {
		updated, err := p.oneShot(ctx)

		if err != nil {
			p.logError("%v.", err)
		} else if !updated {
			p.logDebug("Bundle download skipped, server replied with not modified.")
		} else if p.etag != "" {
			p.logInfo("Bundle downloaded and activated successfully. Etag updated to %v.", p.etag)
		} else {
			p.logInfo("Bundle downloaded and activated successfully.")
		}

		var delay time.Duration

		if err == nil {
			min := float64(*p.config.Polling.MinDelaySeconds)
			max := float64(*p.config.Polling.MaxDelaySeconds)
			delay = time.Duration(((max - min) * rand.Float64()) + min)
		} else {
			delay = util.DefaultBackoff(float64(minRetryDelay), float64(*p.config.Polling.MaxDelaySeconds), retry)
		}

		p.logDebug("Waiting %v before next download/retry.", delay)
		timer := time.NewTimer(delay)

		select {
		case <-timer.C:
			if err != nil {
				retry++
			} else {
				retry = 0
			}
		case done := <-p.stop:
			cancel()
			done <- struct{}{}
			return
		}
	}

}

func (p *Plugin) oneShot(ctx context.Context) (updated bool, err error) {

	defer func() {
		p.setErrorStatus(err)
		status := *p.status

		for _, listener := range p.listeners {
			listener(status)
		}
	}()

	p.logDebug("Download starting.")

	resp, err := p.manager.Client(p.config.Service).
		WithHeader("If-None-Match", p.etag).
		Do(ctx, "GET", fmt.Sprintf("/bundles/%v", p.config.Name))

	if err != nil {
		return false, errors.Wrap(err, "Download request failed")
	}

	defer util.Close(resp)

	switch resp.StatusCode {
	case http.StatusOK:
		if err := p.process(ctx, resp); err != nil {
			return false, err
		}
		return true, nil
	case http.StatusNotModified:
		return false, nil
	case http.StatusNotFound:
		return false, fmt.Errorf("Bundle download failed, server replied with not found")
	case http.StatusUnauthorized:
		return false, fmt.Errorf("Bundle download failed, server replied with not authorized")
	default:
		return false, fmt.Errorf("Bundle download failed, server replied with HTTP %v", resp.StatusCode)
	}
}

func (p *Plugin) process(ctx context.Context, resp *http.Response) error {

	b, err := p.download(ctx, resp)
	if err != nil {
		return errors.Wrap(err, "Bundle download failed")
	}

	if err := p.activate(ctx, resp.Header.Get("ETag"), *b); err != nil {
		return errors.Wrap(err, "Bundle activation failed")
	}

	return nil
}

func (p *Plugin) download(ctx context.Context, resp *http.Response) (*bundle.Bundle, error) {

	p.logDebug("Bundle download in progress.")

	b, err := bundle.Read(resp.Body)
	if err != nil {
		return nil, err
	}

	p.status.LastSuccessfulDownload = time.Now().UTC()
	return &b, nil
}

func (p *Plugin) activate(ctx context.Context, etag string, b bundle.Bundle) error {
	p.logDebug("Bundle activation in progress. Opening storage transaction.")

	return storage.Txn(ctx, p.manager.Store, storage.WriteParams, func(txn storage.Transaction) error {
		p.logDebug("Opened storage transaction (%v).", txn.ID())
		defer p.logDebug("Closing storage transaction (%v).", txn.ID())

		// write data from bundle into store, overwritting contents
		if err := p.manager.Store.Write(ctx, txn, storage.AddOp, storage.Path{}, b.Data); err != nil {
			return err
		}

		if err := p.writeManifest(ctx, txn, b.Manifest); err != nil {
			return err
		}

		// load existing policy ids from store and delete
		ids, err := p.manager.Store.ListPolicies(ctx, txn)
		if err != nil {
			return err
		}

		for _, id := range ids {
			if err := p.manager.Store.DeletePolicy(ctx, txn, id); err != nil {
				return err
			}
		}

		// ensure that policies compile.
		modules := map[string]*ast.Module{}

		for _, file := range b.Modules {
			modules[file.Path] = file.Parsed
		}

		compiler := ast.NewCompiler()
		if compiler.Compile(modules); compiler.Failed() {
			return compiler.Errors
		}

		// write policies from bundle into store.
		for _, file := range b.Modules {
			if err := p.manager.Store.UpsertPolicy(ctx, txn, file.Path, file.Raw); err != nil {
				return err
			}
		}

		p.status.LastSuccessfulActivation = time.Now().UTC()
		p.status.ActiveRevision = b.Manifest.Revision
		p.etag = etag

		return nil
	})
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
		"plugin": "bundle",
		"name":   p.config.Name,
	}
}

func (p *Plugin) setErrorStatus(err error) {

	if err == nil {
		p.status.Code = ""
		p.status.Message = ""
		p.status.Errors = nil
		return
	}

	cause := errors.Cause(err)

	if astErr, ok := cause.(ast.Errors); ok {
		p.status.Code = errCode
		p.status.Message = types.MsgCompileModuleError
		p.status.Errors = make([]error, len(astErr))
		for i := range astErr {
			p.status.Errors[i] = astErr[i]
		}
	} else {
		p.status.Code = errCode
		p.status.Message = err.Error()
		p.status.Errors = nil
	}
}

func (p *Plugin) writeManifest(ctx context.Context, txn storage.Transaction, m bundle.Manifest) error {

	var value interface{} = m

	if err := util.RoundTrip(&value); err != nil {
		return err
	}

	if err := storage.MakeDir(ctx, p.manager.Store, txn, bundlePath); err != nil {
		return err
	}

	return p.manager.Store.Write(ctx, txn, storage.AddOp, manifestPath, value)
}

var (
	bundlePath   = storage.MustParsePath("/system/bundle")
	manifestPath = storage.MustParsePath("/system/bundle/manifest")
)
