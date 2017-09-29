// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package runtime

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"os"
	"time"

	fsnotify "gopkg.in/fsnotify.v1"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/loader"
	"github.com/open-policy-agent/opa/repl"
	"github.com/open-policy-agent/opa/server"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/storage/inmem"
	"github.com/open-policy-agent/opa/version"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// Params stores the configuration for an OPA instance.
type Params struct {

	// Addr is the listening address that the OPA server will bind to.
	Addr string

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

	// Eval is a string to evaluate in the REPL.
	Eval string

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

	// Watch flag controls whether OPA will watch the Paths files for changes.
	// If this flag is true, OPA will watch the Paths files for changes and
	// reload the storage layer each time they change. This is useful for
	// interactive development.
	Watch bool

	// ErrorLimit is the number of errors the compiler will allow to occur before
	// exiting early.
	ErrorLimit int

	// DecisionIDFactory generates decision IDs to include in API responses
	// sent by the server (in response to Data API queries.)
	DecisionIDFactory func() string

	// DiagnosticsBuffer is used by the server to record policy decisions.
	DiagnosticsBuffer server.Buffer

	// Logging configures the logging behaviour.
	Logging LoggingConfig

	// Output is the output stream used when run as an interactive shell. This
	// is mostly for test purposes.
	Output io.Writer
}

// LoggingConfig stores the configuration for OPA's logging behaviour.
type LoggingConfig struct {
	Level  string
	Format string
}

// NewParams returns a new Params object.
func NewParams() Params {
	return Params{
		Output: os.Stdout,
	}
}

// Runtime represents a single OPA instance.
type Runtime struct {
	Params Params
	Store  storage.Store
}

// NewRuntime returns a new Runtime object initialized with params.
func NewRuntime(ctx context.Context, params Params) (*Runtime, error) {

	loaded, err := loader.All(params.Paths)
	if err != nil {
		return nil, err
	}

	store := inmem.New()

	txn, err := store.NewTransaction(ctx, storage.WriteParams)
	if err != nil {
		return nil, err
	}

	if err := store.Write(ctx, txn, storage.AddOp, storage.Path{}, loaded.Documents); err != nil {
		store.Abort(ctx, txn)
		return nil, errors.Wrapf(err, "storage error")
	}

	if err := compileAndStoreInputs(ctx, store, txn, loaded.Modules, params.ErrorLimit); err != nil {
		store.Abort(ctx, txn)
		return nil, errors.Wrapf(err, "compile error")
	}

	if err := store.Commit(ctx, txn); err != nil {
		return nil, errors.Wrapf(err, "storage error")
	}

	rt := &Runtime{
		Store:  store,
		Params: params,
	}

	return rt, nil
}

// StartServer starts the runtime in server mode. This function will block the calling goroutine.
func (rt *Runtime) StartServer(ctx context.Context) {

	setupLogging(rt.Params.Logging)

	logrus.WithFields(logrus.Fields{
		"addr":          rt.Params.Addr,
		"insecure_addr": rt.Params.InsecureAddr,
	}).Infof("First line of log stream.")

	s, err := server.New().
		WithStore(rt.Store).
		WithCompilerErrorLimit(rt.Params.ErrorLimit).
		WithAddress(rt.Params.Addr).
		WithInsecureAddress(rt.Params.InsecureAddr).
		WithCertificate(rt.Params.Certificate).
		WithAuthentication(rt.Params.Authentication).
		WithAuthorization(rt.Params.Authorization).
		WithDiagnosticsBuffer(rt.Params.DiagnosticsBuffer).
		WithDecisionIDFactory(rt.Params.DecisionIDFactory).
		Init(ctx)

	if err != nil {
		logrus.WithField("err", err).Fatalf("Unable to initialize server.")
	}

	if rt.Params.Watch {
		if err := rt.startWatcher(ctx, rt.Params.Paths, onReloadLogger); err != nil {
			fmt.Fprintln(rt.Params.Output, "error opening watch:", err)
			os.Exit(1)
		}
	}

	s.Handler = NewLoggingHandler(s.Handler)

	loop1, loop2 := s.Listeners()
	if loop2 != nil {
		go func() {
			if err := loop2(); err != nil {
				logrus.WithField("err", err).Fatalf("Server exiting.")
			}
		}()
	}

	if err := loop1(); err != nil {
		logrus.WithField("err", err).Fatalf("Server exiting.")
	}

}

// StartREPL starts the runtime in REPL mode. This function will block the calling goroutine.
func (rt *Runtime) StartREPL(ctx context.Context) {

	banner := rt.getBanner()
	repl := repl.New(rt.Store, rt.Params.HistoryPath, rt.Params.Output, rt.Params.OutputFormat, rt.Params.ErrorLimit, banner)

	if rt.Params.Watch {
		if err := rt.startWatcher(ctx, rt.Params.Paths, onReloadPrinter(rt.Params.Output)); err != nil {
			fmt.Fprintln(rt.Params.Output, "error opening watch:", err)
			os.Exit(1)
		}
	}

	if rt.Params.Eval == "" {
		repl.Loop(ctx)
	} else {
		repl.DisableUndefinedOutput(true)
		repl.DisableMultiLineBuffering(true)

		if err := repl.OneShot(ctx, rt.Params.Eval); err != nil {
			fmt.Fprintln(rt.Params.Output, "error:", err)
			os.Exit(1)
		}
	}

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

	loaded, err := loader.All(paths)
	if err != nil {
		return err
	}

	removed = loader.CleanPath(removed)

	return storage.Txn(ctx, rt.Store, storage.WriteParams, func(txn storage.Transaction) error {
		if err := rt.Store.Write(ctx, txn, storage.AddOp, storage.Path{}, loaded.Documents); err != nil {
			return err
		}
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
				// 2. In between FS notification and loader.All() call above, a
				//    policy is removed from disk.
				bs, err := rt.Store.GetPolicy(ctx, txn, id)
				if err != nil {
					return err
				}
				module, err := ast.ParseModule(id, string(bs))
				if err != nil {
					return err
				}
				if _, ok := loaded.Modules[id]; !ok {
					loaded.Modules[id] = &loader.RegoFile{
						Name:   id,
						Raw:    bs,
						Parsed: module,
					}
				}
			}
		}
		if err := compileAndStoreInputs(ctx, rt.Store, txn, loaded.Modules, -1); err != nil {
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

func compileAndStoreInputs(ctx context.Context, store storage.Store, txn storage.Transaction, modules map[string]*loader.RegoFile, errorLimit int) error {

	policies := make(map[string]*ast.Module, len(modules))

	for id, parsed := range modules {
		policies[id] = parsed.Parsed
	}

	c := ast.NewCompiler().SetErrorLimit(errorLimit)

	if c.Compile(policies); c.Failed() {
		return c.Errors
	}

	for id, parsed := range modules {
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
	case "json":
		logrus.SetFormatter(&logrus.JSONFormatter{})
	default:
		logrus.SetFormatter(&logrus.TextFormatter{
			FullTimestamp: true,
		})
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
