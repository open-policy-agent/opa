// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package loader contains utilities for loading files into OPA.
package loader

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/ghodss/yaml"
	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/bundle"
	"github.com/open-policy-agent/opa/util"
	"github.com/pkg/errors"
)

// Result represents the result of successfully loading zero or more files.
type Result struct {
	Documents map[string]interface{}
	Modules   map[string]*RegoFile
	path      []string
}

// RegoFile represents the result of loading a single Rego source file.
type RegoFile struct {
	Name   string
	Parsed *ast.Module
	Raw    []byte
}

// All returns a Result object loaded (recursively) from the specified paths.
func All(paths []string) (*Result, error) {
	return all(paths, func(curr *Result, path string) error {
		result, err := loadFile(path)
		if err != nil {
			return err
		}
		return curr.merge(path, result)
	})
}

// AllRegos returns a Result object loaded (recursively) with all Rego source
// files from the specified paths.
func AllRegos(paths []string) (*Result, error) {
	return all(paths, func(curr *Result, path string) error {
		if !strings.HasSuffix(path, bundle.RegoExt) {
			return nil
		}
		result, err := Rego(path)
		if err != nil {
			return err
		}
		return curr.merge(path, result)
	})
}

// Rego returns a RegoFile object loaded from the given path.
func Rego(path string) (*RegoFile, error) {
	bs, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return loadRego(path, bs)
}

// CleanPath returns the normalized version of a path that can be used as an identifier.
func CleanPath(path string) string {
	return strings.Trim(path, "/")
}

// Paths returns a sorted list of files contained at path. If recurse is true
// and path is a directory, then Paths will walk the directory structure
// recursively and list files at each level.
func Paths(path string, recurse bool) (paths []string, err error) {
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

// SplitPrefix returns a tuple specifying the document prefix and the file
// path.
func SplitPrefix(path string) ([]string, string) {
	parts := strings.SplitN(path, ":", 2)
	if len(parts) == 2 && len(parts[0]) > 0 {
		return strings.Split(parts[0], "."), parts[1]
	}
	return nil, path
}

func (l *Result) merge(path string, result interface{}) error {
	switch result := result.(type) {
	case *RegoFile:
		l.Modules[CleanPath(path)] = result
	default:
		obj, ok := makeDir(l.path, result)
		if !ok {
			return unsupportedDocumentType(path)
		}
		merged, ok := mergeDocs(l.Documents, obj)
		if !ok {
			return mergeError(path)
		}
		for k := range merged {
			l.Documents[k] = merged[k]
		}
	}
	return nil
}

func (l *Result) withParent(p string) *Result {
	path := append(l.path, p)
	return &Result{
		Documents: l.Documents,
		Modules:   l.Modules,
		path:      path,
	}
}

func all(paths []string, f func(*Result, string) error) (*Result, error) {
	errors := loaderErrors{}
	root := newResult()

	for _, path := range paths {

		loaded := root
		prefix, path := SplitPrefix(path)
		if len(prefix) > 0 {
			for _, part := range prefix {
				loaded = loaded.withParent(part)
			}
		}

		info, err := os.Stat(path)
		if err != nil {
			errors.Add(err)
			continue
		}

		if info.IsDir() {
			loadDirRecursive(&errors, path, loaded)
		} else {
			err := f(loaded, path)
			if err != nil {
				errors.Add(err)
			}
		}

	}

	if len(errors) > 0 {
		return nil, errors
	}

	return root, nil
}
func loadDirRecursive(errors *loaderErrors, dirPath string, loaded *Result) {
	files, err := ioutil.ReadDir(dirPath)
	if err != nil {
		errors.Add(err)
		return
	}
	for _, file := range files {
		filePath := filepath.Join(dirPath, file.Name())
		info, err := os.Stat(filePath)
		if err != nil {
			errors.Add(err)
		} else {
			if info.IsDir() {
				loadDirRecursive(errors, filePath, loaded.withParent(info.Name()))
			} else {
				bs, err := ioutil.ReadFile(filePath)
				if err != nil {
					errors.Add(err)
				} else {
					result, err := loadKnownTypes(filePath, bs)
					if err != nil {
						if _, ok := err.(unrecognizedFile); !ok {
							errors.Add(err)
						}
					} else {
						if err := loaded.merge(filePath, result); err != nil {
							errors.Add(err)
						}
					}

				}
			}
		}
	}
}

func loadKnownTypes(path string, bs []byte) (interface{}, error) {
	switch filepath.Ext(path) {
	case ".json":
		return loadJSON(path, bs)
	case ".rego":
		return Rego(path)
	case ".yaml", ".yml":
		return loadYAML(path, bs)
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

func loadFile(path string) (interface{}, error) {
	bs, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	result, err := loadKnownTypes(path, bs)
	if err != nil {
		if isUnrecognizedFile(err) {
			return loadFileForAnyType(path, bs)
		}
		return nil, err
	}
	return result, nil
}

func loadRego(path string, bs []byte) (*RegoFile, error) {
	module, err := ast.ParseModule(path, string(bs))
	if err != nil {
		return nil, err
	}
	if module == nil {
		return nil, emptyModuleError(path)
	}
	result := &RegoFile{
		Name:   path,
		Parsed: module,
		Raw:    bs,
	}
	return result, nil
}

func loadJSON(path string, bs []byte) (interface{}, error) {
	buf := bytes.NewBuffer(bs)
	decoder := util.NewJSONDecoder(buf)
	var x interface{}
	if err := decoder.Decode(&x); err != nil {
		return nil, errors.Wrap(err, path)
	}
	return x, nil
}

func loadYAML(path string, bs []byte) (interface{}, error) {
	bs, err := yaml.YAMLToJSON(bs)
	if err != nil {
		return nil, fmt.Errorf("%v: error converting YAML to JSON: %v", path, err)
	}
	return loadJSON(path, bs)
}

func makeDir(path []string, x interface{}) (map[string]interface{}, bool) {
	if len(path) == 0 {
		obj, ok := x.(map[string]interface{})
		if !ok {
			return nil, false
		}
		return obj, true
	}
	return makeDir(path[:len(path)-1], map[string]interface{}{path[len(path)-1]: x})
}
