// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// +build linux,cgo darwin,cgo

package runtime

// Contains parts of the runtime package that use the plugin package

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"plugin"

	"github.com/pkg/errors"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/loader"
	"github.com/open-policy-agent/opa/plugins"
	"github.com/open-policy-agent/opa/plugins/logs"
	"github.com/open-policy-agent/opa/server"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/storage/inmem"
	"github.com/open-policy-agent/opa/topdown"
)

// NewRuntime returns a new Runtime object initialized with params.
func NewRuntime(ctx context.Context, params Params) (*Runtime, error) {

	if params.ID == "" {
		var err error
		params.ID, err = generateInstanceID()
		if err != nil {
			return nil, err
		}
	}

	loaded, err := loader.Filtered(params.Paths, params.Filter)
	if err != nil {
		return nil, err
	}

	store := inmem.New()

	txn, err := store.NewTransaction(ctx, storage.WriteParams)
	if err != nil {
		return nil, err
	}

	// only register custom plugins if directory specified
	if params.BuiltinDir != "" {
		err = RegisterBuiltinsFromDir(params.BuiltinDir)
		if err != nil {
			return nil, err
		}
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

	// register before init
	if params.PluginDir != "" {
		err = RegisterPluginsFromDir(params.PluginDir)
		if err != nil {
			return nil, err
		}
	}

	m, plugins, err := initPlugins(params.ID, store, params.ConfigFile)
	if err != nil {
		return nil, err
	}

	var decisionLogger func(context.Context, *server.Info)

	if p, ok := plugins["decision_logs"]; ok {
		decisionLogger = p.(*logs.Plugin).Log

		if params.DecisionIDFactory == nil {
			params.DecisionIDFactory = generateDecisionID
		}
	}

	rt := &Runtime{
		Store:          store,
		Manager:        m,
		Params:         params,
		decisionLogger: decisionLogger,
	}

	return rt, nil
}

// RegisterBuiltinsFromDir recursively loads all custom builtins into OPA from dir. This function is idempotent.
func RegisterBuiltinsFromDir(dir string) error {
	return filepath.Walk(dir, walker("*.builtin.so", registerBuiltinFromFile))
}

// RegisterPluginsFromDir recursively loads all custom plugins into OPA from dir. This function is idempotent
func RegisterPluginsFromDir(dir string) error {
	return filepath.Walk(dir, walker("*.plugin.so", registerPluginFromFile))
}

// walker returns a walkfunc that performs handler on every file that matches the file glob pattern.
// it skips all files that do not match pattern TODO: is this what we want?
func walker(pattern string, handler func(string) error) filepath.WalkFunc {
	walk := func(path string, f os.FileInfo, err error) error {
		// if error occurs during traversal to path, exit and crash
		if err != nil {
			return err
		}
		// skip anything that is a directory
		if f.IsDir() {
			return nil
		}

		if ok, err := filepath.Match(pattern, filepath.Base(path)); err != nil {
			return err
		} else if !ok {
			return nil
		}

		return handler(path)
	}
	return walk
}

// loads the builtin from a file path
func registerBuiltinFromFile(path string) error {
	mod, err := plugin.Open(path)
	if err != nil {
		return err
	}
	builtinSym, err := mod.Lookup("Builtin")
	if err != nil {
		return err
	}
	functionSym, err := mod.Lookup("Function")
	if err != nil {
		return err
	}

	// type assert builtin symbol
	builtin, ok := builtinSym.(*ast.Builtin)
	if !ok {
		return fmt.Errorf("symbol Builtin must be of type ast.Builtin")
	}

	// type assert function symbol
	switch fnc := functionSym.(type) {
	case *topdown.BuiltinFunc:
		ast.RegisterBuiltin(builtin)
		topdown.RegisterBuiltinFunc(builtin.Name, *fnc)
	case *topdown.FunctionalBuiltin1:
		ast.RegisterBuiltin(builtin)
		topdown.RegisterFunctionalBuiltin1(builtin.Name, *fnc)
	case *topdown.FunctionalBuiltin2:
		ast.RegisterBuiltin(builtin)
		topdown.RegisterFunctionalBuiltin2(builtin.Name, *fnc)
	case *topdown.FunctionalBuiltin3:
		ast.RegisterBuiltin(builtin)
		topdown.RegisterFunctionalBuiltin3(builtin.Name, *fnc)
	default:
		return fmt.Errorf("symbol Function was of an unrecognized type")
	}

	// TODO: replace with logging
	fmt.Printf("Registered builtin %v from %v\n", builtin.Name, path)
	return nil
}

func registerPluginFromFile(path string) error {
	mod, err := plugin.Open(path)
	if err != nil {
		return err
	}

	nameSym, err := mod.Lookup("Name")
	if err != nil {
		return err
	}
	name, ok := nameSym.(*string)
	if !ok {
		return fmt.Errorf("symbol Name must be of type string")
	}

	pluginSym, err := mod.Lookup("Initializer")
	if err != nil {
		return err
	}

	// type assert initializer function
	initFunc, ok := pluginSym.(*plugins.PluginInitFunc)
	if !ok {
		return fmt.Errorf("symbol Builtin must be of type runtime.PluginInitFunc")
	}

	//TODO: replace this with appropriate logging method
	fmt.Printf("Registered plugin %v from %v\n", name, path)
	RegisterPlugin(*name, *initFunc)
	return nil
}
