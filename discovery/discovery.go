// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

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
)

type bundlePluginListener string

// DiscoveredConfig contains the result of configuration discovery.
type DiscoveredConfig struct {
	Manager                      *plugins.Manager
	Plugins                      map[string]plugins.Plugin
	DefaultDecision              ast.Ref
	DefaultAuthorizationDecision ast.Ref
}

type rawConfig struct {
	DefaultDecision              string `json:"default_decision"`
	DefaultAuthorizationDecision string `json:"default_authorization_decision"`
}

type discoveryConfig struct {
	Discovery json.RawMessage `json:"discovery"`
	Services  []json.RawMessage
}

func parsePathToRef(s string) (ast.Ref, error) {
	s = strings.Replace(strings.Trim(s, "/"), "/", ".", -1)
	return ast.ParseRef("data." + s)
}

const (
	defaultDecisionPath              = "/system/main"
	defaultAuthorizationDecisionPath = "/system/authz/allow"
)

// TODO(tsandall): revisit how plugins are wired up to the manager and how
// everything is started and stopped. We could introduce a package-scoped
// plugin registry that allows for (dynamic) init-time plugin registration.

// ConfigHandler takes the provided configuration and if needed fetches
// OPA's new configuration from a remote server. It then instantiates the
// plugins using either the provided configuration or the one fetched from
// the remote server.
func ConfigHandler(ctx context.Context, opaID string, store storage.Store, configFile string, registeredPlugins map[string]plugins.PluginInitFunc) (*DiscoveredConfig, error) {

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

	config, discoveryEnabled = isDiscoveryEnabled(bs)
	if discoveryEnabled {
		bs, err = discoveryHandler(ctx, config, m)
		if err != nil {
			return nil, err
		}

		// Add labels to the plugin manager
		var parsedConfig struct {
			Labels map[string]string
		}

		if err := util.Unmarshal(bs, &parsedConfig); err != nil {
			return nil, err
		}

		if parsedConfig.Labels != nil {
			parsedConfig.Labels["id"] = m.Labels["id"]
			m.Labels = parsedConfig.Labels
		}
	}

	plugins, err := configurePlugins(m, bs, registeredPlugins)
	if err != nil {
		return nil, err
	}

	var raw rawConfig

	if err := util.Unmarshal(bs, &raw); err != nil {
		return nil, err
	}

	if raw.DefaultDecision == "" {
		raw.DefaultDecision = defaultDecisionPath
	}

	if raw.DefaultAuthorizationDecision == "" {
		raw.DefaultAuthorizationDecision = defaultAuthorizationDecisionPath
	}

	defaultDecision, err := parsePathToRef(raw.DefaultDecision)
	if err != nil {
		return nil, err
	}

	defaultAuthorizationDecision, err := parsePathToRef(raw.DefaultAuthorizationDecision)
	if err != nil {
		return nil, err
	}

	c := &DiscoveredConfig{
		Manager:                      m,
		Plugins:                      plugins,
		DefaultDecision:              defaultDecision,
		DefaultAuthorizationDecision: defaultAuthorizationDecision,
	}

	return c, nil
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

	err = initRegisteredPlugins(m, bs, registeredPlugins)
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

func discoveryHandler(ctx context.Context, config *discoveryConfig, manager *plugins.Manager) ([]byte, error) {
	var discoveryConfig struct {
		Path string `json:"path"`
	}

	if err := util.Unmarshal(config.Discovery, &discoveryConfig); err != nil {
		return nil, err
	}

	if config.Services == nil {
		return nil, fmt.Errorf("endpoint to fetch discovery configuration from not provided")
	}

	var serviceConfig struct {
		Name string `json:"name"`
	}

	if err := util.Unmarshal(config.Services[0], &serviceConfig); err != nil {
		return nil, err
	}

	resp, err := manager.Client(serviceConfig.Name).
		Do(ctx, "GET", discoveryConfig.Path)

	if err != nil {
		return nil, errors.Wrap(err, "Download request failed")
	}

	defer util.Close(resp)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Discovery configuration download failed, server replied with HTTP %v", resp.StatusCode)
	}

	b, err := bundleApi.Read(resp.Body)
	if err != nil {
		return nil, err
	}

	query := "data"
	if discoveryConfig.Path != "" {
		query = fmt.Sprintf("data%v", strings.Replace(discoveryConfig.Path, "/", ".", -1))
	}

	return processBundle(ctx, b, query)
}

func processBundle(ctx context.Context, b bundleApi.Bundle, query string) ([]byte, error) {
	modules := map[string]*ast.Module{}

	for _, file := range b.Modules {
		modules[file.Path] = file.Parsed
	}

	compiler := ast.NewCompiler()
	if compiler.Compile(modules); compiler.Failed() {
		return nil, compiler.Errors
	}

	store := inmem.NewFromObject(b.Data)

	rego := rego.New(
		rego.Query(query),
		rego.Compiler(compiler),
		rego.Store(store),
	)

	rs, err := rego.Eval(ctx)
	if err != nil {
		return nil, err
	}

	if len(rs) == 0 {
		return nil, fmt.Errorf("undefined configuration")
	}

	newConfig, err := json.Marshal(rs[0].Expressions[0].Value)
	if err != nil {
		return nil, err
	}
	return newConfig, nil
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

func initRegisteredPlugins(m *plugins.Manager, bs []byte, registeredPlugins map[string]plugins.PluginInitFunc) error {

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
	}

	return nil

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
