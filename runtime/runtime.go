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
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/open-policy-agent/opa/bundle"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"gopkg.in/fsnotify.v1"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/internal/config"
	"github.com/open-policy-agent/opa/internal/prometheus"
	"github.com/open-policy-agent/opa/internal/report"
	"github.com/open-policy-agent/opa/internal/runtime"
	initload "github.com/open-policy-agent/opa/internal/runtime/init"
	"github.com/open-policy-agent/opa/internal/uuid"
	"github.com/open-policy-agent/opa/loader"
	"github.com/open-policy-agent/opa/metrics"
	"github.com/open-policy-agent/opa/plugins"
	"github.com/open-policy-agent/opa/plugins/discovery"
	"github.com/open-policy-agent/opa/plugins/logs"
	"github.com/open-policy-agent/opa/repl"
	"github.com/open-policy-agent/opa/server"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/storage/inmem"
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

	// CertPool holds the CA certs trusted by the OPA server.
	CertPool *x509.CertPool

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

	// DiagnosticsBuffer is used by the server to record policy decisions.
	// DEPRECATED. Use decision logging instead.
	DiagnosticsBuffer server.Buffer

	// Logging configures the logging behaviour.
	Logging LoggingConfig

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
}

// LoggingConfig stores the configuration for OPA's logging behaviour.
type LoggingConfig struct {
	Level  string
	Format string
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

	server   *server.Server
	metrics  *prometheus.Provider
	reporter *report.Reporter

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

	config, err := config.Load(params.ConfigFile, params.ConfigOverrides, params.ConfigOverrideFiles)
	if err != nil {
		return nil, errors.Wrap(err, "config error")
	}

	var reporter *report.Reporter
	if params.EnableVersionCheck {
		var err error
		reporter, err = report.New(params.ID)
		if err != nil {
			return nil, errors.Wrap(err, "config error")
		}
	}

	loaded, err := initload.LoadPaths(params.Paths, params.Filter, params.BundleMode, params.BundleVerificationConfig, params.SkipBundleVerification)
	if err != nil {
		return nil, errors.Wrap(err, "load error")
	}

	info, err := runtime.Term(runtime.Params{Config: config})
	if err != nil {
		return nil, err
	}

	manager, err := plugins.New(config, params.ID, inmem.New(), plugins.Info(info), plugins.InitBundles(loaded.Bundles), plugins.InitFiles(loaded.Files), plugins.MaxErrors(params.ErrorLimit), plugins.GracefulShutdownPeriod(params.GracefulShutdownPeriod))
	if err != nil {
		return nil, errors.Wrap(err, "config error")
	}

	if err := manager.Init(ctx); err != nil {
		return nil, errors.Wrap(err, "initialization error")
	}

	metrics := prometheus.New(metrics.New(), errorLogger)

	disco, err := discovery.New(manager, discovery.Factories(registeredPlugins), discovery.Metrics(metrics))
	if err != nil {
		return nil, errors.Wrap(err, "config error")
	}

	manager.Register("discovery", disco)

	rt := &Runtime{
		Store:             manager.Store,
		Params:            params,
		Manager:           manager,
		metrics:           metrics,
		reporter:          reporter,
		serverInitialized: false,
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

	setupLogging(rt.Params.Logging)

	logrus.WithFields(logrus.Fields{
		"addrs":            *rt.Params.Addrs,
		"diagnostic-addrs": *rt.Params.DiagnosticAddrs,
	}).Info("Initializing server.")

	if err := rt.Manager.Start(ctx); err != nil {
		logrus.WithField("err", err).Error("Failed to start plugins.")
		return err
	}

	defer rt.Manager.Stop(ctx)

	var err error
	rt.server = server.New().
		WithStore(rt.Store).
		WithManager(rt.Manager).
		WithCompilerErrorLimit(rt.Params.ErrorLimit).
		WithPprofEnabled(rt.Params.PprofEnabled).
		WithAddresses(*rt.Params.Addrs).
		WithH2CEnabled(rt.Params.H2CEnabled).
		WithCertificate(rt.Params.Certificate).
		WithCertPool(rt.Params.CertPool).
		WithAuthentication(rt.Params.Authentication).
		WithAuthorization(rt.Params.Authorization).
		WithDecisionIDFactory(rt.decisionIDFactory).
		WithDecisionLoggerWithErr(rt.decisionLogger).
		WithRuntime(rt.Manager.Info).
		WithMetrics(rt.metrics)

	if rt.Params.DiagnosticAddrs != nil {
		rt.server = rt.server.WithDiagnosticAddresses(*rt.Params.DiagnosticAddrs)
	}

	rt.server, err = rt.server.Init(ctx)
	if err != nil {
		logrus.WithField("err", err).Error("Unable to initialize server.")
		return err
	}

	if rt.Params.Watch {
		if err := rt.startWatcher(ctx, rt.Params.Paths, onReloadLogger); err != nil {
			logrus.WithField("err", err).Error("Unable to open watch.")
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

	rt.server.Handler = NewLoggingHandler(rt.server.Handler)
	rt.server.DiagnosticHandler = NewLoggingHandler(rt.server.DiagnosticHandler)

	if err := rt.waitPluginsReady(
		100*time.Millisecond,
		time.Second*time.Duration(rt.Params.ReadyTimeout)); err != nil {
		logrus.WithField("err", err).Error("Failed to wait for plugins activation.")
		return err
	}

	loops, err := rt.server.Listeners()
	if err != nil {
		logrus.WithField("err", err).Error("Unable to create listeners.")
		return err
	}

	errc := make(chan error)
	for _, loop := range loops {
		go func(serverLoop func() error) {
			errc <- serverLoop()
		}(loop)
	}

	signalc := make(chan os.Signal)
	signal.Notify(signalc, syscall.SIGINT, syscall.SIGTERM)

	rt.serverInitMtx.Lock()
	rt.serverInitialized = true
	rt.serverInitMtx.Unlock()

	logrus.Debug("Server initialized.")

	for {
		select {
		case <-ctx.Done():
			return rt.gracefulServerShutdown(rt.server)
		case <-signalc:
			return rt.gracefulServerShutdown(rt.server)
		case err := <-errc:
			logrus.WithField("err", err).Fatal("Listener failed.")
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
	repl := repl.New(rt.Store, rt.Params.HistoryPath, rt.Params.Output, rt.Params.OutputFormat, rt.Params.ErrorLimit, banner).WithRuntime(rt.Manager.Info)

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

func (rt *Runtime) checkOPAUpdate(ctx context.Context) *report.DataResponse {
	resp, _ := rt.reporter.SendReport(ctx)
	return resp
}

func (rt *Runtime) checkOPAUpdateLoop(ctx context.Context, uploadDuration time.Duration, done chan struct{}) {
	ticker := time.NewTicker(uploadDuration)
	mr.Seed(time.Now().UnixNano())

	for {
		resp, err := rt.reporter.SendReport(ctx)
		if err != nil {
			logrus.WithField("err", err).Debug("Unable to send OPA version report.")
		} else {
			if resp.Latest.OPAUpToDate {
				logrus.WithFields(logrus.Fields{
					"current_version": version.Version,
				}).Debug("OPA is up to date.")
			} else {
				logrus.WithFields(logrus.Fields{
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

	if rt.Params.DiagnosticsBuffer != nil {
		rt.Params.DiagnosticsBuffer.Push(event)
	}

	plugin := logs.Lookup(rt.Manager)
	if plugin == nil {
		return nil
	}

	return plugin.Log(ctx, event)
}

func (rt *Runtime) startWatcher(ctx context.Context, paths []string, onReload func(time.Duration, error)) error {
	watcher, err := getWatcher(paths)
	if err != nil {
		return err
	}
	go rt.readWatcher(ctx, watcher, paths, onReload)
	return nil
}

func (rt *Runtime) readWatcher(ctx context.Context, watcher *fsnotify.Watcher, paths []string, onReload func(time.Duration, error)) {
	for {
		select {
		case evt := <-watcher.Events:

			removalMask := (fsnotify.Remove | fsnotify.Rename)
			mask := (fsnotify.Create | fsnotify.Write | removalMask)
			if (evt.Op & mask) != 0 {
				logrus.WithFields(logrus.Fields{
					"event": evt.String(),
				}).Debugf("registered file event")
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
}

func (rt *Runtime) processWatcherUpdate(ctx context.Context, paths []string, removed string) error {

	loaded, err := initload.LoadPaths(paths, rt.Params.Filter, rt.Params.BundleMode, nil, true)
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
	logrus.Info("Shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(rt.Params.GracefulShutdownPeriod)*time.Second)
	defer cancel()
	err := s.Shutdown(ctx)
	if err != nil {
		logrus.WithField("err", err).Error("Failed to shutdown server gracefully.")
		return err
	}
	logrus.Info("Server shutdown.")
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

	logrus.Debugf("Waiting for plugins activation (%v).", timeout)

	return util.WaitFunc(pluginsReady, checkInterval, timeout)
}

func getWatcher(rootPaths []string) (*fsnotify.Watcher, error) {

	watchPaths, err := getWatchPaths(rootPaths)
	if err != nil {
		return nil, err
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	for _, path := range watchPaths {
		logrus.WithField("path", path).Debug("watching path")
		if err := watcher.Add(path); err != nil {
			return nil, err
		}
	}

	return watcher, nil
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

func onReloadLogger(d time.Duration, err error) {
	logrus.WithFields(logrus.Fields{
		"duration": d,
		"err":      err,
	}).Warn("Processed file watch event.")
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

func setupLogging(config LoggingConfig) {
	var formatter logrus.Formatter
	switch config.Format {
	case "text":
		formatter = &prettyFormatter{}
	case "json-pretty":
		formatter = &logrus.JSONFormatter{PrettyPrint: true}
	case "json":
		fallthrough
	default:
		formatter = &logrus.JSONFormatter{}
	}
	logrus.SetFormatter(formatter)
	// While the plugin console logger logs independently of the configured --log-level,
	// it should follow the configured --log-format
	plugins.GetConsoleLogger().SetFormatter(formatter)

	lvl := logrus.InfoLevel

	if config.Level != "" {
		var err error
		lvl, err = logrus.ParseLevel(config.Level)
		if err != nil {
			logrus.Fatalf("Unable to parse log level: %v", err)
		}
	}

	logrus.SetLevel(lvl)
}

func errorLogger(attrs map[string]interface{}, f string, a ...interface{}) {
	logrus.WithFields(logrus.Fields(attrs)).Errorf(f, a...)
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
