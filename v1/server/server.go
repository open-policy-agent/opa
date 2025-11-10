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
	"errors"
	"fmt"
	"html/template"
	"io"
	"net"
	"net/http"
	"net/http/pprof"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"github.com/open-policy-agent/opa/internal/json/patch"
	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/bundle"
	"github.com/open-policy-agent/opa/v1/hooks"
	"github.com/open-policy-agent/opa/v1/logging"
	"github.com/open-policy-agent/opa/v1/metrics"
	"github.com/open-policy-agent/opa/v1/plugins"
	bundlePlugin "github.com/open-policy-agent/opa/v1/plugins/bundle"
	serverDecodingPlugin "github.com/open-policy-agent/opa/v1/plugins/server/decoding"
	serverEncodingPlugin "github.com/open-policy-agent/opa/v1/plugins/server/encoding"
	"github.com/open-policy-agent/opa/v1/plugins/status"
	"github.com/open-policy-agent/opa/v1/rego"
	"github.com/open-policy-agent/opa/v1/server/authorizer"
	"github.com/open-policy-agent/opa/v1/server/handlers"
	"github.com/open-policy-agent/opa/v1/server/identifier"
	"github.com/open-policy-agent/opa/v1/server/types"
	"github.com/open-policy-agent/opa/v1/server/writer"
	"github.com/open-policy-agent/opa/v1/storage"
	"github.com/open-policy-agent/opa/v1/topdown"
	"github.com/open-policy-agent/opa/v1/topdown/builtins"
	iCache "github.com/open-policy-agent/opa/v1/topdown/cache"
	"github.com/open-policy-agent/opa/v1/topdown/lineage"
	"github.com/open-policy-agent/opa/v1/tracing"
	"github.com/open-policy-agent/opa/v1/util"
	"github.com/open-policy-agent/opa/v1/version"
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

const (
	defaultMinTLSVersion = tls.VersionTLS12

	// Set of handlers for use in the "handler" dimension of the duration metric.
	PromHandlerV0Data     = "v0/data"
	PromHandlerV1Data     = "v1/data"
	PromHandlerV1Query    = "v1/query"
	PromHandlerV1Policies = "v1/policies"
	PromHandlerV1Compile  = "v1/compile"
	PromHandlerV1Config   = "v1/config"
	PromHandlerV1Status   = "v1/status"
	PromHandlerIndex      = "index"
	PromHandlerCatch      = "catchall"
	PromHandlerHealth     = "health"
	PromHandlerAPIAuthz   = "authz"

	pqMaxCacheSize = 100

	// OpenTelemetry attributes
	otelDecisionIDAttr = "opa.decision_id"
)

var (
	supportedTLSVersions       = []uint16{tls.VersionTLS10, tls.VersionTLS11, tls.VersionTLS12, tls.VersionTLS13}
	unsafeBuiltinsMap          = map[string]struct{}{ast.HTTPSend.Name: {}}
	intermediateResultsEnabled = os.Getenv("OPA_DECISIONS_INTERMEDIATE_RESULTS") != ""
)

type IntermediateResultsContextKey struct{}

// Server represents an instance of OPA running in server mode.
type Server struct {
	Handler           http.Handler
	DiagnosticHandler http.Handler

	router                      *http.ServeMux
	addrs                       []string
	diagAddrs                   []string
	h2cEnabled                  bool
	authentication              AuthenticationScheme
	authorization               AuthorizationScheme
	cert                        *tls.Certificate
	tlsConfigMtx                sync.RWMutex
	certFile                    string
	certFileHash                []byte
	certKeyFile                 string
	certKeyFileHash             []byte
	certRefresh                 time.Duration
	certPool                    *x509.CertPool
	certPoolFile                string
	certPoolFileHash            []byte
	minTLSVersion               uint16
	mtx                         sync.RWMutex
	partials                    map[string]rego.PartialResult
	preparedEvalQueries         *cache
	store                       storage.Store
	manager                     *plugins.Manager
	decisionIDFactory           func() string
	logger                      func(context.Context, *Info) error
	errLimit                    int
	pprofEnabled                bool
	runtime                     *ast.Term
	httpListeners               []httpListener
	metrics                     Metrics
	defaultDecisionPath         string
	interQueryBuiltinCache      iCache.InterQueryCache
	interQueryBuiltinValueCache iCache.InterQueryValueCache
	allPluginsOkOnce            bool
	distributedTracingOpts      tracing.Options
	ndbCacheEnabled             bool
	unixSocketPerm              *string
	cipherSuites                *[]uint16
	hooks                       hooks.Hooks

	compileUnknownsCache     *lru.Cache[string, []ast.Ref]
	compileMaskingRulesCache *lru.Cache[string, ast.Ref]
}

// Metrics defines the interface that the server requires for recording HTTP
// handler metrics.
type Metrics interface {
	RegisterEndpoints(registrar func(path, method string, handler http.Handler))
	InstrumentHandler(handler http.Handler, label string) http.Handler
}

// TLSConfig represents the TLS configuration for the server.
// This configuration is used to configure file watchers to reload each file as it
// changes on disk.
type TLSConfig struct {
	// CertFile is the path to the server's serving certificate file.
	CertFile string

	// KeyFile is the path to the server's key file, completing the key pair for the
	// CertFile certificate.
	KeyFile string

	// CertPoolFile is the path to the CA cert pool file. The contents of this file will be
	// reloaded when the file changes on disk and used in as trusted client CAs in the TLS config
	// for new connections to the server.
	CertPoolFile string
}

// Loop will contain all the calls from the server that we'll be listening on.
type Loop func() error

// New returns a new Server.
func New() *Server {
	s := Server{}
	s.compileUnknownsCache, _ = lru.New[string, []ast.Ref](unknownsCacheSize)
	s.compileMaskingRulesCache, _ = lru.New[string, ast.Ref](maskingRuleCacheSize)
	return &s
}

// Init initializes the server. This function MUST be called before starting any loops
// from s.Listeners().
func (s *Server) Init(ctx context.Context) (*Server, error) {
	s.initRouters(ctx)
	var err error
	s.hooks.Each(func(h hooks.Hook) {
		switch h := h.(type) {
		case hooks.InterQueryCacheHook:
			if e := h.OnInterQueryCache(ctx, s.interQueryBuiltinCache); e != nil {
				err = errors.Join(err, e)
			}
		case hooks.InterQueryValueCacheHook:
			if e := h.OnInterQueryValueCache(ctx, s.interQueryBuiltinValueCache); e != nil {
				err = errors.Join(err, e)
			}
		}
	})
	if err != nil {
		return nil, err
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

	s.partials = map[string]rego.PartialResult{}
	s.preparedEvalQueries = newCache(pqMaxCacheSize)
	s.defaultDecisionPath = s.generateDefaultDecisionPath()
	s.manager.RegisterNDCacheTrigger(s.updateNDCache)

	s.Handler = s.initHandlerAuthn(s.Handler)

	// compression handler
	s.Handler, err = s.initHandlerCompression(s.Handler)
	if err != nil {
		return nil, err
	}
	s.DiagnosticHandler = s.initHandlerAuthn(s.DiagnosticHandler)

	s.Handler, err = s.initHandlerDecodingLimits(s.Handler)
	if err != nil {
		return nil, err
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
	for range s.httpListeners {
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

// WithDiagnosticAddresses sets the listening addresses that the server will
// bind to and *only* serve read-only diagnostic API's.
func (s *Server) WithDiagnosticAddresses(addrs []string) *Server {
	s.diagAddrs = addrs
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

// WithCertificatePaths sets the server-side certificate and keyfile paths
// that the server will periodically check for changes, and reload if necessary.
func (s *Server) WithCertificatePaths(certFile, keyFile string, refresh time.Duration) *Server {
	s.certFile = certFile
	s.certKeyFile = keyFile
	s.certRefresh = refresh
	return s
}

// WithCertPool sets the server-side cert pool that the server will use.
func (s *Server) WithCertPool(pool *x509.CertPool) *Server {
	s.certPool = pool
	return s
}

// WithTLSConfig sets the TLS configuration used by the server.
func (s *Server) WithTLSConfig(tlsConfig *TLSConfig) *Server {
	s.certFile = tlsConfig.CertFile
	s.certKeyFile = tlsConfig.KeyFile
	s.certPoolFile = tlsConfig.CertPoolFile
	return s
}

// WithCertRefresh sets the period on which certs, keys and cert pools are reloaded from disk.
func (s *Server) WithCertRefresh(refresh time.Duration) *Server {
	s.certRefresh = refresh
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

// WithH2CEnabled sets whether h2c ("HTTP/2 cleartext") is enabled for the http listener
func (s *Server) WithH2CEnabled(enabled bool) *Server {
	s.h2cEnabled = enabled
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
func (s *Server) WithRouter(router *http.ServeMux) *Server {
	s.router = router
	return s
}

func (s *Server) WithMinTLSVersion(minTLSVersion uint16) *Server {
	if slices.Contains(supportedTLSVersions, minTLSVersion) {
		s.minTLSVersion = minTLSVersion
	} else {
		s.minTLSVersion = defaultMinTLSVersion
	}
	return s
}

// WithDistributedTracingOpts sets the options to be used by distributed tracing.
func (s *Server) WithDistributedTracingOpts(opts tracing.Options) *Server {
	s.distributedTracingOpts = opts
	return s
}

// WithHooks allows passing hooks to the server.
func (s *Server) WithHooks(hs hooks.Hooks) *Server {
	s.hooks = hs
	return s
}

// WithNDBCacheEnabled sets whether the ND builtins cache is to be used.
func (s *Server) WithNDBCacheEnabled(ndbCacheEnabled bool) *Server {
	s.ndbCacheEnabled = ndbCacheEnabled
	return s
}

// WithCipherSuites sets the list of enabled TLS 1.0–1.2 cipher suites.
func (s *Server) WithCipherSuites(cipherSuites *[]uint16) *Server {
	s.cipherSuites = cipherSuites
	return s
}

// WithUnixSocketPermission sets the permission for the Unix domain socket if used to listen for
// incoming connections. Applies to the sockets the server is listening on including diagnostic API's.
func (s *Server) WithUnixSocketPermission(unixSocketPerm *string) *Server {
	s.unixSocketPerm = unixSocketPerm
	return s
}

// Listeners returns functions that listen and serve connections.
func (s *Server) Listeners() ([]Loop, error) {
	loops := []Loop{}

	handlerBindings := map[httpListenerType]struct {
		addrs   []string
		handler http.Handler
	}{
		defaultListenerType:    {s.addrs, s.Handler},
		diagnosticListenerType: {s.diagAddrs, s.DiagnosticHandler},
	}

	for t, binding := range handlerBindings {
		for _, addr := range binding.addrs {
			l, listener, err := s.getListener(addr, binding.handler, t)
			if err != nil {
				return nil, err
			}
			s.httpListeners = append(s.httpListeners, listener)
			loops = append(loops, l...)
		}
	}

	return loops, nil
}

// Addrs returns a list of addresses that the server is listening on.
// If the server hasn't been started it will not return an address.
func (s *Server) Addrs() []string {
	return s.addrsForType(defaultListenerType)
}

// DiagnosticAddrs returns a list of addresses that the server is listening on
// for the read-only diagnostic API's (eg /health, /metrics, etc)
// If the server hasn't been started it will not return an address.
func (s *Server) DiagnosticAddrs() []string {
	return s.addrsForType(diagnosticListenerType)
}

func (s *Server) addrsForType(t httpListenerType) []string {
	var addrs []string
	for _, l := range s.httpListeners {
		a := l.Addr()
		if a != "" && l.Type() == t {
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
	err = tc.SetKeepAlive(true)
	if err != nil {
		return nil, err
	}
	err = tc.SetKeepAlivePeriod(3 * time.Minute)
	if err != nil {
		return nil, err
	}
	return tc, nil
}

type httpListenerType int

const (
	defaultListenerType httpListenerType = iota
	diagnosticListenerType
)

type httpListener interface {
	Addr() string
	ListenAndServe() error
	ListenAndServeTLS(certFile, keyFile string) error
	Shutdown(context.Context) error
	Type() httpListenerType
}

// baseHTTPListener is just a wrapper around http.Server
type baseHTTPListener struct {
	s       *http.Server
	l       net.Listener
	t       httpListenerType
	addr    string
	addrMtx sync.RWMutex
}

var _ httpListener = (*baseHTTPListener)(nil)

func newHTTPListener(srvr *http.Server, t httpListenerType) httpListener {
	return &baseHTTPListener{s: srvr, t: t}
}

func newHTTPUnixSocketListener(srvr *http.Server, l net.Listener, t httpListenerType) httpListener {
	return &baseHTTPListener{s: srvr, l: l, t: t}
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

	b.initAddr()

	return b.s.Serve(tcpKeepAliveListener{b.l.(*net.TCPListener)})
}

func (b *baseHTTPListener) initAddr() {
	b.addrMtx.Lock()
	if addr := b.l.(*net.TCPListener).Addr(); addr != nil {
		b.addr = addr.String()
	}
	b.addrMtx.Unlock()
}

func (b *baseHTTPListener) Addr() string {
	b.addrMtx.Lock()
	defer b.addrMtx.Unlock()
	return b.addr
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

	b.initAddr()

	defer b.l.Close()

	return b.s.ServeTLS(tcpKeepAliveListener{b.l.(*net.TCPListener)}, certFile, keyFile)
}

func (b *baseHTTPListener) Shutdown(ctx context.Context) error {
	return b.s.Shutdown(ctx)
}

func (b *baseHTTPListener) Type() httpListenerType {
	return b.t
}

func (s *Server) getListener(addr string, h http.Handler, t httpListenerType) ([]Loop, httpListener, error) {
	parsedURL, err := parseURL(addr, s.cert != nil)
	if err != nil {
		return nil, nil, err
	}

	var loops []Loop
	var loop Loop
	var listener httpListener
	switch parsedURL.Scheme {
	case "unix":
		loop, listener, err = s.getListenerForUNIXSocket(parsedURL, h, t)
		loops = []Loop{loop}
	case "http":
		loop, listener, err = s.getListenerForHTTPServer(parsedURL, h, t)
		loops = []Loop{loop}
	case "https":
		loop, listener, err = s.getListenerForHTTPSServer(parsedURL, h, t)
		logger := s.manager.Logger().WithFields(map[string]any{
			"cert-file":     s.certFile,
			"cert-key-file": s.certKeyFile,
		})

		// if a manual cert refresh period has been set, then use the polling behavior,
		// otherwise use the fsnotify default behavior
		if s.certRefresh > 0 {
			loops = []Loop{loop, s.certLoopPolling(logger)}
		} else if s.certFile != "" || s.certPoolFile != "" {
			loops = []Loop{loop, s.certLoopNotify(logger)}
		}
	default:
		err = fmt.Errorf("invalid url scheme %q", parsedURL.Scheme)
	}

	return loops, listener, err
}

func (s *Server) getListenerForHTTPServer(u *url.URL, h http.Handler, t httpListenerType) (Loop, httpListener, error) {
	if s.h2cEnabled {
		h2s := &http2.Server{}
		h = h2c.NewHandler(h, h2s)
	}
	h1s := http.Server{
		Addr:    u.Host,
		Handler: h,
	}

	l := newHTTPListener(&h1s, t)

	return l.ListenAndServe, l, nil
}

func (s *Server) getListenerForHTTPSServer(u *url.URL, h http.Handler, t httpListenerType) (Loop, httpListener, error) {
	if s.cert == nil {
		return nil, nil, errors.New("TLS certificate required but not supplied")
	}

	tlsConfig := tls.Config{
		GetCertificate: s.getCertificate,
		// GetConfigForClient is used to ensure that a fresh config is provided containing the latest cert pool.
		// This is not required, but appears to be how connect time updates config should be done:
		// https://github.com/golang/go/issues/16066#issuecomment-250606132
		GetConfigForClient: func(_ *tls.ClientHelloInfo) (*tls.Config, error) {
			s.tlsConfigMtx.Lock()
			defer s.tlsConfigMtx.Unlock()

			cfg := &tls.Config{
				GetCertificate: s.getCertificate,
				ClientCAs:      s.certPool,
			}

			if s.authentication == AuthenticationTLS {
				cfg.ClientAuth = tls.RequireAndVerifyClientCert
			}

			if s.minTLSVersion != 0 {
				cfg.MinVersion = s.minTLSVersion
			} else {
				cfg.MinVersion = defaultMinTLSVersion
			}

			if s.cipherSuites != nil {
				cfg.CipherSuites = *s.cipherSuites
			}

			return cfg, nil
		},
	}

	httpsServer := http.Server{
		Addr:      u.Host,
		Handler:   h,
		TLSConfig: &tlsConfig,
	}

	l := newHTTPListener(&httpsServer, t)

	httpsLoop := func() error { return l.ListenAndServeTLS("", "") }

	return httpsLoop, l, nil
}

func (s *Server) getListenerForUNIXSocket(u *url.URL, h http.Handler, t httpListenerType) (Loop, httpListener, error) {
	socketPath := u.Host + u.Path

	// Recover @ prefix for abstract Unix sockets.
	if strings.HasPrefix(u.String(), u.Scheme+"://@") {
		socketPath = "@" + socketPath
	} else {
		// Remove domain socket file in case it already exists.
		os.Remove(socketPath)
	}

	domainSocketServer := http.Server{Handler: h}
	unixListener, err := net.Listen("unix", socketPath)
	if err != nil {
		return nil, nil, err
	}

	if s.unixSocketPerm != nil {
		modeVal, err := strconv.ParseUint(*s.unixSocketPerm, 8, 32)
		if err != nil {
			return nil, nil, err
		}

		if err := os.Chmod(socketPath, os.FileMode(modeVal)); err != nil {
			return nil, nil, err
		}
	}

	l := newHTTPUnixSocketListener(&domainSocketServer, unixListener, t)

	domainSocketLoop := func() error { return domainSocketServer.Serve(unixListener) }
	return domainSocketLoop, l, nil
}

func (s *Server) initHandlerAuthn(handler http.Handler) http.Handler {
	switch s.authentication {
	case AuthenticationToken:
		handler = identifier.NewTokenBased(handler)
	case AuthenticationTLS:
		handler = identifier.NewTLSBased(handler)
	}

	return handler
}

func (s *Server) initHandlerAuthz(handler http.Handler) http.Handler {
	switch s.authorization {
	case AuthorizationBasic:
		handler = authorizer.NewBasic(
			handler,
			s.getCompiler,
			s.store,
			authorizer.Runtime(s.runtime),
			authorizer.Decision(s.manager.GetConfig().DefaultAuthorizationDecisionRef),
			authorizer.PrintHook(s.manager.PrintHook()),
			authorizer.EnablePrintStatements(s.manager.EnablePrintStatements()),
			authorizer.InterQueryCache(s.interQueryBuiltinCache),
			authorizer.InterQueryValueCache(s.interQueryBuiltinValueCache),
			authorizer.URLPathExpectsBodyFunc(s.manager.ExtraAuthorizerRoutes()))

		if s.metrics != nil {
			handler = s.instrumentHandler(handler.ServeHTTP, PromHandlerAPIAuthz)
		}
	}

	return handler
}

// Enforces request body size limits on incoming requests. For gzipped requests,
// it passes the size limit down the body-reading method via the request
// context.
func (s *Server) initHandlerDecodingLimits(handler http.Handler) (http.Handler, error) {
	cfg := s.manager.GetConfig()
	var decodingRawConfig []byte
	if cfg.Server != nil {
		decodingRawConfig = []byte(cfg.Server.Decoding)
	}
	decodingConfig, err := serverDecodingPlugin.NewConfigBuilder().WithBytes(decodingRawConfig).Parse()
	if err != nil {
		return nil, err
	}
	decodingHandler := handlers.DecodingLimitsHandler(handler, *decodingConfig.MaxLength, *decodingConfig.Gzip.MaxLength)

	return decodingHandler, nil
}

func (s *Server) initHandlerCompression(handler http.Handler) (http.Handler, error) {
	cfg := s.manager.GetConfig()
	var encodingRawConfig []byte
	if cfg.Server != nil {
		encodingRawConfig = []byte(cfg.Server.Encoding)
	}
	encodingConfig, err := serverEncodingPlugin.NewConfigBuilder().WithBytes(encodingRawConfig).Parse()
	if err != nil {
		return nil, err
	}
	compressHandler := handlers.CompressHandler(handler, *encodingConfig.Gzip.MinLength, *encodingConfig.Gzip.CompressionLevel)

	return compressHandler, nil
}

func (s *Server) initRouters(ctx context.Context) {
	mainRouter := s.router
	if mainRouter == nil {
		mainRouter = http.NewServeMux()
	}

	diagRouter := http.NewServeMux()

	// authorizer, if configured, needs the iCache to be set up already

	cacheConfig := s.manager.InterQueryBuiltinCacheConfig()

	s.interQueryBuiltinCache = iCache.NewInterQueryCacheWithContext(ctx, cacheConfig)
	s.interQueryBuiltinValueCache = iCache.NewInterQueryValueCache(ctx, cacheConfig)

	s.manager.RegisterCacheTrigger(s.updateCacheConfig)

	// Add authorization handler. This must come BEFORE authentication handler
	// so that the latter can run first.
	handlerAuthz := s.initHandlerAuthz(mainRouter)

	handlerAuthzDiag := s.initHandlerAuthz(diagRouter)

	// All routers get the same base configuration *and* diagnostic API's
	for _, router := range []*http.ServeMux{mainRouter, diagRouter} {
		if s.metrics != nil {
			s.metrics.RegisterEndpoints(func(path, method string, handler http.Handler) {
				router.Handle(fmt.Sprintf("%s %s", method, path), handler)
			})
		}

		router.Handle("GET /health", s.instrumentHandler(s.unversionedGetHealth, PromHandlerHealth))
		// Use this route to evaluate health policy defined at system.health
		// By convention, policy is typically defined at system.health.live and system.health.ready, and is
		// evaluated by calling /health/live and /health/ready respectively.
		router.Handle("GET /health/{path...}", s.instrumentHandler(s.unversionedGetHealthWithPolicy, PromHandlerHealth))
	}

	for p, r := range s.manager.ExtraRoutes() {
		mainRouter.Handle(p, s.instrumentHandler(r.HandlerFunc, r.PromName))
	}

	if s.pprofEnabled {
		mainRouter.HandleFunc("GET /debug/pprof/", pprof.Index)
		mainRouter.Handle("GET /debug/pprof/allocs", pprof.Handler("allocs"))
		mainRouter.Handle("GET /debug/pprof/block", pprof.Handler("block"))
		mainRouter.Handle("GET /debug/pprof/heap", pprof.Handler("heap"))
		mainRouter.Handle("GET /debug/pprof/mutex", pprof.Handler("mutex"))
		mainRouter.HandleFunc("GET /debug/pprof/cmdline", pprof.Cmdline)
		mainRouter.HandleFunc("GET /debug/pprof/profile", pprof.Profile)
		mainRouter.HandleFunc("GET /debug/pprof/symbol", pprof.Symbol)
		mainRouter.HandleFunc("GET /debug/pprof/trace", pprof.Trace)
	}

	// Only the main mainRouter gets the OPA API's (data, policies, query, etc)
	mainRouter.Handle("POST /v0/data/{path...}", s.instrumentHandler(s.v0DataPost, PromHandlerV0Data))
	mainRouter.Handle("POST /v0/data", s.instrumentHandler(s.v0DataPost, PromHandlerV0Data))
	mainRouter.Handle("DELETE /v1/data/{path...}", s.instrumentHandler(s.v1DataDelete, PromHandlerV1Data))
	mainRouter.Handle("PUT /v1/data/{path...}", s.instrumentHandler(s.v1DataPut, PromHandlerV1Data))
	mainRouter.Handle("PUT /v1/data", s.instrumentHandler(s.v1DataPut, PromHandlerV1Data))
	mainRouter.Handle("GET /v1/data/{path...}", s.instrumentHandler(s.v1DataGet, PromHandlerV1Data))
	mainRouter.Handle("GET /v1/data", s.instrumentHandler(s.v1DataGet, PromHandlerV1Data))
	mainRouter.Handle("PATCH /v1/data/{path...}", s.instrumentHandler(s.v1DataPatch, PromHandlerV1Data))
	mainRouter.Handle("PATCH /v1/data", s.instrumentHandler(s.v1DataPatch, PromHandlerV1Data))
	mainRouter.Handle("POST /v1/data/{path...}", s.instrumentHandler(s.v1DataPost, PromHandlerV1Data))
	mainRouter.Handle("POST /v1/data", s.instrumentHandler(s.v1DataPost, PromHandlerV1Data))
	mainRouter.Handle("GET /v1/policies", s.instrumentHandler(s.v1PoliciesList, PromHandlerV1Policies))
	mainRouter.Handle("DELETE /v1/policies/{path...}", s.instrumentHandler(s.v1PoliciesDelete, PromHandlerV1Policies))
	mainRouter.Handle("GET /v1/policies/{path...}", s.instrumentHandler(s.v1PoliciesGet, PromHandlerV1Policies))
	mainRouter.Handle("PUT /v1/policies/{path...}", s.instrumentHandler(s.v1PoliciesPut, PromHandlerV1Policies))
	mainRouter.Handle("GET /v1/query", s.instrumentHandler(s.v1QueryGet, PromHandlerV1Query))
	mainRouter.Handle("POST /v1/query", s.instrumentHandler(s.v1QueryPost, PromHandlerV1Query))
	mainRouter.Handle("POST /v1/compile", s.instrumentHandler(s.v1CompilePost, PromHandlerV1Compile))
	mainRouter.Handle("POST /v1/compile/{path...}", s.instrumentHandler(s.v1CompileFilters, PromHandlerV1Compile))
	mainRouter.Handle("GET /v1/compile/{path...}", s.instrumentHandler(s.v1CompileFilters, PromHandlerV1Compile))
	mainRouter.Handle("GET /v1/config", s.instrumentHandler(s.v1ConfigGet, PromHandlerV1Config))
	mainRouter.Handle("GET /v1/status", s.instrumentHandler(s.v1StatusGet, PromHandlerV1Status))
	mainRouter.Handle("POST /{$}", s.instrumentHandler(s.unversionedPost, PromHandlerIndex))
	mainRouter.Handle("GET /{$}", s.instrumentHandler(s.indexGet, PromHandlerIndex))

	// These are catch all handlers that respond http.StatusMethodNotAllowed for resources that exist but the method is not allowed
	mainRouter.Handle("/v0/data/{path...}", s.methodNotAllowedHandler())
	mainRouter.Handle("/v0/data", s.methodNotAllowedHandler())
	mainRouter.Handle("/v1/data/{path...}", s.methodNotAllowedHandler())
	mainRouter.Handle("/v1/data", s.methodNotAllowedHandler())
	mainRouter.Handle("/v1/policies", s.methodNotAllowedHandler())
	mainRouter.Handle("/v1/policies/{path...}", s.methodNotAllowedHandler())
	mainRouter.Handle("/v1/query/{path...}", s.methodNotAllowedHandler())
	mainRouter.Handle("/v1/query", s.methodNotAllowedHandler())

	// Add authorization handler in the end so that it can run first
	s.Handler = handlerAuthz
	s.DiagnosticHandler = handlerAuthzDiag
}

func createMiddleware(mw ...func(http.Handler) http.Handler) func(http.Handler) http.Handler {
	return func(hnd http.Handler) http.Handler {
		next := hnd
		for k := len(mw) - 1; k >= 0; k-- {
			next = mw[k](next)
		}
		return next
	}
}

func (s *Server) instrumentHandler(handler func(http.ResponseWriter, *http.Request), label string) http.Handler {
	httpHandler := handlers.DefaultHandler(createMiddleware(
		s.manager.ExtraMiddlewares()...,
	)(http.HandlerFunc(handler)))
	if len(s.distributedTracingOpts) > 0 {
		httpHandler = tracing.NewHandler(httpHandler, label, s.distributedTracingOpts)
	}
	if s.metrics != nil {
		return s.metrics.InstrumentHandler(httpHandler, label)
	}
	return httpHandler
}

func (s *Server) methodNotAllowedHandler() http.Handler {
	return s.instrumentHandler(writer.HTTPStatus(http.StatusMethodNotAllowed), PromHandlerCatch)
}

func (s *Server) execQuery(ctx context.Context, br bundleRevisions, txn storage.Transaction, parsedQuery ast.Body, input ast.Value, rawInput *any, m metrics.Metrics, explainMode types.ExplainModeV1, includeMetrics, includeInstrumentation, pretty bool) (*types.QueryResponseV1, error) {
	results := types.QueryResponseV1{}
	ctx, logger := s.getDecisionLogger(ctx, br)

	var buf *topdown.BufferTracer
	if explainMode != types.ExplainOffV1 {
		buf = topdown.NewBufferTracer()
	}

	var ndbCache builtins.NDBCache
	if s.ndbCacheEnabled {
		ndbCache = builtins.NDBCache{}
	}

	opts := []func(*rego.Rego){
		rego.Store(s.store),
		rego.Transaction(txn),
		rego.Compiler(s.getCompiler()),
		rego.ParsedQuery(parsedQuery),
		rego.ParsedInput(input),
		rego.Metrics(m),
		rego.Instrument(includeInstrumentation),
		rego.QueryTracer(buf),
		rego.Runtime(s.runtime),
		rego.UnsafeBuiltins(unsafeBuiltinsMap),
		rego.InterQueryBuiltinCache(s.interQueryBuiltinCache),
		rego.InterQueryBuiltinValueCache(s.interQueryBuiltinValueCache),
		rego.PrintHook(s.manager.PrintHook()),
		rego.EnablePrintStatements(s.manager.EnablePrintStatements()),
		rego.DistributedTracingOpts(s.distributedTracingOpts),
		rego.NDBuiltinCache(ndbCache),
	}

	for _, r := range s.manager.GetWasmResolvers() {
		for _, entrypoint := range r.Entrypoints() {
			opts = append(opts, rego.Resolver(entrypoint, r))
		}
	}

	rego := rego.New(opts...)

	output, err := rego.Eval(ctx)
	if err != nil {
		_ = logger.Log(ctx, txn, "", parsedQuery.String(), rawInput, input, nil, ndbCache, err, m, nil)
		return nil, err
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

	var x any = results.Result
	if err := logger.Log(ctx, txn, "", parsedQuery.String(), rawInput, input, &x, ndbCache, nil, m, nil); err != nil {
		return nil, err
	}
	return &results, nil
}

func (*Server) indexGet(w http.ResponseWriter, _ *http.Request) {
	_ = indexHTML.Execute(w, struct {
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
}

type bundleRevisions struct {
	LegacyRevision string
	Revisions      map[string]string
}

func getRevisions(ctx context.Context, store storage.Store, txn storage.Transaction) (bundleRevisions, error) {
	var err error
	var br bundleRevisions
	br.Revisions = map[string]string{}

	// Check if we still have a legacy bundle manifest in the store
	br.LegacyRevision, err = bundle.LegacyReadRevisionFromStore(ctx, store, txn)
	if err != nil && !storage.IsNotFound(err) {
		return br, err
	}

	// read all bundle revisions from storage (if any exist)
	names, err := bundle.ReadBundleNamesFromStore(ctx, store, txn)
	if err != nil && !storage.IsNotFound(err) {
		return br, err
	}

	for _, name := range names {
		r, err := bundle.ReadBundleRevisionFromStore(ctx, store, txn, name)
		if err != nil && !storage.IsNotFound(err) {
			return br, err
		}
		br.Revisions[name] = r
	}

	return br, nil
}

func (s *Server) reload(_ context.Context, _ storage.Transaction, evt storage.TriggerEvent) {
	// NOTE(tsandall): We currently rely on the storage txn to provide
	// critical sections in the server.
	//
	// If you modify this function to change any other state on the server, you must
	// review the other places in the server where that state is accessed to avoid data
	// races--the state must be accessed _after_ a txn has been opened.

	// reset some cached info
	s.partials = map[string]rego.PartialResult{}
	s.preparedEvalQueries = newCache(pqMaxCacheSize)
	s.defaultDecisionPath = s.generateDefaultDecisionPath()
	if evt.PolicyChanged() {
		s.compileUnknownsCache.Purge()
		s.compileMaskingRulesCache.Purge()
	}
}

func (s *Server) unversionedPost(w http.ResponseWriter, r *http.Request) {
	s.v0QueryPath(w, r, "", true)
}

func (s *Server) v0DataPost(w http.ResponseWriter, r *http.Request) {
	s.v0QueryPath(w, r, escapedPathValue(r, "path"), false)
}

func (s *Server) v0QueryPath(w http.ResponseWriter, r *http.Request, urlPath string, useDefaultDecisionPath bool) {
	m := metrics.New()
	m.Timer(metrics.ServerHandler).Start()

	decisionID := s.generateDecisionID()
	ctx := logging.WithDecisionID(r.Context(), decisionID)
	annotateSpan(ctx, decisionID)

	input, goInput, err := readInputV0(r)
	if err != nil {
		writer.ErrorString(w, http.StatusBadRequest, types.CodeInvalidParameter, fmt.Errorf("unexpected parse error for input: %w", err))
		return
	}

	// Prepare for query.
	txn, err := s.store.NewTransaction(ctx)
	if err != nil {
		writer.ErrorAuto(w, err)
		return
	}

	defer s.store.Abort(ctx, txn)

	br, err := getRevisions(ctx, s.store, txn)
	if err != nil {
		writer.ErrorAuto(w, err)
		return
	}

	if useDefaultDecisionPath {
		urlPath = s.generateDefaultDecisionPath()
	}

	ctx, logger := s.getDecisionLogger(ctx, br)

	var ndbCache builtins.NDBCache
	if s.ndbCacheEnabled {
		ndbCache = builtins.NDBCache{}
	}

	pqID := "v0QueryPath::" + urlPath
	preparedQuery, ok := s.getCachedPreparedEvalQuery(pqID, m)
	if !ok {
		opts := []func(*rego.Rego){
			rego.Compiler(s.getCompiler()),
			rego.Store(s.store),
		}

		// Set resolvers on the base Rego object to avoid having them get
		// re-initialized, and to propagate them to the prepared query.
		for _, r := range s.manager.GetWasmResolvers() {
			for _, entrypoint := range r.Entrypoints() {
				opts = append(opts, rego.Resolver(entrypoint, r))
			}
		}

		rego, err := s.makeRego(ctx, false, txn, input, urlPath, m, false, nil, opts)
		if err != nil {
			_ = logger.Log(ctx, txn, urlPath, "", goInput, input, nil, ndbCache, err, m, nil)
			writer.ErrorAuto(w, err)
			return
		}

		pq, err := rego.PrepareForEval(ctx)
		if err != nil {
			_ = logger.Log(ctx, txn, urlPath, "", goInput, input, nil, ndbCache, err, m, nil)
			writer.ErrorAuto(w, err)
			return
		}
		preparedQuery = &pq
		s.preparedEvalQueries.Insert(pqID, preparedQuery)
	}

	evalOpts := []rego.EvalOption{
		rego.EvalTransaction(txn),
		rego.EvalParsedInput(input),
		rego.EvalMetrics(m),
		rego.EvalInterQueryBuiltinCache(s.interQueryBuiltinCache),
		rego.EvalInterQueryBuiltinValueCache(s.interQueryBuiltinValueCache),
		rego.EvalNDBuiltinCache(ndbCache),
	}

	rs, err := preparedQuery.Eval(
		ctx,
		evalOpts...,
	)

	m.Timer(metrics.ServerHandler).Stop()

	// Handle results.
	if err != nil {
		_ = logger.Log(ctx, txn, urlPath, "", goInput, input, nil, ndbCache, err, m, nil)
		writer.ErrorAuto(w, err)
		return
	}

	if len(rs) == 0 {
		ref, err := stringPathToDataRef(urlPath)
		if err != nil {
			writer.Error(w, http.StatusBadRequest, types.NewErrorV1(types.CodeInvalidParameter, "invalid path: %v", err))
			return
		}

		messageType := types.MsgMissingError
		if len(s.getCompiler().GetRulesForVirtualDocument(ref)) > 0 {
			messageType = types.MsgFoundUndefinedError
		}
		errV1 := types.NewErrorV1(types.CodeUndefinedDocument, "%v: %v", messageType, ref)
		if err := logger.Log(ctx, txn, urlPath, "", goInput, input, nil, ndbCache, errV1, m, nil); err != nil {
			writer.ErrorAuto(w, err)
			return
		}

		writer.Error(w, http.StatusNotFound, errV1)
		return
	}
	err = logger.Log(ctx, txn, urlPath, "", goInput, input, &rs[0].Expressions[0].Value, ndbCache, nil, m, nil)
	if err != nil {
		writer.ErrorAuto(w, err)
		return
	}

	writer.JSONOK(w, rs[0].Expressions[0].Value, pretty(r))
}

func (s *Server) getCachedPreparedEvalQuery(key string, m metrics.Metrics) (*rego.PreparedEvalQuery, bool) {
	pq, ok := s.preparedEvalQueries.Get(key)
	counter := m.Counter(metrics.ServerQueryCacheHit) // Creates the counter on m if it doesn't exist, starts at 0
	if ok {
		counter.Incr() // Increment counter on hit
		return pq.(*rego.PreparedEvalQuery), true
	}
	return nil, false
}

func (s *Server) canEval(ctx context.Context) bool {
	// Create very simple query that binds a single variable.
	opts := []func(*rego.Rego){
		rego.Compiler(s.getCompiler()),
		rego.Store(s.store),
		rego.Query("x = 1"),
	}

	for _, r := range s.manager.GetWasmResolvers() {
		for _, ep := range r.Entrypoints() {
			opts = append(opts, rego.Resolver(ep, r))
		}
	}

	eval := rego.New(opts...)
	// Run evaluation.
	rs, err := eval.Eval(ctx)
	if err != nil {
		return false
	}

	v, ok := rs[0].Bindings["x"]
	if ok {
		jsonNumber, ok := v.(json.Number)
		if ok && jsonNumber.String() == "1" {
			return true
		}
	}
	return false
}

func (*Server) bundlesReady(pluginStatuses map[string]*plugins.Status) bool {
	// Look for a discovery plugin first, if it exists and isn't ready
	// then don't bother with the others.
	// Note: use "discovery" instead of `discovery.Name` to avoid import
	// cycle problems..
	dpStatus, ok := pluginStatuses["discovery"]
	if ok && dpStatus != nil && (dpStatus.State != plugins.StateOK) {
		return false
	}

	// The bundle plugin won't return "OK" until the first activation
	// of each configured bundle.
	bpStatus, ok := pluginStatuses[bundlePlugin.Name]
	if ok && bpStatus != nil && (bpStatus.State != plugins.StateOK) {
		return false
	}

	return true
}

func (s *Server) unversionedGetHealth(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	includeBundleStatus := getBoolParam(r.URL, types.ParamBundleActivationV1, true) ||
		getBoolParam(r.URL, types.ParamBundlesActivationV1, true)
	includePluginStatus := getBoolParam(r.URL, types.ParamPluginsV1, true)
	excludePlugin := getStringSliceParam(r.URL, types.ParamExcludePluginV1)
	excludePluginMap := map[string]struct{}{}
	for _, name := range excludePlugin {
		excludePluginMap[name] = struct{}{}
	}

	// Ensure the server can evaluate a simple query
	if !s.canEval(ctx) {
		writeHealthResponse(w, errors.New("unable to perform evaluation"))
		return
	}

	pluginStatuses := s.manager.PluginStatus()

	// Ensure that bundles (if configured, and requested to be included in the result)
	// have been activated successfully. This will include discovery bundles as well as
	// normal bundles that are configured.
	if includeBundleStatus && !s.bundlesReady(pluginStatuses) {
		// For backwards compatibility we don't return a payload with statuses for the bundle endpoint
		writeHealthResponse(w, errors.New("one or more bundles are not activated"))
		return
	}

	if includePluginStatus {
		// Ensure that all plugins (if requested to be included in the result) have an OK status.
		hasErr := false
		for name, status := range pluginStatuses {
			if _, exclude := excludePluginMap[name]; exclude {
				continue
			}
			if status != nil && status.State != plugins.StateOK {
				hasErr = true
				break
			}
		}
		if hasErr {
			writeHealthResponse(w, errors.New("one or more plugins are not up"))
			return
		}
	}
	writeHealthResponse(w, nil)
}

func (s *Server) unversionedGetHealthWithPolicy(w http.ResponseWriter, r *http.Request) {
	pluginStatus := s.manager.PluginStatus()
	pluginState := map[string]string{}

	// optimistically assume all plugins are ok
	allPluginsOk := true

	// build input document for health check query
	input := func() map[string]any {
		s.mtx.Lock()
		defer s.mtx.Unlock()

		// iterate over plugin status to extract state
		for name, status := range pluginStatus {
			if status != nil {
				pluginState[name] = string(status.State)
				// if all plugins have not been in OK state yet, then check to see if plugin state is OKx
				if !s.allPluginsOkOnce && status.State != plugins.StateOK {
					allPluginsOk = false
				}
			}
		}
		// once all plugins are OK, set the allPluginsOkOnce flag to true, indicating that all
		// plugins have achieved a "ready" state at least once on the server.
		if allPluginsOk {
			s.allPluginsOkOnce = true
		}

		return map[string]any{
			"plugin_state":  pluginState,
			"plugins_ready": s.allPluginsOkOnce,
		}
	}()

	healthDataPath := "/system/health/" + escapedPathValue(r, "path")

	healthDataPathQuery, err := stringPathToQuery(healthDataPath)
	if err != nil {
		writer.Error(w, http.StatusBadRequest, types.NewErrorV1(types.CodeInvalidParameter, "invalid path: %v", err))
		return
	}

	rego := rego.New(
		rego.ParsedQuery(healthDataPathQuery),
		rego.Compiler(s.getCompiler()),
		rego.Store(s.store),
		rego.Input(input),
		rego.Runtime(s.runtime),
		rego.PrintHook(s.manager.PrintHook()),
	)

	rs, err := rego.Eval(r.Context())
	if err != nil {
		writeHealthResponse(w, err)
		return
	}

	if len(rs) == 0 {
		writeHealthResponse(w, fmt.Errorf("health check (%v) was undefined", healthDataPathQuery))
		return
	}

	result, ok := rs[0].Expressions[0].Value.(bool)
	if ok && result {
		writeHealthResponse(w, nil)
		return
	}

	writeHealthResponse(w, fmt.Errorf("health check (%v) returned unexpected value", healthDataPathQuery))
}

func writeHealthResponse(w http.ResponseWriter, err error) {
	if err != nil {
		writer.JSON(w, http.StatusInternalServerError, types.HealthResponseV1{Error: err.Error()}, false)
		return
	}

	writer.JSONOK(w, types.HealthResponseV1{}, false)
}

func (s *Server) v1CompilePost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	explainMode := getExplain(r.URL, types.ExplainOffV1)
	includeInstrumentation := getBoolParam(r.URL, types.ParamInstrumentV1, true)

	m := metrics.New()
	m.Timer(metrics.ServerHandler).Start()
	m.Timer(metrics.RegoQueryParse).Start()

	// decompress the input if sent as zip
	body, err := util.ReadMaybeCompressedBody(r)
	if err != nil {
		writer.Error(w, http.StatusBadRequest, types.NewErrorV1(types.CodeInvalidParameter, "could not decompress the body"))
		return
	}

	request, reqErr := readInputCompilePostV1(body, s.manager.ParserOptions())
	if reqErr != nil {
		writer.Error(w, http.StatusBadRequest, reqErr)
		return
	}

	m.Timer(metrics.RegoQueryParse).Stop()

	c := storage.NewContext().WithMetrics(m)
	txn, err := s.store.NewTransaction(ctx, storage.TransactionParams{Context: c})
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
		rego.DisableInlining(request.Options.DisableInlining),
		rego.NondeterministicBuiltins(request.Options.NondeterminsiticBuiltins),
		rego.QueryTracer(buf),
		rego.Instrument(includeInstrumentation),
		rego.Metrics(m),
		rego.Runtime(s.runtime),
		rego.UnsafeBuiltins(unsafeBuiltinsMap),
		rego.InterQueryBuiltinCache(s.interQueryBuiltinCache),
		rego.InterQueryBuiltinValueCache(s.interQueryBuiltinValueCache),
		rego.PrintHook(s.manager.PrintHook()),
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

	m.Timer(metrics.ServerHandler).Stop()

	result := types.CompileResponseV1{}

	if includeMetrics(r) || includeInstrumentation {
		result.Metrics = m.All()
	}

	if explainMode != types.ExplainOffV1 {
		result.Explanation = s.getExplainResponse(explainMode, *buf, pretty(r))
	}

	var i any = types.PartialEvaluationResultV1{
		Queries: pq.Queries,
		Support: pq.Support,
	}

	result.Result = &i

	writer.JSONOK(w, result, pretty(r))
}

func (s *Server) v1DataGet(w http.ResponseWriter, r *http.Request) {
	m := metrics.New()

	m.Timer(metrics.ServerHandler).Start()

	decisionID := s.generateDecisionID()
	ctx := logging.WithDecisionID(r.Context(), decisionID)
	annotateSpan(ctx, decisionID)

	urlPath := escapedPathValue(r, "path")
	explainMode := getExplain(r.URL, types.ExplainOffV1)
	includeInstrumentation := getBoolParam(r.URL, types.ParamInstrumentV1, true)
	provenance := getBoolParam(r.URL, types.ParamProvenanceV1, true)
	strictBuiltinErrors := getBoolParam(r.URL, types.ParamStrictBuiltinErrors, true)

	m.Timer(metrics.RegoInputParse).Start()

	inputs := r.URL.Query()[types.ParamInputV1]

	var input ast.Value
	var goInput *any

	if len(inputs) > 0 {
		var err error
		input, goInput, err = readInputGetV1(inputs[len(inputs)-1])
		if err != nil {
			writer.ErrorString(w, http.StatusBadRequest, types.CodeInvalidParameter, err)
			return
		}
	}

	m.Timer(metrics.RegoInputParse).Stop()

	// Prepare for query.
	c := storage.NewContext().WithMetrics(m)
	txn, err := s.store.NewTransaction(ctx, storage.TransactionParams{Context: c})
	if err != nil {
		writer.ErrorAuto(w, err)
		return
	}
	defer s.store.Abort(ctx, txn)

	br, err := getRevisions(ctx, s.store, txn)
	if err != nil {
		writer.ErrorAuto(w, err)
		return
	}

	ctx, logger := s.getDecisionLogger(ctx, br)

	var ndbCache builtins.NDBCache
	if s.ndbCacheEnabled {
		ndbCache = builtins.NDBCache{}
	}

	var buf *topdown.BufferTracer

	if explainMode != types.ExplainOffV1 {
		buf = topdown.NewBufferTracer()
	}

	pqID := "v1DataGet::"
	if strictBuiltinErrors {
		pqID += "strict-builtin-errors::"
	}
	pqID += urlPath
	preparedQuery, ok := s.getCachedPreparedEvalQuery(pqID, m)
	if !ok {
		opts := []func(*rego.Rego){
			rego.Compiler(s.getCompiler()),
			rego.Store(s.store),
		}

		for _, r := range s.manager.GetWasmResolvers() {
			for _, entrypoint := range r.Entrypoints() {
				opts = append(opts, rego.Resolver(entrypoint, r))
			}
		}

		rego, err := s.makeRego(ctx, strictBuiltinErrors, txn, input, urlPath, m, includeInstrumentation, buf, opts)
		if err != nil {
			_ = logger.Log(ctx, txn, urlPath, "", goInput, input, nil, ndbCache, err, m, nil)
			writer.ErrorAuto(w, err)
			return
		}

		pq, err := rego.PrepareForEval(ctx)
		if err != nil {
			_ = logger.Log(ctx, txn, urlPath, "", goInput, input, nil, ndbCache, err, m, nil)
			writer.ErrorAuto(w, err)
			return
		}
		preparedQuery = &pq
		s.preparedEvalQueries.Insert(pqID, preparedQuery)
	}

	evalOpts := []rego.EvalOption{
		rego.EvalTransaction(txn),
		rego.EvalParsedInput(input),
		rego.EvalMetrics(m),
		rego.EvalQueryTracer(buf),
		rego.EvalInterQueryBuiltinCache(s.interQueryBuiltinCache),
		rego.EvalInterQueryBuiltinValueCache(s.interQueryBuiltinValueCache),
		rego.EvalInstrument(includeInstrumentation),
		rego.EvalNDBuiltinCache(ndbCache),
	}

	rs, err := preparedQuery.Eval(
		ctx,
		evalOpts...,
	)

	m.Timer(metrics.ServerHandler).Stop()

	// Handle results.
	if err != nil {
		_ = logger.Log(ctx, txn, urlPath, "", goInput, input, nil, ndbCache, err, m, nil)
		writer.ErrorAuto(w, err)
		return
	}

	result := types.DataResponseV1{
		DecisionID: decisionID,
	}

	if includeMetrics(r) || includeInstrumentation {
		result.Metrics = m.All()
	}

	if provenance {
		result.Provenance = s.getProvenance(br)
	}

	if len(rs) == 0 {
		if explainMode == types.ExplainFullV1 {
			result.Explanation, err = types.NewTraceV1(lineage.Full(*buf), pretty(r))
			if err != nil {
				writer.ErrorAuto(w, err)
				return
			}
		}

		if err := logger.Log(ctx, txn, urlPath, "", goInput, input, nil, ndbCache, nil, m, nil); err != nil {
			writer.ErrorAuto(w, err)
			return
		}
		writer.JSONOK(w, result, pretty(r))
		return
	}

	result.Result = &rs[0].Expressions[0].Value

	if explainMode != types.ExplainOffV1 {
		result.Explanation = s.getExplainResponse(explainMode, *buf, pretty(r))
	}

	if err := logger.Log(ctx, txn, urlPath, "", goInput, input, result.Result, ndbCache, nil, m, nil); err != nil {
		writer.ErrorAuto(w, err)
		return
	}
	writer.JSONOK(w, result, pretty(r))
}

func (s *Server) v1DataPatch(w http.ResponseWriter, r *http.Request) {
	m := metrics.New()
	m.Timer(metrics.ServerHandler).Start()

	ctx := r.Context()
	var ops []types.PatchV1

	m.Timer(metrics.RegoInputParse).Start()
	if err := util.NewJSONDecoder(r.Body).Decode(&ops); err != nil {
		writer.ErrorString(w, http.StatusBadRequest, types.CodeInvalidParameter, err)
		return
	}
	m.Timer(metrics.RegoInputParse).Stop()

	patches, err := s.prepareV1PatchSlice(escapedPathValue(r, "path"), ops)
	if err != nil {
		writer.ErrorAuto(w, err)
		return
	}

	params := storage.WriteParams
	params.Context = storage.NewContext().WithMetrics(m)
	txn, err := s.store.NewTransaction(ctx, params)
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
		return
	}

	m.Timer(metrics.ServerHandler).Stop()

	if includeMetrics(r) {
		result := types.DataResponseV1{
			Metrics: m.All(),
		}
		writer.JSONOK(w, result, false)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) v1DataPost(w http.ResponseWriter, r *http.Request) {
	m := s.getMetrics(r)
	m.Timer(metrics.ServerHandler).Start()

	decisionID := s.generateDecisionID()
	ctx := logging.WithDecisionID(r.Context(), decisionID)
	annotateSpan(ctx, decisionID)

	m.Timer(metrics.RegoInputParse).Start()

	input, goInput, err := readInputPostV1(r)
	if err != nil {
		writer.ErrorString(w, http.StatusBadRequest, types.CodeInvalidParameter, err)
		return
	}

	m.Timer(metrics.RegoInputParse).Stop()

	txn, err := s.store.NewTransaction(ctx, storage.TransactionParams{Context: storage.NewContext().WithMetrics(m)})
	if err != nil {
		writer.ErrorAuto(w, err)
		return
	}

	defer s.store.Abort(ctx, txn)

	provenance := getBoolParam(r.URL, types.ParamProvenanceV1, true)

	var logger decisionLogger
	var br bundleRevisions

	if s.logger != nil || provenance {
		br, err = getRevisions(ctx, s.store, txn)
		if err != nil {
			writer.ErrorAuto(w, err)
			return
		}
		if s.logger != nil {
			ctx, logger = s.getDecisionLogger(ctx, br)
		}
	}

	var buf *topdown.BufferTracer

	explainMode := getExplain(r.URL, types.ExplainOffV1)
	if explainMode != types.ExplainOffV1 {
		buf = topdown.NewBufferTracer()
	}

	var ndbCache builtins.NDBCache
	if s.ndbCacheEnabled {
		ndbCache = builtins.NDBCache{}
	}

	urlPath := escapedPathValue(r, "path")

	strictBuiltinErrors := getBoolParam(r.URL, types.ParamStrictBuiltinErrors, true)
	includeInstrumentation := getBoolParam(r.URL, types.ParamInstrumentV1, true)

	pqID := "v1DataPost::"
	if strictBuiltinErrors {
		pqID = "v1DataPost::strict-builtin-errors::"
	}
	pqID += urlPath
	preparedQuery, ok := s.getCachedPreparedEvalQuery(pqID, m)
	if !ok {
		opts := []func(*rego.Rego){
			rego.Compiler(s.getCompiler()),
			rego.Store(s.store),
		}

		// Set resolvers on the base Rego object to avoid having them get
		// re-initialized, and to propagate them to the prepared query.
		for _, r := range s.manager.GetWasmResolvers() {
			for _, entrypoint := range r.Entrypoints() {
				opts = append(opts, rego.Resolver(entrypoint, r))
			}
		}

		rego, err := s.makeRego(ctx, strictBuiltinErrors, txn, input, urlPath, m, includeInstrumentation, buf, opts)
		if err != nil {
			_ = logger.Log(ctx, txn, urlPath, "", goInput, input, nil, ndbCache, err, m, nil)
			writer.ErrorAuto(w, err)
			return
		}

		pq, err := rego.PrepareForEval(ctx)
		if err != nil {
			_ = logger.Log(ctx, txn, urlPath, "", goInput, input, nil, ndbCache, err, m, nil)
			writer.ErrorAuto(w, err)
			return
		}
		preparedQuery = &pq
		s.preparedEvalQueries.Insert(pqID, preparedQuery)
	}

	rs, err := preparedQuery.Eval(ctx,
		rego.EvalTransaction(txn),
		rego.EvalParsedInput(input),
		rego.EvalMetrics(m),
		rego.EvalQueryTracer(buf),
		rego.EvalInterQueryBuiltinCache(s.interQueryBuiltinCache),
		rego.EvalInterQueryBuiltinValueCache(s.interQueryBuiltinValueCache),
		rego.EvalInstrument(includeInstrumentation),
		rego.EvalNDBuiltinCache(ndbCache),
	)

	m.Timer(metrics.ServerHandler).Stop()

	// Handle results.
	if err != nil {
		_ = logger.Log(ctx, txn, urlPath, "", goInput, input, nil, ndbCache, err, m, nil)
		writer.ErrorAuto(w, err)
		return
	}

	result := types.DataResponseV1{
		DecisionID: decisionID,
	}

	if input == nil {
		result.Warning = types.NewWarning(types.CodeAPIUsageWarn, types.MsgInputKeyMissing)
	}

	includeMetrics := getBoolParam(r.URL, types.ParamMetricsV1, true)
	if includeMetrics || includeInstrumentation {
		result.Metrics = m.All()
	}

	if provenance {
		result.Provenance = s.getProvenance(br)
	}

	if len(rs) == 0 {
		if explainMode == types.ExplainFullV1 {
			if result.Explanation, err = types.NewTraceV1(lineage.Full(*buf), pretty(r)); err != nil {
				writer.ErrorAuto(w, err)
				return
			}
		}
		if err = logger.Log(ctx, txn, urlPath, "", goInput, input, nil, ndbCache, nil, m, nil); err != nil {
			writer.ErrorAuto(w, err)
			return
		}
		writer.JSONOK(w, result, pretty(r))
		return
	}

	result.Result = &rs[0].Expressions[0].Value

	if explainMode != types.ExplainOffV1 {
		result.Explanation = s.getExplainResponse(explainMode, *buf, pretty(r))
	}

	if err := logger.Log(ctx, txn, urlPath, "", goInput, input, result.Result, ndbCache, nil, m, nil); err != nil {
		writer.ErrorAuto(w, err)
		return
	}
	writer.JSONOK(w, result, pretty(r))
}

func escapedPathValue(r *http.Request, key string) string {
	pathValue := r.PathValue(key)
	escaped := r.URL.EscapedPath()
	if !strings.Contains(escaped, "%") {
		return pathValue
	}

	i := strings.Index(r.URL.Path, pathValue)
	if i == -1 || i > len(escaped) {
		return pathValue
	}

	return escaped[i:]
}

func (s *Server) v1DataPut(w http.ResponseWriter, r *http.Request) {
	m := metrics.New()
	m.Timer(metrics.ServerHandler).Start()

	ctx := r.Context()

	m.Timer(metrics.RegoInputParse).Start()
	var value any
	if err := util.NewJSONDecoder(r.Body).Decode(&value); err != nil {
		writer.ErrorString(w, http.StatusBadRequest, types.CodeInvalidParameter, err)
		return
	}
	m.Timer(metrics.RegoInputParse).Stop()

	pv := escapedPathValue(r, "path")

	path, ok := storage.ParsePathEscaped("/" + strings.Trim(pv, "/"))
	if !ok {
		writer.Error(w, http.StatusBadRequest, types.NewErrorV1(types.CodeInvalidParameter, "bad path: %v", pv))
		return
	}

	params := storage.WriteParams
	params.Context = storage.NewContext().WithMetrics(m)
	txn, err := s.store.NewTransaction(ctx, params)
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
		if len(path) > 0 {
			if err := storage.MakeDir(ctx, s.store, txn, path[:len(path)-1]); err != nil {
				s.abortAuto(ctx, txn, w, err)
				return
			}
		}
	} else if r.Header.Get("If-None-Match") == "*" {
		s.store.Abort(ctx, txn)
		w.WriteHeader(http.StatusNotModified)
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
		return
	}

	m.Timer(metrics.ServerHandler).Stop()

	if includeMetrics(r) {
		result := types.DataResponseV1{
			Metrics: m.All(),
		}
		writer.JSONOK(w, result, false)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) v1DataDelete(w http.ResponseWriter, r *http.Request) {
	m := metrics.New()
	m.Timer(metrics.ServerHandler).Start()

	ctx := r.Context()

	pv := escapedPathValue(r, "path")
	path, ok := storage.ParsePathEscaped("/" + strings.Trim(pv, "/"))
	if !ok {
		writer.Error(w, http.StatusBadRequest, types.NewErrorV1(types.CodeInvalidParameter, "bad path: %v", pv))
		return
	}

	params := storage.WriteParams
	params.Context = storage.NewContext().WithMetrics(m)
	txn, err := s.store.NewTransaction(ctx, params)
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
		return
	}

	m.Timer(metrics.ServerHandler).Stop()

	if includeMetrics(r) {
		result := types.DataResponseV1{
			Metrics: m.All(),
		}
		writer.JSONOK(w, result, false)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) v1PoliciesDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("path")

	m := metrics.New()
	params := storage.WriteParams
	params.Context = storage.NewContext().WithMetrics(m)
	txn, err := s.store.NewTransaction(r.Context(), params)
	if err != nil {
		writer.ErrorAuto(w, err)
		return
	}

	if err := s.checkPolicyIDScope(r.Context(), txn, id); err != nil {
		s.abortAuto(r.Context(), txn, w, err)
		return
	}

	modules, err := s.loadModules(r.Context(), txn)
	if err != nil {
		s.abortAuto(r.Context(), txn, w, err)
		return
	}

	delete(modules, id)

	c := ast.NewCompiler().SetErrorLimit(s.errLimit)

	m.Timer(metrics.RegoModuleCompile).Start()

	if c.Compile(modules); c.Failed() {
		s.abort(r.Context(), txn, func() {
			writer.Error(w, http.StatusBadRequest, types.NewErrorV1(types.CodeInvalidOperation, types.MsgCompileModuleError).WithASTErrors(c.Errors))
		})
		return
	}

	m.Timer(metrics.RegoModuleCompile).Stop()

	if err := s.store.DeletePolicy(r.Context(), txn, id); err != nil {
		s.abortAuto(r.Context(), txn, w, err)
		return
	}

	if err := s.store.Commit(r.Context(), txn); err != nil {
		writer.ErrorAuto(w, err)
		return
	}

	resp := types.PolicyDeleteResponseV1{}
	if includeMetrics(r) {
		resp.Metrics = m.All()
	}

	writer.JSONOK(w, resp, pretty(r))
}

func (s *Server) v1PoliciesGet(w http.ResponseWriter, r *http.Request) {
	txn, err := s.store.NewTransaction(r.Context())
	if err != nil {
		writer.ErrorAuto(w, err)
		return
	}

	defer s.store.Abort(r.Context(), txn)

	path := r.PathValue("path")

	bs, err := s.store.GetPolicy(r.Context(), txn, path)
	if err != nil {
		writer.ErrorAuto(w, err)
		return
	}

	resp := types.PolicyGetResponseV1{
		Result: types.PolicyV1{
			ID:  path,
			Raw: string(bs),
			AST: s.getCompiler().Modules[path],
		},
	}

	writer.JSONOK(w, resp, pretty(r))
}

func (s *Server) v1PoliciesList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	txn, err := s.store.NewTransaction(ctx)
	if err != nil {
		writer.ErrorAuto(w, err)
		return
	}

	defer s.store.Abort(ctx, txn)

	policies := []types.PolicyV1{}
	c := s.getCompiler()

	// Only return policies from the store, the compiler
	// may contain additional partially compiled modules.
	ids, err := s.store.ListPolicies(ctx, txn)
	if err != nil {
		writer.ErrorAuto(w, err)
		return
	}
	for _, id := range ids {
		bs, err := s.store.GetPolicy(ctx, txn, id)
		if err != nil {
			writer.ErrorAuto(w, err)
			return
		}
		policy := types.PolicyV1{
			ID:  id,
			Raw: string(bs),
			AST: c.Modules[id],
		}
		policies = append(policies, policy)
	}

	writer.JSONOK(w, types.PolicyListResponseV1{Result: policies}, pretty(r))
}

func (s *Server) v1PoliciesPut(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("path")

	includeMetrics := includeMetrics(r)
	m := metrics.New()

	m.Timer("server_read_bytes").Start()

	buf, err := io.ReadAll(r.Body)
	if err != nil {
		writer.ErrorString(w, http.StatusBadRequest, types.CodeInvalidParameter, err)
		return
	}

	m.Timer("server_read_bytes").Stop()

	params := storage.WriteParams
	params.Context = storage.NewContext().WithMetrics(m)
	txn, err := s.store.NewTransaction(ctx, params)
	if err != nil {
		writer.ErrorAuto(w, err)
		return
	}

	if err := s.checkPolicyIDScope(ctx, txn, id); err != nil && !storage.IsNotFound(err) {
		s.abortAuto(ctx, txn, w, err)
		return
	}

	if bs, err := s.store.GetPolicy(ctx, txn, id); err != nil {
		if !storage.IsNotFound(err) {
			s.abortAuto(ctx, txn, w, err)
			return
		}
	} else if bytes.Equal(buf, bs) {
		s.store.Abort(ctx, txn)
		resp := types.PolicyPutResponseV1{}
		if includeMetrics {
			resp.Metrics = m.All()
		}
		writer.JSONOK(w, resp, pretty(r))
		return
	}

	m.Timer(metrics.RegoModuleParse).Start()
	parsedMod, err := ast.ParseModuleWithOpts(id, string(buf), s.manager.ParserOptions())
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

	modules[id] = parsedMod

	c := ast.NewCompiler().
		SetErrorLimit(s.errLimit).
		WithPathConflictsCheck(storage.NonEmpty(ctx, s.store, txn)).
		WithEnablePrintStatements(s.manager.EnablePrintStatements())

	m.Timer(metrics.RegoModuleCompile).Start()

	if c.Compile(modules); c.Failed() {
		s.abort(ctx, txn, func() {
			writer.Error(w, http.StatusBadRequest, types.NewErrorV1(types.CodeInvalidParameter, types.MsgCompileModuleError).WithASTErrors(c.Errors))
		})
		return
	}

	m.Timer(metrics.RegoModuleCompile).Stop()

	if err := s.store.UpsertPolicy(ctx, txn, id, buf); err != nil {
		s.abortAuto(ctx, txn, w, err)
		return
	}

	if err := s.store.Commit(ctx, txn); err != nil {
		writer.ErrorAuto(w, err)
		return
	}

	resp := types.PolicyPutResponseV1{}

	if includeMetrics {
		resp.Metrics = m.All()
	}

	writer.JSONOK(w, resp, pretty(r))
}

func (s *Server) v1QueryGet(w http.ResponseWriter, r *http.Request) {
	m := metrics.New()

	decisionID := s.generateDecisionID()
	ctx := logging.WithDecisionID(r.Context(), decisionID)
	annotateSpan(ctx, decisionID)

	values := r.URL.Query()

	qStrs := values[types.ParamQueryV1]
	if len(qStrs) == 0 {
		writer.Error(w, http.StatusBadRequest, types.NewErrorV1(types.CodeInvalidParameter, "missing parameter 'q'"))
		return
	}
	qStr := qStrs[len(qStrs)-1]

	parsedQuery, err := validateQuery(qStr, s.manager.ParserOptions())
	if err != nil {
		switch err := err.(type) {
		case ast.Errors:
			writer.Error(w, http.StatusBadRequest, types.NewErrorV1(types.CodeInvalidParameter, types.MsgParseQueryError).WithASTErrors(err))
		default:
			writer.ErrorAuto(w, err)
		}
		return
	}

	explainMode := getExplain(r.URL, types.ExplainOffV1)
	includeInstrumentation := getBoolParam(r.URL, types.ParamInstrumentV1, true)

	params := storage.TransactionParams{Context: storage.NewContext().WithMetrics(m)}
	txn, err := s.store.NewTransaction(ctx, params)
	if err != nil {
		writer.ErrorAuto(w, err)
		return
	}

	defer s.store.Abort(ctx, txn)

	br, err := getRevisions(ctx, s.store, txn)
	if err != nil {
		writer.ErrorAuto(w, err)
		return
	}
	pretty := pretty(r)
	results, err := s.execQuery(ctx, br, txn, parsedQuery, nil, nil, m, explainMode, includeMetrics(r), includeInstrumentation, pretty)
	if err != nil {
		switch err := err.(type) {
		case ast.Errors:
			writer.Error(w, http.StatusBadRequest, types.NewErrorV1(types.CodeInvalidParameter, types.MsgCompileQueryError).WithASTErrors(err))
		default:
			writer.ErrorAuto(w, err)
		}
		return
	}

	writer.JSONOK(w, results, pretty)
}

func (s *Server) v1QueryPost(w http.ResponseWriter, r *http.Request) {
	m := metrics.New()
	m.Timer(metrics.ServerHandler).Start()

	decisionID := s.generateDecisionID()
	ctx := logging.WithDecisionID(r.Context(), decisionID)
	annotateSpan(ctx, decisionID)

	var request types.QueryRequestV1
	err := util.NewJSONDecoder(r.Body).Decode(&request)
	if err != nil {
		writer.Error(w, http.StatusBadRequest, types.NewErrorV1(types.CodeInvalidParameter, "error(s) occurred while decoding request: %v", err.Error()))
		return
	}
	qStr := request.Query
	parsedQuery, err := validateQuery(qStr, s.manager.ParserOptions())
	if err != nil {
		switch err := err.(type) {
		case ast.Errors:
			writer.Error(w, http.StatusBadRequest, types.NewErrorV1(types.CodeInvalidParameter, types.MsgParseQueryError).WithASTErrors(err))
		default:
			writer.ErrorAuto(w, err)
		}
		return
	}

	pretty := pretty(r)
	explainMode := getExplain(r.URL, types.ExplainOffV1)
	includeMetrics := includeMetrics(r)
	includeInstrumentation := getBoolParam(r.URL, types.ParamInstrumentV1, true)

	var input ast.Value

	if request.Input != nil {
		input, err = ast.InterfaceToValue(*request.Input)
		if err != nil {
			writer.ErrorAuto(w, err)
			return
		}
	}

	params := storage.TransactionParams{Context: storage.NewContext().WithMetrics(m)}
	txn, err := s.store.NewTransaction(ctx, params)
	if err != nil {
		writer.ErrorAuto(w, err)
		return
	}

	defer s.store.Abort(ctx, txn)

	br, err := getRevisions(ctx, s.store, txn)
	if err != nil {
		writer.ErrorAuto(w, err)
		return
	}

	results, err := s.execQuery(ctx, br, txn, parsedQuery, input, request.Input, m, explainMode, includeMetrics, includeInstrumentation, pretty)
	if err != nil {
		switch err := err.(type) {
		case ast.Errors:
			writer.Error(w, http.StatusBadRequest, types.NewErrorV1(types.CodeInvalidParameter, types.MsgCompileQueryError).WithASTErrors(err))
		default:
			writer.ErrorAuto(w, err)
		}
		return
	}

	m.Timer(metrics.ServerHandler).Stop()

	if includeMetrics || includeInstrumentation {
		results.Metrics = m.All()
	}

	writer.JSONOK(w, results, pretty)
}

func (s *Server) v1ConfigGet(w http.ResponseWriter, r *http.Request) {
	result, err := s.manager.GetConfig().ActiveConfig()
	if err != nil {
		writer.ErrorAuto(w, err)
		return
	}

	writer.JSONOK(w, types.ConfigResponseV1{Result: &result}, pretty(r))
}

func (s *Server) v1StatusGet(w http.ResponseWriter, r *http.Request) {
	p := status.Lookup(s.manager)
	if p == nil {
		writer.ErrorString(w, http.StatusInternalServerError, types.CodeInternal, errors.New("status plugin not enabled"))
		return
	}

	var st any = p.Snapshot()
	writer.JSONOK(w, types.StatusResponseV1{Result: &st}, pretty(r))
}

func (s *Server) checkPolicyIDScope(ctx context.Context, txn storage.Transaction, id string) error {
	bs, err := s.store.GetPolicy(ctx, txn, id)
	if err != nil {
		return err
	}

	module, err := ast.ParseModuleWithOpts(id, string(bs), s.manager.ParserOptions())
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

func (s *Server) getMetrics(r *http.Request) metrics.Metrics {
	metricsInQuery := getBoolParam(r.URL, types.ParamMetricsV1, true)
	instrumentationInQuery := getBoolParam(r.URL, types.ParamInstrumentV1, true)

	if s.logger == nil && !metricsInQuery && !instrumentationInQuery {
		return metrics.NoOp()
	}

	return metrics.New()
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

	if spath == "" && len(bundleRoots) > 0 {
		return types.BadRequestErr("can't write to document root with bundle roots configured")
	}

	spathParts := strings.Split(spath, "/")

	for name, roots := range bundleRoots {
		if roots == nil {
			return types.BadRequestErr(fmt.Sprintf("all paths owned by bundle %q", name))
		}
		for _, root := range roots {
			if root == "" {
				return types.BadRequestErr(fmt.Sprintf("all paths owned by bundle %q", name))
			}
			if isPathOwned(spathParts, strings.Split(root, "/")) {
				return types.BadRequestErr(fmt.Sprintf("path %v is owned by bundle %q", spath, name))
			}
		}
	}

	return nil
}

func (s *Server) getDecisionLogger(ctx context.Context, br bundleRevisions) (context.Context, decisionLogger) {
	var logger decisionLogger
	if intermediateResultsEnabled {
		ctx = context.WithValue(ctx, IntermediateResultsContextKey{}, make(map[string]any))
	}

	// For backwards compatibility use `revision` as needed.
	if s.hasLegacyBundle(br) {
		logger.revision = br.LegacyRevision
	} else {
		logger.revisions = br.Revisions
	}
	logger.logger = s.logger
	return ctx, logger
}

func (*Server) getExplainResponse(explainMode types.ExplainModeV1, trace []*topdown.Event, pretty bool) (explanation types.TraceV1) {
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
		explanation, err = types.NewTraceV1(lineage.Full(trace), pretty)
		if err != nil {
			break
		}
	case types.ExplainDebugV1:
		var err error
		explanation, err = types.NewTraceV1(lineage.Debug(trace), pretty)
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

		parsed, err := ast.ParseModuleWithOpts(id, string(bs), s.manager.ParserOptions())
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

func (s *Server) makeRego(_ context.Context,
	strictBuiltinErrors bool,
	txn storage.Transaction,
	input ast.Value,
	urlPath string,
	m metrics.Metrics,
	instrument bool,
	tracer topdown.QueryTracer,
	opts []func(*rego.Rego),
) (*rego.Rego, error) {
	query, err := stringPathToQuery(urlPath)
	if err != nil {
		return nil, types.NewErrorV1(types.CodeInvalidParameter, "invalid path: %v", err)
	}

	opts = append(
		opts,
		rego.Transaction(txn),
		rego.ParsedQuery(query),
		rego.ParsedInput(input),
		rego.Metrics(m),
		rego.QueryTracer(tracer),
		rego.Instrument(instrument),
		rego.Runtime(s.runtime),
		rego.UnsafeBuiltins(unsafeBuiltinsMap),
		rego.StrictBuiltinErrors(strictBuiltinErrors),
		rego.PrintHook(s.manager.PrintHook()),
		rego.DistributedTracingOpts(s.distributedTracingOpts),
	)

	return rego.New(opts...), nil
}

func stringPathToQuery(urlPath string) (ast.Body, error) {
	ref, err := stringPathToDataRef(urlPath)
	if err != nil {
		return nil, err
	}

	return parseRefQuery(ref.String())
}

// parseRefQuery parses a string into a query ast.Body.
// The resulting query must be comprised of a single ref, or an error will be returned.
func parseRefQuery(str string) (ast.Body, error) {
	query, err := ast.ParseBody(str)
	if err != nil {
		return nil, errors.New("failed to parse query")
	}

	// assert the query is exactly one statement
	if l := len(query); l == 0 {
		return nil, errors.New("no ref")
	} else if l > 1 {
		return nil, errors.New("complex query")
	}

	// assert the single statement is a lone ref
	expr := query[0]
	switch t := expr.Terms.(type) {
	case *ast.Term:
		switch t.Value.(type) {
		case ast.Ref:
			return query, nil
		}
	}

	return nil, errors.New("complex query")
}

func (*Server) prepareV1PatchSlice(root string, ops []types.PatchV1) (result []patchImpl, err error) {
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
		impl.path, ok = patch.ParsePatchPathEscaped(path)
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

func (s *Server) getProvenance(br bundleRevisions) *types.ProvenanceV1 {
	p := &types.ProvenanceV1{
		Version:   version.Version,
		Vcs:       version.Vcs,
		Timestamp: version.Timestamp,
		Hostname:  version.Hostname,
	}

	// For backwards compatibility, if the bundles are using the old
	// style config we need to fill in the older `Revision` field.
	// Otherwise use the newer `Bundles` keyword.
	if s.hasLegacyBundle(br) {
		p.Revision = br.LegacyRevision
	} else {
		p.Bundles = map[string]types.ProvenanceBundleV1{}
		for name, revision := range br.Revisions {
			p.Bundles[name] = types.ProvenanceBundleV1{Revision: revision}
		}
	}

	return p
}

func (s *Server) hasLegacyBundle(br bundleRevisions) bool {
	bp := bundlePlugin.Lookup(s.manager)
	return br.LegacyRevision != "" || (bp != nil && !bp.Config().IsMultiBundle())
}

func (s *Server) generateDefaultDecisionPath() string {
	// Assume the path is safe to transition back to a url
	p, _ := s.manager.GetConfig().DefaultDecisionRef().Ptr()
	return p
}

func isPathOwned(path, root []string) bool {
	for i := 0; i < len(path) && i < len(root); i++ {
		if path[i] != root[i] {
			return false
		}
	}
	return true
}

func (s *Server) updateCacheConfig(cacheConfig *iCache.Config) {
	s.interQueryBuiltinCache.UpdateConfig(cacheConfig)
	s.interQueryBuiltinValueCache.UpdateConfig(cacheConfig)
}

func (s *Server) updateNDCache(enabled bool) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	s.ndbCacheEnabled = enabled
}

func stringPathToDataRef(s string) (ast.Ref, error) {
	result := ast.Ref{ast.DefaultRootDocument}
	r, err := stringPathToRef(s)
	if err != nil {
		return nil, err
	}
	return append(result, r...), nil
}

func stringPathToRef(s string) (ast.Ref, error) {
	r := ast.Ref{}

	if len(s) == 0 {
		return r, nil
	}

	for x := range strings.SplitSeq(s, "/") {
		if x == "" {
			continue
		}

		if y, err := url.PathUnescape(x); err == nil {
			x = y
		}

		if strings.Contains(x, "\"") {
			return nil, fmt.Errorf("invalid ref term '%s'", x)
		}

		i, err := strconv.Atoi(x)
		if err != nil {
			r = append(r, ast.StringTerm(x))
		} else {
			r = append(r, ast.IntNumberTerm(i))
		}
	}
	return r, nil
}

func validateQuery(query string, opts ast.ParserOptions) (ast.Body, error) {
	return ast.ParseBodyWithOpts(query, opts)
}

func getBoolParam(url *url.URL, name string, ifEmpty bool) bool {
	if url.RawQuery == "" {
		return false
	}

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
		if strings.EqualFold(x, "true") {
			return true
		}
	}

	return false
}

func getStringSliceParam(url *url.URL, name string) []string {
	p, ok := url.Query()[name]
	if !ok {
		return nil
	}

	// Query params w/o values are represented as slice (of len 1) with an
	// empty string.
	if len(p) == 1 && p[0] == "" {
		return nil
	}

	return p
}

func getExplain(url *url.URL, zero types.ExplainModeV1) types.ExplainModeV1 {
	if url.RawQuery == "" {
		return zero
	}

	for _, x := range url.Query()[types.ParamExplainV1] {
		switch x {
		case string(types.ExplainNotesV1):
			return types.ExplainNotesV1
		case string(types.ExplainFailsV1):
			return types.ExplainFailsV1
		case string(types.ExplainFullV1):
			return types.ExplainFullV1
		case string(types.ExplainDebugV1):
			return types.ExplainDebugV1
		}
	}
	return zero
}

func readInputV0(r *http.Request) (ast.Value, *any, error) {
	parsed, ok := authorizer.GetBodyOnContext(r.Context())
	if ok {
		v, err := ast.InterfaceToValue(parsed)
		return v, &parsed, err
	}

	// decompress the input if sent as zip
	bodyBytes, err := util.ReadMaybeCompressedBody(r)
	if err != nil {
		return nil, nil, fmt.Errorf("could not decompress the body: %w", err)
	}

	var x any

	if strings.Contains(r.Header.Get("Content-Type"), "yaml") {
		if len(bodyBytes) > 0 {
			if err = util.Unmarshal(bodyBytes, &x); err != nil {
				return nil, nil, fmt.Errorf("body contains malformed input document: %w", err)
			}
		}
	} else {
		dec := util.NewJSONDecoder(bytes.NewBuffer(bodyBytes))
		if err := dec.Decode(&x); err != nil && err != io.EOF {
			return nil, nil, fmt.Errorf("body contains malformed input document: %w", err)
		}
	}

	v, err := ast.InterfaceToValue(x)
	return v, &x, err
}

func readInputGetV1(str string) (ast.Value, *any, error) {
	var input any
	if err := util.UnmarshalJSON([]byte(str), &input); err != nil {
		return nil, nil, fmt.Errorf("parameter contains malformed input document: %w", err)
	}
	v, err := ast.InterfaceToValue(input)
	return v, &input, err
}

func readInputPostV1(r *http.Request) (ast.Value, *any, error) {
	parsed, ok := authorizer.GetBodyOnContext(r.Context())
	if ok {
		if obj, ok := parsed.(map[string]any); ok {
			if input, ok := obj["input"]; ok {
				v, err := ast.InterfaceToValue(input)
				return v, &input, err
			}
		}
		return nil, nil, nil
	}

	var request types.DataRequestV1

	// decompress the input if sent as zip
	bodyBytes, err := util.ReadMaybeCompressedBody(r)
	if err != nil {
		return nil, nil, fmt.Errorf("could not decompress the body: %w", err)
	}

	ct := r.Header.Get("Content-Type")
	// There is no standard for yaml mime-type so we just look for
	// anything related
	if strings.Contains(ct, "yaml") {
		if len(bodyBytes) > 0 {
			if err = util.Unmarshal(bodyBytes, &request); err != nil {
				return nil, nil, fmt.Errorf("body contains malformed input document: %w", err)
			}
		}
	} else {
		dec := util.NewJSONDecoder(bytes.NewBuffer(bodyBytes))
		if err := dec.Decode(&request); err != nil && err != io.EOF {
			return nil, nil, fmt.Errorf("body contains malformed input document: %w", err)
		}
	}

	if request.Input == nil {
		return nil, nil, nil
	}

	v, err := ast.InterfaceToValue(*request.Input)
	return v, request.Input, err
}

type compileRequest struct {
	Query    ast.Body
	Input    ast.Value
	Unknowns []*ast.Term
	Options  compileRequestOptions
}

type compileRequestOptions struct {
	DisableInlining          []string
	NondeterminsiticBuiltins bool
}

func readInputCompilePostV1(reqBytes []byte, queryParserOptions ast.ParserOptions) (*compileRequest, *types.ErrorV1) {
	var request types.CompileRequestV1

	err := util.NewJSONDecoder(bytes.NewBuffer(reqBytes)).Decode(&request)
	if err != nil {
		return nil, types.NewErrorV1(types.CodeInvalidParameter, "error(s) occurred while decoding request: %v", err.Error())
	}

	query, err := ast.ParseBodyWithOpts(request.Query, queryParserOptions)
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

	return &compileRequest{
		Query:    query,
		Input:    input,
		Unknowns: unknowns,
		Options: compileRequestOptions{
			DisableInlining:          request.Options.DisableInlining,
			NondeterminsiticBuiltins: request.Options.NondeterministicBuiltins,
		},
	}, nil
}

var indexHTML, _ = template.New("index").Parse(`
<html>
<head>
<script type="text/javascript">
function query() {
	params = {
		'query': document.getElementById("query").value,
	}
	if (document.getElementById("input").value !== "") {
		try {
			params["input"] = JSON.parse(document.getElementById("input").value);
		} catch (e) {
			document.getElementById("result").innerHTML = e;
			return;
		}
	}
	body = JSON.stringify(params);
	opts = {
		'method': 'POST',
		'body': body,
	}
	fetch(new Request('v1/query', opts))
		.then(resp => resp.json())
		.then(json => {
			str = JSON.stringify(json, null, 2);
			document.getElementById("result").innerHTML = str;
		});
}
</script>
</head>
</body>
<pre>
 ________      ________    ________
|\   __  \    |\   __  \  |\   __  \
\ \  \|\  \   \ \  \|\  \ \ \  \|\  \
 \ \  \\\  \   \ \   ____\ \ \   __  \
  \ \  \\\  \   \ \  \___|  \ \  \ \  \
   \ \_______\   \ \__\      \ \__\ \__\
    \|_______|    \|__|       \|__|\|__|
</pre>
Open Policy Agent - An open source project to policy-enable your service.<br>
<br>
Version: {{ .Version }}<br>
Build Commit: {{ .BuildCommit }}<br>
Build Timestamp: {{ .BuildTimestamp }}<br>
Build Hostname: {{ .BuildHostname }}<br>
<br>
Query:<br>
<textarea rows="10" cols="50" id="query"></textarea><br>
<br>Input Data (JSON):<br>
<textarea rows="10" cols="50" id="input"></textarea><br>
<br><button onclick="query()">Submit</button>
<pre><div id="result"></div></pre>
</body>
</html>
`)

type decisionLogger struct {
	revisions map[string]string
	revision  string // Deprecated: Use `revisions` instead.
	logger    func(context.Context, *Info) error
}

func (l decisionLogger) Log(
	ctx context.Context,
	txn storage.Transaction,
	path string,
	query string,
	goInput *any,
	astInput ast.Value,
	goResults *any,
	ndbCache builtins.NDBCache,
	err error,
	m metrics.Metrics,
	custom map[string]any,
) error {
	if l.logger == nil {
		return nil
	}

	bundles := map[string]BundleInfo{}
	for name, rev := range l.revisions {
		bundles[name] = BundleInfo{Revision: rev}
	}

	rctx := logging.RequestContext{}
	if r, ok := logging.FromContext(ctx); ok {
		rctx = *r
	}
	decisionID, _ := logging.DecisionIDFromContext(ctx)

	var httpRctx logging.HTTPRequestContext

	httpRctxVal, _ := logging.HTTPRequestContextFromContext(ctx)
	if httpRctxVal != nil {
		httpRctx = *httpRctxVal
	}

	info := &Info{
		Txn:                txn,
		Revision:           l.revision,
		Bundles:            bundles,
		Timestamp:          time.Now().UTC(),
		DecisionID:         decisionID,
		RemoteAddr:         rctx.ClientAddr,
		HTTPRequestContext: httpRctx,
		Path:               path,
		Query:              query,
		Input:              goInput,
		InputAST:           astInput,
		Results:            goResults,
		Error:              err,
		Metrics:            m,
		RequestID:          rctx.ReqID,
		Custom:             custom,
	}

	if ndbCache != nil {
		x, err := ast.JSON(ndbCache.AsValue())
		if err != nil {
			return err
		}
		info.NDBuiltinCache = &x
	}

	sctx := trace.SpanFromContext(ctx).SpanContext()
	if sctx.IsValid() {
		info.TraceID = sctx.TraceID().String()
		info.SpanID = sctx.SpanID().String()
	}

	if intermediateResultsEnabled {
		if iresults, ok := ctx.Value(IntermediateResultsContextKey{}).(map[string]any); ok {
			info.IntermediateResults = iresults
		}
	}

	if l.logger != nil {
		if err := l.logger(ctx, info); err != nil {
			return fmt.Errorf("decision_logs: %w", err)
		}
	}

	return nil
}

type patchImpl struct {
	path  storage.Path
	op    storage.PatchOp
	value any
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

func annotateSpan(ctx context.Context, decisionID string) {
	if decisionID != "" {
		trace.SpanFromContext(ctx).SetAttributes(attribute.String(otelDecisionIDAttr, decisionID))
	}
}

func pretty(r *http.Request) bool {
	return getBoolParam(r.URL, types.ParamPrettyV1, true)
}

func includeMetrics(r *http.Request) bool {
	return getBoolParam(r.URL, types.ParamMetricsV1, true)
}
