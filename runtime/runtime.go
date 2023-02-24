// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package runtime

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	mr "math/rand"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/gorilla/mux"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/propagation"
	"go.uber.org/automaxprocs/maxprocs"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/bundle"
	"github.com/open-policy-agent/opa/internal/config"
	internal_tracing "github.com/open-policy-agent/opa/internal/distributedtracing"
	internal_logging "github.com/open-policy-agent/opa/internal/logging"
	"github.com/open-policy-agent/opa/internal/prometheus"
	"github.com/open-policy-agent/opa/internal/report"
	"github.com/open-policy-agent/opa/internal/runtime"
	initload "github.com/open-policy-agent/opa/internal/runtime/init"
	"github.com/open-policy-agent/opa/internal/uuid"
	"github.com/open-policy-agent/opa/loader"
	"github.com/open-policy-agent/opa/logging"
	"github.com/open-policy-agent/opa/metrics"
	"github.com/open-policy-agent/opa/plugins"
	"github.com/open-policy-agent/opa/plugins/discovery"
	"github.com/open-policy-agent/opa/plugins/logs"
	"github.com/open-policy-agent/opa/repl"
	"github.com/open-policy-agent/opa/server"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/storage/disk"
	"github.com/open-policy-agent/opa/storage/inmem"
	"github.com/open-policy-agent/opa/tracing"
	"github.com/open-policy-agent/opa/util"
	"github.com/open-policy-agent/opa/version"
)

var (
	registeredPlugins    map[string]plugins.Factory
	registeredPluginsMux sync.Mutex
)

const (
	// default interval between OPA version report uploads
	defaultUploadIntervalSec = int64(3600)
)

// RegisterPlugin registers a plugin factory with the runtime
// package. When the runtime is created, the factories are used to parse
// plugin configuration and instantiate plugins. If no configuration is
// provided, plugins are not instantiated. This function is idempotent.
func RegisterPlugin(name string, factory plugins.Factory) {
	registeredPluginsMux.Lock()
	defer registeredPluginsMux.Unlock()
	registeredPlugins[name] = factory
}

// Params stores the configuration for an OPA instance.
type Params struct {
	// Globally unique identifier for this OPA instance. If an ID is not specified,
	// the runtime will generate one.
	ID string

	// Addrs are the listening addresses that the OPA server will bind to.
	Addrs *[]string

	// DiagnosticAddrs are the listening addresses that the OPA server will bind to
	// for read-only diagnostic API's (/health, /metrics, etc)
	DiagnosticAddrs *[]string

	// H2CEnabled flag controls whether OPA will allow H2C (HTTP/2 cleartext) on
	// HTTP listeners.
	H2CEnabled bool

	// Authentication is the type of authentication scheme to use.
	Authentication server.AuthenticationScheme

	// Authorization is the type of authorization scheme to use.
	Authorization server.AuthorizationScheme

	// Certificate is the certificate to use in server-mode. If the certificate
	// is nil, the server will NOT use TLS.
	Certificate *tls.Certificate

	// CertificateFile and CertificateKeyFile are the paths to the cert and its
	// keyfile. It'll be used to periodically reload the files from disk if they
	// have changed. The server will attempt to refresh every 5 minutes, unless
	// a different CertificateRefresh time.Duration is provided
	CertificateFile    string
	CertificateKeyFile string
	CertificateRefresh time.Duration

	// CertPool holds the CA certs trusted by the OPA server.
	CertPool *x509.CertPool

	// MinVersion contains the minimum TLS version that is acceptable.
	// If zero, TLS 1.2 is currently taken as the minimum.
	MinTLSVersion uint16

	// HistoryPath is the filename to store the interactive shell user
	// input history.
	HistoryPath string

	// Output format controls how the REPL will print query results.
	// Default: "pretty".
	OutputFormat string

	// Paths contains filenames of base documents and policy modules to load on
	// startup. Data files may be prefixed with "<dotted-path>:" to indicate
	// where the contained document should be loaded.
	Paths []string

	// Optional filter that will be passed to the file loader.
	Filter loader.Filter

	// BundleMode will enable treating the Paths provided as bundles rather than
	// loading all data & policy files.
	BundleMode bool

	// Watch flag controls whether OPA will watch the Paths files for changes.
	// If this flag is true, OPA will watch the Paths files for changes and
	// reload the storage layer each time they change. This is useful for
	// interactive development.
	Watch bool

	// ErrorLimit is the number of errors the compiler will allow to occur before
	// exiting early.
	ErrorLimit int

	// PprofEnabled flag controls whether pprof endpoints are enabled
	PprofEnabled bool

	// DecisionIDFactory generates decision IDs to include in API responses
	// sent by the server (in response to Data API queries.)
	DecisionIDFactory func() string

	// Logging configures the logging behaviour.
	Logging LoggingConfig

	// Logger sets the logger implementation to use for debug logs.
	Logger logging.Logger

	// ConsoleLogger sets the logger implementation to use for console logs.
	ConsoleLogger logging.Logger

	// ConfigFile refers to the OPA configuration to load on startup.
	ConfigFile string

	// ConfigOverrides are overrides for the OPA configuration that are applied
	// over top the config file They are in a list of key=value syntax that
	// conform to the syntax defined in the `strval` package
	ConfigOverrides []string

	// ConfigOverrideFiles Similar to `ConfigOverrides` except they are in the
	// form of `key=path/to/file`where the file contains the value to be used.
	ConfigOverrideFiles []string

	// Output is the output stream used when run as an interactive shell. This
	// is mostly for test purposes.
	Output io.Writer

	// GracefulShutdownPeriod is the time (in seconds) to wait for the http
	// server to shutdown gracefully.
	GracefulShutdownPeriod int

	// ShutdownWaitPeriod is the time (in seconds) to wait before initiating shutdown.
	ShutdownWaitPeriod int

	// EnableVersionCheck flag controls whether OPA will report its version to an external service.
	// If this flag is true, OPA will report its version to the external service
	EnableVersionCheck bool

	// BundleVerificationConfig sets the key configuration used to verify a signed bundle
	BundleVerificationConfig *bundle.VerificationConfig

	// SkipBundleVerification flag controls whether OPA will verify a signed bundle
	SkipBundleVerification bool

	// ReadyTimeout flag controls if and for how long OPA server will wait (in seconds) for
	// configured bundles and plugins to be activated/ready before listening for traffic.
	// A value of 0 or less means no wait is exercised.
	ReadyTimeout int

	// Router is the router to which handlers for the REST API are added.
	// Router uses a first-matching-route-wins strategy, so no existing routes are overridden
	// If it is nil, a new mux.Router will be created
	Router *mux.Router

	// DiskStorage, if set, will make the runtime instantiate a disk-backed storage
	// implementation (instead of the default, in-memory store).
	// It can also be enabled via config, and this runtime field takes precedence.
	DiskStorage *disk.Options

	DistributedTracingOpts tracing.Options
}

// LoggingConfig stores the configuration for OPA's logging behaviour.
type LoggingConfig struct {
	Level           string
	Format          string
	TimestampFormat string
}

// NewParams returns a new Params object.
func NewParams() Params {
	return Params{
		Output:             os.Stdout,
		BundleMode:         false,
		EnableVersionCheck: false,
	}
}

// Runtime represents a single OPA instance.
type Runtime struct {
	Params  Params
	Store   storage.Store
	Manager *plugins.Manager

	logger        logging.Logger
	server        *server.Server
	metrics       *prometheus.Provider
	reporter      *report.Reporter
	traceExporter *otlptrace.Exporter

	serverInitialized bool
	serverInitMtx     sync.RWMutex
	done              chan struct{}
}

// NewRuntime returns a new Runtime object initialized with params. Clients must
// call StartServer() or StartREPL() to start the runtime in either mode.
func NewRuntime(ctx context.Context, params Params) (*Runtime, error) {
	if params.ID == "" {
		var err error
		params.ID, err = generateInstanceID()
		if err != nil {
			return nil, err
		}
	}

	level, err := internal_logging.GetLevel(params.Logging.Level)
	if err != nil {
		return nil, err
	}

	// NOTE(tsandall): This is a temporary hack to ensure that log formatting
	// and leveling is applied correctly. Currently there are a few places where
	// the global logger is used as a fallback, however, that fallback _should_
	// never be used. This ensures that _if_ the fallback is used accidentally,
	// that the logging configuration is applied. Once we remove all usage of
	// the global logger and we remove the API that allows callers to access the
	// global logger, we can remove this.
	logging.Get().SetFormatter(internal_logging.GetFormatter(params.Logging.Format, params.Logging.TimestampFormat))
	logging.Get().SetLevel(level)

	var logger logging.Logger

	if params.Logger != nil {
		logger = params.Logger
	} else {
		stdLogger := logging.New()
		stdLogger.SetLevel(level)
		stdLogger.SetFormatter(internal_logging.GetFormatter(params.Logging.Format, params.Logging.TimestampFormat))
		logger = stdLogger
	}

	var filePaths []string
	urlPathCount := 0
	for _, path := range params.Paths {
		if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
			urlPathCount++
			override, err := urlPathToConfigOverride(urlPathCount, path)
			if err != nil {
				return nil, err
			}
			params.ConfigOverrides = append(params.ConfigOverrides, override...)
		} else {
			filePaths = append(filePaths, path)
		}
	}
	params.Paths = filePaths

	config, err := config.Load(params.ConfigFile, params.ConfigOverrides, params.ConfigOverrideFiles)
	if err != nil {
		return nil, fmt.Errorf("config error: %w", err)
	}

	var reporter *report.Reporter
	if params.EnableVersionCheck {
		var err error
		reporter, err = report.New(params.ID, report.Options{Logger: logger})
		if err != nil {
			return nil, fmt.Errorf("config error: %w", err)
		}
	}

	loaded, err := initload.LoadPaths(params.Paths, params.Filter, params.BundleMode, params.BundleVerificationConfig, params.SkipBundleVerification, false, nil)
	if err != nil {
		return nil, fmt.Errorf("load error: %w", err)
	}

	info, err := runtime.Term(runtime.Params{Config: config})
	if err != nil {
		return nil, err
	}

	consoleLogger := params.ConsoleLogger
	if consoleLogger == nil {
		l := logging.New()
		l.SetFormatter(internal_logging.GetFormatter(params.Logging.Format, params.Logging.TimestampFormat))
		consoleLogger = l
	}

	if params.Router == nil {
		params.Router = mux.NewRouter()
	}

	metrics := prometheus.New(metrics.New(), errorLogger(logger))

	var store storage.Store
	if params.DiskStorage == nil {
		params.DiskStorage, err = disk.OptionsFromConfig(config, params.ID)
		if err != nil {
			return nil, fmt.Errorf("parse disk store configuration: %w", err)
		}
	}

	if params.DiskStorage != nil {
		store, err = disk.New(ctx, logger, metrics, *params.DiskStorage)
		if err != nil {
			return nil, fmt.Errorf("initialize disk store: %w", err)
		}
	} else {
		store = inmem.NewWithOpts(inmem.OptRoundTripOnWrite(false))
	}

	traceExporter, tracerProvider, err := internal_tracing.Init(ctx, config, params.ID)
	if err != nil {
		return nil, fmt.Errorf("config error: %w", err)
	}
	if tracerProvider != nil {
		params.DistributedTracingOpts = tracing.NewOptions(
			otelhttp.WithTracerProvider(tracerProvider),
			otelhttp.WithPropagators(propagation.TraceContext{}),
		)
	}

	manager, err := plugins.New(config,
		params.ID,
		store,
		plugins.Info(info),
		plugins.InitBundles(loaded.Bundles),
		plugins.InitFiles(loaded.Files),
		plugins.MaxErrors(params.ErrorLimit),
		plugins.GracefulShutdownPeriod(params.GracefulShutdownPeriod),
		plugins.ConsoleLogger(consoleLogger),
		plugins.Logger(logger),
		plugins.EnablePrintStatements(logger.GetLevel() >= logging.Info),
		plugins.PrintHook(loggingPrintHook{logger: logger}),
		plugins.WithRouter(params.Router),
		plugins.WithPrometheusRegister(metrics),
		plugins.WithTracerProvider(tracerProvider))
	if err != nil {
		return nil, fmt.Errorf("config error: %w", err)
	}

	if err := manager.Init(ctx); err != nil {
		return nil, fmt.Errorf("initialization error: %w", err)
	}

	disco, err := discovery.New(manager, discovery.Factories(registeredPlugins), discovery.Metrics(metrics))
	if err != nil {
		return nil, fmt.Errorf("config error: %w", err)
	}

	manager.Register(discovery.Name, disco)

	rt := &Runtime{
		Store:             manager.Store,
		Params:            params,
		Manager:           manager,
		logger:            logger,
		metrics:           metrics,
		reporter:          reporter,
		serverInitialized: false,
		traceExporter:     traceExporter,
	}

	return rt, nil
}

// StartServer starts the runtime in server mode. This function will block the
// calling goroutine and will exit the program on error.
func (rt *Runtime) StartServer(ctx context.Context) {
	err := rt.Serve(ctx)
	if err != nil {
		os.Exit(1)
	}
}

// Serve will start a new REST API server and listen for requests. This
// will block until either: an error occurs, the context is canceled, or
// a SIGTERM or SIGKILL signal is sent.
func (rt *Runtime) Serve(ctx context.Context) error {
	if rt.Params.Addrs == nil {
		return fmt.Errorf("at least one address must be configured in runtime parameters")
	}

	if rt.Params.DiagnosticAddrs == nil {
		rt.Params.DiagnosticAddrs = &[]string{}
	}

	rt.logger.WithFields(map[string]interface{}{
		"addrs":            *rt.Params.Addrs,
		"diagnostic-addrs": *rt.Params.DiagnosticAddrs,
	}).Info("Initializing server.")

	if rt.Params.Authorization == server.AuthorizationOff && rt.Params.Authentication == server.AuthenticationToken {
		rt.logger.Error("Token authentication enabled without authorization. Authentication will be ineffective. See https://www.openpolicyagent.org/docs/latest/security/#authentication-and-authorization for more information.")
	}

	checkUserPrivileges(rt.logger)

	// NOTE(tsandall): at some point, hopefully we can remove this because the
	// Go runtime will just do the right thing. Until then, try to set
	// GOMAXPROCS based on the CPU quota applied to the process.
	undo, err := maxprocs.Set(maxprocs.Logger(func(f string, a ...interface{}) {
		rt.logger.Debug(f, a...)
	}))
	if err != nil {
		rt.logger.WithFields(map[string]interface{}{"err": err}).Debug("Failed to set GOMAXPROCS from CPU quota.")
	}

	defer undo()

	if err := rt.Manager.Start(ctx); err != nil {
		rt.logger.WithFields(map[string]interface{}{"err": err}).Error("Failed to start plugins.")
		return err
	}

	defer rt.Manager.Stop(ctx)

	if rt.traceExporter != nil {
		if err := rt.traceExporter.Start(ctx); err != nil {
			rt.logger.WithFields(map[string]interface{}{"err": err}).Error("Failed to start OpenTelemetry trace exporter.")
			return err
		}

		defer func() {
			err := rt.traceExporter.Shutdown(ctx)
			if err != nil {
				rt.logger.WithFields(map[string]interface{}{"err": err}).Error("Failed to shutdown OpenTelemetry trace exporter gracefully.")
			}
		}()
	}

	rt.server = server.New().
		WithRouter(rt.Params.Router).
		WithStore(rt.Store).
		WithManager(rt.Manager).
		WithCompilerErrorLimit(rt.Params.ErrorLimit).
		WithPprofEnabled(rt.Params.PprofEnabled).
		WithAddresses(*rt.Params.Addrs).
		WithH2CEnabled(rt.Params.H2CEnabled).
		WithCertificate(rt.Params.Certificate).
		WithCertificatePaths(rt.Params.CertificateFile, rt.Params.CertificateKeyFile, rt.Params.CertificateRefresh).
		WithCertPool(rt.Params.CertPool).
		WithAuthentication(rt.Params.Authentication).
		WithAuthorization(rt.Params.Authorization).
		WithDecisionIDFactory(rt.decisionIDFactory).
		WithDecisionLoggerWithErr(rt.decisionLogger).
		WithRuntime(rt.Manager.Info).
		WithMetrics(rt.metrics).
		WithMinTLSVersion(rt.Params.MinTLSVersion).
		WithDistributedTracingOpts(rt.Params.DistributedTracingOpts)

	// If decision_logging plugin enabled, check to see if we opted in to the ND builtins cache.
	if lp := logs.Lookup(rt.Manager); lp != nil {
		rt.server = rt.server.WithNDBCacheEnabled(rt.Manager.Config.NDBuiltinCacheEnabled())
	}

	if rt.Params.DiagnosticAddrs != nil {
		rt.server = rt.server.WithDiagnosticAddresses(*rt.Params.DiagnosticAddrs)
	}

	rt.server, err = rt.server.Init(ctx)
	if err != nil {
		rt.logger.WithFields(map[string]interface{}{"err": err}).Error("Unable to initialize server.")
		return err
	}

	if rt.Params.Watch {
		if err := rt.startWatcher(ctx, rt.Params.Paths, rt.onReloadLogger); err != nil {
			rt.logger.WithFields(map[string]interface{}{"err": err}).Error("Unable to open watch.")
			return err
		}
	}

	if rt.Params.EnableVersionCheck {
		d := time.Duration(int64(time.Second) * defaultUploadIntervalSec)
		rt.done = make(chan struct{})
		go rt.checkOPAUpdateLoop(ctx, d, rt.done)
	}

	defer func() {
		if rt.done != nil {
			rt.done <- struct{}{}
		}
	}()

	rt.server.Handler = NewLoggingHandler(rt.logger, rt.server.Handler)
	rt.server.DiagnosticHandler = NewLoggingHandler(rt.logger, rt.server.DiagnosticHandler)

	if err := rt.waitPluginsReady(
		100*time.Millisecond,
		time.Second*time.Duration(rt.Params.ReadyTimeout)); err != nil {
		rt.logger.WithFields(map[string]interface{}{"err": err}).Error("Failed to wait for plugins activation.")
		return err
	}

	loops, err := rt.server.Listeners()
	if err != nil {
		rt.logger.WithFields(map[string]interface{}{"err": err}).Error("Unable to create listeners.")
		return err
	}

	errc := make(chan error)
	for _, loop := range loops {
		go func(serverLoop func() error) {
			errc <- serverLoop()
		}(loop)
	}

	// Buffer one element as os/signal uses non-blocking channel sends.
	// This prevents potentially dropping the first element and failing to shut
	// down gracefully. A buffer of 1 is sufficient as we're just looking for a
	// one-time shutdown signal.
	signalc := make(chan os.Signal, 1)
	signal.Notify(signalc, syscall.SIGINT, syscall.SIGTERM)

	// Note that there is a small chance the socket of the server listener is still
	// closed by the time this block is executed, due to the serverLoop above
	// executing in a goroutine.
	rt.serverInitMtx.Lock()
	rt.serverInitialized = true
	rt.serverInitMtx.Unlock()
	rt.Manager.ServerInitialized()

	rt.logger.Debug("Server initialized.")

	for {
		select {
		case <-ctx.Done():
			return rt.gracefulServerShutdown(rt.server)
		case <-signalc:
			return rt.gracefulServerShutdown(rt.server)
		case err := <-errc:
			rt.logger.WithFields(map[string]interface{}{"err": err}).Error("Listener failed.")
			os.Exit(1)
		}
	}
}

// Addrs returns a list of addresses that the runtime is listening on (when
// in server mode). Returns an empty list if it hasn't started listening.
func (rt *Runtime) Addrs() []string {
	rt.serverInitMtx.RLock()
	defer rt.serverInitMtx.RUnlock()

	if !rt.serverInitialized {
		return nil
	}

	return rt.server.Addrs()
}

// DiagnosticAddrs returns a list of diagnostic addresses that the runtime is
// listening on (when in server mode). Returns an empty list if it hasn't
// started listening.
func (rt *Runtime) DiagnosticAddrs() []string {
	if rt.server == nil {
		return nil
	}

	return rt.server.DiagnosticAddrs()
}

// StartREPL starts the runtime in REPL mode. This function will block the calling goroutine.
func (rt *Runtime) StartREPL(ctx context.Context) {
	if err := rt.Manager.Start(ctx); err != nil {
		fmt.Fprintln(rt.Params.Output, "error starting plugins:", err)
		os.Exit(1)
	}

	defer rt.Manager.Stop(ctx)

	banner := rt.getBanner()
	repl := repl.New(rt.Store, rt.Params.HistoryPath, rt.Params.Output, rt.Params.OutputFormat, rt.Params.ErrorLimit, banner).
		WithRuntime(rt.Manager.Info)

	if rt.Params.Watch {
		if err := rt.startWatcher(ctx, rt.Params.Paths, onReloadPrinter(rt.Params.Output)); err != nil {
			fmt.Fprintln(rt.Params.Output, "error opening watch:", err)
			os.Exit(1)
		}
	}

	if rt.Params.EnableVersionCheck {
		go func() {
			repl.SetOPAVersionReport(rt.checkOPAUpdate(ctx).Slice())
		}()
	}
	repl.Loop(ctx)
}

// SetDistributedTracingLogging configures the distributed tracing's ErrorHandler,
// and logger instances.
func (rt *Runtime) SetDistributedTracingLogging() {
	internal_tracing.SetupLogging(rt.logger)
}

func (rt *Runtime) checkOPAUpdate(ctx context.Context) *report.DataResponse {
	resp, _ := rt.reporter.SendReport(ctx)
	return resp
}

func (rt *Runtime) checkOPAUpdateLoop(ctx context.Context, uploadDuration time.Duration, done chan struct{}) {
	ticker := time.NewTicker(uploadDuration)
	mr.New(mr.NewSource(time.Now().UnixNano())) // Seed the PRNG.

	for {
		resp, err := rt.reporter.SendReport(ctx)
		if err != nil {
			rt.logger.WithFields(map[string]interface{}{"err": err}).Debug("Unable to send OPA version report.")
		} else {
			if resp.Latest.OPAUpToDate {
				rt.logger.WithFields(map[string]interface{}{
					"current_version": version.Version,
				}).Debug("OPA is up to date.")
			} else {
				rt.logger.WithFields(map[string]interface{}{
					"download_opa":    resp.Latest.Download,
					"release_notes":   resp.Latest.ReleaseNotes,
					"current_version": version.Version,
					"latest_version":  strings.TrimPrefix(resp.Latest.LatestRelease, "v"),
				}).Info("OPA is out of date.")
			}
		}
		select {
		case <-ticker.C:
			ticker.Stop()
			newInterval := mr.Int63n(defaultUploadIntervalSec) + defaultUploadIntervalSec
			ticker = time.NewTicker(time.Duration(int64(time.Second) * newInterval))
		case <-done:
			ticker.Stop()
			return
		}
	}
}

func (rt *Runtime) decisionIDFactory() string {
	if rt.Params.DecisionIDFactory != nil {
		return rt.Params.DecisionIDFactory()
	}
	if logs.Lookup(rt.Manager) != nil {
		return generateDecisionID()
	}
	return ""
}

func (rt *Runtime) decisionLogger(ctx context.Context, event *server.Info) error {
	plugin := logs.Lookup(rt.Manager)
	if plugin == nil {
		return nil
	}

	return plugin.Log(ctx, event)
}

func (rt *Runtime) startWatcher(ctx context.Context, paths []string, onReload func(time.Duration, error)) error {
	watcher, err := rt.getWatcher(paths)
	if err != nil {
		return err
	}
	go rt.readWatcher(ctx, watcher, paths, onReload)
	return nil
}

func (rt *Runtime) readWatcher(ctx context.Context, watcher *fsnotify.Watcher, paths []string, onReload func(time.Duration, error)) {
	for evt := range watcher.Events {
		removalMask := fsnotify.Remove | fsnotify.Rename
		mask := fsnotify.Create | fsnotify.Write | removalMask
		if (evt.Op & mask) != 0 {
			rt.logger.WithFields(map[string]interface{}{
				"event": evt.String(),
			}).Debug("Registered file event.")
			t0 := time.Now()
			removed := ""
			if (evt.Op & removalMask) != 0 {
				removed = evt.Name
			}
			err := rt.processWatcherUpdate(ctx, paths, removed)
			onReload(time.Since(t0), err)
		}
	}
}

func (rt *Runtime) processWatcherUpdate(ctx context.Context, paths []string, removed string) error {
	loaded, err := initload.LoadPaths(paths, rt.Params.Filter, rt.Params.BundleMode, nil, true, false, nil)
	if err != nil {
		return err
	}

	removed = loader.CleanPath(removed)

	return storage.Txn(ctx, rt.Store, storage.WriteParams, func(txn storage.Transaction) error {
		if !rt.Params.BundleMode {
			ids, err := rt.Store.ListPolicies(ctx, txn)
			if err != nil {
				return err
			}
			for _, id := range ids {
				if id == removed {
					if err := rt.Store.DeletePolicy(ctx, txn, id); err != nil {
						return err
					}
				} else if _, exists := loaded.Files.Modules[id]; !exists {
					// This branch get hit in two cases.
					// 1. Another piece of code has access to the store and inserts
					//    a policy out-of-band.
					// 2. In between FS notification and loader.Filtered() call above, a
					//    policy is removed from disk.
					bs, err := rt.Store.GetPolicy(ctx, txn, id)
					if err != nil {
						return err
					}
					module, err := ast.ParseModule(id, string(bs))
					if err != nil {
						return err
					}
					loaded.Files.Modules[id] = &loader.RegoFile{
						Name:   id,
						Raw:    bs,
						Parsed: module,
					}
				}
			}
		}

		_, err := initload.InsertAndCompile(ctx, initload.InsertAndCompileOptions{
			Store:     rt.Store,
			Txn:       txn,
			Files:     loaded.Files,
			Bundles:   loaded.Bundles,
			MaxErrors: -1,
		})
		if err != nil {
			return err
		}

		return nil
	})
}

func (rt *Runtime) getBanner() string {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "OPA %v (commit %v, built at %v)\n", version.Version, version.Vcs, version.Timestamp)
	fmt.Fprintf(&buf, "\n")
	fmt.Fprintf(&buf, "Run 'help' to see a list of commands and check for updates.\n")
	return buf.String()
}

func (rt *Runtime) gracefulServerShutdown(s *server.Server) error {
	if rt.Params.ShutdownWaitPeriod > 0 {
		rt.logger.Info("Waiting %vs before initiating shutdown...", rt.Params.ShutdownWaitPeriod)
		time.Sleep(time.Duration(rt.Params.ShutdownWaitPeriod) * time.Second)
	}

	rt.logger.Info("Shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(rt.Params.GracefulShutdownPeriod)*time.Second)
	defer cancel()
	err := s.Shutdown(ctx)
	if err != nil {
		rt.logger.WithFields(map[string]interface{}{"err": err}).Error("Failed to shutdown server gracefully.")
		return err
	}
	rt.logger.Info("Server shutdown.")
	return nil
}

func (rt *Runtime) waitPluginsReady(checkInterval, timeout time.Duration) error {
	if timeout <= 0 {
		return nil
	}

	// check readiness of all plugins
	pluginsReady := func() bool {
		for _, status := range rt.Manager.PluginStatus() {
			if status != nil && status.State != plugins.StateOK {
				return false
			}
		}
		return true
	}

	rt.logger.Debug("Waiting for plugins activation (%v).", timeout)

	return util.WaitFunc(pluginsReady, checkInterval, timeout)
}

func (rt *Runtime) onReloadLogger(d time.Duration, err error) {
	rt.logger.WithFields(map[string]interface{}{
		"duration": d,
		"err":      err,
	}).Info("Processed file watch event.")
}

func (rt *Runtime) getWatcher(rootPaths []string) (*fsnotify.Watcher, error) {
	watchPaths, err := getWatchPaths(rootPaths)
	if err != nil {
		return nil, err
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	for _, path := range watchPaths {
		rt.logger.WithFields(map[string]interface{}{"path": path}).Debug("watching path")
		if err := watcher.Add(path); err != nil {
			return nil, err
		}
	}

	return watcher, nil
}

func urlPathToConfigOverride(pathCount int, path string) ([]string, error) {
	uri, err := url.Parse(path)
	if err != nil {
		return nil, err
	}
	baseURL := uri.Scheme + "://" + uri.Host
	urlPath := uri.Path
	if uri.RawQuery != "" {
		urlPath += "?" + uri.RawQuery
	}

	return []string{
		fmt.Sprintf("services.cli%d.url=%s", pathCount, baseURL),
		fmt.Sprintf("bundles.cli%d.service=cli%d", pathCount, pathCount),
		fmt.Sprintf("bundles.cli%d.resource=%s", pathCount, urlPath),
		fmt.Sprintf("bundles.cli%d.persist=true", pathCount),
	}, nil
}

func errorLogger(logger logging.Logger) func(attrs map[string]interface{}, f string, a ...interface{}) {
	return func(attrs map[string]interface{}, f string, a ...interface{}) {
		logger.WithFields(attrs).Error(f, a...)
	}
}

func getWatchPaths(rootPaths []string) ([]string, error) {
	paths := []string{}

	for _, path := range rootPaths {

		_, path = loader.SplitPrefix(path)
		result, err := loader.Paths(path, true)
		if err != nil {
			return nil, err
		}

		paths = append(paths, loader.Dirs(result)...)
	}

	return paths, nil
}

func onReloadPrinter(output io.Writer) func(time.Duration, error) {
	return func(d time.Duration, err error) {
		if err != nil {
			fmt.Fprintf(output, "\n# reload error (took %v): %v", d, err)
		} else {
			fmt.Fprintf(output, "\n# reloaded files (took %v)", d)
		}
	}
}

func generateInstanceID() (string, error) {
	return uuid.New(rand.Reader)
}

func generateDecisionID() string {
	id, err := uuid.New(rand.Reader)
	if err != nil {
		return ""
	}
	return id
}

func init() {
	registeredPlugins = make(map[string]plugins.Factory)
}
