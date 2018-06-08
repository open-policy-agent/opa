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

func TestPluginStartSameInput(t *testing.T) {
	ctx := context.Background()

	fixture := newTestFixture(t)
	defer fixture.server.stop()

	fixture.server.ch = make(chan []EventV1, 4)
	var result interface{} = false

	ts, err := time.Parse(time.RFC3339Nano, "2018-01-01T12:00:00.123456Z")
	if err != nil {
		panic(err)
	}

	for i := 0; i < 500; i++ {
		fixture.plugin.Log(ctx, &server.Info{
			Revision:   fmt.Sprint(i),
			DecisionID: fmt.Sprint(i),
			Query:      "data.tda.bar",
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
	chunk3 := <-fixture.server.ch
	chunk4 := <-fixture.server.ch
	expLen1 := 152
	expLen2 := 151
	expLen3 := 151
	expLen4 := 46

	if len(chunk1) != expLen1 || len(chunk2) != expLen2 || len((chunk3)) != expLen3 || len(chunk4) != expLen4 {
		t.Fatalf("Expected chunk lens %v, %v, %v and %v but got: %v, %v, %v and %v", expLen1, expLen2, expLen3, expLen4, len(chunk1), len(chunk2), len(chunk3), len(chunk4))
	}

	var expInput interface{} = map[string]interface{}{"method": "GET"}

	exp := EventV1{
		Labels: map[string]string{
			"id":  "test-instance-id",
			"app": "example-app",
		},
		Revision:    "499",
		DecisionID:  "499",
		Path:        "tda/bar",
		Input:       &expInput,
		Result:      &result,
		RequestedBy: "test",
		Timestamp:   ts,
	}

	if !reflect.DeepEqual(chunk4[expLen4-1], exp) {
		t.Fatalf("Expected %+v but got %+v", exp, chunk4[expLen4-1])
	}
}

func TestPluginStartChangingInputValues(t *testing.T) {
	ctx := context.Background()

	fixture := newTestFixture(t)
	defer fixture.server.stop()

	fixture.server.ch = make(chan []EventV1, 4)
	var result interface{} = false

	ts, err := time.Parse(time.RFC3339Nano, "2018-01-01T12:00:00.123456Z")
	if err != nil {
		panic(err)
	}

	var input map[string]interface{}

	for i := 0; i < 500; i++ {
		input = map[string]interface{}{"method": getValueForMethod(i), "path": getValueForPath(i), "user": getValueForUser(i)}

		fixture.plugin.Log(ctx, &server.Info{
			Revision:   fmt.Sprint(i),
			DecisionID: fmt.Sprint(i),
			Query:      "data.foo.bar",
			Input:      input,
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
	chunk3 := <-fixture.server.ch
	chunk4 := <-fixture.server.ch
	expLen1 := 133
	expLen2 := 132
	expLen3 := 132
	expLen4 := 103

	if len(chunk1) != expLen1 || len(chunk2) != expLen2 || len((chunk3)) != expLen3 || len(chunk4) != expLen4 {
		t.Fatalf("Expected chunk lens %v, %v, %v and %v but got: %v, %v, %v and %v", expLen1, expLen2, expLen3, expLen4, len(chunk1), len(chunk2), len(chunk3), len(chunk4))
	}

	var expInput interface{} = input

	exp := EventV1{
		Labels: map[string]string{
			"id":  "test-instance-id",
			"app": "example-app",
		},
		Revision:    "499",
		DecisionID:  "499",
		Path:        "foo/bar",
		Input:       &expInput,
		Result:      &result,
		RequestedBy: "test",
		Timestamp:   ts,
	}

	if !reflect.DeepEqual(chunk4[expLen4-1], exp) {
		t.Fatalf("Expected %+v but got %+v", exp, chunk4[expLen4-1])
	}
}

func TestPluginStartChangingInputKeysAndValues(t *testing.T) {
	ctx := context.Background()

	fixture := newTestFixture(t)
	defer fixture.server.stop()

	fixture.server.ch = make(chan []EventV1, 5)
	var result interface{} = false

	ts, err := time.Parse(time.RFC3339Nano, "2018-01-01T12:00:00.123456Z")
	if err != nil {
		panic(err)
	}

	var input map[string]interface{}

	for i := 0; i < 250; i++ {
		input = generateInputMap(i)

		fixture.plugin.Log(ctx, &server.Info{
			Revision:   fmt.Sprint(i),
			DecisionID: fmt.Sprint(i),
			Query:      "data.foo.bar",
			Input:      input,
			Results:    &result,
			RemoteAddr: "test",
			Timestamp:  ts,
		})
	}

	_, err = fixture.plugin.oneShot(ctx)
	if err != nil {
		t.Fatal(err)
	}

	<-fixture.server.ch
	chunk2 := <-fixture.server.ch

	var expInput interface{} = input

	exp := EventV1{
		Labels: map[string]string{
			"id":  "test-instance-id",
			"app": "example-app",
		},
		Revision:    "249",
		DecisionID:  "249",
		Path:        "foo/bar",
		Input:       &expInput,
		Result:      &result,
		RequestedBy: "test",
		Timestamp:   ts,
	}

	if !reflect.DeepEqual(chunk2[len(chunk2)-1], exp) {
		t.Fatalf("Expected %+v but got %+v", exp, chunk2[len(chunk2)-1])
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

func getValueForMethod(idx int) string {
	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH"}
	return methods[idx%len(methods)]
}

func getValueForPath(idx int) string {
	paths := []string{"/blah1", "/blah2", "/blah3", "/blah4"}
	return paths[idx%len(paths)]
}

func getValueForUser(idx int) string {
	users := []string{"Alice", "Bob", "Charlie", "David", "Ed"}
	return users[idx%len(users)]
}

func generateInputMap(idx int) map[string]interface{} {
	var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	result := make(map[string]interface{})

	for i := 0; i < 20; i++ {
		n := idx % len(letters)
		key := string(letters[n])
		result[key] = fmt.Sprint(idx)
	}
	return result

}
