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
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/metrics"
	"github.com/open-policy-agent/opa/plugins"
	"github.com/open-policy-agent/opa/server"
	"github.com/open-policy-agent/opa/storage/inmem"
	"github.com/open-policy-agent/opa/version"
)

func TestMain(m *testing.M) {
	// call flag.Parse() here if TestMain uses flags
	setVersion("XY.Z")
	os.Exit(m.Run())
}

type testPlugin struct {
	events []EventV1
}

func (p *testPlugin) Start(context.Context) error {
	return nil
}

func (p *testPlugin) Stop(context.Context) {
}

func (p *testPlugin) Reconfigure(context.Context, interface{}) {
}

func (p *testPlugin) Log(_ context.Context, event EventV1) {
	p.events = append(p.events, event)
}

func TestPluginCustomBackend(t *testing.T) {
	ctx := context.Background()
	manager, _ := plugins.New(nil, "test-instance-id", inmem.New())

	backend := &testPlugin{}
	manager.Register("test_plugin", backend)

	config, err := ParseConfig([]byte(`{"plugin": "test_plugin"}`), nil, []string{"test_plugin"})
	if err != nil {
		t.Fatal(err)
	}

	plugin := New(config, manager)
	plugin.Log(ctx, &server.Info{Revision: "A"})
	plugin.Log(ctx, &server.Info{Revision: "B"})

	if len(backend.events) != 2 || backend.events[0].Revision != "A" || backend.events[1].Revision != "B" {
		t.Fatal("Unexpected events:", backend.events)
	}
}

func TestPluginErrorNoResult(t *testing.T) {
	ctx := context.Background()
	manager, _ := plugins.New(nil, "test-instance-id", inmem.New())

	backend := &testPlugin{}
	manager.Register("test_plugin", backend)

	config, err := ParseConfig([]byte(`{"plugin": "test_plugin"}`), nil, []string{"test_plugin"})
	if err != nil {
		t.Fatal(err)
	}

	plugin := New(config, manager)
	plugin.Log(ctx, &server.Info{Error: fmt.Errorf("some error")})
	plugin.Log(ctx, &server.Info{Error: ast.Errors{&ast.Error{
		Code: "some_error",
	}}})

	if len(backend.events) != 2 || backend.events[0].Error == nil || backend.events[1].Error == nil {
		t.Fatal("Unexpected events:", backend.events)
	}
}

func TestPluginQueriesAndPaths(t *testing.T) {
	ctx := context.Background()
	manager, _ := plugins.New(nil, "test-instance-id", inmem.New())

	backend := &testPlugin{}
	manager.Register("test_plugin", backend)

	config, err := ParseConfig([]byte(`{"plugin": "test_plugin"}`), nil, []string{"test_plugin"})
	if err != nil {
		t.Fatal(err)
	}

	plugin := New(config, manager)
	plugin.Log(ctx, &server.Info{Path: "data.foo"})
	plugin.Log(ctx, &server.Info{Path: "data.foo.bar"})
	plugin.Log(ctx, &server.Info{Query: "a = data.foo"})

	exp := []struct {
		query string
		path  string
	}{
		// TODO(tsandall): we need to fix how POST /v1/data (and
		// friends) are represented here. Currently we can't tell the
		// difference between /v1/data and /v1/data/data. The decision
		// log event paths should be slash prefixed to avoid ambiguity.
		//		{path: "data"},
		{path: "foo"},
		{path: "foo/bar"},
		{query: "a = data.foo"},
	}

	if len(exp) != len(backend.events) {
		t.Fatalf("Expected %d events but got %v", len(exp), len(backend.events))
	}

	for i, e := range exp {
		if e.query != backend.events[i].Query || e.path != backend.events[i].Path {
			t.Fatalf("Unexpected event %d, want %v but got %v", i, e, backend.events[i])
		}
	}
}

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

	testMetrics := getWellKnownMetrics()

	var input interface{} = map[string]interface{}{"method": "GET"}

	for i := 0; i < 400; i++ {
		fixture.plugin.Log(ctx, &server.Info{
			Revision:   fmt.Sprint(i),
			DecisionID: fmt.Sprint(i),
			Path:       "data.tda.bar",
			Input:      &input,
			Results:    &result,
			RemoteAddr: "test",
			Timestamp:  ts,
			Metrics:    testMetrics,
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
	expLen1 := 122
	expLen2 := 121
	expLen3 := 121
	expLen4 := 36

	if len(chunk1) != expLen1 || len(chunk2) != expLen2 || len(chunk3) != expLen3 || len(chunk4) != expLen4 {
		t.Fatalf("Expected chunk lens %v, %v, %v and %v but got: %v, %v, %v and %v", expLen1, expLen2, expLen3, expLen4, len(chunk1), len(chunk2), len(chunk3), len(chunk4))
	}

	var expInput interface{} = map[string]interface{}{"method": "GET"}

	msAsFloat64 := map[string]interface{}{}
	for k, v := range testMetrics.All() {
		msAsFloat64[k] = float64(v.(uint64))
	}

	exp := EventV1{
		Labels: map[string]string{
			"id":  "test-instance-id",
			"app": "example-app",
		},
		Revision:    "399",
		DecisionID:  "399",
		Path:        "tda/bar",
		Input:       &expInput,
		Result:      &result,
		RequestedBy: "test",
		Timestamp:   ts,
		Version:     getVersion(),
		Metrics:     msAsFloat64,
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

	var input interface{}

	for i := 0; i < 400; i++ {
		input = map[string]interface{}{"method": getValueForMethod(i), "path": getValueForPath(i), "user": getValueForUser(i)}

		fixture.plugin.Log(ctx, &server.Info{
			Revision:   fmt.Sprint(i),
			DecisionID: fmt.Sprint(i),
			Path:       "data.foo.bar",
			Input:      &input,
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
	expLen1 := 124
	expLen2 := 123
	expLen3 := 123
	expLen4 := 30

	if len(chunk1) != expLen1 || len(chunk2) != expLen2 || len((chunk3)) != expLen3 || len(chunk4) != expLen4 {
		t.Fatalf("Expected chunk lens %v, %v, %v and %v but got: %v, %v, %v and %v", expLen1, expLen2, expLen3, expLen4, len(chunk1), len(chunk2), len(chunk3), len(chunk4))
	}

	var expInput interface{} = input

	exp := EventV1{
		Labels: map[string]string{
			"id":  "test-instance-id",
			"app": "example-app",
		},
		Revision:    "399",
		DecisionID:  "399",
		Path:        "foo/bar",
		Input:       &expInput,
		Result:      &result,
		RequestedBy: "test",
		Timestamp:   ts,
		Version:     getVersion(),
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

	var input interface{}

	for i := 0; i < 250; i++ {
		input = generateInputMap(i)

		fixture.plugin.Log(ctx, &server.Info{
			Revision:   fmt.Sprint(i),
			DecisionID: fmt.Sprint(i),
			Path:       "data.foo.bar",
			Input:      &input,
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
		Version:     getVersion(),
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

	var input interface{} = map[string]interface{}{"method": "GET"}
	var result1 interface{} = false

	fixture.plugin.Log(ctx, &server.Info{
		DecisionID: "abc",
		Path:       "data.foo.bar",
		Input:      &input,
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

func TestPluginReconfigure(t *testing.T) {

	ctx := context.Background()
	fixture := newTestFixture(t)
	defer fixture.server.stop()

	if err := fixture.plugin.Start(ctx); err != nil {
		t.Fatal(err)
	}

	minDelay := 2
	maxDelay := 3

	pluginConfig := []byte(fmt.Sprintf(`{
			"service": "example",
			"reporting": {
				"min_delay_seconds": %v,
				"max_delay_seconds": %v
			}
		}`, minDelay, maxDelay))

	config, _ := ParseConfig(pluginConfig, fixture.manager.Services(), nil)

	fixture.plugin.Reconfigure(ctx, config)
	fixture.plugin.Stop(ctx)

	actualMin := time.Duration(*fixture.plugin.config.Reporting.MinDelaySeconds) / time.Nanosecond
	expectedMin := time.Duration(minDelay) * time.Second

	if actualMin != expectedMin {
		t.Fatalf("Expected minimum polling interval: %v but got %v", expectedMin, actualMin)
	}

	actualMax := time.Duration(*fixture.plugin.config.Reporting.MaxDelaySeconds) / time.Nanosecond
	expectedMax := time.Duration(maxDelay) * time.Second

	if actualMax != expectedMax {
		t.Fatalf("Expected maximum polling interval: %v but got %v", expectedMax, actualMax)
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

	config, _ := ParseConfig([]byte(pluginConfig), manager.Services(), nil)

	p := New(config, manager)

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
	t.t.Logf("decision log test server received %d events", len(events))
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

func setVersion(opaVersion string) {
	if version.Version == "" {
		version.Version = opaVersion
	}
}

func getVersion() string {
	return version.Version
}

func getWellKnownMetrics() metrics.Metrics {
	m := metrics.New()
	m.Counter("test_counter").Incr()
	return m
}
