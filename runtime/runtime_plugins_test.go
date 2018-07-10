// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// +build linux,cgo darwin,cgo

package runtime

import (
	"fmt"
	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/types"
	"github.com/open-policy-agent/opa/util/test"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func makeBuiltinWithName(name string) string {
	return fmt.Sprintf(`
package main

import (
	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/types"
	"github.com/open-policy-agent/opa/topdown"
)

var Builtin = ast.Builtin{
	Name: "%v",
	Decl: types.NewFunction(
		types.Args(types.N, types.N),
		types.B,
	),
}

var Function topdown.FunctionalBuiltin2 = func(a, b ast.Value) (ast.Value, error) {
	return ast.Boolean(true), nil
}
`, name)
}

func TestRegisterBuiltinSingle(t *testing.T) {

	name := "builtinsingle"
	files := map[string]string{
		"/dir/equals.go": makeBuiltinWithName(name),
	}

	root, cleanup := makeDirWithBuiltin(files)
	defer cleanup()

	builtinDir := filepath.Join(root, "/dir")
	err := RegisterBuiltinsFromDir(builtinDir)
	if err != nil {
		t.Fatalf(err.Error())
	}

	expected := &ast.Builtin{
		Name: name,
		Decl: types.NewFunction(
			types.Args(types.N, types.N),
			types.B,
		),
	}

	// check that builtin function was loaded correctly
	actual := ast.BuiltinMap[name]
	if !reflect.DeepEqual(*expected, *actual) {
		t.Fatalf("Expected builtin %v but got: %v", *expected, *actual)
	}
}

func TestRegisterBuiltinRecursive(t *testing.T) {
	names := []string{"shallow", "parallel", "deep", "deeper"}
	ignored := []string{"ignore", "ignore2"}
	files := map[string]string{
		"/dir/shallow.go":            makeBuiltinWithName("shallow"),
		"/dir/parallel.go":           makeBuiltinWithName("parallel"),
		"/dir/deep/deep.go":          makeBuiltinWithName("deep"),
		"/dir/deep/deeper/deeper.go": makeBuiltinWithName("deeper"),
		"/other/ignore.go":           makeBuiltinWithName("ignore"),
		"/ignore2.go":                makeBuiltinWithName("ignore2"),
	}

	root, cleanup := makeDirWithBuiltin(files)
	defer cleanup()

	builtinDir := filepath.Join(root, "/dir")
	err := RegisterBuiltinsFromDir(builtinDir)
	if err != nil {
		t.Fatalf(err.Error())
	}

	expectedDecl := types.NewFunction(
		types.Args(types.N, types.N),
		types.B,
	)

	// check that every builtin is present
	for _, name := range names {
		actual, ok := ast.BuiltinMap[name]
		if !ok {
			t.Fatalf("builtin %v not present", name)
		}
		if actual.Name != name {
			t.Fatalf("builtin %v has incorrect name %v", name, actual.Name)
		}
		if !reflect.DeepEqual(actual.Decl, expectedDecl) {
			t.Fatalf("Expected builtin %v but got: %v", *expectedDecl, *actual.Decl)
		}
	}

	// check that ignore is absent
	for _, ignore := range ignored {
		_, ok := ast.BuiltinMap[ignore]
		if ok {
			t.Fatalf("builtin %v incorrectly added", ignore)
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
