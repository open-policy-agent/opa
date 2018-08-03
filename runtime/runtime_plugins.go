// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// +build linux,cgo darwin,cgo

package runtime

// Contains parts of the runtime package that use the plugin package

import (
	"fmt"
	"os"
	"path/filepath"
	"plugin"

	"reflect"
)

// RegisterSharedObjectsFromDir registers all .builtin.so and .plugin.so files recursively stored in dir into OPA.
func RegisterSharedObjectsFromDir(dir string) error {
	return filepath.Walk(dir, loadSharedObjectWalker())
}

// loadSharedObjectWalker returns a walkfunc that registers every .builtin.so file and every .plugin.so file into OPA.
// Ignores all other file types.
func loadSharedObjectWalker() filepath.WalkFunc {
	walk := func(path string, f os.FileInfo, err error) error {
		// if error occurs during traversal to path, exit and crash
		if err != nil {
			return err
		}
		// skip anything that is a directory
		if f.IsDir() {
			return nil
		}

		if filepath.Ext(path) == ".so" {
			return registerSharedObjectFromFile(path)
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
	fmt.Println(reflect.TypeOf(initSym).String())
	if !ok {
		return fmt.Errorf("symbol Init must be of type func() error")
	}

	// execute init
	return init()
}
