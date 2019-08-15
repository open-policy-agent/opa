// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/open-policy-agent/opa/ast"
	bundleApi "github.com/open-policy-agent/opa/bundle"
	"github.com/open-policy-agent/opa/download"
	"github.com/open-policy-agent/opa/plugins"
	"github.com/open-policy-agent/opa/plugins/bundle"
	"github.com/open-policy-agent/opa/plugins/status"
	"github.com/open-policy-agent/opa/storage/inmem"
	"github.com/open-policy-agent/opa/util"
	"github.com/open-policy-agent/opa/version"
)

func TestMain(m *testing.M) {
	if version.Version == "" {
		version.Version = "unit-test"
	}
	os.Exit(m.Run())
}

func TestEvaluateBundle(t *testing.T) {

	sampleModule := `
		package foo.bar

		bundle = {
			"name": rt.name,
			"service": "example"
		} {
			rt := opa.runtime()
		}
	`

	b := &bundleApi.Bundle{
		Manifest: bundleApi.Manifest{
			Revision: "quickbrownfaux",
		},
		Data: map[string]interface{}{
			"foo": map[string]interface{}{
				"bar": map[string]interface{}{
					"status": map[string]interface{}{},
				},
			},
		},
		Modules: []bundleApi.ModuleFile{
			{
				Path:   `/example.rego`,
				Raw:    []byte(sampleModule),
				Parsed: ast.MustParseModule(sampleModule),
			},
		},
	}

	info := ast.MustParseTerm(`{"name": "test/bundle1"}`)

	config, err := evaluateBundle(context.Background(), "test-id", info, b, "data.foo.bar")
	if err != nil {
		t.Fatal(err)
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

func TestProcessBundle(t *testing.T) {

	ctx := context.Background()

	manager, err := plugins.New([]byte(`{
		"services": {
			"default": {
				"url": "http://localhost:8181"
			}
		}
	}`), "test-id", inmem.New())
	if err != nil {
		t.Fatal(err)
	}

	initialBundle := makeDataBundle(1, `
		{
			"config": {
				"bundle": {"name": "test1"},
				"status": {},
				"decision_logs": {}
			}
		}
	`)

	_, ps, err := processBundle(ctx, manager, nil, initialBundle, "data.config", nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(ps.Start) != 3 || len(ps.Reconfig) != 0 {
		t.Fatalf("Expected exactly three start events but got %v", ps)
	}

	updatedBundle := makeDataBundle(1, `
		{
			"config": {
				"bundle": {"name": "test2"},
				"status": {"partition_name": "foo"},
				"decision_logs": {"partition_name": "bar"}
			}
		}
	`)

	_, ps, err = processBundle(ctx, manager, nil, updatedBundle, "data.config", nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(ps.Start) != 0 || len(ps.Reconfig) != 3 {
		t.Fatalf("Expected exactly three start events but got %v", ps)
	}

	updatedBundle = makeDataBundle(2, `
		{
			"config": {
				"bundle": {"service": "missing service name", "name": "test2"}
			}
		}
	`)

	_, _, err = processBundle(ctx, manager, nil, updatedBundle, "data.config", nil)
	if err == nil {
		t.Fatal("Expected error but got success")
	}

}

type testFactory struct {
	p *reconfigureTestPlugin
}

func (f testFactory) Validate(*plugins.Manager, []byte) (interface{}, error) {
	return nil, nil
}

func (f testFactory) New(*plugins.Manager, interface{}) plugins.Plugin {
	return f.p
}

type reconfigureTestPlugin struct {
	counts map[string]int
}

func (r *reconfigureTestPlugin) Start(context.Context) error {
	r.counts["start"]++
	return nil
}

func (r *reconfigureTestPlugin) Stop(context.Context) {
}

func (r *reconfigureTestPlugin) Reconfigure(_ context.Context, config interface{}) {
	r.counts["reconfig"]++
}

func TestReconfigure(t *testing.T) {

	manager, err := plugins.New([]byte(`{
		"labels": {"x": "y"},
		"services": {
			"localhost": {
				"url": "http://localhost:9999"
			}
		},
		"discovery": {"name": "config"},
	}`), "test-id", inmem.New())
	if err != nil {
		t.Fatal(err)
	}

	testPlugin := &reconfigureTestPlugin{counts: map[string]int{}}
	testFactory := testFactory{p: testPlugin}

	disco, err := New(manager, Factories(map[string]plugins.Factory{"test_plugin": testFactory}))
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

	initialBundle := makeDataBundle(1, `
		{
			"config": {
				"labels": {"x": "label value changed"},
				"default_decision": "bar/baz",
				"default_authorization_decision": "baz/qux",
				"plugins": {
					"test_plugin": {"a": "b"}
				}
			}
		}
	`)

	disco.oneShot(ctx, download.Update{Bundle: initialBundle})

	// Verify labels are unchanged
	exp := map[string]string{"x": "y", "id": "test-id", "version": version.Version}
	if !reflect.DeepEqual(manager.Labels(), exp) {
		t.Errorf("Expected labels to be unchanged (%v) but got %v", exp, manager.Labels())
	}

	// Verify decision ids set
	expDecision := ast.MustParseTerm("data.bar.baz")
	expAuthzDecision := ast.MustParseTerm("data.baz.qux")
	if !manager.Config.DefaultDecisionRef().Equal(expDecision.Value) {
		t.Errorf("Expected default decision to be %v but got %v", expDecision, manager.Config.DefaultDecisionRef())
	}
	if !manager.Config.DefaultAuthorizationDecisionRef().Equal(expAuthzDecision.Value) {
		t.Errorf("Expected default authz decision to be %v but got %v", expAuthzDecision, manager.Config.DefaultAuthorizationDecisionRef())
	}

	// Verify plugins started
	if !reflect.DeepEqual(testPlugin.counts, map[string]int{"start": 1}) {
		t.Errorf("Expected exactly one plugin start but got %v", testPlugin)
	}

	// Verify plugins reconfigured
	updatedBundle := makeDataBundle(2, `
		{
			"config": {
				"labels": {"x": "label value changed"},
				"default_decision": "bar/baz",
				"default_authorization_decision": "baz/qux",
				"plugins": {
					"test_plugin": {"a": "plugin parameter value changed"}
				}
			}
		}
	`)

	disco.oneShot(ctx, download.Update{Bundle: updatedBundle})

	if !reflect.DeepEqual(testPlugin.counts, map[string]int{"start": 1, "reconfig": 1}) {
		t.Errorf("Expected one plugin start and one reconfig but got %v", testPlugin)
	}

}

type testServer struct {
	t       *testing.T
	mtx     sync.Mutex
	server  *httptest.Server
	updates []status.UpdateRequestV1
}

func (ts *testServer) Start() {
	ts.server = httptest.NewServer(http.HandlerFunc(ts.handle))
}

func (ts *testServer) Stop() {
	ts.server.Close()
}

func (ts *testServer) Updates() []status.UpdateRequestV1 {
	ts.mtx.Lock()
	defer ts.mtx.Unlock()
	return ts.updates
}

func (ts *testServer) handle(w http.ResponseWriter, r *http.Request) {

	var update status.UpdateRequestV1

	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		ts.t.Fatal(err)
	}

	func() {
		ts.mtx.Lock()
		defer ts.mtx.Unlock()
		ts.updates = append(ts.updates, update)
	}()

	w.WriteHeader(200)
}

func TestStatusUpdates(t *testing.T) {

	ts := testServer{t: t}
	ts.Start()
	defer ts.Stop()

	manager, err := plugins.New([]byte(fmt.Sprintf(`{
			"labels": {"x": "y"},
			"services": {
				"localhost": {
					"url": %q
				}
			},
			"discovery": {"name": "config"},
		}`, ts.server.URL)), "test-id", inmem.New())
	if err != nil {
		t.Fatal(err)
	}

	disco, err := New(manager)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

	// Enable status plugin which sends initial update.
	disco.oneShot(ctx, download.Update{ETag: "etag-1", Bundle: makeDataBundle(1, `{
		"config": {
			"status": {}
		}
	}`)})

	// Downloader error.
	disco.oneShot(ctx, download.Update{Error: fmt.Errorf("unknown error")})

	// Clear error.
	disco.oneShot(ctx, download.Update{ETag: "etag-2", Bundle: makeDataBundle(2, `{
		"config": {
			"status": {}
		}
	}`)})

	// Configuration error.
	disco.oneShot(ctx, download.Update{ETag: "etag-3", Bundle: makeDataBundle(3, `{
		"config": {
			"status": {"service": "missing service"}
		}
	}`)})

	// Clear error (last successful reconfigure).
	disco.oneShot(ctx, download.Update{ETag: "etag-2"})

	// Check that all updates were received and active revisions are expected.
	var ok bool
	var updates []status.UpdateRequestV1
	t0 := time.Now()

	for !ok && time.Since(t0) < time.Second {
		updates = ts.Updates()
		ok = len(updates) == 5 &&
			updates[0].Discovery.ActiveRevision == "test-revision-1" && updates[0].Discovery.Code == "" &&
			updates[1].Discovery.ActiveRevision == "test-revision-1" && updates[1].Discovery.Code == "bundle_error" &&
			updates[2].Discovery.ActiveRevision == "test-revision-2" && updates[2].Discovery.Code == "" &&
			updates[3].Discovery.ActiveRevision == "test-revision-2" && updates[3].Discovery.Code == "bundle_error" &&
			updates[4].Discovery.ActiveRevision == "test-revision-2" && updates[4].Discovery.Code == ""
	}

	if !ok {
		t.Fatalf("Did not receive expected updates before timeout expired. Received: %+v", updates)
	}
}

func makeDataBundle(n int, s string) *bundleApi.Bundle {
	return &bundleApi.Bundle{
		Manifest: bundleApi.Manifest{Revision: fmt.Sprintf("test-revision-%v", n)},
		Data:     util.MustUnmarshalJSON([]byte(s)).(map[string]interface{}),
	}
}

func getTestManager(t *testing.T, conf string) *plugins.Manager {
	t.Helper()
	store := inmem.New()
	manager, err := plugins.New([]byte(conf), "test-instance-id", store)
	if err != nil {
		t.Fatalf("failed to create plugin manager: %s", err)
	}
	return manager
}

func TestGetPluginSetWithMixedConfig(t *testing.T) {
	conf := `
services:
  s1:
    url: http://test1.com
  s2:
    url: http://test2.com

bundles:
  bundle-new:
    service: s1

bundle:
  name: bundle-classic
  service: s2
`
	manager := getTestManager(t, conf)
	_, err := getPluginSet(nil, manager, manager.Config, nil)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	p := manager.Plugin(bundle.Name)
	if p == nil {
		t.Fatal("Unable to find bundle plugin on manager")
	}
	bp := p.(*bundle.Plugin)

	// make sure the older style `bundle` config takes precedence
	if bp.Config().Name != "bundle-classic" {
		t.Fatal("Expected bundle plugin config Name to be 'bundle-classic'")
	}

	if len(bp.Config().Bundles) != 1 {
		t.Fatal("Expected a single bundle configured")
	}

	if bp.Config().Bundles["bundle-classic"].Service != "s2" {
		t.Fatalf("Expected the classic bundle to be configured as bundles[0], got: %+v", bp.Config().Bundles)
	}
}

func TestGetPluginSetWithBundlesConfig(t *testing.T) {
	conf := `
services:
  s1:
    url: http://test1.com

bundles:
  bundle-new:
    service: s1
`
	manager := getTestManager(t, conf)
	_, err := getPluginSet(nil, manager, manager.Config, nil)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	p := manager.Plugin(bundle.Name)
	if p == nil {
		t.Fatal("Unable to find bundle plugin on manager")
	}
	bp := p.(*bundle.Plugin)

	if len(bp.Config().Bundles) != 1 {
		t.Fatal("Expected a single bundle configured")
	}

	if bp.Config().Bundles["bundle-new"].Service != "s1" {
		t.Fatalf("Expected the bundle to be configured as bundles[0], got: %+v", bp.Config().Bundles)
	}
}
