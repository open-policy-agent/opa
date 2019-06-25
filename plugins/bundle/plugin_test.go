// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package bundle

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/bundle"
	"github.com/open-policy-agent/opa/config"
	"github.com/open-policy-agent/opa/download"
	"github.com/open-policy-agent/opa/plugins"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/storage/inmem"
	"github.com/open-policy-agent/opa/util"
)

func TestPluginOneShot(t *testing.T) {

	ctx := context.Background()
	manager := getTestManager()
	plugin := Plugin{manager: manager, status: map[string]*Status{}, etags: map[string]string{}}
	bundleName := "test-bundle"
	plugin.status[bundleName] = &Status{Name: bundleName}

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

	plugin.oneShot(ctx, bundleName, download.Update{Bundle: &b})

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
	plugin := Plugin{manager: manager, status: map[string]*Status{}, etags: map[string]string{}}
	bundleName := "test-bundle"
	plugin.status[bundleName] = &Status{Name: bundleName}
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
	plugin.oneShot(ctx, bundleName, download.Update{Bundle: b1})

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
	txn := storage.NewTransactionOrDie(ctx, manager.Store)

	_, err := manager.Store.GetPolicy(ctx, txn, "/example.rego")
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

	txn = storage.NewTransactionOrDie(ctx, manager.Store)

	_, err = manager.Store.GetPolicy(ctx, txn, "/example.rego")
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
	plugin := Plugin{manager: manager, status: map[string]*Status{}, etags: map[string]string{}}
	bundleName := "test-bundle"
	plugin.status[bundleName] = &Status{Name: bundleName}

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

	err := storage.Txn(ctx, manager.Store, storage.TransactionParams{}, func(txn storage.Transaction) error {
		ids, err := manager.Store.ListPolicies(ctx, txn)
		if err != nil {
			return err
		} else if !reflect.DeepEqual([]string{"/example2.rego"}, ids) {
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

func TestPluginListener(t *testing.T) {

	ctx := context.Background()
	manager := getTestManager()
	plugin := Plugin{manager: manager, status: map[string]*Status{}, etags: map[string]string{}}
	bundleName := "test-bundle"
	plugin.status[bundleName] = &Status{Name: bundleName}
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

	if s1.ActiveRevision != "quickbrownfaux" || s1.Code != "" {
		t.Fatal("Unexpected status update, got:", s1)
	}

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

	if s2.ActiveRevision != "quickbrownfaux" || s2.Code == "" || s2.Message == "" || len(s2.Errors) == 0 {
		t.Fatal("Unexpected status update, got:", s2)
	}

	module = "package gork\np[1]"
	b.Manifest.Revision = "fancybluederg"
	b.Modules[0] = bundle.ModuleFile{
		Path:   "/foo.rego",
		Raw:    []byte(module),
		Parsed: ast.MustParseModule(module),
	}

	// Test that new update is successful.
	go plugin.oneShot(ctx, bundleName, download.Update{Bundle: &b})
	s3 := <-ch

	if s3.ActiveRevision != "fancybluederg" || s3.Code != "" || s3.Message != "" || len(s3.Errors) != 0 {
		t.Fatal("Unexpected status update, got:", s3)
	}

	// Test that empty download update results in status update.
	go plugin.oneShot(ctx, bundleName, download.Update{})
	s4 := <-ch

	if !reflect.DeepEqual(s3, s4) {
		t.Fatalf("Expected: %v but got: %v", s3, s4)
	}

}

func TestPluginListenerErrorClearedOn304(t *testing.T) {
	ctx := context.Background()
	manager := getTestManager()
	plugin := Plugin{manager: manager, status: map[string]*Status{}, etags: map[string]string{}}
	bundleName := "test-bundle"
	plugin.status[bundleName] = &Status{Name: bundleName}
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
	plugin := Plugin{manager: manager, status: map[string]*Status{}, etags: map[string]string{}}
	bundleNames := []string{
		"b1",
		"b2",
		"b3",
	}
	for _, name := range bundleNames {
		plugin.status[name] = &Status{Name: name}
	}
	bulkChan := make(chan map[string]*Status)

	plugin.RegisterBulkListener("bulk test", func(status map[string]*Status) {
		bulkChan <- status
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

func TestPluginActivateScopedBundle(t *testing.T) {

	ctx := context.Background()
	manager := getTestManager()
	plugin := Plugin{manager: manager, status: map[string]*Status{}, etags: map[string]string{}}
	bundleName := "test-bundle"
	plugin.status[bundleName] = &Status{Name: bundleName}

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
	expIds := []string{"bundle/id1", "some/id2", "some/id3"}
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
	expIds = []string{"bundle/id2", "some/id3"}
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
	expIds = []string{"bundle/id2", "not_scoped", "some/id3"}
	validateStoreState(ctx, t, manager.Store, "/a", expData, expIds, bundleName, "quickbrownfaux-2")
}

func TestPluginSetCompilerOnContext(t *testing.T) {

	ctx := context.Background()
	manager := getTestManager()
	plugin := Plugin{manager: manager, status: map[string]*Status{}, etags: map[string]string{}}
	bundleName := "test-bundle"
	plugin.status[bundleName] = &Status{Name: bundleName}

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
	} else if !compiler.Modules["/test.rego"].Equal(exp) {
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
			name: "switch to mutli-bundle",
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
}

func TestUpgradeLegacyBundleToMuiltiBundleSameBundle(t *testing.T) {

	ctx := context.Background()
	manager := getTestManager()
	plugin := Plugin{manager: manager, status: map[string]*Status{}, etags: map[string]string{}}
	bundleName := "test-bundle"
	plugin.status[bundleName] = &Status{Name: bundleName}

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

	// None of the data should have changed, only the revision
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
				Path:   "b2/id1",
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
