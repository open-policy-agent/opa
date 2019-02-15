// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package server

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/internal/manifest"
	"github.com/open-policy-agent/opa/metrics"
	"github.com/open-policy-agent/opa/plugins"
	"github.com/open-policy-agent/opa/rego"
	"github.com/open-policy-agent/opa/server/authorizer"
	"github.com/open-policy-agent/opa/server/identifier"
	"github.com/open-policy-agent/opa/server/types"
	"github.com/open-policy-agent/opa/server/writer"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/topdown"
	"github.com/open-policy-agent/opa/util"
	"github.com/open-policy-agent/opa/version"
	"github.com/open-policy-agent/opa/watch"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// AuthenticationScheme enumerates the supported authentication schemes. The
// authentication scheme determines how client identities are established.
type AuthenticationScheme int

// Set of supported authentication schemes.
const (
	AuthenticationOff AuthenticationScheme = iota
	AuthenticationToken
	AuthenticationTLS
)

// AuthorizationScheme enumerates the supported authorization schemes. The authorization
// scheme determines how access to OPA is controlled.
type AuthorizationScheme int

// Set of supported authorization schemes.
const (
	AuthorizationOff AuthorizationScheme = iota
	AuthorizationBasic
)

// Set of handlers for use in the "handler" dimension of the duration metric.
const (
	PromHandlerV0Data     = "v0/data"
	PromHandlerV1Data     = "v1/data"
	PromHandlerV1Query    = "v1/query"
	PromHandlerV1Policies = "v1/policies"
	PromHandlerV1Compile  = "v1/compile"
	PromHandlerIndex      = "index"
	PromHandlerCatch      = "catchall"
	PromHandlerHealth     = "health"
)

// map of unsafe buitins
var unsafeBuiltinsMap = map[string]bool{ast.HTTPSend.Name: true}

// Server represents an instance of OPA running in server mode.
type Server struct {
	Handler http.Handler

	router            *mux.Router
	addrs             []string
	insecureAddr      string
	authentication    AuthenticationScheme
	authorization     AuthorizationScheme
	cert              *tls.Certificate
	certPool          *x509.CertPool
	mtx               sync.RWMutex
	partials          map[string]rego.PartialResult
	store             storage.Store
	manager           *plugins.Manager
	watcher           *watch.Watcher
	decisionIDFactory func() string
	diagnostics       Buffer
	revision          string
	logger            func(context.Context, *Info)
	errLimit          int
	runtime           *ast.Term
}

// Loop will contain all the calls from the server that we'll be listening on.
type Loop func() error

// New returns a new Server.
func New() *Server {
	s := Server{}
	return &s
}

// Init initializes the server. This function MUST be called before Loop.
func (s *Server) Init(ctx context.Context) (*Server, error) {

	s.initRouter()

	// Add authorization handler. This must come BEFORE authentication handler
	// so that the latter can run first.
	switch s.authorization {
	case AuthorizationBasic:
		s.Handler = authorizer.NewBasic(
			s.Handler,
			s.getCompiler,
			s.store,
			authorizer.Runtime(s.runtime),
			authorizer.Decision(s.manager.Config.DefaultAuthorizationDecisionRef))
	}

	switch s.authentication {
	case AuthenticationToken:
		s.Handler = identifier.NewTokenBased(s.Handler)
	case AuthenticationTLS:
		s.Handler = identifier.NewTLSBased(s.Handler)
	}

	txn, err := s.store.NewTransaction(ctx, storage.WriteParams)
	if err != nil {
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

	s.manager.RegisterCompilerTrigger(s.migrateWatcher)

	s.watcher, err = watch.New(ctx, s.store, s.getCompiler(), txn)
	if err != nil {
		return nil, err
	}

	s.partials = map[string]rego.PartialResult{}

	return s, s.store.Commit(ctx, txn)
}

// WithAddresses sets the listening addresses that the server will bind to.
func (s *Server) WithAddresses(addrs []string) *Server {
	s.addrs = addrs
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

// WithCertPool sets the server-side cert pool that the server will use.
func (s *Server) WithCertPool(pool *x509.CertPool) *Server {
	s.certPool = pool
	return s
}

// WithStore sets the storage used by the server.
func (s *Server) WithStore(store storage.Store) *Server {
	s.store = store
	return s
}

// WithManager sets the plugins manager used by the server.
func (s *Server) WithManager(manager *plugins.Manager) *Server {
	s.manager = manager
	return s
}

// WithCompilerErrorLimit sets the limit on the number of compiler errors the server will
// allow.
func (s *Server) WithCompilerErrorLimit(limit int) *Server {
	s.errLimit = limit
	return s
}

// WithDiagnosticsBuffer sets the diagnostics buffer used by the server. DEPRECATED.
func (s *Server) WithDiagnosticsBuffer(buf Buffer) *Server {
	s.diagnostics = buf
	return s
}

// WithDecisionLogger sets the decision logger used by the server.
func (s *Server) WithDecisionLogger(logger func(context.Context, *Info)) *Server {
	s.logger = logger
	return s
}

// WithDecisionIDFactory sets a function on the server to generate decision IDs.
func (s *Server) WithDecisionIDFactory(f func() string) *Server {
	s.decisionIDFactory = f
	return s
}

// WithRuntime sets the runtime data to provide to the evaluation engine.
func (s *Server) WithRuntime(term *ast.Term) *Server {
	s.runtime = term
	return s
}

// WithRouter sets the mux.Router to attach OPA's HTTP API routes onto. If a
// router is not supplied, the server will create it's own.
func (s *Server) WithRouter(router *mux.Router) *Server {
	s.router = router
	return s
}

// Listeners returns functions that listen and serve connections.
func (s *Server) Listeners() ([]Loop, error) {
	loops := []Loop{}
	for _, addr := range s.addrs {
		parsedURL, err := parseURL(addr, s.cert != nil)
		if err != nil {
			return nil, err
		}
		var loop Loop
		switch parsedURL.Scheme {
		case "unix":
			loop, err = s.getListenerForUNIXSocket(parsedURL)
		case "http":
			loop, err = s.getListenerForHTTPServer(parsedURL)
		case "https":
			loop, err = s.getListenerForHTTPSServer(parsedURL)
		default:
			err = fmt.Errorf("invalid url scheme %q", parsedURL.Scheme)
		}
		if err != nil {
			return nil, err
		}
		loops = append(loops, loop)
	}

	if s.insecureAddr != "" {
		parsedURL, err := parseURL(s.insecureAddr, false)
		if err != nil {
			return nil, err
		}
		loop, err := s.getListenerForHTTPServer(parsedURL)
		if err != nil {
			return nil, err
		}
		loops = append(loops, loop)
	}

	return loops, nil
}

func (s *Server) getListenerForHTTPServer(u *url.URL) (Loop, error) {
	httpServer := http.Server{
		Addr:    u.Host,
		Handler: s.Handler,
	}

	return httpServer.ListenAndServe, nil
}

func (s *Server) getListenerForHTTPSServer(u *url.URL) (Loop, error) {

	if s.cert == nil {
		return nil, fmt.Errorf("TLS certificate required but not supplied")
	}

	httpsServer := http.Server{
		Addr:    u.Host,
		Handler: s.Handler,
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{*s.cert},
			ClientCAs:    s.certPool,
		},
	}
	if s.authentication == AuthenticationTLS {
		httpsServer.TLSConfig.ClientAuth = tls.RequireAndVerifyClientCert
	}

	httpsLoop := func() error { return httpsServer.ListenAndServeTLS("", "") }

	return httpsLoop, nil
}

func (s *Server) getListenerForUNIXSocket(u *url.URL) (Loop, error) {
	socketPath := u.Host + u.Path

	// Remove domain socket file in case it already exists.
	os.Remove(socketPath)

	domainSocketServer := http.Server{Handler: s.Handler}
	unixListener, err := net.Listen("unix", socketPath)
	if err != nil {
		return nil, err
	}

	domainSocketLoop := func() error { return domainSocketServer.Serve(unixListener) }
	return domainSocketLoop, nil
}

func (s *Server) initRouter() {

	promRegistry := prometheus.NewRegistry()
	duration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "http_request_duration_seconds",
			Help: "A histogram of duration for requests.",
		},
		[]string{"code", "handler", "method"},
	)
	v0DataDur := duration.MustCurryWith(prometheus.Labels{"handler": PromHandlerV0Data})
	v1DataDur := duration.MustCurryWith(prometheus.Labels{"handler": PromHandlerV1Data})
	v1PoliciesDur := duration.MustCurryWith(prometheus.Labels{"handler": PromHandlerV1Policies})
	v1QueryDur := duration.MustCurryWith(prometheus.Labels{"handler": PromHandlerV1Query})
	v1CompileDur := duration.MustCurryWith(prometheus.Labels{"handler": PromHandlerV1Compile})
	indexDur := duration.MustCurryWith(prometheus.Labels{"handler": PromHandlerIndex})
	catchAllDur := duration.MustCurryWith(prometheus.Labels{"handler": PromHandlerCatch})
	GetHealthDur := duration.MustCurryWith(prometheus.Labels{"handler": PromHandlerHealth})
	promRegistry.MustRegister(duration)

	router := s.router

	if router == nil {
		router = mux.NewRouter()
	}

	router.UseEncodedPath()
	router.StrictSlash(true)
	router.Handle("/metrics", promhttp.HandlerFor(promRegistry, promhttp.HandlerOpts{})).Methods(http.MethodGet)
	router.Handle("/health", promhttp.InstrumentHandlerDuration(GetHealthDur, http.HandlerFunc(s.unversionedGetHealth))).Methods(http.MethodGet)
	s.registerHandler(router, 0, "/data/{path:.+}", http.MethodPost, promhttp.InstrumentHandlerDuration(v0DataDur, http.HandlerFunc(s.v0DataPost)))
	s.registerHandler(router, 0, "/data", http.MethodPost, promhttp.InstrumentHandlerDuration(v0DataDur, http.HandlerFunc(s.v0DataPost)))
	s.registerHandler(router, 1, "/data/system/version", http.MethodGet, promhttp.InstrumentHandlerDuration(v1DataDur, http.HandlerFunc(s.v1VersionGet)))
	s.registerHandler(router, 1, "/data/system/diagnostics", http.MethodGet, promhttp.InstrumentHandlerDuration(v1DataDur, http.HandlerFunc(s.v1DiagnosticsGet)))
	s.registerHandler(router, 1, "/data/{path:.+}", http.MethodDelete, promhttp.InstrumentHandlerDuration(v1DataDur, http.HandlerFunc(s.v1DataDelete)))
	s.registerHandler(router, 1, "/data/{path:.+}", http.MethodPut, promhttp.InstrumentHandlerDuration(v1DataDur, http.HandlerFunc(s.v1DataPut)))
	s.registerHandler(router, 1, "/data", http.MethodPut, promhttp.InstrumentHandlerDuration(v1DataDur, http.HandlerFunc(s.v1DataPut)))
	s.registerHandler(router, 1, "/data/{path:.+}", http.MethodGet, promhttp.InstrumentHandlerDuration(v1DataDur, http.HandlerFunc(s.v1DataGet)))
	s.registerHandler(router, 1, "/data", http.MethodGet, promhttp.InstrumentHandlerDuration(v1DataDur, http.HandlerFunc(s.v1DataGet)))
	s.registerHandler(router, 1, "/data/{path:.+}", http.MethodPatch, promhttp.InstrumentHandlerDuration(v1DataDur, http.HandlerFunc(s.v1DataPatch)))
	s.registerHandler(router, 1, "/data", http.MethodPatch, promhttp.InstrumentHandlerDuration(v1DataDur, http.HandlerFunc(s.v1DataPatch)))
	s.registerHandler(router, 1, "/data/{path:.+}", http.MethodPost, promhttp.InstrumentHandlerDuration(v1DataDur, http.HandlerFunc(s.v1DataPost)))
	s.registerHandler(router, 1, "/data", http.MethodPost, promhttp.InstrumentHandlerDuration(v1DataDur, http.HandlerFunc(s.v1DataPost)))
	s.registerHandler(router, 1, "/policies", http.MethodGet, promhttp.InstrumentHandlerDuration(v1PoliciesDur, http.HandlerFunc(s.v1PoliciesList)))
	s.registerHandler(router, 1, "/policies/{path:.+}", http.MethodDelete, promhttp.InstrumentHandlerDuration(v1PoliciesDur, http.HandlerFunc(s.v1PoliciesDelete)))
	s.registerHandler(router, 1, "/policies/{path:.+}", http.MethodGet, promhttp.InstrumentHandlerDuration(v1PoliciesDur, http.HandlerFunc(s.v1PoliciesGet)))
	s.registerHandler(router, 1, "/policies/{path:.+}", http.MethodPut, promhttp.InstrumentHandlerDuration(v1PoliciesDur, http.HandlerFunc(s.v1PoliciesPut)))
	s.registerHandler(router, 1, "/query", http.MethodGet, promhttp.InstrumentHandlerDuration(v1QueryDur, http.HandlerFunc(s.v1QueryGet)))
	s.registerHandler(router, 1, "/query", http.MethodPost, promhttp.InstrumentHandlerDuration(v1QueryDur, http.HandlerFunc(s.v1QueryPost)))
	s.registerHandler(router, 1, "/compile", http.MethodPost, promhttp.InstrumentHandlerDuration(v1CompileDur, http.HandlerFunc(s.v1CompilePost)))
	router.HandleFunc("/", promhttp.InstrumentHandlerDuration(indexDur, http.HandlerFunc(s.unversionedPost))).Methods(http.MethodPost)
	router.HandleFunc("/", promhttp.InstrumentHandlerDuration(indexDur, http.HandlerFunc(s.indexGet))).Methods(http.MethodGet)
	// These are catch all handlers that respond 405 for resources that exist but the method is not allowed
	router.HandleFunc("/v0/data/{path:.*}", promhttp.InstrumentHandlerDuration(catchAllDur, http.HandlerFunc(writer.HTTPStatus(405)))).Methods(http.MethodGet, http.MethodHead,
		http.MethodConnect, http.MethodDelete, http.MethodOptions, http.MethodPatch, http.MethodPut, http.MethodTrace)
	router.HandleFunc("/v0/data", promhttp.InstrumentHandlerDuration(catchAllDur, http.HandlerFunc(writer.HTTPStatus(405)))).Methods(http.MethodGet, http.MethodHead,
		http.MethodConnect, http.MethodDelete, http.MethodOptions, http.MethodPatch, http.MethodPut,
		http.MethodTrace)
	// v1 Data catch all
	router.HandleFunc("/v1/data/{path:.*}", promhttp.InstrumentHandlerDuration(catchAllDur, http.HandlerFunc(writer.HTTPStatus(405)))).Methods(http.MethodHead,
		http.MethodConnect, http.MethodOptions, http.MethodTrace)
	router.HandleFunc("/v1/data", promhttp.InstrumentHandlerDuration(catchAllDur, http.HandlerFunc(writer.HTTPStatus(405)))).Methods(http.MethodHead,
		http.MethodConnect, http.MethodDelete, http.MethodOptions, http.MethodTrace)
	// Policies catch all
	router.HandleFunc("/v1/policies", promhttp.InstrumentHandlerDuration(catchAllDur, http.HandlerFunc(writer.HTTPStatus(405)))).Methods(http.MethodHead,
		http.MethodConnect, http.MethodDelete, http.MethodOptions, http.MethodTrace, http.MethodPost, http.MethodPut,
		http.MethodPatch)
	// Policies (/policies/{path.+} catch all
	router.HandleFunc("/v1/policies/{path:.*}", promhttp.InstrumentHandlerDuration(catchAllDur, http.HandlerFunc(writer.HTTPStatus(405)))).Methods(http.MethodHead,
		http.MethodConnect, http.MethodOptions, http.MethodTrace, http.MethodPost)
	// Query catch all
	router.HandleFunc("/v1/query/{path:.*}", promhttp.InstrumentHandlerDuration(catchAllDur, http.HandlerFunc(writer.HTTPStatus(405)))).Methods(http.MethodHead,
		http.MethodConnect, http.MethodDelete, http.MethodOptions, http.MethodTrace, http.MethodPost, http.MethodPut, http.MethodPatch)
	router.HandleFunc("/v1/query", promhttp.InstrumentHandlerDuration(catchAllDur, http.HandlerFunc(writer.HTTPStatus(405)))).Methods(http.MethodHead,
		http.MethodConnect, http.MethodDelete, http.MethodOptions, http.MethodTrace, http.MethodPut, http.MethodPatch)
	s.Handler = router
}

func (s *Server) execQuery(ctx context.Context, r *http.Request, decisionID string, parsedQuery ast.Body, input ast.Value, m metrics.Metrics, explainMode types.ExplainModeV1, includeMetrics, includeInstrumentation, pretty bool) (results types.QueryResponseV1, err error) {

	diagLogger := s.evalDiagnosticPolicy(r)

	var buf *topdown.BufferTracer
	if explainMode != types.ExplainOffV1 || diagLogger.Explain() {
		buf = topdown.NewBufferTracer()
	}

	var instrument bool

	if includeInstrumentation || diagLogger.Instrument() {
		instrument = true
	}

	var rawInput *interface{}
	if input != nil {
		x, err := ast.JSON(input)
		if err != nil {
			return results, err
		}
		rawInput = &x
	}

	compiler := s.getCompiler()

	rego := rego.New(
		rego.Store(s.store),
		rego.Compiler(compiler),
		rego.ParsedQuery(parsedQuery),
		rego.ParsedInput(input),
		rego.Metrics(m),
		rego.Instrument(instrument),
		rego.Tracer(buf),
		rego.Runtime(s.runtime),
	)

	output, err := rego.Eval(ctx)
	if err != nil {
		diagLogger.Log(ctx, decisionID, r.RemoteAddr, "", parsedQuery.String(), rawInput, nil, err, m, buf)
		return results, err
	}

	for _, result := range output {
		results.Result = append(results.Result, result.Bindings.WithoutWildcards())
	}

	if includeMetrics || includeInstrumentation {
		results.Metrics = m.All()
	}

	if explainMode != types.ExplainOffV1 {
		results.Explanation = s.getExplainResponse(explainMode, *buf, pretty)
	}

	var x interface{} = results.Result
	diagLogger.Log(ctx, decisionID, r.RemoteAddr, "", parsedQuery.String(), rawInput, &x, nil, m, buf)

	return results, nil
}

func (s *Server) indexGet(w http.ResponseWriter, r *http.Request) {

	decisionID := s.generateDecisionID()

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

	_, parsedQuery, _ := validateQuery(qStr)

	results, err := s.execQuery(ctx, r, decisionID, parsedQuery, input, nil, explainMode, false, false, true)
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

	if revision, err := manifest.ReadBundleRevision(ctx, s.store, txn); err != nil {
		if !storage.IsNotFound(err) {
			panic(err)
		}
	} else {
		s.revision = revision
	}

	s.partials = map[string]rego.PartialResult{}
}

func (s *Server) migrateWatcher(txn storage.Transaction) {
	var err error
	s.watcher, err = s.watcher.Migrate(s.manager.GetCompiler(), txn)
	if err != nil {
		// The only way migration can fail is if the old watcher is closed or if
		// the new one cannot register a trigger with the store. Since we're
		// using an inmem store with a write transaction, neither of these should
		// be possible.
		panic(err)
	}
}

func (s *Server) unversionedPost(w http.ResponseWriter, r *http.Request) {
	s.v0QueryPath(w, r, s.manager.Config.DefaultDecisionRef())
}

func (s *Server) v0DataPost(w http.ResponseWriter, r *http.Request) {
	path := stringPathToDataRef(mux.Vars(r)["path"])
	s.v0QueryPath(w, r, path)
}

func (s *Server) v0QueryPath(w http.ResponseWriter, r *http.Request, path ast.Ref) {
	m := metrics.New()
	m.Timer(metrics.ServerHandler).Start()

	decisionID := s.generateDecisionID()

	ctx := r.Context()
	diagLogger := s.evalDiagnosticPolicy(r)
	input, err := readInputV0(r)
	if err != nil {
		writer.ErrorString(w, http.StatusBadRequest, types.CodeInvalidParameter, errors.Wrapf(err, "unexpected parse error for input"))
		return
	}

	var goInput *interface{}
	if input != nil {
		x, err := ast.JSON(input)
		if err != nil {
			writer.ErrorString(w, http.StatusInternalServerError, types.CodeInvalidParameter, errors.Wrapf(err, "could not marshal input"))
			return
		}
		goInput = &x
	}

	// Prepare for query.
	txn, err := s.store.NewTransaction(ctx)
	if err != nil {
		writer.ErrorAuto(w, err)
		return
	}

	defer s.store.Abort(ctx, txn)

	var buf *topdown.BufferTracer

	if diagLogger.Explain() {
		buf = topdown.NewBufferTracer()
	}

	rego := rego.New(
		rego.Compiler(s.getCompiler()),
		rego.Store(s.store),
		rego.Transaction(txn),
		rego.ParsedInput(input),
		rego.Query(path.String()),
		rego.Metrics(m),
		rego.Instrument(diagLogger.Instrument()),
		rego.Tracer(buf),
		rego.Runtime(s.runtime),
	)

	rs, err := rego.Eval(ctx)

	m.Timer(metrics.ServerHandler).Stop()

	// Handle results.
	if err != nil {
		diagLogger.Log(ctx, decisionID, r.RemoteAddr, path.String(), "", goInput, nil, err, m, buf)
		writer.ErrorAuto(w, err)
		return
	}

	if len(rs) == 0 {
		writer.Error(w, 404, types.NewErrorV1(types.CodeUndefinedDocument, fmt.Sprintf("%v: %v", types.MsgUndefinedError, path)))
		return
	}

	diagLogger.Log(ctx, decisionID, r.RemoteAddr, path.String(), "", goInput, &rs[0].Expressions[0].Value, nil, m, buf)
	writer.JSON(w, 200, rs[0].Expressions[0].Value, false)
}

func (s *Server) v1DiagnosticsGet(w http.ResponseWriter, r *http.Request) {
	pretty := getBoolParam(r.URL, types.ParamPrettyV1, true)
	if s.diagnostics == nil {
		writer.ErrorAuto(w, fmt.Errorf(types.MsgDiagnosticsDisabled))
		return
	}
	explainMode := getExplain(r.URL.Query()[types.ParamExplainV1], types.ExplainFullV1)
	resp := types.DiagnosticsResponseV1{
		Result: []types.DiagnosticsResponseElementV1{},
	}
	s.diagnostics.Iter(func(i *Info) {
		item := types.DiagnosticsResponseElementV1{
			Revision:   i.Revision,
			DecisionID: i.DecisionID,
			RemoteAddr: i.RemoteAddr,
			Timestamp:  i.Timestamp.UTC().Format(time.RFC3339Nano),
			Query:      i.Query,
			Path:       i.Path,
			Input:      i.Input,
			Metrics:    i.Metrics.All(),
		}
		if i.Trace != nil {
			item.Explanation = s.getExplainResponse(explainMode, i.Trace, pretty)
		}
		if i.Error != nil {
			item.Error = types.NewErrorV1(types.CodeInternal, i.Error.Error())
		} else {
			item.Result = i.Results
		}
		resp.Result = append(resp.Result, item)
	})
	writer.JSON(w, 200, resp, pretty)
}

func (s *Server) unversionedGetHealth(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	// Create very simple query that binds a single variable.
	eval := rego.New(rego.Compiler(s.getCompiler()),
		rego.Store(s.store), rego.Query("x = 1"))
	// Run evaluation.
	rs, err := eval.Eval(ctx)
	if err != nil {
		writer.ErrorAuto(w, err)
		return
	}
	type emptyObject struct{}
	v, ok := rs[0].Bindings["x"]
	if ok {
		jsonNumber, ok := v.(json.Number)
		if ok && jsonNumber.String() == "1" {
			writer.JSON(w, http.StatusOK, emptyObject{}, false)
			return
		}
	}
	writer.JSON(w, http.StatusInternalServerError, emptyObject{}, false)
}

func (s *Server) v1VersionGet(w http.ResponseWriter, r *http.Request) {

	code := 200

	opaVersion := `
{
  "result": {
    "Version": "{{.Version}}",
    "BuildCommit": "{{.BuildCommit}}",
    "BuildTimestamp": "{{.BuildTimestamp}}",
    "BuildHostname": "{{.BuildHostname}}"
  }
}
	`
	tmpl, err := template.New("").Parse(opaVersion)
	if err != nil {
		panic(err)
	}

	var buf bytes.Buffer

	err = tmpl.Execute(&buf, struct {
		Version        string
		BuildCommit    string
		BuildTimestamp string
		BuildHostname  string
	}{
		Version:        version.Version,
		BuildCommit:    version.Vcs,
		BuildTimestamp: version.Timestamp,
		BuildHostname:  version.Hostname,
	})

	jsonBytes := buf.Bytes()

	headers := w.Header()
	headers.Add("Content-Type", "application/json")
	writer.Bytes(w, code, jsonBytes)
}

func (s *Server) v1CompilePost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	pretty := getBoolParam(r.URL, types.ParamPrettyV1, true)
	explainMode := getExplain(r.URL.Query()[types.ParamExplainV1], types.ExplainOffV1)
	includeMetrics := getBoolParam(r.URL, types.ParamMetricsV1, true)
	includeInstrumentation := getBoolParam(r.URL, types.ParamInstrumentV1, true)

	m := metrics.New()

	m.Timer(metrics.RegoQueryParse).Start()

	request, reqErr := readInputCompilePostV1(r.Body)
	if reqErr != nil {
		writer.Error(w, http.StatusBadRequest, reqErr)
		return
	}

	unsafeBuiltins, err := validateParsedQuery(request.Query)
	if err != nil {
		writer.ErrorAuto(w, err)
		return
	} else if len(unsafeBuiltins) > 0 {
		writer.Error(w, http.StatusBadRequest, types.NewErrorV1(types.CodeInvalidParameter, "unsafe built-in function calls in query: %v", strings.Join(unsafeBuiltins, ",")))
		return
	}

	m.Timer(metrics.RegoQueryParse).Stop()

	txn, err := s.store.NewTransaction(ctx)
	if err != nil {
		writer.ErrorAuto(w, err)
		return
	}

	defer s.store.Abort(ctx, txn)

	var buf *topdown.BufferTracer
	if explainMode != types.ExplainOffV1 {
		buf = topdown.NewBufferTracer()
	}

	var instrument bool
	if includeInstrumentation {
		instrument = true
	}

	eval := rego.New(
		rego.Compiler(s.getCompiler()),
		rego.Store(s.store),
		rego.Transaction(txn),
		rego.ParsedQuery(request.Query),
		rego.ParsedInput(request.Input),
		rego.ParsedUnknowns(request.Unknowns),
		rego.Tracer(buf),
		rego.Instrument(instrument),
		rego.Metrics(m),
		rego.Runtime(s.runtime),
	)

	pq, err := eval.Partial(ctx)
	if err != nil {
		writer.ErrorAuto(w, err)
		return
	}

	result := types.CompileResponseV1{}

	if includeMetrics || includeInstrumentation {
		result.Metrics = m.All()
	}

	if explainMode != types.ExplainOffV1 {
		result.Explanation = s.getExplainResponse(explainMode, *buf, pretty)
	}

	var i interface{} = types.PartialEvaluationResultV1{
		Queries: pq.Queries,
		Support: pq.Support,
	}

	result.Result = &i

	writer.JSON(w, 200, result, pretty)
}

func (s *Server) v1DataGet(w http.ResponseWriter, r *http.Request) {
	m := metrics.New()

	m.Timer(metrics.ServerHandler).Start()

	decisionID := s.generateDecisionID()

	ctx := r.Context()
	vars := mux.Vars(r)
	path := stringPathToDataRef(vars["path"])
	diagLogger := s.evalDiagnosticPolicy(r)

	watch := getWatch(r.URL.Query()[types.ParamWatchV1])
	if watch {
		s.watchQuery(path.String(), w, r, true)
		return
	}

	pretty := getBoolParam(r.URL, types.ParamPrettyV1, true)
	explainMode := getExplain(r.URL.Query()["explain"], types.ExplainOffV1)
	includeMetrics := getBoolParam(r.URL, types.ParamMetricsV1, true)
	includeInstrumentation := getBoolParam(r.URL, types.ParamInstrumentV1, true)

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

	var goInput *interface{}
	if input != nil {
		x, err := ast.JSON(input)
		if err != nil {
			writer.ErrorString(w, http.StatusInternalServerError, types.CodeInvalidParameter, errors.Wrapf(err, "could not marshal input"))
			return
		}
		goInput = &x
	}

	// Prepare for query.
	txn, err := s.store.NewTransaction(ctx)
	if err != nil {
		writer.ErrorAuto(w, err)
		return
	}

	defer s.store.Abort(ctx, txn)

	var buf *topdown.BufferTracer

	if explainMode != types.ExplainOffV1 || diagLogger.Explain() {
		buf = topdown.NewBufferTracer()
	}

	var instrument bool

	if includeInstrumentation || diagLogger.Instrument() {
		instrument = true
	}

	rego := rego.New(
		rego.Compiler(s.getCompiler()),
		rego.Store(s.store),
		rego.Transaction(txn),
		rego.ParsedInput(input),
		rego.Query(path.String()),
		rego.Metrics(m),
		rego.Tracer(buf),
		rego.Instrument(instrument),
		rego.Runtime(s.runtime),
	)

	rs, err := rego.Eval(ctx)

	// Handle results.
	if err != nil {
		diagLogger.Log(ctx, decisionID, r.RemoteAddr, path.String(), "", goInput, nil, err, m, buf)
		writer.ErrorAuto(w, err)
		return
	}

	result := types.DataResponseV1{
		DecisionID: decisionID,
	}

	m.Timer(metrics.ServerHandler).Stop()

	if includeMetrics || includeInstrumentation {
		result.Metrics = m.All()
	}

	if len(rs) == 0 {
		if explainMode == types.ExplainFullV1 {
			result.Explanation, err = types.NewTraceV1(*buf, pretty)
			if err != nil {
				writer.ErrorAuto(w, err)
			}
		}
		diagLogger.Log(ctx, decisionID, r.RemoteAddr, path.String(), "", goInput, nil, nil, m, buf)
		writer.JSON(w, 200, result, pretty)
		return
	}

	result.Result = &rs[0].Expressions[0].Value

	if explainMode != types.ExplainOffV1 {
		result.Explanation = s.getExplainResponse(explainMode, *buf, pretty)
	}

	diagLogger.Log(ctx, decisionID, r.RemoteAddr, path.String(), "", goInput, result.Result, nil, m, buf)
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
		if err := s.checkPathScope(ctx, txn, patch.path); err != nil {
			s.abortAuto(ctx, txn, w, err)
			return
		}

		if err := s.store.Write(ctx, txn, patch.op, patch.path, patch.value); err != nil {
			s.abortAuto(ctx, txn, w, err)
			return
		}
	}

	if err := ast.CheckPathConflicts(s.getCompiler(), storage.NonEmpty(ctx, s.store, txn)); len(err) > 0 {
		s.store.Abort(ctx, txn)
		writer.ErrorString(w, http.StatusBadRequest, types.CodeInvalidParameter, err)
		return
	}

	if err := s.store.Commit(ctx, txn); err != nil {
		writer.ErrorAuto(w, err)
	} else {
		writer.Bytes(w, 204, nil)
	}
}

func (s *Server) v1DataPost(w http.ResponseWriter, r *http.Request) {
	m := metrics.New()
	m.Timer(metrics.ServerHandler).Start()

	decisionID := s.generateDecisionID()

	ctx := r.Context()
	vars := mux.Vars(r)
	path := stringPathToDataRef(vars["path"])
	diagLogger := s.evalDiagnosticPolicy(r)

	watch := getWatch(r.URL.Query()[types.ParamWatchV1])
	if watch {
		s.watchQuery(path.String(), w, r, true)
		return
	}

	pretty := getBoolParam(r.URL, types.ParamPrettyV1, true)
	explainMode := getExplain(r.URL.Query()[types.ParamExplainV1], types.ExplainOffV1)
	includeMetrics := getBoolParam(r.URL, types.ParamMetricsV1, true)
	includeInstrumentation := getBoolParam(r.URL, types.ParamInstrumentV1, true)
	partial := getBoolParam(r.URL, types.ParamPartialV1, true)

	m.Timer(metrics.RegoQueryParse).Start()

	input, err := readInputPostV1(r)
	if err != nil {
		writer.ErrorString(w, http.StatusBadRequest, types.CodeInvalidParameter, err)
		return
	}

	var goInput *interface{}
	if input != nil {
		x, err := ast.JSON(input)
		if err != nil {
			writer.ErrorString(w, http.StatusInternalServerError, types.CodeInvalidParameter, errors.Wrapf(err, "could not marshal input"))
			return
		}
		goInput = &x
	}

	m.Timer(metrics.RegoQueryParse).Stop()

	txn, err := s.store.NewTransaction(ctx)
	if err != nil {
		writer.ErrorAuto(w, err)
		return
	}

	defer s.store.Abort(ctx, txn)

	opts := []func(*rego.Rego){
		rego.Compiler(s.getCompiler()),
		rego.Store(s.store),
	}

	var buf *topdown.BufferTracer

	if explainMode != types.ExplainOffV1 || diagLogger.Explain() {
		buf = topdown.NewBufferTracer()
	}

	var instrument bool

	if includeInstrumentation || diagLogger.Instrument() {
		instrument = true
	}

	rego, err := s.makeRego(ctx, partial, txn, input, path.String(), m, instrument, buf, opts)

	if err != nil {
		diagLogger.Log(ctx, decisionID, r.RemoteAddr, path.String(), "", goInput, nil, err, m, nil)
		writer.ErrorAuto(w, err)
		return
	}

	rs, err := rego.Eval(ctx)

	m.Timer(metrics.ServerHandler).Stop()

	// Handle results.
	if err != nil {
		diagLogger.Log(ctx, decisionID, r.RemoteAddr, path.String(), "", goInput, nil, err, m, buf)
		writer.ErrorAuto(w, err)
		return
	}

	result := types.DataResponseV1{
		DecisionID: decisionID,
	}

	if includeMetrics || includeInstrumentation {
		result.Metrics = m.All()
	}

	if len(rs) == 0 {
		if explainMode == types.ExplainFullV1 {
			result.Explanation, err = types.NewTraceV1(*buf, pretty)
			if err != nil {
				writer.ErrorAuto(w, err)
			}
		}
		diagLogger.Log(ctx, decisionID, r.RemoteAddr, path.String(), "", goInput, nil, nil, m, buf)
		writer.JSON(w, 200, result, pretty)
		return
	}

	result.Result = &rs[0].Expressions[0].Value

	if explainMode != types.ExplainOffV1 {
		result.Explanation = s.getExplainResponse(explainMode, *buf, pretty)
	}

	diagLogger.Log(ctx, decisionID, r.RemoteAddr, path.String(), "", goInput, result.Result, nil, m, buf)
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

	path, ok := storage.ParsePathEscaped("/" + strings.Trim(vars["path"], "/"))
	if !ok {
		writer.Error(w, http.StatusBadRequest, types.NewErrorV1(types.CodeInvalidParameter, "bad path: %v", vars["path"]))
		return
	}

	txn, err := s.store.NewTransaction(ctx, storage.WriteParams)
	if err != nil {
		writer.ErrorAuto(w, err)
		return
	}

	if err := s.checkPathScope(ctx, txn, path); err != nil {
		s.abortAuto(ctx, txn, w, err)
		return
	}

	_, err = s.store.Read(ctx, txn, path)

	if err != nil {
		if !storage.IsNotFound(err) {
			s.abortAuto(ctx, txn, w, err)
			return
		}
		if err := storage.MakeDir(ctx, s.store, txn, path[:len(path)-1]); err != nil {
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

	if err := ast.CheckPathConflicts(s.getCompiler(), storage.NonEmpty(ctx, s.store, txn)); len(err) > 0 {
		s.store.Abort(ctx, txn)
		writer.ErrorString(w, http.StatusBadRequest, types.CodeInvalidParameter, err)
		return
	}

	if err := s.store.Commit(ctx, txn); err != nil {
		writer.ErrorAuto(w, err)
	} else {
		writer.Bytes(w, 204, nil)
	}
}

func (s *Server) v1DataDelete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)

	path, ok := storage.ParsePathEscaped("/" + strings.Trim(vars["path"], "/"))
	if !ok {
		writer.Error(w, http.StatusBadRequest, types.NewErrorV1(types.CodeInvalidParameter, "bad path: %v", vars["path"]))
		return
	}

	txn, err := s.store.NewTransaction(ctx, storage.WriteParams)
	if err != nil {
		writer.ErrorAuto(w, err)
		return
	}

	if err := s.checkPathScope(ctx, txn, path); err != nil {
		s.abortAuto(ctx, txn, w, err)
		return
	}

	_, err = s.store.Read(ctx, txn, path)
	if err != nil {
		s.abortAuto(ctx, txn, w, err)
		return
	}

	if err := s.store.Write(ctx, txn, storage.RemoveOp, path, nil); err != nil {
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

	if err := s.checkPolicyIDScope(ctx, txn, id); err != nil {
		s.abortAuto(ctx, txn, w, err)
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

	c := s.getCompiler()

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
	c := s.getCompiler()

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

	if err := s.checkPolicyPackageScope(ctx, txn, parsedMod.Package); err != nil {
		s.abortAuto(ctx, txn, w, err)
		return
	}

	modules, err := s.loadModules(ctx, txn)
	if err != nil {
		s.abortAuto(ctx, txn, w, err)
		return
	}

	modules[path] = parsedMod

	c := ast.NewCompiler().SetErrorLimit(s.errLimit).WithPathConflictsCheck(storage.NonEmpty(ctx, s.store, txn))

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

	response := types.PolicyPutResponseV1{}

	if includeMetrics {
		response.Metrics = m.All()
	}

	writer.JSON(w, http.StatusOK, response, pretty)
}

func (s *Server) v1QueryGet(w http.ResponseWriter, r *http.Request) {
	m := metrics.New()

	decisionID := s.generateDecisionID()

	ctx := r.Context()
	values := r.URL.Query()

	qStrs := values[types.ParamQueryV1]
	if len(qStrs) == 0 {
		writer.Error(w, http.StatusBadRequest, types.NewErrorV1(types.CodeInvalidParameter, "missing parameter 'q'"))
		return
	}
	qStr := qStrs[len(qStrs)-1]

	unsafeBuiltins, parsedQuery, err := validateQuery(qStr)
	if err != nil {
		switch err := err.(type) {
		case ast.Errors:
			writer.Error(w, http.StatusBadRequest, types.NewErrorV1(types.CodeInvalidParameter, types.MsgParseQueryError).WithASTErrors(err))
			return
		default:
			writer.ErrorAuto(w, err)
			return
		}
	} else if len(unsafeBuiltins) > 0 {
		writer.Error(w, http.StatusBadRequest, types.NewErrorV1(types.CodeInvalidParameter, "unsafe built-in function calls in query: %v", strings.Join(unsafeBuiltins, ",")))
		return
	}

	watch := getWatch(r.URL.Query()[types.ParamWatchV1])
	if watch {
		s.watchQuery(qStr, w, r, false)
		return
	}

	pretty := getBoolParam(r.URL, types.ParamPrettyV1, true)
	explainMode := getExplain(r.URL.Query()["explain"], types.ExplainOffV1)
	includeMetrics := getBoolParam(r.URL, types.ParamMetricsV1, true)
	includeInstrumentation := getBoolParam(r.URL, types.ParamInstrumentV1, true)

	results, err := s.execQuery(ctx, r, decisionID, parsedQuery, nil, m, explainMode, includeMetrics, includeInstrumentation, pretty)
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

func (s *Server) v1QueryPost(w http.ResponseWriter, r *http.Request) {
	m := metrics.New()

	decisionID := s.generateDecisionID()

	ctx := r.Context()

	var request types.QueryRequestV1
	err := util.NewJSONDecoder(r.Body).Decode(&request)
	if err != nil {
		writer.Error(w, http.StatusBadRequest, types.NewErrorV1(types.CodeInvalidParameter, "error(s) occurred while decoding request: %v", err.Error()))
		return
	}
	qStr := request.Query
	unsafeBuiltins, parsedQuery, err := validateQuery(qStr)
	if err != nil {
		switch err := err.(type) {
		case ast.Errors:
			writer.Error(w, http.StatusBadRequest, types.NewErrorV1(types.CodeInvalidParameter, types.MsgParseQueryError).WithASTErrors(err))
			return
		default:
			writer.ErrorAuto(w, err)
			return
		}
	} else if len(unsafeBuiltins) > 0 {
		writer.Error(w, http.StatusBadRequest, types.NewErrorV1(types.CodeInvalidParameter, "unsafe built-in function calls in query: %v", strings.Join(unsafeBuiltins, ",")))
		return
	}

	watch := getWatch(r.URL.Query()[types.ParamWatchV1])
	if watch {
		s.watchQuery(qStr, w, r, false)
		return
	}

	pretty := getBoolParam(r.URL, types.ParamPrettyV1, true)
	explainMode := getExplain(r.URL.Query()["explain"], types.ExplainOffV1)
	includeMetrics := getBoolParam(r.URL, types.ParamMetricsV1, true)
	includeInstrumentation := getBoolParam(r.URL, types.ParamInstrumentV1, true)

	results, err := s.execQuery(ctx, r, decisionID, parsedQuery, nil, m, explainMode, includeMetrics, includeInstrumentation, pretty)
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
	includeInstrumentation := getBoolParam(r.URL, types.ParamInstrumentV1, true)

	watch := s.watcher.NewQuery(query).WithInstrumentation(includeInstrumentation).WithRuntime(s.runtime)
	err := watch.Start()

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

			if includeMetrics || includeInstrumentation {
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

func (s *Server) checkPolicyIDScope(ctx context.Context, txn storage.Transaction, id string) error {

	bs, err := s.store.GetPolicy(ctx, txn, id)
	if err != nil {
		return err
	}

	module, err := ast.ParseModule(id, string(bs))
	if err != nil {
		return err
	}

	return s.checkPolicyPackageScope(ctx, txn, module.Package)
}

func (s *Server) checkPolicyPackageScope(ctx context.Context, txn storage.Transaction, pkg *ast.Package) error {

	path, err := pkg.Path.Ptr()
	if err != nil {
		return err
	}

	spath, ok := storage.ParsePathEscaped("/" + path)
	if !ok {
		return types.BadRequestErr("invalid package path: cannot determine scope")
	}

	return s.checkPathScope(ctx, txn, spath)
}

func (s *Server) checkPathScope(ctx context.Context, txn storage.Transaction, path storage.Path) error {

	roots, err := manifest.ReadBundleRoots(ctx, s.store, txn)
	if err != nil {
		if !storage.IsNotFound(err) {
			return err
		}
		return nil
	}

	spath := strings.Trim(path.String(), "/")

	for i := range roots {
		if strings.HasPrefix(spath, roots[i]) || strings.HasPrefix(roots[i], spath) {
			return types.BadRequestErr(fmt.Sprintf("path %v is owned by bundle", spath))
		}
	}

	return nil
}

func (s *Server) evalDiagnosticPolicy(r *http.Request) (logger diagnosticsLogger) {

	// XXX(tsandall): set the decision logger on the diagnostic logger. The
	// diagnostic logger is called in all the necessary locations. The diagnostic
	// logger will make sure to call the decision logger regardless of whether a
	// diagnostic policy is configured. In the future, we can refactor this.
	defer func() {
		logger.revision = s.revision
		logger.logger = s.logger
	}()

	if s.diagnostics == nil {
		return diagnosticsLogger{}
	}

	input, err := makeDiagnosticsInput(r)
	if err != nil {
		return diagnosticsLogger{}
	}

	compiler := s.getCompiler()

	rego := rego.New(
		rego.Store(s.store),
		rego.Compiler(compiler),
		rego.Query(`data.system.diagnostics.config`),
		rego.Input(input),
		rego.Runtime(s.runtime),
	)

	output, err := rego.Eval(r.Context())
	if err != nil {
		return diagnosticsLogger{}
	}

	if len(output) == 1 {
		if config, ok := output[0].Expressions[0].Value.(map[string]interface{}); ok {
			switch config["mode"] {
			case "on":
				return diagnosticsLogger{
					buffer: s.diagnostics,
				}
			case "all":
				return diagnosticsLogger{
					buffer:     s.diagnostics,
					instrument: true,
					explain:    true,
				}
			}
		}
	}

	return diagnosticsLogger{}
}

func (s *Server) getExplainResponse(explainMode types.ExplainModeV1, trace []*topdown.Event, pretty bool) (explanation types.TraceV1) {
	switch explainMode {
	case types.ExplainFullV1:
		var err error
		explanation, err = types.NewTraceV1(trace, pretty)
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

func (s *Server) getCompiler() *ast.Compiler {
	return s.manager.GetCompiler()
}

func (s *Server) makeRego(ctx context.Context, partial bool, txn storage.Transaction, input ast.Value, path string, m metrics.Metrics, instrument bool, tracer topdown.Tracer, opts []func(*rego.Rego)) (*rego.Rego, error) {

	if partial {
		s.mtx.Lock()
		defer s.mtx.Unlock()
		pr, ok := s.partials[path]
		if !ok {
			opts = append(opts, rego.Transaction(txn), rego.Query(path), rego.Metrics(m), rego.Instrument(instrument), rego.Runtime(s.runtime))
			r := rego.New(opts...)
			var err error
			pr, err = r.PartialResult(ctx)
			if err != nil {
				return nil, err
			}
			s.partials[path] = pr
		}
		opts := []func(*rego.Rego){
			rego.ParsedInput(input),
			rego.Transaction(txn),
			rego.Metrics(m),
			rego.Instrument(instrument),
			rego.Tracer(tracer),
		}
		return pr.Rego(opts...), nil
	}

	opts = append(opts, rego.Transaction(txn), rego.Query(path), rego.ParsedInput(input), rego.Metrics(m), rego.Tracer(tracer), rego.Instrument(instrument), rego.Runtime(s.runtime))
	return rego.New(opts...), nil
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
		impl.path, ok = storage.ParsePathEscaped(path)
		if !ok {
			return nil, types.BadPatchPathErr(op.Path)
		}

		result = append(result, impl)
	}

	return result, nil
}

func (s *Server) generateDecisionID() string {
	if s.decisionIDFactory != nil {
		return s.decisionIDFactory()
	}
	return ""
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
		if y, err := url.PathUnescape(x); err == nil {
			x = y
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

func validateQuery(query string) ([]string, ast.Body, error) {

	var body ast.Body
	body, err := ast.ParseBody(query)
	if err != nil {
		return []string{}, nil, err
	}

	unsafeOperators, nil := validateParsedQuery(body)
	return unsafeOperators, body, nil
}

func validateParsedQuery(body ast.Body) ([]string, error) {
	unsafeOperators := []string{}
	ast.WalkExprs(body, func(x *ast.Expr) bool {
		if x.IsCall() {
			operator := x.Operator().String()
			if _, ok := unsafeBuiltinsMap[operator]; ok {
				unsafeOperators = append(unsafeOperators, operator)
			}
		}
		return false
	})
	return unsafeOperators, nil
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
		}
	}
	return zero
}

func makeDiagnosticsInput(r *http.Request) (map[string]interface{}, error) {
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

func readInputV0(r *http.Request) (ast.Value, error) {
	bs, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	bs = bytes.TrimSpace(bs)
	if len(bs) == 0 {
		return nil, nil
	}
	var x interface{}

	if strings.Contains(r.Header.Get("Content-Type"), "yaml") {
		if err := util.Unmarshal(bs, &x); err != nil {
			return nil, err
		}
	} else if err := util.UnmarshalJSON(bs, &x); err != nil {
		return nil, err
	}

	return ast.InterfaceToValue(x)
}

func readInputGetV1(str string) (ast.Value, error) {
	var input interface{}
	if err := util.UnmarshalJSON([]byte(str), &input); err != nil {
		return nil, errors.Wrapf(err, "parameter contains malformed input document")
	}
	return ast.InterfaceToValue(input)
}

func readInputPostV1(r *http.Request) (ast.Value, error) {

	bs, err := ioutil.ReadAll(r.Body)

	if err != nil {
		return nil, err
	}

	if len(bs) > 0 {

		ct := r.Header.Get("Content-Type")

		var request types.DataRequestV1

		// There is no standard for yaml mime-type so we just look for
		// anything related
		if strings.Contains(ct, "yaml") {
			if err := util.Unmarshal(bs, &request); err != nil {
				return nil, errors.Wrapf(err, "body contains malformed input document")
			}
		} else if err := util.UnmarshalJSON(bs, &request); err != nil {
			return nil, errors.Wrapf(err, "body contains malformed input document")
		}

		if request.Input == nil {
			return nil, nil
		}

		return ast.InterfaceToValue(*request.Input)
	}

	return nil, nil
}

type compileRequest struct {
	Query    ast.Body
	Input    ast.Value
	Unknowns []*ast.Term
}

func readInputCompilePostV1(r io.ReadCloser) (*compileRequest, *types.ErrorV1) {

	var request types.CompileRequestV1

	err := util.NewJSONDecoder(r).Decode(&request)
	if err != nil {
		return nil, types.NewErrorV1(types.CodeInvalidParameter, "error(s) occurred while decoding request: %v", err.Error())
	}

	query, err := ast.ParseBody(request.Query)
	if err != nil {
		switch err := err.(type) {
		case ast.Errors:
			return nil, types.NewErrorV1(types.CodeInvalidParameter, types.MsgParseQueryError).WithASTErrors(err)
		default:
			return nil, types.NewErrorV1(types.CodeInvalidParameter, "%v: %v", types.MsgParseQueryError, err)
		}
	} else if len(query) == 0 {
		return nil, types.NewErrorV1(types.CodeInvalidParameter, "missing required 'query' value")
	}

	var input ast.Value
	if request.Input != nil {
		input, err = ast.InterfaceToValue(*request.Input)
		if err != nil {
			return nil, types.NewErrorV1(types.CodeInvalidParameter, "error(s) occurred while converting input: %v", err)
		}
	}

	var unknowns []*ast.Term
	if request.Unknowns != nil {
		unknowns = make([]*ast.Term, len(*request.Unknowns))
		for i, s := range *request.Unknowns {
			unknowns[i], err = ast.ParseTerm(s)
			if err != nil {
				return nil, types.NewErrorV1(types.CodeInvalidParameter, "error(s) occurred while parsing unknowns: %v", err)
			}
		}
	}

	result := &compileRequest{
		Query:    query,
		Input:    input,
		Unknowns: unknowns,
	}

	return result, nil
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
	</form>`, template.HTMLEscapeString(query), template.HTMLEscapeString(input), explainRadioCheck[0], explainRadioCheck[1])
}

func renderQueryResult(w io.Writer, results interface{}, err error, t0 time.Time) {

	buf, err2 := json.MarshalIndent(results, "", "  ")
	d := time.Since(t0)

	if err != nil {
		fmt.Fprintf(w, "Query error (took %v): <pre>%v</pre>", d, template.HTMLEscapeString(err.Error()))
	} else if err2 != nil {
		fmt.Fprintf(w, "JSON marshal error: <pre>%v</pre>", template.HTMLEscapeString(err.Error()))
	} else {
		fmt.Fprintf(w, "Query results (took %v):<br>", d)
		fmt.Fprintf(w, "<pre>%s</pre>", template.HTMLEscapeString(string(buf)))
	}
}

func renderVersion(w http.ResponseWriter) {
	fmt.Fprintln(w, "Version: "+version.Version+"<br>")
	fmt.Fprintln(w, "Build Commit: "+version.Vcs+"<br>")
	fmt.Fprintln(w, "Build Timestamp: "+version.Timestamp+"<br>")
	fmt.Fprintln(w, "Build Hostname: "+version.Hostname+"<br>")
	fmt.Fprintln(w, "<br>")
}

type diagnosticsLogger struct {
	logger     func(context.Context, *Info)
	revision   string
	explain    bool
	instrument bool
	buffer     Buffer
}

func (l diagnosticsLogger) Explain() bool {
	return l.explain
}

func (l diagnosticsLogger) Instrument() bool {
	return l.instrument
}

func (l diagnosticsLogger) Log(ctx context.Context, decisionID, remoteAddr, path string, query string, input *interface{}, results *interface{}, err error, m metrics.Metrics, tracer *topdown.BufferTracer) {

	info := &Info{
		Revision:   l.revision,
		Timestamp:  time.Now().UTC(),
		DecisionID: decisionID,
		RemoteAddr: remoteAddr,
		Path:       path,
		Query:      query,
		Input:      input,
		Results:    results,
		Error:      err,
		Metrics:    m,
	}

	if tracer != nil {
		info.Trace = *tracer
	}

	if l.logger != nil {
		l.logger(ctx, info)
	}

	if l.buffer == nil {
		return
	}

	l.buffer.Push(info)
}

type patchImpl struct {
	path  storage.Path
	op    storage.PatchOp
	value interface{}
}

func parseURL(s string, useHTTPSByDefault bool) (*url.URL, error) {
	if !strings.Contains(s, "://") {
		scheme := "http://"
		if useHTTPSByDefault {
			scheme = "https://"
		}
		s = scheme + s
	}
	return url.Parse(s)
}
