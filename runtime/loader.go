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

	"strings"

	"github.com/ghodss/yaml"
	"github.com/open-policy-agent/opa/ast"
	"github.com/pkg/errors"
)

type loaded struct {
	Documents map[string]interface{}
	Modules   map[string]*loadedModule
	path      []string
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

func (l *loaded) WithParent(p string) *loaded {
	path := append(l.path, p)
	return &loaded{
		Documents: l.Documents,
		Modules:   l.Modules,
		path:      path,
	}
}

type unsupportedDocumentType string

func (u unsupportedDocumentType) Error() string {
	return "unsupported document type: " + string(u)
}

type unrecognizedFile string

func (u unrecognizedFile) Error() string {
	return "unrecognized file: " + string(u)
}

func isUnrecognizedFile(err error) bool {
	_, ok := err.(unrecognizedFile)
	return ok
}

func (l *loaded) Merge(path string, result interface{}) error {
	switch result := result.(type) {
	case *loadedModule:
		l.Modules[path] = result
	default:
		obj, err := makeDir(l.path, result)
		if err != nil {
			return err
		}
		merged, err := mergeDocs(l.Documents, obj)
		if err != nil {
			return err
		}
		for k := range merged {
			l.Documents[k] = merged[k]
		}
	}
	return nil
}

func loadAllPaths(paths []string) (*loaded, error) {

	root := newLoaded()

	for _, path := range paths {

		loaded := root
		prefix, path := splitPathPrefix(path)
		if len(prefix) > 0 {
			for _, part := range prefix {
				loaded = loaded.WithParent(part)
			}
		}

		info, err := os.Stat(path)
		if err != nil {
			return nil, err
		}

		if info.IsDir() {
			if err := loadDirRecursive(path, loaded.WithParent(info.Name())); err != nil {
				return nil, err
			}
		} else {
			result, err := loadFile(path)
			if err != nil {
				return nil, err
			}
			if err := loaded.Merge(path, result); err != nil {
				return nil, err
			}
		}
	}

	return root, nil
}

func loadDirRecursive(dirPath string, loaded *loaded) error {
	files, err := ioutil.ReadDir(dirPath)
	if err != nil {
		return err
	}
	for _, file := range files {
		filePath := filepath.Join(dirPath, file.Name())
		info, err := os.Stat(filePath)
		if err != nil {
			return err
		}
		if info.IsDir() {
			if err := loadDirRecursive(filePath, loaded.WithParent(info.Name())); err != nil {
				return err
			}
		} else {
			result, err := loadFileForKnownTypes(filePath)
			if err != nil {
				if _, ok := err.(unrecognizedFile); !ok {
					return err
				}
			} else {
				if err := loaded.Merge(filePath, result); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func loadFileForKnownTypes(path string) (interface{}, error) {
	switch filepath.Ext(path) {
	case ".json":
		return jsonLoad(path)
	case ".rego":
		return regoLoad(path)
	case ".yaml", ".yml":
		return yamlLoad(path)
	}
	return nil, unrecognizedFile(path)
}

func loadFileForAnyType(path string) (interface{}, error) {
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
	return nil, unrecognizedFile(path)
}

func loadFile(path string) (interface{}, error) {
	result, err := loadFileForKnownTypes(path)
	if err != nil {
		if isUnrecognizedFile(err) {
			return loadFileForAnyType(path)
		}
		return nil, err
	}
	return result, nil
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
		return nil, errors.Wrapf(err, path)
	}
	return x, nil
}

func makeDir(path []string, x interface{}) (map[string]interface{}, error) {
	if len(path) == 0 {
		obj, ok := x.(map[string]interface{})
		if !ok {
			return nil, unsupportedDocumentType(fmt.Sprintf("%T", x))
		}
		return obj, nil
	}
	return makeDir(path[:len(path)-1], map[string]interface{}{path[len(path)-1]: x})
}

func splitPathPrefix(path string) ([]string, string) {
	parts := strings.SplitN(path, ":", 2)
	if len(parts) == 2 && len(parts[0]) > 0 {
		return strings.Split(parts[0], "."), parts[1]
	}
	return nil, path
}
