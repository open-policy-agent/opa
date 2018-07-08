// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// +build linux,cgo darwin,cgo

// Contains test cases that use the plugin loader
package loader

import (
	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/types"
	"github.com/open-policy-agent/opa/util/test"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

const (
	testBuiltin = `
package main

import (
	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/types"
	"github.com/open-policy-agent/opa/topdown"
)

var Builtin = ast.Builtin{
	Name: "equals",
	Decl: types.NewFunction(
		types.Args(types.N, types.N),
		types.B,
	),
}

var Function topdown.FunctionalBuiltin2 = func(a, b ast.Value) (ast.Value, error) {
	return ast.Boolean(true), nil
}
`
)

func TestLoadBuiltin(t *testing.T) {

	files := map[string]string{
		"/equals.go": testBuiltin,
	}

	root, cleanup := makeDirWithBuiltin(files)
	defer cleanup()

	sharedObjectFile := filepath.Join(root, "equals.builtin.so")
	loaded, err := All([]string{sharedObjectFile})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expectedBuiltin := ast.Builtin{
		Name: "equals",
		Decl: types.NewFunction(
			types.Args(types.N, types.N),
			types.B,
		),
	}

	actual := loaded.BuiltinFuncs[sharedObjectFile]
	if !reflect.DeepEqual(*actual.Builtin, expectedBuiltin) {
		t.Fatalf("Expected builtin %v but got: %v", expectedBuiltin, *actual.Builtin)
	}
}

func TestLoadDirRecursivePlugin(t *testing.T) {

	files := map[string]string{
		"/a/data1.json":  `{"a": [1,2,3]}`,
		"/a/e.rego":      `package q`,
		"/b/data2.yaml":  `{"aaa": {"bbb": 1}}`,
		"/b/equals.go":   testBuiltin,
		"/b/data3.yaml":  `{"aaa": {"ccc": 2}}`,
		"/b/d/x.json":    "null",
		"/b/d/e.rego":    `package p`,
		"/b/d/ignore":    `deadbeef`,
		"/b/d/equals.go": testBuiltin,
		"/foo":           `{"zzz": "b"}`,
	}

	rootDir, cleanup := makeDirWithBuiltin(files)
	defer cleanup()

	loaded, err := All(mustListPaths(rootDir, false)[1:])
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	expectedDocuments := parseJSON(`
		{
			"zzz": "b",
			"a": [1,2,3],
			"aaa": {
				"bbb": 1,
				"ccc": 2
			},
			"d": null
		}
		`)
	if !reflect.DeepEqual(loaded.Documents, expectedDocuments) {
		t.Fatalf("Expected:\n%v\n\nGot:\n%v", expectedDocuments, loaded.Documents)
	}
	mod1 := ast.MustParseModule(files["/a/e.rego"])
	mod2 := ast.MustParseModule(files["/b/d/e.rego"])
	expectedMod1 := loaded.Modules[CleanPath(filepath.Join(rootDir, "/a/e.rego"))].Parsed
	expectedMod2 := loaded.Modules[CleanPath(filepath.Join(rootDir, "/b/d/e.rego"))].Parsed
	if !mod1.Equal(expectedMod1) {
		t.Fatalf("Expected:\n%v\n\nGot:\n%v", expectedMod1, mod1)
	}
	if !mod2.Equal(expectedMod2) {
		t.Fatalf("Expected:\n%v\n\nGot:\n%v", expectedMod2, mod2)
	}

	so1 := CleanPath(filepath.Join(rootDir, "/b/d/equals.builtin.so"))
	so2 := CleanPath(filepath.Join(rootDir, "/b/equals.builtin.so"))
	expectedBuiltin := ast.Builtin{
		Name: "equals",
		Decl: types.NewFunction(
			types.Args(types.N, types.N),
			types.B,
		),
	}
	actual1 := loaded.BuiltinFuncs[so1]
	if !reflect.DeepEqual(*actual1.Builtin, expectedBuiltin) {
		t.Fatalf("Expected builtin %v but got: %v", expectedBuiltin, *actual1.Builtin)
	}
	actual2 := loaded.BuiltinFuncs[so2]
	if !reflect.DeepEqual(*actual2.Builtin, expectedBuiltin) {
		t.Fatalf("Expected builtin %v but got: %v", expectedBuiltin, *actual2.Builtin)
	}
}

func TestLoadErrorsPlugin(t *testing.T) {

	noFunction := `
package main

import (
	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/types"
)

var Builtin = ast.Builtin{
	Name: "equals",
	Decl: types.NewFunction(
		types.Args(types.N, types.N),
		types.B,
	),
}`
	noBuiltin := `
package main

import (
	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/topdown"
)

var Function topdown.FunctionalBuiltin2 = func(a, b ast.Value) (ast.Value, error) {
	return ast.Boolean(true), nil
}
`

	files := map[string]string{
		"/x1.json":    `{"x": [1,2,3]}`,
		"/x2.json":    `{"x": {"y": 1}}`,
		"/empty.rego": `   `,
		"/dir/a.json": ``,
		"/dir/b.yaml": `
		foo:
		  - bar:
		`,
		"/bad_doc.json":   "[1,2,3]",
		"/no_function.go": noFunction,
		"/no_builtin.go":  noBuiltin,
	}

	rootDir, cleanup := makeDirWithBuiltin(files)
	defer cleanup()
	paths := mustListPaths(rootDir, false)[1:]
	sort.Strings(paths)
	_, err := All(paths)
	if err == nil {
		t.Fatalf("Expected failure")
	}

	expected := []string{
		"bad_doc.json: bad document type",
		"a.json: EOF",
		"b.yaml: error converting YAML to JSON",
		"empty.rego: empty policy",
		"x2.json: merge error",
		"no_builtin.builtin.so: plugin: symbol Builtin not found",
		"no_function.builtin.so: plugin: symbol Function not found",
	}

	for _, s := range expected {
		if !strings.Contains(err.Error(), s) {
			t.Fatalf("Expected error to contain %v but got:\n%v", s, err)
		}
	}
}

// makeDirWithBuiltin creates a new temporary directory containing files.
// it compiles all .go files into identically named .builtin.so files in the corresponding directory
// It returns the root of the directory and a cleanup function.
func makeDirWithBuiltin(files map[string]string) (root string, cleanup func()) {
	root, cleanup, err := test.MakeTempFS("./", "loader_test", files)
	if err != nil {
		panic(err)
	}
	for file := range files {
		if filepath.Ext(file) == ".go" {
			src := filepath.Join(root, file)
			so := strings.TrimSuffix(filepath.Base(src), ".go") + ".builtin.so"
			out := filepath.Join(filepath.Dir(src), so)
			// build latest version of shared object
			cmd := exec.Command("go", "build", "-buildmode=plugin", "-o="+out, src)
			res, err := cmd.Output()
			if err != nil {
				panic(string(res) + err.Error())
			}
		}
	}
	return
}
