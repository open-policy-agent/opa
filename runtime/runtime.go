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
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/open-policy-agent/opa/bundle"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"gopkg.in/fsnotify.v1"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/internal/prometheus"
	"github.com/open-policy-agent/opa/internal/runtime"
	storedversion "github.com/open-policy-agent/opa/internal/version"
	"github.com/open-policy-agent/opa/loader"
	"github.com/open-policy-agent/opa/metrics"
	"github.com/open-policy-agent/opa/plugins"
	"github.com/open-policy-agent/opa/plugins/discovery"
	"github.com/open-policy-agent/opa/plugins/logs"
	"github.com/open-policy-agent/opa/repl"
	"github.com/open-policy-agent/opa/server"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/storage/inmem"
	"github.com/open-policy-agent/opa/version"
)

var (
	registeredPlugins    map[string]plugins.Factory
	registeredPluginsMux sync.Mutex
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

	// InsecureAddr is the listening address that the OPA server will bind to
	// in addition to Addr if TLS is enabled.
	InsecureAddr string

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
}

// LoggingConfig stores the configuration for OPA's logging behaviour.
type LoggingConfig struct {
	Level  string
	Format string
}

// NewParams returns a new Params object.
func NewParams() Params {
	return Params{
		Output:     os.Stdout,
		BundleMode: false,
	}
}

// Runtime represents a single OPA instance.
type Runtime struct {
	Params  Params
	Store   storage.Store
	Manager *plugins.Manager

	// TODO(tsandall): remove this field since it's available on the manager
	// and doesn't have to duplicated here or on the server.
	info *ast.Term // runtime information provided to evaluation engine

	server  *server.Server
	metrics *prometheus.Provider
}

// NewRuntime returns a new Runtime object initialized with params.
func NewRuntime(ctx context.Context, params Params) (*Runtime, error) {

	if params.ID == "" {
		var err error
		params.ID, err = generateInstanceID()
		if err != nil {
			return nil, err
		}
	}

	loaded, err := loadPaths(params.Paths, params.Filter, params.BundleMode)
	if err != nil {
		return nil, errors.Wrap(err, "load error")
	}

	store := inmem.New()

	txn, err := store.NewTransaction(ctx, storage.WriteParams)
	if err != nil {
		return nil, err
	}

	if len(loaded.Documents) > 0 {
		if err := store.Write(ctx, txn, storage.AddOp, storage.Path{}, loaded.Documents); err != nil {
			return nil, errors.Wrap(err, "storage error")
		}
	}

	if err := compileAndStoreInputs(ctx, store, txn, loaded, params.ErrorLimit); err != nil {
		store.Abort(ctx, txn)
		return nil, errors.Wrap(err, "compile error")
	}

	// Write the version *after* any data loaded from files or bundles to
	// avoid it being deleted.
	if err := storedversion.Write(ctx, store, txn); err != nil {
		store.Abort(ctx, txn)
		return nil, errors.Wrap(err, "storage error")
	}

	if err := store.Commit(ctx, txn); err != nil {
		return nil, errors.Wrap(err, "storage error")
	}

	bs, err := loadConfig(params)
	if err != nil {
		return nil, errors.Wrap(err, "config error")
	}

	info, err := runtime.Term(runtime.Params{Config: bs})
	if err != nil {
		return nil, err
	}

	manager, err := plugins.New(bs, params.ID, store, plugins.Info(info))
	if err != nil {
		return nil, errors.Wrap(err, "config error")
	}

	metrics := prometheus.New(metrics.New(), errorLogger)

	disco, err := discovery.New(manager, discovery.Factories(registeredPlugins), discovery.Metrics(metrics))
	if err != nil {
		return nil, errors.Wrap(err, "config error")
	}

	manager.Register("discovery", disco)

	rt := &Runtime{
		Store:   store,
		Params:  params,
		Manager: manager,
		info:    info,
		metrics: metrics,
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
	setupLogging(rt.Params.Logging)

	logrus.WithFields(logrus.Fields{
		"addrs":         *rt.Params.Addrs,
		"insecure_addr": rt.Params.InsecureAddr,
	}).Info("Initializing server.")

	if err := rt.Manager.Start(ctx); err != nil {
		logrus.WithField("err", err).Error("Failed to start plugins.")
		return err
	}

	defer rt.Manager.Stop(ctx)

	var err error
	rt.server, err = server.New().
		WithStore(rt.Store).
		WithManager(rt.Manager).
		WithCompilerErrorLimit(rt.Params.ErrorLimit).
		WithPprofEnabled(rt.Params.PprofEnabled).
		WithAddresses(*rt.Params.Addrs).
		WithInsecureAddress(rt.Params.InsecureAddr).
		WithCertificate(rt.Params.Certificate).
		WithCertPool(rt.Params.CertPool).
		WithAuthentication(rt.Params.Authentication).
		WithAuthorization(rt.Params.Authorization).
		WithDecisionIDFactory(rt.decisionIDFactory).
		WithDecisionLoggerWithErr(rt.decisionLogger).
		WithRuntime(rt.info).
		WithMetrics(rt.metrics).
		Init(ctx)

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

	rt.server.Handler = NewLoggingHandler(rt.server.Handler)

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
	if rt.server == nil {
		return nil
	}

	return rt.server.Addrs()
}

// StartREPL starts the runtime in REPL mode. This function will block the calling goroutine.
func (rt *Runtime) StartREPL(ctx context.Context) {

	if err := rt.Manager.Start(ctx); err != nil {
		fmt.Fprintln(rt.Params.Output, "error starting plugins:", err)
		os.Exit(1)
	}

	defer rt.Manager.Stop(ctx)

	banner := rt.getBanner()
	repl := repl.New(rt.Store, rt.Params.HistoryPath, rt.Params.Output, rt.Params.OutputFormat, rt.Params.ErrorLimit, banner).WithRuntime(rt.info)

	if rt.Params.Watch {
		if err := rt.startWatcher(ctx, rt.Params.Paths, onReloadPrinter(rt.Params.Output)); err != nil {
			fmt.Fprintln(rt.Params.Output, "error opening watch:", err)
			os.Exit(1)
		}
	}

	repl.Loop(ctx)
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

	loaded, err := loadPaths(paths, rt.Params.Filter, rt.Params.BundleMode)
	if err != nil {
		return err
	}

	removed = loader.CleanPath(removed)

	return storage.Txn(ctx, rt.Store, storage.WriteParams, func(txn storage.Transaction) error {
		if len(loaded.Documents) > 0 {
			if err := rt.Store.Write(ctx, txn, storage.AddOp, storage.Path{}, loaded.Documents); err != nil {
				return err
			}
		}

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
				} else if _, exists := loaded.Modules[id]; !exists {
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
					loaded.Modules[id] = &loader.RegoFile{
						Name:   id,
						Raw:    bs,
						Parsed: module,
					}
				}
			}
		}
		if err := compileAndStoreInputs(ctx, rt.Store, txn, loaded, -1); err != nil {
			return err
		}

		// re-add the version as it might have been overwritten from loading data files
		if err := storedversion.Write(ctx, rt.Store, txn); err != nil {
			return err
		}

		return nil
	})
}

func (rt *Runtime) getBanner() string {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "OPA %v (commit %v, built at %v)\n", version.Version, version.Vcs, version.Timestamp)
	fmt.Fprintf(&buf, "\n")
	fmt.Fprintf(&buf, "Run 'help' to see a list of commands.\n")
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

type loadResult struct {
	loader.Result
	Bundles map[string]*bundle.Bundle
}

func loadPaths(paths []string, filter loader.Filter, asBundle bool) (*loadResult, error) {
	result := &loadResult{}
	var err error

	if asBundle {
		result.Bundles = make(map[string]*bundle.Bundle, len(paths))
		for _, path := range paths {
			result.Bundles[path], err = loader.AsBundle(path)
			if err != nil {
				return nil, err
			}
		}
	} else {
		loaded, err := loader.Filtered(paths, filter)
		if err != nil {
			return nil, err
		}
		result.Modules = loaded.Modules
		result.Documents = loaded.Documents
	}

	return result, nil
}

func compileAndStoreInputs(ctx context.Context, store storage.Store, txn storage.Transaction, loaded *loadResult, errorLimit int) error {

	policies := make(map[string]*ast.Module, len(loaded.Modules))

	for id, parsed := range loaded.Modules {
		policies[id] = parsed.Parsed
	}

	c := ast.NewCompiler().SetErrorLimit(errorLimit).WithPathConflictsCheck(storage.NonEmpty(ctx, store, txn))

	opts := &bundle.ActivateOpts{
		Ctx:          ctx,
		Store:        store,
		Txn:          txn,
		Compiler:     c,
		Metrics:      metrics.New(),
		Bundles:      loaded.Bundles,
		ExtraModules: policies,
	}

	err := bundle.Activate(opts)
	if err != nil {
		return err
	}

	// Policies in bundles will have already been added to the store, but
	// modules loaded outside of bundles will need to be added manually.
	for id, parsed := range loaded.Modules {
		if err := store.UpsertPolicy(ctx, txn, id, parsed.Raw); err != nil {
			return err
		}
	}

	return nil
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

		paths = append(paths, result...)
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
	switch config.Format {
	case "text":
		logrus.SetFormatter(&prettyFormatter{})
	case "json-pretty":
		logrus.SetFormatter(&logrus.JSONFormatter{PrettyPrint: true})
	case "json":
		fallthrough
	default:
		logrus.SetFormatter(&logrus.JSONFormatter{})
	}

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
	return uuid4()
}

func generateDecisionID() string {
	id, err := uuid4()
	if err != nil {
		return ""
	}
	return id
}

func uuid4() (string, error) {
	bs := make([]byte, 16)
	n, err := io.ReadFull(rand.Reader, bs)
	if n != len(bs) || err != nil {
		return "", err
	}
	bs[8] = bs[8]&^0xc0 | 0x80
	bs[6] = bs[6]&^0xf0 | 0x40
	return fmt.Sprintf("%x-%x-%x-%x-%x", bs[0:4], bs[4:6], bs[6:8], bs[8:10], bs[10:]), nil
}

func init() {
	registeredPlugins = make(map[string]plugins.Factory)
}
