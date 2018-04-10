// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package bundle

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
	"time"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/bundle"
	"github.com/open-policy-agent/opa/plugins"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/storage/inmem"
	"github.com/open-policy-agent/opa/util"
)

func TestNew(t *testing.T) {

	store := inmem.New()

	manager, err := plugins.New([]byte(`{
		"services": [
			{
				"name": "foo"
			}
		]
	}`), "test-instance-id", store)

	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		input   string
		wantErr bool
		expMin  time.Duration
		expMax  time.Duration
	}{
		{
			input: `{
			}`,
			wantErr: true,
		},
		{
			input: `{
				"name": "a/b/c",
				"service": "missing service"
			}`,
			wantErr: true,
		},
		{
			input: `{
				"name": "bad/delays",
			"service": "foo",
				"polling": {
					"min_delay_seconds": 10,
					"max_delay_seconds": 1
				}
			}`,
			wantErr: true,
		},
		{
			input: `{
				"name": "defaults",
				"service": "foo"
			}`,
			expMin: time.Second * time.Duration(defaultMinDelaySeconds),
			expMax: time.Second * time.Duration(defaultMaxDelaySeconds),
		},
		{
			input: `{
				"name": "missing/min",
				"service": "foo",
				"polling": {
					"max_delay_seconds": 10
				}
			}`,
			wantErr: true,
		},
		{
			input: `{
				"name": "missing/max",
				"service": "foo",
				"polling": {
					"min_delay_seconds": 1
				}
			}`,
			wantErr: true,
		},
		{
			input: `{
				"name": "user/min/max",
				"service": "foo",
				"polling": {
					"min_delay_seconds": 10,
					"max_delay_seconds": 30
				}
			}`,
			expMin: time.Second * 10,
			expMax: time.Second * 30,
		},
	}

	for _, test := range tests {
		p, err := New([]byte(test.input), manager)
		if err != nil && !test.wantErr {
			t.Errorf("Unexpected error on: %v, err: %v", test.input, err)
		}
		if err == nil {
			if time.Duration(*p.config.Polling.MinDelaySeconds) != test.expMin {
				t.Errorf("For %q expected min %v but got %v", p.config.Name, test.expMin, time.Duration(*p.config.Polling.MinDelaySeconds))
			}
			if time.Duration(*p.config.Polling.MaxDelaySeconds) != test.expMax {
				t.Errorf("For %q expected min %v but got %v", p.config.Name, test.expMax, time.Duration(*p.config.Polling.MaxDelaySeconds))
			}
		}
	}
}

func TestPluginStart(t *testing.T) {

	ctx := context.Background()
	fixture := newTestFixture(t)
	defer fixture.server.stop()

	if err := fixture.plugin.Start(ctx); err != nil {
		t.Fatal(err)
	}

	txn := storage.NewTransactionOrDie(ctx, fixture.store, storage.WriteParams)
	done := make(chan struct{})

	fixture.store.Register(ctx, txn, storage.TriggerConfig{
		OnCommit: func(ctx context.Context, txn storage.Transaction, event storage.TriggerEvent) {
			if !event.DataChanged() || !event.PolicyChanged() {
				return
			}
			ids, err := fixture.store.ListPolicies(ctx, txn)
			if err != nil {
				t.Fatal(err)
			} else if len(ids) != 1 {
				t.Fatal("Expected 1 policy")
			}
			bs, err := fixture.store.GetPolicy(ctx, txn, ids[0])
			exp := []byte("package foo\n\ncorge=1")
			if err != nil {
				t.Fatal(err)
			} else if !bytes.Equal(bs, exp) {
				t.Fatalf("Bad policy content. Exp:\n%v\n\nGot:\n\n%v", string(exp), string(bs))
			}
			data, err := fixture.store.Read(ctx, txn, storage.Path{})
			expData := util.MustUnmarshalJSON([]byte(`{"foo": {"bar": 1, "baz": "qux"}, "system": {"bundle": {"manifest": {"revision": "quickbrownfaux"}}}}`))
			if err != nil {
				t.Fatal(err)
			} else if !reflect.DeepEqual(data, expData) {
				t.Fatalf("Bad data content. Exp:\n%v\n\nGot:\n\n%v", expData, data)
			}
			done <- struct{}{}
		},
	})

	if err := fixture.store.Commit(ctx, txn); err != nil {
		t.Fatal(err)
	}

	<-done
	fixture.plugin.Stop(ctx)
}

func TestPluginEtagCaching(t *testing.T) {

	ctx := context.Background()
	fixture := newTestFixture(t)
	fixture.server.expEtag = "some etag value"
	defer fixture.server.stop()

	updated, err := fixture.plugin.oneShot(ctx)
	if err != nil {
		t.Fatal("Unexpected:", err)
	} else if !updated {
		t.Fatal("expected update")
	}

	updated, err = fixture.plugin.oneShot(ctx)
	if err != nil {
		t.Fatal("Unexpected:", err)
	} else if updated {
		t.Fatal("expected not update")
	}
}

func TestPluginFailureAuthn(t *testing.T) {

	ctx := context.Background()
	fixture := newTestFixture(t)
	fixture.server.expAuth = "Bearer anothersecret"
	defer fixture.server.stop()

	_, err := fixture.plugin.oneShot(ctx)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestPluginFailureNotFound(t *testing.T) {

	ctx := context.Background()
	fixture := newTestFixture(t)
	delete(fixture.server.bundles, "test/bundle1")
	defer fixture.server.stop()

	_, err := fixture.plugin.oneShot(ctx)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestPluginFailureUnexpected(t *testing.T) {

	ctx := context.Background()
	fixture := newTestFixture(t)
	fixture.server.expCode = 500
	defer fixture.server.stop()

	_, err := fixture.plugin.oneShot(ctx)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestPluginFailureCompile(t *testing.T) {

	ctx := context.Background()
	fixture := newTestFixture(t)
	defer fixture.server.stop()

	_, err := fixture.plugin.oneShot(ctx)
	if err != nil {
		t.Fatal("expected error")
	}

	fixture.server.bundles["test/bundle1"] = bundle.Bundle{
		Data: map[string]interface{}{},
		Modules: []bundle.ModuleFile{
			{
				Path: `/example.rego`,
				Raw:  []byte("package foo\n\np[x]"),
			},
		},
	}

	_, err = fixture.plugin.oneShot(ctx)
	if err == nil {
		t.Fatal("expected error")
	}

	// ensure data/policy is intact
	txn := storage.NewTransactionOrDie(ctx, fixture.store, storage.TransactionParams{})

	_, err = fixture.store.GetPolicy(ctx, txn, "/example.rego")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	data, err := fixture.store.Read(ctx, txn, storage.Path{})
	if err != nil || data == nil {
		t.Fatalf("Expected data to be intact but got: %v, err: %v", data, err)
	}
}

func TestPluginActivatationRemovesOld(t *testing.T) {

	managerConfig := []byte(`{
		"services": [
			{
				"name": "example",
				"url": "http://localhost"
			}
		]
	}`)
	store := inmem.New()
	manager, err := plugins.New(managerConfig, "test-instance-id", store)
	if err != nil {
		t.Fatal(err)
	}

	p, err := New([]byte(`{"name": "test", "service": "example"}`), manager)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

	module1 := `package example

	p = 1`

	b := bundle.Bundle{
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

	if err := p.activate(ctx, b); err != nil {
		t.Fatal("Unexpected:", err)
	}

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

	if err := p.activate(ctx, b2); err != nil {
		t.Fatal("Unexpected:", err)
	}

	err = storage.Txn(ctx, store, storage.TransactionParams{}, func(txn storage.Transaction) error {
		ids, err := store.ListPolicies(ctx, txn)
		if err != nil {
			return err
		} else if !reflect.DeepEqual([]string{"/example2.rego"}, ids) {
			return fmt.Errorf("expected updated policy ids")
		}
		data, err := store.Read(ctx, txn, storage.Path{})
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
	fixture := newTestFixture(t)
	defer fixture.server.stop()

	b := fixture.server.bundles["test/bundle1"]
	ch := make(chan Status, 1)

	fixture.plugin.Register("test", func(status Status) {
		ch <- status
	})

	// Test that initial bundle is ok.
	fixture.plugin.oneShot(ctx)
	s1 := <-ch

	if s1.ActiveRevision != "quickbrownfaux" || s1.Code != "" {
		t.Fatal("Unexpected status update, got:", s1)
	}

	// Test that next update is failed.
	b.Manifest.Revision = "slowgreenburd"
	b.Modules[0] = bundle.ModuleFile{
		Path: "/foo.rego",
		Raw:  []byte("package gork\np[x]"),
	}
	fixture.server.bundles["test/bundle1"] = b

	fixture.plugin.oneShot(ctx)
	s2 := <-ch

	if s2.ActiveRevision != "quickbrownfaux" || s2.Code == "" || s2.Message == "" || len(s2.Errors) == 0 {
		t.Fatal("Unexpected status update, got:", s2)
	}

	// Test that new update is successful.
	b.Manifest.Revision = "fancybluederg"
	b.Modules[0] = bundle.ModuleFile{
		Path: "/foo.rego",
		Raw:  []byte("package gork\np[1]"),
	}
	fixture.server.bundles["test/bundle1"] = b
	fixture.server.expEtag = "etagvalue"

	fixture.plugin.oneShot(ctx)
	s3 := <-ch

	if s3.ActiveRevision != "fancybluederg" || s3.Code != "" || s3.Message != "" || len(s3.Errors) != 0 {
		t.Fatal("Unexpected status update, got:", s3)
	}

	// Test that 304 results in status update.
	fixture.plugin.oneShot(ctx)
	s4 := <-ch

	if !reflect.DeepEqual(s3, s4) {
		t.Fatalf("Expected: %v but got: %v", s3, s4)
	}

}

type testFixture struct {
	store   storage.Store
	manager *plugins.Manager
	plugin  *Plugin
	server  *testServer
}

func newTestFixture(t *testing.T) testFixture {

	ts := testServer{
		t:       t,
		expAuth: "Bearer secret",
		bundles: map[string]bundle.Bundle{
			"test/bundle1": {
				Manifest: bundle.Manifest{
					Revision: "quickbrownfaux",
				},
				Data: map[string]interface{}{
					"foo": map[string]interface{}{
						"bar": json.Number("1"),
						"baz": "qux",
					},
				},
				Modules: []bundle.ModuleFile{
					{
						Path: `/example.rego`,
						Raw:  []byte("package foo\n\ncorge=1"),
					},
				},
			},
		},
	}

	ts.start()

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
			]}`, ts.server.URL))

	store := inmem.New()

	manager, err := plugins.New(managerConfig, "test-instance-id", store)
	if err != nil {
		t.Fatal(err)
	}

	pluginConfig := []byte(fmt.Sprintf(`{
			"name": "test/bundle1",
			"service": "example",
			"polling": {
				"min_delay_seconds": 1,
				"max_delay_seconds": 1
			}
		}`))

	p, err := New(pluginConfig, manager)
	if err != nil {
		t.Fatal(err)
	}

	return testFixture{
		store:   store,
		manager: manager,
		plugin:  p,
		server:  &ts,
	}

}

type testServer struct {
	t       *testing.T
	expCode int
	expEtag string
	expAuth string
	bundles map[string]bundle.Bundle
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

	name := strings.TrimPrefix(r.URL.Path, "/bundles/")
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

	if err := bundle.Write(&buf, b); err != nil {
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
