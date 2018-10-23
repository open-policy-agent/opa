// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/open-policy-agent/opa/ast"
	bundleApi "github.com/open-policy-agent/opa/bundle"
	"github.com/open-policy-agent/opa/plugins"
	"github.com/open-policy-agent/opa/plugins/bundle"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/storage/inmem"
	"github.com/open-policy-agent/opa/util"
	"github.com/open-policy-agent/opa/util/test"
)

func TestInit(t *testing.T) {
	ctx := context.Background()

	tmp1, err := ioutil.TempFile("", "docFile")
	if err != nil {
		panic(err)
	}
	defer os.Remove(tmp1.Name())

	doc1 := `{"foo": "bar", "x": {"y": {"z": [1]}}}`
	if _, err := tmp1.Write([]byte(doc1)); err != nil {
		panic(err)
	}
	if err := tmp1.Close(); err != nil {
		panic(err)
	}

	tmp2, err := ioutil.TempFile("", "policyFile")
	if err != nil {
		panic(err)
	}
	defer os.Remove(tmp2.Name())
	mod1 := `package a.b.c

import data.foo

p = true { foo = "bar" }
p = true { 1 = 2 }`
	if _, err := tmp2.Write([]byte(mod1)); err != nil {
		panic(err)
	}
	if err := tmp2.Close(); err != nil {
		panic(err)
	}

	params := NewParams()
	params.Paths = []string{tmp1.Name(), tmp2.Name()}

	rt, err := NewRuntime(ctx, params)
	if err != nil {
		t.Fatal(err)
	}

	txn := storage.NewTransactionOrDie(ctx, rt.Store)

	node, err := rt.Store.Read(ctx, txn, storage.MustParsePath("/foo"))
	if util.Compare(node, "bar") != 0 || err != nil {
		t.Errorf("Expected %v but got %v (err: %v)", "bar", node, err)
		return
	}

	ids, err := rt.Store.ListPolicies(ctx, txn)
	if err != nil {
		t.Fatal(err)
	}

	result, err := rt.Store.GetPolicy(ctx, txn, ids[0])
	if err != nil || string(result) != mod1 {
		t.Fatalf("Expected %v but got: %v (err: %v)", mod1, result, err)
	}
}

func TestWatchPaths(t *testing.T) {

	fs := map[string]string{
		"/foo/bar/baz.json": "true",
	}

	expected := []string{
		"/foo", "/foo/bar", "/foo/bar/baz.json",
	}

	test.WithTempFS(fs, func(rootDir string) {
		paths, err := getWatchPaths([]string{"prefix:" + rootDir + "/foo"})
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		result := []string{}
		for _, path := range paths {
			result = append(result, strings.TrimPrefix(path, rootDir))
		}
		if !reflect.DeepEqual(expected, result) {
			t.Fatalf("Expected %v but got: %v", expected, result)
		}
	})
}

func TestRuntimeProcessWatchEvents(t *testing.T) {

	ctx := context.Background()
	fs := map[string]string{
		"/some/data.json": `{
			"hello": "world"
		}`,
	}

	test.WithTempFS(fs, func(rootDir string) {
		params := NewParams()
		params.Paths = []string{rootDir}
		rt, err := NewRuntime(ctx, params)
		if err != nil {
			t.Fatal(err)
		}

		var buf bytes.Buffer

		if err := rt.startWatcher(ctx, params.Paths, onReloadPrinter(&buf)); err != nil {
			t.Fatalf("Unexpected watcher init error: %v", err)
		}

		expected := map[string]interface{}{
			"hello": "world-2",
		}

		if err := ioutil.WriteFile(path.Join(rootDir, "some/data.json"), util.MustMarshalJSON(expected), 0644); err != nil {
			panic(err)
		}

		t0 := time.Now()
		path := storage.MustParsePath("/some")

		// In practice, reload takes ~100us on development machine.
		maxWaitTime := time.Second * 1
		var val interface{}

		for time.Since(t0) < maxWaitTime {
			time.Sleep(1 * time.Millisecond)
			txn := storage.NewTransactionOrDie(ctx, rt.Store)
			var err error
			val, err = rt.Store.Read(ctx, txn, path)
			if err != nil {
				panic(err)
			}
			rt.Store.Abort(ctx, txn)
			if reflect.DeepEqual(val, expected) {
				return // success
			}
		}

		t.Fatalf("Did not see expected change in %v, last value: %v, buf: %v", maxWaitTime, val, buf.String())
	})
}

func TestRuntimeProcessWatchEventPolicyError(t *testing.T) {

	ctx := context.Background()

	fs := map[string]string{
		"/x.rego": `package test

		default x = 1
		`,
	}

	test.WithTempFS(fs, func(rootDir string) {
		params := NewParams()
		params.Paths = []string{rootDir}
		rt, err := NewRuntime(ctx, params)
		if err != nil {
			t.Fatal(err)
		}

		storage.Txn(ctx, rt.Store, storage.WriteParams, func(txn storage.Transaction) error {
			return rt.Store.UpsertPolicy(ctx, txn, "out-of-band.rego", []byte(`package foo`))
		})

		ch := make(chan error)

		testFunc := func(d time.Duration, err error) {
			ch <- err
		}

		if err := rt.startWatcher(ctx, params.Paths, testFunc); err != nil {
			t.Fatalf("Unexpected watcher init error: %v", err)
		}

		newModule := []byte(`package test

		default x = 2`)

		if err := ioutil.WriteFile(path.Join(rootDir, "y.rego"), newModule, 0644); err != nil {
			t.Fatal(err)
		}

		// Wait for up to 1 second before considering test failed. On Linux we
		// observe multiple events on write (e.g., create -> write) which
		// triggers two errors instead of one, whereas on Darwin only a single
		// event (e.g., create) is sent. Same as below.
		maxWait := time.Second
		timer := time.NewTimer(maxWait)

		// Expect type error.
		func() {
			for {
				select {
				case result := <-ch:
					if errs, ok := result.(ast.Errors); ok {
						if errs[0].Code == ast.TypeErr {
							err = nil
							return
						}
					}
					err = result
				case <-timer.C:
					return
				}
			}
		}()

		if err != nil {
			t.Fatalf("Expected specific failure before %v. Last error: %v", maxWait, err)
		}

		if err := os.Remove(path.Join(rootDir, "x.rego")); err != nil {
			t.Fatal(err)
		}

		timer = time.NewTimer(maxWait)

		// Expect no error.
		func() {
			for {
				select {
				case result := <-ch:
					if result == nil {
						err = nil
						return
					}
					err = result
				case <-timer.C:
					return
				}
			}
		}()

		if err != nil {
			t.Fatalf("Expected result to succeed before %v. Last error: %v", maxWait, err)
		}

	})
}

func TestConfigDiscoveryEnabled(t *testing.T) {
	config := []byte(`{
			"discovery": {
				"path": "/foo/bar"
			}}`)
	result := isDiscoveryEnabled(config)

	if !result {
		t.Fatal("Expected discovery to be enabled")
	}
}

func TestConfigDiscoveryDisabled(t *testing.T) {
	result := isDiscoveryEnabled([]byte{})

	if result {
		t.Fatal("Expected discovery not to be enabled")
	}
}

func TestConfigDiscoveryHandlerWithModule(t *testing.T) {
	fixture := newTestFixtureWithModule(t)
	defer fixture.server.stop()
	testConfigDiscoveryHandler(t, fixture)
}

func TestConfigDiscoveryHandlerWithData(t *testing.T) {
	fixture := newTestFixtureWithData(t)
	defer fixture.server.stop()
	testConfigDiscoveryHandler(t, fixture)
}

func testConfigDiscoveryHandler(t *testing.T, fixture testFixture) {
	ctx := context.Background()
	newConfig, err := discoveryHandler(ctx, fixture.managerConfig, fixture.manager)

	if err != nil {
		t.Fatal("Unexpected error:", err)
	}

	var config struct {
		Bundle json.RawMessage `json:"bundle"`
	}

	if err := util.Unmarshal(newConfig, &config); err != nil {
		t.Fatal("Unexpected error:", err)
	}

	if config.Bundle == nil {
		t.Fatal("Expected a bundle configuration")
	}

	var parsedConfig bundle.Config

	if err := util.Unmarshal(config.Bundle, &parsedConfig); err != nil {
		t.Fatal("Unexpected error:", err)
	}

	expectedBundleConfig := bundle.Config{
		Name:    "test/bundle1",
		Service: "example",
	}

	if !reflect.DeepEqual(expectedBundleConfig, parsedConfig) {
		t.Fatalf("Expected bundle config %v, but got %v", expectedBundleConfig, parsedConfig)
	}
}

type testFixture struct {
	store         storage.Store
	manager       *plugins.Manager
	server        *testServer
	managerConfig []byte
}

func newTestFixtureWithData(t *testing.T) testFixture {
	ts := testServer{
		t:       t,
		expAuth: "Bearer secret",
		bundles: map[string]bundleApi.Bundle{
			"foo/bar": {
				Manifest: bundleApi.Manifest{
					Revision: "quickbrownfaux",
				},
				Data: map[string]interface{}{
					"foo": map[string]interface{}{
						"bar": map[string]interface{}{"bundle": map[string]interface{}{"name": "test/bundle1", "service": "example"}},
						"baz": "qux",
					},
				},
			},
		},
	}

	ts.start()
	return getFixture(t, ts)
}

func newTestFixtureWithModule(t *testing.T) testFixture {

	sampleModule := `
		package foo

		bar = {
			"bundle": {
				"name": "test/bundle1",
				"service": "example"
			}
		}
	`

	ts := testServer{
		t:       t,
		expAuth: "Bearer secret",
		bundles: map[string]bundleApi.Bundle{
			"foo/bar": {
				Manifest: bundleApi.Manifest{
					Revision: "quickbrownfaux",
				},
				Data: map[string]interface{}{
					"baz": "qux",
				},
				Modules: []bundleApi.ModuleFile{
					{
						Path:   `/example.rego`,
						Raw:    []byte(sampleModule),
						Parsed: ast.MustParseModule(sampleModule),
					},
				},
			},
		},
	}

	ts.start()
	return getFixture(t, ts)

}

func getFixture(t *testing.T, ts testServer) testFixture {
	managerConfig := []byte(fmt.Sprintf(`{
			"services": [
				{
					"name": "example",
					"url": %q,
					"credentials": {
						"bearer": {
							"scheme": "Bearer",
							"token": "secret"
						}
					}
				}
			],
			"discovery": {
				"path": "/foo/bar"
			}}`, ts.server.URL))

	store := inmem.New()

	manager, err := plugins.New(managerConfig, "test-instance-id", store)
	if err != nil {
		t.Fatal(err)
	}

	return testFixture{
		store:         store,
		manager:       manager,
		server:        &ts,
		managerConfig: managerConfig,
	}
}

type testServer struct {
	t       *testing.T
	expCode int
	expEtag string
	expAuth string
	bundles map[string]bundleApi.Bundle
	server  *httptest.Server
}

func (t *testServer) handle(w http.ResponseWriter, r *http.Request) {

	if t.expCode != 0 {
		w.WriteHeader(t.expCode)
		return
	}

	if t.expAuth != "" {
		if r.Header.Get("Authorization") != t.expAuth {
			w.WriteHeader(401)
			return
		}
	}

	name := strings.TrimPrefix(r.URL.Path, "/")
	b, ok := t.bundles[name]
	if !ok {
		w.WriteHeader(404)
		return
	}

	if t.expEtag != "" {
		etag := r.Header.Get("If-None-Match")
		if etag == t.expEtag {
			w.WriteHeader(304)
			return
		}
	}

	w.Header().Add("Content-Type", "application/gzip")

	if t.expEtag != "" {
		w.Header().Add("Etag", t.expEtag)
	}

	w.WriteHeader(200)

	var buf bytes.Buffer

	if err := bundleApi.Write(&buf, b); err != nil {
		w.WriteHeader(500)
	}

	if _, err := w.Write(buf.Bytes()); err != nil {
		panic(err)
	}
}

func (t *testServer) start() {
	t.server = httptest.NewServer(http.HandlerFunc(t.handle))
}

func (t *testServer) stop() {
	t.server.Close()
}
