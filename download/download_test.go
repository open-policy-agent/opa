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
	"github.com/open-policy-agent/opa/keys"
	"github.com/open-policy-agent/opa/plugins/rest"
)

func TestStartStop(t *testing.T) {
	ctx := context.Background()
	fixture := newTestFixture(t)

	updates := make(chan *Update)

	config := Config{}
	if err := config.ValidateAndInjectDefaults(); err != nil {
		t.Fatal(err)
	}

	d := New(config, fixture.client, "/bundles/test/bundle1").WithCallback(func(_ context.Context, u Update) {
		updates <- &u
	})

	d.Start(ctx)
	u1 := <-updates

	if u1.Bundle == nil || len(u1.Bundle.Modules) == 0 {
		t.Fatal("expected bundle with at least one module but got:", u1)
	}

	if !strings.HasSuffix(u1.Bundle.Modules[0].URL, u1.Bundle.Modules[0].Path) {
		t.Fatalf("expected URL to have path as suffix but got %v and %v", u1.Bundle.Modules[0].URL, u1.Bundle.Modules[0].Path)
	}

	d.Stop(ctx)
}

func TestEtagCachingLifecycle(t *testing.T) {

	ctx := context.Background()
	fixture := newTestFixture(t)
	fixture.d = New(Config{}, fixture.client, "/bundles/test/bundle1").WithCallback(fixture.oneShot)
	defer fixture.server.stop()

	// check etag on the downloader is empty
	if fixture.d.etag != "" {
		t.Fatalf("Expected empty downloader ETag but got %v", fixture.d.etag)
	}

	// simulate successful bundle activation and check updated etag on the downloader
	fixture.server.expEtag = "some etag value"
	err := fixture.d.oneShot(ctx)
	if err != nil {
		t.Fatal("Unexpected:", err)
	} else if len(fixture.updates) != 1 {
		t.Fatal("expected update")
	} else if fixture.d.etag != fixture.server.expEtag {
		t.Fatalf("Expected downloader ETag %v but got %v", fixture.server.expEtag, fixture.d.etag)
	}

	// simulate downloader error and check etag is cleared
	fixture.server.expCode = 500
	err = fixture.d.oneShot(ctx)
	if err == nil {
		t.Fatal("Expected error but got nil")
	} else if len(fixture.updates) != 2 {
		t.Fatal("expected update")
	} else if fixture.d.etag != "" {
		t.Fatalf("Expected empty downloader ETag but got %v", fixture.d.etag)
	}

	// simulate successful bundle activation and check updated etag on the downloader
	fixture.server.expCode = 0
	fixture.server.expEtag = "some new etag value"
	err = fixture.d.oneShot(ctx)
	if err != nil {
		t.Fatal("Unexpected:", err)
	} else if len(fixture.updates) != 3 {
		t.Fatal("expected update")
	} else if fixture.d.etag != fixture.server.expEtag {
		t.Fatalf("Expected downloader ETag %v but got %v", fixture.server.expEtag, fixture.d.etag)
	}

	// simulate bundle activation error and check etag is cleared
	fixture.mockBundleActivationError = true
	fixture.server.expEtag = "some newer etag value"
	err = fixture.d.oneShot(ctx)
	if err != nil {
		t.Fatal("Unexpected:", err)
	} else if len(fixture.updates) != 4 {
		t.Fatal("expected update")
	} else if fixture.d.etag != "" {
		t.Fatalf("Expected empty downloader ETag but got %v", fixture.d.etag)
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

func TestEtagInResponse(t *testing.T) {
	ctx := context.Background()
	fixture := newTestFixture(t)
	fixture.server.etagInResponse = true
	fixture.d = New(Config{}, fixture.client, "/bundles/test/bundle1").WithCallback(fixture.oneShot)
	defer fixture.server.stop()

	if fixture.d.etag != "" {
		t.Fatalf("Expected empty downloader ETag but got %v", fixture.d.etag)
	}

	fixture.server.expEtag = "some etag value"

	err := fixture.d.oneShot(ctx)
	if err != nil {
		t.Fatal("Unexpected:", err)
	} else if len(fixture.updates) != 1 {
		t.Fatal("expected update")
	} else if fixture.d.etag != fixture.server.expEtag {
		t.Fatalf("Expected downloader ETag %v but got %v", fixture.server.expEtag, fixture.d.etag)
	}

	if fixture.updates[0].Bundle == nil {
		// 200 response on first request, bundle should be present
		t.Errorf("Expected bundle in response")
	}

	err = fixture.d.oneShot(ctx)
	if err != nil {
		t.Fatal("Unexpected:", err)
	} else if len(fixture.updates) != 2 {
		t.Fatal("expected two updates")
	} else if fixture.d.etag != fixture.server.expEtag {
		t.Fatalf("Expected downloader ETag %v but got %v", fixture.server.expEtag, fixture.d.etag)
	}

	if fixture.updates[1].Bundle != nil {
		// 304 response on second request, bundle should _not_ be present
		t.Errorf("Expected no bundle in response")
	}
}

type testFixture struct {
	d                         *Downloader
	client                    rest.Client
	server                    *testServer
	updates                   []Update
	mockBundleActivationError bool
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

	tc, err := rest.New(restConfig, map[string]*keys.Config{})

	if err != nil {
		t.Fatal(err)
	}

	return testFixture{
		client:  tc,
		server:  &ts,
		updates: []Update{},
	}
}

func (t *testFixture) oneShot(ctx context.Context, u Update) {

	t.updates = append(t.updates, u)

	if u.Error != nil {
		t.d.ClearCache()
		return
	}

	if u.Bundle != nil {
		if t.mockBundleActivationError {
			t.d.ClearCache()
			return
		}
	}
}

type testServer struct {
	t              *testing.T
	expCode        int
	expEtag        string
	expAuth        string
	bundles        map[string]bundle.Bundle
	server         *httptest.Server
	etagInResponse bool
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
			if t.etagInResponse {
				w.Header().Add("Etag", t.expEtag)
			}
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
