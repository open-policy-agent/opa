// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

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
	"testing"
	"time"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/metrics"
	"github.com/open-policy-agent/opa/plugins"
	"github.com/open-policy-agent/opa/plugins/bundle"
	"github.com/open-policy-agent/opa/plugins/status"
	"github.com/open-policy-agent/opa/rego"
	"github.com/open-policy-agent/opa/server"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/storage/inmem"
	"github.com/open-policy-agent/opa/topdown"
	"github.com/open-policy-agent/opa/util"
	"github.com/open-policy-agent/opa/version"
	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
)

func TestMain(m *testing.M) {
	version.Version = "XY.Z"
	os.Exit(m.Run())
}

type testPlugin struct {
	events []EventV1
}

type testPluginCustomizer func(c *Config)

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

	fixture := newTestFixture(t, func(c *Config) {
		limit := int64(300)
		c.Reporting.UploadSizeLimitBytes = &limit
	})
	defer fixture.server.stop()

	fixture.server.ch = make(chan []EventV1, 3)

	var input interface{} = map[string]interface{}{"method": "GET"}
	var result1 interface{} = false

	_ = fixture.plugin.Log(ctx, logServerInfo("abc", input, result1))
	_ = fixture.plugin.Log(ctx, logServerInfo("def", input, result1))
	_ = fixture.plugin.Log(ctx, logServerInfo("ghi", input, result1))

	bufLen := fixture.plugin.buffer.Len()
	if bufLen < 2 {
		t.Fatal("Expected buffer length of at least 2")
	}

	fixture.server.expCode = 500
	_, err := fixture.plugin.oneShot(ctx)
	if err == nil {
		t.Fatal("Expected error")
	}

	_ = <-fixture.server.ch

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

	fixture := newTestFixture(t, func(c *Config) {
		limit := float64(numDecisions)
		c.Reporting.MaxDecisionsPerSecond = &limit
	}, func(c *Config) {
		limit := int64(300)
		c.Reporting.UploadSizeLimitBytes = &limit
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

	compareLogEvent(t, chunk, exp)
}

func TestPluginRateLimitFloat(t *testing.T) {
	ctx := context.Background()

	ts, err := time.Parse(time.RFC3339Nano, "2018-01-01T12:00:00.123456Z")
	if err != nil {
		panic(err)
	}

	numDecisions := 0.1 // 0.1 decision per second ie. 1 decision per 10 seconds

	fixture := newTestFixture(t, func(c *Config) {
		limit := float64(numDecisions)
		c.Reporting.MaxDecisionsPerSecond = &limit
	}, func(c *Config) {
		limit := int64(300)
		c.Reporting.UploadSizeLimitBytes = &limit
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

	compareLogEvent(t, chunk, exp)
}

func TestPluginRateLimitRequeue(t *testing.T) {
	ctx := context.Background()

	numDecisions := 100 // 100 decisions per second

	fixture := newTestFixture(t, func(c *Config) {
		limit := float64(numDecisions)
		c.Reporting.MaxDecisionsPerSecond = &limit
	}, func(c *Config) {
		limit := int64(300)
		c.Reporting.UploadSizeLimitBytes = &limit
	})
	defer fixture.server.stop()

	fixture.server.ch = make(chan []EventV1, 3)

	var input interface{} = map[string]interface{}{"method": "GET"}
	var result1 interface{} = false

	_ = fixture.plugin.Log(ctx, logServerInfo("abc", input, result1)) // event 1
	_ = fixture.plugin.Log(ctx, logServerInfo("def", input, result1)) // event 2
	_ = fixture.plugin.Log(ctx, logServerInfo("ghi", input, result1)) // event 3

	bufLen := fixture.plugin.buffer.Len()
	if bufLen < 2 {
		t.Fatal("Expected buffer length of at least 2")
	}

	fixture.server.expCode = 500
	_, err := fixture.plugin.oneShot(ctx)
	if err == nil {
		t.Fatal("Expected error")
	}

	_ = <-fixture.server.ch

	if fixture.plugin.buffer.Len() < bufLen {
		t.Fatal("Expected buffer to be preserved")
	}

	chunk, err := fixture.plugin.enc.Flush()
	if err != nil {
		t.Fatal(err)
	}

	events := decodeLogEvent(t, chunk)
	if len(events) != 1 {
		t.Fatalf("Expected 1 event but got %v", len(events))
	}
}

func TestPluginRateLimitDropCountStatus(t *testing.T) {
	ctx := context.Background()

	ts, err := time.Parse(time.RFC3339Nano, "2018-01-01T12:00:00.123456Z")
	if err != nil {
		panic(err)
	}

	numDecisions := 1 // 1 decision per second

	fixture := newTestFixture(t, func(c *Config) {
		limit := float64(numDecisions)
		c.Reporting.MaxDecisionsPerSecond = &limit
	}, func(c *Config) {
		limit := int64(300)
		c.Reporting.UploadSizeLimitBytes = &limit
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

	if fixture.plugin.enc.bytesWritten == 0 {
		t.Fatal("Expected event to be written into the encoder")
	}

	// Create a status plugin that logs to console
	pluginConfig := []byte(fmt.Sprintf(`{
			"console": true,
		}`))

	config, _ := status.ParseConfig(pluginConfig, fixture.manager.Services())
	p := status.New(config, fixture.manager).WithMetrics(fixture.plugin.metrics)

	fixture.manager.Register(status.Name, p)
	if err := fixture.manager.Start(ctx); err != nil {
		panic(err)
	}

	logLevel := logrus.GetLevel()
	defer logrus.SetLevel(logLevel)

	// Ensure that status messages are printed to console even with the standard logger configured to log errors only
	logrus.SetLevel(logrus.ErrorLevel)

	hook := test.NewLocal(plugins.GetConsoleLogger())

	_ = fixture.plugin.Log(ctx, event2) // event 2 should not be written into the encoder as rate limit exceeded
	_ = fixture.plugin.Log(ctx, event3) // event 3 should not be written into the encoder as rate limit exceeded

	// Trigger a status update
	status := testStatus()
	p.UpdateDiscoveryStatus(*status)

	// Give the logger / console some time to process and print the events
	time.Sleep(10 * time.Millisecond)

	entries := hook.AllEntries()
	if len(entries) == 0 {
		t.Fatal("Expected log entries but got none")
	}

	// Pick the last entry as it should have the drop count
	e := entries[len(entries)-1]

	if _, ok := e.Data["metrics"]; !ok {
		t.Fatal("Expected metrics")
	}

	exp := map[string]interface{}{"<built-in>": map[string]interface{}{"counter_decision_logs_dropped": json.Number("2")}}

	if !reflect.DeepEqual(e.Data["metrics"], exp) {
		t.Fatalf("Expected %v but got %v", exp, e.Data["metrics"])
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

func TestPluginMasking(t *testing.T) {
	tests := []struct {
		note        string
		rawPolicy   []byte
		expErased   []string
		expMasked   []string
		errManager  error
		expErr      error
		input       interface{}
		expected    interface{}
		reconfigure bool
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

			// Create and start manager. Start is required so that stored policies
			// get compiled and made available to the plugin.
			manager, err := plugins.New(nil, "test", store)
			if err != nil {
				t.Fatal(err)
			} else if err := manager.Start(ctx); err != nil {
				if tc.errManager != nil {
					if tc.errManager.Error() != err.Error() {
						t.Fatalf("expected error %s, but got %s", tc.errManager.Error(), err.Error())
					}
				}
			}

			// Instantiate the plugin.
			cfg := &Config{Service: "svc"}
			cfg.validateAndInjectDefaults([]string{"svc"}, nil)
			plugin := New(cfg, manager)

			if err := plugin.Start(ctx); err != nil {
				t.Fatal(err)
			}

			event := &EventV1{
				Input: &tc.input,
			}

			if err := plugin.maskEvent(ctx, nil, event); err != nil {
				t.Fatal(err)
			}

			if !reflect.DeepEqual(tc.expected, *event.Input) {
				t.Fatalf("Expected %#+v but got %#+v:", tc.expected, *event.Input)
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

			// if reconfigure in test is on
			if tc.reconfigure {
				// Reconfigure and ensure that mask is invalidated.
				maskDecision := "dead/beef"
				newConfig := &Config{Service: "svc", MaskDecision: &maskDecision}
				if err := newConfig.validateAndInjectDefaults([]string{"svc"}, nil); err != nil {
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

type testFixture struct {
	manager *plugins.Manager
	plugin  *Plugin
	server  *testServer
}

func newTestFixture(t *testing.T, options ...testPluginCustomizer) testFixture {

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

	manager, err := plugins.New(managerConfig, "test-instance-id", inmem.New(), plugins.GracefulShutdownPeriod(10))
	if err != nil {
		t.Fatal(err)
	}

	config, _ := ParseConfig([]byte(`{"service": "example"}`), manager.Services(), nil)
	for _, option := range options {
		option(config)
	}

	if s, ok := manager.PluginStatus()[Name]; ok {
		t.Fatalf("Unexpected status found in plugin manager for %s: %+v", Name, s)
	}

	p := New(config, manager)

	ensurePluginState(t, p, plugins.StateNotReady)

	return testFixture{
		manager: manager,
		plugin:  p,
		server:  &ts,
	}

}

func TestParseConfigUseDefaultServiceNoConsole(t *testing.T) {
	services := []string{
		"s0",
		"s1",
		"s3",
	}

	loggerConfig := []byte(fmt.Sprintf(`{
		"console": false
	}`))

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

	loggerConfig := []byte(fmt.Sprintf(`{
		"console": true
	}`))

	config, err := ParseConfig([]byte(loggerConfig), services, nil)

	if err != nil {
		t.Errorf("Unexpected error: %s", err)
	}

	if config.Service != "" {
		t.Errorf("Expected no service in config, actual = '%s'", config.Service)
	}
}

func TestParseConfigDefaultServiceWithNoServiceOrConsole(t *testing.T) {
	loggerConfig := []byte(fmt.Sprintf(`{}`))

	_, err := ParseConfig([]byte(loggerConfig), []string{}, nil)

	if err == nil {
		t.Errorf("Expected an error but err==nil")
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

	tDownload, _ := time.Parse("2018-01-01T00:00:00.0000000Z", time.RFC3339Nano)
	tActivate, _ := time.Parse("2018-01-01T00:00:01.0000000Z", time.RFC3339Nano)

	status := bundle.Status{
		Name:                     "example/authz",
		ActiveRevision:           "quickbrawnfaux",
		LastSuccessfulDownload:   tDownload,
		LastSuccessfulActivation: tActivate,
	}

	return &status
}
