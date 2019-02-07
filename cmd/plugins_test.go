// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// +build linux,cgo darwin,cgo

package cmd

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/runtime"
	"github.com/open-policy-agent/opa/types"
	"github.com/open-policy-agent/opa/util/test"
)

// whenever a plugin is initialized it adds an item to this channel
var initChan = make(chan struct{}, 256)
var testDirRoot string

const (
	rootDir   = "./"
	prefixDir = "plugin_test_tempdir"
)

// makeDirWithSharedObjects creates a new temporary directory containing files under the runtime directory
// it compiles all .go files into shared object files with extension ext in the corresponding directory
// It returns the root of the directory and a cleanup function.
//
// Code is duplicated from MakeTempFS() due to https://github.com/open-policy-agent/opa/issues/1185
func makeDirWithSharedObjects(files map[string]string, ext string) (string, func()) {

	tempRootDir, err := ioutil.TempDir(rootDir, prefixDir)
	if err != nil {
		panic(err)
	}
	cleanup := func() {
		if err := os.RemoveAll(tempRootDir); err != nil {
			fmt.Printf("failed to cleanup directory %q: %v \n", tempRootDir, err)
		}
	}
	// We install the signal handler soon after the creation of the temp directory
	signalHandler(tempRootDir, cleanup)

	for path, content := range files {
		dirname, filename := filepath.Split(path)
		dirPath := filepath.Join(tempRootDir, dirname)
		if err := os.MkdirAll(dirPath, 0777); err != nil {
			fmt.Printf("failed to create directory %q: %v \n", dirPath, err)
			panic(err)
		}

		f, err := os.Create(filepath.Join(dirPath, filename))
		if err != nil {
			panic(err)
		}

		if _, err := f.WriteString(content); err != nil {
			panic(err)
		}
	}

	for file := range files {
		if filepath.Ext(file) == ".go" {
			src := filepath.Join(tempRootDir, file)
			so := strings.TrimSuffix(filepath.Base(src), ".go") + ext
			out := filepath.Join(filepath.Dir(src), so)
			// build latest version of shared object
			cmd := exec.Command("go", "build", "-buildmode=plugin", "-o="+out, src)
			stdoutStderr, err := cmd.CombinedOutput()
			if err != nil {
				panic(fmt.Sprintf("attempted to build %v to %v \n", src, out) + string(stdoutStderr))
			}
		}
	}
	return tempRootDir, cleanup
}

// emptyInitChan removes all current items in initChan
func emptyInitChan() {
	for len(initChan) > 0 {
		<-initChan
	}
}

// Runs all tests with the filesystem given below. The plugins add an item to initChan upon activation.
// This is a separate function in order to allow deferred calls to activate.
// TestMain does not honor deferred calls as it uses os.Exit.
func testMainInEnvironment(m *testing.M) int {

	// server sends item to channel upon request
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		initChan <- struct{}{}
	}))
	defer ts.Close()

	config := `
plugins:
 test:
   key: secret
`
	badConfig := `
plugins:
 test:
   key: fake
`

	files := map[string]string{
		"/builtins/true.go":        getBuiltinWithName("true"),
		"/plugins/test.go":         getPluginWithNameAndURL("test", ts.URL),
		"/plugins/config.yaml":     config,
		"/plugins/bad-config.yaml": badConfig,
	}
	root, cleanup := makeDirWithSharedObjects(files, ".so")
	testDirRoot = root
	defer cleanup()
	return m.Run()
}

// returns a builtin that always returns true with name name
func getBuiltinWithName(name string) string {
	return fmt.Sprintf(`
package main

import (
	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/types"
	"github.com/open-policy-agent/opa/topdown"
)

var TruthfulBuiltin = &ast.Builtin{
	Name: "%v",
	Decl: types.NewFunction(
		types.Args(types.N, types.N),
		types.B,
	),
}

func Truthful(a, b ast.Value) (ast.Value, error) {
	return ast.Boolean(true), nil
}

func Init() error {
	ast.RegisterBuiltin(TruthfulBuiltin)
	topdown.RegisterFunctionalBuiltin2(TruthfulBuiltin.Name, Truthful)
	return nil
}
`, name)
}

// returns go code for a plugin named name that makes a single get request to URL upon start and requires that
// the key "secret" is provided to start.
func getPluginWithNameAndURL(name, url string) string {
	return fmt.Sprintf(`
package main

import (
	"context"
	"fmt"
	"net/http"

	"github.com/open-policy-agent/opa/plugins"
	"github.com/open-policy-agent/opa/util"
	"github.com/open-policy-agent/opa/runtime"
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

func (t *Tester) Reconfigure(ctx context.Context, config interface{}) {
	return
}

type Config struct { Key string }

type Factory struct {}

func (f Factory) Validate(_ *plugins.Manager, config []byte) (interface{}, error) {
	test := Config{}

	if err := util.Unmarshal(config, &test); err != nil {
		return nil, err
	}

	if test.Key != "secret" {
		return nil, fmt.Errorf("got " + test.Key + ", expected secret")
	}

    return test, nil
}

func (f Factory) New(_ *plugins.Manager, config interface{}) plugins.Plugin {
	return &Tester{}
}

func Init() error {
	runtime.RegisterPlugin(Name, Factory{})
	return nil
}
`, name, url)
}

func TestMain(m *testing.M) {
	os.Exit(testMainInEnvironment(m))
}

// Tests that a single builtin is loaded correctly
func TestRegisterBuiltin(t *testing.T) {

	name := "true"
	builtinDir := filepath.Join(testDirRoot, "/builtins")
	err := registerSharedObjectsFromDir(builtinDir)
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

// Tests that a single plugin is loaded correctly
func TestRegisterPlugin(t *testing.T) {

	// load the plugins
	pluginDir := filepath.Join(testDirRoot, "/plugins")
	if err := registerSharedObjectsFromDir(pluginDir); err != nil {
		t.Fatalf(err.Error())
	}

	params := runtime.NewParams()
	params.ConfigFile = filepath.Join(testDirRoot, "/plugins/config.yaml")
	rt, err := runtime.NewRuntime(context.Background(), params)
	if err != nil {
		t.Fatalf(err.Error())
	}

	// make sure starting the manager kicks the plugin in
	emptyInitChan()
	if err := rt.Manager.Start(context.Background()); err != nil {
		t.Fatalf("Unable to initialize plugins: %v", err.Error())
	}

	if len(initChan) != 1 {
		t.Fatalf("Plugin was started %v times", len(initChan))
	}
}

// Tests that a plugin does not start without a config file
func TestPluginDoesNotStartWithoutConfig(t *testing.T) {

	// load the plugins
	pluginDir := filepath.Join(testDirRoot, "/plugins")
	if err := registerSharedObjectsFromDir(pluginDir); err != nil {
		t.Fatalf(err.Error())
	}

	params := runtime.NewParams()
	rt, err := runtime.NewRuntime(context.Background(), params)
	if err != nil {
		t.Fatalf(err.Error())
	}

	// make sure starting the manager kicks the plugin in
	emptyInitChan()
	if err := rt.Manager.Start(context.Background()); err != nil {
		t.Fatalf("Unable to initialize plugins: %v", err.Error())
	}
	if len(initChan) != 0 {
		t.Fatalf("Plugin was started %v times", len(initChan))
	}
}

// Tests that a plugin correctly runs its registration
func TestPluginNoRegistrationWithWrongKey(t *testing.T) {

	// load the plugins
	pluginDir := filepath.Join(testDirRoot, "/plugins")
	if err := registerSharedObjectsFromDir(pluginDir); err != nil {
		t.Fatalf(err.Error())
	}

	params := runtime.NewParams()
	params.ConfigFile = filepath.Join(testDirRoot, "/plugins/bad-config.yaml")
	_, err := runtime.NewRuntime(context.Background(), params)
	if err == nil || !strings.Contains(err.Error(), "expected secret") {
		t.Fatalf("Runtime exited incorrectly with error %v", err)
	}
}

// Tests that the recursive file walker works as expected
func TestLambdaFileWalker(t *testing.T) {

	files := map[string]string{
		"one.go":              "",
		"two.go":              "",
		"fake.html":           "",
		"deep/three.go":       "",
		"deep/deeper/four.go": "",
		"deep/fake/fake.html": "",
	}

	test.WithTempFS(files, func(root string) {
		count := 0
		err := filepath.Walk(root, lambdaWalker(func(s string) error {
			count++
			return nil
		}, ".go"))
		if err != nil {
			t.Fatalf(err.Error())
		}
		if count != 4 {
			t.Fatalf("Expected 4, got %v", count)
		}
	})
}

func signalHandler(tempRootDir string, cleanup func()) {
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		s := <-signalChan
		switch s {

		case syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT:
			fmt.Printf("Received signal %s. Cleaning %q and exiting \n", s.String(), tempRootDir)
			cleanup()
			os.Exit(1)
		}
	}()
}
