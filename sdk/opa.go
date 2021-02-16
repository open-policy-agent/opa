// Copyright 2021 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package sdk

import (
	"context"
	"crypto/rand"
	"fmt"
	"io/ioutil"
	"strings"
	"sync"
	"time"

	"github.com/open-policy-agent/opa/bundle"
	initload "github.com/open-policy-agent/opa/internal/runtime/init"
	"github.com/open-policy-agent/opa/internal/uuid"
	"github.com/open-policy-agent/opa/loader"
	"github.com/open-policy-agent/opa/logging"
	"github.com/open-policy-agent/opa/metrics"
	"github.com/open-policy-agent/opa/plugins"
	"github.com/open-policy-agent/opa/plugins/discovery"
	"github.com/open-policy-agent/opa/plugins/logs"
	"github.com/open-policy-agent/opa/rego"
	"github.com/open-policy-agent/opa/server"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/storage/inmem"
	iCache "github.com/open-policy-agent/opa/topdown/cache"
)

// OPA represents an instance of the policy engine.
type OPA struct {
	manager                *plugins.Manager
	configBytes            []byte
	paths                  []string
	filter                 loader.Filter
	bundleMode             bool
	store                  storage.Store
	preparedQueries        map[string]*rego.PreparedEvalQuery
	interQueryBuiltinCache iCache.InterQueryCache
	logger                 logging.Logger
	mtx                    *sync.Mutex
}

// Decision response from querying OPA
type Decision struct {
	ID      string
	Data    interface{}
	Metrics metrics.Metrics
}

func newDecision() (*Decision, error) {
	decisionID, err := uuid.New(rand.Reader)
	if err != nil {
		return nil, err
	}

	return &Decision{
		ID:      decisionID,
		Metrics: metrics.New(),
	}, nil
}

// ConfigFile sets the configuration file to use on the OPA instance.
func ConfigFile(fileName string) func(*OPA) error {
	return func(opa *OPA) error {
		bs, err := ioutil.ReadFile(fileName)
		if err != nil {
			return err
		}
		opa.configBytes = bs
		return nil
	}
}

// Config sets the configuration to use on the OPA instance.
func Config(config string) func(*OPA) error {
	return func(opa *OPA) error {
		opa.configBytes = []byte(config)
		return nil
	}
}

// Paths sets the paths to search for policy, data files and bundles.
func Paths(paths []string) func(*OPA) error {
	return func(opa *OPA) error {
		opa.paths = paths
		return nil
	}
}

// Store allows providing storage implementation for OPA.
func Store(store storage.Store) func(*OPA) error {
	return func(opa *OPA) error {
		opa.store = store
		return nil
	}
}

// Logger sets the logging implementation to use for SDK integration.
func Logger(logger logging.Logger) func(*OPA) error {
	return func(opa *OPA) error {
		opa.logger = logger
		return nil
	}
}

// New returns a new OPA object.
func New(opts ...func(*OPA) error) (*OPA, error) {
	opa := &OPA{
		mtx:             new(sync.Mutex),
		preparedQueries: map[string]*rego.PreparedEvalQuery{},
	}

	for _, opt := range opts {
		if err := opt(opa); err != nil {
			return nil, err
		}
	}

	id, err := uuid.New(rand.Reader)
	if err != nil {
		return nil, err
	}

	var pluginInitializers []func(*plugins.Manager)

	if opa.logger == nil {
		opa.logger = logging.NewNoOpLogger()
	}
	pluginInitializers = append(pluginInitializers, plugins.Logger(opa.logger))

	if len(opa.paths) > 0 {
		loaded, err := initload.LoadPaths(opa.paths, opa.filter, opa.bundleMode, nil, false)
		if err != nil {
			return nil, fmt.Errorf("load error %w", err)
		}
		pluginInitializers = append(pluginInitializers, plugins.InitBundles(loaded.Bundles))
		pluginInitializers = append(pluginInitializers, plugins.InitFiles(loaded.Files))
	}

	if opa.store == nil {
		opa.store = inmem.New()
	}

	opa.manager, err = plugins.New(opa.configBytes, id, opa.store, pluginInitializers...)
	if err != nil {
		return nil, err
	}

	discovery, err := discovery.New(opa.manager)
	if err != nil {
		return nil, err
	}

	opa.manager.RegisterCompilerTrigger(opa.compilerUpdated)
	opa.manager.Register("discovery", discovery)

	opa.interQueryBuiltinCache = iCache.NewInterQueryCache(opa.manager.InterQueryBuiltinCacheConfig())

	return opa, nil
}

// StartOPAWaitUntilReady start new OPA and wait until plugins and bundles have been loaded
func StartOPAWaitUntilReady(ctx context.Context, opts ...func(*OPA) error) (*OPA, error) {
	opa, err := New(opts...)
	if err != nil {
		return nil, err
	}
	err = opa.Start(ctx)
	if err != nil {
		return nil, err
	}
	if !opa.AwaitReady(ctx) {
		return nil, fmt.Errorf("timeout waiting for plugins or bundles to be ready")
	}

	return opa, nil
}

// Start asynchronously starts the policy engine's plugins that download
// policies, report status, etc.
func (opa *OPA) Start(ctx context.Context) error {
	return opa.manager.Start(ctx)
}

// Stop asynchronously stops the policy engine's plugins.
func (opa *OPA) Stop(ctx context.Context) {
	opa.manager.Stop(ctx)
}

// Query sends a query to the OPA instance and returns the decision
func (opa *OPA) Query(ctx context.Context, decisionPath string, input interface{}, opts ...func(*rego.Rego)) (decision *Decision, err error) {
	decision, err = newDecision()
	if err != nil {
		return nil, err
	}

	decision.Metrics.Timer("sdk_query").Start()
	defer decision.Metrics.Timer("sdk_query").Stop()

	var bundles map[string]server.BundleInfo

	err = storage.Txn(ctx, opa.manager.Store, storage.TransactionParams{}, func(txn storage.Transaction) error {
		var err error

		if err := opa.getPreparedQuery(decisionPath, txn, decision.Metrics, opts...); err != nil {
			return err
		}
		pq := opa.preparedQueries[decisionPath]

		rs, err := pq.Eval(
			ctx,
			rego.EvalInput(input),
			rego.EvalTransaction(txn),
			rego.EvalMetrics(decision.Metrics),
			rego.EvalInterQueryBuiltinCache(opa.interQueryBuiltinCache),
		)

		if err != nil {
			return err
		} else if len(rs) == 0 {
			return fmt.Errorf("undefined decision")
		} else {
			decision.Data = rs[0].Expressions[0].Value
		}

		bundles, err = opa.bundles(ctx, txn)
		if err != nil {
			return fmt.Errorf("unexpected error %w", err)
		}

		return nil
	})

	if logger := logs.Lookup(opa.manager); logger != nil {
		record := &server.Info{
			Bundles:    bundles,
			DecisionID: decision.ID,
			Timestamp:  time.Now(),
			Query:      decisionPath,
			Input:      &input,
			Error:      err,
			Metrics:    decision.Metrics,
		}
		if err == nil {
			var x = decision.Data
			record.Results = &x
		}

		if err := logger.Log(ctx, record); err != nil {
			return nil, fmt.Errorf("failed to log decision: %w", err)
		}
	}

	return decision, err
}

// QueryDefault sends a query to the default decision path of the OPA instance and returns the decision
func (opa *OPA) QueryDefault(ctx context.Context, input interface{}, opts ...func(*rego.Rego)) (decision *Decision, err error) {
	decisionPath := *opa.manager.Config.DefaultDecision

	return opa.Query(ctx, "data"+strings.ReplaceAll(decisionPath, "/", "."), input, opts...)
}

// AwaitReady waits for plugins and bundles to be loaded or until timeout on context is reached. Returns ready state.
func (opa *OPA) AwaitReady(ctx context.Context) (ready bool) {
	for {
		pluginStatuses := opa.manager.PluginStatus()

		if server.PluginsReady(pluginStatuses) && server.BundlesReady(pluginStatuses) {
			ready = true
			break
		}

		select {
		case <-ctx.Done():
			break
		default:
			time.Sleep(100 * time.Millisecond)
		}
	}

	return ready
}

func (opa *OPA) compilerUpdated(_ storage.Transaction) {
	opa.mtx.Lock()
	defer opa.mtx.Unlock()

	opa.preparedQueries = map[string]*rego.PreparedEvalQuery{}
}

func (opa *OPA) getPreparedQuery(decisionPath string, txn storage.Transaction, m metrics.Metrics, opts ...func(*rego.Rego)) error {
	opa.mtx.Lock()
	defer opa.mtx.Unlock()

	if _, ok := opa.preparedQueries[decisionPath]; !ok {
		opts = append(opts,
			rego.Metrics(m),
			rego.Query(decisionPath),
			rego.Compiler(opa.manager.GetCompiler()),
			rego.Store(opa.manager.Store),
			rego.Transaction(txn),
			rego.Runtime(opa.manager.Info),
		)

		var pq rego.PreparedEvalQuery
		pq, err := rego.New(opts...).PrepareForEval(context.Background())
		if err != nil {
			return err
		}
		opa.preparedQueries[decisionPath] = &pq
	}

	return nil
}

func (opa *OPA) bundles(ctx context.Context, txn storage.Transaction) (map[string]server.BundleInfo, error) {
	bundles := map[string]server.BundleInfo{}

	names, err := bundle.ReadBundleNamesFromStore(ctx, opa.manager.Store, txn)
	if err != nil && !storage.IsNotFound(err) {
		return nil, err
	}
	for _, name := range names {
		r, err := bundle.ReadBundleRevisionFromStore(ctx, opa.manager.Store, txn, name)
		if err != nil && !storage.IsNotFound(err) {
			return nil, err
		}
		bundles[name] = server.BundleInfo{Revision: r}
	}

	return bundles, nil
}
