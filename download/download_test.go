// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package download

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/bundle"
	"github.com/open-policy-agent/opa/plugins/rest"
)

func TestStartStop(t *testing.T) {
	ctx := context.Background()
	fixture := newTestFixture(t)

	called := make(chan struct{})

	config := Config{}
	if err := config.ValidateAndInjectDefaults(); err != nil {
		t.Fatal(err)
	}

	d := New(config, fixture.client, "/bundles/test/bundle1").WithCallback(func(context.Context, Update) {
		called <- struct{}{}
	})

	d.Start(ctx)
	_ = <-called
	d.Stop(ctx)
}

func TestEtagCaching(t *testing.T) {

	ctx := context.Background()
	fixture := newTestFixture(t)
	fixture.server.expEtag = "some etag value"
	defer fixture.server.stop()

	updates := []Update{}

	d := New(Config{}, fixture.client, "/bundles/test/bundle1").WithCallback(func(ctx context.Context, u Update) {
		updates = append(updates, u)
	})

	err := d.oneShot(ctx)
	if err != nil {
		t.Fatal("Unexpected:", err)
	} else if len(updates) != 1 || updates[0].ETag != "some etag value" {
		t.Fatal("expected update")
	}

	err = d.oneShot(ctx)
	if err != nil {
		t.Fatal("Unexpected:", err)
	} else if len(updates) != 2 || updates[1].Bundle != nil {
		t.Fatal("expected no change")
	}
}

func TestFailureAuthn(t *testing.T) {

	ctx := context.Background()
	fixture := newTestFixture(t)
	fixture.server.expAuth = "Bearer anothersecret"
	defer fixture.server.stop()

	d := New(Config{}, fixture.client, "/bundles/test/bundle1")

	err := d.oneShot(ctx)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestFailureNotFound(t *testing.T) {

	ctx := context.Background()
	fixture := newTestFixture(t)
	delete(fixture.server.bundles, "test/bundle1")
	defer fixture.server.stop()

	d := New(Config{}, fixture.client, "/bundles/test/non-existent")

	err := d.oneShot(ctx)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestFailureUnexpected(t *testing.T) {

	ctx := context.Background()
	fixture := newTestFixture(t)
	fixture.server.expCode = 500
	defer fixture.server.stop()

	d := New(Config{}, fixture.client, "/bundles/test/bundle1")

	err := d.oneShot(ctx)
	if err == nil {
		t.Fatal("expected error")
	}
}

type testFixture struct {
	d      *Downloader
	client rest.Client
	server *testServer
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

	restConfig := []byte(fmt.Sprintf(`{
		"url": %q,
		"credentials": {
			"bearer": {
				"scheme": "Bearer",
				"token": "secret"
			}
		}
	}`, ts.server.URL))

	tc, err := rest.New(restConfig)

	if err != nil {
		t.Fatal(err)
	}

	return testFixture{
		client: tc,
		server: &ts,
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
