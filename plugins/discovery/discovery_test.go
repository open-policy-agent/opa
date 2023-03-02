// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// nolint: goconst // string duplication is for test readability.
package discovery

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/bundle"
	bundleApi "github.com/open-policy-agent/opa/bundle"
	"github.com/open-policy-agent/opa/download"
	"github.com/open-policy-agent/opa/logging/test"
	"github.com/open-policy-agent/opa/metrics"
	"github.com/open-policy-agent/opa/plugins"
	bundlePlugin "github.com/open-policy-agent/opa/plugins/bundle"
	"github.com/open-policy-agent/opa/plugins/logs"
	"github.com/open-policy-agent/opa/plugins/status"
	"github.com/open-policy-agent/opa/rego"
	"github.com/open-policy-agent/opa/server"
	"github.com/open-policy-agent/opa/storage"
	inmem "github.com/open-policy-agent/opa/storage/inmem/test"
	"github.com/open-policy-agent/opa/topdown/cache"
	"github.com/open-policy-agent/opa/util"
	"github.com/open-policy-agent/opa/version"
)

const (
	snapshotBundleSize = 1024
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

	var parsedConfig bundlePlugin.Config

	if err := util.Unmarshal(config.Bundle, &parsedConfig); err != nil {
		t.Fatal("Unexpected error:", err)
	}

	expectedBundleConfig := bundlePlugin.Config{
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

func (*reconfigureTestPlugin) Stop(context.Context) {
}

func (r *reconfigureTestPlugin) Reconfigure(_ context.Context, config interface{}) {
	r.counts["reconfig"]++
}

func TestStartWithBundlePersistence(t *testing.T) {
	dir := t.TempDir()

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

	initialBundle.Manifest.Init()

	var buf bytes.Buffer
	if err := bundle.NewWriter(&buf).Write(*initialBundle); err != nil {
		t.Fatal("unexpected error:", err)
	}

	bundleDir := filepath.Join(dir, "bundles", "config")

	err := os.MkdirAll(bundleDir, os.ModePerm)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	if err := os.WriteFile(filepath.Join(bundleDir, "bundle.tar.gz"), buf.Bytes(), 0644); err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	manager, err := plugins.New([]byte(`{
		"labels": {"x": "y"},
		"services": {
			"localhost": {
				"url": "http://localhost:9999"
			}
		},
		"discovery": {"name": "config", "persist": true},
	}`), "test-id", inmem.New())
	if err != nil {
		t.Fatal(err)
	}

	manager.Config.PersistenceDirectory = &dir

	testPlugin := &reconfigureTestPlugin{counts: map[string]int{}}
	testFactory := testFactory{p: testPlugin}

	disco, err := New(manager, Factories(map[string]plugins.Factory{"test_plugin": testFactory}))
	if err != nil {
		t.Fatal(err)
	}

	err = disco.Start(context.Background())
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	ensurePluginState(t, disco, plugins.StateOK)

	// verify the test plugin was registered on the manager
	if plugin := manager.Plugin("test_plugin"); plugin == nil {
		t.Fatalf("expected \"test_plugin\" to be regsitered with the plugin manager")
	}

	// verify the test plugin was started
	count, ok := testPlugin.counts["start"]
	if !ok {
		t.Fatal("expected test plugin to have start counter")
	}

	if count != 1 {
		t.Fatalf("expected test plugin to have a start count of 1 but got %v", count)
	}
}

func TestOneShotWithBundlePersistence(t *testing.T) {
	dir := t.TempDir()

	manager, err := plugins.New([]byte(`{
		"labels": {"x": "y"},
		"services": {
			"localhost": {
				"url": "http://localhost:9999"
			}
		},
		"discovery": {"name": "config", "persist": true},
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

	disco.bundlePersistPath = filepath.Join(dir, ".opa")

	ensurePluginState(t, disco, plugins.StateNotReady)

	// simulate a bundle download error with no bundle on disk
	disco.oneShot(ctx, download.Update{Error: fmt.Errorf("unknown error")})

	if disco.status.Message == "" {
		t.Fatal("expected error but got none")
	}

	ensurePluginState(t, disco, plugins.StateNotReady)

	// download a bundle and persist to disk. Then verify the bundle persisted to disk
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

	initialBundle.Manifest.Init()
	expBndl := initialBundle.Copy()

	var buf bytes.Buffer
	if err := bundle.NewWriter(&buf).Write(*initialBundle); err != nil {
		t.Fatal("unexpected error:", err)
	}

	disco.oneShot(ctx, download.Update{Bundle: initialBundle, ETag: "etag-1", Raw: &buf})

	ensurePluginState(t, disco, plugins.StateOK)

	result, err := disco.loadBundleFromDisk()
	if err != nil {
		t.Fatal("unexpected error:", err)
	}

	if !result.Equal(expBndl) {
		t.Fatalf("expected the downloaded bundle to be equal to the one loaded from disk: result=%v, exp=%v", result, expBndl)
	}

	// verify the test plugin was registered on the manager
	if plugin := manager.Plugin("test_plugin"); plugin == nil {
		t.Fatalf("expected \"test_plugin\" to be regsitered with the plugin manager")
	}

	// verify the test plugin was started
	count, ok := testPlugin.counts["start"]
	if !ok {
		t.Fatal("expected test plugin to have start counter")
	}

	if count != 1 {
		t.Fatalf("expected test plugin to have a start count of 1 but got %v", count)
	}
}

func TestLoadAndActivateBundleFromDisk(t *testing.T) {
	dir := t.TempDir()

	manager, err := plugins.New([]byte(`{
		"labels": {"x": "y"},
		"services": {
			"localhost": {
				"url": "http://localhost:9999"
			}
		},
		"discovery": {"name": "config", "persist": true},
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

	disco.bundlePersistPath = filepath.Join(dir, ".opa")

	ensurePluginState(t, disco, plugins.StateNotReady)

	// persist a bundle to disk and then load it
	initialBundle := makeDataBundle(1, `
		{
			"config": {
				"labels": {"x": "label value changed"},
				"default_decision": "bar/baz",
				"default_authorization_decision": "baz/qux",
				"plugins": {
					"test_plugin": {"a": "b"}
				},
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

	initialBundle.Manifest.Init()

	var buf bytes.Buffer
	if err := bundle.NewWriter(&buf).Write(*initialBundle); err != nil {
		t.Fatal("unexpected error:", err)
	}

	err = disco.saveBundleToDisk(&buf)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	disco.loadAndActivateBundleFromDisk(ctx)

	ensurePluginState(t, disco, plugins.StateOK)

	// verify the test plugin was registered on the manager
	if plugin := manager.Plugin("test_plugin"); plugin == nil {
		t.Fatalf("expected \"test_plugin\" to be regsitered with the plugin manager")
	}

	// verify the test plugin was started
	count, ok := testPlugin.counts["start"]
	if !ok {
		t.Fatal("expected test plugin to have start counter")
	}

	if count != 1 {
		t.Fatalf("expected test plugin to have a start count of 1 but got %v", count)
	}

	// verify the bundle plugin was registered on the manager
	if plugin := bundlePlugin.Lookup(disco.manager); plugin == nil {
		t.Fatalf("expected bundle plugin to be regsitered with the plugin manager")
	}
}

func TestLoadAndActivateSignedBundleFromDisk(t *testing.T) {
	dir := t.TempDir()

	manager, err := plugins.New([]byte(`{
		"labels": {"x": "y"},
		"services": {
			"localhost": {
				"url": "http://localhost:9999"
			}
		},
		"discovery": {"name": "config", "persist": true},
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

	disco.bundlePersistPath = filepath.Join(dir, ".opa")
	disco.config.Signing = bundle.NewVerificationConfig(map[string]*bundle.KeyConfig{"foo": {Key: "secret", Algorithm: "HS256"}}, "foo", "", nil)

	ensurePluginState(t, disco, plugins.StateNotReady)

	// persist a bundle to disk and then load it
	initialBundle := makeDataBundle(1, `
		{
			"config": {
				"labels": {"x": "label value changed"},
				"default_decision": "bar/baz",
				"default_authorization_decision": "baz/qux",
				"plugins": {
					"test_plugin": {"a": "b"}
				},
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

	initialBundle.Manifest.Init()

	if err := initialBundle.GenerateSignature(bundle.NewSigningConfig("secret", "HS256", ""), "foo", false); err != nil {
		t.Fatal("Unexpected error:", err)
	}

	var buf bytes.Buffer
	if err := bundle.NewWriter(&buf).Write(*initialBundle); err != nil {
		t.Fatal("unexpected error:", err)
	}

	err = disco.saveBundleToDisk(&buf)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	disco.loadAndActivateBundleFromDisk(ctx)

	ensurePluginState(t, disco, plugins.StateOK)

	// verify the test plugin was registered on the manager
	if plugin := manager.Plugin("test_plugin"); plugin == nil {
		t.Fatalf("expected \"test_plugin\" to be regsitered with the plugin manager")
	}

	// verify the test plugin was started
	count, ok := testPlugin.counts["start"]
	if !ok {
		t.Fatal("expected test plugin to have start counter")
	}

	if count != 1 {
		t.Fatalf("expected test plugin to have a start count of 1 but got %v", count)
	}

	// verify the bundle plugin was registered on the manager
	if plugin := bundlePlugin.Lookup(disco.manager); plugin == nil {
		t.Fatalf("expected bundle plugin to be regsitered with the plugin manager")
	}
}

func TestLoadAndActivateBundleFromDiskMaxAttempts(t *testing.T) {
	dir := t.TempDir()

	manager, err := plugins.New([]byte(`{
		"labels": {"x": "y"},
		"services": {
			"localhost": {
				"url": "http://localhost:9999"
			}
		},
		"discovery": {"name": "config", "persist": true},
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

	disco.bundlePersistPath = filepath.Join(dir, ".opa")

	ensurePluginState(t, disco, plugins.StateNotReady)

	// persist a bundle to disk and then load it
	// this bundle should never activate as the service discovery depends on is modified
	initialBundle := makeDataBundle(1, `
		{
			"config": {
				"labels": {"x": "label value changed"},
				"default_decision": "bar/baz",
				"default_authorization_decision": "baz/qux",
				"plugins": {
					"test_plugin": {"a": "b"}
				},
				"services": {
					"localhost": {
						"url": "http://localhost:8181"
					}
				},
				"bundles": {
					"authz": {
						"service": "localhost"
					}
				}
			}
		}
	`)

	initialBundle.Manifest.Init()

	var buf bytes.Buffer
	if err := bundle.NewWriter(&buf).Write(*initialBundle); err != nil {
		t.Fatal("unexpected error:", err)
	}

	err = disco.saveBundleToDisk(&buf)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	disco.loadAndActivateBundleFromDisk(ctx)

	ensurePluginState(t, disco, plugins.StateNotReady)

	if len(manager.Plugins()) != 0 {
		t.Fatal("expected no plugins to be registered with the plugin manager")
	}
}

func TestSaveBundleToDiskNew(t *testing.T) {
	dir := t.TempDir()

	manager, err := plugins.New([]byte(`{
		"labels": {"x": "y"},
		"services": {
			"localhost": {
				"url": "http://localhost:9999"
			}
		},
		"discovery": {"name": "config", "persist": true},
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

	disco.bundlePersistPath = filepath.Join(dir, ".opa")

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

	initialBundle.Manifest.Init()

	var buf bytes.Buffer
	if err := bundle.NewWriter(&buf).Write(*initialBundle); err != nil {
		t.Fatal("unexpected error:", err)
	}

	err = disco.saveBundleToDisk(&buf)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
}

func TestSaveBundleToDiskNewConfiguredPersistDir(t *testing.T) {
	dir := t.TempDir()

	manager, err := plugins.New([]byte(`{
		"labels": {"x": "y"},
		"services": {
			"localhost": {
				"url": "http://localhost:9999"
			}
		},
		"discovery": {"name": "config", "persist": true},
	}`), "test-id", inmem.New())
	if err != nil {
		t.Fatal(err)
	}

	// configure persistence dir instead of using the default. Discover plugin should pick this up
	manager.Config.PersistenceDirectory = &dir

	testPlugin := &reconfigureTestPlugin{counts: map[string]int{}}
	testFactory := testFactory{p: testPlugin}

	disco, err := New(manager, Factories(map[string]plugins.Factory{"test_plugin": testFactory}))
	if err != nil {
		t.Fatal(err)
	}

	err = disco.Start(context.Background())
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

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

	initialBundle.Manifest.Init()

	var buf bytes.Buffer
	if err := bundle.NewWriter(&buf).Write(*initialBundle); err != nil {
		t.Fatal("unexpected error:", err)
	}

	err = disco.saveBundleToDisk(&buf)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	expectBundlePath := filepath.Join(dir, "bundles", "config", "bundle.tar.gz")
	_, err = os.Stat(expectBundlePath)
	if err != nil {
		t.Errorf("expected bundle persisted at path %v, %v", expectBundlePath, err)
	}
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

	disco.oneShot(ctx, download.Update{Bundle: initialBundle, Size: snapshotBundleSize})

	if disco.status == nil {
		t.Fatal("Expected to find status, found nil")
	} else if disco.status.Type != bundle.SnapshotBundleType {
		t.Fatalf("expected snapshot bundle but got %v", disco.status.Type)
	} else if disco.status.Size != snapshotBundleSize {
		t.Fatalf("expected snapshot bundle size %d but got %d", snapshotBundleSize, disco.status.Size)
	}

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

	if disco.status == nil {
		t.Fatal("Expected to find status, found nil")
	} else if disco.status.Type != bundle.SnapshotBundleType {
		t.Fatalf("expected snapshot bundle but got %v", disco.status.Type)
	}

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

	bPlugin := bundlePlugin.Lookup(disco.manager)
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

	bPlugin = bundlePlugin.Lookup(disco.manager)
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

func TestProcessBundleWithNoSigningConfig(t *testing.T) {
	ctx := context.Background()

	manager, err := plugins.New([]byte(`{
		"labels": {"x": "y"},
		"services": {
			"localhost": {
				"url": "http://localhost:9999"
			}
		},
		"discovery": {"name": "config"}
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
				"bundles": {"test1": {"service": "localhost"}},
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

	ctx := context.Background()

	testLogger := test.New()

	manager, err := plugins.New([]byte(`{
		"services": {
			"localhost": {
				"url": "http://localhost:9999"
			}
		},
		"discovery": {"name": "config"},
	}`), "test-id", inmem.New(), plugins.ConsoleLogger(testLogger))
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
			"bundles": {"test-bundle": {"service": "localhost"}}
		}
	}`)})

	status.Lookup(manager).Stop(ctx)

	entries := testLogger.Entries()
	if len(entries) == 0 {
		t.Fatal("Expected log entries but got none")
	}

	// Pick the last entry as it should have the drop count
	e := entries[len(entries)-1]

	if _, ok := e.Fields["metrics"]; !ok {
		t.Fatal("Expected metrics")
	}

	builtInMet := e.Fields["metrics"].(map[string]interface{})["<built-in>"]
	dropCount := builtInMet.(map[string]interface{})["counter_decision_logs_dropped"]

	actual, err := dropCount.(json.Number).Int64()
	if err != nil {
		t.Fatal(err)
	}

	// Along with event 2 and event 3, event 1 could also get dropped. This happens when the decision log plugin
	// tries to requeue event 1 after a failed upload attempt to a non-existent remote endpoint
	if actual < 2 {
		t.Fatal("Expected at least 2 events to be dropped")
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
	trigger := plugins.TriggerManual
	_, err := getPluginSet(nil, manager, manager.Config, nil, &trigger)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	p := manager.Plugin(bundlePlugin.Name)
	if p == nil {
		t.Fatal("Unable to find bundle plugin on manager")
	}
	bp := p.(*bundlePlugin.Plugin)

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
	trigger := plugins.TriggerManual
	_, err := getPluginSet(nil, manager, manager.Config, nil, &trigger)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	p := manager.Plugin(bundlePlugin.Name)
	if p == nil {
		t.Fatal("Unable to find bundle plugin on manager")
	}
	bp := p.(*bundlePlugin.Plugin)

	if len(bp.Config().Bundles) != 1 {
		t.Fatal("Expected a single bundle configured")
	}

	if bp.Config().Bundles["bundle-new"].Service != "s1" {
		t.Fatalf("Expected the bundle to be configured as bundles[0], got: %+v", bp.Config().Bundles)
	}
}

func TestGetPluginSetWithBadManualTriggerBundlesConfig(t *testing.T) {
	confGood := `
services:
  s1:
    url: http://test1.com

bundles:
  bundle-new:
    service: s1
`

	confBad := `
services:
  s1:
    url: http://test1.com

bundles:
  bundle-new:
    service: s1
    trigger: periodic
`

	tests := map[string]struct {
		conf    string
		wantErr bool
		err     error
	}{
		"no_trigger_mode_mismatch": {
			confGood, false, nil,
		},
		"trigger_mode_mismatch": {
			confBad, true, fmt.Errorf("invalid configuration for bundle \"bundle-new\": trigger mode mismatch: manual and periodic (hint: check discovery configuration)"),
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {

			manager := getTestManager(t, tc.conf)
			trigger := plugins.TriggerManual
			_, err := getPluginSet(nil, manager, manager.Config, nil, &trigger)

			if tc.wantErr {
				if err == nil {
					t.Fatal("Expected error but got nil")
				}

				if tc.err != nil && tc.err.Error() != err.Error() {
					t.Fatalf("Expected error message %v but got %v", tc.err.Error(), err.Error())
				}
			} else {
				if err != nil {
					t.Fatalf("Unexpected error %v", err)
				}
			}
		})
	}
}

func TestGetPluginSetWithBadManualTriggerDecisionLogConfig(t *testing.T) {

	confGood := `
services:
  s1:
    url: http://test1.com

bundles:
  bundle-new:
    service: s1
    trigger: manual
decision_logs:
  service: s1
`

	confBad := `
services:
  s1:
    url: http://test1.com

bundles:
  bundle-new:
    service: s1
    trigger: manual
decision_logs:
  service: s1
  reporting:
    trigger: periodic
`

	tests := map[string]struct {
		conf    string
		wantErr bool
		err     error
	}{
		"no_trigger_mode_mismatch": {
			confGood, false, nil,
		},
		"trigger_mode_mismatch": {
			confBad, true, fmt.Errorf("invalid decision_log config: trigger mode mismatch: manual and periodic (hint: check discovery configuration)"),
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {

			manager := getTestManager(t, tc.conf)
			trigger := plugins.TriggerManual
			_, err := getPluginSet(nil, manager, manager.Config, nil, &trigger)

			if tc.wantErr {
				if err == nil {
					t.Fatal("Expected error but got nil")
				}

				if tc.err != nil && tc.err.Error() != err.Error() {
					t.Fatalf("Expected error message %v but got %v", tc.err.Error(), err.Error())
				}
			} else {
				if err != nil {
					t.Fatalf("Unexpected error %v", err)
				}
			}
		})
	}
}

func TestGetPluginSetWithBadManualTriggerStatusConfig(t *testing.T) {
	confGood := `
services:
  s1:
    url: http://test1.com

bundles:
  bundle-new:
    service: s1
    trigger: manual
decision_logs:
  service: s1
  reporting:
    trigger: manual
status:
  service: s1
`

	confBad := `
services:
  s1:
    url: http://test1.com

bundles:
  bundle-new:
    service: s1
    trigger: manual
decision_logs:
  service: s1
  reporting:
    trigger: manual
status:
  service: s1
  trigger: periodic
`

	tests := map[string]struct {
		conf    string
		wantErr bool
		err     error
	}{
		"no_trigger_mode_mismatch": {
			confGood, false, nil,
		},
		"trigger_mode_mismatch": {
			confBad, true, fmt.Errorf("invalid status config: trigger mode mismatch: manual and periodic (hint: check discovery configuration)"),
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {

			manager := getTestManager(t, tc.conf)
			trigger := plugins.TriggerManual
			_, err := getPluginSet(nil, manager, manager.Config, nil, &trigger)

			if tc.wantErr {
				if err == nil {
					t.Fatal("Expected error but got nil")
				}

				if tc.err != nil && tc.err.Error() != err.Error() {
					t.Fatalf("Expected error message %v but got %v", tc.err.Error(), err.Error())
				}
			} else {
				if err != nil {
					t.Fatalf("Unexpected error %v", err)
				}
			}
		})
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

func TestNDBuiltinCacheConfigUpdate(t *testing.T) {
	type exampleConfig struct {
		v bool
	}
	var config1 *exampleConfig
	var config2 *exampleConfig
	manager, err := plugins.New([]byte(`{
		"discovery": {"name": "config"},
		"services": {
			"localhost": {
				"url": "http://localhost:9999"
			}
		},
  }`), "test-id", inmem.New())
	manager.RegisterNDCacheTrigger(func(x bool) {
		if config1 == nil {
			config1 = &exampleConfig{v: x}
		} else if config2 == nil {
			config2 = &exampleConfig{v: x}
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
			"nd_builtin_cache": true
		}
	}`)

	disco.oneShot(ctx, download.Update{Bundle: initialBundle})

	// Verify NDBuiltinCache is triggered with initial config
	if config1 == nil || config1.v != true {
		t.Fatalf("Expected ND builtin cache to be enabled after initial discovery, got: %v", config1.v)
	}

	// Verify NDBuiltinCache is reconfigured
	updatedBundle := makeDataBundle(2, `{
		"config": {
			"nd_builtin_cache": false
		}
	}`)

	disco.oneShot(ctx, download.Update{Bundle: updatedBundle})

	if config2 == nil || config2.v != false {
		t.Fatalf("Expected ND builtin cache to be disabled after discovery reconfigure, got: %v", config2.v)
	}
}

func TestPluginManualTriggerLifecycle(t *testing.T) {
	ctx := context.Background()
	m := metrics.New()

	fixture := newTestFixture(t)
	defer fixture.stop()

	// run query
	result, err := fixture.runQuery(ctx, "data.foo.bar", m)
	if err != nil {
		t.Fatal(err)
	}

	if result != nil {
		t.Fatalf("Expected nil result but got %v", result)
	}

	// log result (there should not be a decision log plugin on the manager yet)
	err = fixture.log(ctx, "data.foo.bar", m, &result)
	if err != nil {
		t.Fatal(err)
	}

	// start the discovery plugin
	if err := fixture.plugin.Start(ctx); err != nil {
		t.Fatal(err)
	}

	// trigger the discovery plugin
	fixture.server.discoConfig = `
		{
			"config": {
				"bundles": {
					"authz": {
						"service": "example",
						"trigger": "manual"
					}
				},
				"status": {"service": "example", "trigger": "manual"},
				"decision_logs": {"service": "example", "reporting": {"trigger": "manual"}}
			}
		}`

	fixture.server.dicsoBundleRev = 1

	trigger := make(chan struct{})
	fixture.discoTrigger <- trigger
	<-trigger

	// check if the discovery, bundle, decision log and status plugin are configured
	expectedNum := 4
	if len(fixture.manager.Plugins()) != expectedNum {
		t.Fatalf("Expected %v configured plugins but got %v", expectedNum, len(fixture.manager.Plugins()))
	}

	// run query (since the bundle plugin is not triggered yet, there should not be any activated bundles)
	result, err = fixture.runQuery(ctx, "data.foo.bar", m)
	if err != nil {
		t.Fatal(err)
	}

	if result != nil {
		t.Fatalf("Expected nil result but got %v", result)
	}

	// log result
	err = fixture.log(ctx, "data.foo.bar", m, &result)
	if err != nil {
		t.Fatal(err)
	}

	// trigger the bundle plugin
	fixture.server.bundleData = map[string]interface{}{
		"foo": map[string]interface{}{
			"bar": "hello",
		},
	}
	fixture.server.bundleRevision = "abc"

	trigger = make(chan struct{})
	fixture.bundleTrigger <- trigger
	<-trigger

	// ensure the bundle was activated
	txn := storage.NewTransactionOrDie(ctx, fixture.manager.Store)
	names, err := bundleApi.ReadBundleNamesFromStore(ctx, fixture.manager.Store, txn)
	if err != nil {
		t.Fatal(err)
	}

	expectedNum = 1
	if len(names) != expectedNum {
		t.Fatalf("Expected %d bundles in store, found %d", expectedNum, len(names))
	}

	// stop the "read" transaction
	fixture.manager.Store.Abort(ctx, txn)

	// run query
	result, err = fixture.runQuery(ctx, "data.foo.bar", m)
	if err != nil {
		t.Fatal(err)
	}

	expected := "hello"
	if result != expected {
		t.Fatalf("Expected result %v but got %v", expected, result)
	}

	// log result
	err = fixture.log(ctx, "data.foo.bar", m, &result)
	if err != nil {
		t.Fatal(err)
	}

	// trigger the decision log plugin
	trigger = make(chan struct{})
	fixture.decisionLogTrigger <- trigger
	<-trigger

	expectedNum = 2
	if len(fixture.server.logEvent) != expectedNum {
		t.Fatalf("Expected %d decision log events, found %d", expectedNum, len(fixture.server.logEvent))
	}

	// verify the result in the last log
	if *fixture.server.logEvent[1].Result != expected {
		t.Fatalf("Expected result %v but got %v", expected, result)
	}

	// trigger the status plugin
	trigger = make(chan struct{})
	fixture.statusTrigger <- trigger
	<-trigger

	expectedNum = 1
	if len(fixture.server.statusEvent) != expectedNum {
		t.Fatalf("Expected %d status updates, found %d", expectedNum, len(fixture.server.statusEvent))
	}

	// update the service bundle and trigger the bundle plugin again
	fixture.testServiceBundleUpdateScenario(ctx, m)

	// reconfigure the service bundle config to go from manual to periodic polling. This should result in an error
	// when the discovery plugin tries to reconfigure the bundle
	fixture.server.discoConfig = `
		{
			"config": {
				"bundles": {
					"authz": {
						"service": "example",
						"trigger": "periodic"
					}
				}
			}
		}`
	fixture.server.dicsoBundleRev = 2

	trigger = make(chan struct{})
	fixture.discoTrigger <- trigger
	<-trigger

	// trigger the status plugin
	trigger = make(chan struct{})
	fixture.statusTrigger <- trigger
	<-trigger

	expectedNum = 3
	if len(fixture.server.statusEvent) != expectedNum {
		t.Fatalf("Expected %d status updates, found %d", expectedNum, len(fixture.server.statusEvent))
	}

	// check for error in the last update corresponding to the bad service bundle config
	disco, _ := fixture.server.statusEvent[2].(map[string]interface{})
	errMsg := disco["discovery"].(map[string]interface{})["message"]

	expErrMsg := "invalid configuration for bundle \"authz\": trigger mode mismatch: manual and periodic (hint: check discovery configuration)"
	if errMsg != expErrMsg {
		t.Fatalf("Expected error %v but got %v", expErrMsg, errMsg)
	}

	// reconfigure plugins via discovery and then trigger discovery
	fixture.testDiscoReconfigurationScenario(ctx, m)
}

type testFixture struct {
	manager            *plugins.Manager
	plugin             *Discovery
	discoTrigger       chan chan struct{}
	bundleTrigger      chan chan struct{}
	decisionLogTrigger chan chan struct{}
	statusTrigger      chan chan struct{}
	stopCh             chan chan struct{}
	server             *testFixtureServer
}

func newTestFixture(t *testing.T) *testFixture {
	ts := testFixtureServer{
		t:           t,
		statusEvent: []interface{}{},
		logEvent:    []logs.EventV1{},
	}

	ts.start()

	managerConfig := []byte(fmt.Sprintf(`{
			"labels": {
				"app": "example-app"
			},
            "discovery": {"name": "disco", "trigger": "manual", decision: config},
			"services": [
				{
					"name": "example",
					"url": %q
				}
			]}`, ts.server.URL))

	manager, err := plugins.New(managerConfig, "test-id", inmem.New())
	if err != nil {
		t.Fatal(err)
	}

	disco, err := New(manager)
	if err != nil {
		t.Fatal(err)
	}

	manager.Register(Name, disco)

	tf := testFixture{
		manager:            manager,
		plugin:             disco,
		server:             &ts,
		discoTrigger:       make(chan chan struct{}),
		bundleTrigger:      make(chan chan struct{}),
		decisionLogTrigger: make(chan chan struct{}),
		statusTrigger:      make(chan chan struct{}),
		stopCh:             make(chan chan struct{}),
	}

	go tf.loop(context.Background())

	return &tf
}

func (t *testFixture) loop(ctx context.Context) {

	for {
		select {
		case stop := <-t.stopCh:
			close(stop)
			return
		case done := <-t.discoTrigger:
			if p, ok := t.manager.Plugin(Name).(plugins.Triggerable); ok {
				_ = p.Trigger(ctx)
			}
			close(done)

		case done := <-t.bundleTrigger:
			if p, ok := t.manager.Plugin(bundlePlugin.Name).(plugins.Triggerable); ok {
				_ = p.Trigger(ctx)
			}
			close(done)

		case done := <-t.decisionLogTrigger:
			if p, ok := t.manager.Plugin(logs.Name).(plugins.Triggerable); ok {
				_ = p.Trigger(ctx)
			}
			close(done)
		case done := <-t.statusTrigger:
			if p, ok := t.manager.Plugin(status.Name).(plugins.Triggerable); ok {
				_ = p.Trigger(ctx)
			}
			close(done)
		}
	}
}

func (t *testFixture) runQuery(ctx context.Context, query string, m metrics.Metrics) (interface{}, error) {
	r := rego.New(
		rego.Query(query),
		rego.Store(t.manager.Store),
		rego.Metrics(m),
	)

	//Run evaluation.
	rs, err := r.Eval(ctx)
	if err != nil {
		return nil, err
	}

	if len(rs) == 0 {
		return nil, nil
	}

	return rs[0].Expressions[0].Value, nil
}

func (t *testFixture) log(ctx context.Context, query string, m metrics.Metrics, result *interface{}) error {

	record := server.Info{
		Timestamp: time.Now(),
		Path:      query,
		Metrics:   m,
		Results:   result,
	}

	if logger := logs.Lookup(t.manager); logger != nil {
		if err := logger.Log(ctx, &record); err != nil {
			return fmt.Errorf("decision log: %w", err)
		}
	}
	return nil
}

func (t *testFixture) testServiceBundleUpdateScenario(ctx context.Context, m metrics.Metrics) {
	t.server.bundleData = map[string]interface{}{
		"foo": map[string]interface{}{
			"bar": "world",
		},
	}
	t.server.bundleRevision = "def"

	trigger := make(chan struct{})
	t.bundleTrigger <- trigger
	<-trigger

	// run query
	result, err := t.runQuery(ctx, "data.foo.bar", m)
	if err != nil {
		t.server.t.Fatal(err)
	}

	expected := "world"
	if result != expected {
		t.server.t.Fatalf("Expected result %v but got %v", expected, result)
	}

	// log result
	err = t.log(ctx, "data.foo.bar", m, &result)
	if err != nil {
		t.server.t.Fatal(err)
	}

	// trigger the decision log plugin
	trigger = make(chan struct{})
	t.decisionLogTrigger <- trigger
	<-trigger

	expectedNum := 3
	if len(t.server.logEvent) != expectedNum {
		t.server.t.Fatalf("Expected %d decision log events, found %d", expectedNum, len(t.server.logEvent))
	}

	// verify the result in the last log
	if *t.server.logEvent[2].Result != expected {
		t.server.t.Fatalf("Expected result %v but got %v", expected, result)
	}

	// trigger the status plugin (there should be a pending update corresponding to the last service bundle activation)
	trigger = make(chan struct{})
	t.statusTrigger <- trigger
	<-trigger

	expectedNum = 2
	if len(t.server.statusEvent) != expectedNum {
		t.server.t.Fatalf("Expected %d status updates, found %d", expectedNum, len(t.server.statusEvent))
	}

	// verify the updated bundle revision in the last status update
	bundles, _ := t.server.statusEvent[1].(map[string]interface{})
	actual := bundles["bundles"].(map[string]interface{})["authz"].(map[string]interface{})["active_revision"]

	if actual != t.server.bundleRevision {
		t.server.t.Fatalf("Expected revision %v but got %v", t.server.bundleRevision, actual)
	}
}

func (t *testFixture) testDiscoReconfigurationScenario(ctx context.Context, m metrics.Metrics) {
	t.server.discoConfig = `
		{
			"config": {
				"bundles": {
					"authz": {
						"service": "example",
                        "resource": "newbundles/authz",
						"trigger": "manual"
					}
				},
				"status": {"service": "example", "trigger": "manual", "partition_name": "new"},
				"decision_logs": {"service": "example", "resource": "newlogs", "reporting": {"trigger": "manual"}}
			}
		}`

	t.server.dicsoBundleRev = 3

	trigger := make(chan struct{})
	t.discoTrigger <- trigger
	<-trigger

	// trigger the bundle plugin
	t.server.bundleData = map[string]interface{}{
		"bux": map[string]interface{}{
			"qux": "hello again!",
		},
	}
	t.server.bundleRevision = "ghi"

	trigger = make(chan struct{})
	t.bundleTrigger <- trigger
	<-trigger

	// run query
	result, err := t.runQuery(ctx, "data.bux.qux", m)
	if err != nil {
		t.server.t.Fatal(err)
	}

	expected := "hello again!"
	if result != expected {
		t.server.t.Fatalf("Expected result %v but got %v", expected, result)
	}

	// trigger the status plugin (there should be pending updates corresponding to the last discovery and service bundle activation)
	trigger = make(chan struct{})
	t.statusTrigger <- trigger
	<-trigger

	expectedNum := 4
	if len(t.server.statusEvent) != expectedNum {
		t.server.t.Fatalf("Expected %d status updates, found %d", expectedNum, len(t.server.statusEvent))
	}

	// verify the updated discovery and service bundle revisions in the last status update
	bundles, _ := t.server.statusEvent[3].(map[string]interface{})
	actual := bundles["bundles"].(map[string]interface{})["authz"].(map[string]interface{})["active_revision"]

	if actual != t.server.bundleRevision {
		t.server.t.Fatalf("Expected revision %v but got %v", t.server.bundleRevision, actual)
	}

	disco, _ := t.server.statusEvent[3].(map[string]interface{})
	actual = disco["discovery"].(map[string]interface{})["active_revision"]

	expectedRev := fmt.Sprintf("test-revision-%v", t.server.dicsoBundleRev)
	if actual != expectedRev {
		t.server.t.Fatalf("Expected discovery bundle revision %v but got %v", expectedRev, actual)
	}
}

func (t *testFixture) stop() {
	done := make(chan struct{})
	t.stopCh <- done
	<-done

	t.server.stop()
}

type testFixtureServer struct {
	t              *testing.T
	server         *httptest.Server
	discoConfig    string
	dicsoBundleRev int
	bundleData     map[string]interface{}
	bundleRevision string
	statusEvent    []interface{}
	logEvent       []logs.EventV1
}

func (t *testFixtureServer) handle(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/bundles/disco" {
		// prepare a discovery bundle with some configured plugins

		b := makeDataBundle(t.dicsoBundleRev, t.discoConfig)

		err := bundleApi.NewWriter(w).Write(*b)
		if err != nil {
			t.t.Fatal(err)
		}
	} else if r.URL.Path == "/bundles/authz" || r.URL.Path == "/newbundles/authz" {
		// prepare a regular bundle

		b := bundleApi.Bundle{
			Data:     t.bundleData,
			Manifest: bundleApi.Manifest{Revision: t.bundleRevision},
		}

		err := bundleApi.NewWriter(w).Write(b)
		if err != nil {
			t.t.Fatal(err)
		}
	} else if r.URL.Path == "/status" || r.URL.Path == "/status/new" {

		var event interface{}

		if err := util.NewJSONDecoder(r.Body).Decode(&event); err != nil {
			t.t.Fatal(err)
		}

		t.statusEvent = append(t.statusEvent, event)

	} else if r.URL.Path == "/logs" || r.URL.Path == "/newlogs" {
		gr, err := gzip.NewReader(r.Body)
		if err != nil {
			t.t.Fatal(err)
		}
		var events []logs.EventV1
		if err := json.NewDecoder(gr).Decode(&events); err != nil {
			t.t.Fatal(err)
		}
		if err := gr.Close(); err != nil {
			t.t.Fatal(err)
		}

		t.logEvent = append(t.logEvent, events...)

	} else {
		t.t.Fatalf("unknown path %v", r.URL.Path)
	}

}

func (t *testFixtureServer) start() {
	t.server = httptest.NewServer(http.HandlerFunc(t.handle))
}

func (t *testFixtureServer) stop() {
	t.server.Close()
}

func ensurePluginState(t *testing.T, d *Discovery, state plugins.State) {
	t.Helper()
	status, ok := d.manager.PluginStatus()[Name]
	if !ok {
		t.Fatalf("Expected to find state for %s, found nil", Name)
		return
	}
	if status.State != state {
		t.Fatalf("Unexpected status state found in plugin manager for %s:\n\n\tFound:%+v\n\n\tExpected: %s", Name, status.State, state)
	}
}
