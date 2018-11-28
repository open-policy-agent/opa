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
	"github.com/open-policy-agent/opa/server/types"
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
	status              *plugins.Status
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
	Name        *string       `json:"name"`
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

	errCode         = "discovery_error"
	discoveryPlugin = "discovery"
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

	var config *discoveryConfig
	var discoveryEnabled bool
	var discPathConfig *discoveryPathConfig

	config, discoveryEnabled = isDiscoveryEnabled(bs)
	if discoveryEnabled {
		discPathConfig, err = validateAndInjectDefaults(config, m.Services())
		if err != nil {
			return nil, err
		}
	}

	p, err := initPlugins(m, bs, registeredPlugins)
	if err != nil {
		return nil, err
	}

	defaultDecision, defaultAuthorizationDecision, err := configureDefaultDecision(bs)
	if err != nil {
		return nil, err
	}

	c := &Discovery{
		Manager:                      m,
		Plugins:                      p,
		DefaultDecision:              defaultDecision,
		DefaultAuthorizationDecision: defaultAuthorizationDecision,
		discoveryPathConfig:          discPathConfig,
		discoveryEnabled:             discoveryEnabled,
	}

	if discoveryEnabled {
		c.status = &plugins.Status{
			Plugin: discoveryPlugin,
			Name:   *discPathConfig.Name,
		}
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
			c.logInfo("New configuration successfully downloaded. Now updating plugins.")

			if bs != nil {
				c.Manager.Update(bs)
			}

			err = c.validateAndConfigurePlugins(ctx, bs)
			if err != nil {
				c.logError("%v.", err)
			}
		}

		c.status.LastSuccessfulDownload = time.Now().UTC()

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

func validateBundlePluginConfig(m *plugins.Manager, bs []byte) (*bundle.Config, error) {
	var config struct {
		Bundle json.RawMessage `json:"bundle"`
	}

	if err := util.Unmarshal(bs, &config); err != nil {
		return nil, err
	}

	if config.Bundle == nil {
		return nil, nil
	}

	return bundle.ParseConfig(config.Bundle, m.Services())
}

func validateDecisionLogsPluginConfig(m *plugins.Manager, bs []byte) (*logs.Config, error) {
	var config struct {
		DecisionLogs json.RawMessage `json:"decision_logs"`
	}

	if err := util.Unmarshal(bs, &config); err != nil {
		return nil, err
	}

	if config.DecisionLogs == nil {
		return nil, nil
	}

	return logs.ParseConfig(config.DecisionLogs, m.Services())
}

func validateStatusPluginConfig(m *plugins.Manager, bs []byte) (*status.Config, error) {
	var config struct {
		Status json.RawMessage `json:"status"`
	}

	if err := util.Unmarshal(bs, &config); err != nil {
		return nil, err
	}

	if config.Status == nil {
		return nil, nil
	}

	return status.ParseConfig(config.Status, m.Services())
}

func (c *Discovery) validateAndConfigurePlugins(ctx context.Context, bs []byte) (err error) {

	defer func() {
		c.setErrorStatus(err)

		if plugin, ok := c.Plugins["status"]; ok {
			plugin.(*status.Plugin).Update(*c.status)
		}
	}()

	bundleConfig, err := validateBundlePluginConfig(c.Manager, bs)
	if err != nil {
		return err
	}

	decisionLogsConfig, err := validateDecisionLogsPluginConfig(c.Manager, bs)
	if err != nil {
		return err
	}

	statusConfig, err := validateStatusPluginConfig(c.Manager, bs)
	if err != nil {
		return err
	}

	if bundleConfig != nil {
		err := c.configureBundlePlugin(ctx, bundleConfig)
		if err != nil {
			return err
		}
	}

	if decisionLogsConfig != nil {
		err = c.configureDecisionLogsPlugin(ctx, decisionLogsConfig)
		if err != nil {
			return err
		}
	}

	if statusConfig != nil {
		err = c.configureStatusPlugin(ctx, statusConfig)
		if err != nil {
			return err
		}
	}

	err = c.configureRegisteredPlugins(bs)
	if err != nil {
		return err
	}

	return nil
}

func (c *Discovery) configureBundlePlugin(ctx context.Context, config *bundle.Config) error {

	plugin, ok := c.Plugins["bundle"]
	if !ok {
		bundlePlugin, err := createBundlePlugin(c.Manager, config)
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
			c.logInfo("Bundle plugin configured successfully.")
		}
	} else {
		if plugin.(*bundle.Plugin).Equal(config) {
			c.logDebug("No updated configuration for bundle plugin.")
		} else {
			reconfig := plugins.ReconfigData{
				Manager: c.Manager,
				Config:  config,
			}
			plugin.(*bundle.Plugin).Reconfigure(reconfig)
			c.logInfo("Bundle plugin reconfigured successfully.")
		}
	}

	return nil
}

func (c *Discovery) configureDecisionLogsPlugin(ctx context.Context, config *logs.Config) error {

	plugin, ok := c.Plugins["decision_logs"]
	if !ok {
		decisionLogsPlugin, err := createDecisionLogsPlugin(c.Manager, config)
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
			c.logInfo("Decision logs plugin configured successfully.")
		}
	} else {
		if plugin.(*logs.Plugin).Equal(config) {
			c.logDebug("No updated configuration for decision logs plugin.")
		} else {
			reconfig := plugins.ReconfigData{
				Manager: c.Manager,
				Config:  config,
			}
			plugin.(*logs.Plugin).Reconfigure(reconfig)
			c.logInfo("Decision logs plugin reconfigured successfully.")
		}
	}

	return nil
}

func (c *Discovery) configureStatusPlugin(ctx context.Context, config *status.Config) error {

	plugin, ok := c.Plugins["status"]
	if !ok {
		bundlePlugin := c.Plugins["bundle"]
		if bundlePlugin != nil {
			statusPlugin, err := createStatusPlugin(c.Manager, config, bundlePlugin.(*bundle.Plugin))
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
				c.logInfo("Status plugin configured successfully.")
			}
		}
	} else {
		if plugin.(*status.Plugin).Equal(config) {
			c.logDebug("No updated configuration for status plugin.")
		} else {
			reconfig := plugins.ReconfigData{
				Manager: c.Manager,
				Config:  config,
			}
			plugin.(*status.Plugin).Reconfigure(reconfig)
			c.logInfo("Status plugin reconfigured successfully.")
		}
	}

	return nil
}

func (c *Discovery) configureRegisteredPlugins(bs []byte) error {

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

func initPlugins(m *plugins.Manager, bs []byte, registeredPlugins map[string]plugins.PluginInitFunc) (map[string]plugins.Plugin, error) {

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

func initBundlePlugin(m *plugins.Manager, bs []byte) (*bundle.Plugin, error) {

	bundleConfig, err := validateBundlePluginConfig(m, bs)
	if err != nil {
		return nil, err
	}

	if bundleConfig == nil {
		return nil, nil
	}

	return createBundlePlugin(m, bundleConfig)
}

func initDecisionLogsPlugin(m *plugins.Manager, bs []byte) (*logs.Plugin, error) {

	decisionLogsConfig, err := validateDecisionLogsPluginConfig(m, bs)
	if err != nil {
		return nil, err
	}

	if decisionLogsConfig == nil {
		return nil, nil
	}

	return createDecisionLogsPlugin(m, decisionLogsConfig)
}

func initStatusPlugin(m *plugins.Manager, bs []byte, bundlePlugin *bundle.Plugin) (*status.Plugin, error) {

	statusConfig, err := validateStatusPluginConfig(m, bs)
	if err != nil {
		return nil, err
	}

	if statusConfig == nil {
		return nil, nil
	}

	return createStatusPlugin(m, statusConfig, bundlePlugin)
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

func createBundlePlugin(m *plugins.Manager, config *bundle.Config) (*bundle.Plugin, error) {

	p, err := bundle.New(config, m)
	if err != nil {
		return nil, err
	}

	m.Register(p)

	return p, nil
}

func createDecisionLogsPlugin(m *plugins.Manager, config *logs.Config) (*logs.Plugin, error) {

	p, err := logs.New(config, m)
	if err != nil {
		return nil, err
	}

	m.Register(p)

	return p, nil
}

func createStatusPlugin(m *plugins.Manager, config *status.Config, bundlePlugin *bundle.Plugin) (*status.Plugin, error) {

	p, err := status.New(config, m)
	if err != nil {
		return nil, err
	}

	m.Register(p)

	bundlePlugin.Register(bundlePluginListener("status-plugin"), func(s plugins.Status) {
		p.Update(s)
	})

	return p, nil
}

func validateAndInjectDefaults(config *discoveryConfig, services []string) (*discoveryPathConfig, error) {

	var discoveryConfig discoveryPathConfig

	if err := util.Unmarshal(config.Discovery, &discoveryConfig); err != nil {
		return nil, err
	}

	if len(services) == 0 {
		return nil, fmt.Errorf("endpoint to fetch discovery configuration from not provided")
	}

	if discoveryConfig.Name == nil {
		return nil, fmt.Errorf("discovery plugin name not provided")
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
		return process(ctx, resp, discoveryConfig.Name)
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

	if config.Name != nil {
		path = *config.Name
	}

	return fmt.Sprintf("%v/%v", strings.Trim(prefix, "/"), strings.Trim(path, "/"))
}

func process(ctx context.Context, resp *http.Response, path *string) ([]byte, bool, error) {
	br := bundleApi.NewReader(resp.Body)
	b, err := br.Read()
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
		"plugin": "discovery",
		"name":   *c.discoveryPathConfig.Name,
	}
}

func (c *Discovery) setErrorStatus(err error) {

	if err == nil {
		c.status.Code = ""
		c.status.Message = ""
		c.status.Errors = nil
		return
	}

	cause := errors.Cause(err)

	if astErr, ok := cause.(ast.Errors); ok {
		c.status.Code = errCode
		c.status.Message = types.MsgPluginConfigError
		c.status.Errors = make([]error, len(astErr))
		for i := range astErr {
			c.status.Errors[i] = astErr[i]
		}
	} else {
		c.status.Code = errCode
		c.status.Message = err.Error()
		c.status.Errors = nil
	}
}
