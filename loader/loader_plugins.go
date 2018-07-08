// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// +build linux,cgo darwin,cgo

package loader

// Builds version of the loader that can read plugins

import (
	"fmt"
	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/topdown"
	"github.com/pkg/errors"
	"path/filepath"
	"plugin"
)

// Builtin returns a CustomBuiltin object loaded from the given path.
// Will only work on darwin or linux OS
func Builtin(path string) (bfunc *CustomBuiltin, err error) {

	defer func() {
		err = errors.Wrap(err, path)
	}()

	bfunc = &CustomBuiltin{}

	mod, err := plugin.Open(path)
	if err != nil {
		return nil, err
	}
	builtinSym, err := mod.Lookup("Builtin")
	if err != nil {
		return nil, err
	}
	functionSym, err := mod.Lookup("Function")
	if err != nil {
		return nil, err
	}

	// type assert builtin symbol
	builtin, ok := builtinSym.(*ast.Builtin)
	if !ok {
		return nil, fmt.Errorf("symbol Builtin must be of type ast.Builtin")
	}
	bfunc.Builtin = builtin

	// type assert function symbol
	switch fnc := functionSym.(type) {
	case *topdown.BuiltinFunc, *topdown.FunctionalBuiltin1, *topdown.FunctionalBuiltin2, *topdown.FunctionalBuiltin3:
		bfunc.Function = fnc
	default:
		return nil, fmt.Errorf("symbol Function was of an unrecognized type")
	}

	return
}

func loadKnownTypes(path string, bs []byte) (interface{}, error) {
	switch filepath.Ext(path) {
	case ".json":
		return loadJSON(path, bs)
	case ".rego":
		return Rego(path)
	case ".yaml", ".yml":
		return loadYAML(path, bs)
	case ".so":
		if ok, _ := filepath.Match("*.builtin.so", filepath.Base(path)); ok {
			return Builtin(path)
		}
	}
	return nil, unrecognizedFile(path)
}

func loadFileForAnyType(path string, bs []byte) (interface{}, error) {
	module, err := loadRego(path, bs)
	if err == nil {
		return module, nil
	}
	doc, err := loadJSON(path, bs)
	if err == nil {
		return doc, nil
	}
	doc, err = loadYAML(path, bs)
	if err == nil {
		return doc, nil
	}
	builtin, err := Builtin(path)
	if err == nil {
		return builtin, nil
	}
	return nil, unrecognizedFile(path)
}
