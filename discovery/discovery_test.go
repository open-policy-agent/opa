// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package discovery

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/ast"
	bundleApi "github.com/open-policy-agent/opa/bundle"
	"github.com/open-policy-agent/opa/plugins"
	"github.com/open-policy-agent/opa/plugins/bundle"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/storage/inmem"
	"github.com/open-policy-agent/opa/util"
)

func TestConfigDiscoveryEnabled(t *testing.T) {
	config := []byte(`{
			"discovery": {
				"path": "/foo/bar"
			}}`)
	_, result := isDiscoveryEnabled(config)

	if !result {
		t.Fatal("Expected discovery to be enabled")
	}
}

func TestConfigDiscoveryDisabled(t *testing.T) {
	_, result := isDiscoveryEnabled([]byte{})

	if result {
		t.Fatal("Expected discovery not to be enabled")
	}
}

func TestGetDiscoveryServicePath(t *testing.T) {

	prefix := "/bundles"
	path := "/foo/bar/"
	config := discoveryPathConfig{
		Prefix: &prefix,
		Name:   &path,
	}

	expected := "bundles/foo/bar"
	result := getDiscoveryServicePath(config)
	if result != expected {
		t.Fatalf("Expected discovery service path %v, but got %v", expected, result)
	}

	prefix = ""
	path = "/foo/bar/"
	config = discoveryPathConfig{
		Prefix: &prefix,
		Name:   &path,
	}

	expected = "/foo/bar"
	result = getDiscoveryServicePath(config)
	if result != expected {
		t.Fatalf("Expected discovery service path %v, but got %v", expected, result)
	}

	prefix = "bundles/v1"
	path = ""
	config = discoveryPathConfig{
		Prefix: &prefix,
		Name:   &path,
	}

	expected = "bundles/v1/"
	result = getDiscoveryServicePath(config)
	if result != expected {
		t.Fatalf("Expected discovery service path %v, but got %v", expected, result)
	}

	prefix = ""
	path = ""
	config = discoveryPathConfig{
		Prefix: &prefix,
		Name:   &path,
	}

	expected = "/"
	result = getDiscoveryServicePath(config)
	if result != expected {
		t.Fatalf("Expected discovery service path %v, but got %v", expected, result)
	}

	prefix = "/bundles"
	config = discoveryPathConfig{
		Prefix: &prefix,
	}

	expected = "bundles/"
	result = getDiscoveryServicePath(config)
	if result != expected {
		t.Fatalf("Expected discovery service path %v, but got %v", expected, result)
	}

	prefix = ""
	config = discoveryPathConfig{
		Prefix: &prefix,
	}

	expected = "/"
	result = getDiscoveryServicePath(config)
	if result != expected {
		t.Fatalf("Expected discovery service path %v, but got %v", expected, result)
	}

	path = "/foo/bar/"
	config = discoveryPathConfig{
		Name: &path,
	}

	expected = "bundles/foo/bar"
	result = getDiscoveryServicePath(config)
	if result != expected {
		t.Fatalf("Expected discovery service path %v, but got %v", expected, result)
	}

	path = ""
	config = discoveryPathConfig{
		Name: &path,
	}

	expected = "bundles/"
	result = getDiscoveryServicePath(config)
	if result != expected {
		t.Fatalf("Expected discovery service path %v, but got %v", expected, result)
	}

	config = discoveryPathConfig{}

	expected = "bundles/"
	result = getDiscoveryServicePath(config)
	if result != expected {
		t.Fatalf("Expected discovery service path %v, but got %v", expected, result)
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

func TestConfigDiscoveryHandlerWithBadConfig(t *testing.T) {
	fixture := newTestFixtureWithBadConfig(t)
	defer fixture.server.stop()

	discConfig, err := getDiscoveryConfig(fixture.managerConfig)
	if err != nil {
		t.Fatal("Unexpected error:", err)
	}

	discPathConfig, err := validateAndInjectDefaults(discConfig, fixture.manager.Services())
	if err != nil {
		t.Fatal("Unexpected error:", err)
	}

	_, _, err = discoveryHandler(context.Background(), discPathConfig, fixture.manager)

	if err == nil {
		t.Fatal("Expected error but got nil")
	}
}

func TestConfigDiscoveryHandler404Status(t *testing.T) {
	fixture := newTestFixtureWithData(t)
	defer fixture.server.stop()

	fixture.managerConfig = []byte(fmt.Sprintf(`{
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
				"name": "/foo"
			}}`, fixture.server.server.URL))

	discConfig, err := getDiscoveryConfig(fixture.managerConfig)
	if err != nil {
		t.Fatal("Unexpected error:", err)
	}

	discPathConfig, err := validateAndInjectDefaults(discConfig, fixture.manager.Services())
	if err != nil {
		t.Fatal("Unexpected error:", err)
	}

	_, _, err = discoveryHandler(context.Background(), discPathConfig, fixture.manager)
	if err == nil {
		t.Fatal("Expected error but got nil")
	}

	expected := "Discovery configuration download failed, server replied with not found"
	if err.Error() != expected {
		t.Fatalf("Expected error: %v but got %v", expected, err.Error())
	}
}

func testConfigDiscoveryHandler(t *testing.T, fixture testFixture) {
	ctx := context.Background()

	discConfig, err := getDiscoveryConfig(fixture.managerConfig)
	if err != nil {
		t.Fatal("Unexpected error:", err)
	}

	discPathConfig, err := validateAndInjectDefaults(discConfig, fixture.manager.Services())
	if err != nil {
		t.Fatal("Unexpected error:", err)
	}

	newConfig, _, err := discoveryHandler(ctx, discPathConfig, fixture.manager)

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
			"bundles/foo/bar": {
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
			"bundles/foo/bar": {
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

func newTestFixtureWithBadConfig(t *testing.T) testFixture {

	sampleModule := `
		package foo

		bar = [{
			"bundle": {
				"name": "test/bundle1",
				"service": "example"
			}
		}]
	`

	ts := testServer{
		t:       t,
		expAuth: "Bearer secret",
		bundles: map[string]bundleApi.Bundle{
			"bundles/foo/bar": {
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
				"name": "/foo/bar"
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
