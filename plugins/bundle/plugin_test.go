// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package bundle

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/open-policy-agent/opa/keys"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/bundle"
	"github.com/open-policy-agent/opa/config"
	"github.com/open-policy-agent/opa/download"
	"github.com/open-policy-agent/opa/metrics"
	"github.com/open-policy-agent/opa/plugins"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/storage/inmem"
	"github.com/open-policy-agent/opa/util"
)

func TestPluginOneShot(t *testing.T) {

	ctx := context.Background()
	manager := getTestManager()
	plugin := New(&Config{}, manager)
	bundleName := "test-bundle"
	plugin.status[bundleName] = &Status{Name: bundleName, Metrics: metrics.New()}
	plugin.downloaders[bundleName] = download.New(download.Config{}, plugin.manager.Client(""), bundleName)

	ensurePluginState(t, plugin, plugins.StateNotReady)

	module := "package foo\n\ncorge=1"

	b := bundle.Bundle{
		Manifest: bundle.Manifest{Revision: "quickbrownfaux"},
		Data:     util.MustUnmarshalJSON([]byte(`{"foo": {"bar": 1, "baz": "qux"}}`)).(map[string]interface{}),
		Modules: []bundle.ModuleFile{
			bundle.ModuleFile{
				Path:   "/foo/bar",
				Parsed: ast.MustParseModule(module),
				Raw:    []byte(module),
			},
		},
	}

	b.Manifest.Init()

	plugin.oneShot(ctx, bundleName, download.Update{Bundle: &b, Metrics: metrics.New()})

	ensurePluginState(t, plugin, plugins.StateOK)

	txn := storage.NewTransactionOrDie(ctx, manager.Store)
	defer manager.Store.Abort(ctx, txn)

	ids, err := manager.Store.ListPolicies(ctx, txn)
	if err != nil {
		t.Fatal(err)
	} else if len(ids) != 1 {
		t.Fatal("Expected 1 policy")
	}

	bs, err := manager.Store.GetPolicy(ctx, txn, ids[0])
	exp := []byte("package foo\n\ncorge=1")
	if err != nil {
		t.Fatal(err)
	} else if !bytes.Equal(bs, exp) {
		t.Fatalf("Bad policy content. Exp:\n%v\n\nGot:\n\n%v", string(exp), string(bs))
	}

	data, err := manager.Store.Read(ctx, txn, storage.Path{})
	expData := util.MustUnmarshalJSON([]byte(`{"foo": {"bar": 1, "baz": "qux"}, "system": {"bundles": {"test-bundle": {"manifest": {"revision": "quickbrownfaux", "roots": [""]}}}}}`))
	if err != nil {
		t.Fatal(err)
	} else if !reflect.DeepEqual(data, expData) {
		t.Fatalf("Bad data content. Exp:\n%v\n\nGot:\n\n%v", expData, data)
	}
}

func TestPluginStart(t *testing.T) {

	ctx := context.Background()
	manager := getTestManager()
	bundles := map[string]*Source{}

	plugin := New(&Config{Bundles: bundles}, manager)
	err := plugin.Start(ctx)
	if err != nil {
		t.Fatal("unexpected error:", err)
	}
}

func TestPluginOneShotBundlePersistence(t *testing.T) {

	ctx := context.Background()
	manager := getTestManager()

	dir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	defer os.RemoveAll(dir)

	bundleName := "test-bundle"
	bundleSource := Source{
		Persist: true,
	}

	bundles := map[string]*Source{}
	bundles[bundleName] = &bundleSource

	plugin := New(&Config{Bundles: bundles}, manager)

	plugin.status[bundleName] = &Status{Name: bundleName, Metrics: metrics.New()}
	plugin.downloaders[bundleName] = download.New(download.Config{}, plugin.manager.Client(""), bundleName)
	plugin.bundlePersistPath = filepath.Join(dir, ".opa")

	ensurePluginState(t, plugin, plugins.StateNotReady)

	// simulate a bundle download error with no bundle on disk
	plugin.oneShot(ctx, bundleName, download.Update{Error: fmt.Errorf("unknown error")})

	if plugin.status[bundleName].Message == "" {
		t.Fatal("expected error but got none")
	}

	ensurePluginState(t, plugin, plugins.StateNotReady)

	// download a bundle and persist to disk. Then verify the bundle persisted to disk
	module := "package foo\n\ncorge=1"

	b := bundle.Bundle{
		Manifest: bundle.Manifest{Revision: "quickbrownfaux"},
		Data:     util.MustUnmarshalJSON([]byte(`{"foo": {"bar": 1, "baz": "qux"}}`)).(map[string]interface{}),
		Modules: []bundle.ModuleFile{
			bundle.ModuleFile{
				URL:    "/foo/bar.rego",
				Path:   "/foo/bar.rego",
				Parsed: ast.MustParseModule(module),
				Raw:    []byte(module),
			},
		},
	}

	b.Manifest.Init()

	plugin.oneShot(ctx, bundleName, download.Update{Bundle: &b, Metrics: metrics.New()})

	ensurePluginState(t, plugin, plugins.StateOK)

	result, err := loadBundleFromDisk(plugin.bundlePersistPath, bundleName, nil)
	if err != nil {
		t.Fatal("unexpected error:", err)
	}

	if !result.Equal(b) {
		t.Fatal("expected the downloaded bundle to be equal to the one loaded from disk")
	}

	// simulate a bundle download error and verify that the bundle on disk is activated
	plugin.oneShot(ctx, bundleName, download.Update{Error: fmt.Errorf("unknown error")})

	ensurePluginState(t, plugin, plugins.StateOK)

	txn := storage.NewTransactionOrDie(ctx, manager.Store)
	defer manager.Store.Abort(ctx, txn)

	ids, err := manager.Store.ListPolicies(ctx, txn)
	if err != nil {
		t.Fatal(err)
	} else if len(ids) != 1 {
		t.Fatal("Expected 1 policy")
	}

	bs, err := manager.Store.GetPolicy(ctx, txn, ids[0])
	exp := []byte("package foo\n\ncorge=1")
	if err != nil {
		t.Fatal(err)
	} else if !bytes.Equal(bs, exp) {
		t.Fatalf("Bad policy content. Exp:\n%v\n\nGot:\n\n%v", string(exp), string(bs))
	}

	data, err := manager.Store.Read(ctx, txn, storage.Path{})
	expData := util.MustUnmarshalJSON([]byte(`{"foo": {"bar": 1, "baz": "qux"}, "system": {"bundles": {"test-bundle": {"manifest": {"revision": "quickbrownfaux", "roots": [""]}}}}}`))
	if err != nil {
		t.Fatal(err)
	} else if !reflect.DeepEqual(data, expData) {
		t.Fatalf("Bad data content. Exp:\n%v\n\nGot:\n\n%v", expData, data)
	}
}

func TestLoadAndActivateBundlesFromDisk(t *testing.T) {

	ctx := context.Background()
	manager := getTestManager()

	dir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	defer os.RemoveAll(dir)

	bundleName := "test-bundle"
	bundleSource := Source{
		Persist: true,
	}

	bundleNameOther := "test-bundle-other"
	bundleSourceOther := Source{}

	bundles := map[string]*Source{}
	bundles[bundleName] = &bundleSource
	bundles[bundleNameOther] = &bundleSourceOther

	plugin := New(&Config{Bundles: bundles}, manager)
	plugin.bundlePersistPath = filepath.Join(dir, ".opa")

	err = plugin.loadAndActivateBundlesFromDisk(ctx)
	if err != nil {
		t.Fatal("unexpected error:", err)
	}

	// persist a bundle to disk and then load it
	module := "package foo\n\ncorge=1"

	b := bundle.Bundle{
		Manifest: bundle.Manifest{Revision: "quickbrownfaux"},
		Data:     util.MustUnmarshalJSON([]byte(`{"foo": {"bar": 1, "baz": "qux"}}`)).(map[string]interface{}),
		Modules: []bundle.ModuleFile{
			bundle.ModuleFile{
				URL:    "/foo/bar.rego",
				Path:   "/foo/bar.rego",
				Parsed: ast.MustParseModule(module),
				Raw:    []byte(module),
			},
		},
	}

	b.Manifest.Init()

	err = plugin.saveBundleToDisk(bundleName, &b)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	err = plugin.loadAndActivateBundlesFromDisk(ctx)
	if err != nil {
		t.Fatal("unexpected error:", err)
	}

	txn := storage.NewTransactionOrDie(ctx, manager.Store)
	defer manager.Store.Abort(ctx, txn)

	ids, err := manager.Store.ListPolicies(ctx, txn)
	if err != nil {
		t.Fatal(err)
	} else if len(ids) != 1 {
		t.Fatal("Expected 1 policy")
	}

	bs, err := manager.Store.GetPolicy(ctx, txn, ids[0])
	exp := []byte("package foo\n\ncorge=1")
	if err != nil {
		t.Fatal(err)
	} else if !bytes.Equal(bs, exp) {
		t.Fatalf("Bad policy content. Exp:\n%v\n\nGot:\n\n%v", string(exp), string(bs))
	}

	data, err := manager.Store.Read(ctx, txn, storage.Path{})
	expData := util.MustUnmarshalJSON([]byte(`{"foo": {"bar": 1, "baz": "qux"}, "system": {"bundles": {"test-bundle": {"manifest": {"revision": "quickbrownfaux", "roots": [""]}}}}}`))
	if err != nil {
		t.Fatal(err)
	} else if !reflect.DeepEqual(data, expData) {
		t.Fatalf("Bad data content. Exp:\n%v\n\nGot:\n\n%v", expData, data)
	}
}

func TestPluginOneShotCompileError(t *testing.T) {

	ctx := context.Background()
	manager := getTestManager()
	plugin := New(&Config{}, manager)
	bundleName := "test-bundle"
	plugin.status[bundleName] = &Status{Name: bundleName}
	plugin.downloaders[bundleName] = download.New(download.Config{}, plugin.manager.Client(""), bundleName)

	ensurePluginState(t, plugin, plugins.StateNotReady)

	raw1 := "package foo\n\np[x] { x = 1 }"

	b1 := &bundle.Bundle{
		Data: map[string]interface{}{"a": "b"},
		Modules: []bundle.ModuleFile{
			{
				Path:   "/example.rego",
				Raw:    []byte(raw1),
				Parsed: ast.MustParseModule(raw1),
			},
		},
	}

	b1.Manifest.Init()
	plugin.oneShot(ctx, bundleName, download.Update{Bundle: b1, Metrics: metrics.New()})

	ensurePluginState(t, plugin, plugins.StateOK)

	b2 := &bundle.Bundle{
		Data: map[string]interface{}{"a": "b"},
		Modules: []bundle.ModuleFile{
			{
				Path:   "/example2.rego",
				Parsed: ast.MustParseModule("package foo\n\np[x]"),
			},
		},
	}

	b2.Manifest.Init()
	plugin.oneShot(ctx, bundleName, download.Update{Bundle: b2})

	ensurePluginState(t, plugin, plugins.StateOK)

	txn := storage.NewTransactionOrDie(ctx, manager.Store)

	_, err := manager.Store.GetPolicy(ctx, txn, filepath.Join(bundleName, "/example.rego"))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	data, err := manager.Store.Read(ctx, txn, storage.Path{"a"})
	if err != nil || !reflect.DeepEqual("b", data) {
		t.Fatalf("Expected data to be intact but got: %v, err: %v", data, err)
	}

	manager.Store.Abort(ctx, txn)

	b3 := &bundle.Bundle{
		Data: map[string]interface{}{"foo": map[string]interface{}{"p": "a"}},
		Modules: []bundle.ModuleFile{
			{
				Path:   "/example3.rego",
				Parsed: ast.MustParseModule("package foo\np=1"),
			},
		},
	}

	b3.Manifest.Init()
	plugin.oneShot(ctx, bundleName, download.Update{Bundle: b3})

	ensurePluginState(t, plugin, plugins.StateOK)

	txn = storage.NewTransactionOrDie(ctx, manager.Store)

	_, err = manager.Store.GetPolicy(ctx, txn, filepath.Join(bundleName, "/example.rego"))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	data, err = manager.Store.Read(ctx, txn, storage.Path{"a"})
	if err != nil || !reflect.DeepEqual("b", data) {
		t.Fatalf("Expected data to be intact but got: %v, err: %v", data, err)
	}

}

func TestPluginOneShotActivationRemovesOld(t *testing.T) {

	ctx := context.Background()
	manager := getTestManager()
	plugin := New(&Config{}, manager)
	bundleName := "test-bundle"
	plugin.status[bundleName] = &Status{Name: bundleName}
	plugin.downloaders[bundleName] = download.New(download.Config{}, plugin.manager.Client(""), bundleName)

	ensurePluginState(t, plugin, plugins.StateNotReady)

	module1 := `package example

		p = 1`

	b1 := bundle.Bundle{
		Data: map[string]interface{}{
			"foo": "bar",
		},
		Modules: []bundle.ModuleFile{
			{
				Path:   "/example.rego",
				Raw:    []byte(module1),
				Parsed: ast.MustParseModule(module1),
			},
		},
	}

	b1.Manifest.Init()
	plugin.oneShot(ctx, bundleName, download.Update{Bundle: &b1})

	ensurePluginState(t, plugin, plugins.StateOK)

	module2 := `package example

		p = 2`

	b2 := bundle.Bundle{
		Data: map[string]interface{}{
			"baz": "qux",
		},
		Modules: []bundle.ModuleFile{
			{
				Path:   "/example2.rego",
				Raw:    []byte(module2),
				Parsed: ast.MustParseModule(module2),
			},
		},
	}

	b2.Manifest.Init()
	plugin.oneShot(ctx, bundleName, download.Update{Bundle: &b2})

	ensurePluginState(t, plugin, plugins.StateOK)

	err := storage.Txn(ctx, manager.Store, storage.TransactionParams{}, func(txn storage.Transaction) error {
		ids, err := manager.Store.ListPolicies(ctx, txn)
		if err != nil {
			return err
		} else if !reflect.DeepEqual([]string{filepath.Join(bundleName, "/example2.rego")}, ids) {
			return fmt.Errorf("expected updated policy ids")
		}
		data, err := manager.Store.Read(ctx, txn, storage.Path{})
		// remove system key to make comparison simpler
		delete(data.(map[string]interface{}), "system")
		if err != nil {
			return err
		} else if !reflect.DeepEqual(data, map[string]interface{}{"baz": "qux"}) {
			return fmt.Errorf("expected updated data")
		}
		return nil
	})

	if err != nil {
		t.Fatal("Unexpected:", err)
	}
}

func TestPluginOneShotActivationConflictingRoots(t *testing.T) {
	ctx := context.Background()
	manager := getTestManager()
	plugin := New(&Config{}, manager)

	ensurePluginState(t, plugin, plugins.StateNotReady)

	bundleNames := []string{"test-bundle1", "test-bundle2", "test-bundle3"}

	for _, name := range bundleNames {
		plugin.status[name] = &Status{Name: name}
		plugin.downloaders[name] = download.New(download.Config{}, plugin.manager.Client(""), name)
	}

	// Start with non-conflicting updates
	plugin.oneShot(ctx, bundleNames[0], download.Update{Bundle: &bundle.Bundle{
		Manifest: bundle.Manifest{
			Roots: &[]string{"a/b"},
		},
	}})

	ensurePluginState(t, plugin, plugins.StateNotReady)

	plugin.oneShot(ctx, bundleNames[1], download.Update{Bundle: &bundle.Bundle{
		Manifest: bundle.Manifest{
			Roots: &[]string{"a/c"},
		},
	}})

	ensurePluginState(t, plugin, plugins.StateNotReady)

	// ensure that both bundles are *not* in error status
	ensureBundleOverlapStatus(t, plugin, bundleNames, []bool{false, false, false})

	// Add a third bundle that conflicts with one
	plugin.oneShot(ctx, bundleNames[2], download.Update{Bundle: &bundle.Bundle{
		Manifest: bundle.Manifest{
			Roots: &[]string{"a/b/aa"},
		},
	}})

	ensurePluginState(t, plugin, plugins.StateNotReady)

	// ensure that both in the conflict go into error state
	ensureBundleOverlapStatus(t, plugin, bundleNames, []bool{false, false, true})

	// Update to fix conflict
	plugin.oneShot(ctx, bundleNames[2], download.Update{Bundle: &bundle.Bundle{
		Manifest: bundle.Manifest{
			Roots: &[]string{"b"},
		},
	}})

	ensurePluginState(t, plugin, plugins.StateOK)
	ensureBundleOverlapStatus(t, plugin, bundleNames, []bool{false, false, false})

	// Ensure empty roots conflict with all roots
	plugin.oneShot(ctx, bundleNames[2], download.Update{Bundle: &bundle.Bundle{
		Manifest: bundle.Manifest{
			Roots: &[]string{""},
		},
	}})

	ensurePluginState(t, plugin, plugins.StateOK)
	ensureBundleOverlapStatus(t, plugin, bundleNames, []bool{false, false, true})
}

func TestPluginOneShotActivationPrefixMatchingRoots(t *testing.T) {
	ctx := context.Background()
	manager := getTestManager()
	plugin := Plugin{manager: manager, status: map[string]*Status{}, etags: map[string]string{}, downloaders: map[string]*download.Downloader{}}
	bundleNames := []string{"test-bundle1", "test-bundle2"}

	for _, name := range bundleNames {
		plugin.status[name] = &Status{Name: name}
		plugin.downloaders[name] = download.New(download.Config{}, plugin.manager.Client(""), name)
	}

	plugin.oneShot(ctx, bundleNames[0], download.Update{Bundle: &bundle.Bundle{
		Manifest: bundle.Manifest{
			Roots: &[]string{"a/b/c"},
		},
	}})

	plugin.oneShot(ctx, bundleNames[1], download.Update{Bundle: &bundle.Bundle{
		Manifest: bundle.Manifest{
			Roots: &[]string{"a/b/cat"},
		},
	}})

	ensureBundleOverlapStatus(t, &plugin, bundleNames, []bool{false, false})

	// Ensure that empty roots conflict
	plugin.oneShot(ctx, bundleNames[1], download.Update{Bundle: &bundle.Bundle{
		Manifest: bundle.Manifest{
			Roots: &[]string{""},
		},
	}})

	ensureBundleOverlapStatus(t, &plugin, bundleNames, []bool{false, true})

}

func ensureBundleOverlapStatus(t *testing.T, p *Plugin, bundleNames []string, expectedErrs []bool) {
	t.Helper()
	for i, name := range bundleNames {
		hasErr := p.status[name].Message != ""
		if expectedErrs[i] && !hasErr {
			t.Fatalf("expected bundle %s to be in an error state", name)
		} else if !expectedErrs[i] && hasErr {
			t.Fatalf("unexpected error state for bundle %s", name)
		} else if hasErr && expectedErrs[i] && !strings.Contains(p.status[name].Message, "detected overlapping roots") {
			t.Fatalf("expected bundle overlap error for bundle %s, got: %s", name, p.status[name].Message)
		}
	}
}

func TestPluginListener(t *testing.T) {

	ctx := context.Background()
	manager := getTestManager()
	plugin := New(&Config{}, manager)
	bundleName := "test-bundle"
	plugin.status[bundleName] = &Status{Name: bundleName}
	plugin.downloaders[bundleName] = download.New(download.Config{}, plugin.manager.Client(""), bundleName)
	ch := make(chan Status)

	plugin.Register("test", func(status Status) {
		ch <- status
	})

	module := "package gork\np[x] { x = 1 }"

	b := bundle.Bundle{
		Manifest: bundle.Manifest{
			Revision: "quickbrownfaux",
		},
		Data: map[string]interface{}{},
		Modules: []bundle.ModuleFile{
			{
				Path:   "/foo.rego",
				Parsed: ast.MustParseModule(module),
				Raw:    []byte(module),
			},
		},
	}

	b.Manifest.Init()

	// Test that initial bundle is ok. Defer to separate goroutine so we can
	// check result with channel.
	go plugin.oneShot(ctx, bundleName, download.Update{Bundle: &b})
	s1 := <-ch

	validateStatus(t, s1, "quickbrownfaux", false)

	module = "package gork\np[x]"

	b.Manifest.Revision = "slowgreenburd"
	b.Modules[0] = bundle.ModuleFile{
		Path:   "/foo.rego",
		Raw:    []byte(module),
		Parsed: ast.MustParseModule(module),
	}

	// Test that next update is failed.
	go plugin.oneShot(ctx, bundleName, download.Update{Bundle: &b})
	s2 := <-ch

	validateStatus(t, s2, "quickbrownfaux", true)

	module = "package gork\np[1]"
	b.Manifest.Revision = "fancybluederg"
	b.Modules[0] = bundle.ModuleFile{
		Path:   "/foo.rego",
		Raw:    []byte(module),
		Parsed: ast.MustParseModule(module),
	}

	// Test that the new update is successful.
	go plugin.oneShot(ctx, bundleName, download.Update{Bundle: &b})
	s3 := <-ch

	validateStatus(t, s3, "fancybluederg", false)

	// Test that empty download update results in status update.
	go plugin.oneShot(ctx, bundleName, download.Update{})
	s4 := <-ch

	// Nothing should have changed in the update
	validateStatus(t, s4, s3.ActiveRevision, false)
}

func isErrStatus(s Status) bool {
	return s.Code != "" || len(s.Errors) != 0 || s.Message != ""
}

func validateStatus(t *testing.T, actual Status, expected string, expectStatusErr bool) {
	t.Helper()

	if expectStatusErr && !isErrStatus(actual) {
		t.Errorf("Expected status to be in an error state, but no error has occured.")
	} else if !expectStatusErr && isErrStatus(actual) {
		t.Errorf("Unexpected error status %s", actual)
	}

	if actual.ActiveRevision != expected {
		t.Errorf("Expected status revision %s, got %s", expected, actual.ActiveRevision)
	}
}

func TestPluginListenerErrorClearedOn304(t *testing.T) {
	ctx := context.Background()
	manager := getTestManager()
	plugin := Plugin{manager: manager, status: map[string]*Status{}, etags: map[string]string{}, downloaders: map[string]*download.Downloader{}}
	bundleName := "test-bundle"
	plugin.status[bundleName] = &Status{Name: bundleName}
	plugin.downloaders[bundleName] = download.New(download.Config{}, plugin.manager.Client(""), bundleName)
	ch := make(chan Status)

	plugin.Register("test", func(status Status) {
		ch <- status
	})

	b := bundle.Bundle{
		Manifest: bundle.Manifest{
			Revision: "quickbrownfaux",
		},
		Data: map[string]interface{}{"foo": "bar"},
	}

	b.Manifest.Init()

	// Test that initial bundle is ok.
	go plugin.oneShot(ctx, bundleName, download.Update{Bundle: &b})
	s1 := <-ch

	if s1.ActiveRevision != "quickbrownfaux" || s1.Code != "" {
		t.Fatal("Unexpected status update, got:", s1)
	}

	// Test that service error triggers failure notification.
	go plugin.oneShot(ctx, bundleName, download.Update{Error: fmt.Errorf("some error")})
	s2 := <-ch

	if s2.ActiveRevision != "quickbrownfaux" || s2.Code == "" {
		t.Fatal("Unexpected status update, got:", s2)
	}

	// Test that service recovery triggers healthy notification.
	go plugin.oneShot(ctx, bundleName, download.Update{})
	s3 := <-ch

	if s3.ActiveRevision != "quickbrownfaux" || s3.Code != "" {
		t.Fatal("Unexpected status update, got:", s3)
	}
}

func TestPluginBulkListener(t *testing.T) {
	ctx := context.Background()
	manager := getTestManager()
	plugin := Plugin{manager: manager, status: map[string]*Status{}, etags: map[string]string{}, downloaders: map[string]*download.Downloader{}}
	bundleNames := []string{
		"b1",
		"b2",
		"b3",
	}
	for _, name := range bundleNames {
		plugin.status[name] = &Status{Name: name}
		plugin.downloaders[name] = download.New(download.Config{}, plugin.manager.Client(""), name)
	}
	bulkChan := make(chan map[string]*Status)

	plugin.RegisterBulkListener("bulk test", func(status map[string]*Status) {
		bulkChan <- status
	})

	module := "package gork\np[x] { x = 1 }"

	b := bundle.Bundle{
		Manifest: bundle.Manifest{
			Revision: "quickbrownfaux",
			Roots:    &[]string{"gork"},
		},
		Data: map[string]interface{}{},
		Modules: []bundle.ModuleFile{
			{
				Path:   "/foo.rego",
				Parsed: ast.MustParseModule(module),
				Raw:    []byte(module),
			},
		},
	}

	b.Manifest.Init()

	// Test that initial bundle is ok. Defer to separate goroutine so we can
	// check result with channel.
	go plugin.oneShot(ctx, bundleNames[0], download.Update{Bundle: &b})
	s1 := <-bulkChan

	s := s1[bundleNames[0]]
	if s.ActiveRevision != "quickbrownfaux" || s.Code != "" {
		t.Fatal("Unexpected status update, got:", s1)
	}

	for i := 1; i < len(bundleNames); i++ {
		name := bundleNames[i]
		s, ok := s1[name]
		if !ok {
			t.Errorf("Expected to have bundle status for %q included in update, got: %+v", name, s1)
		}
		// they should be defaults at this point
		if !reflect.DeepEqual(s, &Status{Name: name}) {
			t.Errorf("Expected bundle %q to have an empty status, got: %+v", name, s1)
		}
	}

	module = "package gork\np[x]"

	b.Manifest.Revision = "slowgreenburd"
	b.Modules[0] = bundle.ModuleFile{
		Path:   "/foo.rego",
		Raw:    []byte(module),
		Parsed: ast.MustParseModule(module),
	}

	// Test that next update is failed.
	go plugin.oneShot(ctx, bundleNames[0], download.Update{Bundle: &b})
	s2 := <-bulkChan

	s = s2[bundleNames[0]]
	if s.ActiveRevision != "quickbrownfaux" || s.Code == "" || s.Message == "" || len(s.Errors) == 0 {
		t.Fatal("Unexpected status update, got:", s2)
	}

	for i := 1; i < len(bundleNames); i++ {
		name := bundleNames[i]
		s, ok := s2[name]
		if !ok {
			t.Errorf("Expected to have bundle status for %q included in update, got: %+v", name, s2)
		}
		// they should be still defaults
		if !reflect.DeepEqual(s, &Status{Name: name}) {
			t.Errorf("Expected bundle %q to have an empty status, got: %+v", name, s2)
		}
	}

	module = "package gork\np[1]"
	b.Manifest.Revision = "fancybluederg"
	b.Modules[0] = bundle.ModuleFile{
		Path:   "/foo.rego",
		Raw:    []byte(module),
		Parsed: ast.MustParseModule(module),
	}

	// Test that new update is successful.
	go plugin.oneShot(ctx, bundleNames[0], download.Update{Bundle: &b})
	s3 := <-bulkChan

	s = s3[bundleNames[0]]
	if s.ActiveRevision != "fancybluederg" || s.Code != "" || s.Message != "" || len(s.Errors) != 0 {
		t.Fatal("Unexpected status update, got:", s3)
	}

	for i := 1; i < len(bundleNames); i++ {
		name := bundleNames[i]
		s, ok := s3[name]
		if !ok {
			t.Errorf("Expected to have bundle status for %q included in update, got: %+v", name, s3)
		}
		// they should still be defaults
		if !reflect.DeepEqual(s, &Status{Name: name}) {
			t.Errorf("Expected bundle %q to have an empty status, got: %+v", name, s3)
		}
	}

	// Test that empty download update results in status update.
	go plugin.oneShot(ctx, bundleNames[0], download.Update{})
	s4 := <-bulkChan

	if !reflect.DeepEqual(s3, s4) {
		t.Fatalf("Expected: %v but got: %v", s3, s4)
	}

	// Test updates the other bundles
	module = "package p1\np[x] { x = 1 }"

	b1 := bundle.Bundle{
		Manifest: bundle.Manifest{
			Revision: "123",
			Roots:    &[]string{"p1"},
		},
		Data: map[string]interface{}{},
		Modules: []bundle.ModuleFile{
			{
				Path:   "/foo1.rego",
				Parsed: ast.MustParseModule(module),
				Raw:    []byte(module),
			},
		},
	}

	b1.Manifest.Init()

	// Test that new update is successful.
	go plugin.oneShot(ctx, bundleNames[1], download.Update{Bundle: &b1})
	s5 := <-bulkChan

	s = s5[bundleNames[1]]
	if s.ActiveRevision != "123" || s.Code != "" || s.Message != "" || len(s.Errors) != 0 {
		t.Fatal("Unexpected status update, got:", s5)
	}

	if !reflect.DeepEqual(s5[bundleNames[0]], s4[bundleNames[0]]) {
		t.Fatalf("Expected bundle %q to have the same status as before updating bundle %q, got: %+v", bundleNames[0], bundleNames[1], s5)
	}

	for i := 2; i < len(bundleNames); i++ {
		name := bundleNames[i]
		s, ok := s5[name]
		if !ok {
			t.Errorf("Expected to have bundle status for %q included in update, got: %+v", name, s5)
		}
		// they should still be defaults
		if !reflect.DeepEqual(s, &Status{Name: name}) {
			t.Errorf("Expected bundle %q to have an empty status, got: %+v", name, s5)
		}
	}
}

func TestPluginBulkListenerStatusCopyOnly(t *testing.T) {
	ctx := context.Background()
	manager := getTestManager()
	plugin := Plugin{manager: manager, status: map[string]*Status{}, etags: map[string]string{}, downloaders: map[string]*download.Downloader{}}
	bundleNames := []string{
		"b1",
		"b2",
		"b3",
	}
	for _, name := range bundleNames {
		plugin.status[name] = &Status{Name: name}
		plugin.downloaders[name] = download.New(download.Config{}, plugin.manager.Client(""), name)
	}
	bulkChan := make(chan map[string]*Status)

	plugin.RegisterBulkListener("bulk test", func(status map[string]*Status) {
		bulkChan <- status
	})

	module := "package gork\np[x] { x = 1 }"

	b := bundle.Bundle{
		Manifest: bundle.Manifest{
			Revision: "quickbrownfaux",
			Roots:    &[]string{"gork"},
		},
		Data: map[string]interface{}{},
		Modules: []bundle.ModuleFile{
			{
				Path:   "/foo.rego",
				Parsed: ast.MustParseModule(module),
				Raw:    []byte(module),
			},
		},
	}

	b.Manifest.Init()

	// Test that initial bundle is ok. Defer to separate goroutine so we can
	// check result with channel.
	go plugin.oneShot(ctx, bundleNames[0], download.Update{Bundle: &b})
	s1 := <-bulkChan

	// Modify the status map received and ensure it doesn't affect the one on the plugin
	delete(s1, "b1")

	if _, ok := plugin.status["b1"]; !ok {
		t.Fatalf("Expected status for 'b1' to still be in 'plugin.status'")
	}
}

func TestPluginActivateScopedBundle(t *testing.T) {

	ctx := context.Background()
	manager := getTestManager()
	plugin := Plugin{manager: manager, status: map[string]*Status{}, etags: map[string]string{}, downloaders: map[string]*download.Downloader{}}
	bundleName := "test-bundle"
	plugin.status[bundleName] = &Status{Name: bundleName}
	plugin.downloaders[bundleName] = download.New(download.Config{}, plugin.manager.Client(""), bundleName)

	// Transact test data and policies that represent data coming from
	// _outside_ the bundle. The test will verify that data _outside_
	// the bundle is both not erased and is overwritten appropriately.
	//
	// The test data claims a/{a1-6} where even paths are policy and
	// odd paths are raw JSON.
	if err := storage.Txn(ctx, manager.Store, storage.WriteParams, func(txn storage.Transaction) error {

		externalData := map[string]interface{}{"a": map[string]interface{}{"a1": "x1", "a3": "x2", "a5": "x3"}}

		if err := manager.Store.Write(ctx, txn, storage.AddOp, storage.Path{}, externalData); err != nil {
			return err
		}
		if err := manager.Store.UpsertPolicy(ctx, txn, "some/id1", []byte(`package a.a2`)); err != nil {
			return err
		}
		if err := manager.Store.UpsertPolicy(ctx, txn, "some/id2", []byte(`package a.a4`)); err != nil {
			return err
		}
		if err := manager.Store.UpsertPolicy(ctx, txn, "some/id3", []byte(`package a.a6`)); err != nil {
			return err
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	// Activate a bundle that is scoped to a/a1 and a/a2. This will
	// erase and overwrite the external data at these paths but leave
	// a3-6 untouched.
	module := "package a.a2\n\nbar=1"

	b := bundle.Bundle{
		Manifest: bundle.Manifest{Revision: "quickbrownfaux", Roots: &[]string{"a/a1", "a/a2"}},
		Data: map[string]interface{}{
			"a": map[string]interface{}{
				"a1": "foo",
			},
		},
		Modules: []bundle.ModuleFile{
			bundle.ModuleFile{
				Path:   "bundle/id1",
				Parsed: ast.MustParseModule(module),
				Raw:    []byte(module),
			},
		},
	}

	b.Manifest.Init()

	plugin.oneShot(ctx, bundleName, download.Update{Bundle: &b})

	// Ensure a/a3-6 are intact. a1-2 are overwritten by bundle, and
	// that the manifest has been written to storage.
	expData := util.MustUnmarshalJSON([]byte(`{"a1": "foo", "a3": "x2", "a5": "x3"}`))
	expIds := []string{filepath.Join(bundleName, "bundle/id1"), "some/id2", "some/id3"}
	validateStoreState(ctx, t, manager.Store, "/a", expData, expIds, bundleName, "quickbrownfaux")

	// Activate a bundle that is scoped to a/a3 ad a/a6. Include a function
	// inside package a.a4 that we can depend on outside of the bundle scope to
	// exercise the compile check with remaining modules.
	module = "package a.a4\n\nbar=1\n\nfunc(x) = x"

	b = bundle.Bundle{
		Manifest: bundle.Manifest{Revision: "quickbrownfaux-2", Roots: &[]string{"a/a3", "a/a4"}},
		Data: map[string]interface{}{
			"a": map[string]interface{}{
				"a3": "foo",
			},
		},
		Modules: []bundle.ModuleFile{
			bundle.ModuleFile{
				Path:   "bundle/id2",
				Parsed: ast.MustParseModule(module),
				Raw:    []byte(module),
			},
		},
	}

	b.Manifest.Init()
	plugin.oneShot(ctx, bundleName, download.Update{Bundle: &b})

	// Ensure a/a5-a6 are intact. a3 and a4 are overwritten by bundle.
	expData = util.MustUnmarshalJSON([]byte(`{"a3": "foo", "a5": "x3"}`))
	expIds = []string{filepath.Join(bundleName, "bundle/id2"), "some/id3"}
	validateStoreState(ctx, t, manager.Store, "/a", expData, expIds, bundleName, "quickbrownfaux-2")

	// Upsert policy outside of bundle scope that depends on bundle.
	if err := storage.Txn(ctx, manager.Store, storage.WriteParams, func(txn storage.Transaction) error {
		return manager.Store.UpsertPolicy(ctx, txn, "not_scoped", []byte("package not_scoped\np { data.a.a4.func(1) = 1 }"))
	}); err != nil {
		t.Fatal(err)
	}

	b = bundle.Bundle{
		Manifest: bundle.Manifest{Revision: "quickbrownfaux-3", Roots: &[]string{"a/a3", "a/a4"}},
		Data:     map[string]interface{}{},
		Modules:  []bundle.ModuleFile{},
	}

	b.Manifest.Init()
	plugin.oneShot(ctx, bundleName, download.Update{Bundle: &b})

	// Ensure bundle activation failed by checking that previous revision is
	// still active.
	expIds = []string{filepath.Join(bundleName, "bundle/id2"), "not_scoped", "some/id3"}
	validateStoreState(ctx, t, manager.Store, "/a", expData, expIds, bundleName, "quickbrownfaux-2")
}

func TestPluginSetCompilerOnContext(t *testing.T) {

	ctx := context.Background()
	manager := getTestManager()
	plugin := Plugin{manager: manager, status: map[string]*Status{}, etags: map[string]string{}, downloaders: map[string]*download.Downloader{}}
	bundleName := "test-bundle"
	plugin.status[bundleName] = &Status{Name: bundleName}
	plugin.downloaders[bundleName] = download.New(download.Config{}, plugin.manager.Client(""), bundleName)

	module := `
		package test

		p = 1
		`

	b := bundle.Bundle{
		Manifest: bundle.Manifest{Revision: "quickbrownfaux"},
		Data:     map[string]interface{}{},
		Modules: []bundle.ModuleFile{
			bundle.ModuleFile{
				Path:   "/test.rego",
				Parsed: ast.MustParseModule(module),
				Raw:    []byte(module),
			},
		},
	}

	b.Manifest.Init()

	events := []storage.TriggerEvent{}

	if err := storage.Txn(ctx, manager.Store, storage.WriteParams, func(txn storage.Transaction) error {
		manager.Store.Register(ctx, txn, storage.TriggerConfig{
			OnCommit: func(ctx context.Context, txn storage.Transaction, event storage.TriggerEvent) {
				events = append(events, event)
			},
		})
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	plugin.oneShot(ctx, bundleName, download.Update{Bundle: &b})

	exp := ast.MustParseModule(module)

	// Expect two events. One for trigger registration, one for policy update.
	if len(events) != 2 {
		t.Fatalf("Expected 2 events but got: %+v", events)
	} else if compiler := plugins.GetCompilerOnContext(events[1].Context); compiler == nil {
		t.Fatalf("Expected compiler on 2nd event but got: %+v", events)
	} else if !compiler.Modules[filepath.Join(bundleName, "/test.rego")].Equal(exp) {
		t.Fatalf("Expected module on compiler but got: %v", compiler.Modules)
	}
}

func getTestManager() *plugins.Manager {
	store := inmem.New()
	manager, err := plugins.New(nil, "test-instance-id", store)
	if err != nil {
		panic(err)
	}
	return manager
}

func TestPluginReconfigure(t *testing.T) {
	tsURLBase := "/opa-test/"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, tsURLBase) {
			t.Fatalf("Invalid request URL path: %s, expected prefix %s", r.URL.Path, tsURLBase)
		}
		fmt.Fprintln(w, "") // Note: this is an invalid bundle and will fail the download
	}))
	defer ts.Close()

	ctx := context.Background()
	manager := getTestManager()

	serviceName := "test-svc"
	err := manager.Reconfigure(&config.Config{
		Services: []byte(fmt.Sprintf("{\"%s\":{ \"url\": \"%s\"}}", serviceName, ts.URL+tsURLBase)),
	})
	if err != nil {
		t.Fatalf("Error configuring plugin manager: %s", err)
	}

	plugin := New(&Config{}, manager)

	var delay int64 = 10
	baseConf := download.Config{Polling: download.PollingConfig{MinDelaySeconds: &delay, MaxDelaySeconds: &delay}}

	// Expect the plugin to emit a "not ready" status update each time we change the configuration
	updateCount := 0
	manager.RegisterPluginStatusListener(t.Name(), func(status map[string]*plugins.Status) {
		updateCount++
		bStatus, ok := status[Name]
		if !ok {
			t.Errorf("Expected to find status for %s in plugin status update, got: %+v", Name, status)
		}

		if bStatus.State != plugins.StateNotReady {
			t.Errorf("Expected plugin status update to have state = %s, got %s", plugins.StateNotReady, bStatus.State)
		}
	})

	// Note: test stages are accumulating state with reconfigures between them, the order does matter!
	// Each stage defines the new config, side effects are validated.
	stages := []struct {
		name string
		cfg  *Config
	}{
		{
			name: "start with single legacy bundle",
			cfg: &Config{
				Name:    "bundle.tar.gz",
				Service: serviceName,
				Config:  baseConf,
				// Note: the config validation and default injection will add an entry
				// to the Bundles map for the older style configuration.
				Bundles: map[string]*Source{
					"bundle.tar.gz": {Config: baseConf, Service: serviceName, Resource: "/bundles/bundle.tar.gz"},
				},
			},
		},
		{
			name: "switch to multi-bundle",
			cfg: &Config{
				Bundles: map[string]*Source{
					"b1": {Config: baseConf, Service: serviceName, Resource: "/bundles/bundle.tar.gz"},
				},
			},
		},
		{
			name: "add second bundle",
			cfg: &Config{
				Bundles: map[string]*Source{
					"b1": {Config: baseConf, Service: serviceName, Resource: "/bundles/bundle1.tar.gz"},
					"b2": {Config: baseConf, Service: serviceName, Resource: "/bundles/bundle2.tar.gz"},
				},
			},
		},
		{
			name: "remove initial bundle",
			cfg: &Config{
				Bundles: map[string]*Source{
					"b2": {Config: baseConf, Service: serviceName, Resource: "/bundles/bundle2.tar.gz"},
				},
			},
		},
		{
			name: "Update single bundle",
			cfg: &Config{
				Bundles: map[string]*Source{
					"b2": {Config: baseConf, Service: serviceName, Resource: "/new/path/bundles/bundle2.tar.gz"},
				},
			},
		},
		{
			name: "Add multiple new bundles",
			cfg: &Config{
				Bundles: map[string]*Source{
					"b3": {Config: baseConf, Service: serviceName, Resource: "/bundle3.tar.gz"},
					"b4": {Config: baseConf, Service: serviceName, Resource: "/bundles/bundle4.tar.gz"},
					"b5": {Config: baseConf, Service: serviceName, Resource: "/bundles/bundle5.tar.gz"},
				},
			},
		},
		{
			name: "Remove multiple bundles",
			cfg: &Config{
				Bundles: map[string]*Source{
					"b2": {Config: baseConf, Service: serviceName, Resource: "/new/path/bundles/bundle2.tar.gz"},
					"b4": {Config: baseConf, Service: serviceName, Resource: "/bundles/bundle4.tar.gz"},
				},
			},
		},
		{
			name: "Update multiple bundles",
			cfg: &Config{
				Bundles: map[string]*Source{
					"b2": {Config: baseConf, Service: serviceName, Resource: "/update2/bundle2.tar.gz"},
					"b4": {Config: baseConf, Service: serviceName, Resource: "/update2/bundle4.tar.gz"},
				},
			},
		},
		{
			name: "Remove and add bundle",
			cfg: &Config{
				Bundles: map[string]*Source{
					"b6": {Config: baseConf, Service: serviceName, Resource: "bundle6.tar.gz"},
				},
			},
		},
		{
			name: "Add and update bundle",
			cfg: &Config{
				Bundles: map[string]*Source{
					"b6": {Config: baseConf, Service: serviceName, Resource: "/update3/bundle6.tar.gz"},
					"b7": {Config: baseConf, Service: serviceName, Resource: "bundle7.tar.gz"},
					"b8": {Config: baseConf, Service: serviceName, Resource: "bundle8.tar.gz"},
				},
			},
		},
		{
			name: "Update and remove",
			cfg: &Config{
				Bundles: map[string]*Source{
					"b6": {Config: baseConf, Service: serviceName, Resource: "/update4/bundle6.tar.gz"},
					"b8": {Config: baseConf, Service: serviceName, Resource: "bundle8.tar.gz"},
				},
			},
		},
		// Add, Update, and Remove
		{
			name: "Add update and remove",
			cfg: &Config{
				Bundles: map[string]*Source{
					"b8": {Config: baseConf, Service: serviceName, Resource: "/update5/bundle8.tar.gz"},
					"b9": {Config: baseConf, Service: serviceName, Resource: "bundle9.tar.gz"},
				},
			},
		},
	}

	for _, stage := range stages {
		t.Run(stage.name, func(t *testing.T) {

			plugin.Reconfigure(ctx, stage.cfg)

			var expectedNumBundles int
			if stage.cfg.Name != "" {
				expectedNumBundles = 1
			} else {
				expectedNumBundles = len(stage.cfg.Bundles)
			}

			if expectedNumBundles != len(plugin.downloaders) {
				t.Fatalf("Expected a downloader for each configured bundle, expected %d found %d", expectedNumBundles, len(plugin.downloaders))
			}

			if expectedNumBundles != len(plugin.status) {
				t.Fatalf("Expected a status entry for each configured bundle, expected %d found %d", expectedNumBundles, len(plugin.status))
			}

			for name := range stage.cfg.Bundles {
				if _, found := plugin.downloaders[name]; !found {
					t.Fatalf("bundle %q not found in downloaders map", name)
				}

				if _, found := plugin.status[name]; !found {
					t.Fatalf("bundle %q not found in status map", name)
				}
			}
		})
	}
	if len(stages) != updateCount {
		t.Fatalf("Expected to have recieved %d updates, got %d", len(stages), updateCount)
	}
}

func TestPluginRequestVsDownloadTimestamp(t *testing.T) {

	ctx := context.Background()
	manager := getTestManager()
	plugin := Plugin{manager: manager, status: map[string]*Status{}, etags: map[string]string{}, downloaders: map[string]*download.Downloader{}}
	bundleName := "test-bundle"
	plugin.status[bundleName] = &Status{Name: bundleName}
	plugin.downloaders[bundleName] = download.New(download.Config{}, plugin.manager.Client(""), bundleName)

	b := &bundle.Bundle{}
	b.Manifest.Init()

	// simulate HTTP 200 response from downloader
	plugin.oneShot(ctx, bundleName, download.Update{Bundle: b})

	if plugin.status[bundleName].LastSuccessfulDownload != plugin.status[bundleName].LastSuccessfulRequest || plugin.status[bundleName].LastSuccessfulDownload != plugin.status[bundleName].LastRequest {
		t.Fatal("expected last successful request to be same as download and request")
	}

	// The time resolution is 1ns so sleeping for 1ms should be more than enough.
	time.Sleep(time.Millisecond)

	// simulate HTTP 304 response from downloader.
	plugin.oneShot(ctx, bundleName, download.Update{Bundle: nil})

	if plugin.status[bundleName].LastSuccessfulDownload == plugin.status[bundleName].LastSuccessfulRequest || plugin.status[bundleName].LastSuccessfulDownload == plugin.status[bundleName].LastRequest {
		t.Fatal("expected last successful request to differ from download and request")
	}

	// simulate HTTP 200 response from downloader
	plugin.oneShot(ctx, bundleName, download.Update{Bundle: b})

	if plugin.status[bundleName].LastSuccessfulDownload != plugin.status[bundleName].LastSuccessfulRequest || plugin.status[bundleName].LastSuccessfulDownload != plugin.status[bundleName].LastRequest {
		t.Fatal("expected last successful request to be same as download and request")
	}

	// simulate error response from downloader
	plugin.oneShot(ctx, bundleName, download.Update{Error: errors.New("xxx")})

	if plugin.status[bundleName].LastSuccessfulDownload != plugin.status[bundleName].LastSuccessfulRequest || plugin.status[bundleName].LastSuccessfulDownload == plugin.status[bundleName].LastRequest {
		t.Fatal("expected last successful request to be same as download but different from request")
	}
}

func TestUpgradeLegacyBundleToMuiltiBundleSameBundle(t *testing.T) {

	ctx := context.Background()
	manager := getTestManager()
	plugin := Plugin{manager: manager, status: map[string]*Status{}, etags: map[string]string{}, downloaders: map[string]*download.Downloader{}}
	bundleName := "test-bundle"
	plugin.status[bundleName] = &Status{Name: bundleName}
	plugin.downloaders[bundleName] = download.New(download.Config{}, plugin.manager.Client(""), bundleName)

	// Start with a "legacy" style config for a single bundle
	plugin.config = Config{
		Bundles: map[string]*Source{
			bundleName: &Source{
				Service: "s1",
			},
		},
		Name:    bundleName,
		Service: "s1",
		Prefix:  nil,
	}

	module := "package a.a1\n\nbar=1"

	b := bundle.Bundle{
		Manifest: bundle.Manifest{Revision: "quickbrownfaux", Roots: &[]string{"a/a1", "a/a2"}},
		Data: map[string]interface{}{
			"a": map[string]interface{}{
				"a2": "foo",
			},
		},
		Modules: []bundle.ModuleFile{
			bundle.ModuleFile{
				Path:   "bundle/id1",
				Parsed: ast.MustParseModule(module),
				Raw:    []byte(module),
			},
		},
	}

	b.Manifest.Init()

	plugin.oneShot(ctx, bundleName, download.Update{Bundle: &b})

	// Ensure it has been activated
	expData := util.MustUnmarshalJSON([]byte(`{"a2": "foo"}`))
	expIds := []string{"bundle/id1"}
	validateStoreState(ctx, t, manager.Store, "/a", expData, expIds, bundleName, "quickbrownfaux")

	if plugin.config.IsMultiBundle() {
		t.Fatalf("Expected plugin to be in non-multi bundle config mode")
	}

	// Update to the newer style config with the same bundle
	multiBundleConf := &Config{
		Bundles: map[string]*Source{
			bundleName: &Source{
				Service: "s1",
			},
		},
	}

	plugin.Reconfigure(ctx, multiBundleConf)
	b.Manifest.Revision = "quickbrownfaux-2"
	plugin.oneShot(ctx, bundleName, download.Update{Bundle: &b})

	// The only thing that should have changed is the store id for the policy
	expIds = []string{"test-bundle/bundle/id1"}
	validateStoreState(ctx, t, manager.Store, "/a", expData, expIds, bundleName, "quickbrownfaux-2")

	// Make sure the legacy path is gone now that we are in multi-bundle mode
	var actual string
	err := storage.Txn(ctx, plugin.manager.Store, storage.WriteParams, func(txn storage.Transaction) error {
		var err error
		if actual, err = bundle.LegacyReadRevisionFromStore(ctx, plugin.manager.Store, txn); err != nil && !storage.IsNotFound(err) {
			t.Fatalf("Failed to read manifest revision from store: %s", err)
			return err
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Unexpected error finishing transaction: %s", err)
	}
	if actual != "" {
		t.Fatalf("Expected to not find manifest revision but got %s", actual)
	}

	if !plugin.config.IsMultiBundle() {
		t.Fatalf("Expected plugin to be in multi bundle config mode")
	}
}

func TestUpgradeLegacyBundleToMuiltiBundleNewBundles(t *testing.T) {
	ctx := context.Background()
	manager := getTestManager()
	plugin := Plugin{
		manager:     manager,
		status:      map[string]*Status{},
		etags:       map[string]string{},
		downloaders: map[string]*download.Downloader{},
	}
	bundleName := "test-bundle"
	plugin.status[bundleName] = &Status{Name: bundleName}
	plugin.downloaders[bundleName] = download.New(download.Config{}, plugin.manager.Client(""), bundleName)

	tsURLBase := "/opa-test/"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, tsURLBase) {
			t.Fatalf("Invalid request URL path: %s, expected prefix %s", r.URL.Path, tsURLBase)
		}
		fmt.Fprintln(w, "") // Note: this is an invalid bundle and will fail the download
	}))
	defer ts.Close()

	serviceName := "test-svc"
	err := manager.Reconfigure(&config.Config{
		Services: []byte(fmt.Sprintf("{\"%s\":{ \"url\": \"%s\"}}", serviceName, ts.URL+tsURLBase)),
	})
	if err != nil {
		t.Fatalf("Error configuring plugin manager: %s", err)
	}

	var delay int64 = 10
	downloadConf := download.Config{Polling: download.PollingConfig{MinDelaySeconds: &delay, MaxDelaySeconds: &delay}}

	// Start with a "legacy" style config for a single bundle
	plugin.config = Config{
		Bundles: map[string]*Source{
			bundleName: &Source{
				Config:  downloadConf,
				Service: serviceName,
			},
		},
		Name:    bundleName,
		Service: serviceName,
		Prefix:  nil,
	}

	module := "package a.a1\n\nbar=1"

	b := bundle.Bundle{
		Manifest: bundle.Manifest{Revision: "quickbrownfaux", Roots: &[]string{"a/a1", "a/a2"}},
		Data: map[string]interface{}{
			"a": map[string]interface{}{
				"a2": "foo",
			},
		},
		Modules: []bundle.ModuleFile{
			bundle.ModuleFile{
				Path:   "bundle/id1",
				Parsed: ast.MustParseModule(module),
				Raw:    []byte(module),
			},
		},
	}

	b.Manifest.Init()

	plugin.oneShot(ctx, bundleName, download.Update{Bundle: &b})

	// Ensure it has been activated
	expData := util.MustUnmarshalJSON([]byte(`{"a2": "foo"}`))
	expIds := []string{"bundle/id1"}
	validateStoreState(ctx, t, manager.Store, "/a", expData, expIds, bundleName, "quickbrownfaux")

	if plugin.config.IsMultiBundle() {
		t.Fatalf("Expected plugin to be in non-multi bundle config mode")
	}

	// Update to the newer style config with a new bundle
	multiBundleConf := &Config{
		Bundles: map[string]*Source{
			"b2": &Source{
				Config:  downloadConf,
				Service: serviceName,
			},
		},
	}

	delete(plugin.downloaders, bundleName)
	plugin.downloaders["b2"] = download.New(download.Config{}, plugin.manager.Client(""), "b2")
	plugin.Reconfigure(ctx, multiBundleConf)

	module = "package a.c\n\nbar=1"
	b = bundle.Bundle{
		Manifest: bundle.Manifest{Revision: fmt.Sprintf("b2-1"), Roots: &[]string{"a/b2", "a/c"}},
		Data: map[string]interface{}{
			"a": map[string]interface{}{
				"b2": "foo",
			},
		},
		Modules: []bundle.ModuleFile{
			bundle.ModuleFile{
				Path:   "id1",
				Parsed: ast.MustParseModule(module),
				Raw:    []byte(module),
			},
		},
	}
	b.Manifest.Init()
	plugin.oneShot(ctx, "b2", download.Update{Bundle: &b})

	expData = util.MustUnmarshalJSON([]byte(`{"b2": "foo"}`))
	expIds = []string{"b2/id1"}
	validateStoreState(ctx, t, manager.Store, "/a", expData, expIds, "b2", "b2-1")

	// Make sure the legacy path is gone now that we are in multi-bundle mode
	var actual string
	err = storage.Txn(ctx, plugin.manager.Store, storage.WriteParams, func(txn storage.Transaction) error {
		var err error
		if actual, err = bundle.LegacyReadRevisionFromStore(ctx, plugin.manager.Store, txn); err != nil && !storage.IsNotFound(err) {
			t.Fatalf("Failed to read manifest revision from store: %s", err)
			return err
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Unexpected error finishing transaction: %s", err)
	}
	if actual != "" {
		t.Fatalf("Expected to not find manifest revision but got %s", actual)
	}

	if !plugin.config.IsMultiBundle() {
		t.Fatalf("Expected plugin to be in multi bundle config mode")
	}
}

func TestSaveBundleToDiskNew(t *testing.T) {

	manager := getTestManager()

	dir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	defer os.RemoveAll(dir)

	bundles := map[string]*Source{}
	plugin := New(&Config{Bundles: bundles}, manager)
	plugin.bundlePersistPath = filepath.Join(dir, ".opa")

	b := getTestBundle(t)

	err = plugin.saveBundleToDisk("foo", &b)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
}

func TestSaveBundleToDiskNewConfiguredPersistDir(t *testing.T) {
	dir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	defer os.RemoveAll(dir)

	manager := getTestManager()
	manager.Config.PersistenceDirectory = &dir
	bundles := map[string]*Source{}
	plugin := New(&Config{Bundles: bundles}, manager)

	err = plugin.Start(context.Background())
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	b := getTestBundle(t)

	err = plugin.saveBundleToDisk("foo", &b)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	expectBundlePath := filepath.Join(dir, "bundles", "foo", "bundle.tar.gz")
	_, err = os.Stat(expectBundlePath)
	if err != nil {
		t.Errorf("expected bundle persisted at path %v, %v", expectBundlePath, err)
	}
}

func TestSaveBundleToDiskOverWrite(t *testing.T) {

	manager := getTestManager()

	// test to check existing bundle is replaced
	dir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	defer os.RemoveAll(dir)

	bundles := map[string]*Source{}
	plugin := New(&Config{Bundles: bundles}, manager)
	plugin.bundlePersistPath = filepath.Join(dir, ".opa")

	bundleName := "foo"
	bundleDir := filepath.Join(plugin.bundlePersistPath, bundleName)

	err = os.MkdirAll(bundleDir, os.ModePerm)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	b2 := writeTestBundleToDisk(t, bundleDir, false)

	module := "package a.a1\n\nbar=1"

	newBundle := bundle.Bundle{
		Manifest: bundle.Manifest{Revision: "quickbrownfaux", Roots: &[]string{"a/a1", "a/a2"}},
		Data: map[string]interface{}{
			"a": map[string]interface{}{
				"a2": "foo",
			},
		},
		Modules: []bundle.ModuleFile{
			bundle.ModuleFile{
				Path:   "bundle/id1",
				Parsed: ast.MustParseModule(module),
				Raw:    []byte(module),
			},
		},
	}
	newBundle.Manifest.Init()

	err = plugin.saveBundleToDisk("foo", &newBundle)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	actual, err := loadBundleFromDisk(plugin.bundlePersistPath, "foo", nil)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	if actual.Equal(b2) {
		t.Fatal("expected existing bundle to be overwritten")
	}
}

func TestSaveCurrentBundleToDisk(t *testing.T) {
	b := getTestBundle(t)

	srcDir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	defer os.RemoveAll(srcDir)

	err = saveCurrentBundleToDisk(srcDir, "bundle.tar.gz", &b)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	if _, err := os.Stat(filepath.Join(srcDir, "bundle.tar.gz")); err != nil {
		t.Fatalf("unexpected error %v", err)
	}
}

func TestLoadBundleFromDisk(t *testing.T) {

	// no bundle on disk
	_, err := loadBundleFromDisk("foo", "bar", nil)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	// create a test bundle and load it from disk
	dir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	defer os.RemoveAll(dir)

	bundleName := "foo"
	bundleDir := filepath.Join(dir, bundleName)

	err = os.MkdirAll(bundleDir, os.ModePerm)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	b := writeTestBundleToDisk(t, bundleDir, false)

	result, err := loadBundleFromDisk(dir, bundleName, nil)
	if err != nil {
		t.Fatal("unexpected error:", err)
	}

	if !result.Equal(b) {
		t.Fatal("expected the test bundle to be equal to the one loaded from disk")
	}
}

func TestLoadSignedBundleFromDisk(t *testing.T) {

	// no bundle on disk
	_, err := loadBundleFromDisk("foo", "bar", nil)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	// create a test signed bundle and load it from disk
	dir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	defer os.RemoveAll(dir)

	bundleName := "foo"
	bundleDir := filepath.Join(dir, bundleName)

	err = os.MkdirAll(bundleDir, os.ModePerm)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	b := writeTestBundleToDisk(t, bundleDir, true)

	src := Source{
		Signing: bundle.NewVerificationConfig(map[string]*keys.Config{"foo": {Key: "secret", Algorithm: "HS256"}}, "foo", "", nil),
	}

	result, err := loadBundleFromDisk(dir, bundleName, &src)
	if err != nil {
		t.Fatal("unexpected error:", err)
	}

	if !result.Equal(b) {
		t.Fatal("expected the test bundle to be equal to the one loaded from disk")
	}

	if !reflect.DeepEqual(result.Signatures, b.Signatures) {
		t.Fatal("Expected signatures to be same")
	}
}

func TestGetDefaultBundlePersistPath(t *testing.T) {
	plugin := New(&Config{}, getTestManager())
	path, err := plugin.getBundlePersistPath()
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	if !strings.HasSuffix(path, ".opa/bundles") {
		t.Fatal("expected default persist path to end with '.opa/bundles' dir")
	}
}

func TestConfiguredBundlePersistPath(t *testing.T) {
	persistPath := "/var/opa"
	manager := getTestManager()
	manager.Config.PersistenceDirectory = &persistPath
	plugin := New(&Config{}, manager)

	path, err := plugin.getBundlePersistPath()
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	if path != "/var/opa/bundles" {
		t.Errorf("expected configured persist path '/var/opa/bundles'")
	}
}

func writeTestBundleToDisk(t *testing.T, srcDir string, signed bool) bundle.Bundle {
	t.Helper()

	var b bundle.Bundle

	if signed {
		b = getTestSignedBundle(t)
	} else {
		b = getTestBundle(t)
	}

	var buf bytes.Buffer
	if err := bundle.NewWriter(&buf).UseModulePath(true).Write(b); err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	if err := ioutil.WriteFile(filepath.Join(srcDir, "bundle.tar.gz"), buf.Bytes(), 0644); err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	return b
}

func getTestBundle(t *testing.T) bundle.Bundle {
	t.Helper()

	module := "package gork\np[x] { x = 1 }"

	b := bundle.Bundle{
		Manifest: bundle.Manifest{
			Revision: "quickbrownfaux",
		},
		Data: map[string]interface{}{},
		Modules: []bundle.ModuleFile{
			{
				Path:   "/foo.rego",
				URL:    "/foo.rego",
				Parsed: ast.MustParseModule(module),
				Raw:    []byte(module),
			},
		},
	}

	b.Manifest.Init()
	return b
}

func getTestSignedBundle(t *testing.T) bundle.Bundle {
	t.Helper()

	b := getTestBundle(t)

	if err := b.GenerateSignature(bundle.NewSigningConfig("secret", "HS256", ""), "foo", false); err != nil {
		t.Fatal("Unexpected error:", err)
	}
	return b
}

func validateStoreState(ctx context.Context, t *testing.T, store storage.Store, root string, expData interface{}, expIds []string, expBundleName string, expBundleRev string) {
	t.Helper()
	if err := storage.Txn(ctx, store, storage.TransactionParams{}, func(txn storage.Transaction) error {
		value, err := store.Read(ctx, txn, storage.MustParsePath(root))
		if err != nil {
			return err
		}

		if !reflect.DeepEqual(value, expData) {
			return fmt.Errorf("Expected %v but got %v", expData, value)
		}

		ids, err := store.ListPolicies(ctx, txn)
		if err != nil {
			return err
		}

		sort.Strings(ids)
		sort.Strings(expIds)

		if !reflect.DeepEqual(ids, expIds) {
			return fmt.Errorf("Expected ids %v but got %v", expIds, ids)
		}

		rev, err := bundle.ReadBundleRevisionFromStore(ctx, store, txn, expBundleName)
		if err != nil {
			return fmt.Errorf("Unexpected error when reading bundle revision from store: %s", err)
		}

		if rev != expBundleRev {
			return fmt.Errorf("Unexpected revision found on bundle: %s", rev)
		}

		return nil

	}); err != nil {
		t.Fatal(err)
	}
}

func ensurePluginState(t *testing.T, p *Plugin, state plugins.State) {
	t.Helper()
	status, ok := p.manager.PluginStatus()[Name]
	if !ok {
		t.Fatalf("Expected to find state for %s, found nil", Name)
		return
	}
	if status.State != state {
		t.Fatalf("Unexpected status state found in plugin manager for %s:\n\n\tFound:%+v\n\n\tExpected: %s", Name, status.State, state)
	}
}
