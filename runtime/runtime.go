// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package runtime

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"time"

	fsnotify "gopkg.in/fsnotify.v1"

	"path/filepath"

	"github.com/golang/glog"
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

	// PolicyDir is the filename of the directory to persist policy
	// definitions in. Policy definitions stored in this directory
	// are automatically loaded on startup.
	PolicyDir string

	// Server flag controls whether the OPA instance will start a server.
	// By default, the OPA instance acts as an interactive shell.
	Server bool

	// Watch flag controls whether OPA will watch the Paths files for changes.
	// This flag is only supported if OPA instance is running as an interactive
	// shell. If this flag is true, OPA will watch the Paths files for changes
	// and reload the storage layer each time they change. This is useful for
	// interactive development.
	Watch bool

	// Output is the output stream used when run as an interactive shell. This
	// is mostly for test purposes.
	Output io.Writer
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

	if err := rt.init(params); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if params.Server {
		rt.startServer(params)
	} else {
		rt.startRepl(params)
	}

}

func (rt *Runtime) init(params *Params) error {

	if len(params.PolicyDir) > 0 {
		if err := os.MkdirAll(params.PolicyDir, 0755); err != nil {
			return errors.Wrap(err, "unable to make --policy-dir")
		}
	}

	loaded, err := loadAllPaths(params.Paths)
	if err != nil {
		return err
	}

	// Open data store and load base documents.
	store := storage.New(storage.InMemoryConfig().WithPolicyDir(params.PolicyDir))

	if err := store.Open(); err != nil {
		return err
	}

	txn, err := store.NewTransaction()
	if err != nil {
		return err
	}

	defer store.Close(txn)

	ref := ast.Ref{ast.DefaultRootDocument}
	if err := store.Write(txn, storage.AddOp, ref, loaded.Documents); err != nil {
		return errors.Wrapf(err, "storage error")
	}

	// Load policies provided via input.
	if err := compileAndStoreInputs(loaded.Modules, store, txn); err != nil {
		return errors.Wrapf(err, "compile error")
	}

	rt.Store = store

	return nil
}

func (rt *Runtime) startServer(params *Params) {

	glog.Infof("First line of log stream.")
	glog.V(2).Infof("Server listening address: %v.", params.Addr)

	persist := len(params.PolicyDir) > 0

	s, err := server.New(rt.Store, params.Addr, persist)

	if err != nil {
		glog.Fatalf("Error creating server: %v", err)
	}

	s.Handler = NewLoggingHandler(s.Handler)

	if err := s.Loop(); err != nil {
		glog.Fatalf("Server exiting: %v", err)
	}
}

func (rt *Runtime) startRepl(params *Params) {

	banner := rt.getBanner()
	repl := repl.New(rt.Store, params.HistoryPath, params.Output, params.OutputFormat, banner)

	if params.Watch {
		watcher, err := rt.getWatcher(params.Paths)
		if err != nil {
			fmt.Fprintln(params.Output, "error opening watch:", err)
			os.Exit(1)
		}
		go rt.readWatcher(watcher, params.Paths)
	}

	if params.Eval == "" {
		repl.Loop()
	} else {
		repl.DisableUndefinedOutput(true)
		repl.DisableMultiLineBuffering(true)
		if err := repl.OneShot(params.Eval); err != nil {
			fmt.Fprintln(params.Output, "error:", err)
			os.Exit(1)
		}
	}

}

func (rt *Runtime) getWatcher(rootPaths []string) (*fsnotify.Watcher, error) {

	watchPaths := []string{}
	for _, path := range rootPaths {
		result, err := listPaths(path, true)
		if err != nil {
			return nil, err
		}
		watchPaths = append(watchPaths, result...)
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

func (rt *Runtime) readWatcher(watcher *fsnotify.Watcher, paths []string) {
	for {
		select {
		case evt := <-watcher.Events:
			mask := (fsnotify.Create | fsnotify.Remove | fsnotify.Rename | fsnotify.Write)
			if (evt.Op & mask) != 0 {
				t0 := time.Now()
				if err := rt.processWatcherUpdate(paths); err != nil {
					fmt.Fprintf(os.Stdout, "\n# reload error (took %v): %v", time.Since(t0), err)
				} else {
					fmt.Fprintf(os.Stdout, "\n# reloaded files (took %v)", time.Since(t0))
				}
			}
		}
	}
}

func (rt *Runtime) processWatcherUpdate(paths []string) error {

	loaded, err := loadAllPaths(paths)
	if err != nil {
		return err
	}

	txn, err := rt.Store.NewTransaction()
	if err != nil {
		return err
	}

	defer rt.Store.Close(txn)

	ref := ast.Ref{ast.DefaultRootDocument}
	if err := rt.Store.Write(txn, storage.AddOp, ref, loaded.Documents); err != nil {
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
		if err := store.InsertPolicy(txn, id, modules[id].Parsed, modules[id].Raw, false); err != nil {
			return err
		}
	}

	return nil
}

func mustListPaths(path string, recurse bool) (paths []string) {
	paths, err := listPaths(path, recurse)
	if err != nil {
		panic(err)
	}
	return paths
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
