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
	"net/http/pprof"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/pkg/errors"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/bundle"
	"github.com/open-policy-agent/opa/metrics"
	"github.com/open-policy-agent/opa/plugins"
	bundlePlugin "github.com/open-policy-agent/opa/plugins/bundle"
	"github.com/open-policy-agent/opa/rego"
	"github.com/open-policy-agent/opa/server/authorizer"
	"github.com/open-policy-agent/opa/server/identifier"
	"github.com/open-policy-agent/opa/server/types"
	"github.com/open-policy-agent/opa/server/writer"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/topdown"
	"github.com/open-policy-agent/opa/topdown/lineage"
	"github.com/open-policy-agent/opa/util"
	"github.com/open-policy-agent/opa/version"
	"github.com/open-policy-agent/opa/watch"
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

// map of unsafe builtins
var unsafeBuiltinsMap = map[string]struct{}{ast.HTTPSend.Name: struct{}{}}

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
	revisions         map[string]string
	legacyRevision    string
	buffer            Buffer
	logger            func(context.Context, *Info) error
	errLimit          int
	pprofEnabled      bool
	runtime           *ast.Term
	httpListeners     []httpListener
	bundleStatuses    map[string]*bundlePlugin.Status
	bundleStatusMtx   sync.RWMutex
	metrics           Metrics
}

// Metrics defines the interface that the server requires for recording HTTP
// handler metrics.
type Metrics interface {
	RegisterEndpoints(registrar func(path, method string, handler http.Handler))
	InstrumentHandler(handler http.Handler, label string) http.Handler
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

	bp := bundlePlugin.Lookup(s.manager)
	if bp != nil {

		// initialize statuses to empty defaults for server /health check
		s.bundleStatuses = map[string]*bundlePlugin.Status{}
		for bundleName := range bp.Config().Bundles {
			s.bundleStatuses[bundleName] = &bundlePlugin.Status{Name: bundleName}
		}

		bp.RegisterBulkListener("REST API Server", s.updateBundleStatus)
	}

	// Check if there is a bundle revision available at the legacy storage path
	rev, err := bundle.LegacyReadRevisionFromStore(ctx, s.store, txn)
	if err == nil && rev != "" {
		s.legacyRevision = rev
	}

	return s, s.store.Commit(ctx, txn)
}

// Shutdown will attempt to gracefully shutdown each of the http servers
// currently in use by the OPA Server. If any exceed the deadline specified
// by the context an error will be returned.
func (s *Server) Shutdown(ctx context.Context) error {
	errChan := make(chan error)
	for _, srvr := range s.httpListeners {
		go func(s httpListener) {
			errChan <- s.Shutdown(ctx)
		}(srvr)
	}
	// wait until each server has finished shutting down
	var errorList []error
	for i := 0; i < len(s.httpListeners); i++ {
		err := <-errChan
		if err != nil {
			errorList = append(errorList, err)
		}
	}

	if len(errorList) > 0 {
		errMsg := "error while shutting down: "
		for i, err := range errorList {
			errMsg += fmt.Sprintf("(%d) %s. ", i, err.Error())
		}
		return errors.New(errMsg)
	}
	return nil
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

// WithMetrics sets the metrics provider used by the server.
func (s *Server) WithMetrics(m Metrics) *Server {
	s.metrics = m
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

// WithPprofEnabled sets whether pprof endpoints are enabled
func (s *Server) WithPprofEnabled(pprofEnabled bool) *Server {
	s.pprofEnabled = pprofEnabled
	return s
}

// WithDecisionLogger sets the decision logger used by the
// server. DEPRECATED. Use WithDecisionLoggerWithErr instead.
func (s *Server) WithDecisionLogger(logger func(context.Context, *Info)) *Server {
	s.logger = func(ctx context.Context, info *Info) error {
		logger(ctx, info)
		return nil
	}
	return s
}

// WithDecisionLoggerWithErr sets the decision logger used by the server.
func (s *Server) WithDecisionLoggerWithErr(logger func(context.Context, *Info) error) *Server {
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
		var listener httpListener
		switch parsedURL.Scheme {
		case "unix":
			loop, listener, err = s.getListenerForUNIXSocket(parsedURL)
		case "http":
			loop, listener, err = s.getListenerForHTTPServer(parsedURL)
		case "https":
			loop, listener, err = s.getListenerForHTTPSServer(parsedURL)
		default:
			err = fmt.Errorf("invalid url scheme %q", parsedURL.Scheme)
		}
		if err != nil {
			return nil, err
		}
		s.httpListeners = append(s.httpListeners, listener)
		loops = append(loops, loop)
	}

	if s.insecureAddr != "" {
		parsedURL, err := parseURL(s.insecureAddr, false)
		if err != nil {
			return nil, err
		}
		loop, httpListener, err := s.getListenerForHTTPServer(parsedURL)
		if err != nil {
			return nil, err
		}
		s.httpListeners = append(s.httpListeners, httpListener)
		loops = append(loops, loop)
	}

	return loops, nil
}

// Addrs returns a list of addresses that the server is listening on.
// if the server hasn't been started it will not return an address.
func (s *Server) Addrs() []string {
	var addrs []string
	for _, l := range s.httpListeners {
		a := l.Addr()
		if a != "" {
			addrs = append(addrs, a)
		}
	}
	return addrs
}

type tcpKeepAliveListener struct {
	*net.TCPListener
}

func (ln tcpKeepAliveListener) Accept() (net.Conn, error) {
	tc, err := ln.AcceptTCP()
	if err != nil {
		return nil, err
	}
	tc.SetKeepAlive(true)
	tc.SetKeepAlivePeriod(3 * time.Minute)
	return tc, nil
}

type httpListener interface {
	Addr() string
	ListenAndServe() error
	ListenAndServeTLS(certFile, keyFile string) error
	Shutdown(ctx context.Context) error
}

// baseHTTPListener is just a wrapper around http.Server
type baseHTTPListener struct {
	s *http.Server
	l net.Listener
}

var _ httpListener = (*baseHTTPListener)(nil)

func newHTTPListener(srvr *http.Server) httpListener {
	return &baseHTTPListener{srvr, nil}
}

func newHTTPUnixSocketListener(srvr *http.Server, l net.Listener) httpListener {
	return &baseHTTPListener{srvr, l}
}

func (b *baseHTTPListener) ListenAndServe() error {
	addr := b.s.Addr
	if addr == "" {
		addr = ":http"
	}
	var err error
	b.l, err = net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	return b.s.Serve(tcpKeepAliveListener{b.l.(*net.TCPListener)})
}

func (b *baseHTTPListener) Addr() string {
	if b.l != nil {
		if addr := b.l.(*net.TCPListener).Addr(); addr != nil {
			return addr.String()
		}
	}
	return ""
}

func (b *baseHTTPListener) ListenAndServeTLS(certFile, keyFile string) error {
	addr := b.s.Addr
	if addr == "" {
		addr = ":https"
	}

	var err error
	b.l, err = net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	defer b.l.Close()

	return b.s.ServeTLS(tcpKeepAliveListener{b.l.(*net.TCPListener)}, certFile, keyFile)
}

func (b *baseHTTPListener) Shutdown(ctx context.Context) error {
	return b.s.Shutdown(ctx)
}

func (s *Server) getListenerForHTTPServer(u *url.URL) (Loop, httpListener, error) {
	httpServer := http.Server{
		Addr:    u.Host,
		Handler: s.Handler,
	}

	l := newHTTPListener(&httpServer)

	return l.ListenAndServe, l, nil
}

func (s *Server) getListenerForHTTPSServer(u *url.URL) (Loop, httpListener, error) {

	if s.cert == nil {
		return nil, nil, fmt.Errorf("TLS certificate required but not supplied")
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

	l := newHTTPListener(&httpsServer)

	httpsLoop := func() error { return l.ListenAndServeTLS("", "") }

	return httpsLoop, l, nil
}

func (s *Server) getListenerForUNIXSocket(u *url.URL) (Loop, httpListener, error) {
	socketPath := u.Host + u.Path

	// Remove domain socket file in case it already exists.
	os.Remove(socketPath)

	domainSocketServer := http.Server{Handler: s.Handler}
	unixListener, err := net.Listen("unix", socketPath)
	if err != nil {
		return nil, nil, err
	}

	l := newHTTPUnixSocketListener(&domainSocketServer, unixListener)

	domainSocketLoop := func() error { return domainSocketServer.Serve(unixListener) }
	return domainSocketLoop, l, nil
}

func (s *Server) initRouter() {
	router := s.router

	if router == nil {
		router = mux.NewRouter()
	}

	router.UseEncodedPath()
	router.StrictSlash(true)
	if s.metrics != nil {
		s.metrics.RegisterEndpoints(func(path, method string, handler http.Handler) {
			router.Handle(path, handler).Methods(method)
		})
	}
	router.Handle("/health", s.instrumentHandler(http.HandlerFunc(s.unversionedGetHealth), PromHandlerHealth)).Methods(http.MethodGet)
	if s.pprofEnabled {
		router.HandleFunc("/debug/pprof/", pprof.Index)
		router.Handle("/debug/pprof/allocs", pprof.Handler("allocs"))
		router.Handle("/debug/pprof/block", pprof.Handler("block"))
		router.Handle("/debug/pprof/heap", pprof.Handler("heap"))
		router.Handle("/debug/pprof/mutex", pprof.Handler("mutex"))
		router.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		router.HandleFunc("/debug/pprof/profile", pprof.Profile)
		router.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		router.HandleFunc("/debug/pprof/trace", pprof.Trace)
	}
	s.registerHandler(router, 0, "/data/{path:.+}", http.MethodPost, s.instrumentHandler(s.v0DataPost, PromHandlerV0Data))
	s.registerHandler(router, 0, "/data", http.MethodPost, s.instrumentHandler(s.v0DataPost, PromHandlerV0Data))
	s.registerHandler(router, 1, "/data/{path:.+}", http.MethodDelete, s.instrumentHandler(s.v1DataDelete, PromHandlerV1Data))
	s.registerHandler(router, 1, "/data/{path:.+}", http.MethodPut, s.instrumentHandler(s.v1DataPut, PromHandlerV1Data))
	s.registerHandler(router, 1, "/data", http.MethodPut, s.instrumentHandler(s.v1DataPut, PromHandlerV1Data))
	s.registerHandler(router, 1, "/data/{path:.+}", http.MethodGet, s.instrumentHandler(s.v1DataGet, PromHandlerV1Data))
	s.registerHandler(router, 1, "/data", http.MethodGet, s.instrumentHandler(s.v1DataGet, PromHandlerV1Data))
	s.registerHandler(router, 1, "/data/{path:.+}", http.MethodPatch, s.instrumentHandler(s.v1DataPatch, PromHandlerV1Data))
	s.registerHandler(router, 1, "/data", http.MethodPatch, s.instrumentHandler(s.v1DataPatch, PromHandlerV1Data))
	s.registerHandler(router, 1, "/data/{path:.+}", http.MethodPost, s.instrumentHandler(s.v1DataPost, PromHandlerV1Data))
	s.registerHandler(router, 1, "/data", http.MethodPost, s.instrumentHandler(s.v1DataPost, PromHandlerV1Data))
	s.registerHandler(router, 1, "/policies", http.MethodGet, s.instrumentHandler(s.v1PoliciesList, PromHandlerV1Policies))
	s.registerHandler(router, 1, "/policies/{path:.+}", http.MethodDelete, s.instrumentHandler(s.v1PoliciesDelete, PromHandlerV1Policies))
	s.registerHandler(router, 1, "/policies/{path:.+}", http.MethodGet, s.instrumentHandler(s.v1PoliciesGet, PromHandlerV1Policies))
	s.registerHandler(router, 1, "/policies/{path:.+}", http.MethodPut, s.instrumentHandler(s.v1PoliciesPut, PromHandlerV1Policies))
	s.registerHandler(router, 1, "/query", http.MethodGet, s.instrumentHandler(s.v1QueryGet, PromHandlerV1Query))
	s.registerHandler(router, 1, "/query", http.MethodPost, s.instrumentHandler(s.v1QueryPost, PromHandlerV1Query))
	s.registerHandler(router, 1, "/compile", http.MethodPost, s.instrumentHandler(s.v1CompilePost, PromHandlerV1Compile))
	router.Handle("/", s.instrumentHandler(http.HandlerFunc(s.unversionedPost), PromHandlerIndex)).Methods(http.MethodPost)
	router.Handle("/", s.instrumentHandler(http.HandlerFunc(s.indexGet), PromHandlerIndex)).Methods(http.MethodGet)
	// These are catch all handlers that respond 405 for resources that exist but the method is not allowed
	router.Handle("/v0/data/{path:.*}", s.instrumentHandler(writer.HTTPStatus(405), PromHandlerCatch)).Methods(http.MethodGet, http.MethodHead,
		http.MethodConnect, http.MethodDelete, http.MethodOptions, http.MethodPatch, http.MethodPut, http.MethodTrace)
	router.Handle("/v0/data", s.instrumentHandler(writer.HTTPStatus(405), PromHandlerCatch)).Methods(http.MethodGet, http.MethodHead,
		http.MethodConnect, http.MethodDelete, http.MethodOptions, http.MethodPatch, http.MethodPut,
		http.MethodTrace)
	// v1 Data catch all
	router.Handle("/v1/data/{path:.*}", s.instrumentHandler(writer.HTTPStatus(405), PromHandlerCatch)).Methods(http.MethodHead,
		http.MethodConnect, http.MethodOptions, http.MethodTrace)
	router.Handle("/v1/data", s.instrumentHandler(writer.HTTPStatus(405), PromHandlerCatch)).Methods(http.MethodHead,
		http.MethodConnect, http.MethodDelete, http.MethodOptions, http.MethodTrace)
	// Policies catch all
	router.Handle("/v1/policies", s.instrumentHandler(writer.HTTPStatus(405), PromHandlerCatch)).Methods(http.MethodHead,
		http.MethodConnect, http.MethodDelete, http.MethodOptions, http.MethodTrace, http.MethodPost, http.MethodPut,
		http.MethodPatch)
	// Policies (/policies/{path.+} catch all
	router.Handle("/v1/policies/{path:.*}", s.instrumentHandler(writer.HTTPStatus(405), PromHandlerCatch)).Methods(http.MethodHead,
		http.MethodConnect, http.MethodOptions, http.MethodTrace, http.MethodPost)
	// Query catch all
	router.Handle("/v1/query/{path:.*}", s.instrumentHandler(writer.HTTPStatus(405), PromHandlerCatch)).Methods(http.MethodHead,
		http.MethodConnect, http.MethodDelete, http.MethodOptions, http.MethodTrace, http.MethodPost, http.MethodPut, http.MethodPatch)
	router.Handle("/v1/query", s.instrumentHandler(writer.HTTPStatus(405), PromHandlerCatch)).Methods(http.MethodHead,
		http.MethodConnect, http.MethodDelete, http.MethodOptions, http.MethodTrace, http.MethodPut, http.MethodPatch)
	s.Handler = router
}

func (s *Server) instrumentHandler(handler func(http.ResponseWriter, *http.Request), label string) http.Handler {
	if s.metrics != nil {
		return s.metrics.InstrumentHandler(http.HandlerFunc(handler), label)
	}
	return http.HandlerFunc(handler)
}

func (s *Server) execQuery(ctx context.Context, r *http.Request, txn storage.Transaction, decisionID string, parsedQuery ast.Body, input ast.Value, m metrics.Metrics, explainMode types.ExplainModeV1, includeMetrics, includeInstrumentation, pretty bool) (results types.QueryResponseV1, err error) {

	logger := s.getDecisionLogger()

	var buf *topdown.BufferTracer
	if explainMode != types.ExplainOffV1 {
		buf = topdown.NewBufferTracer()
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
		rego.Transaction(txn),
		rego.Compiler(compiler),
		rego.ParsedQuery(parsedQuery),
		rego.ParsedInput(input),
		rego.Metrics(m),
		rego.Instrument(includeInstrumentation),
		rego.Tracer(buf),
		rego.Runtime(s.runtime),
		rego.UnsafeBuiltins(unsafeBuiltinsMap),
	)

	output, err := rego.Eval(ctx)
	if err != nil {
		_ = logger.Log(ctx, txn, decisionID, r.RemoteAddr, "", parsedQuery.String(), rawInput, nil, err, m)
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
	err = logger.Log(ctx, txn, decisionID, r.RemoteAddr, "", parsedQuery.String(), rawInput, &x, nil, m)
	return results, err
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

	parsedQuery, _ := validateQuery(qStr)

	txn, err := s.store.NewTransaction(ctx)
	if err != nil {
		renderQueryResult(w, nil, err, t0)
		return
	}

	defer s.store.Abort(ctx, txn)

	results, err := s.execQuery(ctx, r, txn, decisionID, parsedQuery, input, nil, explainMode, false, false, true)
	if err != nil {
		renderQueryResult(w, nil, err, t0)
		return
	}

	renderQueryResult(w, results, err, t0)
}

func (s *Server) registerHandler(router *mux.Router, version int, path string, method string, h http.Handler) {
	prefix := fmt.Sprintf("/v%d", version)
	router.Handle(prefix+path, h).Methods(method)
}

func (s *Server) reload(ctx context.Context, txn storage.Transaction, event storage.TriggerEvent) {
	// reset some cached info
	s.partials = map[string]rego.PartialResult{}
	s.revisions = map[string]string{}

	// read all bundle revisions from storage (if any exist)
	names, err := bundle.ReadBundleNamesFromStore(ctx, s.store, txn)
	if err != nil && !storage.IsNotFound(err) {
		panic(err)
	}

	for _, name := range names {
		r, err := bundle.ReadBundleRevisionFromStore(ctx, s.store, txn, name)
		if err != nil && !storage.IsNotFound(err) {
			panic(err)
		}
		s.revisions[name] = r
	}

	// Check if we still have a legacy bundle manifest in the store
	s.legacyRevision, err = bundle.LegacyReadRevisionFromStore(ctx, s.store, txn)
	if err != nil && !storage.IsNotFound(err) {
		panic(err)
	}
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
	logger := s.getDecisionLogger()
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

	rego := rego.New(
		rego.Compiler(s.getCompiler()),
		rego.Store(s.store),
		rego.Transaction(txn),
		rego.ParsedInput(input),
		rego.Query(path.String()),
		rego.Metrics(m),
		rego.Runtime(s.runtime),
		rego.UnsafeBuiltins(unsafeBuiltinsMap),
	)

	rs, err := rego.Eval(ctx)

	m.Timer(metrics.ServerHandler).Stop()

	// Handle results.
	if err != nil {
		_ = logger.Log(ctx, txn, decisionID, r.RemoteAddr, path.String(), "", goInput, nil, err, m)
		writer.ErrorAuto(w, err)
		return
	}

	if len(rs) == 0 {
		err := types.NewErrorV1(types.CodeUndefinedDocument, fmt.Sprintf("%v: %v", types.MsgUndefinedError, path))

		if logErr := logger.Log(ctx, txn, decisionID, r.RemoteAddr, path.String(), "", goInput, nil, err, m); logErr != nil {
			writer.ErrorAuto(w, logErr)
			return
		}

		writer.Error(w, 404, err)
		return
	}

	err = logger.Log(ctx, txn, decisionID, r.RemoteAddr, path.String(), "", goInput, &rs[0].Expressions[0].Value, nil, m)
	if err != nil {
		writer.ErrorAuto(w, err)
		return
	}

	writer.JSON(w, 200, rs[0].Expressions[0].Value, false)
}

func (s *Server) updateBundleStatus(status map[string]*bundlePlugin.Status) {
	s.bundleStatusMtx.Lock()
	defer s.bundleStatusMtx.Unlock()
	s.bundleStatuses = status
}

func (s *Server) canEval(ctx context.Context) bool {
	// Create very simple query that binds a single variable.
	eval := rego.New(rego.Compiler(s.getCompiler()),
		rego.Store(s.store), rego.Query("x = 1"))
	// Run evaluation.
	rs, err := eval.Eval(ctx)
	if err != nil {
		return false
	}
	type emptyObject struct{}
	v, ok := rs[0].Bindings["x"]
	if ok {
		jsonNumber, ok := v.(json.Number)
		if ok && jsonNumber.String() == "1" {
			return true
		}
	}
	return false
}

func (s *Server) bundlesActivated() bool {
	s.bundleStatusMtx.RLock()
	defer s.bundleStatusMtx.RUnlock()

	for _, status := range s.bundleStatuses {
		// Ensure that all of the bundle statuses have an activation time set on them
		if (status.LastSuccessfulActivation == time.Time{}) {
			return false
		}
	}

	return true
}

func (s *Server) unversionedGetHealth(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	includeBundleStatus := getBoolParam(r.URL, types.ParamBundleActivationV1, false)

	// Ensure the server can evaluate a simple query
	type emptyObject struct{}
	if !s.canEval(ctx) {
		writer.JSON(w, http.StatusInternalServerError, emptyObject{}, false)
		return
	}

	// Ensure that bundles (if configured, and requested to be included in the result)
	// have been activated successfully.
	if includeBundleStatus && s.hasBundle() && !s.bundlesActivated() {
		writer.JSON(w, http.StatusInternalServerError, emptyObject{}, false)
		return
	}

	writer.JSON(w, http.StatusOK, emptyObject{}, false)
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

	eval := rego.New(
		rego.Compiler(s.getCompiler()),
		rego.Store(s.store),
		rego.Transaction(txn),
		rego.ParsedQuery(request.Query),
		rego.ParsedInput(request.Input),
		rego.ParsedUnknowns(request.Unknowns),
		rego.Tracer(buf),
		rego.Instrument(includeInstrumentation),
		rego.Metrics(m),
		rego.Runtime(s.runtime),
		rego.UnsafeBuiltins(unsafeBuiltinsMap),
	)

	pq, err := eval.Partial(ctx)
	if err != nil {
		switch err := err.(type) {
		case ast.Errors:
			writer.Error(w, http.StatusBadRequest, types.NewErrorV1(types.CodeInvalidParameter, types.MsgCompileModuleError).WithASTErrors(err))
		default:
			writer.ErrorAuto(w, err)
		}
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
	logger := s.getDecisionLogger()

	watch := getWatch(r.URL.Query()[types.ParamWatchV1])
	if watch {
		s.watchQuery(path.String(), w, r, true)
		return
	}

	pretty := getBoolParam(r.URL, types.ParamPrettyV1, true)
	explainMode := getExplain(r.URL.Query()["explain"], types.ExplainOffV1)
	includeMetrics := getBoolParam(r.URL, types.ParamMetricsV1, true)
	includeInstrumentation := getBoolParam(r.URL, types.ParamInstrumentV1, true)
	provenance := getBoolParam(r.URL, types.ParamProvenanceV1, true)

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

	if explainMode != types.ExplainOffV1 {
		buf = topdown.NewBufferTracer()
	}

	rego := rego.New(
		rego.Compiler(s.getCompiler()),
		rego.Store(s.store),
		rego.Transaction(txn),
		rego.ParsedInput(input),
		rego.Query(path.String()),
		rego.Metrics(m),
		rego.Tracer(buf),
		rego.Instrument(includeInstrumentation),
		rego.Runtime(s.runtime),
		rego.UnsafeBuiltins(unsafeBuiltinsMap),
	)

	rs, err := rego.Eval(ctx)

	// Handle results.
	if err != nil {
		_ = logger.Log(ctx, txn, decisionID, r.RemoteAddr, path.String(), "", goInput, nil, err, m)
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

	if provenance {
		result.Provenance = s.getProvenance()
	}

	if len(rs) == 0 {
		if explainMode == types.ExplainFullV1 {
			result.Explanation, err = types.NewTraceV1(*buf, pretty)
			if err != nil {
				writer.ErrorAuto(w, err)
			}
		}
		err = logger.Log(ctx, txn, decisionID, r.RemoteAddr, path.String(), "", goInput, nil, nil, m)
		if err != nil {
			writer.ErrorAuto(w, err)
			return
		}
		writer.JSON(w, 200, result, pretty)
		return
	}

	result.Result = &rs[0].Expressions[0].Value

	if explainMode != types.ExplainOffV1 {
		result.Explanation = s.getExplainResponse(explainMode, *buf, pretty)
	}

	err = logger.Log(ctx, txn, decisionID, r.RemoteAddr, path.String(), "", goInput, result.Result, nil, m)
	if err != nil {
		writer.ErrorAuto(w, err)
		return
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
	logger := s.getDecisionLogger()

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
	provenance := getBoolParam(r.URL, types.ParamProvenanceV1, true)

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

	if explainMode != types.ExplainOffV1 {
		buf = topdown.NewBufferTracer()
	}

	rego, err := s.makeRego(ctx, partial, txn, input, path.String(), m, includeInstrumentation, buf, opts)

	if err != nil {
		_ = logger.Log(ctx, txn, decisionID, r.RemoteAddr, path.String(), "", goInput, nil, err, m)
		writer.ErrorAuto(w, err)
		return
	}

	rs, err := rego.Eval(ctx)

	m.Timer(metrics.ServerHandler).Stop()

	// Handle results.
	if err != nil {
		_ = logger.Log(ctx, txn, decisionID, r.RemoteAddr, path.String(), "", goInput, nil, err, m)
		writer.ErrorAuto(w, err)
		return
	}

	result := types.DataResponseV1{
		DecisionID: decisionID,
	}

	if includeMetrics || includeInstrumentation {
		result.Metrics = m.All()
	}

	if provenance {
		result.Provenance = s.getProvenance()
	}

	if len(rs) == 0 {
		if explainMode == types.ExplainFullV1 {
			result.Explanation, err = types.NewTraceV1(*buf, pretty)
			if err != nil {
				writer.ErrorAuto(w, err)
			}
		}
		err = logger.Log(ctx, txn, decisionID, r.RemoteAddr, path.String(), "", goInput, nil, nil, m)
		if err != nil {
			writer.ErrorAuto(w, err)
			return
		}
		writer.JSON(w, 200, result, pretty)
		return
	}

	result.Result = &rs[0].Expressions[0].Value

	if explainMode != types.ExplainOffV1 {
		result.Explanation = s.getExplainResponse(explainMode, *buf, pretty)
	}

	err = logger.Log(ctx, txn, decisionID, r.RemoteAddr, path.String(), "", goInput, result.Result, nil, m)
	if err != nil {
		writer.ErrorAuto(w, err)
		return
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

	txn, err := s.store.NewTransaction(ctx, storage.WriteParams)
	if err != nil {
		writer.ErrorAuto(w, err)
		return
	}

	if bs, err := s.store.GetPolicy(ctx, txn, path); err != nil {
		if !storage.IsNotFound(err) {
			s.abortAuto(ctx, txn, w, err)
			return
		}
	} else if bytes.Equal(buf, bs) {
		s.store.Abort(ctx, txn)
		response := types.PolicyPutResponseV1{}
		if includeMetrics {
			response.Metrics = m.All()
		}
		writer.JSON(w, http.StatusOK, response, pretty)
		return
	}

	m.Timer(metrics.RegoModuleParse).Start()
	parsedMod, err := ast.ParseModule(path, string(buf))
	m.Timer(metrics.RegoModuleParse).Stop()

	if err != nil {
		s.store.Abort(ctx, txn)
		switch err := err.(type) {
		case ast.Errors:
			writer.Error(w, http.StatusBadRequest, types.NewErrorV1(types.CodeInvalidParameter, types.MsgCompileModuleError).WithASTErrors(err))
		default:
			writer.ErrorString(w, http.StatusBadRequest, types.CodeInvalidParameter, err)
		}
		return
	}

	if parsedMod == nil {
		s.store.Abort(ctx, txn)
		writer.Error(w, http.StatusBadRequest, types.NewErrorV1(types.CodeInvalidParameter, "empty module"))
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

	parsedQuery, err := validateQuery(qStr)
	if err != nil {
		switch err := err.(type) {
		case ast.Errors:
			writer.Error(w, http.StatusBadRequest, types.NewErrorV1(types.CodeInvalidParameter, types.MsgParseQueryError).WithASTErrors(err))
			return
		default:
			writer.ErrorAuto(w, err)
			return
		}
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

	txn, err := s.store.NewTransaction(ctx)
	if err != nil {
		writer.ErrorAuto(w, err)
		return
	}

	defer s.store.Abort(ctx, txn)

	results, err := s.execQuery(ctx, r, txn, decisionID, parsedQuery, nil, m, explainMode, includeMetrics, includeInstrumentation, pretty)
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
	parsedQuery, err := validateQuery(qStr)
	if err != nil {
		switch err := err.(type) {
		case ast.Errors:
			writer.Error(w, http.StatusBadRequest, types.NewErrorV1(types.CodeInvalidParameter, types.MsgParseQueryError).WithASTErrors(err))
			return
		default:
			writer.ErrorAuto(w, err)
			return
		}
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

	txn, err := s.store.NewTransaction(ctx)
	if err != nil {
		writer.ErrorAuto(w, err)
		return
	}

	defer s.store.Abort(ctx, txn)

	results, err := s.execQuery(ctx, r, txn, decisionID, parsedQuery, nil, m, explainMode, includeMetrics, includeInstrumentation, pretty)
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

	names, err := bundle.ReadBundleNamesFromStore(ctx, s.store, txn)
	if err != nil {
		if !storage.IsNotFound(err) {
			return err
		}
		return nil
	}

	bundleRoots := map[string][]string{}
	for _, name := range names {
		roots, err := bundle.ReadBundleRootsFromStore(ctx, s.store, txn, name)
		if err != nil && !storage.IsNotFound(err) {
			return err
		}
		bundleRoots[name] = roots
	}

	spath := strings.Trim(path.String(), "/")

	for name, roots := range bundleRoots {
		for _, root := range roots {
			if strings.HasPrefix(spath, root) || strings.HasPrefix(root, spath) {
				return types.BadRequestErr(fmt.Sprintf("path %v is owned by bundle %q", spath, name))
			}
		}
	}

	return nil
}

func (s *Server) getDecisionLogger() (logger decisionLogger) {
	// For backwards compatibility use `revision` as needed.
	if s.hasLegacyBundle() {
		logger.revision = s.legacyRevision
	} else {
		logger.revisions = s.revisions
	}
	logger.logger = s.logger
	logger.buffer = s.buffer
	return logger
}

func (s *Server) getExplainResponse(explainMode types.ExplainModeV1, trace []*topdown.Event, pretty bool) (explanation types.TraceV1) {
	switch explainMode {
	case types.ExplainNotesV1:
		var err error
		explanation, err = types.NewTraceV1(lineage.Notes(trace), pretty)
		if err != nil {
			break
		}
	case types.ExplainFailsV1:
		var err error
		explanation, err = types.NewTraceV1(lineage.Fails(trace), pretty)
		if err != nil {
			break
		}
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

	opts = append(opts, rego.Transaction(txn), rego.Query(path), rego.ParsedInput(input), rego.Metrics(m), rego.Tracer(tracer), rego.Instrument(instrument), rego.Runtime(s.runtime), rego.UnsafeBuiltins(unsafeBuiltinsMap))
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
		impl.path, ok = parsePatchPathEscaped(path)
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

func (s *Server) getProvenance() *types.ProvenanceV1 {

	p := &types.ProvenanceV1{
		Version:   version.Version,
		Vcs:       version.Vcs,
		Timestamp: version.Timestamp,
		Hostname:  version.Hostname,
	}

	// For backwards compatibility, if the bundles are using the old
	// style config we need to fill in the older `Revision` field.
	// Otherwise use the newer `Bundles` keyword.
	if s.hasLegacyBundle() {
		p.Revision = s.legacyRevision
	} else {
		p.Bundles = map[string]types.ProvenanceBundleV1{}
		for name, revision := range s.revisions {
			p.Bundles[name] = types.ProvenanceBundleV1{Revision: revision}
		}
	}

	return p
}

func (s *Server) hasBundle() bool {
	return bundlePlugin.Lookup(s.manager) != nil || s.legacyRevision != ""
}

func (s *Server) hasLegacyBundle() bool {
	bp := bundlePlugin.Lookup(s.manager)
	return s.legacyRevision != "" || (bp != nil && !bp.Config().IsMultiBundle())
}

// parsePatchPathEscaped returns a new path for the given escaped str.
// This is based on storage.ParsePathEscaped so will do URL unescaping of
// the provided str for backwards compatibility, but also handles the
// specific escape strings defined in RFC 6901 (JSON Pointer) because
// that's what's mandated by RFC 6902 (JSON Patch).
func parsePatchPathEscaped(str string) (path storage.Path, ok bool) {
	path, ok = storage.ParsePathEscaped(str)
	if !ok {
		return
	}
	for i := range path {
		// RFC 6902 section 4: "[The "path" member's] value is a string containing
		// a JSON-Pointer value [RFC6901] that references a location within the
		// target document (the "target location") where the operation is performed."
		//
		// RFC 6901 section 3: "Because the characters '~' (%x7E) and '/' (%x2F)
		// have special meanings in JSON Pointer, '~' needs to be encoded as '~0'
		// and '/' needs to be encoded as '~1' when these characters appear in a
		// reference token."

		// RFC 6901 section 4: "Evaluation of each reference token begins by
		// decoding any escaped character sequence.  This is performed by first
		// transforming any occurrence of the sequence '~1' to '/', and then
		// transforming any occurrence of the sequence '~0' to '~'.  By performing
		// the substitutions in this order, an implementation avoids the error of
		// turning '~01' first into '~1' and then into '/', which would be
		// incorrect (the string '~01' correctly becomes '~1' after transformation)."
		path[i] = strings.Replace(path[i], "~1", "/", -1)
		path[i] = strings.Replace(path[i], "~0", "~", -1)
	}
	return
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

func validateQuery(query string) (ast.Body, error) {

	var body ast.Body
	body, err := ast.ParseBody(query)
	if err != nil {
		return nil, err
	}
	return body, nil
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
		case string(types.ExplainNotesV1):
			return types.ExplainNotesV1
		case string(types.ExplainFullV1):
			return types.ExplainFullV1
		}
	}
	return zero
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

type decisionLogger struct {
	revisions map[string]string
	revision  string // Deprecated: Use `revisions` instead.
	logger    func(context.Context, *Info) error
	buffer    Buffer
}

func (l decisionLogger) Log(ctx context.Context, txn storage.Transaction, decisionID, remoteAddr, path string, query string, input *interface{}, results *interface{}, err error, m metrics.Metrics) error {

	bundles := map[string]BundleInfo{}
	for name, rev := range l.revisions {
		bundles[name] = BundleInfo{Revision: rev}
	}

	info := &Info{
		Txn:        txn,
		Revision:   l.revision,
		Bundles:    bundles,
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

	if l.logger != nil {
		if err := l.logger(ctx, info); err != nil {
			return errors.Wrap(err, "decision_logs")
		}
	}

	if l.buffer != nil {
		l.buffer.Push(info)
	}

	return nil
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
