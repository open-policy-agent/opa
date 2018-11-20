// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/open-policy-agent/opa/ast"
	bundleApi "github.com/open-policy-agent/opa/bundle"
	"github.com/open-policy-agent/opa/plugins"
	"github.com/open-policy-agent/opa/plugins/bundle"
	"github.com/open-policy-agent/opa/plugins/logs"
	"github.com/open-policy-agent/opa/plugins/status"
	"github.com/open-policy-agent/opa/rego"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/storage/inmem"
	"github.com/open-policy-agent/opa/util"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type bundlePluginListener string

// Discovery contains the result of configuration discovery.
type Discovery struct {
	Manager                      *plugins.Manager
	Plugins                      map[string]plugins.Plugin
	DefaultDecision              ast.Ref
	DefaultAuthorizationDecision ast.Ref

	discoveryEnabled    bool
	discoveryPathConfig *discoveryPathConfig
	stop                chan chan struct{}
	pluginsMux          sync.Mutex
}

// PollingConfig represents configuration for discovery's polling behaviour.
type PollingConfig struct {
	MinDelaySeconds *int64 `json:"min_delay_seconds,omitempty"` // min amount of time to wait between successful poll attempts
	MaxDelaySeconds *int64 `json:"max_delay_seconds,omitempty"` // max amount of time to wait between poll attempts
}

type rawConfig struct {
	DefaultDecision              string `json:"default_decision"`
	DefaultAuthorizationDecision string `json:"default_authorization_decision"`
}

type discoveryConfig struct {
	Discovery json.RawMessage `json:"discovery"`
}

type discoveryPathConfig struct {
	Path        *string       `json:"path"`
	Prefix      *string       `json:"prefix"`
	Polling     PollingConfig `json:"polling"`
	serviceURL  string
	serviceHost string
}

func parsePathToRef(s string) (ast.Ref, error) {
	s = strings.Replace(strings.Trim(s, "/"), "/", ".", -1)
	return ast.ParseRef("data." + s)
}

const (
	defaultDecisionPath              = "/system/main"
	defaultAuthorizationDecisionPath = "/system/authz/allow"
	defaultDiscoveryPathPrefix       = "bundles"
	defaultDiscoveryQueryPrefix      = "data"

	// min amount of time to wait following a failure
	minRetryDelay          = time.Millisecond * 100
	defaultMinDelaySeconds = int64(60)
	defaultMaxDelaySeconds = int64(120)
)

// TODO(tsandall): revisit how plugins are wired up to the manager and how
// everything is started and stopped. We could introduce a package-scoped
// plugin registry that allows for (dynamic) init-time plugin registration.

// New takes the provided configuration and if needed fetches
// OPA's new configuration from a remote server. It then instantiates the
// plugins using either the provided configuration or the one fetched from
// the remote server.
func New(ctx context.Context, opaID string, store storage.Store, configFile string, registeredPlugins map[string]plugins.PluginInitFunc) (*Discovery, error) {

	var bs []byte
	var err error

	if configFile != "" {
		bs, err = ioutil.ReadFile(configFile)
		if err != nil {
			return nil, err
		}
	}

	m, err := plugins.New(bs, opaID, store)
	if err != nil {
		return nil, err
	}

	// If discovery is enabled, get the new configuration
	// from the remote server.
	var config *discoveryConfig
	var discoveryEnabled bool
	var discPathConfig *discoveryPathConfig

	config, discoveryEnabled = isDiscoveryEnabled(bs)
	if discoveryEnabled {
		discPathConfig, err = validateAndInjectDefaults(config, m.Services())
		if err != nil {
			return nil, err
		}

		bs, _, err = discoveryHandler(ctx, discPathConfig, m)
		if err != nil {
			return nil, err
		}

		if bs != nil {
			m.Update(bs)
		}
	}

	plugins, err := configurePlugins(m, bs, registeredPlugins)
	if err != nil {
		return nil, err
	}

	defaultDecision, defaultAuthorizationDecision, err := configureDefaultDecision(bs)
	if err != nil {
		return nil, err
	}

	c := &Discovery{
		Manager:                      m,
		Plugins:                      plugins,
		DefaultDecision:              defaultDecision,
		DefaultAuthorizationDecision: defaultAuthorizationDecision,
		discoveryPathConfig:          discPathConfig,
		discoveryEnabled:             discoveryEnabled,
	}
	return c, nil
}

// Start starts the plugin manager and periodic configuration discovery.
func (c *Discovery) Start(ctx context.Context) error {

	if err := c.Manager.Start(ctx); err != nil {
		return err
	}

	if c.discoveryEnabled {
		go c.loop()
	}
	return nil
}

// Stop stops the plugin manager and periodic configuration discovery.
func (c *Discovery) Stop(ctx context.Context) {
	c.Manager.Stop(ctx)

	if c.discoveryEnabled {
		done := make(chan struct{})
		c.stop <- done
		_ = <-done
	}
}

func (c *Discovery) loop() {

	ctx, cancel := context.WithCancel(context.Background())

	var retry int

	for {
		bs, updated, err := discoveryHandler(ctx, c.discoveryPathConfig, c.Manager)
		if err != nil {
			c.logError("%v.", err)
		} else if !updated {
			c.logDebug("Configuration download skipped, server replied with not modified.")
		} else {
			c.logInfo("New configuration successfully downloaded. Now re-configuring plugins.")

			if bs != nil {
				c.Manager.Update(bs)
			}

			err = c.reconfigurePlugins(ctx, bs)
			if err != nil {
				c.logError("%v.", err)
			} else {
				c.logInfo("Plugins reconfigured successfully.")
			}
		}

		var delay time.Duration

		if err == nil {
			min := float64(*c.discoveryPathConfig.Polling.MinDelaySeconds)
			max := float64(*c.discoveryPathConfig.Polling.MaxDelaySeconds)
			delay = time.Duration(((max - min) * rand.Float64()) + min)
		} else {
			delay = util.DefaultBackoff(float64(minRetryDelay), float64(*c.discoveryPathConfig.Polling.MaxDelaySeconds), retry)
		}

		c.logDebug("Waiting %v before next download/retry.", delay)
		timer := time.NewTimer(delay)

		select {
		case <-timer.C:
			if err != nil {
				retry++
			} else {
				retry = 0
			}
		case done := <-c.stop:
			cancel()
			done <- struct{}{}
			return
		}
	}
}

func (c *Discovery) reconfigurePlugins(ctx context.Context, bs []byte) error {

	err := c.reconfigureBundlePlugin(ctx, bs)
	if err != nil {
		return err
	}

	err = c.reconfigureStatusPlugin(ctx, bs)
	if err != nil {
		return err
	}

	err = c.reconfigureDecisionLogsPlugin(ctx, bs)
	if err != nil {
		return err
	}

	err = c.reconfigureRegisteredPlugins(bs)
	if err != nil {
		return err
	}

	return nil
}

func (c *Discovery) reconfigureBundlePlugin(ctx context.Context, bs []byte) error {
	var config struct {
		Bundle json.RawMessage `json:"bundle"`
	}

	if err := util.Unmarshal(bs, &config); err != nil {
		return err
	}

	if config.Bundle == nil {
		return nil
	}

	plugin, ok := c.Plugins["bundle"]
	if !ok {
		bundlePlugin, err := initBundlePlugin(c.Manager, bs)
		if err != nil {
			return err
		} else if bundlePlugin != nil {
			c.pluginsMux.Lock()
			c.Plugins["bundle"] = bundlePlugin
			c.pluginsMux.Unlock()
			err = bundlePlugin.Start(ctx)
			if err != nil {
				return err
			}
		}
	} else {
		reconfig := plugins.ReconfigData{
			Manager: c.Manager,
			Config:  config.Bundle,
		}
		plugin.(*bundle.Plugin).Reconfigure(reconfig)
	}

	return nil
}

func (c *Discovery) reconfigureStatusPlugin(ctx context.Context, bs []byte) error {
	var config struct {
		Status json.RawMessage `json:"status"`
	}

	if err := util.Unmarshal(bs, &config); err != nil {
		return err
	}

	if config.Status == nil {
		return nil
	}

	plugin, ok := c.Plugins["status"]
	if !ok {
		bundlePlugin := c.Plugins["bundle"]
		if bundlePlugin != nil {
			statusPlugin, err := initStatusPlugin(c.Manager, bs, bundlePlugin.(*bundle.Plugin))
			if err != nil {
				return err
			} else if statusPlugin != nil {
				c.pluginsMux.Lock()
				c.Plugins["status"] = statusPlugin
				c.pluginsMux.Unlock()
				err = statusPlugin.Start(ctx)
				if err != nil {
					return err
				}
			}
		}
	} else {
		reconfig := plugins.ReconfigData{
			Manager: c.Manager,
			Config:  config.Status,
		}
		plugin.(*status.Plugin).Reconfigure(reconfig)
	}

	return nil
}

func (c *Discovery) reconfigureDecisionLogsPlugin(ctx context.Context, bs []byte) error {
	var config struct {
		DecisionLogs json.RawMessage `json:"decision_logs"`
	}

	if err := util.Unmarshal(bs, &config); err != nil {
		return err
	}

	if config.DecisionLogs == nil {
		return nil
	}

	plugin, ok := c.Plugins["decision_logs"]
	if !ok {
		decisionLogsPlugin, err := initDecisionLogsPlugin(c.Manager, bs)
		if err != nil {
			return err
		} else if decisionLogsPlugin != nil {
			c.pluginsMux.Lock()
			c.Plugins["decision_logs"] = decisionLogsPlugin
			c.pluginsMux.Unlock()
			err = decisionLogsPlugin.Start(ctx)
			if err != nil {
				return err
			}
		}
	} else {
		reconfig := plugins.ReconfigData{
			Manager: c.Manager,
			Config:  config.DecisionLogs,
		}
		plugin.(*logs.Plugin).Reconfigure(reconfig)
	}

	return nil
}

func (c *Discovery) reconfigureRegisteredPlugins(bs []byte) error {

	var config struct {
		Plugins map[string]json.RawMessage `json:"plugins"`
	}

	if err := util.Unmarshal(bs, &config); err != nil {
		return err
	}

	for name, plugin := range c.Plugins {
		pc, ok := config.Plugins[name]
		if !ok {
			continue
		}

		reconfig := plugins.ReconfigData{
			Manager: c.Manager,
			Config:  pc,
		}
		plugin.Reconfigure(reconfig)
	}

	return nil
}

func validateAndInjectDefaults(config *discoveryConfig, services []string) (*discoveryPathConfig, error) {

	var discoveryConfig discoveryPathConfig

	if err := util.Unmarshal(config.Discovery, &discoveryConfig); err != nil {
		return nil, err
	}

	if len(services) == 0 {
		return nil, fmt.Errorf("endpoint to fetch discovery configuration from not provided")
	}

	discoveryConfig.serviceHost = services[0]
	discoveryConfig.serviceURL = getDiscoveryServicePath(discoveryConfig)

	min := defaultMinDelaySeconds
	max := defaultMaxDelaySeconds

	// reject bad min/max values
	if discoveryConfig.Polling.MaxDelaySeconds != nil && discoveryConfig.Polling.MinDelaySeconds != nil {
		if *discoveryConfig.Polling.MaxDelaySeconds < *discoveryConfig.Polling.MinDelaySeconds {
			return nil, fmt.Errorf("max polling delay must be >= min polling delay in discovery configuration")
		}
		min = *discoveryConfig.Polling.MinDelaySeconds
		max = *discoveryConfig.Polling.MaxDelaySeconds
	} else if discoveryConfig.Polling.MaxDelaySeconds == nil && discoveryConfig.Polling.MinDelaySeconds != nil {
		return nil, fmt.Errorf("polling configuration missing 'max_delay_seconds' in discovery configuration")
	} else if discoveryConfig.Polling.MinDelaySeconds == nil && discoveryConfig.Polling.MaxDelaySeconds != nil {
		return nil, fmt.Errorf("polling configuration missing 'min_delay_seconds' in discovery configuration")
	}

	// scale to seconds
	minSeconds := int64(time.Duration(min) * time.Second)
	discoveryConfig.Polling.MinDelaySeconds = &minSeconds

	maxSeconds := int64(time.Duration(max) * time.Second)
	discoveryConfig.Polling.MaxDelaySeconds = &maxSeconds

	return &discoveryConfig, nil
}

func configureDefaultDecision(bs []byte) (ast.Ref, ast.Ref, error) {

	var raw rawConfig

	if err := util.Unmarshal(bs, &raw); err != nil {
		return nil, nil, err
	}

	if raw.DefaultDecision == "" {
		raw.DefaultDecision = defaultDecisionPath
	}

	if raw.DefaultAuthorizationDecision == "" {
		raw.DefaultAuthorizationDecision = defaultAuthorizationDecisionPath
	}

	defaultDecision, err := parsePathToRef(raw.DefaultDecision)
	if err != nil {
		return nil, nil, err
	}

	defaultAuthorizationDecision, err := parsePathToRef(raw.DefaultAuthorizationDecision)
	if err != nil {
		return nil, nil, err
	}

	return defaultDecision, defaultAuthorizationDecision, nil
}

func configurePlugins(m *plugins.Manager, bs []byte, registeredPlugins map[string]plugins.PluginInitFunc) (map[string]plugins.Plugin, error) {

	plugins := map[string]plugins.Plugin{}

	bundlePlugin, err := initBundlePlugin(m, bs)
	if err != nil {
		return nil, err
	} else if bundlePlugin != nil {
		plugins["bundle"] = bundlePlugin
	}

	if bundlePlugin != nil {
		statusPlugin, err := initStatusPlugin(m, bs, bundlePlugin)
		if err != nil {
			return nil, err
		} else if statusPlugin != nil {
			plugins["status"] = statusPlugin
		}
	}

	decisionLogsPlugin, err := initDecisionLogsPlugin(m, bs)
	if err != nil {
		return nil, err
	} else if decisionLogsPlugin != nil {
		plugins["decision_logs"] = decisionLogsPlugin
	}

	err = initRegisteredPlugins(m, bs, registeredPlugins, plugins)
	if err != nil {
		return nil, err
	}

	return plugins, nil
}

func getDiscoveryConfig(bs []byte) (*discoveryConfig, error) {
	var config discoveryConfig

	if err := util.Unmarshal(bs, &config); err != nil {
		return nil, err
	}
	return &config, nil
}

func isDiscoveryEnabled(bs []byte) (*discoveryConfig, bool) {
	config, err := getDiscoveryConfig(bs)
	if err != nil {
		return nil, false
	}

	if config.Discovery == nil {
		return nil, false
	}
	return config, true
}

func discoveryHandler(ctx context.Context, discoveryConfig *discoveryPathConfig, manager *plugins.Manager) ([]byte, bool, error) {

	resp, err := manager.Client(discoveryConfig.serviceHost).
		Do(ctx, "GET", discoveryConfig.serviceURL)

	if err != nil {
		return nil, false, errors.Wrap(err, "Download request failed")
	}

	defer util.Close(resp)

	switch resp.StatusCode {
	case http.StatusOK:
		return process(ctx, resp, discoveryConfig.Path)
	case http.StatusNotModified:
		return nil, false, nil
	case http.StatusNotFound:
		return nil, false, fmt.Errorf("Discovery configuration download failed, server replied with not found")
	case http.StatusUnauthorized:
		return nil, false, fmt.Errorf("Discovery configuration download failed, server replied with not authorized")
	default:
		return nil, false, fmt.Errorf("Discovery configuration download failed, server replied with HTTP %v", resp.StatusCode)
	}
}

func getDiscoveryServicePath(config discoveryPathConfig) string {
	prefix := defaultDiscoveryPathPrefix
	path := ""

	if config.Prefix != nil {
		prefix = *config.Prefix
	}

	if config.Path != nil {
		path = *config.Path
	}

	return fmt.Sprintf("%v/%v", strings.Trim(prefix, "/"), strings.Trim(path, "/"))
}

func process(ctx context.Context, resp *http.Response, path *string) ([]byte, bool, error) {
	b, err := bundleApi.Read(resp.Body)
	if err != nil {
		return nil, false, err
	}

	query := defaultDiscoveryQueryPrefix
	if path != nil && *path != "" {
		*path = strings.Trim(*path, "/")
		query = fmt.Sprintf("%v.%v", query, strings.Replace(*path, "/", ".", -1))
	}

	return processBundle(ctx, b, query)
}

func processBundle(ctx context.Context, b bundleApi.Bundle, query string) ([]byte, bool, error) {
	modules := map[string]*ast.Module{}

	for _, file := range b.Modules {
		modules[file.Path] = file.Parsed
	}

	compiler := ast.NewCompiler()
	if compiler.Compile(modules); compiler.Failed() {
		return nil, false, compiler.Errors
	}

	store := inmem.NewFromObject(b.Data)

	rego := rego.New(
		rego.Query(query),
		rego.Compiler(compiler),
		rego.Store(store),
	)

	rs, err := rego.Eval(ctx)
	if err != nil {
		return nil, false, err
	}

	if len(rs) == 0 {
		return nil, false, fmt.Errorf("undefined configuration")
	}

	result := rs[0].Expressions[0].Value

	switch result.(type) {
	case map[string]interface{}:
		newConfig, err := json.Marshal(result)
		if err != nil {
			return nil, false, err
		}
		return newConfig, true, nil
	default:
		return nil, false, fmt.Errorf("expected discovery rule to generate an object but got %T", result)
	}
}

func initBundlePlugin(m *plugins.Manager, bs []byte) (*bundle.Plugin, error) {

	var config struct {
		Bundle json.RawMessage `json:"bundle"`
	}

	if err := util.Unmarshal(bs, &config); err != nil {
		return nil, err
	}

	if config.Bundle == nil {
		return nil, nil
	}

	p, err := bundle.New(config.Bundle, m)
	if err != nil {
		return nil, err
	}

	m.Register(p)

	return p, nil
}

func initStatusPlugin(m *plugins.Manager, bs []byte, bundlePlugin *bundle.Plugin) (*status.Plugin, error) {

	var config struct {
		Status json.RawMessage `json:"status"`
	}

	if err := util.Unmarshal(bs, &config); err != nil {
		return nil, err
	}

	if config.Status == nil {
		return nil, nil
	}

	p, err := status.New(config.Status, m)
	if err != nil {
		return nil, err
	}

	m.Register(p)

	bundlePlugin.Register(bundlePluginListener("status-plugin"), func(s bundle.Status) {
		p.Update(s)
	})

	return p, nil
}

func initDecisionLogsPlugin(m *plugins.Manager, bs []byte) (*logs.Plugin, error) {

	var config struct {
		DecisionLogs json.RawMessage `json:"decision_logs"`
	}

	if err := util.Unmarshal(bs, &config); err != nil {
		return nil, err
	}

	if config.DecisionLogs == nil {
		return nil, nil
	}

	p, err := logs.New(config.DecisionLogs, m)
	if err != nil {
		return nil, err
	}

	m.Register(p)

	return p, nil
}

func initRegisteredPlugins(m *plugins.Manager, bs []byte, registeredPlugins map[string]plugins.PluginInitFunc, allPlugins map[string]plugins.Plugin) error {

	var config struct {
		Plugins map[string]json.RawMessage `json:"plugins"`
	}

	if err := util.Unmarshal(bs, &config); err != nil {
		return err
	}

	for name, factory := range registeredPlugins {
		pc, ok := config.Plugins[name]
		if !ok {
			continue
		}
		plugin, err := factory(m, pc)
		if err != nil {
			return err
		}
		m.Register(plugin)
		allPlugins[name] = plugin
	}

	return nil
}

func (c *Discovery) logError(fmt string, a ...interface{}) {
	logrus.WithFields(c.logrusFields()).Errorf(fmt, a...)
}

func (c *Discovery) logInfo(fmt string, a ...interface{}) {
	logrus.WithFields(c.logrusFields()).Infof(fmt, a...)
}

func (c *Discovery) logDebug(fmt string, a ...interface{}) {
	logrus.WithFields(c.logrusFields()).Debugf(fmt, a...)
}

func (c *Discovery) logrusFields() logrus.Fields {
	return logrus.Fields{
		"config": "discovery",
	}
}
