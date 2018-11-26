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
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/storage/inmem"
	"github.com/open-policy-agent/opa/util"
	"github.com/pkg/errors"
)

// Result represents the result of successfully loading zero or more files.
type Result struct {
	Documents map[string]interface{}
	Modules   map[string]*RegoFile
	path      []string
}

// ParsedModules returns the parsed modules stored on the result.
func (l *Result) ParsedModules() map[string]*ast.Module {
	modules := make(map[string]*ast.Module)
	for _, module := range l.Modules {
		modules[module.Name] = module.Parsed
	}
	return modules
}

// Compiler returns a Compiler object with the compiled modules from this loader
// result.
func (l *Result) Compiler() (*ast.Compiler, error) {
	compiler := ast.NewCompiler()
	compiler.Compile(l.ParsedModules())
	if compiler.Failed() {
		return nil, compiler.Errors
	}
	return compiler, nil
}

// Store returns a Store object with the documents from this loader result.
func (l *Result) Store() (storage.Store, error) {
	return inmem.NewFromObject(l.Documents), nil
}

// RegoFile represents the result of loading a single Rego source file.
type RegoFile struct {
	Name   string
	Parsed *ast.Module
	Raw    []byte
}

// Filter defines the interface for filtering files during loading. If the
// filter returns true, the file should be excluded from the result.
type Filter func(abspath string, info os.FileInfo, depth int) bool

// GlobExcludeName excludes files and directories whose names do not match the
// shell style pattern at minDepth or greater.
func GlobExcludeName(pattern string, minDepth int) Filter {
	return func(abspath string, info os.FileInfo, depth int) bool {
		match, _ := filepath.Match(pattern, info.Name())
		return match && depth >= minDepth
	}
}

// All returns a Result object loaded (recursively) from the specified paths.
func All(paths []string) (*Result, error) {
	return Filtered(paths, nil)
}

// AllRegos returns a Result object loaded (recursively) with all Rego source
// files from the specified paths.
func AllRegos(paths []string) (*Result, error) {
	return Filtered(paths, func(_ string, info os.FileInfo, depth int) bool {
		return !info.IsDir() && !strings.HasSuffix(info.Name(), bundle.RegoExt)
	})
}

// Filtered returns a Result object loaded (recursively) from the specified
// paths while applying the given filters. If any filter returns true, the
// file/directory is excluded.
func Filtered(paths []string, filter Filter) (*Result, error) {
	return all(paths, filter, func(curr *Result, path string, depth int) error {

		bs, err := ioutil.ReadFile(path)
		if err != nil {
			return err
		}

		result, err := loadKnownTypes(path, bs)
		if err != nil {
			if !isUnrecognizedFile(err) {
				return err
			}
			if depth > 0 {
				return nil
			}
			result, err = loadFileForAnyType(path, bs)
			if err != nil {
				return err
			}
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
	case bundle.Bundle:
		for _, module := range result.Modules {
			l.Modules[module.Path] = &RegoFile{
				Name:   module.Path,
				Parsed: module.Parsed,
				Raw:    module.Raw,
			}
		}
		return l.mergeDocument(path, result.Data)
	case *RegoFile:
		l.Modules[CleanPath(path)] = result
		return nil
	default:
		return l.mergeDocument(path, result)
	}
}

func (l *Result) mergeDocument(path string, doc interface{}) error {
	obj, ok := makeDir(l.path, doc)
	if !ok {
		return unsupportedDocumentType(path)
	}
	merged, ok := mergeInterfaces(l.Documents, obj)
	if !ok {
		return mergeError(path)
	}
	for k := range merged {
		l.Documents[k] = merged[k]
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

func all(paths []string, filter Filter, f func(*Result, string, int) error) (*Result, error) {
	errors := loaderErrors{}
	root := newResult()

	for _, path := range paths {

		// Paths can be prefixed with a string that specifies where content should be
		// loaded under data. E.g., foo.bar:/path/to/some.json will load the content
		// of some.json under {"foo": {"bar": ...}}.
		loaded := root
		prefix, path := SplitPrefix(path)
		if len(prefix) > 0 {
			for _, part := range prefix {
				loaded = loaded.withParent(part)
			}
		}

		allRec(path, filter, &errors, loaded, 0, f)
	}

	if len(errors) > 0 {
		return nil, errors
	}

	return root, nil
}

func allRec(path string, filter Filter, errors *loaderErrors, loaded *Result, depth int, f func(*Result, string, int) error) {
	info, err := os.Stat(path)
	if err != nil {
		errors.Add(err)
		return
	}

	if filter != nil && filter(path, info, depth) {
		return
	}

	if !info.IsDir() {
		if err := f(loaded, path, depth); err != nil {
			errors.Add(err)
		}
		return
	}

	// If we are recursing on directories then content must be loaded under path
	// speciifed by directory hierarchy.
	if depth > 0 {
		loaded = loaded.withParent(info.Name())
	}

	files, err := ioutil.ReadDir(path)
	if err != nil {
		errors.Add(err)
		return
	}

	for _, file := range files {
		allRec(filepath.Join(path, file.Name()), filter, errors, loaded, depth+1, f)
	}
}

func exclude(filters []Filter, path string, info os.FileInfo, depth int) bool {
	for _, f := range filters {
		if f(path, info, depth) {
			return true
		}
	}
	return false
}

func loadKnownTypes(path string, bs []byte) (interface{}, error) {
	switch filepath.Ext(path) {
	case ".json":
		return loadJSON(path, bs)
	case ".rego":
		return Rego(path)
	case ".yaml", ".yml":
		return loadYAML(path, bs)
	default:
		if strings.HasSuffix(path, ".tar.gz") {
			return loadBundle(bs)
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
	return nil, unrecognizedFile(path)
}

func loadBundle(bs []byte) (bundle.Bundle, error) {
	br := bundle.NewReader(bytes.NewBuffer(bs)).IncludeManifestInData(true)
	return br.Read()
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
