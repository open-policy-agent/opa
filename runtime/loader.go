// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package runtime

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/ghodss/yaml"
	"github.com/open-policy-agent/opa/ast"
	"github.com/pkg/errors"
)

type loaded struct {
	Documents map[string]interface{}
	Modules   map[string]*loadedModule
}

type loadedModule struct {
	Parsed *ast.Module
	Raw    []byte
}

func newLoaded() *loaded {
	return &loaded{
		Documents: map[string]interface{}{},
		Modules:   map[string]*loadedModule{},
	}
}

func loadAllPaths(paths []string) (*loaded, error) {

	loaded := newLoaded()

	for _, path := range paths {

		result, err := loadFile(path)
		if err != nil {
			return nil, err
		}

		switch result := result.(type) {
		case *loadedModule:
			loaded.Modules[path] = result
		case map[string]interface{}:
			loaded.Documents, err = mergeDocs(loaded.Documents, result)
			if err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("unsupported document type %T", result)
		}
	}

	return loaded, nil
}

func loadFile(path string) (interface{}, error) {
	switch filepath.Ext(path) {
	case ".json":
		return jsonLoad(path)
	case ".rego":
		return regoLoad(path)
	case ".yaml", ".yml":
		return yamlLoad(path)
	default:
		return guessLoad(path)
	}
}

func guessLoad(path string) (interface{}, error) {
	module, err := regoLoad(path)
	if err == nil {
		return module, nil
	}
	doc, err := jsonLoad(path)
	if err == nil {
		return doc, nil
	}
	doc, err = yamlLoad(path)
	if err == nil {
		return doc, nil
	}
	return nil, fmt.Errorf("unrecognized file: %v", path)
}

func jsonLoad(path string) (interface{}, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, errors.Wrapf(err, path)
	}
	defer f.Close()
	decoder := json.NewDecoder(f)
	var x interface{}
	return x, decoder.Decode(&x)
}

func regoLoad(path string) (interface{}, error) {
	bs, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	module, err := ast.ParseModule(path, string(bs))
	if err != nil {
		return nil, err
	}
	result := &loadedModule{
		Parsed: module,
		Raw:    bs,
	}
	return result, nil
}

func yamlLoad(path string) (interface{}, error) {
	bs, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var x interface{}
	if err := yaml.Unmarshal(bs, &x); err != nil {
		return nil, err
	}
	return x, nil
}
