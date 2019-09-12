// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Specifies additional cmd commands that available to systems that can load plugins
// +build linux,cgo darwin,cgo

package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"plugin"

	"github.com/spf13/cobra"
)

// registerSharedObjectsFromDir recursively loads all .so files in dir into OPA.
func registerSharedObjectsFromDir(dir string) error {
	return filepath.Walk(dir, lambdaWalker(registerSharedObjectFromFile, ".so"))
}

// lambdaWalker returns a walkfunc that applies lambda to every file with extension ext. Ignores all other file types.
// Lambda should take the file path as its parameter.
func lambdaWalker(lambda func(string) error, ext string) filepath.WalkFunc {
	walk := func(path string, f os.FileInfo, err error) error {
		// if error occurs during traversal to path, exit and crash
		if err != nil {
			return err
		}
		// skip anything that is a directory
		if f.IsDir() {
			return nil
		}

		if filepath.Ext(path) == ext {
			return lambda(path)
		}

		// ignore anything else
		return nil
	}
	return walk
}

// loads the builtin from a file path
func registerSharedObjectFromFile(path string) error {
	mod, err := plugin.Open(path)
	if err != nil {
		return err
	}
	initSym, err := mod.Lookup("Init")
	if err != nil {
		return err
	}

	// type assert init symbol
	init, ok := initSym.(func() error)
	if !ok {
		return fmt.Errorf("symbol Init must be of type func() error")
	}

	// execute init
	return init()
}

func init() {
	var pluginDir string

	// flag is persistent (can be loaded on all children commands)
	RootCommand.PersistentFlags().StringVarP(&pluginDir, "plugin-dir", "", "", `set directory path to load built-in and plugin shared object files from`)
	RootCommand.PersistentFlags().MarkDeprecated("plugin-dir", "Shared objects are deprecated. See https://www.openpolicyagent.org/docs/latest/extensions/.")

	// Runs before *all* children commands
	RootCommand.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		// only register custom plugins if directory specified
		if pluginDir != "" {
			return registerSharedObjectsFromDir(pluginDir)
		}
		return nil
	}
}
