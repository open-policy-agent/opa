// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

//go:build slow
// +build slow

package logs

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/logging/test"
	"github.com/open-policy-agent/opa/metrics"
	"github.com/open-policy-agent/opa/plugins"
	"github.com/open-policy-agent/opa/plugins/bundle"
	"github.com/open-policy-agent/opa/plugins/status"
	"github.com/open-policy-agent/opa/rego"
	"github.com/open-policy-agent/opa/server"
	"github.com/open-policy-agent/opa/storage"
	inmem "github.com/open-policy-agent/opa/storage/inmem/test"
	"github.com/open-policy-agent/opa/topdown"
	"github.com/open-policy-agent/opa/topdown/builtins"
	"github.com/open-policy-agent/opa/topdown/print"
	"github.com/open-policy-agent/opa/util"
	"github.com/open-policy-agent/opa/version"
)

func TestMain(m *testing.M) {
	version.Version = "XY.Z"
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

func (p *testPlugin) Log(_ context.Context, event EventV1) error {
	p.events = append(p.events, event)
	return nil
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

	// Server events with only `Revision` should not include bundles in the EventV1 struct
	for _, e := range backend.events {
		if len(e.Bundles) > 0 {
			t.Errorf("Unexpected `bundles` in event")
		}
	}
}

func TestPluginCustomBackendAndHTTPServiceAndConsole(t *testing.T) {

	ctx := context.Background()
	backend := testPlugin{}
	testLogger := test.New()

	fixture := newTestFixture(t, testFixtureOptions{
		ConsoleLogger: testLogger,
		ExtraManagerConfig: map[string]interface{}{
			"plugins": map[string]interface{}{"test_plugin": struct{}{}},
		},
		ExtraConfig: map[string]interface{}{
			"plugin":  "test_plugin",
			"console": true,
		},
		ManagerInit: func(m *plugins.Manager) {
			m.Register("test_plugin", &backend)
		},
	})

	defer fixture.server.stop()

	fixture.server.ch = make(chan []EventV1, 1)

	for i := 0; i < 2; i++ {
		fixture.plugin.Log(ctx, &server.Info{
			Revision: fmt.Sprint(i),
		})
	}
	fixture.plugin.flushDecisions(ctx)

	_, err := fixture.plugin.oneShot(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// check service
	var evs []EventV1
	select {
	case evs = <-fixture.server.ch:
	default:
	}

	if exp, act := 2, len(evs); exp != act {
		t.Errorf("Service: expected chunk len %v but got: %v", exp, act)
	}

	// check plugin
	if exp, act := 2, len(backend.events); exp != act {
		t.Fatalf("Plugin: expected %d events, got %d", exp, act)
	}
	if exp, act := "0", backend.events[0].Revision; exp != act {
		t.Errorf("Plugin: expected event 0 rev %s, got %s", exp, act)
	}
	if exp, act := "1", backend.events[1].Revision; exp != act {
		t.Errorf("Plugin: expected event 1 rev %s, got %s", exp, act)
	}

	// check console logger
	if exp, act := 2, len(testLogger.Entries()); exp != act {
		t.Fatalf("Console: expected %d events, got %d", exp, act)
	}
}

func TestPluginSingleBundle(t *testing.T) {
	ctx := context.Background()
	manager, _ := plugins.New(nil, "test-instance-id", inmem.New())

	backend := &testPlugin{}
	manager.Register("test_plugin", backend)

	config, err := ParseConfig([]byte(`{"plugin": "test_plugin"}`), nil, []string{"test_plugin"})
	if err != nil {
		t.Fatal(err)
	}

	plugin := New(config, manager)
	plugin.Log(ctx, &server.Info{Bundles: map[string]server.BundleInfo{"b1": {Revision: "A"}}})

	// Server events with `Bundles` should *not* have `Revision` set
	if len(backend.events) != 1 {
		t.Fatalf("Unexpected number of events: %v", backend.events)
	}

	if backend.events[0].Revision != "" || backend.events[0].Bundles["b1"].Revision != "A" {
		t.Fatal("Unexpected events: ", backend.events)
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
	plugin.Log(ctx, &server.Info{Path: "/"})
	plugin.Log(ctx, &server.Info{Path: "/data"}) // /v1/data/data case
	plugin.Log(ctx, &server.Info{Path: "/foo"})
	plugin.Log(ctx, &server.Info{Path: "foo"})
	plugin.Log(ctx, &server.Info{Path: "/foo/bar"})
	plugin.Log(ctx, &server.Info{Path: "a.b.c"})
	plugin.Log(ctx, &server.Info{Path: "/foo/a.b.c/bar"})
	plugin.Log(ctx, &server.Info{Query: "a = data.foo"})

	exp := []struct {
		query string
		path  string
	}{
		{path: "/"},
		{path: "/data"},
		{path: "/foo"},
		{path: "foo"},
		{path: "/foo/bar"},
		{path: "a.b.c"},
		{path: "/foo/a.b.c/bar"},
		{query: "a = data.foo"},
	}

	if len(exp) != len(backend.events) {
		t.Fatalf("Expected %d events but got %v", len(exp), len(backend.events))
	}

	for i, e := range exp {
		if e.query != backend.events[i].Query || e.path != backend.events[i].Path {
			t.Fatalf("Unexpected event %d, want %+v but got %+v", i, e, backend.events[i])
		}
	}
}

func TestPluginStartSameInput(t *testing.T) {

	ctx := context.Background()

	fixture := newTestFixture(t)
	defer fixture.server.stop()

	fixture.server.ch = make(chan []EventV1, 3)
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
			Path:       "tda/bar",
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
	expLen1 := 122
	expLen2 := 242
	expLen3 := 36

	if len(chunk1) != expLen1 || len(chunk2) != expLen2 || len(chunk3) != expLen3 {
		t.Fatalf("Expected chunk lens %v, %v, and %v but got: %v, %v, and %v", expLen1, expLen2, expLen3, len(chunk1), len(chunk2), len(chunk3))
	}

	var expInput interface{} = map[string]interface{}{"method": "GET"}

	msAsFloat64 := map[string]interface{}{}
	for k, v := range testMetrics.All() {
		msAsFloat64[k] = float64(v.(uint64))
	}

	exp := EventV1{
		Labels: map[string]string{
			"id":      "test-instance-id",
			"app":     "example-app",
			"version": version.Version,
		},
		Revision:    "399",
		DecisionID:  "399",
		Path:        "tda/bar",
		Input:       &expInput,
		Result:      &result,
		RequestedBy: "test",
		Timestamp:   ts,
		Metrics:     msAsFloat64,
	}

	if !reflect.DeepEqual(chunk3[expLen3-1], exp) {
		t.Fatalf("Expected %+v but got %+v", exp, chunk3[expLen3-1])
	}

	if fixture.plugin.status.Code != "" {
		t.Fatal("expected no error in status update")
	}
}

func TestPluginStartChangingInputValues(t *testing.T) {

	ctx := context.Background()

	fixture := newTestFixture(t)
	defer fixture.server.stop()

	fixture.server.ch = make(chan []EventV1, 3)
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
			Path:       "foo/bar",
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
	expLen1 := 124
	expLen2 := 247
	expLen3 := 29

	if len(chunk1) != expLen1 || len(chunk2) != expLen2 || len((chunk3)) != expLen3 {
		t.Fatalf("Expected chunk lens %v, %v and %v but got: %v, %v and %v", expLen1, expLen2, expLen3, len(chunk1), len(chunk2), len(chunk3))
	}

	var expInput interface{} = input

	exp := EventV1{
		Labels: map[string]string{
			"id":      "test-instance-id",
			"app":     "example-app",
			"version": version.Version,
		},
		Revision:    "399",
		DecisionID:  "399",
		Path:        "foo/bar",
		Input:       &expInput,
		Result:      &result,
		RequestedBy: "test",
		Timestamp:   ts,
	}

	if !reflect.DeepEqual(chunk3[expLen3-1], exp) {
		t.Fatalf("Expected %+v but got %+v", exp, chunk3[expLen3-1])
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
			Path:       "foo/bar",
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
			"id":      "test-instance-id",
			"app":     "example-app",
			"version": version.Version,
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

func logServerInfo(id string, input interface{}, result interface{}) *server.Info {
	return &server.Info{
		DecisionID: id,
		Path:       "data.foo.bar",
		Input:      &input,
		Results:    &result,
		RemoteAddr: "test",
		Timestamp:  time.Now().UTC(),
	}
}

func TestPluginRequeBufferPreserved(t *testing.T) {
	ctx := context.Background()

	fixture := newTestFixture(t, testFixtureOptions{ReportingUploadSizeLimitBytes: 300})
	defer fixture.server.stop()

	fixture.server.ch = make(chan []EventV1, 3)

	var input interface{} = map[string]interface{}{"method": "GET"}
	var result1 interface{} = false

	_ = fixture.plugin.Log(ctx, logServerInfo("abc", input, result1))
	_ = fixture.plugin.Log(ctx, logServerInfo("def", input, result1))
	_ = fixture.plugin.Log(ctx, logServerInfo("ghi", input, result1))

	bufLen := fixture.plugin.buffer.Len()
	if bufLen < 1 {
		t.Fatal("Expected buffer length of at least 1")
	}

	fixture.server.expCode = 500
	_, err := fixture.plugin.oneShot(ctx)
	if err == nil {
		t.Fatal("Expected error")
	}

	<-fixture.server.ch

	if fixture.plugin.buffer.Len() < bufLen {
		t.Fatal("Expected buffer to be preserved")
	}
}

func TestPluginRateLimitInt(t *testing.T) {
	ctx := context.Background()

	ts, err := time.Parse(time.RFC3339Nano, "2018-01-01T12:00:00.123456Z")
	if err != nil {
		panic(err)
	}

	numDecisions := 1 // 1 decision per second

	fixture := newTestFixture(t, testFixtureOptions{
		ReportingMaxDecisionsPerSecond: float64(numDecisions),
		ReportingUploadSizeLimitBytes:  300,
	})
	defer fixture.server.stop()

	var input interface{} = map[string]interface{}{"method": "GET"}
	var result interface{} = false

	event1 := &server.Info{
		DecisionID: "abc",
		Path:       "foo/bar",
		Input:      &input,
		Results:    &result,
		RemoteAddr: "test-1",
		Timestamp:  ts,
	}

	event2 := &server.Info{
		DecisionID: "def",
		Path:       "foo/baz",
		Input:      &input,
		Results:    &result,
		RemoteAddr: "test-2",
		Timestamp:  ts,
	}

	_ = fixture.plugin.Log(ctx, event1) // event 1 should be written into the encoder

	bytesWritten := fixture.plugin.enc.bytesWritten
	if bytesWritten == 0 {
		t.Fatal("Expected event to be written into the encoder")
	}

	_ = fixture.plugin.Log(ctx, event2) // event 2 should not be written into the encoder as rate limit exceeded

	if fixture.plugin.enc.bytesWritten != bytesWritten {
		t.Fatalf("Expected %v bytes written into the encoder but got %v", bytesWritten, fixture.plugin.enc.bytesWritten)
	}

	time.Sleep(1 * time.Second)
	_ = fixture.plugin.Log(ctx, event2) // event 2 should now be written into the encoder

	if fixture.plugin.buffer.Len() != 1 {
		t.Fatalf("Expected buffer length of 1 but got %v", fixture.plugin.buffer.Len())
	}

	exp := EventV1{
		Labels: map[string]string{
			"id":      "test-instance-id",
			"app":     "example-app",
			"version": version.Version,
		},
		DecisionID:  "abc",
		Path:        "foo/bar",
		Input:       &input,
		Result:      &result,
		RequestedBy: "test-1",
		Timestamp:   ts,
	}
	compareLogEvent(t, fixture.plugin.buffer.Pop(), exp)

	chunk, err := fixture.plugin.enc.Flush()
	if err != nil {
		t.Fatal(err)
	}

	if len(chunk) != 1 {
		t.Fatalf("Expected 1 chunk but got %v", len(chunk))
	}

	exp = EventV1{
		Labels: map[string]string{
			"id":      "test-instance-id",
			"app":     "example-app",
			"version": version.Version,
		},
		DecisionID:  "def",
		Path:        "foo/baz",
		Input:       &input,
		Result:      &result,
		RequestedBy: "test-2",
		Timestamp:   ts,
	}

	compareLogEvent(t, chunk[0], exp)
}

func TestPluginRateLimitFloat(t *testing.T) {
	ctx := context.Background()

	ts, err := time.Parse(time.RFC3339Nano, "2018-01-01T12:00:00.123456Z")
	if err != nil {
		panic(err)
	}

	numDecisions := 0.1 // 0.1 decision per second ie. 1 decision per 10 seconds
	fixture := newTestFixture(t, testFixtureOptions{
		ReportingMaxDecisionsPerSecond: float64(numDecisions),
		ReportingUploadSizeLimitBytes:  300,
	})
	defer fixture.server.stop()

	var input interface{} = map[string]interface{}{"method": "GET"}
	var result interface{} = false

	event1 := &server.Info{
		DecisionID: "abc",
		Path:       "foo/bar",
		Input:      &input,
		Results:    &result,
		RemoteAddr: "test-1",
		Timestamp:  ts,
	}

	event2 := &server.Info{
		DecisionID: "def",
		Path:       "foo/baz",
		Input:      &input,
		Results:    &result,
		RemoteAddr: "test-2",
		Timestamp:  ts,
	}

	_ = fixture.plugin.Log(ctx, event1) // event 1 should be written into the encoder

	bytesWritten := fixture.plugin.enc.bytesWritten
	if bytesWritten == 0 {
		t.Fatal("Expected event to be written into the encoder")
	}

	_ = fixture.plugin.Log(ctx, event2) // event 2 should not be written into the encoder as rate limit exceeded

	if fixture.plugin.enc.bytesWritten != bytesWritten {
		t.Fatalf("Expected %v bytes written into the encoder but got %v", bytesWritten, fixture.plugin.enc.bytesWritten)
	}

	time.Sleep(5 * time.Second)
	_ = fixture.plugin.Log(ctx, event2) // event 2 should not be written into the encoder as rate limit exceeded

	if fixture.plugin.enc.bytesWritten != bytesWritten {
		t.Fatalf("Expected %v bytes written into the encoder but got %v", bytesWritten, fixture.plugin.enc.bytesWritten)
	}

	time.Sleep(5 * time.Second)
	_ = fixture.plugin.Log(ctx, event2) // event 2 should now be written into the encoder

	if fixture.plugin.buffer.Len() != 1 {
		t.Fatalf("Expected buffer length of 1 but got %v", fixture.plugin.buffer.Len())
	}

	exp := EventV1{
		Labels: map[string]string{
			"id":      "test-instance-id",
			"app":     "example-app",
			"version": version.Version,
		},
		DecisionID:  "abc",
		Path:        "foo/bar",
		Input:       &input,
		Result:      &result,
		RequestedBy: "test-1",
		Timestamp:   ts,
	}

	compareLogEvent(t, fixture.plugin.buffer.Pop(), exp)

	chunk, err := fixture.plugin.enc.Flush()
	if err != nil {
		t.Fatal(err)
	}

	if len(chunk) != 1 {
		t.Fatalf("Expected 1 chunk but got %v", len(chunk))
	}

	exp = EventV1{
		Labels: map[string]string{
			"id":      "test-instance-id",
			"app":     "example-app",
			"version": version.Version,
		},
		DecisionID:  "def",
		Path:        "foo/baz",
		Input:       &input,
		Result:      &result,
		RequestedBy: "test-2",
		Timestamp:   ts,
	}

	compareLogEvent(t, chunk[0], exp)
}

func TestPluginStatusUpdateHTTPError(t *testing.T) {
	ctx := context.Background()

	fixture := newTestFixture(t, testFixtureOptions{ReportingUploadSizeLimitBytes: 300})
	defer fixture.server.stop()

	fixture.server.ch = make(chan []EventV1, 3)

	var input interface{} = map[string]interface{}{"method": "GET"}
	var result1 interface{} = false

	_ = fixture.plugin.Log(ctx, logServerInfo("abc", input, result1))
	_ = fixture.plugin.Log(ctx, logServerInfo("def", input, result1))
	_ = fixture.plugin.Log(ctx, logServerInfo("ghi", input, result1))

	bufLen := fixture.plugin.buffer.Len()
	if bufLen < 1 {
		t.Fatal("Expected buffer length of at least 1")
	}

	fixture.server.expCode = 500
	err := fixture.plugin.doOneShot(ctx)
	if err == nil {
		t.Fatal("Expected error")
	}

	<-fixture.server.ch

	if fixture.plugin.buffer.Len() < bufLen {
		t.Fatal("Expected buffer to be preserved")
	}

	if fixture.plugin.status.HTTPCode != "500" {
		t.Fatal("expected http_code to be 500 instead of ", fixture.plugin.status.HTTPCode)
	}

	msg := "log upload failed, server replied with HTTP 500 Internal Server Error"
	if fixture.plugin.status.Message != msg {
		t.Fatalf("expected status message to be %v instead of %v", msg, fixture.plugin.status.Message)
	}
}

func TestPluginStatusUpdate(t *testing.T) {
	ctx := context.Background()
	testLogger := test.New()

	ts, err := time.Parse(time.RFC3339Nano, "2018-01-01T12:00:00.123456Z")
	if err != nil {
		panic(err)
	}

	numDecisions := 1 // 1 decision per second
	fixture := newTestFixture(t, testFixtureOptions{
		ConsoleLogger:                  testLogger,
		ReportingMaxDecisionsPerSecond: float64(numDecisions),
		ReportingUploadSizeLimitBytes:  300,
	})
	defer fixture.server.stop()

	fixture.server.ch = make(chan []EventV1, 1)

	fixture.plugin.metrics = metrics.New()

	var input interface{} = map[string]interface{}{"method": "GET"}
	var result interface{} = false

	event1 := &server.Info{
		DecisionID: "abc",
		Path:       "foo/bar",
		Input:      &input,
		Results:    &result,
		RemoteAddr: "test-1",
		Timestamp:  ts,
	}

	event2 := &server.Info{
		DecisionID: "def",
		Path:       "foo/baz",
		Input:      &input,
		Results:    &result,
		RemoteAddr: "test-2",
		Timestamp:  ts,
	}

	event3 := &server.Info{
		DecisionID: "ghi",
		Path:       "foo/aux",
		Input:      &input,
		Results:    &result,
		RemoteAddr: "test-3",
		Timestamp:  ts,
	}

	_ = fixture.plugin.Log(ctx, event1) // event 1 should be written into the encoder

	fixture.plugin.mtx.Lock()
	if fixture.plugin.enc.bytesWritten == 0 {
		t.Fatal("Expected event to be written into the encoder")
	}
	fixture.plugin.mtx.Unlock()

	// Create a status plugin that logs to console
	pluginConfig := []byte(`{
			"console": true,
		}`)

	config, _ := status.ParseConfig(pluginConfig, fixture.manager.Services(), nil)
	p := status.New(config, fixture.manager).WithMetrics(fixture.plugin.metrics)

	fixture.manager.Register(status.Name, p)
	if err := fixture.manager.Start(ctx); err != nil {
		t.Fatal(err)
	}

	_ = fixture.plugin.Log(ctx, event2) // event 2 should not be written into the encoder as rate limit exceeded
	_ = fixture.plugin.Log(ctx, event3) // event 3 should not be written into the encoder as rate limit exceeded

	// Trigger a status update
	fixture.server.expCode = 200
	err = fixture.plugin.doOneShot(ctx)
	if err != nil {
		t.Fatal("Unexpected error")
	}

	<-fixture.server.ch

	// Give the logger / console some time to process and print the events
	time.Sleep(10 * time.Millisecond)
	p.Stop(ctx)

	entries := testLogger.Entries()
	if len(entries) == 0 {
		t.Fatal("Expected log entries but got none")
	}

	// Pick the last entry as it should have the decision log update
	e := entries[len(entries)-1]

	if _, ok := e.Fields["decision_logs"]; !ok {
		t.Fatal("Expected decision_log status update")
	}

	exp := map[string]interface{}{"metrics": map[string]interface{}{"counter_decision_logs_dropped": json.Number("2")}}

	if !reflect.DeepEqual(e.Fields["decision_logs"], exp) {
		t.Fatalf("Expected %v but got %v", exp, e.Fields["decision_logs"])
	}
}

func TestPluginRateLimitRequeue(t *testing.T) {
	ctx := context.Background()

	numDecisions := 100 // 100 decisions per second

	fixture := newTestFixture(t, testFixtureOptions{
		ReportingMaxDecisionsPerSecond: float64(numDecisions),
		ReportingUploadSizeLimitBytes:  300,
	})
	defer fixture.server.stop()

	fixture.server.ch = make(chan []EventV1, 3)

	var input interface{} = map[string]interface{}{"method": "GET"}
	var result1 interface{} = false

	_ = fixture.plugin.Log(ctx, logServerInfo("abc", input, result1)) // event 1
	_ = fixture.plugin.Log(ctx, logServerInfo("def", input, result1)) // event 2
	_ = fixture.plugin.Log(ctx, logServerInfo("ghi", input, result1)) // event 3

	bufLen := fixture.plugin.buffer.Len()
	if bufLen < 1 {
		t.Fatal("Expected buffer length of at least 1")
	}

	fixture.server.expCode = 500
	_, err := fixture.plugin.oneShot(ctx)
	if err == nil {
		t.Fatal("Expected error")
	}

	<-fixture.server.ch

	if fixture.plugin.buffer.Len() < bufLen {
		t.Fatal("Expected buffer to be preserved")
	}

	chunk, err := fixture.plugin.enc.Flush()
	if err != nil {
		t.Fatal(err)
	}

	if len(chunk) != 1 {
		t.Fatalf("Expected 1 chunk but got %v", len(chunk))
	}

	events := decodeLogEvent(t, chunk[0])

	if len(events) != 2 {
		t.Fatalf("Expected 2 event but got %v", len(events))
	}

	exp := "def"
	if events[0].DecisionID != exp {
		t.Fatalf("Expected decision log event id %v but got %v", exp, events[0].DecisionID)
	}

	exp = "ghi"
	if events[1].DecisionID != exp {
		t.Fatalf("Expected decision log event id %v but got %v", exp, events[1].DecisionID)
	}
}

func TestPluginRateLimitDropCountStatus(t *testing.T) {
	ctx := context.Background()
	testLogger := test.New()

	ts, err := time.Parse(time.RFC3339Nano, "2018-01-01T12:00:00.123456Z")
	if err != nil {
		panic(err)
	}

	numDecisions := 1 // 1 decision per second
	fixture := newTestFixture(t, testFixtureOptions{
		ConsoleLogger:                  testLogger,
		ReportingMaxDecisionsPerSecond: float64(numDecisions),
		ReportingUploadSizeLimitBytes:  300,
	})
	defer fixture.server.stop()

	fixture.plugin.metrics = metrics.New()

	var input interface{} = map[string]interface{}{"method": "GET"}
	var result interface{} = false

	event1 := &server.Info{
		DecisionID: "abc",
		Path:       "foo/bar",
		Input:      &input,
		Results:    &result,
		RemoteAddr: "test-1",
		Timestamp:  ts,
	}

	event2 := &server.Info{
		DecisionID: "def",
		Path:       "foo/baz",
		Input:      &input,
		Results:    &result,
		RemoteAddr: "test-2",
		Timestamp:  ts,
	}

	event3 := &server.Info{
		DecisionID: "ghi",
		Path:       "foo/aux",
		Input:      &input,
		Results:    &result,
		RemoteAddr: "test-3",
		Timestamp:  ts,
	}

	_ = fixture.plugin.Log(ctx, event1) // event 1 should be written into the encoder

	fixture.plugin.mtx.Lock()
	if fixture.plugin.enc.bytesWritten == 0 {
		t.Fatal("Expected event to be written into the encoder")
	}
	fixture.plugin.mtx.Unlock()

	// Create a status plugin that logs to console
	pluginConfig := []byte(`{
			"console": true,
		}`)

	config, _ := status.ParseConfig(pluginConfig, fixture.manager.Services(), nil)
	p := status.New(config, fixture.manager).WithMetrics(fixture.plugin.metrics)

	fixture.manager.Register(status.Name, p)
	if err := fixture.manager.Start(ctx); err != nil {
		t.Fatal(err)
	}

	_ = fixture.plugin.Log(ctx, event2) // event 2 should not be written into the encoder as rate limit exceeded
	_ = fixture.plugin.Log(ctx, event3) // event 3 should not be written into the encoder as rate limit exceeded

	// Trigger a status update
	status := testStatus()
	p.UpdateDiscoveryStatus(*status)

	// Give the logger / console some time to process and print the events
	time.Sleep(10 * time.Millisecond)
	p.Stop(ctx)

	entries := testLogger.Entries()
	if len(entries) == 0 {
		t.Fatal("Expected log entries but got none")
	}

	// Pick the last entry as it should have the drop count
	e := entries[len(entries)-1]

	if _, ok := e.Fields["metrics"]; !ok {
		t.Fatal("Expected metrics")
	}

	exp := map[string]interface{}{"<built-in>": map[string]interface{}{"counter_decision_logs_dropped": json.Number("2")}}

	if !reflect.DeepEqual(e.Fields["metrics"], exp) {
		t.Fatalf("Expected %v but got %v", exp, e.Fields["metrics"])
	}
}

func TestChunkMaxUploadSizeLimitNDBCacheDropping(t *testing.T) {
	ctx := context.Background()
	testLogger := test.New()

	ts, err := time.Parse(time.RFC3339Nano, "2018-01-01T12:00:00.123456Z")
	if err != nil {
		panic(err)
	}

	fixture := newTestFixture(t, testFixtureOptions{
		ConsoleLogger:                  testLogger,
		ReportingMaxDecisionsPerSecond: float64(1), // 1 decision per second
		ReportingUploadSizeLimitBytes:  400,
	})
	defer fixture.server.stop()

	fixture.plugin.metrics = metrics.New()

	var input interface{} = map[string]interface{}{"method": "GET"}
	var result interface{} = false

	// Purposely oversized NDBCache entry will force dropping during Log().
	var ndbCacheExample interface{} = ast.MustJSON(builtins.NDBCache{
		"test.custom_space_waster": ast.NewObject([2]*ast.Term{
			ast.ArrayTerm(),
			ast.StringTerm(strings.Repeat("Wasted space... ", 200)),
		}),
	}.AsValue())

	event := &server.Info{
		DecisionID:     "abc",
		Path:           "foo/bar",
		Input:          &input,
		Results:        &result,
		RemoteAddr:     "test",
		Timestamp:      ts,
		NDBuiltinCache: &ndbCacheExample,
	}

	beforeNDBDropCount := fixture.plugin.metrics.Counter(logNDBDropCounterName).Value().(uint64)
	err = fixture.plugin.Log(ctx, event) // event should be written into the encoder
	if err != nil {
		t.Fatal(err)
	}
	afterNDBDropCount := fixture.plugin.metrics.Counter(logNDBDropCounterName).Value().(uint64)

	if afterNDBDropCount != beforeNDBDropCount+1 {
		t.Fatalf("Expected %v NDBCache drop events, saw %v events instead.", beforeNDBDropCount+1, afterNDBDropCount)
	}
}

func TestPluginRateLimitBadConfig(t *testing.T) {
	manager, _ := plugins.New(nil, "test-instance-id", inmem.New())

	bufSize := 40000
	numDecisions := 10

	pluginConfig := []byte(fmt.Sprintf(`{
			"console": true,
			"reporting": {
				"buffer_size_limit_bytes": %v,
				"max_decisions_per_second": %v
			}
		}`, bufSize, numDecisions))

	_, err := ParseConfig(pluginConfig, manager.Services(), nil)
	if err == nil {
		t.Fatal("Expected error but got nil")
	}

	expected := "invalid decision_log config, specify either 'buffer_size_limit_bytes' or 'max_decisions_per_second'"
	if err.Error() != expected {
		t.Fatalf("Expected error message %v but got %v", expected, err.Error())
	}
}

func TestPluginNoLogging(t *testing.T) {
	// Given no custom plugin, no service(s) and no console logging configured,
	// this should not be an error, but neither do we need to initiate the plugin
	cases := []struct {
		note   string
		config []byte
	}{
		{
			note:   "no plugin attributes",
			config: []byte(`{}`),
		},
		{
			note:   "empty plugin configuration",
			config: []byte(`{"decision_logs": {}}`),
		},
		{
			note:   "only disabled console logger",
			config: []byte(`{"decision_logs": {"console": "false"}}`),
		},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			config, err := ParseConfig(tc.config, []string{}, nil)
			if err != nil {
				t.Errorf("expected no error: %v", err)
			}
			if config != nil {
				t.Errorf("excected no config for a no-op logging plugin")
			}
		})
	}
}

func TestPluginTriggerManual(t *testing.T) {
	ctx := context.Background()

	fixture := newTestFixture(t)
	defer fixture.server.stop()

	fixture.server.server.Config.SetKeepAlivesEnabled(false)

	fixture.server.ch = make(chan []EventV1, 4)
	tr := plugins.TriggerManual
	fixture.plugin.config.Reporting.Trigger = &tr

	if err := fixture.plugin.Start(ctx); err != nil {
		t.Fatal(err)
	}

	testMetrics := getWellKnownMetrics()
	msAsFloat64 := map[string]interface{}{}
	for k, v := range testMetrics.All() {
		msAsFloat64[k] = float64(v.(uint64))
	}

	var input interface{} = map[string]interface{}{"method": "GET"}
	var result interface{} = false

	ts, err := time.Parse(time.RFC3339Nano, "2018-01-01T12:00:00.123456Z")
	if err != nil {
		panic(err)
	}

	exp := EventV1{
		Labels: map[string]string{
			"id":      "test-instance-id",
			"app":     "example-app",
			"version": version.Version,
		},
		Path:        "tda/bar",
		Input:       &input,
		Result:      &result,
		RequestedBy: "test",
		Timestamp:   ts,
		Metrics:     msAsFloat64,
	}

	for i := 0; i < 400; i++ {
		fixture.plugin.Log(ctx, &server.Info{
			Revision:   fmt.Sprint(i),
			DecisionID: fmt.Sprint(i),
			Path:       "tda/bar",
			Input:      &input,
			Results:    &result,
			RemoteAddr: "test",
			Timestamp:  ts,
			Metrics:    testMetrics,
		})

		// trigger the decision log upload
		go func() {
			fixture.plugin.Trigger(ctx)
		}()

		chunk := <-fixture.server.ch

		expLen := 1
		if len(chunk) != 1 {
			t.Fatalf("Expected chunk len %v but got: %v", expLen, len(chunk))
		}

		exp.Revision = fmt.Sprint(i)
		exp.DecisionID = fmt.Sprint(i)

		if !reflect.DeepEqual(chunk[0], exp) {
			t.Fatalf("Expected %+v but got %+v", exp, chunk[0])
		}
	}

	fixture.plugin.Stop(ctx)
}

func TestPluginTriggerManualWithTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(3 * time.Second) // this should cause the context deadline to exceed
	}))

	// setup plugin pointing at fake server
	managerConfig := []byte(fmt.Sprintf(`{
			"labels": {
				"app": "example-app"
			},
			"services": [
				{
					"name": "example",
					"url": %q
				}
			]}`, s.URL))

	manager, err := plugins.New(
		managerConfig,
		"test-instance-id",
		inmem.New(),
		plugins.GracefulShutdownPeriod(10))
	if err != nil {
		t.Fatal(err)
	}

	pluginConfig := make(map[string]interface{})

	pluginConfig["service"] = "example"
	pluginConfig["resource"] = "/"

	pluginConfigBytes, err := json.MarshalIndent(pluginConfig, "", "  ")
	if err != nil {
		t.Fatal(err)
	}

	config, _ := ParseConfig(pluginConfigBytes, manager.Services(), nil)

	tr := plugins.TriggerManual
	config.Reporting.Trigger = &tr

	if s, ok := manager.PluginStatus()[Name]; ok {
		t.Fatalf("Unexpected status found in plugin manager for %s: %+v", Name, s)
	}

	p := New(config, manager)

	ensurePluginState(t, p, plugins.StateNotReady)

	if err := p.Start(ctx); err != nil {
		t.Fatal(err)
	}

	var input interface{} = map[string]interface{}{"method": "GET"}
	var result interface{} = false

	ts, err := time.Parse(time.RFC3339Nano, "2018-01-01T12:00:00.123456Z")
	if err != nil {
		panic(err)
	}

	err = p.Log(ctx, &server.Info{
		DecisionID: "0",
		Path:       "tda/bar",
		Input:      &input,
		Results:    &result,
		RemoteAddr: "test",
		Timestamp:  ts,
	})
	if err != nil {
		t.Fatal(err)
	}

	done := make(chan struct{})
	go func() {
		// this call should block till the context deadline exceeds
		p.Trigger(ctx)
		close(done)
	}()
	<-done

	if ctx.Err() == nil {
		t.Fatal("Expected error but got nil")
	}

	exp := "context deadline exceeded"
	if ctx.Err().Error() != exp {
		t.Fatalf("Expected error %v but got %v", exp, ctx.Err().Error())
	}

	p.Stop(ctx)
}

func TestPluginGracefulShutdownFlushesDecisions(t *testing.T) {
	ctx := context.Background()

	fixture := newTestFixture(t)
	defer fixture.server.stop()

	fixture.server.ch = make(chan []EventV1, 8)

	if err := fixture.plugin.Start(ctx); err != nil {
		t.Fatal(err)
	}

	var input interface{} = map[string]interface{}{"method": "GET"}
	var result interface{} = false

	logsSent := 200
	for i := 0; i < logsSent; i++ {
		input = generateInputMap(i)
		_ = fixture.plugin.Log(ctx, logServerInfo("abc", input, result))
	}

	fixture.server.expCode = 200

	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	fixture.plugin.Stop(timeoutCtx)

	close(fixture.server.ch)
	logsReceived := 0
	for element := range fixture.server.ch {
		logsReceived += len(element)
	}

	if logsReceived != logsSent {
		t.Fatalf("Expected %v, got %v", logsSent, logsReceived)
	}
}

func TestPluginTerminatesAfterGracefulShutdownPeriod(t *testing.T) {
	ctx := context.Background()

	fixture := newTestFixture(t)
	defer fixture.server.stop()

	fixture.server.ch = make(chan []EventV1, 1)
	fixture.server.expCode = 500

	if err := fixture.plugin.Start(ctx); err != nil {
		t.Fatal(err)
	}

	var input interface{} = map[string]interface{}{"method": "GET"}
	var result interface{} = false

	input = generateInputMap(0)
	_ = fixture.plugin.Log(ctx, logServerInfo("abc", input, result))

	timeoutCtx, cancel := context.WithTimeout(ctx, 1*time.Nanosecond)
	defer cancel()

	fixture.plugin.Stop(timeoutCtx)

	// Ensure the plugin was stopped without flushing its whole buffer
	if fixture.plugin.buffer.Len() == 0 && fixture.plugin.enc.buf.Len() == 0 {
		t.Errorf("Expected the plugin to still have buffered messages")
	}
}

func TestPluginTerminatesAfterGracefulShutdownPeriodWithoutLogs(t *testing.T) {
	ctx := context.Background()

	fixture := newTestFixture(t)
	defer fixture.server.stop()

	if err := fixture.plugin.Start(ctx); err != nil {
		t.Fatal(err)
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()

	fixture.plugin.Stop(timeoutCtx)
	if timeoutCtx.Err() != nil {
		t.Fatal("Stop did not exit before context expiration")
	}
}

func TestPluginReconfigure(t *testing.T) {

	ctx := context.Background()
	fixture := newTestFixture(t)
	defer fixture.server.stop()

	if err := fixture.plugin.Start(ctx); err != nil {
		t.Fatal(err)
	}

	ensurePluginState(t, fixture.plugin, plugins.StateOK)

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
	ensurePluginState(t, fixture.plugin, plugins.StateOK)

	fixture.plugin.Stop(ctx)
	ensurePluginState(t, fixture.plugin, plugins.StateNotReady)

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

func TestPluginReconfigureUploadSizeLimit(t *testing.T) {

	ctx := context.Background()
	limit := int64(300)

	fixture := newTestFixture(t, testFixtureOptions{
		ReportingUploadSizeLimitBytes: limit,
	})
	defer fixture.server.stop()

	if err := fixture.plugin.Start(ctx); err != nil {
		t.Fatal(err)
	}

	ensurePluginState(t, fixture.plugin, plugins.StateOK)

	fixture.plugin.mtx.Lock()
	if fixture.plugin.enc.limit != limit {
		t.Fatalf("Expected upload size limit %v but got %v", limit, fixture.plugin.enc.limit)
	}
	fixture.plugin.mtx.Unlock()

	newLimit := int64(600)

	pluginConfig := []byte(fmt.Sprintf(`{
			"service": "example",
			"reporting": {
				"upload_size_limit_bytes": %v,
			}
		}`, newLimit))

	config, _ := ParseConfig(pluginConfig, fixture.manager.Services(), nil)

	fixture.plugin.Reconfigure(ctx, config)
	ensurePluginState(t, fixture.plugin, plugins.StateOK)

	fixture.plugin.Stop(ctx)
	ensurePluginState(t, fixture.plugin, plugins.StateNotReady)

	fixture.plugin.mtx.Lock()
	if fixture.plugin.enc.limit != newLimit {
		t.Fatalf("Expected upload size limit %v but got %v", newLimit, fixture.plugin.enc.limit)
	}
	fixture.plugin.mtx.Unlock()
}

type appendingPrintHook struct {
	printed *[]string
}

func (a appendingPrintHook) Print(_ print.Context, s string) error {
	*a.printed = append(*a.printed, s)
	return nil
}

func TestPluginMasking(t *testing.T) {
	tests := []struct {
		note          string
		rawPolicy     []byte
		expErased     []string
		expMasked     []string
		expPrinted    []string
		errManager    error
		expErr        error
		input         interface{}
		expected      interface{}
		ndbcache      interface{}
		ndbc_expected interface{}
		reconfigure   bool
	}{
		{
			note: "simple erase (with body true)",
			rawPolicy: []byte(`
				package system.log
				mask["/input/password"] {
					input.input.is_sensitive
				}`),
			expErased: []string{"/input/password"},
			input: map[string]interface{}{
				"is_sensitive": true,
				"password":     "secret",
			},
			expected: map[string]interface{}{
				"is_sensitive": true,
			},
		},
		{
			note: "simple erase (with body true, plugin reconfigured)",
			rawPolicy: []byte(`
				package system.log
				mask["/input/password"] {
					input.input.is_sensitive
				}`),
			expErased: []string{"/input/password"},
			input: map[string]interface{}{
				"is_sensitive": true,
				"password":     "secret",
			},
			expected: map[string]interface{}{
				"is_sensitive": true,
			},
			reconfigure: true,
		},
		{
			note: "simple upsert (with body true)",
			rawPolicy: []byte(`
				package system.log
				mask[{"op": "upsert", "path": "/input/password", "value": x}] {
					input.input.password
					x := "**REDACTED**"
				}`),
			expMasked: []string{"/input/password"},
			input: map[string]interface{}{
				"is_sensitive": true,
				"password":     "mySecretPassword",
			},
			expected: map[string]interface{}{
				"is_sensitive": true,
				"password":     "**REDACTED**",
			},
		},
		{
			note: "remove even with value set in rule body",
			rawPolicy: []byte(`
				package system.log
				mask[{"op": "remove", "path": "/input/password", "value": x}] {
					input.input.password
					x := "**REDACTED**"
				}`),
			expErased: []string{"/input/password"},
			input: map[string]interface{}{
				"is_sensitive": true,
				"password":     "mySecretPassword",
			},
			expected: map[string]interface{}{
				"is_sensitive": true,
			},
		},
		{
			note: "remove when value not defined",
			rawPolicy: []byte(`
				package system.log
				mask[{"op": "remove", "path": "/input/password"}] {
					input.input.password
				}`),
			expErased: []string{"/input/password"},
			input: map[string]interface{}{
				"is_sensitive": true,
				"password":     "mySecretPassword",
			},
			expected: map[string]interface{}{
				"is_sensitive": true,
			},
		},
		{
			note: "remove when value not defined in rule body",
			rawPolicy: []byte(`
				package system.log
				mask[{"op": "remove", "path": "/input/password", "value": x}] {
					input.input.password
				}`),
			errManager: fmt.Errorf("1 error occurred: test.rego:3: rego_unsafe_var_error: var x is unsafe"),
		},
		{
			note: "simple erase - no match",
			rawPolicy: []byte(`
				package system.log
				mask["/input/password"] {
					input.input.is_sensitive
				}`),
			input: map[string]interface{}{
				"is_not_sensitive": true,
				"password":         "secret",
			},
			expected: map[string]interface{}{
				"is_not_sensitive": true,
				"password":         "secret",
			},
		},
		{
			note: "complex upsert - object key",
			rawPolicy: []byte(`
				package system.log
				mask[{"op": "upsert", "path": "/input/foo", "value": x}] {
					input.input.foo
					x := [
						{"nabs": 1}
					]
				}`),
			input: map[string]interface{}{
				"bar": 1,
				"foo": []map[string]interface{}{{"baz": 1}},
			},
			// Due to ast.JSON() parsing as part of rego.eval, internal mapped
			// types from mask rule valuations (for numbers) will be json.Number.
			// This affects explicitly providing the expected interface{} value.
			//
			// See TestMaksRuleErase where tests are written to confirm json marshalled
			// output is as expected.
			expected: map[string]interface{}{
				"bar": 1,
				"foo": []interface{}{map[string]interface{}{"nabs": json.Number("1")}},
			},
		},
		{
			note: "upsert failure: unsupported type []map[string]interface{}",
			rawPolicy: []byte(`
				package system.log
				mask[{"op": "upsert", "path": "/input/foo/boo", "value": x}] {
					x := [
						{"nabs": 1}
					]
				}`),
			input: map[string]interface{}{
				"bar": json.Number("1"),
				"foo": []map[string]interface{}{{"baz": json.Number("1")}},
			},
			expected: map[string]interface{}{
				"bar": json.Number("1"),
				"foo": []map[string]interface{}{{"baz": json.Number("1")}},
			},
		},
		{
			note: "mixed mode - complex #1",
			rawPolicy: []byte(`
				package system.log

				mask["/input/password"] {
					input.input.is_sensitive
				}

				# invalidate JWT signature
				mask[{"op": "upsert", "path": "/input/jwt", "value": x}]  {
					input.input.jwt

					# split jwt string
					parts := split(input.input.jwt, ".")

					# make sure we have 3 parts
					count(parts) == 3

					# replace signature
					new := array.concat(array.slice(parts, 0, 2), [base64url.encode("**REDACTED**")])
					x = concat(".", new)

				}

				mask[{"op": "upsert", "path": "/input/foo", "value": x}] {
					input.input.foo
					x := [
						{"changed": 1}
					]
				}`),
			input: map[string]interface{}{
				"is_sensitive": true,
				"jwt":          "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.cThIIoDvwdueQB468K5xDc5633seEFoqwxjF_xSJyQQ",
				"bar":          1,
				"foo":          []map[string]interface{}{{"baz": 1}},
				"password":     "mySecretPassword",
			},
			expected: map[string]interface{}{
				"is_sensitive": true,
				"jwt":          "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.KipSRURBQ1RFRCoq",
				"bar":          1,
				"foo":          []interface{}{map[string]interface{}{"changed": json.Number("1")}},
			},
		},
		{
			note: "print() works",
			rawPolicy: []byte(`
				package system.log
				mask["/input/password"] {
					print("Erasing /input/password")
					input.input.is_sensitive
				}`),
			expErased: []string{"/input/password"},
			input: map[string]interface{}{
				"is_sensitive": true,
				"password":     "secret",
			},
			expected: map[string]interface{}{
				"is_sensitive": true,
			},
			expPrinted: []string{"Erasing /input/password"},
		},
		{
			note: "simple upsert on nd_builtin_cache",
			rawPolicy: []byte(`
				package system.log
				mask[{"op": "upsert", "path": "/nd_builtin_cache/rand.intn", "value": x}] {
					input.nd_builtin_cache["rand.intn"]
					x := "**REDACTED**"
				}`),
			expMasked: []string{"/nd_builtin_cache/rand.intn"},
			ndbcache: map[string]interface{}{
				// Simulate rand.intn("z", 15) call, with output of 7.
				"rand.intn": map[string]interface{}{"[\"z\",15]": json.Number("7")},
			},
			ndbc_expected: map[string]interface{}{
				"rand.intn": "**REDACTED**",
			},
		},
		{
			note: "simple upsert on nd_builtin_cache with multiple entries",
			rawPolicy: []byte(`
				package system.log
				mask[{"op": "upsert", "path": "/nd_builtin_cache/rand.intn", "value": x}] {
					input.nd_builtin_cache["rand.intn"]
					x := "**REDACTED**"
				}

				mask[{"op": "upsert", "path": "/nd_builtin_cache/net.lookup_ip_addr", "value": y}] {
					obj := input.nd_builtin_cache["net.lookup_ip_addr"]
					y := object.union({k: "4.4.x.x" | obj[k]; startswith(k, "[\"4.4.")},
					                  {k: obj[k] | obj[k]; not startswith(k, "[\"4.4.")})
				}
				`),
			expMasked: []string{"/nd_builtin_cache/net.lookup_ip_addr", "/nd_builtin_cache/rand.intn"},
			ndbcache: map[string]interface{}{
				// Simulate rand.intn("z", 15) call, with output of 7.
				"rand.intn": map[string]interface{}{"[\"z\",15]": json.Number("7")},
				"net.lookup_ip_addr": map[string]interface{}{
					"[\"1.1.1.1\"]": "1.1.1.1",
					"[\"2.2.2.2\"]": "2.2.2.2",
					"[\"3.3.3.3\"]": "3.3.3.3",
					"[\"4.4.4.4\"]": "4.4.4.4",
				},
			},
			ndbc_expected: map[string]interface{}{
				"rand.intn": "**REDACTED**",
				"net.lookup_ip_addr": map[string]interface{}{
					"[\"1.1.1.1\"]": "1.1.1.1",
					"[\"2.2.2.2\"]": "2.2.2.2",
					"[\"3.3.3.3\"]": "3.3.3.3",
					"[\"4.4.4.4\"]": "4.4.x.x",
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			// Setup masking fixture. Populate store with simple masking policy.
			ctx := context.Background()
			store := inmem.New()

			err := storage.Txn(ctx, store, storage.WriteParams, func(txn storage.Transaction) error {
				if err := store.UpsertPolicy(ctx, txn, "test.rego", tc.rawPolicy); err != nil {
					return err
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}

			var output []string

			// Create and start manager. Start is required so that stored policies
			// get compiled and made available to the plugin.
			manager, err := plugins.New(
				nil,
				"test",
				store,
				plugins.EnablePrintStatements(true),
				plugins.PrintHook(appendingPrintHook{printed: &output}),
			)
			if err != nil {
				t.Fatal(err)
			} else if err := manager.Start(ctx); err != nil {
				if tc.errManager != nil {
					if tc.errManager.Error() != err.Error() {
						t.Fatalf("expected error %s, but got %s", tc.errManager.Error(), err.Error())
					}
					return
				}
			}

			// Instantiate the plugin.
			cfg := &Config{Service: "svc"}
			trigger := plugins.DefaultTriggerMode
			cfg.validateAndInjectDefaults([]string{"svc"}, nil, &trigger)

			plugin := New(cfg, manager)

			if err := plugin.Start(ctx); err != nil {
				t.Fatal(err)
			}

			event := &EventV1{
				Input:          &tc.input,
				NDBuiltinCache: &tc.ndbcache,
			}

			if err := plugin.maskEvent(ctx, nil, event); err != nil {
				t.Fatal(err)
			}

			if !reflect.DeepEqual(tc.expected, *event.Input) {
				t.Fatalf("Expected %#+v but got %#+v:", tc.expected, *event.Input)
			}

			if !reflect.DeepEqual(tc.ndbc_expected, *event.NDBuiltinCache) {
				t.Fatalf("Expected %#+v but got %#+v:", tc.ndbc_expected, *event.NDBuiltinCache)
			}

			if len(tc.expErased) > 0 {
				if !reflect.DeepEqual(tc.expErased, event.Erased) {
					t.Fatalf("Expected erased %v set but got %v", tc.expErased, event.Erased)
				}
			}

			if len(tc.expMasked) > 0 {
				if !reflect.DeepEqual(tc.expMasked, event.Masked) {
					t.Fatalf("Expected masked %v set but got %v", tc.expMasked, event.Masked)
				}
			}

			if !reflect.DeepEqual(tc.expPrinted, output) {
				t.Errorf("Expected output %v, got %v", tc.expPrinted, output)
			}

			// if reconfigure in test is on
			if tc.reconfigure {
				// Reconfigure and ensure that mask is invalidated.
				maskDecision := "dead/beef"
				newConfig := &Config{Service: "svc", MaskDecision: &maskDecision}
				if err := newConfig.validateAndInjectDefaults([]string{"svc"}, nil, &trigger); err != nil {
					t.Fatal(err)
				}

				plugin.Reconfigure(ctx, newConfig)

				event = &EventV1{
					Input: &tc.input,
				}

				if err := plugin.maskEvent(ctx, nil, event); err != nil {
					t.Fatal(err)
				}

				if !reflect.DeepEqual(*event.Input, tc.input) {
					t.Fatalf("Expected %v but got modified input %v", tc.input, event.Input)
				}

			}

		})
	}
}

func TestPluginDrop(t *testing.T) {
	// Test cases
	tests := []struct {
		note      string
		rawPolicy []byte
		event     *EventV1
		expected  bool
	}{
		{
			note: "simple drop",
			rawPolicy: []byte(`
			package system.log
			drop {
				endswith(input.path, "bar")
			}`),
			event: &EventV1{Path: "foo/bar"},

			expected: true,
		},
		{
			note: "no drop",
			rawPolicy: []byte(`
			package system.log
			drop {
				endswith(input.path, "bar")
			}`),
			event:    &EventV1{Path: "foo/foo"},
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			// Setup fixture. Populate store with simple drop policy.
			ctx := context.Background()
			store := inmem.New()

			//checks if raw policy is valid and stores policy in store
			err := storage.Txn(ctx, store, storage.WriteParams, func(txn storage.Transaction) error {
				if err := store.UpsertPolicy(ctx, txn, "test.rego", tc.rawPolicy); err != nil {
					return err
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}

			var output []string

			// Create and start manager. Start is required so that stored policies
			// get compiled and made available to the plugin.
			manager, err := plugins.New(
				nil,
				"test",
				store,
				plugins.EnablePrintStatements(true),
				plugins.PrintHook(appendingPrintHook{printed: &output}),
			)
			if err != nil {
				t.Fatal(err)
			}
			if err := manager.Start(ctx); err != nil {
				t.Fatal(err)
			}

			// Instantiate the plugin.
			cfg := &Config{Service: "svc"}
			trigger := plugins.DefaultTriggerMode
			cfg.validateAndInjectDefaults([]string{"svc"}, nil, &trigger)

			plugin := New(cfg, manager)

			if err := plugin.Start(ctx); err != nil {
				t.Fatal(err)
			}

			drop, err := plugin.dropEvent(ctx, nil, tc.event)
			if err != nil {
				t.Fatal(err)
			}

			if tc.expected != drop {
				t.Errorf("Plugin: Expected drop to be %v got %v", tc.expected, drop)
			}
		})
	}
}

type testFixtureOptions struct {
	ConsoleLogger                  *test.Logger
	ReportingUploadSizeLimitBytes  int64
	ReportingMaxDecisionsPerSecond float64
	Resource                       *string
	TestServerPath                 *string
	PartitionName                  *string
	ExtraConfig                    map[string]interface{}
	ExtraManagerConfig             map[string]interface{}
	ManagerInit                    func(*plugins.Manager)
}

type testFixture struct {
	manager       *plugins.Manager
	consoleLogger *test.Logger
	plugin        *Plugin
	server        *testServer
}

func newTestFixture(t *testing.T, opts ...testFixtureOptions) testFixture {

	var options testFixtureOptions
	if len(opts) > 0 {
		options = opts[0]
	}

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

	mgrCfg := make(map[string]interface{})
	err := json.Unmarshal(managerConfig, &mgrCfg)
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range options.ExtraManagerConfig {
		mgrCfg[k] = v
	}
	managerConfig, err = json.MarshalIndent(mgrCfg, "", "  ")
	if err != nil {
		t.Fatal(err)
	}

	manager, err := plugins.New(
		managerConfig,
		"test-instance-id",
		inmem.New(),
		plugins.GracefulShutdownPeriod(10),
		plugins.ConsoleLogger(options.ConsoleLogger))
	if err != nil {
		t.Fatal(err)
	}
	if init := options.ManagerInit; init != nil {
		init(manager)
	}

	pluginConfig := map[string]interface{}{
		"service": "example",
	}

	if options.Resource != nil {
		pluginConfig["resource"] = *options.Resource
	}

	if options.PartitionName != nil {
		pluginConfig["partition_name"] = *options.PartitionName
	}

	for k, v := range options.ExtraConfig {
		pluginConfig[k] = v
	}

	pluginConfigBytes, err := json.MarshalIndent(pluginConfig, "", "  ")
	if err != nil {
		t.Fatal(err)
	}

	config, err := ParseConfig(pluginConfigBytes, manager.Services(), manager.Plugins())
	if err != nil {
		t.Fatal(err)
	}

	if options.TestServerPath != nil {
		ts.path = *options.TestServerPath
	}

	if options.ReportingMaxDecisionsPerSecond != 0 {
		config.Reporting.MaxDecisionsPerSecond = &options.ReportingMaxDecisionsPerSecond
	}

	if options.ReportingUploadSizeLimitBytes != 0 {
		config.Reporting.UploadSizeLimitBytes = &options.ReportingUploadSizeLimitBytes
	}

	if s, ok := manager.PluginStatus()[Name]; ok {
		t.Fatalf("Unexpected status found in plugin manager for %s: %+v", Name, s)
	}

	p := New(config, manager)

	ensurePluginState(t, p, plugins.StateNotReady)

	return testFixture{
		manager:       manager,
		consoleLogger: options.ConsoleLogger,
		plugin:        p,
		server:        &ts,
	}

}

func TestParseConfigUseDefaultServiceNoConsole(t *testing.T) {
	services := []string{
		"s0",
		"s1",
		"s3",
	}

	loggerConfig := []byte(`{
		"console": false
	}`)

	config, err := ParseConfig([]byte(loggerConfig), services, nil)

	if err != nil {
		t.Errorf("Unexpected error: %s", err)
	}

	if config.Service != services[0] {
		t.Errorf("Expected %s service in config, actual = '%s'", services[0], config.Service)
	}
}

func TestParseConfigDefaultServiceWithConsole(t *testing.T) {
	services := []string{
		"s0",
		"s1",
		"s3",
	}

	loggerConfig := []byte(`{
		"console": true
	}`)

	config, err := ParseConfig([]byte(loggerConfig), services, nil)

	if err != nil {
		t.Errorf("Unexpected error: %s", err)
	}

	if config.Service != "" {
		t.Errorf("Expected no service in config, actual = '%s'", config.Service)
	}
}

func TestParseConfigTriggerMode(t *testing.T) {
	cases := []struct {
		note     string
		config   []byte
		expected plugins.TriggerMode
		wantErr  bool
		err      error
	}{
		{
			note:     "default trigger mode",
			config:   []byte(`{}`),
			expected: plugins.DefaultTriggerMode,
		},
		{
			note:     "manual trigger mode",
			config:   []byte(`{"reporting": {"trigger": "manual"}}`),
			expected: plugins.TriggerManual,
		},
		{
			note:     "trigger mode mismatch",
			config:   []byte(`{"reporting": {"trigger": "manual"}}`),
			expected: plugins.TriggerPeriodic,
			wantErr:  true,
			err:      fmt.Errorf("invalid decision_log config: trigger mode mismatch: periodic and manual (hint: check discovery configuration)"),
		},
		{
			note:     "bad trigger mode",
			config:   []byte(`{"reporting": {"trigger": "foo"}}`),
			expected: "foo",
			wantErr:  true,
			err:      fmt.Errorf("invalid decision_log config: invalid trigger mode \"foo\" (want \"periodic\" or \"manual\")"),
		},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {

			c, err := NewConfigBuilder().WithBytes(tc.config).WithServices([]string{"s0"}).WithTriggerMode(&tc.expected).Parse()

			if tc.wantErr {
				if err == nil {
					t.Fatal("Expected error but got nil")
				}

				if tc.err != nil && tc.err.Error() != err.Error() {
					t.Fatalf("Expected error message %v but got %v", tc.err.Error(), err.Error())
				}
			} else {
				if err != nil {
					t.Fatalf("Unexpected error %v", err)
				}

				if *c.Reporting.Trigger != tc.expected {
					t.Fatalf("Expected trigger mode %v but got %v", tc.expected, *c.Reporting.Trigger)
				}
			}
		})
	}
}

func TestEventV1ToAST(t *testing.T) {
	input := `{"foo": [{"bar": 1, "baz": {"2": 3.3333333, "4": null}}]}`
	var goInput interface{} = string(util.MustMarshalJSON(input))
	astInput, err := roundtripJSONToAST(goInput)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	var result interface{} = map[string]interface{}{
		"x": true,
	}

	var bigEvent EventV1
	if err := util.UnmarshalJSON([]byte(largeEvent), &bigEvent); err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	var ndbCacheExample interface{} = ast.MustJSON(builtins.NDBCache{
		"time.now_ns": ast.NewObject([2]*ast.Term{
			ast.ArrayTerm(),
			ast.NumberTerm("1663803565571081429"),
		}),
	}.AsValue())

	cases := []struct {
		note  string
		event EventV1
	}{
		{
			note:  "empty event",
			event: EventV1{},
		},
		{
			note: "basic event no result",
			event: EventV1{
				Labels:      map[string]string{"foo": "1", "bar": "2"},
				DecisionID:  "1234567890",
				Path:        "/http/authz/allow",
				RequestedBy: "[::1]:59943",
				Timestamp:   time.Now(),
			},
		},
		{
			note: "event with error",
			event: EventV1{
				Labels:     map[string]string{},
				DecisionID: "1234567890",
				Path:       "/system/main",
				Error: rego.Errors{&topdown.Error{
					Code:     topdown.BuiltinErr,
					Message:  "Some error happened somewhere",
					Location: ast.NewLocation([]byte("myfunc(x)"), "policy.rego", 22, 17),
				}},
				RequestedBy: "[::1]:59943",
				Timestamp:   time.Now(),
			},
		},
		{
			note: "event with input and result",
			event: EventV1{
				Labels:      map[string]string{"foo": "1", "bar": "2"},
				DecisionID:  "1234567890",
				Input:       &goInput,
				Path:        "/http/authz/allow",
				RequestedBy: "[::1]:59943",
				Result:      &result,
				Timestamp:   time.Now(),
				inputAST:    astInput,
			},
		},
		{
			note: "event without ast input",
			event: EventV1{
				Labels:      map[string]string{"foo": "1", "bar": "2"},
				DecisionID:  "1234567890",
				Input:       &goInput,
				Path:        "/http/authz/allow",
				RequestedBy: "[::1]:59943",
				Result:      &result,
				Timestamp:   time.Now(),
			},
		},
		{
			note: "event with bundles",
			event: EventV1{
				Labels:     map[string]string{"foo": "1", "bar": "2"},
				DecisionID: "1234567890",
				Bundles: map[string]BundleInfoV1{
					"b1": {"revision7"},
					"b2": {"0"},
					"b3": {},
				},
				Input:       &goInput,
				Path:        "/http/authz/allow",
				RequestedBy: "[::1]:59943",
				Result:      &result,
				Timestamp:   time.Now(),
				inputAST:    astInput,
			},
		},
		{
			note: "event with erased",
			event: EventV1{
				Erased:     []string{"input/password", "result/secret"},
				Labels:     map[string]string{"foo": "1", "bar": "2"},
				DecisionID: "1234567890",
				Bundles: map[string]BundleInfoV1{
					"b1": {"revision7"},
					"b2": {"0"},
					"b3": {},
				},
				Input:       &goInput,
				Path:        "/http/authz/allow",
				RequestedBy: "[::1]:59943",
				Result:      &result,
				Timestamp:   time.Now(),
				inputAST:    astInput,
			},
		},
		{
			note: "event with masked",
			event: EventV1{
				Masked:     []string{"input/password", "result/secret"},
				Labels:     map[string]string{"foo": "1", "bar": "2"},
				DecisionID: "1234567890",
				Bundles: map[string]BundleInfoV1{
					"b1": {"revision7"},
					"b2": {"0"},
					"b3": {},
				},
				Input:       &goInput,
				Path:        "/http/authz/allow",
				RequestedBy: "[::1]:59943",
				Result:      &result,
				Timestamp:   time.Now(),
				inputAST:    astInput,
			},
		},
		{
			note:  "big event",
			event: bigEvent,
		},
		{
			note: "event with nd_builtin_cache",
			event: EventV1{
				Labels:     map[string]string{"foo": "1", "bar": "2"},
				DecisionID: "1234567890",
				Bundles: map[string]BundleInfoV1{
					"b1": {"revision7"},
					"b2": {"0"},
					"b3": {},
				},
				Input:          &goInput,
				Path:           "/http/authz/allow",
				RequestedBy:    "[::1]:59943",
				Result:         &result,
				Timestamp:      time.Now(),
				inputAST:       astInput,
				NDBuiltinCache: &ndbCacheExample,
			},
		},
		{
			note: "event with req id",
			event: EventV1{
				Labels:     map[string]string{"foo": "1", "bar": "2"},
				DecisionID: "1234567890",
				Bundles: map[string]BundleInfoV1{
					"b1": {"revision7"},
					"b2": {"0"},
					"b3": {},
				},
				Input:       &goInput,
				Path:        "/http/authz/allow",
				RequestedBy: "[::1]:59943",
				Result:      &result,
				Timestamp:   time.Now(),
				RequestID:   1,
				inputAST:    astInput,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {

			// Ensure that the custom AST() function gives the same
			// result as round tripping through JSON

			expected, err := roundtripJSONToAST(tc.event)
			if err != nil {
				t.Fatalf("Unexpected error: %s", err)
			}
			actual, err := tc.event.AST()
			if err != nil {
				t.Fatalf("Unexpected error: %s", err)
			}

			if expected.Compare(actual) != 0 {
				t.Fatalf("\nExpected:\n%s\n\nGot:\n%s\n\n", expected, actual)
			}

		})
	}
}

func TestPluginDefaultResourcePath(t *testing.T) {

	ctx := context.Background()

	testServerPath := "/logs"

	fixture := newTestFixture(t, testFixtureOptions{
		TestServerPath: &testServerPath,
	})
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

	if *fixture.plugin.config.Resource != defaultResourcePath {
		t.Errorf("Expected the resource path to be the default %s, actual = '%s'", defaultResourcePath, *fixture.plugin.config.Resource)
	}

	fixture.server.expCode = 200

	_, err := fixture.plugin.oneShot(ctx)
	if err != nil {
		t.Fatal(err)
	}
}

func TestPluginResourcePathAndPartitionName(t *testing.T) {

	ctx := context.Background()

	resourcePath := "/resource/path"
	partitionName := "partition"
	expectedPath := fmt.Sprintf("/logs/%v", partitionName)

	fixture := newTestFixture(t, testFixtureOptions{
		Resource:       &resourcePath,
		TestServerPath: &expectedPath,
		PartitionName:  &partitionName,
	})
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

	if *fixture.plugin.config.Resource != expectedPath {
		t.Errorf("Expected resource to be %s, but got %s", expectedPath, *fixture.plugin.config.Resource)
	}

	fixture.server.expCode = 200

	_, err := fixture.plugin.oneShot(ctx)
	if err != nil {
		t.Fatal(err)
	}
}

func TestPluginResourcePath(t *testing.T) {

	ctx := context.Background()

	resourcePath := "/plugin/log/path"
	testServerPath := "/plugin/log/path"

	fixture := newTestFixture(t, testFixtureOptions{
		Resource:       &resourcePath,
		TestServerPath: &testServerPath,
	})
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

	if *fixture.plugin.config.Resource != resourcePath {
		t.Errorf("Expected resource to be %s, but got %s", resourcePath, *fixture.plugin.config.Resource)
	}

	fixture.server.expCode = 200

	_, err := fixture.plugin.oneShot(ctx)
	if err != nil {
		t.Fatal(err)
	}
}

type testServer struct {
	t       *testing.T
	expCode int
	server  *httptest.Server
	ch      chan []EventV1
	path    string
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
	if t.path != "" && r.URL.Path != t.path {
		t.t.Fatalf("expecting the request path %s to equal the configured path: %s", r.URL.Path, t.path)
	}

	t.t.Logf("decision log test server received %d events at path %s", len(events), r.URL.Path)
	t.ch <- events
	w.WriteHeader(t.expCode)
}

func (t *testServer) start() {
	t.server = httptest.NewServer(http.HandlerFunc(t.handle))
}

// stop the testServer. This should only be done at the end of a test!
func (t *testServer) stop() {
	// Drain any pending events to ensure the server can stop
	for len(t.ch) > 0 {
		<-t.ch
	}
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

func getWellKnownMetrics() metrics.Metrics {
	m := metrics.New()
	m.Counter("test_counter").Incr()
	return m
}

func ensurePluginState(t *testing.T, p *Plugin, state plugins.State) {
	t.Helper()
	status, ok := p.manager.PluginStatus()[Name]
	if !ok {
		t.Fatalf("Expected to find state for %s, found nil", Name)
		return
	}
	if status.State != state {
		t.Fatalf("Unexpected status state found in plugin manager for %s:\n\n\tFound:%+v\n\n\tExpected: %s", Name, status.State, plugins.StateOK)
	}
}

func decodeLogEvent(t *testing.T, bs []byte) []EventV1 {
	gr, err := gzip.NewReader(bytes.NewReader(bs))
	if err != nil {
		t.Fatal(err)
	}

	var events []EventV1
	if err := json.NewDecoder(gr).Decode(&events); err != nil {
		t.Fatal(err)
	}

	if err := gr.Close(); err != nil {
		t.Fatal(err)
	}

	return events
}

func compareLogEvent(t *testing.T, actual []byte, exp EventV1) {
	events := decodeLogEvent(t, actual)
	if len(events) != 1 {
		t.Fatalf("Expected 1 event but got %v", len(events))
	}

	if !reflect.DeepEqual(events[0], exp) {
		t.Fatalf("Expected %+v but got %+v", exp, events[0])
	}
}

func testStatus() *bundle.Status {

	tDownload, _ := time.Parse(time.RFC3339Nano, "2018-01-01T00:00:00.0000000Z")
	tActivate, _ := time.Parse(time.RFC3339Nano, "2018-01-01T00:00:01.0000000Z")

	status := bundle.Status{
		Name:                     "example/authz",
		ActiveRevision:           "quickbrawnfaux",
		LastSuccessfulDownload:   tDownload,
		LastSuccessfulActivation: tActivate,
	}

	return &status
}
