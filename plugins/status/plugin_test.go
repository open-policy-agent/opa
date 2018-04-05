// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package status

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/open-policy-agent/opa/plugins"
	"github.com/open-policy-agent/opa/plugins/bundle"
	"github.com/open-policy-agent/opa/storage/inmem"
	"github.com/open-policy-agent/opa/util"
)

func TestPluginStart(t *testing.T) {

	fixture := newTestFixture(t)
	fixture.server.ch = make(chan UpdateRequestV1)
	defer fixture.server.stop()

	ctx := context.Background()

	fixture.plugin.Start(ctx)
	defer fixture.plugin.Stop(ctx)

	status := testStatus()

	fixture.plugin.Update(status)
	result := <-fixture.server.ch

	exp := UpdateRequestV1{
		Labels: map[string]string{
			"id":  "test-instance-id",
			"app": "example-app",
		},
		Bundle: status,
	}

	if !reflect.DeepEqual(result, exp) {
		t.Fatalf("Expected: %v but got: %v", exp, result)
	}
}

func TestPluginBadAuth(t *testing.T) {
	fixture := newTestFixture(t)
	ctx := context.Background()
	fixture.server.expCode = 401
	defer fixture.server.stop()
	err := fixture.plugin.oneShot(ctx, bundle.Status{})
	if err == nil {
		t.Fatal("Expected error")
	}
}

func TestPluginBadPath(t *testing.T) {
	fixture := newTestFixture(t)
	ctx := context.Background()
	fixture.server.expCode = 404
	defer fixture.server.stop()
	err := fixture.plugin.oneShot(ctx, bundle.Status{})
	if err == nil {
		t.Fatal("Expected error")
	}
}

func TestPluginBadStatus(t *testing.T) {
	fixture := newTestFixture(t)
	ctx := context.Background()
	fixture.server.expCode = 500
	defer fixture.server.stop()
	err := fixture.plugin.oneShot(ctx, bundle.Status{})
	if err == nil {
		t.Fatal("Expected error")
	}
}

type testFixture struct {
	manager *plugins.Manager
	plugin  *Plugin
	server  *testServer
}

func newTestFixture(t *testing.T) testFixture {

	ts := testServer{
		t:       t,
		expCode: 200,
	}

	ts.start()

	managerConfig := []byte(fmt.Sprintf(`{
			"labels": {
				"app": "example-app"
			},
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

	manager, err := plugins.New(managerConfig, "test-instance-id", inmem.New())
	if err != nil {
		t.Fatal(err)
	}

	pluginConfig := []byte(fmt.Sprintf(`{
			"service": "example",
		}`))

	p, err := New(pluginConfig, manager)
	if err != nil {
		t.Fatal(err)
	}

	return testFixture{
		manager: manager,
		plugin:  p,
		server:  &ts,
	}

}

type testServer struct {
	t       *testing.T
	expCode int
	server  *httptest.Server
	ch      chan UpdateRequestV1
}

func (t *testServer) handle(w http.ResponseWriter, r *http.Request) {

	status := UpdateRequestV1{}

	if err := util.NewJSONDecoder(r.Body).Decode(&status); err != nil {
		t.t.Fatal(err)
	}

	if t.ch != nil {
		t.ch <- status
	}

	w.WriteHeader(t.expCode)
}

func (t *testServer) start() {
	t.server = httptest.NewServer(http.HandlerFunc(t.handle))
}

func (t *testServer) stop() {
	t.server.Close()
}

func testStatus() bundle.Status {

	tDownload, _ := time.Parse("2018-01-01T00:00:00.0000000Z", time.RFC3339Nano)
	tActivate, _ := time.Parse("2018-01-01T00:00:01.0000000Z", time.RFC3339Nano)

	status := bundle.Status{
		Name:                     "example/authz",
		ActiveRevision:           "quickbrawnfaux",
		LastSuccessfulDownload:   tDownload,
		LastSuccessfulActivation: tActivate,
	}

	return status
}
