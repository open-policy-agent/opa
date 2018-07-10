// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// +build !linux,!darwin !cgo

// Builds version of the loader that cannot read plugins
package loader

import (
	"fmt"
	"path/filepath"
)

func loadKnownTypes(path string, bs []byte) (interface{}, error) {
	switch filepath.Ext(path) {
	case ".json":
		return loadJSON(path, bs)
	case ".rego":
		return Rego(path)
	case ".yaml", ".yml":
		return loadYAML(path, bs)
	case ".so":
		return nil, fmt.Errorf(".so files only supported on linux/darwin + cgo builds")
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
	return nil, unrecognizedFile(path)
}
