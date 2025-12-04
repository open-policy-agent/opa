// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

//go:build slow

package logs

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/logging"
	"github.com/open-policy-agent/opa/v1/logging/test"
	"github.com/open-policy-agent/opa/v1/metrics"
	"github.com/open-policy-agent/opa/v1/plugins"
	"github.com/open-policy-agent/opa/v1/plugins/bundle"
	"github.com/open-policy-agent/opa/v1/plugins/status"
	"github.com/open-policy-agent/opa/v1/rego"
	"github.com/open-policy-agent/opa/v1/server"
	"github.com/open-policy-agent/opa/v1/storage"
	inmem "github.com/open-policy-agent/opa/v1/storage/inmem/test"
	"github.com/open-policy-agent/opa/v1/topdown"
	"github.com/open-policy-agent/opa/v1/topdown/builtins"
	"github.com/open-policy-agent/opa/v1/topdown/print"
	"github.com/open-policy-agent/opa/v1/util"
	"github.com/open-policy-agent/opa/v1/version"
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

func (p *testPlugin) Reconfigure(context.Context, any) {
}

func (p *testPlugin) Log(_ context.Context, event EventV1) error {
	p.events = append(p.events, event)
	return nil
}

func TestPluginCustomBackend(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	manager, _ := plugins.New(nil, "test-instance-id", inmem.New())

	backend := &testPlugin{}
	manager.Register("test_plugin", backend)

	config, err := ParseConfig([]byte(`{"plugin": "test_plugin"}`), nil, []string{"test_plugin"})
	if err != nil {
		t.Fatal(err)
	}

	plugin := New(config, manager)
	if err := plugin.Log(ctx, &server.Info{Revision: "A"}); err != nil {
		t.Fatal(err)
	}
	if err := plugin.Log(ctx, &server.Info{Revision: "B"}); err != nil {
		t.Fatal(err)
	}

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

func TestLogCustomField(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	manager, _ := plugins.New(nil, "test-instance-id", inmem.New())

	backend := &testPlugin{}
	manager.Register("test_plugin", backend)

	config, err := ParseConfig([]byte(`{"plugin": "test_plugin"}`), nil, []string{"test_plugin"})
	if err != nil {
		t.Fatal(err)
	}

	plugin := New(config, manager)
	if err := plugin.Log(ctx, &server.Info{Custom: map[string]any{"abc": "xyz"}}); err != nil {
		t.Fatal(err)
	}
	if err := plugin.Log(ctx, &server.Info{Custom: map[string]any{"ok": 2025}}); err != nil {
		t.Fatal(err)
	}

	if exp, act := 2, len(backend.events); exp != act {
		t.Fatalf("expected %d events, got %d", exp, act)
	}

	exp := []map[string]any{
		{"abc": "xyz"},
		{"ok": 2025},
	}
	act := []map[string]any{
		backend.events[0].Custom,
		backend.events[1].Custom,
	}
	if diff := cmp.Diff(exp, act); diff != "" {
		t.Errorf("unexpected logs (-want, +got):\n%s", diff)
	}
}

func TestPluginCustomBackendAndHTTPServiceAndConsole(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	backend := testPlugin{}
	testLogger := test.New()

	fixture := newTestFixture(t, testFixtureOptions{
		ConsoleLogger: testLogger,
		ExtraManagerConfig: map[string]any{
			"plugins": map[string]any{"test_plugin": struct{}{}},
		},
		ExtraConfig: map[string]any{
			"plugin":  "test_plugin",
			"console": true,
		},
		ManagerInit: func(m *plugins.Manager) {
			m.Register("test_plugin", &backend)
		},
	})

	defer fixture.server.stop()

	fixture.server.ch = make(chan []EventV1, 1)

	for i := range 2 {
		if err := fixture.plugin.Log(ctx, &server.Info{
			Revision: strconv.Itoa(i),
		}); err != nil {
			t.Fatal(err)
		}
	}
	fixture.plugin.flushDecisions(ctx)

	err := fixture.plugin.b.Upload(ctx)
	fmt.Println(errors.Is(err, &bufferEmpty{}))
	if err != nil && !errors.Is(err, &bufferEmpty{}) {
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

func TestPluginRequestContext(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	manager, _ := plugins.New(nil, "test-instance-id", inmem.New())

	backend := &testPlugin{}
	manager.Register("test_plugin", backend)

	h1 := http.Header{}
	h1.Set("foo", "bar")

	h2 := http.Header{}
	h2.Set("foo", "bar")
	h2.Add("foo2", "bar")
	h2.Add("foo2", "bar2")

	cases := []struct {
		note         string
		config       []byte
		decisionInfo *server.Info
		expected     *RequestContext
	}{
		{
			note:         "no request context config - no request context in decision info",
			config:       []byte(`{"plugin": "test_plugin"}`),
			decisionInfo: &server.Info{Bundles: map[string]server.BundleInfo{"b1": {Revision: "A"}}},
			expected:     nil,
		},
		{
			note:         "request context in config (single header) - no request context in decision info",
			config:       []byte(`{"plugin": "test_plugin", "request_context": {"http": {"headers": ["foo"]}}}`),
			decisionInfo: &server.Info{Bundles: map[string]server.BundleInfo{"b1": {Revision: "A"}}},
			expected:     nil,
		},
		{
			note:         "request context in config (single header) - request context in decision info (no header map)",
			config:       []byte(`{"plugin": "test_plugin", "request_context": {"http": {"headers": ["foo"]}}}`),
			decisionInfo: &server.Info{Bundles: map[string]server.BundleInfo{"b1": {Revision: "A"}}, HTTPRequestContext: logging.HTTPRequestContext{Header: nil}},
			expected:     nil,
		},
		{
			note:         "request context in config (single header) - request context in decision info (with header map)",
			config:       []byte(`{"plugin": "test_plugin", "request_context": {"http": {"headers": ["foo"]}}}`),
			decisionInfo: &server.Info{Bundles: map[string]server.BundleInfo{"b1": {Revision: "A"}}, HTTPRequestContext: logging.HTTPRequestContext{Header: h1}},
			expected:     &RequestContext{HTTPRequest: &HTTPRequestContext{Headers: map[string][]string{"foo": {"bar"}}}},
		},
		{
			note:         "request context in config (multiple headers) - request context in decision info (with header map partial)",
			config:       []byte(`{"plugin": "test_plugin", "request_context": {"http": {"headers": ["foo", "foo2"]}}}`),
			decisionInfo: &server.Info{Bundles: map[string]server.BundleInfo{"b1": {Revision: "A"}}, HTTPRequestContext: logging.HTTPRequestContext{Header: h1}},
			expected:     &RequestContext{HTTPRequest: &HTTPRequestContext{Headers: map[string][]string{"foo": {"bar"}}}},
		},
		{
			note:         "request context in config (multiple headers) - request context in decision info (with header map full)",
			config:       []byte(`{"plugin": "test_plugin", "request_context": {"http": {"headers": ["foo", "foo2"]}}}`),
			decisionInfo: &server.Info{Bundles: map[string]server.BundleInfo{"b1": {Revision: "A"}}, HTTPRequestContext: logging.HTTPRequestContext{Header: h2}},
			expected:     &RequestContext{HTTPRequest: &HTTPRequestContext{Headers: map[string][]string{"foo": {"bar"}, "foo2": {"bar", "bar2"}}}},
		},
		{
			note:         "request context in config (single header) - request context in decision info (with header map full)",
			config:       []byte(`{"plugin": "test_plugin", "request_context": {"http": {"headers": ["foo"]}}}`),
			decisionInfo: &server.Info{Bundles: map[string]server.BundleInfo{"b1": {Revision: "A"}}, HTTPRequestContext: logging.HTTPRequestContext{Header: h2}},
			expected:     &RequestContext{HTTPRequest: &HTTPRequestContext{Headers: map[string][]string{"foo": {"bar"}}}},
		},
		{
			note:         "no request context in config - request context in decision info (with header map)",
			config:       []byte(`{"plugin": "test_plugin"}`),
			decisionInfo: &server.Info{Bundles: map[string]server.BundleInfo{"b1": {Revision: "A"}}, HTTPRequestContext: logging.HTTPRequestContext{Header: h1}},
			expected:     nil,
		},
		{
			note:         "request context in config (no http) - request context in decision info (with header map)",
			config:       []byte(`{"plugin": "test_plugin", "request_context": {}}`),
			decisionInfo: &server.Info{Bundles: map[string]server.BundleInfo{"b1": {Revision: "A"}}, HTTPRequestContext: logging.HTTPRequestContext{Header: h1}},
			expected:     nil,
		},
		{
			note:         "request context in config (no headers) - request context in decision info (with header map)",
			config:       []byte(`{"plugin": "test_plugin", "request_context": {"http": {}}}`),
			decisionInfo: &server.Info{Bundles: map[string]server.BundleInfo{"b1": {Revision: "A"}}, HTTPRequestContext: logging.HTTPRequestContext{Header: h1}},
			expected:     nil,
		},
		{
			note:         "request context in config (empty headers list) - request context in decision info (with header map)",
			config:       []byte(`{"plugin": "test_plugin", "request_context": {"http": {"headers": []}}}`),
			decisionInfo: &server.Info{Bundles: map[string]server.BundleInfo{"b1": {Revision: "A"}}, HTTPRequestContext: logging.HTTPRequestContext{Header: h1}},
			expected:     nil,
		},
	}

	for i, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			config, err := ParseConfig(tc.config, nil, []string{"test_plugin"})
			if err != nil {
				t.Fatal(err)
			}

			plugin := New(config, manager)
			if err := plugin.Log(ctx, tc.decisionInfo); err != nil {
				t.Fatal(err)
			}

			if len(backend.events) == 0 {
				t.Fatal("expected at least one event")
			}

			if !reflect.DeepEqual(backend.events[i].RequestContext, tc.expected) {
				t.Fatalf("unexpected request context, want %+v but got %+v", tc.expected, backend.events[0].RequestContext)
			}
		})
	}
}

func TestPluginSingleBundle(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	manager, _ := plugins.New(nil, "test-instance-id", inmem.New())

	backend := &testPlugin{}
	manager.Register("test_plugin", backend)

	config, err := ParseConfig([]byte(`{"plugin": "test_plugin"}`), nil, []string{"test_plugin"})
	if err != nil {
		t.Fatal(err)
	}

	plugin := New(config, manager)
	if err := plugin.Log(ctx, &server.Info{Bundles: map[string]server.BundleInfo{"b1": {Revision: "A"}}}); err != nil {
		t.Fatal(err)
	}

	// Server events with `Bundles` should *not* have `Revision` set
	if len(backend.events) != 1 {
		t.Fatalf("Unexpected number of events: %v", backend.events)
	}

	if backend.events[0].Revision != "" || backend.events[0].Bundles["b1"].Revision != "A" {
		t.Fatal("Unexpected events: ", backend.events)
	}
}

func TestPluginErrorNoResult(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	manager, _ := plugins.New(nil, "test-instance-id", inmem.New())

	backend := &testPlugin{}
	manager.Register("test_plugin", backend)

	config, err := ParseConfig([]byte(`{"plugin": "test_plugin"}`), nil, []string{"test_plugin"})
	if err != nil {
		t.Fatal(err)
	}

	plugin := New(config, manager)
	if err := plugin.Log(ctx, &server.Info{Error: errors.New("some error")}); err != nil {
		t.Fatal(err)
	}
	if err := plugin.Log(ctx, &server.Info{Error: ast.Errors{&ast.Error{Code: "some_error"}}}); err != nil {
		t.Fatal(err)
	}

	if len(backend.events) != 2 || backend.events[0].Error == nil || backend.events[1].Error == nil {
		t.Fatal("Unexpected events:", backend.events)
	}
}

func TestPluginQueriesAndPaths(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	manager, _ := plugins.New(nil, "test-instance-id", inmem.New())

	backend := &testPlugin{}
	manager.Register("test_plugin", backend)

	config, err := ParseConfig([]byte(`{"plugin": "test_plugin"}`), nil, []string{"test_plugin"})
	if err != nil {
		t.Fatal(err)
	}

	plugin := New(config, manager)
	if err := plugin.Log(ctx, &server.Info{Path: "/"}); err != nil {
		t.Fatal(err)
	}
	if err := plugin.Log(ctx, &server.Info{Path: "/data"}); err != nil { // /v1/data/data case
		t.Fatal(err)
	}
	if err := plugin.Log(ctx, &server.Info{Path: "/foo"}); err != nil {
		t.Fatal(err)
	}
	if err := plugin.Log(ctx, &server.Info{Path: "foo"}); err != nil {
		t.Fatal(err)
	}
	if err := plugin.Log(ctx, &server.Info{Path: "/foo/bar"}); err != nil {
		t.Fatal(err)
	}
	if err := plugin.Log(ctx, &server.Info{Path: "a.b.c"}); err != nil {
		t.Fatal(err)
	}
	if err := plugin.Log(ctx, &server.Info{Path: "/foo/a.b.c/bar"}); err != nil {
		t.Fatal(err)
	}
	if err := plugin.Log(ctx, &server.Info{Query: "a = data.foo"}); err != nil {
		t.Fatal(err)
	}

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
	t.Parallel()
	ctx := context.Background()

	fixture := newTestFixture(t)
	defer fixture.server.stop()

	fixture.server.ch = make(chan []EventV1, 3)
	var result any = false

	ts, err := time.Parse(time.RFC3339Nano, "2018-01-01T12:00:00.123456Z")
	if err != nil {
		panic(err)
	}

	testMetrics := getWellKnownMetrics()

	var input any = map[string]any{"method": "GET"}

	for i := range 400 {
		if err := fixture.plugin.Log(ctx, &server.Info{
			Revision:   strconv.Itoa(i),
			DecisionID: strconv.Itoa(i),
			Path:       "tda/bar",
			Input:      &input,
			Results:    &result,
			RemoteAddr: "test",
			Timestamp:  ts,
			Metrics:    testMetrics,
		}); err != nil {
			t.Fatal(err)
		}
	}

	err = fixture.plugin.b.Upload(ctx)
	if err != nil {
		t.Fatal(err)
	}

	chunk1 := <-fixture.server.ch
	chunk2 := <-fixture.server.ch
	chunk3 := <-fixture.server.ch
	// first size is smallest as the adaptive uncompressed limit increases more events can be added
	expLen1 := 122
	expLen2 := 243
	expLen3 := 35

	if len(chunk1) != expLen1 || len(chunk2) != expLen2 || len(chunk3) != expLen3 {
		t.Fatalf("Expected chunk lens %v, %v, and %v but got: %v, %v, and %v", expLen1, expLen2, expLen3, len(chunk1), len(chunk2), len(chunk3))
	}

	var expInput any = map[string]any{"method": "GET"}

	msAsFloat64 := map[string]any{}
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
	t.Parallel()

	ctx := context.Background()

	fixture := newTestFixture(t)
	defer fixture.server.stop()

	fixture.server.ch = make(chan []EventV1, 3)
	var result any = false

	ts, err := time.Parse(time.RFC3339Nano, "2018-01-01T12:00:00.123456Z")
	if err != nil {
		panic(err)
	}

	var input any

	for i := range 400 {
		input = map[string]any{"method": getValueForMethod(i), "path": getValueForPath(i), "user": getValueForUser(i)}

		if err := fixture.plugin.Log(ctx, &server.Info{
			Revision:   strconv.Itoa(i),
			DecisionID: strconv.Itoa(i),
			Path:       "foo/bar",
			Input:      &input,
			Results:    &result,
			RemoteAddr: "test",
			Timestamp:  ts,
		}); err != nil {
			t.Fatal(err)
		}
	}

	err = fixture.plugin.b.Upload(ctx)
	if err != nil {
		t.Fatal(err)
	}

	chunk1 := <-fixture.server.ch
	chunk2 := <-fixture.server.ch
	chunk3 := <-fixture.server.ch
	expLen1 := 125
	expLen2 := 248
	expLen3 := 27

	if len(chunk1) != expLen1 || len(chunk2) != expLen2 || len(chunk3) != expLen3 {
		t.Fatalf("Expected chunk lens %v, %v and %v but got: %v, %v and %v", expLen1, expLen2, expLen3, len(chunk1), len(chunk2), len(chunk3))
	}

	exp := EventV1{
		Labels: map[string]string{
			"id":      "test-instance-id",
			"app":     "example-app",
			"version": version.Version,
		},
		Revision:    "399",
		DecisionID:  "399",
		Path:        "foo/bar",
		Input:       &input,
		Result:      &result,
		RequestedBy: "test",
		Timestamp:   ts,
	}

	if !reflect.DeepEqual(chunk3[expLen3-1], exp) {
		t.Fatalf("Expected %+v but got %+v", exp, chunk3[expLen3-1])
	}
}

func TestPluginStartChangingInputKeysAndValues(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	fixture := newTestFixture(t)
	defer fixture.server.stop()

	fixture.server.ch = make(chan []EventV1, 2)
	var result any = false

	ts, err := time.Parse(time.RFC3339Nano, "2018-01-01T12:00:00.123456Z")
	if err != nil {
		panic(err)
	}

	var input any

	for i := range 250 {
		input = generateInputMap(i)

		if err := fixture.plugin.Log(ctx, &server.Info{
			Revision:   strconv.Itoa(i),
			DecisionID: strconv.Itoa(i),
			Path:       "foo/bar",
			Input:      &input,
			Results:    &result,
			RemoteAddr: "test",
			Timestamp:  ts,
		}); err != nil {
			t.Fatal(err)
		}
	}

	err = fixture.plugin.b.Upload(ctx)
	if err != nil {
		t.Fatal(err)
	}

	<-fixture.server.ch
	chunk2 := <-fixture.server.ch

	exp := EventV1{
		Labels: map[string]string{
			"id":      "test-instance-id",
			"app":     "example-app",
			"version": version.Version,
		},
		Revision:    "249",
		DecisionID:  "249",
		Path:        "foo/bar",
		Input:       &input,
		Result:      &result,
		RequestedBy: "test",
		Timestamp:   ts,
	}

	if !reflect.DeepEqual(chunk2[len(chunk2)-1], exp) {
		t.Fatalf("Expected %+v but got %+v", exp, chunk2[len(chunk2)-1])
	}
}

func TestPluginRequeue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                string
		reportingBufferType string
	}{
		{
			name:                "using event buffer",
			reportingBufferType: eventBufferType,
		},
		{
			name: "using size buffer",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			fixture := newTestFixture(t, testFixtureOptions{
				ReportingBufferType: tc.reportingBufferType,
			})
			defer fixture.server.stop()

			fixture.server.ch = make(chan []EventV1, 1)

			var input any = map[string]any{"method": "GET"}
			var result1 any = false

			if err := fixture.plugin.Log(ctx, &server.Info{
				DecisionID: "abc",
				Path:       "data.foo.bar",
				Input:      &input,
				Results:    &result1,
				RemoteAddr: "test",
				Timestamp:  time.Now().UTC(),
			}); err != nil {
				t.Fatal(err)
			}

			fixture.server.expCode = 500
			err := fixture.plugin.b.Upload(ctx)
			if err == nil {
				t.Fatal("Expected error")
			}

			events1 := <-fixture.server.ch

			fixture.server.expCode = 200

			err = fixture.plugin.b.Upload(ctx)
			if err != nil {
				t.Fatal(err)
			}

			events2 := <-fixture.server.ch

			if !reflect.DeepEqual(events1, events2) {
				t.Fatalf("Expected %v but got: %v", events1, events2)
			}

			err = fixture.plugin.b.Upload(ctx)
			if err != nil && !errors.Is(err, &bufferEmpty{}) {
				t.Fatalf("Unexpected error or upload, err: %v", err)
			}
		})
	}
}

func logServerInfo(id string, input any, result any) *server.Info {
	return &server.Info{
		DecisionID: id,
		Path:       "data.foo.bar",
		Input:      &input,
		Results:    &result,
		RemoteAddr: "test",
		Timestamp:  time.Now().UTC(),
	}
}

func TestPluginRequeueBufferPreserved(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	fixture := newTestFixture(t, testFixtureOptions{ReportingUploadSizeLimitBytes: 300})
	defer fixture.server.stop()

	fixture.server.ch = make(chan []EventV1, 3)

	var input any = map[string]any{"method": "GET"}
	var result1 any = false

	_ = fixture.plugin.Log(ctx, logServerInfo("abc", input, result1))
	_ = fixture.plugin.Log(ctx, logServerInfo("def", input, result1))
	_ = fixture.plugin.Log(ctx, logServerInfo("ghi", input, result1))

	bufLen := fixture.plugin.b.(*sizeBuffer).buffer.Len()
	if bufLen < 1 {
		t.Fatal("Expected buffer length of at least 1")
	}

	fixture.server.expCode = 500
	err := fixture.plugin.b.Upload(ctx)
	if err == nil {
		t.Fatal("Expected error")
	}

	<-fixture.server.ch

	if fixture.plugin.b.(*sizeBuffer).buffer.Len() < bufLen {
		t.Fatal("Expected buffer to be preserved")
	}
}

func TestPluginRateLimitInt(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		bufferType string
	}{
		{
			name:       "using event buffer",
			bufferType: eventBufferType,
		},
		{
			name: "using size buffer",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()

			ts, err := time.Parse(time.RFC3339Nano, "2018-01-01T12:00:00.123456Z")
			if err != nil {
				panic(err)
			}

			numDecisions := 1 // 1 decision per second

			fixture := newTestFixture(t, testFixtureOptions{
				ReportingMaxDecisionsPerSecond: float64(numDecisions),
				ReportingUploadSizeLimitBytes:  defaultUploadSizeLimitBytes,
				ReportingBufferType:            tc.bufferType,
			})
			defer fixture.server.stop()

			var input any = map[string]any{"method": "GET"}
			var result any = false

			eventSize := 217
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

			expectedLen := 1
			currentLen := getBufferLen(t, fixture, eventSize)
			if currentLen != expectedLen {
				t.Fatalf("Expected %v events to be written but got %v", expectedLen, currentLen)
			}

			_ = fixture.plugin.Log(ctx, event2) // event 2 should not be written into the encoder as rate limit exceeded

			currentLen = getBufferLen(t, fixture, eventSize)
			if currentLen != expectedLen {
				t.Fatalf("Expected %v events to be written but got %v", expectedLen, currentLen)
			}

			time.Sleep(1 * time.Second)
			_ = fixture.plugin.Log(ctx, event2) // event 2 should now be written into the encoder

			expectedLen = 2
			currentLen = getBufferLen(t, fixture, eventSize)
			if currentLen != expectedLen {
				t.Fatalf("Expected %v events to be written but got %v", expectedLen, currentLen)
			}

			expectedEvent1 := EventV1{
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
			expectedEvent2 := EventV1{
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

			var bufferEvent1, bufferEvent2 EventV1
			switch fixture.plugin.b.Name() {
			case sizeBufferType:
				chunk, err := fixture.plugin.b.(*sizeBuffer).enc.Flush()
				if err != nil {
					t.Fatal(err)
				}
				if len(chunk) != 1 {
					t.Fatalf("Expected 1 chunk but got %v", len(chunk))
				}
				events := decodeLogEvent(t, bytes.NewReader(chunk[0]))
				if len(events) != 2 {
					t.Fatalf("Expected 2 events but got %v", len(events))
				}
				bufferEvent1 = events[0]
				bufferEvent2 = events[1]
			case eventBufferType:
				bufferEvent1 = *(<-fixture.plugin.b.(*eventBuffer).buffer).EventV1
				bufferEvent1.Bundles = nil

				bufferEvent2 = *(<-fixture.plugin.b.(*eventBuffer).buffer).EventV1
				bufferEvent2.Bundles = nil

			}
			bufferEvent1.inputAST = nil
			if !reflect.DeepEqual(bufferEvent1, expectedEvent1) {
				t.Fatalf("Expected %+v but got %+v", expectedEvent1, event1)
			}
			bufferEvent2.inputAST = nil
			if !reflect.DeepEqual(bufferEvent2, expectedEvent2) {
				t.Fatalf("Expected %+v but got %+v", expectedEvent1, event1)
			}
		})
	}
}

// getBufferLen returns the buffer length for either the event or size buffer.
func getBufferLen(t *testing.T, fixture testFixture, eventSize int) int {
	switch fixture.plugin.b.Name() {
	case eventBufferType:
		return len(fixture.plugin.b.(*eventBuffer).buffer)
	case sizeBufferType:
		// events stay in the encoder until the upload limit is reached
		return fixture.plugin.b.(*sizeBuffer).enc.bytesWritten / eventSize
	default:
		t.Fatal("unknown buffer type")
		return 0
	}
}

func TestPluginRateLimitFloat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		bufferType string
	}{
		{
			name:       "using event buffer",
			bufferType: eventBufferType,
		},
		{
			name: "using size buffer",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()

			ts, err := time.Parse(time.RFC3339Nano, "2018-01-01T12:00:00.123456Z")
			if err != nil {
				panic(err)
			}

			numDecisions := 0.5 // 0.5 decision per second i.e. 1 decision per 2 seconds
			fixture := newTestFixture(t, testFixtureOptions{
				ReportingMaxDecisionsPerSecond: numDecisions,
				ReportingUploadSizeLimitBytes:  defaultUploadSizeLimitBytes,
				ReportingBufferType:            tc.bufferType,
			})
			defer fixture.server.stop()

			var input any = map[string]any{"method": "GET"}
			var result any = false

			eventSize := 217
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

			// event 1 should be written into the encoder
			if err := fixture.plugin.Log(ctx, event1); err != nil {
				t.Fatal(err)
			}

			expectedLen := 1
			currentLen := getBufferLen(t, fixture, eventSize)
			if currentLen != expectedLen {
				t.Fatalf("Expected %v events to be written but got %v", expectedLen, currentLen)
			}

			// event 2 should not be written into the encoder as rate limit exceeded
			if err := fixture.plugin.Log(ctx, event2); err != nil {
				t.Fatal(err)
			}

			currentLen = getBufferLen(t, fixture, eventSize)
			if currentLen != expectedLen {
				t.Fatalf("Expected %v events to be written but got %v", expectedLen, currentLen)
			}

			time.Sleep(1 * time.Second)
			// event 2 should not be written into the encoder as rate limit exceeded
			if err := fixture.plugin.Log(ctx, event2); err != nil {
				t.Fatal(err)
			}

			currentLen = getBufferLen(t, fixture, eventSize)
			if currentLen != expectedLen {
				t.Fatalf("Expected %v events to be written but got %v", expectedLen, currentLen)
			}

			time.Sleep(1 * time.Second)
			_ = fixture.plugin.Log(ctx, event2) // event 2 should now be written into the encoder

			expectedLen = 2
			currentLen = getBufferLen(t, fixture, eventSize)
			if currentLen != expectedLen {
				t.Fatalf("Expected %v events to be written but got %v", expectedLen, currentLen)
			}

			expectedEvent1 := EventV1{
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
			expectedEvent2 := EventV1{
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

			var bufferEvent1, bufferEvent2 EventV1
			switch fixture.plugin.b.Name() {
			case sizeBufferType:
				chunk, err := fixture.plugin.b.(*sizeBuffer).enc.Flush()
				if err != nil {
					t.Fatal(err)
				}
				if len(chunk) != 1 {
					t.Fatalf("Expected 1 chunk but got %v", len(chunk))
				}
				events := decodeLogEvent(t, bytes.NewReader(chunk[0]))
				if len(events) != 2 {
					t.Fatalf("Expected 2 events but got %v", len(events))
				}
				bufferEvent1 = events[0]
				bufferEvent2 = events[1]
			case eventBufferType:
				bufferEvent1 = *(<-fixture.plugin.b.(*eventBuffer).buffer).EventV1
				bufferEvent1.Bundles = nil

				bufferEvent2 = *(<-fixture.plugin.b.(*eventBuffer).buffer).EventV1
				bufferEvent2.Bundles = nil

			}
			bufferEvent1.inputAST = nil
			if !reflect.DeepEqual(bufferEvent1, expectedEvent1) {
				t.Fatalf("Expected %+v but got %+v", expectedEvent1, event1)
			}
			bufferEvent2.inputAST = nil
			if !reflect.DeepEqual(bufferEvent2, expectedEvent2) {
				t.Fatalf("Expected %+v but got %+v", expectedEvent1, event1)
			}
		})
	}
}

func TestPluginStatusUpdateHTTPError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		bufferType string
	}{
		{
			name:       "using event buffer",
			bufferType: eventBufferType,
		},
		{
			name: "using size buffer",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()

			fixture := newTestFixture(t, testFixtureOptions{
				ReportingUploadSizeLimitBytes: defaultUploadSizeLimitBytes,
				ReportingBufferType:           tc.bufferType,
			})
			defer fixture.server.stop()

			fixture.server.ch = make(chan []EventV1, 3)

			input := map[string]any{"method": "GET"}
			var result1 bool

			if err := fixture.plugin.Log(ctx, logServerInfo("abc", input, result1)); err != nil {
				t.Fatal(err)
			}
			if err := fixture.plugin.Log(ctx, logServerInfo("def", input, result1)); err != nil {
				t.Fatal(err)
			}
			if err := fixture.plugin.Log(ctx, logServerInfo("ghi", input, result1)); err != nil {
				t.Fatal(err)
			}

			eventSize := 218
			bufLen := getBufferLen(t, fixture, eventSize)
			if bufLen != 3 {
				t.Fatalf("Expected buffer length of 3 but got %v", bufLen)
			}

			fixture.server.expCode = 500
			err := fixture.plugin.doOneShot(ctx)
			if err == nil {
				t.Fatal("Expected error")
			}

			<-fixture.server.ch

			if fixture.plugin.status.HTTPCode != "500" {
				t.Fatal("expected http_code to be 500 instead of ", fixture.plugin.status.HTTPCode)
			}

			msg := "log upload failed, server replied with HTTP 500 Internal Server Error"
			if fixture.plugin.status.Message != msg {
				t.Fatalf("expected status message to be %v instead of %v", msg, fixture.plugin.status.Message)
			}
		})
	}
}

func TestPluginStatusUpdateEncodingFailure(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	testLogger := test.New()

	ts, err := time.Parse(time.RFC3339Nano, "2018-01-01T12:00:00.123456Z")
	if err != nil {
		panic(err)
	}

	fixture := newTestFixture(t, testFixtureOptions{
		ConsoleLogger:                 testLogger,
		ReportingUploadSizeLimitBytes: 1,
	})
	defer fixture.server.stop()

	m := metrics.New()
	fixture.plugin.metrics = m
	fixture.plugin.b.(*sizeBuffer).enc.metrics = m

	var input any = map[string]any{"method": "GET"}
	var result any = false

	event := &server.Info{
		DecisionID: "abc",
		Path:       "foo/bar",
		Input:      &input,
		Results:    &result,
		RemoteAddr: "test-1",
		Timestamp:  ts,
	}

	err = fixture.plugin.Log(ctx, event)
	if err != nil {
		t.Fatal(err)
	}

	fixture.plugin.b.(*sizeBuffer).mtx.Lock()
	if fixture.plugin.b.(*sizeBuffer).enc.bytesWritten != 0 {
		t.Fatal("Expected no event to be written into the encoder")
	}
	fixture.plugin.b.(*sizeBuffer).mtx.Unlock()

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

	// Trigger a status update
	fixture.server.expCode = 200
	err = fixture.plugin.doOneShot(ctx)
	if err != nil && !errors.Is(err, &bufferEmpty{}) {
		t.Fatal("Unexpected error")
	}

	// Give the logger / console some time to process and print the events
	time.Sleep(10 * time.Millisecond)
	p.Stop(ctx)

	entries := testLogger.Entries()
	if len(entries) == 0 {
		t.Fatal("Expected log entries but got none")
	}

	// Pick the last entry as it should have the decision log metrics
	e := entries[len(entries)-1]

	if _, ok := e.Fields["metrics"]; !ok {
		t.Fatal("Expected metrics field in status update")
	}

	fmt.Println(e.Fields["metrics"])

	exp := map[string]any{"<built-in>": map[string]any{"counter_decision_logs_encoding_failure": json.Number("1"),
		"counter_enc_log_exceeded_upload_size_limit_bytes": json.Number("1")}}

	if !reflect.DeepEqual(e.Fields["metrics"], exp) {
		t.Fatalf("Expected %v but got %v", exp, e.Fields["metrics"])
	}
}

func TestPluginStatusUpdateBufferSizeExceeded(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	testLogger := test.New()

	ts, err := time.Parse(time.RFC3339Nano, "2018-01-01T12:00:00.123456Z")
	if err != nil {
		panic(err)
	}

	fixture := newTestFixture(t, testFixtureOptions{
		ConsoleLogger:                 testLogger,
		ReportingBufferSizeLimitBytes: 200,
		ReportingUploadSizeLimitBytes: 300,
	})
	defer fixture.server.stop()

	fixture.server.ch = make(chan []EventV1, 1)

	fixture.plugin = fixture.plugin.WithMetrics(metrics.New())

	var input any = map[string]any{"method": "GET"}
	var result any = false

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

	// write event 1 and 2 into the encoder and check the chunk is inserted into the buffer
	if err := fixture.plugin.Log(ctx, event1); err != nil {
		t.Error(err)
	}

	if err := fixture.plugin.Log(ctx, event2); err != nil {
		t.Error(err)
	}
	fixture.plugin.b.(*sizeBuffer).mtx.Lock()

	if fixture.plugin.b.(*sizeBuffer).enc.bytesWritten == 0 {
		t.Fatal("Expected event to be written into the encoder")
	}

	if fixture.plugin.b.(*sizeBuffer).buffer.Len() == 0 {
		t.Fatal("Expected one chunk to be written into the buffer")
	}
	fixture.plugin.b.(*sizeBuffer).mtx.Unlock()

	// write event 3 into the encoder and then flush the encoder which will result in the event being
	// written to the buffer. But given the buffer size it won't be able to hold this event and will
	// drop the existing chunk
	_ = fixture.plugin.Log(ctx, event3)

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

	// Pick the last entry as it should have the decision log metrics
	e := entries[len(entries)-1]

	if _, ok := e.Fields["metrics"]; !ok {
		t.Fatal("Expected metrics field in status update")
	}

	if e.Fields["metrics"].(map[string]any)["<built-in>"].(map[string]any)["counter_decision_logs_dropped_buffer_size_limit_bytes_exceeded"] != json.Number("1") {
		t.Fatal("Expected metrics field in status update")
	}
	if e.Fields["metrics"].(map[string]any)["<built-in>"].(map[string]any)["counter_decision_logs_dropped_buffer_size_limit_exceeded"] != json.Number("1") {
		t.Fatal("Expected metrics field in status update")
	}
}

func TestPluginStatusUpdateRateLimitExceeded(t *testing.T) {
	t.Parallel()

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

	fixture.plugin = fixture.plugin.WithMetrics(metrics.New())

	var input any = map[string]any{"method": "GET"}
	var result any = false

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

	fixture.plugin.b.(*sizeBuffer).mtx.Lock()
	if fixture.plugin.b.(*sizeBuffer).enc.bytesWritten == 0 {
		t.Fatal("Expected event to be written into the encoder")
	}
	fixture.plugin.b.(*sizeBuffer).mtx.Unlock()

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

	// Pick the last entry as it should have the decision log metrics
	e := entries[len(entries)-1]

	if _, ok := e.Fields["metrics"]; !ok {
		t.Fatal("Expected metrics field in status update")
	}

	exp := map[string]any{"<built-in>": map[string]any{"counter_decision_logs_dropped_rate_limit_exceeded": json.Number("2")}}

	if !reflect.DeepEqual(e.Fields["metrics"], exp) {
		t.Fatalf("Expected %v but got %v", exp, e.Fields["metrics"])
	}
}

func TestPluginRateLimitRequeue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		bufferType string
	}{
		{
			name:       "using event buffer",
			bufferType: eventBufferType,
		},
		{
			name: "using size buffer",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()

			numDecisions := 100 // 100 decisions per second
			fixture := newTestFixture(t, testFixtureOptions{
				ReportingMaxDecisionsPerSecond: float64(numDecisions),
				ReportingUploadSizeLimitBytes:  defaultUploadSizeLimitBytes,
				ReportingBufferType:            tc.bufferType,
			})
			defer fixture.server.stop()

			fixture.server.ch = make(chan []EventV1, 3)

			var input any = map[string]any{"method": "GET"}
			var result1 any = false

			if err := fixture.plugin.Log(ctx, logServerInfo("abc", input, result1)); err != nil {
				t.Fatal(err)
			}
			if err := fixture.plugin.Log(ctx, logServerInfo("def", input, result1)); err != nil {
				t.Fatal(err)
			}
			if err := fixture.plugin.Log(ctx, logServerInfo("ghi", input, result1)); err != nil {
				t.Fatal(err)
			}

			eventSize := 218
			bufLen := getBufferLen(t, fixture, eventSize)
			if bufLen != 3 {
				t.Fatal("Expected buffer length of 3 but got ", bufLen)
			}

			fixture.server.expCode = 500
			err := fixture.plugin.b.Upload(ctx)
			if err == nil {
				t.Fatal("Expected error")
			}
			<-fixture.server.ch

			var event1, event2, event3 EventV1
			switch fixture.plugin.b.Name() {
			case eventBufferType:
				// buffer will put a single event with the failed uploaded chunk back in the buffer
				bufLen = getBufferLen(t, fixture, eventSize)
				if bufLen != 1 {
					t.Fatal("Expected buffer length of 3 but got ", bufLen)
				}

				chunk := (<-fixture.plugin.b.(*eventBuffer).buffer).chunk

				events, err := newChunkDecoder(chunk).decode()
				if err != nil {
					t.Fatal(err)
				}
				event1 = events[0]
				event2 = events[1]
				event3 = events[2]

			case sizeBufferType:
				// size buffer will put individual events with the failed uploaded chunk back in the buffer
				bufLen = getBufferLen(t, fixture, eventSize)
				if bufLen != 3 {
					t.Fatal("Expected buffer length of 3 but got ", bufLen)
				}

				chunks, err := fixture.plugin.b.(*sizeBuffer).enc.Flush()
				if err != nil {
					t.Fatal(err)
				}
				if len(chunks) != 1 {
					t.Fatalf("Expected 1 chunk but got %v", len(chunks))
				}
				events := decodeLogEvent(t, bytes.NewReader(chunks[0]))
				event1 = events[0]
				event2 = events[1]
				event3 = events[2]
			}

			exp := "abc"
			if event1.DecisionID != exp {
				t.Fatalf("Expected decision log event id %v but got %v", exp, event1.DecisionID)
			}

			exp = "def"
			if event2.DecisionID != exp {
				t.Fatalf("Expected decision log event id %v but got %v", exp, event2.DecisionID)
			}

			exp = "ghi"
			if event3.DecisionID != exp {
				t.Fatalf("Expected decision log event id %v but got %v", exp, event3.DecisionID)
			}

		})
	}
}

func TestPluginRateLimitDropCountStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		bufferType string
	}{
		{
			name:       "using event buffer",
			bufferType: eventBufferType,
		},
		{
			name: "using size buffer",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			testLogger := test.New()

			ts, err := time.Parse(time.RFC3339Nano, "2018-01-01T12:00:00.123456Z")
			if err != nil {
				t.Fatal(err)
			}

			numDecisions := 1 // 1 decision per second
			fixture := newTestFixture(t, testFixtureOptions{
				ConsoleLogger:                  testLogger,
				ReportingMaxDecisionsPerSecond: float64(numDecisions),
				ReportingUploadSizeLimitBytes:  300,
				ReportingBufferType:            tc.bufferType,
			})
			defer fixture.server.stop()

			fixture.plugin = fixture.plugin.WithMetrics(metrics.New())

			var input any = map[string]any{"method": "GET"}
			var result any = false

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

			if err := fixture.plugin.Log(ctx, event1); err != nil {
				t.Fatal(err)
			}

			eventSize := 217
			expectedLen := 1
			currentLen := getBufferLen(t, fixture, eventSize)
			if currentLen != expectedLen {
				t.Fatalf("Expected %v events to be written but got %v", expectedLen, currentLen)
			}

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

			// event 2 should not be written into the encoder as rate limit exceeded
			if err := fixture.plugin.Log(ctx, event2); err != nil {
				t.Fatal(err)
			}
			// event 3 should not be written into the encoder as rate limit exceeded
			if err := fixture.plugin.Log(ctx, event3); err != nil {
				t.Fatal(err)
			}

			// Trigger a status update
			p.UpdateDiscoveryStatus(*testStatus())

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

			exp := map[string]any{"<built-in>": map[string]any{"counter_decision_logs_dropped_rate_limit_exceeded": json.Number("2")}}

			if !reflect.DeepEqual(e.Fields["metrics"], exp) {
				t.Fatalf("Expected %v but got %v", exp, e.Fields["metrics"])
			}
		})
	}
}

func TestPluginRateLimitBadConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		config         []byte
		expectedErrMsg string
	}{
		{
			name: "invalid buffer_size_limit_bytes & max_decisions_per_second",
			config: []byte(`{
				"console": true,
				"reporting": {
					"buffer_size_limit_bytes": 1,
					"max_decisions_per_second": 1
				}
			}`),
			expectedErrMsg: "invalid decision_log config, specify either 'buffer_size_limit_bytes' or 'max_decisions_per_second'",
		},
		{
			name: "invalid buffer_size_limit_events used with size buffer",
			config: []byte(`{
				"console": true,
				"reporting": {
					"buffer_size_limit_events": 1
				}
			}`),
			expectedErrMsg: "invalid decision_log config, 'buffer_size_limit_events' isn't supported for the size buffer type",
		},
		{
			name: "invalid buffer_size_limit_bytes used with event buffer",
			config: []byte(`{
				"console": true,
				"reporting": {
					"buffer_type": "event",
					"buffer_size_limit_bytes": 1
				}
			}`),
			expectedErrMsg: "invalid decision_log config, 'buffer_size_limit_bytes' isn't supported for the event buffer type",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {

			manager, _ := plugins.New(nil, "test-instance-id", inmem.New())

			_, err := ParseConfig(tc.config, manager.Services(), nil)
			if err == nil {
				t.Fatal("Expected error but got nil")
			}

			if err.Error() != tc.expectedErrMsg {
				t.Fatalf("Expected error message %v but got %v", tc.expectedErrMsg, err.Error())
			}
		})
	}
}

func TestPluginNoLogging(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

	tests := []struct {
		name                           string
		reportingBufferType            string
		reportingBufferSizeLimitEvents int64
	}{
		{
			name:                "using event buffer",
			reportingBufferType: eventBufferType,
		},
		{
			name: "using size buffer",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()

			fixture := newTestFixture(t, testFixtureOptions{
				ReportingBufferType:            tc.reportingBufferType,
				ReportingBufferSizeLimitEvents: tc.reportingBufferSizeLimitEvents,
			})
			defer fixture.server.stop()

			fixture.server.server.Config.SetKeepAlivesEnabled(false)

			fixture.server.ch = make(chan []EventV1, 4)
			tr := plugins.TriggerManual
			fixture.plugin.config.Reporting.Trigger = &tr

			if err := fixture.plugin.Start(ctx); err != nil {
				t.Fatal(err)
			}

			testMetrics := getWellKnownMetrics()
			msAsFloat64 := map[string]any{}
			for k, v := range testMetrics.All() {
				msAsFloat64[k] = float64(v.(uint64))
			}

			var input any = map[string]any{"method": "GET"}
			var result any = false

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

			for i := range 400 {
				if err := fixture.plugin.Log(ctx, &server.Info{
					Revision:   strconv.Itoa(i),
					DecisionID: strconv.Itoa(i),
					Path:       "tda/bar",
					Input:      &input,
					Results:    &result,
					RemoteAddr: "test",
					Timestamp:  ts,
					Metrics:    testMetrics,
				}); err != nil {
					t.Fatal(err)
				}

				// trigger the decision log upload
				go func(i int) {
					fixture.plugin.Trigger(ctx)
				}(i)

				fmt.Println("waiting")
				chunk := <-fixture.server.ch

				expLen := 1
				if len(chunk) != 1 {
					t.Fatalf("Expected chunk len %v but got: %v", expLen, len(chunk))
				}

				exp.Revision = strconv.Itoa(i)
				exp.DecisionID = strconv.Itoa(i)

				if !reflect.DeepEqual(chunk[0], exp) {
					t.Fatalf("Expected %+v but got %+v", exp, chunk[0])
				}
			}

			fixture.plugin.Stop(ctx)
		})
	}
}

func TestPluginTriggerManualWithTimeout(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(3 * time.Second) // this should cause the context deadline to exceed
	}))

	// setup plugin pointing at fake server
	managerConfig := fmt.Appendf(nil, `{
			"labels": {
				"app": "example-app"
			},
			"services": [
				{
					"name": "example",
					"url": %q
				}
			]}`, s.URL)

	manager, err := plugins.New(
		managerConfig,
		"test-instance-id",
		inmem.New(),
		plugins.GracefulShutdownPeriod(10))
	if err != nil {
		t.Fatal(err)
	}

	pluginConfig := make(map[string]any)

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

	var input any = map[string]any{"method": "GET"}
	var result any = false

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
	tests := []struct {
		name       string
		mode       plugins.TriggerMode
		bufferType string
	}{
		{
			name:       "immediate mode, event buffer",
			bufferType: eventBufferType,
			mode:       plugins.TriggerImmediate,
		},
		{
			name:       "immediate mode, size buffer",
			bufferType: "size",
			mode:       plugins.TriggerImmediate,
		},
		{
			name:       "periodic mode, event buffer",
			bufferType: eventBufferType,
			mode:       plugins.TriggerPeriodic,
		},
		{
			name:       "periodic mode, size buffer",
			bufferType: "size",
			mode:       plugins.TriggerPeriodic,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()

			fixture := newTestFixture(t, testFixtureOptions{
				TriggerMode:         tc.mode,
				ReportingBufferType: tc.bufferType,
			})
			defer fixture.server.stop()

			fixture.server.ch = make(chan []EventV1, 8)

			if err := fixture.plugin.Start(ctx); err != nil {
				t.Fatal(err)
			}

			var input any
			var result any = false

			logsSent := 200
			for i := range logsSent {
				input = generateInputMap(i)
				_ = fixture.plugin.Log(ctx, logServerInfo("abc", input, result))
			}

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
		})
	}
}

func TestPluginTerminatesAfterGracefulShutdownPeriod(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	fixture := newTestFixture(t)
	defer fixture.server.stop()

	fixture.server.ch = make(chan []EventV1, 1)
	fixture.server.expCode = 500

	if err := fixture.plugin.Start(ctx); err != nil {
		t.Fatal(err)
	}

	var result any = false
	var input any = generateInputMap(0)
	_ = fixture.plugin.Log(ctx, logServerInfo("abc", input, result))

	timeoutCtx, cancel := context.WithTimeout(ctx, 1*time.Nanosecond)
	defer cancel()

	fixture.plugin.Stop(timeoutCtx)

	// Ensure the plugin was stopped without flushing its whole buffer
	if fixture.plugin.b.(*sizeBuffer).buffer.Len() == 0 && fixture.plugin.b.(*sizeBuffer).enc.buf.Len() == 0 {
		t.Errorf("Expected the plugin to still have buffered messages")
	}
}

func TestPluginTerminatesAfterGracefulShutdownPeriodWithoutLogs(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

	tests := []struct {
		name              string
		currentBufferType string
		newBufferType     string
		limitEvents       int64
		limitBytes        int64
	}{
		{
			name:              "Reconfigure from event to size buffer",
			currentBufferType: eventBufferType,
			newBufferType:     sizeBufferType,
		},
		{
			name:              "Reconfigure from size to event buffer",
			currentBufferType: sizeBufferType,
			newBufferType:     eventBufferType,
		},
		{
			name:              "Reconfigure from size to size buffer",
			currentBufferType: "size",
			newBufferType:     "size",
			limitBytes:        100,
		},
		{
			name:              "Reconfigure from event to event buffer",
			currentBufferType: eventBufferType,
			newBufferType:     eventBufferType,
			limitEvents:       200,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			fixture := newTestFixture(t, testFixtureOptions{
				ReportingBufferType: tc.currentBufferType,
			})
			defer fixture.server.stop()

			if err := fixture.plugin.Start(ctx); err != nil {
				t.Fatal(err)
			}

			ensurePluginState(t, fixture.plugin, plugins.StateOK)

			var config Config
			resource := ""
			config.Resource = &resource
			// defaults to periodic, so this will always change to something new
			trigger := plugins.TriggerImmediate
			config.Reporting.Trigger = &trigger
			config.Reporting.BufferType = tc.newBufferType
			config.Reporting.BufferSizeLimitBytes = &tc.limitBytes
			config.Reporting.BufferSizeLimitEvents = &tc.limitEvents

			minDelay := int64(2)
			maxDelay := int64(3)
			config.Reporting.MinDelaySeconds = &minDelay
			config.Reporting.MaxDelaySeconds = &maxDelay

			uploadLimit := int64(100)
			config.Reporting.UploadSizeLimitBytes = &uploadLimit

			fixture.plugin.Reconfigure(ctx, &config)
			ensurePluginState(t, fixture.plugin, plugins.StateOK)

			fixture.plugin.Stop(ctx)
			ensurePluginState(t, fixture.plugin, plugins.StateNotReady)

			if *fixture.plugin.config.Reporting.MinDelaySeconds != minDelay {
				t.Fatalf("Expected minimum polling interval: %v but got %v", minDelay, *fixture.plugin.config.Reporting.MinDelaySeconds)
			}

			if *fixture.plugin.config.Reporting.MaxDelaySeconds != maxDelay {
				t.Fatalf("Expected maximum polling interval: %v but got %v", maxDelay, *fixture.plugin.config.Reporting.MaxDelaySeconds)
			}

			if *fixture.plugin.config.Reporting.BufferSizeLimitEvents != tc.limitEvents {
				t.Fatalf("Expected limit events %v, but got %v", tc.limitEvents, *fixture.plugin.config.Reporting.BufferSizeLimitEvents)
			}

			if *fixture.plugin.config.Reporting.BufferSizeLimitBytes != tc.limitBytes {
				t.Fatalf("Expected limit bytes %v, but got %v", tc.limitBytes, *fixture.plugin.config.Reporting.BufferSizeLimitBytes)
			}

			if *fixture.plugin.config.Reporting.UploadSizeLimitBytes != uploadLimit {
				t.Fatalf("Expected upload limit %v, but got %v", uploadLimit, *fixture.plugin.config.Reporting.UploadSizeLimitBytes)
			}

			if *fixture.plugin.config.Reporting.Trigger != trigger {
				t.Fatalf("Expected trigger mode %v, but got %v", trigger, *fixture.plugin.config.Reporting.Trigger)
			}
		})
	}
}

func TestPluginReconfigureUploadSizeLimit(t *testing.T) {
	t.Parallel()

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

	fixture.plugin.b.(*sizeBuffer).mtx.Lock()
	if fixture.plugin.b.(*sizeBuffer).enc.limit != limit {
		t.Fatalf("Expected upload size limit %v but got %v", limit, fixture.plugin.b.(*sizeBuffer).enc.limit)
	}
	fixture.plugin.b.(*sizeBuffer).mtx.Unlock()

	newLimit := int64(600)

	pluginConfig := fmt.Appendf(nil, `{
			"service": "example",
			"reporting": {
				"upload_size_limit_bytes": %v,
			}
		}`, newLimit)

	config, _ := ParseConfig(pluginConfig, fixture.manager.Services(), nil)

	fixture.plugin.Reconfigure(ctx, config)
	ensurePluginState(t, fixture.plugin, plugins.StateOK)

	fixture.plugin.Stop(ctx)
	ensurePluginState(t, fixture.plugin, plugins.StateNotReady)

	fixture.plugin.b.(*sizeBuffer).mtx.Lock()
	if fixture.plugin.b.(*sizeBuffer).enc.limit != newLimit {
		t.Fatalf("Expected upload size limit %v but got %v", newLimit, fixture.plugin.b.(*sizeBuffer).enc.limit)
	}
	fixture.plugin.b.(*sizeBuffer).mtx.Unlock()
}

type appendingPrintHook struct {
	printed *[]string
}

func (a appendingPrintHook) Print(_ print.Context, s string) error {
	*a.printed = append(*a.printed, s)
	return nil
}

func TestPluginMasking(t *testing.T) {
	t.Parallel()

	tests := []struct {
		note          string
		rawPolicy     []byte
		expErased     []string
		expMasked     []string
		expPrinted    []string
		errManager    error
		expErr        error
		input         any
		expected      any
		ndbcache      any
		ndbc_expected any
		reconfigure   bool
	}{
		{
			note: "simple erase (with body true)",
			rawPolicy: []byte(`
				package system.log
				import rego.v1
				mask contains "/input/password" if {
					input.input.is_sensitive
				}`),
			expErased: []string{"/input/password"},
			input: map[string]any{
				"is_sensitive": true,
				"password":     "secret",
			},
			expected: map[string]any{
				"is_sensitive": true,
			},
		},
		{
			note: "simple erase (with body true, plugin reconfigured)",
			rawPolicy: []byte(`
				package system.log
				import rego.v1
				mask contains "/input/password" if {
					input.input.is_sensitive
				}`),
			expErased: []string{"/input/password"},
			input: map[string]any{
				"is_sensitive": true,
				"password":     "secret",
			},
			expected: map[string]any{
				"is_sensitive": true,
			},
			reconfigure: true,
		},
		{
			note: "simple upsert (with body true)",
			rawPolicy: []byte(`
				package system.log
				import rego.v1
				mask contains {"op": "upsert", "path": "/input/password", "value": x} if {
					input.input.password
					x := "**REDACTED**"
				}`),
			expMasked: []string{"/input/password"},
			input: map[string]any{
				"is_sensitive": true,
				"password":     "mySecretPassword",
			},
			expected: map[string]any{
				"is_sensitive": true,
				"password":     "**REDACTED**",
			},
		},
		{
			note: "remove even with value set in rule body",
			rawPolicy: []byte(`
				package system.log
				import rego.v1
				mask contains {"op": "remove", "path": "/input/password", "value": x} if {
					input.input.password
					x := "**REDACTED**"
				}`),
			expErased: []string{"/input/password"},
			input: map[string]any{
				"is_sensitive": true,
				"password":     "mySecretPassword",
			},
			expected: map[string]any{
				"is_sensitive": true,
			},
		},
		{
			note: "remove when value not defined",
			rawPolicy: []byte(`
				package system.log
				import rego.v1
				mask contains {"op": "remove", "path": "/input/password"} if {
					input.input.password
				}`),
			expErased: []string{"/input/password"},
			input: map[string]any{
				"is_sensitive": true,
				"password":     "mySecretPassword",
			},
			expected: map[string]any{
				"is_sensitive": true,
			},
		},
		{
			note: "remove when value not defined in rule body",
			rawPolicy: []byte(`
				package system.log
				import rego.v1
				mask contains {"op": "remove", "path": "/input/password", "value": x} if {
					input.input.password
				}`),
			errManager: errors.New("1 error occurred: test.rego:4: rego_unsafe_var_error: var x is unsafe"),
		},
		{
			note: "simple erase - no match",
			rawPolicy: []byte(`
				package system.log
				mask["/input/password"] {
					input.input.is_sensitive
				}`),
			input: map[string]any{
				"is_not_sensitive": true,
				"password":         "secret",
			},
			expected: map[string]any{
				"is_not_sensitive": true,
				"password":         "secret",
			},
		},
		{
			note: "complex upsert - object key",
			rawPolicy: []byte(`
				package system.log
				import rego.v1
				mask contains {"op": "upsert", "path": "/input/foo", "value": x} if {
					input.input.foo
					x := [
						{"nabs": 1}
					]
				}`),
			input: map[string]any{
				"bar": 1,
				"foo": []map[string]any{{"baz": 1}},
			},
			// Due to ast.JSON() parsing as part of rego.eval, internal mapped
			// types from mask rule valuations (for numbers) will be json.Number.
			// This affects explicitly providing the expected any value.
			//
			// See TestMaksRuleErase where tests are written to confirm json marshalled
			// output is as expected.
			expected: map[string]any{
				"bar": 1,
				"foo": []any{map[string]any{"nabs": json.Number("1")}},
			},
		},
		{
			note: "upsert failure: unsupported type []map[string]any",
			rawPolicy: []byte(`
				package system.log
				mask[{"op": "upsert", "path": "/input/foo/boo", "value": x}] {
					x := [
						{"nabs": 1}
					]
				}`),
			input: map[string]any{
				"bar": json.Number("1"),
				"foo": []map[string]any{{"baz": json.Number("1")}},
			},
			expected: map[string]any{
				"bar": json.Number("1"),
				"foo": []map[string]any{{"baz": json.Number("1")}},
			},
		},
		{
			note: "mixed mode - complex #1",
			rawPolicy: []byte(`
				package system.log

				import rego.v1

				mask contains "/input/password" if {
					input.input.is_sensitive
				}

				# invalidate JWT signature
				mask contains {"op": "upsert", "path": "/input/jwt", "value": x} if {
					input.input.jwt

					# split jwt string
					parts := split(input.input.jwt, ".")

					# make sure we have 3 parts
					count(parts) == 3

					# replace signature
					new := array.concat(array.slice(parts, 0, 2), [base64url.encode("**REDACTED**")])
					x = concat(".", new)

				}

				mask contains {"op": "upsert", "path": "/input/foo", "value": x} if {
					input.input.foo
					x := [
						{"changed": 1}
					]
				}`),
			input: map[string]any{
				"is_sensitive": true,
				"jwt":          "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.cThIIoDvwdueQB468K5xDc5633seEFoqwxjF_xSJyQQ",
				"bar":          1,
				"foo":          []map[string]any{{"baz": 1}},
				"password":     "mySecretPassword",
			},
			expected: map[string]any{
				"is_sensitive": true,
				"jwt":          "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.KipSRURBQ1RFRCoq",
				"bar":          1,
				"foo":          []any{map[string]any{"changed": json.Number("1")}},
			},
		},
		{
			note: "print() works",
			rawPolicy: []byte(`
				package system.log
				import rego.v1
				mask contains "/input/password" if {
					print("Erasing /input/password")
					input.input.is_sensitive
				}`),
			expErased: []string{"/input/password"},
			input: map[string]any{
				"is_sensitive": true,
				"password":     "secret",
			},
			expected: map[string]any{
				"is_sensitive": true,
			},
			expPrinted: []string{"Erasing /input/password"},
		},
		{
			note: "simple upsert on nd_builtin_cache",
			rawPolicy: []byte(`
				package system.log
				import rego.v1
				mask contains {"op": "upsert", "path": "/nd_builtin_cache/rand.intn", "value": x} if {
					input.nd_builtin_cache["rand.intn"]
					x := "**REDACTED**"
				}`),
			expMasked: []string{"/nd_builtin_cache/rand.intn"},
			ndbcache: map[string]any{
				// Simulate rand.intn("z", 15) call, with output of 7.
				"rand.intn": map[string]any{"[\"z\",15]": json.Number("7")},
			},
			ndbc_expected: map[string]any{
				"rand.intn": "**REDACTED**",
			},
		},
		{
			note: "simple upsert on nd_builtin_cache with multiple entries",
			rawPolicy: []byte(`
				package system.log
				import rego.v1
				mask contains {"op": "upsert", "path": "/nd_builtin_cache/rand.intn", "value": x} if {
					input.nd_builtin_cache["rand.intn"]
					x := "**REDACTED**"
				}

				mask contains {"op": "upsert", "path": "/nd_builtin_cache/net.lookup_ip_addr", "value": y} if {
					obj := input.nd_builtin_cache["net.lookup_ip_addr"]
					y := object.union({k: "4.4.x.x" | obj[k]; startswith(k, "[\"4.4.")},
					                  {k: obj[k] | obj[k]; not startswith(k, "[\"4.4.")})
				}
				`),
			expMasked: []string{"/nd_builtin_cache/net.lookup_ip_addr", "/nd_builtin_cache/rand.intn"},
			ndbcache: map[string]any{
				// Simulate rand.intn("z", 15) call, with output of 7.
				"rand.intn": map[string]any{"[\"z\",15]": json.Number("7")},
				"net.lookup_ip_addr": map[string]any{
					"[\"1.1.1.1\"]": "1.1.1.1",
					"[\"2.2.2.2\"]": "2.2.2.2",
					"[\"3.3.3.3\"]": "3.3.3.3",
					"[\"4.4.4.4\"]": "4.4.4.4",
				},
			},
			ndbc_expected: map[string]any{
				"rand.intn": "**REDACTED**",
				"net.lookup_ip_addr": map[string]any{
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
			if err := cfg.validateAndInjectDefaults([]string{"svc"}, nil, &trigger, nil); err != nil {
				t.Fatal(err)
			}

			plugin := New(cfg, manager)

			if err := plugin.Start(ctx); err != nil {
				t.Fatal(err)
			}

			event := &EventV1{
				Input:          &tc.input,
				NDBuiltinCache: &tc.ndbcache,
			}
			input, err := event.AST()
			if err != nil {
				t.Fatal(err)
			}

			if err := plugin.maskEvent(ctx, nil, input, event); err != nil {
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
				if err := newConfig.validateAndInjectDefaults([]string{"svc"}, nil, &trigger, nil); err != nil {
					t.Fatal(err)
				}

				plugin.Reconfigure(ctx, newConfig)

				event = &EventV1{
					Input: &tc.input,
				}
				input, err := event.AST()
				if err != nil {
					t.Fatal(err)
				}

				if err := plugin.maskEvent(ctx, nil, input, event); err != nil {
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
	t.Parallel()

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
			import rego.v1
			drop if {
				endswith(input.path, "bar")
			}`),
			event: &EventV1{Path: "foo/bar"},

			expected: true,
		},
		{
			note: "no drop",
			rawPolicy: []byte(`
			package system.log
			import rego.v1
			drop if {
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
			if err := cfg.validateAndInjectDefaults([]string{"svc"}, nil, &trigger, nil); err != nil {
				t.Fatal(err)
			}

			plugin := New(cfg, manager)

			if err := plugin.Start(ctx); err != nil {
				t.Fatal(err)
			}
			input, err := tc.event.AST()
			if err != nil {
				t.Fatal(err)
			}

			drop, err := plugin.dropEvent(ctx, nil, input)
			if err != nil {
				t.Fatal(err)
			}

			if tc.expected != drop {
				t.Errorf("Plugin: Expected drop to be %v got %v", tc.expected, drop)
			}
		})
	}
}

func TestPluginMaskErrorHandling(t *testing.T) {
	t.Parallel()

	rawPolicy := []byte(`
			package system.log
			import rego.v1
			drop if {
				endswith(input.path, "bar")
			}`)
	event := &EventV1{Path: "foo/bar"}

	// Setup fixture. Populate store with simple drop policy.
	ctx := context.Background()
	store := inmem.New()

	// checks if raw policy is valid and stores policy in store
	err := storage.Txn(ctx, store, storage.WriteParams, func(txn storage.Transaction) error {
		if err := store.UpsertPolicy(ctx, txn, "test.rego", rawPolicy); err != nil {
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
	if err := cfg.validateAndInjectDefaults([]string{"svc"}, nil, &trigger, nil); err != nil {
		t.Fatal(err)
	}

	plugin := New(cfg, manager)

	if err := plugin.Start(ctx); err != nil {
		t.Fatal(err)
	}
	input, err := event.AST()
	if err != nil {
		t.Fatal(err)
	}

	type badTransaction struct {
		storage.Transaction
	}

	expErr := "storage_invalid_txn_error: unexpected transaction type *logs.badTransaction"
	err = plugin.maskEvent(ctx, &badTransaction{}, input, event)
	if err.Error() != expErr {
		t.Fatalf("Expected error %v got %v", expErr, err)
	}

	// We expect the same error on a second call, even though the mask query failed to prepare and won't be prepared again.
	err = plugin.maskEvent(ctx, nil, input, event)
	if err.Error() != expErr {
		t.Fatalf("Expected error %v got %v", expErr, err)
	}
}

func TestPluginDropErrorHandling(t *testing.T) {
	t.Parallel()

	rawPolicy := []byte(`
			package system.log
			import rego.v1
			drop if {
				endswith(input.path, "bar")
			}`)
	event := &EventV1{Path: "foo/bar"}

	// Setup fixture. Populate store with simple drop policy.
	ctx := context.Background()
	store := inmem.New()

	//checks if raw policy is valid and stores policy in store
	err := storage.Txn(ctx, store, storage.WriteParams, func(txn storage.Transaction) error {
		if err := store.UpsertPolicy(ctx, txn, "test.rego", rawPolicy); err != nil {
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
	if err := cfg.validateAndInjectDefaults([]string{"svc"}, nil, &trigger, nil); err != nil {
		t.Fatal(err)
	}

	plugin := New(cfg, manager)

	if err := plugin.Start(ctx); err != nil {
		t.Fatal(err)
	}
	input, err := event.AST()
	if err != nil {
		t.Fatal(err)
	}

	type badTransaction struct {
		storage.Transaction
	}

	expErr := "storage_invalid_txn_error: unexpected transaction type *logs.badTransaction"
	_, err = plugin.dropEvent(ctx, &badTransaction{}, input)
	if err.Error() != expErr {
		t.Fatalf("Expected error %v got %v", expErr, err)
	}

	// We expect the same error on a second call, even though the drop query failed to prepare and won't be prepared again.
	_, err = plugin.dropEvent(ctx, nil, input)
	if err.Error() != expErr {
		t.Fatalf("Expected error %v got %v", expErr, err)
	}
}

type testFixtureOptions struct {
	ConsoleLogger                  *test.Logger
	ReportingBufferType            string
	ReportingBufferSizeLimitEvents int64
	ReportingUploadSizeLimitBytes  int64
	ReportingMaxDecisionsPerSecond float64
	ReportingBufferSizeLimitBytes  int64
	Resource                       *string
	TestServerPath                 *string
	PartitionName                  *string
	ExtraConfig                    map[string]any
	ExtraManagerConfig             map[string]any
	ManagerInit                    func(*plugins.Manager)
	TriggerMode                    plugins.TriggerMode
	MinDelay                       int64
	MaxDelay                       int64
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

	managerConfig := fmt.Appendf(nil, `{
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
			]}`, ts.server.URL)

	mgrCfg := make(map[string]any)
	err := json.Unmarshal(managerConfig, &mgrCfg)
	if err != nil {
		t.Fatal(err)
	}
	maps.Copy(mgrCfg, options.ExtraManagerConfig)
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

	pluginConfig := map[string]any{
		"service": "example",
	}

	if options.Resource != nil {
		pluginConfig["resource"] = *options.Resource
	}

	if options.PartitionName != nil {
		pluginConfig["partition_name"] = *options.PartitionName
	}

	maps.Copy(pluginConfig, options.ExtraConfig)

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

	ts.uploadLimit = *config.Reporting.UploadSizeLimitBytes

	if options.ReportingBufferSizeLimitBytes != 0 {
		config.Reporting.BufferSizeLimitBytes = &options.ReportingBufferSizeLimitBytes
	}

	if options.ReportingBufferSizeLimitEvents != 0 {
		config.Reporting.BufferSizeLimitEvents = &options.ReportingBufferSizeLimitEvents
	}

	if options.ReportingBufferType != "" {
		config.Reporting.BufferType = options.ReportingBufferType
	}

	if options.TriggerMode != "" {
		config.Reporting.Trigger = &options.TriggerMode
	}

	if options.MinDelay != 0 {
		minSeconds := int64(time.Duration(options.MinDelay) * time.Second)
		config.Reporting.MinDelaySeconds = &minSeconds
	}
	if options.MinDelay != 0 {
		maxSeconds := int64(time.Duration(options.MaxDelay) * time.Second)
		config.Reporting.MaxDelaySeconds = &maxSeconds
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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
			err:      errors.New("invalid decision_log config: trigger mode mismatch: periodic and manual (hint: check discovery configuration)"),
		},
		{
			note:     "bad trigger mode",
			config:   []byte(`{"reporting": {"trigger": "foo"}}`),
			expected: "foo",
			wantErr:  true,
			err:      errors.New("invalid decision_log config: invalid trigger mode \"foo\" (want \"periodic\", \"manual\" or \"immediate\")"),
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
	t.Parallel()

	input := `{"foo": [{"bar": 1, "baz": {"2": 3.3333333, "4": null}}]}`
	var goInput any = string(util.MustMarshalJSON(input))
	astInput, err := roundtripJSONToAST(goInput)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	var result any = map[string]any{
		"x": true,
	}

	var bigEvent EventV1
	if err := util.UnmarshalJSON([]byte(largeEvent), &bigEvent); err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	var ndbCacheExample = ast.MustJSON(builtins.NDBCache{
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
		{
			note: "event with batch decision id",
			event: EventV1{
				Labels:          map[string]string{"foo": "1", "bar": "2"},
				DecisionID:      "1234567890",
				BatchDecisionID: "abcdefghij",
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
		{
			note: "event with intermediate results",
			event: EventV1{
				Labels:     map[string]string{"foo": "1", "bar": "2"},
				DecisionID: "1234567890",
				Bundles: map[string]BundleInfoV1{
					"b1": {"revision7"},
					"b2": {"0"},
					"b3": {},
				},
				Input:               &goInput,
				Path:                "/http/authz/allow",
				RequestedBy:         "[::1]:59943",
				Result:              &result,
				IntermediateResults: map[string]any{"foo": "bar"},
				Timestamp:           time.Now(),
				RequestID:           1,
				inputAST:            astInput,
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
	t.Parallel()

	tests := []struct {
		name                string
		reportingBufferType string
	}{
		{
			name:                "using event buffer",
			reportingBufferType: eventBufferType,
		},
		{
			name: "using size buffer",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()

			testServerPath := "/logs"

			fixture := newTestFixture(t, testFixtureOptions{
				TestServerPath: &testServerPath,
			})
			defer fixture.server.stop()

			fixture.server.ch = make(chan []EventV1, 1)

			var input any = map[string]any{"method": "GET"}
			var result1 any = false

			if err := fixture.plugin.Log(ctx, &server.Info{
				DecisionID: "abc",
				Path:       "data.foo.bar",
				Input:      &input,
				Results:    &result1,
				RemoteAddr: "test",
				Timestamp:  time.Now().UTC(),
			}); err != nil {
				t.Fatal(err)
			}

			if *fixture.plugin.config.Resource != defaultResourcePath {
				t.Errorf("Expected the resource path to be the default %s, actual = '%s'", defaultResourcePath, *fixture.plugin.config.Resource)
			}

			fixture.server.expCode = 200

			err := fixture.plugin.b.Upload(ctx)
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestPluginResourcePathAndPartitionName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                string
		reportingBufferType string
	}{
		{
			name:                "using event buffer",
			reportingBufferType: eventBufferType,
		},
		{
			name: "using size buffer",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
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

			var input any = map[string]any{"method": "GET"}
			var result1 any = false

			if err := fixture.plugin.Log(ctx, &server.Info{
				DecisionID: "abc",
				Path:       "data.foo.bar",
				Input:      &input,
				Results:    &result1,
				RemoteAddr: "test",
				Timestamp:  time.Now().UTC(),
			}); err != nil {
				t.Fatal(err)
			}

			if *fixture.plugin.config.Resource != expectedPath {
				t.Errorf("Expected resource to be %s, but got %s", expectedPath, *fixture.plugin.config.Resource)
			}

			fixture.server.expCode = 200

			err := fixture.plugin.b.Upload(ctx)
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestPluginResourcePath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                           string
		reportingBufferType            string
		reportingBufferSizeLimitEvents int64
	}{
		{
			name:                "using event buffer",
			reportingBufferType: eventBufferType,
		},
		{
			name: "using size buffer",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()

			resourcePath := "/plugin/log/path"
			testServerPath := "/plugin/log/path"

			fixture := newTestFixture(t, testFixtureOptions{
				ReportingBufferType: tc.reportingBufferType,
				Resource:            &resourcePath,
				TestServerPath:      &testServerPath,
			})
			defer fixture.server.stop()

			fixture.server.ch = make(chan []EventV1, 1)

			var input any = map[string]any{"method": "GET"}
			var result1 any = false

			if err := fixture.plugin.Log(ctx, &server.Info{
				DecisionID: "abc",
				Path:       "data.foo.bar",
				Input:      &input,
				Results:    &result1,
				RemoteAddr: "test",
				Timestamp:  time.Now().UTC(),
			}); err != nil {
				t.Fatal(err)
			}

			if *fixture.plugin.config.Resource != resourcePath {
				t.Errorf("Expected resource to be %s, but got %s", resourcePath, *fixture.plugin.config.Resource)
			}

			fixture.server.expCode = 200

			err := fixture.plugin.b.Upload(ctx)
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

type testServer struct {
	t           *testing.T
	expCode     int
	server      *httptest.Server
	ch          chan []EventV1
	path        string
	uploadLimit int64
}

func (t *testServer) handle(w http.ResponseWriter, r *http.Request) {
	t.t.Helper()

	b, err := io.ReadAll(r.Body)
	if err != nil {
		t.t.Fatal(err)
	}

	if int64(len(b)) > t.uploadLimit {
		t.t.Fatalf("upload limit exceeded expected less than %d but got %d", t.uploadLimit, int64(len(b)))
	}

	gr, err := gzip.NewReader(bytes.NewReader(b))
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

func generateInputMap(idx int) map[string]any {
	var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	result := make(map[string]any)

	for range 20 {
		n := idx % len(letters)
		key := string(letters[n])
		result[key] = strconv.Itoa(idx)
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

func testStatus() *bundle.Status {
	tDownload, _ := time.Parse(time.RFC3339Nano, "2018-01-01T00:00:00.0000000Z")
	tActivate, _ := time.Parse(time.RFC3339Nano, "2018-01-01T00:00:01.0000000Z")

	return &bundle.Status{
		Name:                     "example/authz",
		ActiveRevision:           "quickbrawnfaux",
		LastSuccessfulDownload:   tDownload,
		LastSuccessfulActivation: tActivate,
	}
}

func TestConfigUploadLimit(t *testing.T) {
	tests := []struct {
		name          string
		limit         int64
		expectedLimit int64
		expectedLog   string
		expectedErr   string
	}{
		{
			name:          "exceed maximum limit",
			limit:         int64(8589934592),
			expectedLimit: maxUploadSizeLimitBytes,
			expectedLog:   "the configured `upload_size_limit_bytes` (8589934592) has been set to the maximum limit (4294967296)",
		},
		{
			name:          "nothing changes",
			limit:         1000,
			expectedLimit: 1000,
		},
		{
			name:          "negative limit",
			limit:         -1,
			expectedLimit: minUploadSizeLimitBytes,
			expectedLog:   "the configured `upload_size_limit_bytes` (-1) has been set to the minimum limit (90)",
		},
		{
			name:          "equal to minimum",
			limit:         minUploadSizeLimitBytes,
			expectedLimit: minUploadSizeLimitBytes,
		},
		{
			name:          "equal to maximum",
			limit:         maxUploadSizeLimitBytes,
			expectedLimit: maxUploadSizeLimitBytes,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {

			testLogger := test.New()

			cfg := &Config{
				Service: "svc",
				Reporting: ReportingConfig{
					UploadSizeLimitBytes: &tc.limit,
				},
			}
			trigger := plugins.DefaultTriggerMode
			if err := cfg.validateAndInjectDefaults([]string{"svc"}, nil, &trigger, testLogger); err != nil {
				if tc.expectedErr != "" {
					if tc.expectedErr != err.Error() {
						t.Fatalf("Expected error to be `%s` but got `%s`", tc.expectedErr, err.Error())
					} else {
						return
					}
				} else {
					t.Fatal(err)
				}
			}

			if *cfg.Reporting.UploadSizeLimitBytes != tc.expectedLimit {
				t.Fatalf("Expected upload limit to be %d but got %d", tc.expectedLimit, cfg.Reporting.UploadSizeLimitBytes)
			}

			if tc.expectedLog != "" {
				e := testLogger.Entries()
				if e[0].Message != tc.expectedLog {
					t.Fatalf("Expected log to be %s but got %s", tc.expectedLog, e[0].Message)
				}
			} else {
				if len(testLogger.Entries()) != 0 {
					t.Fatalf("Expected log to be empty but got %s", testLogger.Entries()[0].Message)
				}
			}
		})
	}
}

func TestAdaptiveSoftLimitBetweenUpload(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		bufferType       string
		initialSoftLimit int64
		newSoftLimit     int64
	}{
		{
			name:             "using event buffer",
			bufferType:       eventBufferType,
			initialSoftLimit: 300,
			newSoftLimit:     600,
		},
		{
			name:             "using size buffer",
			bufferType:       sizeBufferType,
			initialSoftLimit: 300,
			newSoftLimit:     600,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()

			fixture := newTestFixture(t, testFixtureOptions{
				ReportingBufferType:           tc.bufferType,
				ReportingUploadSizeLimitBytes: tc.initialSoftLimit,
			})
			defer fixture.server.stop()
			defer fixture.plugin.Stop(ctx)

			s := currentSoftLimit(t, fixture.plugin, tc.bufferType)
			if s != tc.initialSoftLimit {
				t.Fatalf("expected %d, got %d", tc.initialSoftLimit, s)
			}

			fixture.server.server.Config.SetKeepAlivesEnabled(false)

			fixture.server.ch = make(chan []EventV1, 4)
			tr := plugins.TriggerManual
			fixture.plugin.config.Reporting.Trigger = &tr

			if err := fixture.plugin.Start(ctx); err != nil {
				t.Fatal(err)
			}

			testMetrics := getWellKnownMetrics()
			msAsFloat64 := map[string]any{}
			for k, v := range testMetrics.All() {
				msAsFloat64[k] = float64(v.(uint64))
			}

			var input any = map[string]any{"method": "GET"}
			var result any = false

			ts, err := time.Parse(time.RFC3339Nano, "2018-01-01T12:00:00.123456Z")
			if err != nil {
				panic(err)
			}

			event := &server.Info{
				Revision:   strconv.Itoa(1),
				DecisionID: strconv.Itoa(1),
				Path:       "tda/bar",
				Input:      &input,
				Results:    &result,
				RemoteAddr: "test",
				Timestamp:  ts,
				Metrics:    testMetrics,
			}

			if err := fixture.plugin.Log(ctx, event); err != nil {
				t.Fatal(err)
			}

			if err := fixture.plugin.Log(ctx, event); err != nil {
				t.Fatal(err)
			}

			// this will increase the soft limit
			if err := fixture.plugin.b.Upload(ctx); err != nil {
				t.Fatal(err)
			}

			s = currentSoftLimit(t, fixture.plugin, tc.bufferType)
			if s != tc.newSoftLimit {
				t.Fatalf("expected %d, got %d", tc.newSoftLimit, s)
			}

			if err := fixture.plugin.Log(ctx, event); err != nil {
				t.Fatal(err)
			}

			if err := fixture.plugin.Log(ctx, event); err != nil {
				t.Fatal(err)
			}

			// the soft limit will stay the same and not be reset to the initial soft limit
			if err := fixture.plugin.b.Upload(ctx); err != nil {
				t.Fatal(err)
			}

			s = currentSoftLimit(t, fixture.plugin, tc.bufferType)
			if s != tc.newSoftLimit {
				t.Fatalf("expected %d, got %d", tc.newSoftLimit, s)
			}
		})
	}
}

func currentSoftLimit(t *testing.T, plugin *Plugin, bufferType string) int64 {
	t.Helper()

	switch bufferType {
	case eventBufferType:
		return plugin.b.(*eventBuffer).enc.uncompressedLimit
	case sizeBufferType:
		return plugin.b.(*sizeBuffer).enc.uncompressedLimit
	default:
		t.Fatal("Unknown buffer type")
	}

	return 0
}

func TestImmediateMode(t *testing.T) {
	tests := []struct {
		name       string
		bufferType string
	}{
		{
			name:       "using event buffer",
			bufferType: eventBufferType,
		},
		{
			name:       "using size buffer",
			bufferType: sizeBufferType,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := t.Context()

			// Configured with a 1-second delay to make sure it is flushed quickly
			delay := int64(1)
			fixture := newTestFixture(t, testFixtureOptions{
				ReportingBufferType: tc.bufferType,
				TriggerMode:         plugins.TriggerImmediate,
				MinDelay:            delay,
				MaxDelay:            delay,
			})
			start := time.Now()
			if err := fixture.plugin.Start(ctx); err != nil {
				t.Fatal(err)
			}
			// Make sure the plugin loop is running
			// Would be really nice to use synctest here but the loop isn't durably blocked
			// Because multiple external channels can stop the loop
			time.Sleep(1 * time.Second)
			defer fixture.plugin.Stop(ctx)
			defer fixture.server.stop()

			fixture.server.ch = make(chan []EventV1, 1)

			event := &server.Info{
				Revision:   strconv.Itoa(1),
				DecisionID: strconv.Itoa(1),
				Path:       "tda/bar",
				RemoteAddr: "test",
			}

			// This event won't create a chunk because of the large default upload limit
			// So it will need to be flushed by the timer
			if err := fixture.plugin.Log(ctx, event); err != nil {
				t.Fatal(err)
			}

			evs := <-fixture.server.ch
			if evs[0].DecisionID != "1" {
				t.Fatalf("expected decision ID %s, got %s", "1", evs[0].DecisionID)
			}
			elapsed := time.Since(start)
			if elapsed < time.Duration(delay)*time.Second {
				t.Fatalf("expected event to be flushed after %d second, got %s", delay, elapsed)
			}

			newConfig := *fixture.plugin.Config()
			// Reconfigure the plugin delay to 5 seconds so that the chunk is returned by the encoder
			delay = int64(5)
			newConfig.Reporting.MinDelaySeconds = &delay
			newConfig.Reporting.MaxDelaySeconds = &delay
			// With this upload limit one logged event will result in a chunk
			uploadLimit := int64(180)
			newConfig.Reporting.UploadSizeLimitBytes = &uploadLimit

			fixture.plugin.reconfigure(t.Context(), &newConfig)

			start = time.Now()
			event2 := &server.Info{
				Revision:   strconv.Itoa(2),
				DecisionID: strconv.Itoa(2),
				Path:       "tda/bar",
				RemoteAddr: "test",
			}
			// This will create a chunk because of the low upload limit
			if err := fixture.plugin.Log(ctx, event2); err != nil {
				t.Fatal(err)
			}

			evs = <-fixture.server.ch
			if evs[0].DecisionID != "2" {
				t.Fatalf("expected decision ID %s, got %s", "1", evs[0].DecisionID)
			}
			elapsed = time.Since(start)
			if elapsed >= time.Duration(delay)*time.Second {
				t.Fatalf("expected chunk to be uploaded sooner than %d seconds, got %s", delay, elapsed)
			}
		})
	}
}
