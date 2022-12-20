//go:build usegorillamux

// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package plugins implements plugin management for the policy engine.
package plugins

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/sdk/trace"

	"github.com/gorilla/mux"
	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/bundle"
	"github.com/open-policy-agent/opa/config"
	"github.com/open-policy-agent/opa/keys"
	"github.com/open-policy-agent/opa/loader"
	"github.com/open-policy-agent/opa/logging"
	"github.com/open-policy-agent/opa/plugins/rest"
	"github.com/open-policy-agent/opa/resolver/wasm"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/topdown/cache"
	"github.com/open-policy-agent/opa/topdown/print"
)

// Manager implements lifecycle management of plugins and gives plugins access
// to engine-wide components like storage.
type Manager struct {
	Store  storage.Store
	Config *config.Config
	Info   *ast.Term
	ID     string

	compiler                     *ast.Compiler
	compilerMux                  sync.RWMutex
	wasmResolvers                []*wasm.Resolver
	wasmResolversMtx             sync.RWMutex
	services                     map[string]rest.Client
	keys                         map[string]*keys.Config
	plugins                      []namedplugin
	registeredTriggers           []func(storage.Transaction)
	mtx                          sync.Mutex
	pluginStatus                 map[string]*Status
	pluginStatusListeners        map[string]StatusListener
	initBundles                  map[string]*bundle.Bundle
	initFiles                    loader.Result
	maxErrors                    int
	initialized                  bool
	interQueryBuiltinCacheConfig *cache.Config
	gracefulShutdownPeriod       int
	registeredCacheTriggers      []func(*cache.Config)
	logger                       logging.Logger
	consoleLogger                logging.Logger
	serverInitialized            chan struct{}
	serverInitializedOnce        sync.Once
	printHook                    print.Hook
	enablePrintStatements        bool
	router                       *mux.Router
	prometheusRegister           prometheus.Registerer
	tracerProvider               *trace.TracerProvider
	registeredNDCacheTriggers    []func(bool)
}

func WithRouter(r *mux.Router) func(*Manager) {
	return func(m *Manager) {
		m.router = r
	}
}

// GetRouter returns the managers router if set
func (m *Manager) GetRouter() *mux.Router {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	return m.router
}
