// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// +build linux,cgo darwin,cgo

package runtime

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/types"
	"github.com/open-policy-agent/opa/util/test"
	"net/http"
	"net/http/httptest"
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

// returns go code for a plugin named name that makes a single get request to url upon start
func getPluginWithNameAndURL(name, url string) string {
	return fmt.Sprintf(
		`
package main

import (
	"context"
    "net/http"
	"github.com/open-policy-agent/opa/plugins"
)

var Name = "%v"

type Tester struct {}

func (t *Tester) Start(ctx context.Context) error {
    _, err := http.Get("%v")
	return err
}

func (t *Tester) Stop(ctx context.Context) {
	return
}

var Initializer plugins.PluginInitFunc = func(m *plugins.Manager, config []byte) (plugins.Plugin, error) {
	return &Tester{}, nil
}`, name, url)
}

func TestRegisterBuiltinSingle(t *testing.T) {

	name := "builtinsingle"
	files := map[string]string{
		"/dir/equals.go": makeBuiltinWithName(name),
	}

	root, cleanup := makeDirWithSharedObjects(files, ".builtin.so")
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

	root, cleanup := makeDirWithSharedObjects(files, ".builtin.so")
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

// Tests that a plugins starts only once correctly
func TestRegisterPluginSingle(t *testing.T) {
	name := "test"

	ch := make(chan struct{}, 10)

	// server upon request sends item to channel
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		ch <- struct{}{}
	}))
	defer ts.Close()

	config := `
plugins:
 test: test-message
`

	files := map[string]string{
		"/dir/test.go": getPluginWithNameAndURL(name, ts.URL),
		"/config.yaml": config,
	}

	root, cleanup := makeDirWithSharedObjects(files, ".plugin.so")
	defer cleanup()

	pluginDir := filepath.Join(root, "/dir")
	params := NewParams()
	params.PluginDir = pluginDir
	params.ConfigFile = filepath.Join(root, "/config.yaml")
	rt, err := NewRuntime(context.Background(), params)
	if err != nil {
		t.Fatalf(err.Error())
	}

	// make sure plugin loaded correctly
	_, ok := registeredPlugins[name]
	if !ok {
		t.Fatalf("plugin not present in registeredPlugins map")
	}

	// make sure starting the manager kicks the plugin in
	if err := rt.Manager.Start(context.Background()); err != nil {
		t.Fatalf("Unable to initialize plugins.")
	}

	if len(ch) != 1 {
		t.Fatalf("plugin started %v times", len(ch))
	}
	return
}

// Custom plugins should not start unless expressly specified under config
func TestRegisterPluginsNoConfig(t *testing.T) {
	name := "test"

	ch := make(chan struct{}, 10)

	// server upon request sends item to channel
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		ch <- struct{}{}
	}))
	defer ts.Close()

	files := map[string]string{
		"/dir/test.go": getPluginWithNameAndURL(name, ts.URL),
	}

	root, cleanup := makeDirWithSharedObjects(files, ".plugin.so")
	defer cleanup()

	pluginDir := filepath.Join(root, "/dir")
	params := NewParams()
	params.PluginDir = pluginDir
	rt, err := NewRuntime(context.Background(), params)
	if err != nil {
		t.Fatalf(err.Error())
	}

	// make sure plugin loaded correctly
	_, ok := registeredPlugins[name]
	if !ok {
		t.Fatalf("plugin not present in registeredPlugins map")
	}

	// make sure starting the manager kicks the plugin in
	if err := rt.Manager.Start(context.Background()); err != nil {
		t.Fatalf("Unable to initialize plugins.")
	}

	if len(ch) > 0 {
		t.Fatalf("plugin started without config file")
	}

	return
}

// Tests that plugins have access to the config file
func TestPluginsConfigAccess(t *testing.T) {

	// init func checks for "secret" under check in config file
	plugin := `
package main

import (
	"context"
    "fmt"
	"github.com/open-policy-agent/opa/plugins"
	"github.com/open-policy-agent/opa/util"
)

var Name = "test"

type Tester struct {}

func (t *Tester) Start(ctx context.Context) error {
	return nil
}

func (t *Tester) Stop(ctx context.Context) {
	return
}

var Initializer plugins.PluginInitFunc = func(m *plugins.Manager, config []byte) (plugins.Plugin, error) {
	var test struct {
		Key string
	}

	if err := util.Unmarshal(config, &test); err != nil {
		return nil, err
	}

    if test.Key != "secret" {
		return nil, fmt.Errorf("got %v, expected %v", test.Key, "secret")
    }

	return &Tester{}, nil
}`

	config := `
plugins:
  test:
    key: secret
`

	files := map[string]string{
		"/dir/test.go": plugin,
		"/config.yaml": config,
	}

	root, cleanup := makeDirWithSharedObjects(files, ".plugin.so")
	defer cleanup()

	pluginDir := filepath.Join(root, "/dir")
	params := NewParams()
	params.PluginDir = pluginDir
	params.ConfigFile = filepath.Join(root, "/config.yaml")
	rt, err := NewRuntime(context.Background(), params)
	if err != nil {
		t.Fatalf(err.Error())
	}

	// make sure plugin loaded correctly
	_, ok := registeredPlugins["test"]
	if !ok {
		t.Fatalf("plugin not present in registeredPlugins map")
	}

	// make sure starting the manager kicks the plugin in
	if err := rt.Manager.Start(context.Background()); err != nil {
		t.Fatalf("Unable to initialize plugins.")
	}

	return
}

// makeDirWithSharedObjects creates a new temporary directory containing files under the runtime directory
// it compiles all .go files into shared object files with extension ext in the corresponding directory
// It returns the root of the directory and a cleanup function.
func makeDirWithSharedObjects(files map[string]string, ext string) (root string, cleanup func()) {
	root, cleanup, err := test.MakeTempFS("./", "runtime_test_tempdir", files)
	if err != nil {
		panic(err)
	}
	for file := range files {
		if filepath.Ext(file) == ".go" {
			src := filepath.Join(root, file)
			so := strings.TrimSuffix(filepath.Base(src), ".go") + ext
			out := filepath.Join(filepath.Dir(src), so)
			// build latest version of shared object
			cmd := exec.Command("go", "build", "-buildmode=plugin", "-o="+out, src)
			res, err := cmd.Output()
			if err != nil {
				panic(fmt.Sprintf("attempted to build %v to %v\n", src, out) + string(res) + err.Error())
			}
		}
	}
	return
}
