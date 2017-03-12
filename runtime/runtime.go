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

	"path/filepath"

	"github.com/Sirupsen/logrus"
	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/repl"
	"github.com/open-policy-agent/opa/server"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/version"
	"github.com/pkg/errors"
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

	// Server flag controls whether the OPA instance will start a server.
	// By default, the OPA instance acts as an interactive shell.
	Server bool

	// Watch flag controls whether OPA will watch the Paths files for changes.
	// This flag is only supported if OPA instance is running as an interactive
	// shell. If this flag is true, OPA will watch the Paths files for changes
	// and reload the storage layer each time they change. This is useful for
	// interactive development.
	Watch bool

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
func NewParams() *Params {
	return &Params{
		Output: os.Stdout,
	}
}

// Runtime represents a single OPA instance.
type Runtime struct {
	Store *storage.Storage
}

// Start is the entry point of an OPA instance.
func (rt *Runtime) Start(params *Params) {

	ctx := context.Background()

	if err := rt.init(ctx, params); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if params.Server {
		rt.startServer(ctx, params)
	} else {
		rt.startRepl(ctx, params)
	}

}

func (rt *Runtime) init(ctx context.Context, params *Params) error {

	loaded, err := loadAllPaths(params.Paths)
	if err != nil {
		return err
	}

	// Open data store and load base documents.
	store := storage.New(storage.InMemoryConfig())

	if err := store.Open(ctx); err != nil {
		return err
	}

	txn, err := store.NewTransaction(ctx)
	if err != nil {
		return err
	}

	defer store.Close(ctx, txn)

	if err := store.Write(ctx, txn, storage.AddOp, storage.Path{}, loaded.Documents); err != nil {
		return errors.Wrapf(err, "storage error")
	}

	// Load policies provided via input.
	if err := compileAndStoreInputs(loaded.Modules, store, txn); err != nil {
		return errors.Wrapf(err, "compile error")
	}

	rt.Store = store

	return nil
}

func (rt *Runtime) startServer(ctx context.Context, params *Params) {

	setupLogging(params.Logging)

	logrus.WithFields(logrus.Fields{
		"addr":          params.Addr,
		"insecure_addr": params.InsecureAddr,
	}).Infof("First line of log stream.")

	s, err := server.New().
		WithStorage(rt.Store).
		WithAddress(params.Addr).
		WithInsecureAddress(params.InsecureAddr).
		WithCertificate(params.Certificate).
		WithAuthentication(params.Authentication).
		WithAuthorization(params.Authorization).
		Init(ctx)

	if err != nil {
		logrus.WithField("err", err).Fatalf("Unable to initialize server.")
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

func (rt *Runtime) startRepl(ctx context.Context, params *Params) {

	banner := rt.getBanner()
	repl := repl.New(rt.Store, params.HistoryPath, params.Output, params.OutputFormat, banner)

	if params.Watch {

		watcher, err := getWatcher(params.Paths)
		if err != nil {
			fmt.Fprintln(params.Output, "error opening watch:", err)
			os.Exit(1)
		}

		go rt.readWatcher(ctx, watcher, params.Paths)
	}

	if params.Eval == "" {
		repl.Loop(ctx)
	} else {
		repl.DisableUndefinedOutput(true)
		repl.DisableMultiLineBuffering(true)

		if err := repl.OneShot(ctx, params.Eval); err != nil {
			fmt.Fprintln(params.Output, "error:", err)
			os.Exit(1)
		}
	}

}

func (rt *Runtime) readWatcher(ctx context.Context, watcher *fsnotify.Watcher, paths []string) {
	for {
		select {
		case evt := <-watcher.Events:
			mask := (fsnotify.Create | fsnotify.Remove | fsnotify.Rename | fsnotify.Write)
			if (evt.Op & mask) != 0 {
				t0 := time.Now()
				if err := rt.processWatcherUpdate(ctx, paths); err != nil {
					fmt.Fprintf(os.Stdout, "\n# reload error (took %v): %v", time.Since(t0), err)
				} else {
					fmt.Fprintf(os.Stdout, "\n# reloaded files (took %v)", time.Since(t0))
				}
			}
		}
	}
}

func (rt *Runtime) processWatcherUpdate(ctx context.Context, paths []string) error {

	loaded, err := loadAllPaths(paths)
	if err != nil {
		return err
	}

	txn, err := rt.Store.NewTransaction(ctx)
	if err != nil {
		return err
	}

	defer rt.Store.Close(ctx, txn)

	if err := rt.Store.Write(ctx, txn, storage.AddOp, storage.Path{}, loaded.Documents); err != nil {
		return err
	}

	return compileAndStoreInputs(loaded.Modules, rt.Store, txn)
}

func (rt *Runtime) getBanner() string {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "OPA %v (commit %v, built at %v)\n", version.Version, version.Vcs, version.Timestamp)
	fmt.Fprintf(&buf, "\n")
	fmt.Fprintf(&buf, "Run 'help' to see a list of commands.\n")
	return buf.String()
}

func compileAndStoreInputs(modules map[string]*loadedModule, store *storage.Storage, txn storage.Transaction) error {

	policies := store.ListPolicies(txn)

	for id, mod := range modules {
		policies[id] = mod.Parsed
	}

	c := ast.NewCompiler()

	if c.Compile(policies); c.Failed() {
		return c.Errors
	}

	for id := range modules {
		if err := store.InsertPolicy(txn, id, modules[id].Parsed, modules[id].Raw); err != nil {
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

		_, path = splitPathPrefix(path)
		result, err := listPaths(path, true)
		if err != nil {
			return nil, err
		}

		paths = append(paths, result...)
	}

	return paths, nil
}

// listPaths returns a sorted list of files contained at path. If recurse is
// true and path is a directory, then listPaths will walk the directory
// structure recursively and list files at each level.
func listPaths(path string, recurse bool) (paths []string, err error) {
	err = filepath.Walk(path, func(f string, info os.FileInfo, err error) error {
		if !recurse {
			if path != f && path != filepath.Dir(f) {
				return filepath.SkipDir
			}
		}
		paths = append(paths, f)
		return nil
	})
	return paths, err
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
