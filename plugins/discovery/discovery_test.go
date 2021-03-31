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

	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"

	"github.com/open-policy-agent/opa/ast"
	bundleApi "github.com/open-policy-agent/opa/bundle"
	"github.com/open-policy-agent/opa/download"
	"github.com/open-policy-agent/opa/metrics"
	"github.com/open-policy-agent/opa/plugins"
	"github.com/open-policy-agent/opa/plugins/bundle"
	"github.com/open-policy-agent/opa/plugins/logs"
	"github.com/open-policy-agent/opa/plugins/status"
	"github.com/open-policy-agent/opa/server"
	"github.com/open-policy-agent/opa/storage/inmem"
	"github.com/open-policy-agent/opa/topdown/cache"
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
		},
		"discovery": {"name": "config"}
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

	disco, err := New(manager)
	if err != nil {
		t.Fatal(err)
	}

	ps, err := disco.processBundle(ctx, initialBundle)
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

	ps, err = disco.processBundle(ctx, updatedBundle)
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

	_, err = disco.processBundle(ctx, updatedBundle)
	if err == nil {
		t.Fatal("Expected error but got success")
	}

}

func TestProcessBundleWithActiveConfig(t *testing.T) {

	ctx := context.Background()

	manager, err := plugins.New([]byte(`{
		"labels": {"x": "y"},
		"services": {
			"localhost": {
				"url": "http://localhost:9999",
				"credentials": {"bearer": {"token": "test"}}
			}
		},
		"keys": {
			"local_key": {
				"private_key": "local"
			}
		},
		"discovery": {"name": "config"},
	}`), "test-id", inmem.New())
	if err != nil {
		t.Fatal(err)
	}

	initialBundle := makeDataBundle(1, `
		{
			"config": {
				"services": {
					"acmecorp": {
						"url": "https://example.com/control-plane-api/v1",
						"credentials": {"bearer": {"token": "test-acmecorp"}}
					}
				},
				"bundles": {"test-bundle": {"service": "localhost"}},
				"status": {"partition_name": "foo"},
				"decision_logs": {"partition_name": "bar"},
				"default_decision": "bar/baz",
				"default_authorization_decision": "baz/qux",
				"keys": {
					"global_key": {
						"scope": "read",
						"key": "secret"
					}
				}
			}
		}
	`)

	disco, err := New(manager)
	if err != nil {
		t.Fatal(err)
	}

	_, err = disco.processBundle(ctx, initialBundle)
	if err != nil {
		t.Fatal(err)
	}

	actual, err := manager.Config.ActiveConfig()
	if err != nil {
		t.Fatal(err)
	}

	expectedConfig := fmt.Sprintf(`{
		"services": {
			"acmecorp": {
				"url": "https://example.com/control-plane-api/v1"
			}
		},
		"labels": {
			"id": "test-id",
			"version": %v,
			"x": "y"
		},
		"keys": {
			"global_key": {
				"scope": "read"
			}
		},
		"decision_logs": {
			"partition_name": "bar"
		},
		"status": {
			"partition_name": "foo"
		},
		"bundles": {
			"test-bundle": {
				"service": "localhost"
			}
		},
		"default_authorization_decision": "baz/qux",
		"default_decision": "bar/baz"}`, version.Version)

	var expected map[string]interface{}
	if err := util.Unmarshal([]byte(expectedConfig), &expected); err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("want %v got %v", expected, actual)
	}

	initialBundle = makeDataBundle(2, `
		{
			"config": {
				"services": {
					"opa.example.com": {
						"url": "https://opa.example.com",
						"credentials": {"bearer": {"token": "test-opa"}}
					}
				},
				"bundles": {"test-bundle-2": {"service": "opa.example.com"}},
				"decision_logs": {},
				"keys": {
					"global_key_2": {
						"scope": "write",
						"key": "secret_2"
					}
				}
			}
		}
	`)

	_, err = disco.processBundle(ctx, initialBundle)
	if err != nil {
		t.Fatal(err)
	}

	actual, err = manager.Config.ActiveConfig()
	if err != nil {
		t.Fatal(err)
	}

	expectedConfig2 := fmt.Sprintf(`{
		"services": {
			"opa.example.com": {
				"url": "https://opa.example.com"
			}
		},
		"labels": {
			"id": "test-id",
			"version": %v,
			"x": "y"
		},
		"keys": {
			"global_key_2": {
				"scope": "write"
			}
		},
		"decision_logs": {},
		"bundles": {
			"test-bundle-2": {
				"service": "opa.example.com"
			}
		},
		"default_authorization_decision": "/system/authz/allow",
		"default_decision": "/system/main"}`, version.Version)

	var expected2 map[string]interface{}
	if err := util.Unmarshal([]byte(expectedConfig2), &expected2); err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(actual, expected2) {
		t.Fatalf("want %v got %v", expected, actual)
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

func TestReconfigureWithUpdates(t *testing.T) {

	ctx := context.Background()

	manager, err := plugins.New([]byte(`{
		"labels": {"x": "y"},
		"services": {
			"localhost": {
				"url": "http://localhost:9999"
			}
		},
		"discovery": {"name": "config"},
		"keys": {
			"global_key": {
				"key": "secret",
				"algorithm": "HS256",
				"scope": "read"
			},
			"local_key": {
				"key": "some_private_key",
				"scope": "write"
			}
		}
	}`), "test-id", inmem.New())
	if err != nil {
		t.Fatal(err)
	}

	disco, err := New(manager)
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

	err = disco.reconfigure(ctx, download.Update{Bundle: initialBundle})
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}

	originalConfig := disco.config
	// update the discovery configuration and check
	// the boot configuration is not overwritten
	updatedBundle := makeDataBundle(2, `
		{
			"config": {
				"discovery": {
					"name": "config",
					"decision": "/foo/bar"
				}
			}
		}
	`)

	err = disco.reconfigure(ctx, download.Update{Bundle: updatedBundle})
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}

	if !reflect.DeepEqual(originalConfig, disco.config) {
		t.Fatal("Discovery configuration updated")
	}

	// no update to the discovery configuration and check no error generated
	updatedBundle = makeDataBundle(3, `
		{
			"config": {
				"discovery": {
					"name": "config"
				}
			}
		}
	`)

	err = disco.reconfigure(ctx, download.Update{Bundle: updatedBundle})
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}

	if !reflect.DeepEqual(originalConfig, disco.config) {
		t.Fatal("Discovery configuration updated")
	}

	// update the discovery service and check that error generated
	updatedBundle = makeDataBundle(4, `
		{
			"config": {
				"services": {
					"localhost": {
						"url": "http://localhost:9999",
						"credentials": {"bearer": {"token": "blah"}}
					}
				}
			}
		}
	`)

	err = disco.reconfigure(ctx, download.Update{Bundle: updatedBundle})
	if err == nil {
		t.Fatal("Expected error but got nil")
	}

	expectedErrMsg := "updates to the discovery service are not allowed"
	if err.Error() != expectedErrMsg {
		t.Fatalf("Expected error message: %v but got: %v", expectedErrMsg, err.Error())
	}

	// no update to the discovery service and check no error generated
	updatedBundle = makeDataBundle(5, `
		{
			"config": {
				"services": {
					"localhost": {
						"url": "http://localhost:9999"
					}
				}
			}
		}
	`)

	err = disco.reconfigure(ctx, download.Update{Bundle: updatedBundle})
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}

	// add a new service and a new bundle
	updatedBundle = makeDataBundle(6, `
		{
			"config": {
				"services": {
					"acmecorp": {
						"url": "http://localhost:8181"
					}
				},
				"bundles": {
					"authz": {
						"service": "acmecorp"
					}
				}
			}
		}
	`)

	err = disco.reconfigure(ctx, download.Update{Bundle: updatedBundle})
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}

	if len(disco.manager.Services()) != 2 {
		t.Fatalf("Expected two services but got %v\n", len(disco.manager.Services()))
	}

	bPlugin := bundle.Lookup(disco.manager)
	config := bPlugin.Config()
	expected := "acmecorp"
	if config.Bundles["authz"].Service != expected {
		t.Fatalf("Expected service %v for bundle authz but got %v", expected, config.Bundles["authz"].Service)
	}

	// update existing bundle's config and add a new bundle
	updatedBundle = makeDataBundle(7, `
		{
			"config": {
				"bundles": {
					"authz": {
						"service": "localhost",
						"resource": "foo/bar"
					},
					"main": {
						"resource": "baz/bar"
					}
				}
			}
		}
	`)

	err = disco.reconfigure(ctx, download.Update{Bundle: updatedBundle})
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}

	bPlugin = bundle.Lookup(disco.manager)
	config = bPlugin.Config()
	expectedSvc := "localhost"
	if config.Bundles["authz"].Service != expectedSvc {
		t.Fatalf("Expected service %v for bundle authz but got %v", expectedSvc, config.Bundles["authz"].Service)
	}

	expectedRes := "foo/bar"
	if config.Bundles["authz"].Resource != expectedRes {
		t.Fatalf("Expected resource %v for bundle authz but got %v", expectedRes, config.Bundles["authz"].Resource)
	}

	expectedSvcs := map[string]bool{"localhost": true, "acmecorp": true}
	if _, ok := expectedSvcs[config.Bundles["main"].Service]; !ok {
		t.Fatalf("Expected service for bundle main to be one of [%v, %v] but got %v", "localhost", "acmecorp", config.Bundles["main"].Service)
	}

	// update existing (non-discovery)service's config
	updatedBundle = makeDataBundle(8, `
		{
			"config": {
				"services": {
					"acmecorp": {
						"url": "http://localhost:8181",
						"credentials": {"bearer": {"token": "blah"}}
						}
				},
				"bundles": {
					"authz": {
						"service": "localhost",
						"resource": "foo/bar"
					}
				}
			}
		}
	`)

	err = disco.reconfigure(ctx, download.Update{Bundle: updatedBundle})
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}

	// add a new key
	updatedBundle = makeDataBundle(9, `
		{
			"config": {
				"keys": {
					"new_global_key": {
						"key": "secret",
						"algorithm": "HS256",
						"scope": "read"
					}
				}
			}
		}
	`)

	err = disco.reconfigure(ctx, download.Update{Bundle: updatedBundle})
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}

	// update a key in the boot config
	updatedBundle = makeDataBundle(10, `
		{
			"config": {
				"keys": {
					"global_key": {
						"key": "new_secret",
						"algorithm": "HS256",
						"scope": "read"
					}
				}
			}
		}
	`)

	err = disco.reconfigure(ctx, download.Update{Bundle: updatedBundle})
	if err == nil {
		t.Fatal("Expected error but got nil")
	}

	errMsg := "updates to keys specified in the boot configuration are not allowed"
	if err.Error() != errMsg {
		t.Fatalf("Expected error message: %v but got: %v", errMsg, err.Error())
	}

	// no config change for a key in the boot config
	updatedBundle = makeDataBundle(11, `
		{
			"config": {
				"keys": {
					"global_key": {
						"key": "secret",
						"algorithm": "HS256",
						"scope": "read"
					}
				}
			}
		}
	`)

	err = disco.reconfigure(ctx, download.Update{Bundle: updatedBundle})
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}

	// update a key not in the boot config
	updatedBundle = makeDataBundle(12, `
		{
			"config": {
				"keys": {
					"new_global_key": {
						"key": "secret",
						"algorithm": "HS256",
						"scope": "write"
					}
				}
			}
		}
	`)

	err = disco.reconfigure(ctx, download.Update{Bundle: updatedBundle})
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}
}

func TestProcessBundleWithSigning(t *testing.T) {

	ctx := context.Background()

	manager, err := plugins.New([]byte(`{
		"labels": {"x": "y"},
		"services": {
			"localhost": {
				"url": "http://localhost:9999"
			}
		},
		"discovery": {"name": "config", "signing": {"keyid": "my_global_key"}},
		"keys": {"my_global_key": {"algorithm": "HS256", "key": "secret"}},
	}`), "test-id", inmem.New())
	if err != nil {
		t.Fatal(err)
	}

	disco, err := New(manager)
	if err != nil {
		t.Fatal(err)
	}

	initialBundle := makeDataBundle(1, `
		{
			"config": {
				"bundle": {"name": "test1"},
				"status": {},
				"decision_logs": {},
				"keys": {"my_local_key": {"algorithm": "HS256", "key": "new_secret"}}
			}
		}
	`)

	_, err = disco.processBundle(ctx, initialBundle)
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
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
		ok = len(updates) == 7 &&
			updates[0].Plugins["discovery"].State == plugins.StateNotReady && updates[0].Plugins["status"].State == plugins.StateOK &&
			updates[1].Plugins["discovery"].State == plugins.StateOK && updates[1].Plugins["status"].State == plugins.StateOK &&
			updates[2].Plugins["discovery"].State == plugins.StateOK && updates[2].Discovery.ActiveRevision == "test-revision-1" && updates[2].Discovery.Code == "" &&
			updates[3].Plugins["discovery"].State == plugins.StateOK && updates[3].Discovery.ActiveRevision == "test-revision-1" && updates[3].Discovery.Code == "bundle_error" &&
			updates[4].Plugins["discovery"].State == plugins.StateOK && updates[4].Discovery.ActiveRevision == "test-revision-2" && updates[4].Discovery.Code == "" &&
			updates[5].Plugins["discovery"].State == plugins.StateOK && updates[5].Discovery.ActiveRevision == "test-revision-2" && updates[5].Discovery.Code == "bundle_error" &&
			updates[6].Plugins["discovery"].State == plugins.StateOK && updates[6].Discovery.ActiveRevision == "test-revision-2" && updates[6].Discovery.Code == ""
	}

	if !ok {
		t.Fatalf("Did not receive expected updates before timeout expired. Received: %+v", updates)
	}
}

func TestStatusUpdatesTimestamp(t *testing.T) {

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

	// simulate HTTP 200 response from downloader
	disco.oneShot(ctx, download.Update{ETag: "etag-1", Bundle: makeDataBundle(1, `{
		"config": {
			"status": {}
		}
	}`)})

	if disco.status.LastSuccessfulDownload != disco.status.LastSuccessfulRequest || disco.status.LastSuccessfulDownload != disco.status.LastRequest {
		t.Fatal("expected last successful request to be same as download and request")
	}

	if disco.status.LastSuccessfulActivation.IsZero() {
		t.Fatal("expected last successful activation to be non-zero")
	}

	time.Sleep(time.Millisecond)

	// simulate HTTP 304 response from downloader
	disco.oneShot(ctx, download.Update{ETag: "etag-1", Bundle: nil})
	if disco.status.LastSuccessfulDownload == disco.status.LastSuccessfulRequest || disco.status.LastSuccessfulDownload == disco.status.LastRequest {
		t.Fatal("expected last successful download to differ from request and last request")
	}

	// simulate HTTP 200 response from downloader
	disco.oneShot(ctx, download.Update{ETag: "etag-2", Bundle: makeDataBundle(2, `{
		"config": {
			"status": {}
		}
	}`)})

	if disco.status.LastSuccessfulDownload != disco.status.LastSuccessfulRequest || disco.status.LastSuccessfulDownload != disco.status.LastRequest {
		t.Fatal("expected last successful request to be same as download and request")
	}

	if disco.status.LastSuccessfulActivation.IsZero() {
		t.Fatal("expected last successful activation to be non-zero")
	}

	// simulate error response from downloader
	disco.oneShot(ctx, download.Update{Error: fmt.Errorf("unknown error")})

	if disco.status.LastSuccessfulDownload != disco.status.LastSuccessfulRequest || disco.status.LastSuccessfulDownload == disco.status.LastRequest {
		t.Fatal("expected last successful request to be same as download but different from request")
	}
}

func TestStatusMetricsForLogDrops(t *testing.T) {

	ts := testServer{t: t}
	ts.Start()
	defer ts.Stop()

	logLevel := logrus.GetLevel()
	defer logrus.SetLevel(logLevel)

	// Ensure that status messages are printed to console even with the standard logger configured to log errors only
	logrus.SetLevel(logrus.ErrorLevel)

	hook := test.NewLocal(plugins.GetConsoleLogger())

	ctx := context.Background()

	manager, err := plugins.New([]byte(fmt.Sprintf(`{
			"services": {
				"localhost": {
					"url": %q
				}
			},
			"discovery": {"name": "config"}
		}`, ts.server.URL)), "test-id", inmem.New())
	if err != nil {
		t.Fatal(err)
	}

	initialBundle := makeDataBundle(1, `
		{
			"config": {
				"status": {"console": true},
				"decision_logs": {
					"service": "localhost",
					"reporting": {
						"max_decisions_per_second": 1
					}
				}
			}
		}
	`)

	disco, err := New(manager, Metrics(metrics.New()))
	if err != nil {
		t.Fatal(err)
	}

	ps, err := disco.processBundle(ctx, initialBundle)
	if err != nil {
		t.Fatal(err)
	}

	// start the decision log and status plugins
	for _, p := range ps.Start {
		if err := p.Start(ctx); err != nil {
			t.Fatal(err)
		}
	}

	plugin := logs.Lookup(manager)
	if plugin == nil {
		t.Fatal("Expected decision log plugin registered on manager")
	}

	var input interface{} = map[string]interface{}{"method": "GET"}
	var result interface{} = false

	event1 := &server.Info{
		DecisionID: "abc",
		Path:       "foo/bar",
		Input:      &input,
		Results:    &result,
		RemoteAddr: "test-1",
	}

	event2 := &server.Info{
		DecisionID: "def",
		Path:       "foo/baz",
		Input:      &input,
		Results:    &result,
		RemoteAddr: "test-2",
	}

	event3 := &server.Info{
		DecisionID: "ghi",
		Path:       "foo/aux",
		Input:      &input,
		Results:    &result,
		RemoteAddr: "test-3",
	}

	_ = plugin.Log(ctx, event1) // event 1 should be written into the decision log encoder
	_ = plugin.Log(ctx, event2) // event 2 should not be written into the decision log encoder as rate limit exceeded
	_ = plugin.Log(ctx, event3) // event 3 should not be written into the decision log encoder as rate limit exceeded

	// trigger a status update
	disco.oneShot(ctx, download.Update{ETag: "etag-1", Bundle: makeDataBundle(1, `{
		"config": {
			"bundle": {"name": "test1"}
		}
	}`)})

	entries := hook.AllEntries()
	if len(entries) == 0 {
		t.Fatal("Expected log entries but got none")
	}

	// Pick the last entry as it should have the drop count
	e := entries[len(entries)-1]

	if _, ok := e.Data["metrics"]; !ok {
		t.Fatal("Expected metrics")
	}

	exp := map[string]interface{}{"<built-in>": map[string]interface{}{"counter_decision_logs_dropped": json.Number("2")}}

	if !reflect.DeepEqual(e.Data["metrics"], exp) {
		t.Fatalf("Expected %v but got %v", exp, e.Data["metrics"])
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

func TestInterQueryBuiltinCacheConfigUpdate(t *testing.T) {
	var config1 *cache.Config
	var config2 *cache.Config
	manager, err := plugins.New([]byte(`{
		"discovery": {"name": "config"},
		"services": {
			"localhost": {
				"url": "http://localhost:9999"
			}
		},
  }`), "test-id", inmem.New())
	manager.RegisterCacheTrigger(func(c *cache.Config) {
		if config1 == nil {
			config1 = c
		} else if config2 == nil {
			config2 = c
		} else {
			t.Fatal("Expected cache trigger to only be called twice")
		}
	})
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

	initialBundle := makeDataBundle(1, `{
    "config": {
      "caching": {
        "inter_query_builtin_cache": {
          "max_size_bytes": 100
        }
      }
    }
  }`)

	disco.oneShot(ctx, download.Update{Bundle: initialBundle})

	// Verify interQueryBuiltinCacheConfig is triggered with initial config
	if config1 == nil || *config1.InterQueryBuiltinCache.MaxSizeBytes != int64(100) {
		t.Fatalf("Expected cache max size bytes to be 100 after initial discovery, got: %v", config1.InterQueryBuiltinCache.MaxSizeBytes)
	}

	// Verify interQueryBuiltinCache is reconfigured
	updatedBundle := makeDataBundle(2, `{
    "config": {
      "caching": {
        "inter_query_builtin_cache": {
          "max_size_bytes": 200
        }
      }
    }
  }`)

	disco.oneShot(ctx, download.Update{Bundle: updatedBundle})

	if config2 == nil || *config2.InterQueryBuiltinCache.MaxSizeBytes != int64(200) {
		t.Fatalf("Expected cache max size bytes to be 200 after discovery reconfigure, got: %v", config2.InterQueryBuiltinCache.MaxSizeBytes)
	}
}
