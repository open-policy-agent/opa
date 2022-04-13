// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

//go:build slow
// +build slow

package download

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/open-policy-agent/opa/bundle"
	"github.com/open-policy-agent/opa/keys"
	"github.com/open-policy-agent/opa/logging"
	"github.com/open-policy-agent/opa/logging/test"
	"github.com/open-policy-agent/opa/metrics"
	"github.com/open-policy-agent/opa/plugins"
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

	// Give time for some download events to occur
	time.Sleep(1 * time.Second)

	u1 := <-updates

	if u1.Bundle == nil || len(u1.Bundle.Modules) == 0 {
		t.Fatal("expected bundle with at least one module but got:", u1)
	}

	if !strings.HasSuffix(u1.Bundle.Modules[0].URL, u1.Bundle.Modules[0].Path) {
		t.Fatalf("expected URL to have path as suffix but got %v and %v", u1.Bundle.Modules[0].URL, u1.Bundle.Modules[0].Path)
	}

	d.Stop(ctx)
}

func TestStartStopWithBundlePersistence(t *testing.T) {
	ctx := context.Background()
	fixture := newTestFixture(t)

	updates := make(chan *Update)

	config := Config{}
	if err := config.ValidateAndInjectDefaults(); err != nil {
		t.Fatal(err)
	}

	d := New(config, fixture.client, "/bundles/test/bundle1").WithCallback(func(_ context.Context, u Update) {
		updates <- &u
	}).WithBundlePersistence(true)

	d.Start(ctx)

	// Give time for some download events to occur
	time.Sleep(1 * time.Second)

	u1 := <-updates

	if u1.Bundle == nil || len(u1.Bundle.Modules) == 0 {
		t.Fatal("expected bundle with at least one module but got:", u1)
	}

	if !strings.HasSuffix(u1.Bundle.Modules[0].URL, u1.Bundle.Modules[0].Path) {
		t.Fatalf("expected URL to have path as suffix but got %v and %v", u1.Bundle.Modules[0].URL, u1.Bundle.Modules[0].Path)
	}

	if u1.Raw == nil {
		t.Fatal("expected bundle reader to be non-nil")
	}

	r := bundle.NewReader(u1.Raw)

	b, err := r.Read()
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(b.Data, u1.Bundle.Data) {
		t.Fatal("expected the bundle object and reader to have the same data")
	}

	if len(b.Modules) != len(u1.Bundle.Modules) {
		t.Fatal("expected the bundle object and reader to have the same number of bundle modules")
	}

	d.Stop(ctx)
}

func TestStopWithMultipleCalls(t *testing.T) {
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

	// Give time for some download events to occur
	time.Sleep(1 * time.Second)

	u1 := <-updates

	if u1.Bundle == nil || len(u1.Bundle.Modules) == 0 {
		t.Fatal("expected bundle with at least one module but got:", u1)
	}

	done := make(chan struct{})
	go func() {
		d.Stop(ctx)
		close(done)
	}()

	d.Stop(ctx)
	<-done

	if !d.stopped {
		t.Fatal("expected downloader to be stopped")
	}
}

func TestStartStopWithDeltaBundleMode(t *testing.T) {
	ctx := context.Background()

	updates := make(chan *Update)

	config := Config{}

	if err := config.ValidateAndInjectDefaults(); err != nil {
		t.Fatal(err)
	}

	fixture := newTestFixture(t)

	d := New(config, fixture.client, "/bundles/test/bundle2").WithCallback(func(_ context.Context, u Update) {
		updates <- &u
	})

	d.Start(ctx)

	// Give time for some download events to occur
	time.Sleep(1 * time.Second)

	u1 := <-updates

	if u1.Bundle == nil || u1.Bundle.Manifest.Revision != deltaBundleMode {
		t.Fatal("expected delta bundle but got:", u1)
	}

	d.Stop(ctx)
}

func TestStartStopWithLongPollNotSupported(t *testing.T) {
	ctx := context.Background()

	config := Config{}
	min := int64(1)
	max := int64(2)
	timeout := int64(1)
	config.Polling.MinDelaySeconds = &min
	config.Polling.MaxDelaySeconds = &max
	config.Polling.LongPollingTimeoutSeconds = &timeout

	if err := config.ValidateAndInjectDefaults(); err != nil {
		t.Fatal(err)
	}

	fixture := newTestFixture(t)
	fixture.d = New(config, fixture.client, "/bundles/test/bundle1").WithCallback(fixture.oneShot)
	defer fixture.server.stop()

	fixture.d.Start(ctx)

	// Give time for some download events to occur
	time.Sleep(5 * time.Second)

	fixture.d.Stop(ctx)
	if len(fixture.updates) < 2 {
		t.Fatalf("Expected at least 2 updates but got %v\n", len(fixture.updates))
	}
}

func TestStartStopWithLongPollSupportedByServer(t *testing.T) {
	ctx := context.Background()

	config := Config{}
	min := int64(1)
	max := int64(2)
	config.Polling.MinDelaySeconds = &min
	config.Polling.MaxDelaySeconds = &max

	// simulate scenario where server supports long polling but long polling timeout not provided
	config.Polling.LongPollingTimeoutSeconds = nil

	if err := config.ValidateAndInjectDefaults(); err != nil {
		t.Fatal(err)
	}

	fixture := newTestFixture(t)
	fixture.d = New(config, fixture.client, "/bundles/test/bundle1").WithCallback(fixture.oneShot)
	fixture.server.longPoll = true
	defer fixture.server.stop()

	fixture.d.Start(ctx)

	// Give time for some download events to occur
	time.Sleep(3 * time.Second)

	fixture.d.Stop(ctx)
	if len(fixture.updates) == 0 {
		t.Fatal("expected update but got none")
	}

	if *fixture.d.client.Config().ResponseHeaderTimeoutSeconds == 0 {
		t.Fatal("expected non-zero value for response header timeout")
	}
}

func TestStartStopWithLongPollSupported(t *testing.T) {
	ctx := context.Background()

	config := Config{}
	timeout := int64(1)
	config.Polling.LongPollingTimeoutSeconds = &timeout

	if err := config.ValidateAndInjectDefaults(); err != nil {
		t.Fatal(err)
	}

	fixture := newTestFixture(t)
	fixture.d = New(config, fixture.client, "/bundles/test/bundle1").WithCallback(fixture.oneShot)
	fixture.server.longPoll = true
	defer fixture.server.stop()

	fixture.d.Start(ctx)

	// Give time for some download events to occur
	time.Sleep(3 * time.Second)

	fixture.d.Stop(ctx)
	if len(fixture.updates) == 0 {
		t.Fatal("expected update but got none")
	}
}

func TestStartStopWithLongPollWithLongTimeout(t *testing.T) {
	ctx := context.Background()

	config := Config{}
	timeout := int64(3) // this will result in the test server sleeping for 3 seconds
	config.Polling.LongPollingTimeoutSeconds = &timeout

	if err := config.ValidateAndInjectDefaults(); err != nil {
		t.Fatal(err)
	}

	fixture := newTestFixture(t)
	fixture.d = New(config, fixture.client, "/bundles/test/bundle1").WithCallback(fixture.oneShot)
	fixture.server.longPoll = true
	defer fixture.server.stop()

	fixture.d.Start(ctx)

	time.Sleep(500 * time.Millisecond)

	if len(fixture.updates) != 0 {
		t.Fatalf("expected no update but got %v", len(fixture.updates))
	}

	fixture.d.Stop(ctx)

	if len(fixture.updates) != 1 {
		t.Fatalf("expected one update but got %v", len(fixture.updates))
	}

	if fixture.updates[0].Error == nil {
		t.Fatal("expected error but got nil")
	}
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

	// simulate downloader error on first bundle download
	fixture.server.expCode = 500
	fixture.server.expEtag = "some etag value"
	err := fixture.d.oneShot(ctx)
	if err == nil {
		t.Fatal("Expected error but got nil")
	} else if len(fixture.updates) != 1 {
		t.Fatal("expected update")
	} else if fixture.d.etag != "" {
		t.Fatalf("Expected empty downloader ETag but got %v", fixture.d.etag)
	}

	// simulate successful bundle activation and check updated etag on the downloader
	fixture.server.expCode = 0
	err = fixture.d.oneShot(ctx)
	if err != nil {
		t.Fatal("Unexpected:", err)
	} else if len(fixture.updates) != 2 {
		t.Fatal("expected update")
	} else if fixture.d.etag != fixture.server.expEtag {
		t.Fatalf("Expected downloader ETag %v but got %v", fixture.server.expEtag, fixture.d.etag)
	}

	// simulate another successful bundle activation and check updated etag on the downloader
	fixture.server.expEtag = "some etag value - 2"
	err = fixture.d.oneShot(ctx)
	if err != nil {
		t.Fatal("Unexpected:", err)
	} else if len(fixture.updates) != 3 {
		t.Fatal("expected update")
	} else if fixture.d.etag != fixture.server.expEtag {
		t.Fatalf("Expected downloader ETag %v but got %v", fixture.server.expEtag, fixture.d.etag)
	}

	// simulate bundle activation error and check etag is set from the last successful activation
	fixture.mockBundleActivationError = true
	fixture.server.expEtag = "some newer etag value - 3"
	err = fixture.d.oneShot(ctx)
	if err != nil {
		t.Fatal("Unexpected:", err)
	} else if len(fixture.updates) != 4 {
		t.Fatal("expected update")
	} else if fixture.d.etag != "some etag value - 2" {
		t.Fatalf("Expected downloader ETag %v but got %v", "some etag value - 2", fixture.d.etag)
	}

	// simulate successful bundle activation and check updated etag on the downloader
	fixture.server.expCode = 0
	fixture.mockBundleActivationError = false
	err = fixture.d.oneShot(ctx)
	if err != nil {
		t.Fatal("Unexpected:", err)
	} else if len(fixture.updates) != 5 {
		t.Fatal("expected update")
	} else if fixture.d.etag != fixture.server.expEtag {
		t.Fatalf("Expected downloader ETag %v but got %v", fixture.server.expEtag, fixture.d.etag)
	}

	// simulate downloader error and check etag is set from the last successful activation
	fixture.server.expCode = 500
	err = fixture.d.oneShot(ctx)
	if err == nil {
		t.Fatal("Expected error but got nil")
	} else if len(fixture.updates) != 6 {
		t.Fatal("expected update")
	} else if fixture.d.etag != fixture.server.expEtag {
		t.Fatalf("Expected downloader ETag %v but got %v", fixture.server.expEtag, fixture.d.etag)
	}

	// simulate bundle activation error and check etag is set from the last successful activation
	fixture.mockBundleActivationError = true
	fixture.server.expCode = 0
	err = fixture.d.oneShot(ctx)
	if err != nil {
		t.Fatal("Unexpected:", err)
	} else if len(fixture.updates) != 7 {
		t.Fatal("expected update")
	} else if fixture.d.etag != fixture.server.expEtag {
		t.Fatalf("Expected downloader ETag %v but got %v", fixture.server.expEtag, fixture.d.etag)
	}
}

func TestOneShotWithBundleEtag(t *testing.T) {

	ctx := context.Background()
	fixture := newTestFixture(t)
	fixture.d = New(Config{}, fixture.client, "/bundles/test/bundle1").WithCallback(fixture.oneShot)
	fixture.server.expEtag = "some etag value"
	defer fixture.server.stop()

	// check etag on the downloader is empty
	if fixture.d.etag != "" {
		t.Fatalf("Expected empty downloader ETag but got %v", fixture.d.etag)
	}

	// simulate successful bundle activation and check updated etag on the downloader
	fixture.server.expCode = 0
	err := fixture.d.oneShot(ctx)
	if err != nil {
		t.Fatal("Unexpected:", err)
	}

	if fixture.d.etag != fixture.server.expEtag {
		t.Fatalf("Expected downloader ETag %v but got %v", fixture.server.expEtag, fixture.d.etag)
	}

	if fixture.updates[0].Bundle == nil {
		// 200 response on first request, bundle should be present
		t.Errorf("Expected bundle in response")
	}

	if fixture.updates[0].Bundle.Etag != fixture.server.expEtag {
		t.Fatalf("Expected bundle ETag %v but got %v", fixture.server.expEtag, fixture.updates[0].Bundle.Etag)
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
	var hErr HTTPError
	if !errors.As(err, &hErr) {
		t.Fatal("expected HTTPError")
	}
	if hErr.StatusCode != 500 {
		t.Fatal("expected status code 500")
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

func TestTriggerManual(t *testing.T) {

	ctx := context.Background()
	fixture := newTestFixture(t)

	config := Config{}
	tr := plugins.TriggerManual
	config.Trigger = &tr

	if err := config.ValidateAndInjectDefaults(); err != nil {
		t.Fatal(err)
	}

	updates := make(chan *Update)

	d := New(config, fixture.client, "/bundles/test/bundle1").
		WithCallback(func(_ context.Context, u Update) {
			updates <- &u
		})

	d.Start(ctx)

	// execute a series of triggers and expect responses
	for i := 0; i < 10; i++ {

		// mutate the fixture server's bundle for this trigger
		exp := fmt.Sprintf("rev%d", i)
		b := fixture.server.bundles["test/bundle1"]
		b.Manifest.Revision = exp
		fixture.server.bundles["test/bundle1"] = b

		// trigger the downloader
		go func() {
			d.Trigger(ctx)
		}()

		// wait for the update
		u := <-updates

		if u.Bundle.Manifest.Revision != exp {
			t.Fatalf("expected revision %q but got %q", exp, u.Bundle.Manifest.Revision)
		}
	}

	d.Stop(ctx)
}

func TestTriggerManualWithTimeout(t *testing.T) {

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	fixture := newTestFixture(t)

	config := Config{}
	tr := plugins.TriggerManual
	config.Trigger = &tr

	if err := config.ValidateAndInjectDefaults(); err != nil {
		t.Fatal(err)
	}

	d := New(config, fixture.client, "/bundles/test/bundle1").
		WithCallback(func(context.Context, Update) {
			time.Sleep(3 * time.Second) // this should cause the context deadline to exceed
		})

	d.Start(ctx)

	b := fixture.server.bundles["test/bundle1"]
	b.Manifest.Revision = "rev%0"
	fixture.server.bundles["test/bundle1"] = b

	// trigger the downloader
	done := make(chan struct{})
	go func() {
		// this call should block till the context deadline exceeds
		d.Trigger(ctx)
		close(done)
	}()
	<-done

	if ctx.Err() == nil {
		t.Fatal("Expected error but got nil")
	}

	exp := context.DeadlineExceeded
	if ctx.Err() != exp {
		t.Fatalf("Expected error %v but got %v", exp, ctx.Err())
	}

	d.Stop(context.Background())
}

func TestDownloadLongPollNotModifiedOn304(t *testing.T) {

	ctx := context.Background()
	config := Config{}
	timeout := int64(3) // this will result in the test server sleeping for 3 seconds
	config.Polling.LongPollingTimeoutSeconds = &timeout

	if err := config.ValidateAndInjectDefaults(); err != nil {
		t.Fatal(err)
	}

	fixture := newTestFixture(t)
	fixture.d = New(config, fixture.client, "/bundles/test/bundle1").WithCallback(fixture.oneShot)
	fixture.server.longPoll = true
	fixture.server.expEtag = "foo"
	fixture.d.etag = fixture.server.expEtag // not modified
	fixture.server.expCode = 0
	defer fixture.server.stop()

	resp, err := fixture.d.download(ctx, metrics.New())
	if err != nil {
		t.Fatal("Unexpected:", err)
	}
	if resp.longPoll != fixture.d.longPollingEnabled {
		t.Fatalf("Expected same value for longPoll and longPollingEnabled")
	}
}

func TestOneShotLongPollingSwitch(t *testing.T) {
	ctx := context.Background()
	config := Config{}
	timeout := int64(3) // this will result in the test server sleeping for 3 seconds
	config.Polling.LongPollingTimeoutSeconds = &timeout
	if err := config.ValidateAndInjectDefaults(); err != nil {
		t.Fatal(err)
	}
	fixture := newTestFixture(t)
	fixture.d = New(config, fixture.client, "/bundles/test/bundle1").WithCallback(fixture.oneShot)
	fixture.server.expCode = 0
	defer fixture.server.stop()

	fixture.server.longPoll = true
	err := fixture.d.oneShot(ctx)
	if err != nil {
		t.Fatal("Unexpected:", err)
	}
	if fixture.d.longPollingEnabled != fixture.server.longPoll {
		t.Fatalf("Expected same value for longPoll and longPollingEnabled")
	}

	fixture.server.longPoll = false
	err = fixture.d.oneShot(ctx)
	if err != nil {
		t.Fatal("Unexpected:", err)
	}
	if fixture.d.longPollingEnabled != fixture.server.longPoll {
		t.Fatalf("Expected same value for longPollingEnabled and longPoll")
	}
}

func TestOneShotNotLongPollingSwitch(t *testing.T) {
	ctx := context.Background()
	config := Config{}
	timeout := int64(3)
	config.Polling.LongPollingTimeoutSeconds = &timeout
	if err := config.ValidateAndInjectDefaults(); err != nil {
		t.Fatal(err)
	}
	fixture := newTestFixture(t)
	fixture.d = New(config, fixture.client, "/bundles/test/bundle1").WithCallback(fixture.oneShot)
	fixture.server.expCode = 0

	defer fixture.server.stop()

	fixture.server.longPoll = true
	err := fixture.d.oneShot(ctx)
	if err != nil {
		t.Fatal("Unexpected:", err)
	}
	if !fixture.d.longPollingEnabled {
		t.Fatal("Expected long polling to be enabled")
	}

	fixture.server.longPoll = false
	err = fixture.d.oneShot(ctx)
	if err != nil {
		t.Fatal("Unexpected:", err)
	}
	if fixture.d.longPollingEnabled {
		t.Fatal("Expected long polling to be disabled")
	}
}

func TestWarnOnNonBundleContentType(t *testing.T) {
	ctx := context.Background()
	fixture := newTestFixture(t)
	fixture.server.bundles["not-a-bundle"] = bundle.Bundle{}

	config := Config{}
	if err := config.ValidateAndInjectDefaults(); err != nil {
		t.Fatal(err)
	}

	d := New(config, fixture.client, "/bundles/not-a-bundle")
	logger := test.New()
	logger.SetLevel(logging.Debug)
	d.logger = logger

	d.Start(ctx)

	time.Sleep(1 * time.Second)

	d.Stop(ctx)

	expectLogged := "Content-Type response header set to text/html. " +
		"Expected one of [application/gzip application/octet-stream application/vnd.openpolicyagent.bundles]. " +
		"Possibly not a bundle being downloaded."
	var found bool
	for _, entry := range logger.Entries() {
		if entry.Message == expectLogged {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("Expected log entry: %s", expectLogged)
	}
}

type testFixture struct {
	d                         *Downloader
	client                    rest.Client
	server                    *testServer
	updates                   []Update
	mockBundleActivationError bool
	etags                     map[string]string
}

func newTestFixture(t *testing.T) testFixture {

	patch := bundle.PatchOperation{
		Op:    "upsert",
		Path:  "/a/c/d",
		Value: []string{"foo", "bar"},
	}

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
			"test/bundle2": {
				Manifest: bundle.Manifest{
					Revision: deltaBundleMode,
				},
				Patch: bundle.Patch{Data: []bundle.PatchOperation{patch}},
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
		etags:   make(map[string]string),
	}
}

func (t *testFixture) oneShot(ctx context.Context, u Update) {

	t.updates = append(t.updates, u)

	if u.Error != nil {
		etag := t.etags["test/bundle1"]
		t.d.SetCache(etag)
		return
	}

	if u.Bundle != nil {
		if t.mockBundleActivationError {
			etag := t.etags["test/bundle1"]
			t.d.SetCache(etag)
			return
		}
	}

	t.etags["test/bundle1"] = u.ETag
}

type testServer struct {
	t              *testing.T
	expCode        int
	expEtag        string
	expAuth        string
	bundles        map[string]bundle.Bundle
	server         *httptest.Server
	etagInResponse bool
	longPoll       bool
}

func (t *testServer) handle(w http.ResponseWriter, r *http.Request) {

	if t.longPoll {

		var timeout time.Duration

		wait := getPreferHeaderField(r, "wait")
		if wait != "" {
			waitTime, err := strconv.Atoi(wait)
			if err != nil {
				panic(err)
			}
			timeout = time.Duration(waitTime) * time.Second
		}

		// simulate long operation
		time.Sleep(timeout)
	}

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

	// check to verify if server can send a delta bundle to OPA
	if b.Manifest.Revision == deltaBundleMode {
		modes := strings.Split(getPreferHeaderField(r, "modes"), ",")

		found := false
		for _, m := range modes {
			if m == deltaBundleMode {
				found = true
				break
			}
		}

		if !found {
			panic("delta bundle requested but OPA does not support it")
		}
	}

	contentTypeShouldBeSend := true
	if t.expEtag != "" {
		etag := r.Header.Get("If-None-Match")
		if etag == t.expEtag {
			contentTypeShouldBeSend = false
			if t.etagInResponse {
				w.Header().Add("Etag", t.expEtag)
			}
			w.WriteHeader(304)
			return
		}
	}

	if t.longPoll && contentTypeShouldBeSend {
		// in 304 Content-Type is not send according https://datatracker.ietf.org/doc/html/rfc7232#section-4.1
		w.Header().Add("Content-Type", "application/vnd.openpolicyagent.bundles")
	} else {
		if r.URL.Path == "/bundles/not-a-bundle" {
			w.Header().Add("Content-Type", "text/html")
		} else {
			w.Header().Add("Content-Type", "application/gzip")
		}
	}

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

func getPreferHeaderField(r *http.Request, field string) string {
	for _, line := range r.Header.Values("prefer") {
		for _, part := range strings.Split(line, ";") {
			preference := strings.Split(strings.TrimSpace(part), "=")
			if len(preference) == 2 {
				if strings.ToLower(preference[0]) == field {
					return preference[1]
				}
			}
		}
	}
	return ""
}
