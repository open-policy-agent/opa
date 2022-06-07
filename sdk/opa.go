// Copyright 2021 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package sdk contains a high-level API for embedding OPA inside of Go programs.
package sdk

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"sync"
	"time"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/bundle"
	"github.com/open-policy-agent/opa/internal/ref"
	"github.com/open-policy-agent/opa/internal/runtime"
	"github.com/open-policy-agent/opa/internal/uuid"
	"github.com/open-policy-agent/opa/logging"
	"github.com/open-policy-agent/opa/metrics"
	"github.com/open-policy-agent/opa/plugins"
	"github.com/open-policy-agent/opa/plugins/discovery"
	"github.com/open-policy-agent/opa/plugins/logs"
	"github.com/open-policy-agent/opa/rego"
	"github.com/open-policy-agent/opa/server"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/storage/inmem"
	"github.com/open-policy-agent/opa/topdown/cache"
	"github.com/open-policy-agent/opa/topdown/print"
)

// OPA represents an instance of the policy engine. OPA can be started with
// several options that control configuration, logging, and lifecycle.
type OPA struct {
	id      string
	state   *state
	mtx     sync.Mutex
	logger  logging.Logger
	console logging.Logger
	plugins map[string]plugins.Factory
	config  []byte
}

type state struct {
	manager                *plugins.Manager
	interQueryBuiltinCache cache.InterQueryCache
	queryCache             *queryCache
}

// New returns a new OPA object. This function should minimally be called with
// options that specify an OPA configuration file.
func New(ctx context.Context, opts Options) (*OPA, error) {

	var err error

	id := opts.ID
	if id == "" {
		id, err = uuid.New(rand.Reader)
		if err != nil {
			return nil, err
		}
	}

	if err := opts.init(); err != nil {
		return nil, err
	}

	opa := &OPA{
		id: id,
		state: &state{
			queryCache: newQueryCache(),
		},
	}

	opa.config = opts.config
	opa.logger = opts.Logger
	opa.console = opts.ConsoleLogger
	opa.plugins = opts.Plugins

	return opa, opa.configure(ctx, opa.config, opts.Ready, opts.block)
}

// Plugin returns the named plugin. If the plugin does not exist, this function
// returns nil.
func (opa *OPA) Plugin(name string) plugins.Plugin {
	opa.mtx.Lock()
	defer opa.mtx.Unlock()
	return opa.state.manager.Plugin(name)
}

// Configure updates the configuration of the OPA in-place. This function should
// be called in response to configuration updates in the environment. This
// function is atomic. If the configuration update cannot be successfully
// applied, the old configuration will remain intact.
func (opa *OPA) Configure(ctx context.Context, opts ConfigOptions) error {

	if err := opts.init(); err != nil {
		return err
	}

	// NOTE(tsandall): In future we could be more intelligent about
	// re-configuration and avoid expensive background processing.
	opa.mtx.Lock()
	equal := bytes.Equal(opts.config, opa.config)
	opa.mtx.Unlock()

	if equal {
		close(opts.Ready)
		return nil
	}

	return opa.configure(ctx, opts.config, opts.Ready, opts.block)
}

func (opa *OPA) configure(ctx context.Context, bs []byte, ready chan struct{}, block bool) error {
	info, err := runtime.Term(runtime.Params{Config: opa.config})
	if err != nil {
		return err
	}

	manager, err := plugins.New(
		bs,
		opa.id,
		inmem.New(),
		plugins.Info(info),
		plugins.Logger(opa.logger),
		plugins.ConsoleLogger(opa.console),
		plugins.EnablePrintStatements(opa.logger.GetLevel() >= logging.Info),
		plugins.PrintHook(loggingPrintHook{logger: opa.logger}))
	if err != nil {
		return err
	}

	manager.RegisterCompilerTrigger(func(storage.Transaction) {
		opa.mtx.Lock()
		opa.state.queryCache.Clear()
		opa.mtx.Unlock()
	})

	manager.RegisterPluginStatusListener("sdk", func(status map[string]*plugins.Status) {

		select {
		case <-ready:
			return
		default:
		}
		// NOTE(tsandall): we do not include a special case for the discovery
		// plugin. If the discovery plugin is the only plugin and it goes ready,
		// then OPA will be considered ready. The discovery plugin only goes ready
		// _after_ it has successfully processed a discovery bundle. During
		// discovery bundle processing, other plugins will register so their states
		// will be accounted for. If a discovery bundle did not enable any other
		// plugins (bundles, etc.) the OPA will still be operational.
		for _, s := range status {
			if s.State != plugins.StateOK {
				return
			}
		}

		close(ready)
	})

	d, err := discovery.New(manager, discovery.Factories(opa.plugins))
	if err != nil {
		return err
	}

	manager.Register(discovery.Name, d)

	if err := manager.Start(ctx); err != nil {
		return err
	}

	if block {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ready:
		}
	}

	opa.mtx.Lock()
	defer opa.mtx.Unlock()

	// NOTE(tsandall): there is no return value from Stop() and it could block
	// on async operations (e.g., decision log uploading) so defer the call to
	// another goroutine.
	//
	// TODO(tsandall): if we need to block on operations like decision log
	// uploading, perhaps we could rely on a manual trigger.
	previousManager := opa.state.manager
	go func() {
		if previousManager != nil {
			previousManager.Stop(ctx)
		}
	}()

	opa.state.manager = manager
	opa.state.queryCache.Clear()
	opa.state.interQueryBuiltinCache = cache.NewInterQueryCache(manager.InterQueryBuiltinCacheConfig())
	opa.config = bs

	return nil
}

// Stop closes the OPA. The OPA cannot be restarted.
func (opa *OPA) Stop(ctx context.Context) {

	opa.mtx.Lock()
	mgr := opa.state.manager
	opa.mtx.Unlock()

	if mgr != nil {
		mgr.Stop(ctx)
	}
}

// Decision returns a named decision. This function is threadsafe.
func (opa *OPA) Decision(ctx context.Context, options DecisionOptions) (*DecisionResult, error) {

	record := server.Info{
		Timestamp: options.Now,
		Path:      options.Path,
		Input:     &options.Input,
	}

	result, err := opa.executeTransaction(
		ctx,
		&record,
		func(s state, result *DecisionResult) {
			result.Result, record.InputAST, record.Bundles, record.Error = evaluate(ctx, evalArgs{
				runtime:         s.manager.Info,
				printHook:       s.manager.PrintHook(),
				compiler:        s.manager.GetCompiler(),
				store:           s.manager.Store,
				queryCache:      s.queryCache,
				interQueryCache: s.interQueryBuiltinCache,
				txn:             record.Txn,
				now:             record.Timestamp,
				path:            record.Path,
				input:           *record.Input,
				m:               record.Metrics,
			})
			if record.Error == nil {
				record.Results = &result.Result
			}
		},
	)
	if err != nil {
		return nil, err
	}

	return result, record.Error
}

// DecisionOptions contains parameters for query evaluation.
type DecisionOptions struct {
	Now   time.Time   // specifies wallclock time used for time.now_ns(), decision log timestamp, etc.
	Path  string      // specifies name of policy decision to evaluate (e.g., example/allow)
	Input interface{} // specifies value of the input document to evaluate policy with
}

// DecisionResult contains the output of query evaluation.
type DecisionResult struct {
	ID     string      // provides a globally unique identifier for this decision (which is included in the decision log.)
	Result interface{} // provides the output of query evaluation.
}

func newDecisionResult() (*DecisionResult, error) {
	id, err := uuid.New(rand.Reader)
	if err != nil {
		return nil, err
	}
	result := &DecisionResult{ID: id}
	return result, nil
}

func (opa *OPA) executeTransaction(ctx context.Context, record *server.Info, work func(state, *DecisionResult)) (*DecisionResult, error) {
	m := metrics.New()
	m.Timer(metrics.SDKDecisionEval).Start()

	result, err := newDecisionResult()
	if err != nil {
		return nil, err
	}

	opa.mtx.Lock()
	s := *opa.state
	opa.mtx.Unlock()

	record.DecisionID = result.ID
	record.Metrics = m

	if record.Timestamp.IsZero() {
		record.Timestamp = time.Now().UTC()
	}

	if record.Path == "" {
		record.Path = *s.manager.Config.DefaultDecision
	}

	record.Txn, record.Error = s.manager.Store.NewTransaction(ctx, storage.TransactionParams{})

	if record.Error == nil {
		defer s.manager.Store.Abort(ctx, record.Txn)
		work(s, result)
	}

	m.Timer(metrics.SDKDecisionEval).Stop()

	if logger := logs.Lookup(s.manager); logger != nil {
		if err := logger.Log(ctx, record); err != nil {
			return result, fmt.Errorf("decision log: %w", err)
		}
	}
	return result, nil
}

// Partial returns a named decision. This function is threadsafe.
func (opa *OPA) Partial(ctx context.Context, options PartialOptions) (*PartialResult, error) {

	if options.Mapper == nil {
		options.Mapper = &RawMapper{}
	}

	record := server.Info{
		Timestamp: options.Now,
		Input:     &options.Input,
		Query:     options.Query,
	}

	var pq *rego.PartialQueries
	decision, err := opa.executeTransaction(
		ctx,
		&record,
		func(s state, result *DecisionResult) {
			pq, record.InputAST, record.Bundles, record.Error = partial(ctx, partialEvalArgs{
				runtime:   s.manager.Info,
				printHook: s.manager.PrintHook(),
				compiler:  s.manager.GetCompiler(),
				store:     s.manager.Store,
				txn:       record.Txn,
				now:       record.Timestamp,
				query:     record.Query,
				unknowns:  options.Unknowns,
				input:     *record.Input,
				m:         record.Metrics,
			})
			if record.Error == nil {
				result.Result, record.Error = options.Mapper.MapResults(pq)
				var pqAst interface{}
				if record.Error == nil {
					var mappedResults interface{}
					mappedResults, record.Error = options.Mapper.ResultToJSON(result.Result)
					record.MappedResults = &mappedResults
					pqAst = pq
					record.Results = &pqAst
				}
			}
		},
	)
	if err != nil {
		return nil, err
	}

	return &PartialResult{
		ID:     decision.ID,
		Result: decision.Result,
		AST:    pq,
	}, record.Error
}

type PartialQueryMapper interface {
	// The first interface being returned is the type that will be used for further processing
	MapResults(pq *rego.PartialQueries) (interface{}, error)
	// This should be able to take the Result object from MapResults and return a type that can be logged as JSON
	ResultToJSON(result interface{}) (interface{}, error)
}

// PartialOptions contains parameters for partial query evaluation.
type PartialOptions struct {
	Now      time.Time          // specifies wallclock time used for time.now_ns(), decision log timestamp, etc.
	Input    interface{}        // specifies value of the input document to evaluate policy with
	Query    string             // specifies the query to be partially evaluated
	Unknowns []string           // specifies the unknown elements of the policy
	Mapper   PartialQueryMapper // specifies the mapper to use when processing results
}

type PartialResult struct {
	ID     string               // decision ID
	Result interface{}          // mapped result
	AST    *rego.PartialQueries // raw result
}

// Error represents an internal error in the SDK.
type Error struct {
	Code    string `json:"code"`
	Message string `json:"message,omitempty"`
}

func (err *Error) Error() string {
	return fmt.Sprintf("%v: %v", err.Code, err.Message)
}

const (
	// UndefinedErr indicates that the queried decision was undefined.
	UndefinedErr = "opa_undefined_error"
)

func undefinedDecisionErr(path string) *Error {
	return &Error{
		Code:    UndefinedErr,
		Message: fmt.Sprintf("%v decision was undefined", path),
	}
}

// IsUndefinedErr returns true of the err represents an undefined decision error.
func IsUndefinedErr(err error) bool {
	actual, ok := err.(*Error)
	return ok && actual.Code == UndefinedErr
}

type evalArgs struct {
	runtime         *ast.Term
	printHook       print.Hook
	compiler        *ast.Compiler
	store           storage.Store
	txn             storage.Transaction
	queryCache      *queryCache
	interQueryCache cache.InterQueryCache
	now             time.Time
	path            string
	input           interface{}
	m               metrics.Metrics
}

func evaluate(ctx context.Context, args evalArgs) (interface{}, ast.Value, map[string]server.BundleInfo, error) {

	bundles, err := bundles(ctx, args.store, args.txn)
	if err != nil {
		return nil, nil, nil, err
	}

	r, err := ref.ParseDataPath(args.path)
	if err != nil {
		return nil, nil, bundles, err
	}

	pq, err := args.queryCache.Get(r.String(), func(query string) (*rego.PreparedEvalQuery, error) {
		pq, err := rego.New(
			rego.Time(args.now),
			rego.Metrics(args.m),
			rego.Query(query),
			rego.Compiler(args.compiler),
			rego.Store(args.store),
			rego.Transaction(args.txn),
			rego.PrintHook(args.printHook),
			rego.Runtime(args.runtime)).PrepareForEval(ctx)
		if err != nil {
			return nil, err
		}
		return &pq, err
	})
	if err != nil {
		return nil, nil, bundles, err
	}

	inputAST, err := ast.InterfaceToValue(args.input)
	if err != nil {
		return nil, nil, bundles, err
	}

	rs, err := pq.Eval(
		ctx,
		rego.EvalTime(args.now),
		rego.EvalParsedInput(inputAST),
		rego.EvalTransaction(args.txn),
		rego.EvalMetrics(args.m),
		rego.EvalInterQueryBuiltinCache(args.interQueryCache),
	)
	if err != nil {
		return nil, inputAST, bundles, err
	} else if len(rs) == 0 {
		return nil, inputAST, bundles, undefinedDecisionErr(args.path)
	}

	return rs[0].Expressions[0].Value, inputAST, bundles, nil
}

type partialEvalArgs struct {
	runtime   *ast.Term
	compiler  *ast.Compiler
	printHook print.Hook
	store     storage.Store
	txn       storage.Transaction
	unknowns  []string
	query     string
	now       time.Time
	input     interface{}
	m         metrics.Metrics
}

func partial(ctx context.Context, args partialEvalArgs) (*rego.PartialQueries, ast.Value, map[string]server.BundleInfo, error) {

	bundles, err := bundles(ctx, args.store, args.txn)
	if err != nil {
		return nil, nil, nil, err
	}

	inputAST, err := ast.InterfaceToValue(args.input)
	if err != nil {
		return nil, nil, bundles, err
	}
	re := rego.New(
		rego.Time(args.now),
		rego.Metrics(args.m),
		rego.Store(args.store),
		rego.Compiler(args.compiler),
		rego.Transaction(args.txn),
		rego.Runtime(args.runtime),
		rego.Input(args.input),
		rego.Query(args.query),
		rego.Unknowns(args.unknowns),
		rego.PrintHook(args.printHook),
	)

	pq, err := re.Partial(ctx)
	if err != nil {
		return nil, nil, bundles, err
	}
	return pq, inputAST, bundles, err
}

type queryCache struct {
	sync.Mutex
	cache map[string]*rego.PreparedEvalQuery
}

func newQueryCache() *queryCache {
	return &queryCache{cache: map[string]*rego.PreparedEvalQuery{}}
}

func (qc *queryCache) Get(key string, orElse func(string) (*rego.PreparedEvalQuery, error)) (*rego.PreparedEvalQuery, error) {
	qc.Lock()
	defer qc.Unlock()

	result, ok := qc.cache[key]
	if ok {
		return result, nil
	}

	result, err := orElse(key)
	if err != nil {
		return nil, err
	}

	qc.cache[key] = result
	return result, nil
}

func (qc *queryCache) Clear() {
	qc.Lock()
	defer qc.Unlock()

	qc.cache = make(map[string]*rego.PreparedEvalQuery)
}

func bundles(ctx context.Context, store storage.Store, txn storage.Transaction) (map[string]server.BundleInfo, error) {
	bundles := map[string]server.BundleInfo{}
	names, err := bundle.ReadBundleNamesFromStore(ctx, store, txn)
	if err != nil && !storage.IsNotFound(err) {
		return nil, fmt.Errorf("failed to read bundle names: %w", err)
	}
	for _, name := range names {
		r, err := bundle.ReadBundleRevisionFromStore(ctx, store, txn, name)
		if err != nil {
			return nil, fmt.Errorf("failed to read bundle revisions: %w", err)
		}
		bundles[name] = server.BundleInfo{Revision: r}
	}
	return bundles, nil
}

type loggingPrintHook struct {
	logger logging.Logger
}

func (h loggingPrintHook) Print(pctx print.Context, msg string) error {
	h.logger.WithFields(map[string]interface{}{"line": pctx.Location.String()}).Info(msg)
	return nil
}
