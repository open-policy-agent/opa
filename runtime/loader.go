// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package runtime

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/ghodss/yaml"
	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/util"
	"github.com/pkg/errors"
)

type loaderErrors []error

func (e loaderErrors) Error() string {
	if len(e) == 0 {
		return "no error(s)"
	}
	if len(e) == 1 {
		return "1 error occurred during loading: " + e[0].Error()
	}
	buf := make([]string, len(e))
	for i := range buf {
		buf[i] = e[i].Error()
	}
	return fmt.Sprintf("%v errors occured during loading:\n", len(e)) + strings.Join(buf, "\n")
}

func (e *loaderErrors) Add(err error) {
	*e = append(*e, err)
}

type loaded struct {
	Documents map[string]interface{}
	Modules   map[string]*LoadedModule
	path      []string
}

// LoadedModule represents a module that has been successfully loaded.
type LoadedModule struct {
	Parsed *ast.Module
	Raw    []byte
}

func newLoaded() *loaded {
	return &loaded{
		Documents: map[string]interface{}{},
		Modules:   map[string]*LoadedModule{},
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

func (path unsupportedDocumentType) Error() string {
	return string(path) + ": bad document type"
}

type unrecognizedFile string

func (path unrecognizedFile) Error() string {
	return string(path) + ": can't recognize file type"
}

func isUnrecognizedFile(err error) bool {
	_, ok := err.(unrecognizedFile)
	return ok
}

type mergeError string

func (e mergeError) Error() string {
	return string(e) + ": merge error"
}

type emptyModuleError string

func (e emptyModuleError) Error() string {
	return string(e) + ": empty policy"
}

func (l *loaded) Merge(path string, result interface{}) error {
	switch result := result.(type) {
	case *LoadedModule:
		l.Modules[normalizeModuleID(path)] = result
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

func loadAllPaths(paths []string) (*loaded, error) {

	root := newLoaded()
	errors := loaderErrors{}

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
			errors.Add(err)
			continue
		}

		if info.IsDir() {
			loadDirRecursive(&errors, path, loaded.WithParent(info.Name()))
		} else {
			result, err := loadFile(path)
			if err != nil {
				errors.Add(err)
			} else {
				if err := loaded.Merge(path, result); err != nil {
					errors.Add(err)
				}
			}
		}
	}

	if len(errors) > 0 {
		return nil, errors
	}

	return root, nil
}

func loadDirRecursive(errors *loaderErrors, dirPath string, loaded *loaded) {
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
				loadDirRecursive(errors, filePath, loaded.WithParent(info.Name()))
			} else {
				result, err := loadFileForKnownTypes(filePath)
				if err != nil {
					if _, ok := err.(unrecognizedFile); !ok {
						errors.Add(err)
					}
				} else {
					if err := loaded.Merge(filePath, result); err != nil {
						errors.Add(err)
					}
				}
			}
		}
	}
}

func loadFileForKnownTypes(path string) (interface{}, error) {
	switch filepath.Ext(path) {
	case ".json":
		return jsonLoad(path)
	case ".rego":
		return RegoLoad(path)
	case ".yaml", ".yml":
		return yamlLoad(path)
	}
	return nil, unrecognizedFile(path)
}

func loadFileForAnyType(path string) (interface{}, error) {
	module, err := RegoLoad(path)
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
	decoder := util.NewJSONDecoder(f)
	var x interface{}
	if err = decoder.Decode(&x); err != nil {
		return nil, errors.Wrapf(err, path)
	}
	return x, nil
}

// RegoLoad loads and parses a rego source file.
func RegoLoad(path string) (*LoadedModule, error) {
	bs, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	module, err := ast.ParseModule(path, string(bs))
	if err != nil {
		return nil, err
	}
	if module == nil {
		return nil, emptyModuleError(path)
	}
	result := &LoadedModule{
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
	if err := unmarshalYAML(bs, &x); err != nil {
		return nil, errors.Wrapf(err, path)
	}
	return x, nil
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

func normalizeModuleID(x string) string {
	return strings.Trim(x, "/")
}

func splitPathPrefix(path string) ([]string, string) {
	parts := strings.SplitN(path, ":", 2)
	if len(parts) == 2 && len(parts[0]) > 0 {
		return strings.Split(parts[0], "."), parts[1]
	}
	return nil, path
}

// unmarshalYAML re-implements yaml.Unmarshal so that the JSON decoder can have
// UseNumber set.
func unmarshalYAML(y []byte, o interface{}) error {
	bs, err := yaml.YAMLToJSON(y)
	if err != nil {
		return fmt.Errorf("error converting YAML to JSON: %v", err)
	}
	buf := bytes.NewBuffer(bs)
	decoder := util.NewJSONDecoder(buf)
	return decoder.Decode(o)
}
