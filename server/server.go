// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"strconv"
	"strings"
	"sync"
	"time"

	"crypto/tls"

	"net/url"

	"github.com/gorilla/mux"
	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/metrics"
	"github.com/open-policy-agent/opa/rego"
	"github.com/open-policy-agent/opa/server/authorizer"
	"github.com/open-policy-agent/opa/server/identifier"
	"github.com/open-policy-agent/opa/server/types"
	"github.com/open-policy-agent/opa/server/writer"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/topdown"
	"github.com/open-policy-agent/opa/topdown/explain"
	"github.com/open-policy-agent/opa/util"
	"github.com/open-policy-agent/opa/version"
	"github.com/open-policy-agent/opa/watch"
	"github.com/pkg/errors"
)

// AuthenticationScheme enumerates the supported authentication schemes. The
// authentication scheme determines how client identities are established.
type AuthenticationScheme int

// Set of supported authentication schemes.
const (
	AuthenticationOff   AuthenticationScheme = iota
	AuthenticationToken                      = iota
)

// AuthorizationScheme enumerates the supported authorization schemes. The authorization
// scheme determines how access to OPA is controlled.
type AuthorizationScheme int

// Set of supported authorization schemes.
const (
	AuthorizationOff   AuthorizationScheme = iota
	AuthorizationBasic                     = iota
)

// DefaultDiagnosticsBufferSize is the default size of the server's diagnostic buffer.
const DefaultDiagnosticsBufferSize = 10

var systemMainPath = ast.MustParseRef("data.system.main")

// Server represents an instance of OPA running in server mode.
type Server struct {
	Handler http.Handler

	addr           string
	insecureAddr   string
	authentication AuthenticationScheme
	authorization  AuthorizationScheme
	cert           *tls.Certificate
	mtx            sync.RWMutex
	compiler       *ast.Compiler
	store          storage.Store
	watcher        *watch.Watcher

	diagnostics Buffer

	errLimit int
}

// New returns a new Server.
func New() *Server {

	s := Server{}

	// Initialize HTTP handlers.
	router := mux.NewRouter()
	s.registerHandler(router, 0, "/data/{path:.+}", http.MethodPost, s.v0DataPost)
	s.registerHandler(router, 0, "/data", http.MethodPost, s.v0DataPost)
	s.registerHandler(router, 1, "/data/{path:.+}", http.MethodPut, s.v1DataPut)
	s.registerHandler(router, 1, "/data", http.MethodPut, s.v1DataPut)
	s.registerHandler(router, 1, "/data/system/diagnostics", http.MethodGet, s.v1DiagnosticsGet)
	s.registerHandler(router, 1, "/data/{path:.+}", http.MethodGet, s.v1DataGet)
	s.registerHandler(router, 1, "/data", http.MethodGet, s.v1DataGet)
	s.registerHandler(router, 1, "/data/{path:.+}", http.MethodPatch, s.v1DataPatch)
	s.registerHandler(router, 1, "/data", http.MethodPatch, s.v1DataPatch)
	s.registerHandler(router, 1, "/data/{path:.+}", http.MethodPost, s.v1DataPost)
	s.registerHandler(router, 1, "/data", http.MethodPost, s.v1DataPost)
	s.registerHandler(router, 1, "/policies", http.MethodGet, s.v1PoliciesList)
	s.registerHandler(router, 1, "/policies/{path:.+}", http.MethodDelete, s.v1PoliciesDelete)
	s.registerHandler(router, 1, "/policies/{path:.+}", http.MethodGet, s.v1PoliciesGet)
	s.registerHandler(router, 1, "/policies/{path:.+}", http.MethodPut, s.v1PoliciesPut)
	s.registerHandler(router, 1, "/query", http.MethodGet, s.v1QueryGet)
	router.HandleFunc("/", s.unversionedPost).Methods(http.MethodPost)
	router.HandleFunc("/", s.indexGet).Methods(http.MethodGet)
	s.Handler = router

	s.diagnostics = NewBoundedBuffer(DefaultDiagnosticsBufferSize)
	return &s
}

// Init initializes the server. This function MUST be called before Loop.
func (s *Server) Init(ctx context.Context) (*Server, error) {

	// Add authorization handler. This must come BEFORE authentication handler
	// so that the latter can run first.
	switch s.authorization {
	case AuthorizationBasic:
		s.Handler = authorizer.NewBasic(s.Handler, s.Compiler, s.store)
	}

	switch s.authentication {
	case AuthenticationToken:
		s.Handler = identifier.NewTokenBased(s.Handler)
	}

	txn, err := s.store.NewTransaction(ctx, storage.WriteParams)
	if err != nil {
		return nil, err
	}

	// Load policies from storage and initialize server's cached compiler.
	if err := s.reloadCompiler(ctx, txn); err != nil {
		s.store.Abort(ctx, txn)
		return nil, err
	}

	// Register triggers so that if runtime reloads the policies, the
	// server sees the change.
	config := storage.TriggerConfig{
		OnCommit: s.reload,
	}
	if _, err := s.store.Register(ctx, txn, config); err != nil {
		s.store.Abort(ctx, txn)
		return nil, err
	}
	s.watcher, err = watch.New(ctx, s.store, s.Compiler(), txn)
	if err != nil {
		return nil, err
	}

	return s, s.store.Commit(ctx, txn)
}

// WithAddress sets the listening address that the server will bind to.
func (s *Server) WithAddress(addr string) *Server {
	s.addr = addr
	return s
}

// WithInsecureAddress sets the listening address that the server will bind to.
func (s *Server) WithInsecureAddress(addr string) *Server {
	s.insecureAddr = addr
	return s
}

// WithAuthentication sets authentication scheme to use on the server.
func (s *Server) WithAuthentication(scheme AuthenticationScheme) *Server {
	s.authentication = scheme
	return s
}

// WithAuthorization sets authorization scheme to use on the server.
func (s *Server) WithAuthorization(scheme AuthorizationScheme) *Server {
	s.authorization = scheme
	return s
}

// WithCertificate sets the server-side certificate that the server will use.
func (s *Server) WithCertificate(cert *tls.Certificate) *Server {
	s.cert = cert
	return s
}

// WithStore sets the storage used by the server.
func (s *Server) WithStore(store storage.Store) *Server {
	s.store = store
	return s
}

// WithCompilerErrorLimit sets the limit on the number of compiler errors the server will
// allow.
func (s *Server) WithCompilerErrorLimit(limit int) *Server {
	s.errLimit = limit
	return s
}

// WithDiagnosticsBuffer sets the diagnostics buffer stored by the server.
func (s *Server) WithDiagnosticsBuffer(buf Buffer) *Server {
	s.diagnostics = buf
	return s
}

// Compiler returns the server's compiler.
//
// The server's compiler contains the compiled versions of all modules added to
// the server as well as data structures for performing query analysis. This is
// intended to allow services to embed the OPA server while still relying on the
// topdown package for query evaluation.
func (s *Server) Compiler() *ast.Compiler {
	s.mtx.RLock()
	defer s.mtx.RUnlock()
	return s.compiler
}

// Listeners returns functions that listen and serve connections.
func (s *Server) Listeners() (func() error, func() error) {

	server1 := http.Server{
		Addr:    s.addr,
		Handler: s.Handler,
	}

	loop1 := func() error { return server1.ListenAndServe() }

	if s.cert == nil {
		return loop1, nil
	}

	server2 := http.Server{
		Addr:    s.addr,
		Handler: s.Handler,
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{*s.cert},
		},
	}

	loop2 := func() error { return server2.ListenAndServeTLS("", "") }

	if s.insecureAddr == "" {
		return loop2, nil
	}

	server1.Addr = s.insecureAddr

	return loop2, loop1
}

func (s *Server) execQuery(ctx context.Context, r *http.Request, query string, input ast.Value, explainMode types.ExplainModeV1, includeMetrics, pretty bool) (results types.QueryResponseV1, err error) {

	settings := s.evalDiagnosticPolicy(r)

	var buf *topdown.BufferTracer
	if explainMode != types.ExplainOffV1 || settings.explain {
		buf = topdown.NewBufferTracer()
	}

	m := metrics.New()

	compiler := s.Compiler()
	rego := rego.New(
		rego.Store(s.store),
		rego.Compiler(compiler),
		rego.Query(query),
		rego.Input(input),
		rego.Metrics(m),
		rego.Tracer(buf),
	)

	output, err := rego.Eval(ctx)
	if err != nil {
		s.logDiagnostics(query, input, nil, err, m, buf, settings)
		return results, err
	}

	for _, result := range output {
		results.Result = append(results.Result, result.Bindings.WithoutWildcards())
	}

	if includeMetrics {
		results.Metrics = m.All()
	}

	if explainMode != types.ExplainOffV1 {
		results.Explanation = s.getExplainResponse(explainMode, *buf, pretty)
	}

	var x interface{} = results.Result
	s.logDiagnostics(query, input, &x, nil, m, buf, settings)
	return results, nil
}

func (s *Server) indexGet(w http.ResponseWriter, r *http.Request) {

	renderHeader(w)
	renderBanner(w)
	renderVersion(w)
	defer renderFooter(w)

	values := r.URL.Query()
	qStrs := values[types.ParamQueryV1]
	inputStrs := values[types.ParamInputV1]

	explainMode := getExplain(r.URL.Query()[types.ParamExplainV1], types.ExplainOffV1)
	ctx := r.Context()

	renderQueryForm(w, qStrs, inputStrs, explainMode)

	if len(qStrs) == 0 {
		return
	}

	qStr := qStrs[len(qStrs)-1]
	t0 := time.Now()

	var input ast.Value
	if len(inputStrs) > 0 && len(inputStrs[len(qStrs)-1]) > 0 {
		t, err := ast.ParseTerm(inputStrs[len(qStrs)-1])
		if err != nil {
			renderQueryResult(w, nil, err, t0)
			return
		}
		input = t.Value
	}

	results, err := s.execQuery(ctx, r, qStr, input, explainMode, false, true)
	if err != nil {
		renderQueryResult(w, nil, err, t0)
		return
	}

	renderQueryResult(w, results, err, t0)
}

func (s *Server) registerHandler(router *mux.Router, version int, path string, method string, h func(http.ResponseWriter, *http.Request)) {
	prefix := fmt.Sprintf("/v%d", version)
	router.HandleFunc(prefix+path, h).Methods(method)
}

func (s *Server) reload(ctx context.Context, txn storage.Transaction, event storage.TriggerEvent) {

	if !event.PolicyChanged() {
		return
	}

	err := s.reloadCompiler(ctx, txn)
	if err != nil {
		panic(err)
	}

	s.watcher, err = s.watcher.Migrate(s.Compiler(), txn)
	if err != nil {
		// The only way migration can fail is if the old watcher is closed or if
		// the new one cannot register a trigger with the store. Since we're
		// using an inmem store with a write transaction, neither of these should
		// be possible.
		panic(err)
	}
}

func (s *Server) reloadCompiler(ctx context.Context, txn storage.Transaction) error {

	modules, err := s.loadModules(ctx, txn)
	if err != nil {
		return err
	}

	compiler := ast.NewCompiler().SetErrorLimit(s.errLimit)

	if compiler.Compile(modules); compiler.Failed() {
		return compiler.Errors
	}

	s.setCompiler(compiler)
	return nil
}

func (s *Server) unversionedPost(w http.ResponseWriter, r *http.Request) {
	s.v0QueryPath(w, r, systemMainPath)
}

func (s *Server) v0DataPost(w http.ResponseWriter, r *http.Request) {
	path := stringPathToDataRef(mux.Vars(r)["path"])
	s.v0QueryPath(w, r, path)
}

func (s *Server) v0QueryPath(w http.ResponseWriter, r *http.Request, path ast.Ref) {
	ctx := r.Context()

	settings := s.evalDiagnosticPolicy(r)
	input, err := readInputV0(r.Body)
	if err != nil {
		writer.ErrorString(w, http.StatusBadRequest, types.CodeInvalidParameter, errors.Wrapf(err, "unexpected parse error for input"))
		return
	}

	var goInput interface{}
	if input != nil {
		if goInput, err = ast.JSON(input); err != nil {
			writer.ErrorString(w, http.StatusInternalServerError, types.CodeInvalidParameter, errors.Wrapf(err, "could not marshal input"))
			return
		}
	}

	// Prepare for query.
	txn, err := s.store.NewTransaction(ctx)
	if err != nil {
		writer.ErrorAuto(w, err)
		return
	}

	defer s.store.Abort(ctx, txn)

	compiler := s.Compiler()

	params := topdown.NewQueryParams(ctx, compiler, s.store, txn, input, path)

	var m metrics.Metrics
	var buf *topdown.BufferTracer
	if settings.explain {
		buf = topdown.NewBufferTracer()
		params.Tracer = buf
	}

	if settings.metrics {
		m = metrics.New()
		params.Metrics = m
	}

	// Execute query.
	qrs, err := topdown.Query(params)

	// Handle results.
	if err != nil {
		s.logDiagnostics(path.String(), goInput, nil, err, m, buf, settings)
		writer.ErrorAuto(w, err)
		return
	}

	if qrs.Undefined() {
		writer.Error(w, 404, types.NewErrorV1(types.CodeUndefinedDocument, fmt.Sprintf("%v: %v", types.MsgUndefinedError, path)))
		return
	}

	s.logDiagnostics(path.String(), goInput, &qrs[0].Result, nil, m, buf, settings)
	writer.JSON(w, 200, qrs[0].Result, false)
}

func (s *Server) v1DiagnosticsGet(w http.ResponseWriter, r *http.Request) {
	pretty := getBoolParam(r.URL, types.ParamPrettyV1, true)
	explainMode := getExplain(r.URL.Query()[types.ParamExplainV1], types.ExplainTruthV1)

	var results []types.DiagnosticsResponseElementV1
	s.diagnostics.Iter(func(i *Info) {
		result := types.DiagnosticsResponseElementV1{
			Timestamp: i.Timestamp.UnixNano(),
			Query:     i.Query,
			Input:     i.Input,
		}

		if i.Metrics != nil {
			result.Metrics = i.Metrics.All()
		}

		if i.Trace != nil {
			result.Explanation = s.getExplainResponse(explainMode, i.Trace, pretty)
		}

		if i.Error != nil {
			result.Error = types.NewErrorV1(types.CodeInternal, i.Error.Error())
		} else {
			result.Result = i.Results
		}

		results = append(results, result)
	})

	writer.JSON(w, 200, results, pretty)
}

func (s *Server) v1DataGet(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	path := stringPathToDataRef(vars["path"])
	settings := s.evalDiagnosticPolicy(r)

	watch := getWatch(r.URL.Query()[types.ParamWatchV1])
	if watch {
		s.watchQuery(path.String(), w, r, true)
		return
	}

	pretty := getBoolParam(r.URL, types.ParamPrettyV1, true)
	explainMode := getExplain(r.URL.Query()["explain"], types.ExplainOffV1)
	includeMetrics := getBoolParam(r.URL, types.ParamMetricsV1, true)

	m := metrics.New()
	m.Timer(metrics.RegoQueryParse).Start()

	inputs := r.URL.Query()[types.ParamInputV1]

	var input ast.Value
	if len(inputs) > 0 {
		var err error
		input, err = readInputGetV1(inputs[len(inputs)-1])
		if err != nil {
			writer.ErrorString(w, http.StatusBadRequest, types.CodeInvalidParameter, err)
			return
		}
	}

	m.Timer(metrics.RegoQueryParse).Stop()

	var goInput interface{}
	if input != nil {
		var err error
		if goInput, err = ast.JSON(input); err != nil {
			writer.ErrorString(w, http.StatusInternalServerError, types.CodeInvalidParameter, errors.Wrapf(err, "could not marshal input"))
			return
		}
	}

	// Prepare for query.
	txn, err := s.store.NewTransaction(ctx)
	if err != nil {
		writer.ErrorAuto(w, err)
		return
	}

	defer s.store.Abort(ctx, txn)

	compiler := s.Compiler()
	params := topdown.NewQueryParams(ctx, compiler, s.store, txn, input, path)
	params.Metrics = m

	var buf *topdown.BufferTracer
	if explainMode != types.ExplainOffV1 || settings.explain {
		buf = topdown.NewBufferTracer()
		params.Tracer = buf
	}

	// Execute query.
	qrs, err := topdown.Query(params)

	// Handle results.
	if err != nil {
		s.logDiagnostics(path.String(), goInput, nil, err, m, buf, settings)
		writer.ErrorAuto(w, err)
		return
	}

	result := types.DataResponseV1{}

	if includeMetrics {
		result.Metrics = m.All()
	}

	if qrs.Undefined() {
		if explainMode == types.ExplainFullV1 {
			result.Explanation, err = types.NewTraceV1(*buf, pretty)
			if err != nil {
				writer.ErrorAuto(w, err)
			}
		}
		s.logDiagnostics(path.String(), goInput, nil, nil, m, buf, settings)
		writer.JSON(w, 200, result, pretty)
		return
	}
	result.Result = &qrs[0].Result

	s.logDiagnostics(path.String(), goInput, result.Result, nil, m, buf, settings)

	if explainMode != types.ExplainOffV1 {
		result.Explanation = s.getExplainResponse(explainMode, *buf, pretty)
	}
	writer.JSON(w, 200, result, pretty)
}

func (s *Server) v1DataPatch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)

	ops := []types.PatchV1{}

	if err := util.NewJSONDecoder(r.Body).Decode(&ops); err != nil {
		writer.ErrorString(w, http.StatusBadRequest, types.CodeInvalidParameter, err)
		return
	}

	patches, err := s.prepareV1PatchSlice(vars["path"], ops)
	if err != nil {
		writer.ErrorAuto(w, err)
		return
	}

	txn, err := s.store.NewTransaction(ctx, storage.WriteParams)
	if err != nil {
		writer.ErrorAuto(w, err)
		return
	}

	for _, patch := range patches {
		if err := s.store.Write(ctx, txn, patch.op, patch.path, patch.value); err != nil {
			s.abortAuto(ctx, txn, w, err)
			return
		}
	}

	if err := s.store.Commit(ctx, txn); err != nil {
		writer.ErrorAuto(w, err)
	} else {
		writer.Bytes(w, 204, nil)
	}
}

func (s *Server) v1DataPost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	path := stringPathToDataRef(vars["path"])
	settings := s.evalDiagnosticPolicy(r)

	watch := getWatch(r.URL.Query()[types.ParamWatchV1])
	if watch {
		s.watchQuery(path.String(), w, r, true)
		return
	}

	pretty := getBoolParam(r.URL, types.ParamPrettyV1, true)
	explainMode := getExplain(r.URL.Query()[types.ParamExplainV1], types.ExplainOffV1)
	includeMetrics := getBoolParam(r.URL, types.ParamMetricsV1, true)

	m := metrics.New()

	m.Timer(metrics.RegoQueryParse).Start()

	input, err := readInputPostV1(r.Body)
	if err != nil {
		writer.ErrorString(w, http.StatusBadRequest, types.CodeInvalidParameter, err)
		return
	}

	var goInput interface{}
	if input != nil {
		if goInput, err = ast.JSON(input); err != nil {
			writer.ErrorString(w, http.StatusInternalServerError, types.CodeInvalidParameter, errors.Wrapf(err, "could not marshal input"))
			return
		}
	}

	m.Timer(metrics.RegoQueryParse).Stop()

	// Prepare for query.
	txn, err := s.store.NewTransaction(ctx)
	if err != nil {
		writer.ErrorAuto(w, err)
		return
	}

	defer s.store.Abort(ctx, txn)

	compiler := s.Compiler()
	params := topdown.NewQueryParams(ctx, compiler, s.store, txn, input, path)

	params.Metrics = m

	var buf *topdown.BufferTracer
	if explainMode != types.ExplainOffV1 || settings.explain {
		buf = topdown.NewBufferTracer()
		params.Tracer = buf
	}

	// Execute query.
	qrs, err := topdown.Query(params)

	// Handle results.
	if err != nil {
		s.logDiagnostics(path.String(), goInput, nil, err, m, buf, settings)
		writer.ErrorAuto(w, err)
		return
	}

	result := types.DataResponseV1{}
	if includeMetrics {
		result.Metrics = m.All()
	}

	if qrs.Undefined() {
		if explainMode == types.ExplainFullV1 {
			result.Explanation, err = types.NewTraceV1(*buf, pretty)
			if err != nil {
				writer.ErrorAuto(w, err)
			}
		}
		s.logDiagnostics(path.String(), goInput, nil, nil, m, buf, settings)
		writer.JSON(w, 200, result, pretty)
		return
	}

	result.Result = &qrs[0].Result

	s.logDiagnostics(path.String(), goInput, result.Result, nil, m, buf, settings)

	if explainMode != types.ExplainOffV1 {
		result.Explanation = s.getExplainResponse(explainMode, *buf, pretty)
	}
	writer.JSON(w, 200, result, pretty)
}

func (s *Server) v1DataPut(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)

	var value interface{}
	if err := util.NewJSONDecoder(r.Body).Decode(&value); err != nil {
		writer.ErrorString(w, http.StatusBadRequest, types.CodeInvalidParameter, err)
		return
	}

	path, ok := storage.ParsePath("/" + strings.Trim(vars["path"], "/"))
	if !ok {
		writer.Error(w, http.StatusBadRequest, types.NewErrorV1(types.CodeInvalidParameter, "bad path: %v", vars["path"]))
		return
	}

	txn, err := s.store.NewTransaction(ctx, storage.WriteParams)
	if err != nil {
		writer.ErrorAuto(w, err)
		return
	}

	_, err = s.store.Read(ctx, txn, path)

	if err != nil {
		if !storage.IsNotFound(err) {
			s.abortAuto(ctx, txn, w, err)
			return
		}
		if err := s.makeDir(ctx, txn, path[:len(path)-1]); err != nil {
			s.abortAuto(ctx, txn, w, err)
			return
		}
	} else if r.Header.Get("If-None-Match") == "*" {
		s.store.Abort(ctx, txn)
		writer.Bytes(w, 304, nil)
		return
	}

	if err := s.store.Write(ctx, txn, storage.AddOp, path, value); err != nil {
		s.abortAuto(ctx, txn, w, err)
		return
	}

	if err := s.store.Commit(ctx, txn); err != nil {
		writer.ErrorAuto(w, err)
	} else {
		writer.Bytes(w, 204, nil)
	}
}

func (s *Server) v1PoliciesDelete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	pretty := getBoolParam(r.URL, types.ParamPrettyV1, true)
	includeMetrics := getBoolParam(r.URL, types.ParamPrettyV1, true)
	id := vars["path"]
	m := metrics.New()

	txn, err := s.store.NewTransaction(ctx, storage.WriteParams)
	if err != nil {
		writer.ErrorAuto(w, err)
		return
	}

	modules, err := s.loadModules(ctx, txn)

	if err != nil {
		s.abortAuto(ctx, txn, w, err)
		return
	}

	delete(modules, id)

	c := ast.NewCompiler().SetErrorLimit(s.errLimit)

	m.Timer(metrics.RegoModuleCompile).Start()

	if c.Compile(modules); c.Failed() {
		s.abort(ctx, txn, func() {
			writer.Error(w, http.StatusBadRequest, types.NewErrorV1(types.CodeInvalidOperation, types.MsgCompileModuleError).WithASTErrors(c.Errors))
		})
		return
	}

	m.Timer(metrics.RegoModuleCompile).Stop()

	if err := s.store.DeletePolicy(ctx, txn, id); err != nil {
		s.abortAuto(ctx, txn, w, err)
		return
	}

	if err := s.store.Commit(ctx, txn); err != nil {
		writer.ErrorAuto(w, err)
		return
	}

	s.setCompiler(c)

	response := types.PolicyDeleteResponseV1{}
	if includeMetrics {
		response.Metrics = m.All()
	}

	writer.JSON(w, http.StatusOK, response, pretty)
}

func (s *Server) v1PoliciesGet(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	path := vars["path"]
	pretty := getBoolParam(r.URL, types.ParamPrettyV1, true)

	txn, err := s.store.NewTransaction(ctx)
	if err != nil {
		writer.ErrorAuto(w, err)
		return
	}

	defer s.store.Abort(ctx, txn)

	bs, err := s.store.GetPolicy(ctx, txn, path)
	if err != nil {
		writer.ErrorAuto(w, err)
		return
	}

	c := s.Compiler()

	response := types.PolicyGetResponseV1{
		Result: types.PolicyV1{
			ID:  path,
			Raw: string(bs),
			AST: c.Modules[path],
		},
	}

	writer.JSON(w, http.StatusOK, response, pretty)
}

func (s *Server) v1PoliciesList(w http.ResponseWriter, r *http.Request) {

	ctx := r.Context()
	pretty := getBoolParam(r.URL, types.ParamPrettyV1, true)

	txn, err := s.store.NewTransaction(ctx)
	if err != nil {
		writer.ErrorAuto(w, err)
		return
	}

	defer s.store.Abort(ctx, txn)

	policies := []types.PolicyV1{}
	c := s.Compiler()

	for id, mod := range c.Modules {
		bs, err := s.store.GetPolicy(ctx, txn, id)
		if err != nil {
			writer.ErrorAuto(w, err)
			return
		}
		policy := types.PolicyV1{
			ID:  id,
			Raw: string(bs),
			AST: mod,
		}
		policies = append(policies, policy)
	}

	response := types.PolicyListResponseV1{
		Result: policies,
	}

	writer.JSON(w, http.StatusOK, response, pretty)
}

func (s *Server) v1PoliciesPut(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	path := vars["path"]
	includeMetrics := getBoolParam(r.URL, types.ParamMetricsV1, true)
	pretty := getBoolParam(r.URL, types.ParamPrettyV1, true)
	m := metrics.New()

	m.Timer("server_read_bytes").Start()

	buf, err := ioutil.ReadAll(r.Body)
	if err != nil {
		writer.ErrorString(w, http.StatusBadRequest, types.CodeInvalidParameter, err)
		return
	}

	m.Timer("server_read_bytes").Stop()
	m.Timer(metrics.RegoModuleParse).Start()

	parsedMod, err := ast.ParseModule(path, string(buf))

	m.Timer(metrics.RegoModuleParse).Stop()

	if err != nil {
		switch err := err.(type) {
		case ast.Errors:
			writer.Error(w, http.StatusBadRequest, types.NewErrorV1(types.CodeInvalidParameter, types.MsgCompileModuleError).WithASTErrors(err))
		default:
			writer.ErrorString(w, http.StatusBadRequest, types.CodeInvalidParameter, err)
		}
		return
	}

	if parsedMod == nil {
		writer.Error(w, http.StatusBadRequest, types.NewErrorV1(types.CodeInvalidParameter, "empty module"))
		return
	}

	txn, err := s.store.NewTransaction(ctx, storage.WriteParams)

	if err != nil {
		writer.ErrorAuto(w, err)
		return
	}

	modules, err := s.loadModules(ctx, txn)
	if err != nil {
		s.abortAuto(ctx, txn, w, err)
		return
	}

	modules[path] = parsedMod

	c := ast.NewCompiler().SetErrorLimit(s.errLimit)

	m.Timer(metrics.RegoModuleCompile).Start()

	if c.Compile(modules); c.Failed() {
		s.abort(ctx, txn, func() {
			writer.Error(w, http.StatusBadRequest, types.NewErrorV1(types.CodeInvalidParameter, types.MsgCompileModuleError).WithASTErrors(c.Errors))
		})
		return
	}

	m.Timer(metrics.RegoModuleCompile).Stop()

	if err := s.store.UpsertPolicy(ctx, txn, path, buf); err != nil {
		s.abortAuto(ctx, txn, w, err)
		return
	}

	if err := s.store.Commit(ctx, txn); err != nil {
		writer.ErrorAuto(w, err)
		return
	}

	s.setCompiler(c)

	response := types.PolicyPutResponseV1{}

	if includeMetrics {
		response.Metrics = m.All()
	}

	writer.JSON(w, http.StatusOK, response, pretty)
}

func (s *Server) v1QueryGet(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	values := r.URL.Query()

	qStrs := values[types.ParamQueryV1]
	if len(qStrs) == 0 {
		writer.Error(w, http.StatusBadRequest, types.NewErrorV1(types.CodeInvalidParameter, "missing parameter 'q'"))
		return
	}
	qStr := qStrs[len(qStrs)-1]

	watch := getWatch(r.URL.Query()[types.ParamWatchV1])
	if watch {
		s.watchQuery(qStr, w, r, false)
		return
	}

	pretty := getBoolParam(r.URL, types.ParamPrettyV1, true)
	explainMode := getExplain(r.URL.Query()["explain"], types.ExplainOffV1)
	includeMetrics := getBoolParam(r.URL, types.ParamMetricsV1, true)

	results, err := s.execQuery(ctx, r, qStr, nil, explainMode, includeMetrics, pretty)
	if err != nil {
		switch err := err.(type) {
		case ast.Errors:
			writer.Error(w, http.StatusBadRequest, types.NewErrorV1(types.CodeInvalidParameter, types.MsgCompileQueryError).WithASTErrors(err))
		default:
			writer.ErrorAuto(w, err)
		}
		return
	}

	writer.JSON(w, 200, results, pretty)
}

func (s *Server) watchQuery(query string, w http.ResponseWriter, r *http.Request, data bool) {
	pretty := getBoolParam(r.URL, types.ParamPrettyV1, true)
	explainMode := getExplain(r.URL.Query()["explain"], types.ExplainOffV1)
	includeMetrics := getBoolParam(r.URL, types.ParamMetricsV1, true)

	watch, err := s.watcher.Query(query)
	if err != nil {
		writer.ErrorAuto(w, err)
		return
	}
	defer watch.Stop()

	h, ok := w.(http.Hijacker)
	if !ok {
		writer.ErrorString(w, http.StatusInternalServerError, "server does not support hijacking", errors.New("streaming not supported"))
		return
	}

	conn, bufrw, err := h.Hijack()
	if err != nil {
		writer.ErrorAuto(w, err)
		return
	}
	defer conn.Close()
	defer bufrw.Flush()

	// Manually write the HTTP header since we can't use the original ResponseWriter.
	bufrw.WriteString(fmt.Sprintf("%s %d OK\n", r.Proto, http.StatusOK))
	bufrw.WriteString("Content-Type: application/json\n")
	bufrw.WriteString("Transfer-Encoding: chunked\n\n")

	buf := httputil.NewChunkedWriter(bufrw)
	defer buf.Close()

	encoder := json.NewEncoder(buf)
	if pretty {
		encoder.SetIndent("", "  ")
	}

	abort := r.Context().Done()
	for {
		select {
		case e, ok := <-watch.C:
			if !ok {
				return // The channel was closed by an invalidated query.
			}

			r := types.WatchResponseV1{Result: e.Value}
			if e.Error != nil {
				r.Error = types.NewErrorV1(types.CodeEvaluation, e.Error.Error())
			} else if data && len(e.Value) > 0 && len(e.Value[0].Expressions) > 0 {
				r.Result = e.Value[0].Expressions[0].Value
			}

			if includeMetrics {
				r.Metrics = e.Metrics.All()
			}

			r.Explanation = s.getExplainResponse(explainMode, e.Tracer, pretty)
			if err := encoder.Encode(r); err != nil {
				return
			}

			// Flush the response writer, otherwise the notifications may not
			// be sent until much later.
			bufrw.Flush()
		case <-abort:
			return
		}
	}
}

func constructDiagnosticsInput(r *http.Request) (map[string]interface{}, error) {
	var body interface{}
	if r.Body != nil {
		buf, err := ioutil.ReadAll(r.Body)
		if err != nil {
			return nil, err
		}
		r.Body = ioutil.NopCloser(bytes.NewReader(buf))

		decoder := util.NewJSONDecoder(bytes.NewReader(buf))
		if err := decoder.Decode(&body); err != nil {
			body = nil
		}
	}

	input := map[string]interface{}{
		"method": r.Method,
		"path":   r.URL.Path,
		"body":   body,
	}

	params := map[string]interface{}{}
	for param, value := range r.URL.Query() {
		var ifaces []interface{}
		for _, v := range value {
			ifaces = append(ifaces, v)
		}
		params[param] = ifaces
	}
	input["params"] = params

	header := map[string]interface{}{}
	for key, value := range r.Header {
		var ifaces []interface{}
		for _, v := range value {
			ifaces = append(ifaces, v)
		}
		header[key] = ifaces
	}
	input["header"] = header

	return input, nil
}

func (s *Server) evalDiagnosticPolicy(r *http.Request) settings {
	input, err := constructDiagnosticsInput(r)
	if err != nil {
		return diagsOff
	}

	compiler := s.Compiler()
	rego := rego.New(
		rego.Store(s.store),
		rego.Compiler(compiler),
		rego.Query(`config = data.system.diagnostics.config`),
		rego.Input(input),
	)

	output, err := rego.Eval(r.Context())
	if err != nil {
		return diagsOff
	}

	if len(output) != 1 {
		return diagsOff
	} else if exprs := len(output[0].Expressions); exprs != 1 {
		return diagsOff
	}

	if value, ok := output[0].Expressions[0].Value.(bool); !ok {
		return diagsOff
	} else if !value {
		return diagsOff
	}

	config, ok := output[0].Bindings["config"]
	if !ok {
		return diagsOff
	}

	bindings, ok := config.(map[string]interface{})
	if !ok {
		return diagsOff
	}

	var infoSettings settings
	mode, err := getStringVar(bindings, "mode")
	if err != nil || mode == "off" {
		return diagsOff
	} else if mode == "all" {
		return diagsFull
	} else if mode == "on" {
		infoSettings.on = true
	}

	infoSettings.metrics, err = getBooleanVar(bindings, "metrics")
	if err != nil {
		return diagsOff
	}

	infoSettings.explain, err = getBooleanVar(bindings, "explain")
	if err != nil {
		return diagsOff
	}

	infoSettings.on = infoSettings.on || infoSettings.metrics || infoSettings.explain
	return infoSettings
}

func (s *Server) logDiagnostics(q string, input interface{}, result *interface{}, err error, m metrics.Metrics, t *topdown.BufferTracer, config settings) {
	if !config.on {
		return
	}

	i := newInfo(q, input, result)
	if err != nil {
		i = i.withError(err)
	}

	if config.metrics {
		i = i.withMetrics(m)
	}

	if config.explain {
		i = i.withTrace(*t)
	}

	s.diagnostics.Push(i)
}

func getBooleanVar(bindings rego.Vars, name string) (bool, error) {
	if v, ok := bindings[name]; ok {
		t, ok := v.(bool)
		if !ok {
			return false, fmt.Errorf("non-boolean '%s' field: %v (%T)", name, v, v)
		}
		return t, nil
	}
	return false, nil
}

func getStringVar(bindings rego.Vars, name string) (string, error) {
	if v, ok := bindings[name]; ok {
		t, ok := v.(string)
		if !ok {
			return "", fmt.Errorf("non-string '%s' field: %v (%T)", name, v, v)
		}
		return t, nil
	}
	return "", nil
}

func (s *Server) getExplainResponse(explainMode types.ExplainModeV1, trace []*topdown.Event, pretty bool) (explanation types.TraceV1) {
	switch explainMode {
	case types.ExplainFullV1:
		var err error
		explanation, err = types.NewTraceV1(trace, pretty)
		if err != nil {
			break
		}
	case types.ExplainTruthV1:
		answer, err := explain.Truth(s.Compiler(), trace)
		if err != nil {
			break
		}
		explanation, err = types.NewTraceV1(answer, pretty)
		if err != nil {
			break
		}
	}
	return explanation
}

func (s *Server) abort(ctx context.Context, txn storage.Transaction, finish func()) {
	s.store.Abort(ctx, txn)
	finish()
}

func (s *Server) abortAuto(ctx context.Context, txn storage.Transaction, w http.ResponseWriter, err error) {
	s.abort(ctx, txn, func() { writer.ErrorAuto(w, err) })
}

func (s *Server) loadModules(ctx context.Context, txn storage.Transaction) (map[string]*ast.Module, error) {

	ids, err := s.store.ListPolicies(ctx, txn)
	if err != nil {
		return nil, err
	}

	modules := make(map[string]*ast.Module, len(ids))

	for _, id := range ids {
		bs, err := s.store.GetPolicy(ctx, txn, id)
		if err != nil {
			return nil, err
		}

		parsed, err := ast.ParseModule(id, string(bs))
		if err != nil {
			return nil, err
		}

		modules[id] = parsed
	}

	return modules, nil
}

func (s *Server) setCompiler(compiler *ast.Compiler) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	s.compiler = compiler
}

func (s *Server) makeDir(ctx context.Context, txn storage.Transaction, path storage.Path) error {

	node, err := s.store.Read(ctx, txn, path)
	if err == nil {
		if _, ok := node.(map[string]interface{}); ok {
			return nil
		}
		return types.WriteConflictErr{
			Path: path,
		}
	}

	if !storage.IsNotFound(err) {
		return err
	}

	if err := s.makeDir(ctx, txn, path[:len(path)-1]); err != nil {
		return err
	}

	if err := s.writeConflict(storage.AddOp, path); err != nil {
		return err
	}

	return s.store.Write(ctx, txn, storage.AddOp, path, map[string]interface{}{})
}

func (s *Server) prepareV1PatchSlice(root string, ops []types.PatchV1) (result []patchImpl, err error) {

	root = "/" + strings.Trim(root, "/")

	for _, op := range ops {

		impl := patchImpl{
			value: op.Value,
		}

		// Map patch operation.
		switch op.Op {
		case "add":
			impl.op = storage.AddOp
		case "remove":
			impl.op = storage.RemoveOp
		case "replace":
			impl.op = storage.ReplaceOp
		default:
			return nil, types.BadPatchOperationErr(op.Op)
		}

		// Construct patch path.
		path := strings.Trim(op.Path, "/")
		if len(path) > 0 {
			if root == "/" {
				path = root + path
			} else {
				path = root + "/" + path
			}
		} else {
			path = root
		}

		var ok bool
		impl.path, ok = storage.ParsePath(path)
		if !ok {
			return nil, types.BadPatchPathErr(op.Path)
		}

		if err := s.writeConflict(impl.op, impl.path); err != nil {
			return nil, err
		}

		result = append(result, impl)
	}

	return result, nil
}

// TODO(tsandall): this ought to be enforced by the storage layer.
func (s *Server) writeConflict(op storage.PatchOp, path storage.Path) error {

	if op == storage.AddOp && len(path) > 0 && path[len(path)-1] == "-" {
		path = path[:len(path)-1]
	}

	ref := path.Ref(ast.DefaultRootDocument)

	if rs := s.Compiler().GetRulesForVirtualDocument(ref); rs != nil {
		return types.WriteConflictErr{
			Path: path,
		}
	}

	return nil
}

func handleCompileError(w http.ResponseWriter, err error) {
	switch err := err.(type) {
	case ast.Errors:
		writer.Error(w, http.StatusBadRequest, types.NewErrorV1(types.CodeInvalidParameter, types.MsgCompileQueryError).WithASTErrors(err))
	default:
		writer.ErrorString(w, http.StatusBadRequest, types.CodeInvalidParameter, err)
	}
}

func stringPathToDataRef(s string) (r ast.Ref) {
	result := ast.Ref{ast.DefaultRootDocument}
	result = append(result, stringPathToRef(s)...)
	return result
}

func stringPathToRef(s string) (r ast.Ref) {
	if len(s) == 0 {
		return r
	}
	p := strings.Split(s, "/")
	for _, x := range p {
		if x == "" {
			continue
		}
		i, err := strconv.Atoi(x)
		if err != nil {
			r = append(r, ast.StringTerm(x))
		} else {
			r = append(r, ast.IntNumberTerm(i))
		}
	}
	return r
}

func getBoolParam(url *url.URL, name string, ifEmpty bool) bool {

	p, ok := url.Query()[name]
	if !ok {
		return false
	}

	// Query params w/o values are represented as slice (of len 1) with an
	// empty string.
	if len(p) == 1 && p[0] == "" {
		return ifEmpty
	}

	for _, x := range p {
		if strings.ToLower(x) == "true" {
			return true
		}
	}

	return false
}

func getWatch(p []string) (watch bool) {
	return len(p) > 0
}

func getExplain(p []string, zero types.ExplainModeV1) types.ExplainModeV1 {
	for _, x := range p {
		switch x {
		case string(types.ExplainFullV1):
			return types.ExplainFullV1
		case string(types.ExplainTruthV1):
			return types.ExplainTruthV1
		}
	}
	return zero
}

func readInputV0(r io.ReadCloser) (ast.Value, error) {

	bs, err := ioutil.ReadAll(r)

	if err != nil {
		return nil, err
	}

	s := strings.TrimSpace(string(bs))
	if len(s) == 0 {
		return nil, nil
	}

	term, err := ast.ParseTerm(s)
	if err != nil {
		return nil, err
	}

	return term.Value, nil
}

func readInputGetV1(str string) (ast.Value, error) {
	var input interface{}

	if err := util.UnmarshalJSON([]byte(str), &input); err != nil {
		return nil, errors.Wrapf(err, "parameter contains malformed input document")
	}

	return ast.InterfaceToValue(input)
}

func readInputPostV1(r io.ReadCloser) (ast.Value, error) {

	bs, err := ioutil.ReadAll(r)

	if err != nil {
		return nil, err
	}

	if len(bs) > 0 {

		var request types.DataRequestV1

		if err := util.UnmarshalJSON(bs, &request); err != nil {
			return nil, errors.Wrapf(err, "body contains malformed input document")
		}

		if request.Input == nil {
			return nil, nil
		}

		return ast.InterfaceToValue(*request.Input)
	}

	return nil, nil
}

func renderBanner(w http.ResponseWriter) {
	fmt.Fprintln(w, `<pre>
 ________      ________    ________
|\   __  \    |\   __  \  |\   __  \
\ \  \|\  \   \ \  \|\  \ \ \  \|\  \
 \ \  \\\  \   \ \   ____\ \ \   __  \
  \ \  \\\  \   \ \  \___|  \ \  \ \  \
   \ \_______\   \ \__\      \ \__\ \__\
    \|_______|    \|__|       \|__|\|__|
	</pre>`)
	fmt.Fprintln(w, "Open Policy Agent - An open source project to policy-enable your service.<br>")
	fmt.Fprintln(w, "<br>")
}

func renderFooter(w http.ResponseWriter) {
	fmt.Fprintln(w, "</body>")
	fmt.Fprintln(w, "</html>")
}

func renderHeader(w http.ResponseWriter) {
	fmt.Fprintln(w, "<html>")
	fmt.Fprintln(w, "<body>")
}

func renderQueryForm(w http.ResponseWriter, qStrs []string, inputStrs []string, explain types.ExplainModeV1) {

	query := ""

	if len(qStrs) > 0 {
		query = qStrs[len(qStrs)-1]
	}

	input := ""
	if len(inputStrs) > 0 {
		input = inputStrs[len(inputStrs)-1]
	}

	explainRadioCheck := []string{"", "", ""}
	switch explain {
	case types.ExplainOffV1:
		explainRadioCheck[0] = "checked"
	case types.ExplainFullV1:
		explainRadioCheck[1] = "checked"
	case types.ExplainTruthV1:
		explainRadioCheck[2] = "checked"
	}

	fmt.Fprintf(w, `
	<form>
  	Query:<br>
	<textarea rows="10" cols="50" name="q">%s</textarea><br>
	<br>Input Data (JSON):<br>
	<textarea rows="10" cols="50" name="input">%s</textarea><br>
	<br><input type="submit" value="Submit"> Explain:
	<input type="radio" name="explain" value="off" %v>Off
	<input type="radio" name="explain" value="full" %v>Full
	<input type="radio" name="explain" value="truth" %v>Truth
	</form>`, query, input, explainRadioCheck[0], explainRadioCheck[1], explainRadioCheck[2])
}

func renderQueryResult(w io.Writer, results interface{}, err error, t0 time.Time) {

	buf, err2 := json.MarshalIndent(results, "", "  ")
	d := time.Since(t0)

	if err != nil {
		fmt.Fprintf(w, "Query error (took %v): <pre>%v</pre>", d, err)
	} else if err2 != nil {
		fmt.Fprintf(w, "JSON marshal error: <pre>%v</pre>", err2)
	} else {
		fmt.Fprintf(w, "Query results (took %v):<br>", d)
		fmt.Fprintf(w, "<pre>%s</pre>", string(buf))
	}
}

func renderVersion(w http.ResponseWriter) {
	fmt.Fprintln(w, "Version: "+version.Version+"<br>")
	fmt.Fprintln(w, "Build Commit: "+version.Vcs+"<br>")
	fmt.Fprintln(w, "Build Timestamp: "+version.Timestamp+"<br>")
	fmt.Fprintln(w, "Build Hostname: "+version.Hostname+"<br>")
	fmt.Fprintln(w, "<br>")
}

type patchImpl struct {
	path  storage.Path
	op    storage.PatchOp
	value interface{}
}
