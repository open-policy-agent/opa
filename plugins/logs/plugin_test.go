// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package logs

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/open-policy-agent/opa/plugins"
	"github.com/open-policy-agent/opa/server"
	"github.com/open-policy-agent/opa/storage/inmem"
)

func TestPluginStart(t *testing.T) {
	ctx := context.Background()

	fixture := newTestFixture(t)
	defer fixture.server.stop()

	fixture.server.ch = make(chan []EventV1, 2)
	var result interface{} = false

	ts, err := time.Parse(time.RFC3339Nano, "2018-01-01T12:00:00.123456Z")
	if err != nil {
		panic(err)
	}

	for i := 0; i < 154; i++ { // first chunk fits exactly n-1 events
		fixture.plugin.Log(ctx, &server.Info{
			Revision:   "a",
			DecisionID: fmt.Sprint(i),
			Query:      "data.foo.bar",
			Input:      map[string]interface{}{"method": "GET"},
			Results:    &result,
			RemoteAddr: "test",
			Timestamp:  ts,
		})
	}

	_, err = fixture.plugin.oneShot(ctx)
	if err != nil {
		t.Fatal(err)
	}

	chunk1 := <-fixture.server.ch
	chunk2 := <-fixture.server.ch
	expLen1 := 153
	expLen2 := 1

	if len(chunk1) != expLen1 || len(chunk2) != expLen2 {
		t.Fatalf("Expected chunk lens %v and %v but got: %v and %v", expLen1, expLen2, len(chunk1), len(chunk2))
	}

	var expInput interface{} = map[string]interface{}{"method": "GET"}

	exp := EventV1{
		Labels: map[string]string{
			"id":  "test-instance-id",
			"app": "example-app",
		},
		Revision:    "a",
		DecisionID:  "153",
		Path:        "foo/bar",
		Input:       &expInput,
		Result:      &result,
		RequestedBy: "test",
		Timestamp:   ts,
	}

	if !reflect.DeepEqual(chunk2[0], exp) {
		t.Fatalf("Expected %v but got %v", exp, chunk2[0])
	}
}

func TestPluginRequeue(t *testing.T) {
	ctx := context.Background()

	fixture := newTestFixture(t)
	defer fixture.server.stop()

	fixture.server.ch = make(chan []EventV1, 1)

	var result1 interface{} = false

	fixture.plugin.Log(ctx, &server.Info{
		DecisionID: "abc",
		Query:      "data.foo.bar",
		Input:      map[string]interface{}{"method": "GET"},
		Results:    &result1,
		RemoteAddr: "test",
		Timestamp:  time.Now().UTC(),
	})

	fixture.server.expCode = 500
	_, err := fixture.plugin.oneShot(ctx)
	if err == nil {
		t.Fatal("Expected error")
	}

	events1 := <-fixture.server.ch

	fixture.server.expCode = 200

	_, err = fixture.plugin.oneShot(ctx)
	if err != nil {
		t.Fatal(err)
	}

	events2 := <-fixture.server.ch

	if !reflect.DeepEqual(events1, events2) {
		t.Fatalf("Expected %v but got: %v", events1, events2)
	}

	uploaded, err := fixture.plugin.oneShot(ctx)
	if uploaded || err != nil {
		t.Fatalf("Unexpected error or upload, err: %v", err)
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
	ch      chan []EventV1
}

func (t *testServer) handle(w http.ResponseWriter, r *http.Request) {
	gr, err := gzip.NewReader(r.Body)
	if err != nil {
		t.t.Fatal(err)
	}
	var events []EventV1
	if err := json.NewDecoder(gr).Decode(&events); err != nil {
		t.t.Fatal(err)
	}
	if err := gr.Close(); err != nil {
		t.t.Fatal(err)
	}
	t.ch <- events
	w.WriteHeader(t.expCode)
}

func (t *testServer) start() {
	t.server = httptest.NewServer(http.HandlerFunc(t.handle))
}

func (t *testServer) stop() {
	t.server.Close()
}
