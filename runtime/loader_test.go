// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package runtime

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/ghodss/yaml"
	"github.com/open-policy-agent/opa/ast"
)

func TestLoadJSON(t *testing.T) {

	files := map[string]string{
		"/foo.json": `{"a": [1,2,3]}`,
	}

	withTempFS(files, func(rootDir string) {

		loaded, err := loadAllPaths([]string{filepath.Join(rootDir, "foo.json")})

		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		expected := parseJSON(files["/foo.json"])

		if !reflect.DeepEqual(loaded.Documents, expected) {
			t.Fatalf("Expected %v but got: %v", expected, loaded.Documents)
		}
	})
}

func TestLoadRego(t *testing.T) {

	files := map[string]string{
		"/foo.rego": `package ex
        p :- true`,
	}

	withTempFS(files, func(rootDir string) {
		moduleFile := filepath.Join(rootDir, "foo.rego")
		loaded, err := loadAllPaths([]string{moduleFile})
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		expected := ast.MustParseModule(files["/foo.rego"])
		if !expected.Equal(loaded.Modules[moduleFile].Parsed) {
			t.Fatalf("Expected:\n%v\n\nGot:\n%v", expected, loaded.Modules[moduleFile])
		}
	})
}

func TestLoadYAML(t *testing.T) {

	files := map[string]string{
		"/foo.yml": `
        a:
            - 1
            - b
            - "c"
            - null
            - true
            - false
        `,
	}

	withTempFS(files, func(rootDir string) {
		yamlFile := filepath.Join(rootDir, "foo.yml")
		loaded, err := loadAllPaths([]string{yamlFile})
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		expected := parseJSON(`
        {"a": [1, "b", "c", null, true, false]}`)
		if !reflect.DeepEqual(loaded.Documents, expected) {
			t.Fatalf("Expected %v but got: %v", expected, loaded.Documents)
		}
	})
}

func TestLoadGuessYAML(t *testing.T) {
	files := map[string]string{
		"/foo": `
        a: b
        `,
	}
	withTempFS(files, func(rootDir string) {
		yamlFile := filepath.Join(rootDir, "foo")
		loaded, err := loadAllPaths([]string{yamlFile})
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		expected := parseJSON(`{"a": "b"}`)
		if !reflect.DeepEqual(loaded.Documents, expected) {
			t.Fatalf("Expected %v but got: %v", expected, loaded.Documents)
		}
	})
}

func withTempFS(files map[string]string, f func(string)) {
	rootDir, cleanup, err := makeTempFS(files)
	if err != nil {
		panic(err)
	}
	defer cleanup()
	f(rootDir)
}

// makeTempFS creates a temporary directory structure for test purposes. If the
// creation fails, cleanup is nil and the caller does not have to invoke it. If
// creation succeeds, the caller should invoke cleanup when they are done.
func makeTempFS(files map[string]string) (rootDir string, cleanup func(), err error) {

	rootDir, err = ioutil.TempDir("", "")

	if err != nil {
		return "", nil, err
	}

	cleanup = func() {
		os.RemoveAll(rootDir)
	}

	skipCleanup := false

	// Cleanup unless flag is unset. It will be unset if we succeed.
	defer func() {
		if !skipCleanup {
			cleanup()
		}
	}()

	for path, content := range files {
		dirname, filename := filepath.Split(path)
		dirPath := filepath.Join(rootDir, dirname)
		if err := os.MkdirAll(dirPath, 777); err != nil {
			return "", nil, err
		}

		f, err := os.Create(filepath.Join(dirPath, filename))
		if err != nil {
			return "", nil, err
		}

		if _, err := f.WriteString(content); err != nil {
			return "", nil, err
		}
	}

	skipCleanup = true

	return rootDir, cleanup, nil
}

func parseYAML(s string) interface{} {
	var x interface{}
	if err := yaml.Unmarshal([]byte(s), &x); err != nil {
		panic(err)
	}
	return x
}
