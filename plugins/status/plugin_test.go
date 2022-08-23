// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package status

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	prom "github.com/prometheus/client_golang/prometheus"

	"github.com/open-policy-agent/opa/metrics"
	"github.com/open-policy-agent/opa/plugins"
	"github.com/open-policy-agent/opa/plugins/bundle"
	inmem "github.com/open-policy-agent/opa/storage/inmem/test"
	"github.com/open-policy-agent/opa/util"
	"github.com/open-policy-agent/opa/version"
)

func TestMain(m *testing.M) {
	if version.Version == "" {
		version.Version = "unit-test"
	}
	os.Exit(m.Run())
}

func TestPluginStart(t *testing.T) {

	fixture := newTestFixture(t, nil)
	fixture.server.ch = make(chan UpdateRequestV1)
	defer fixture.server.stop()

	ctx := context.Background()

	err := fixture.plugin.Start(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer fixture.plugin.Stop(ctx)

	// Start will trigger a status update when the plugin state switches
	// from "not ready" to "ok".
	result := <-fixture.server.ch

	exp := UpdateRequestV1{
		Labels: map[string]string{
			"id":      "test-instance-id",
			"app":     "example-app",
			"version": version.Version,
		},
		Plugins: map[string]*plugins.Status{
			"status": {State: plugins.StateOK},
		},
	}

	if !reflect.DeepEqual(result, exp) {
		t.Fatalf("Expected: %v but got: %v", exp, result)
	}

	status := testStatus()

	fixture.plugin.BulkUpdateBundleStatus(map[string]*bundle.Status{"test": status})
	result = <-fixture.server.ch

	exp.Bundles = map[string]*bundle.Status{"test": status}

	if !reflect.DeepEqual(result, exp) {
		t.Fatalf("Expected: %v but got: %v", exp, result)
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
			config: []byte(`{"status": {}}`),
		},
		{
			note:   "only disabled console logger",
			config: []byte(`{"status": {"console": "false"}}`),
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

func TestPluginStartTriggerManual(t *testing.T) {

	fixture := newTestFixture(t, nil)
	fixture.server.ch = make(chan UpdateRequestV1)
	defer fixture.server.stop()

	ctx := context.Background()
	tr := plugins.TriggerManual
	fixture.plugin.config.Trigger = &tr

	// Start will trigger a status update when the plugin state switches
	// from "not ready" to "ok". This status update will be sent only after a manual trigger
	err := fixture.plugin.Start(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer fixture.plugin.Stop(ctx)

	// trigger the status update
	go func() {
		_ = fixture.plugin.Trigger(ctx)
	}()

	result := <-fixture.server.ch

	exp := UpdateRequestV1{
		Labels: map[string]string{
			"id":      "test-instance-id",
			"app":     "example-app",
			"version": version.Version,
		},
		Plugins: map[string]*plugins.Status{
			"status": {State: plugins.StateOK},
		},
	}

	if !reflect.DeepEqual(result, exp) {
		t.Fatalf("Expected: %v but got: %v", exp, result)
	}

	status := testStatus()

	fixture.plugin.BulkUpdateBundleStatus(map[string]*bundle.Status{"test": status})

	// trigger the status update
	go func() {
		_ = fixture.plugin.Trigger(ctx)
	}()

	result = <-fixture.server.ch

	exp.Bundles = map[string]*bundle.Status{"test": status}

	if !reflect.DeepEqual(result, exp) {
		t.Fatalf("Expected: %v but got: %v", exp, result)
	}
}

func TestPluginStartTriggerManualMultiple(t *testing.T) {

	fixture := newTestFixture(t, nil)
	fixture.server.ch = make(chan UpdateRequestV1)
	defer fixture.server.stop()

	ctx := context.Background()
	tr := plugins.TriggerManual
	fixture.plugin.config.Trigger = &tr

	err := fixture.plugin.Start(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer fixture.plugin.Stop(ctx)

	status := testStatus()
	fixture.plugin.BulkUpdateBundleStatus(map[string]*bundle.Status{"test": status})
	fixture.plugin.UpdateDiscoveryStatus(*status)

	exp := UpdateRequestV1{
		Labels: map[string]string{
			"id":      "test-instance-id",
			"app":     "example-app",
			"version": version.Version,
		},
		Plugins: map[string]*plugins.Status{
			"status": {State: plugins.StateOK},
		},
	}

	// trigger the status update
	go func() {
		_ = fixture.plugin.Trigger(ctx)
	}()

	result := <-fixture.server.ch

	exp.Bundles = map[string]*bundle.Status{"test": status}
	exp.Discovery = status

	if !reflect.DeepEqual(result, exp) {
		t.Fatalf("Expected: %v but got: %v", exp, result)
	}
}

func TestPluginStartTriggerManualWithTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(3 * time.Second) // this should cause the context deadline to exceed
	}))

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

	manager, err := plugins.New(managerConfig, "test-instance-id", inmem.New())
	if err != nil {
		t.Fatal(err)
	}

	pluginConfig := []byte(`{
			"service": "example",
		}`)

	config, _ := ParseConfig(pluginConfig, manager.Services(), nil)
	tr := plugins.TriggerManual
	config.Trigger = &tr

	p := New(config, manager)

	// Start will trigger a status update when the plugin state switches
	// from "not ready" to "ok". This status update will be sent only after a manual trigger
	err = p.Start(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Stop(ctx)

	// trigger the status update
	done := make(chan struct{})
	go func() {
		// this call should block till the context deadline exceeds
		_ = p.Trigger(ctx)
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
}

func TestPluginStartTriggerManualWithError(t *testing.T) {
	ctx := context.Background()

	managerConfig := []byte(`{
			"labels": {
				"app": "example-app"
			},
			"services": [
				{
					"name": "example",
					"url": "http://localhost:12345"
				}
			]}`)

	manager, err := plugins.New(managerConfig, "test-instance-id", inmem.New())
	if err != nil {
		t.Fatal(err)
	}

	pluginConfig := []byte(`{
			"service": "example",
		}`)

	config, _ := ParseConfig(pluginConfig, manager.Services(), nil)
	tr := plugins.TriggerManual
	config.Trigger = &tr

	p := New(config, manager)

	// Start will trigger a status update when the plugin state switches
	// from "not ready" to "ok". This status update will be sent only after a manual trigger
	err = p.Start(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Stop(ctx)

	// trigger the status update
	// this call should result in an error from the bad service config
	err = p.Trigger(ctx)
	if err == nil {
		t.Fatal("Expected error but got nil")
	}

	exp := "connection refused"
	if !strings.Contains(err.Error(), exp) {
		t.Fatalf("Unexpected error message %v", err.Error())
	}
}

func TestPluginStartBulkUpdate(t *testing.T) {

	fixture := newTestFixture(t, nil)
	fixture.server.ch = make(chan UpdateRequestV1)
	defer fixture.server.stop()

	ctx := context.Background()

	err := fixture.plugin.Start(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer fixture.plugin.Stop(ctx)

	// Start will trigger a status update when the plugin state switches
	// from "not ready" to "ok".
	<-fixture.server.ch // Discard first request.

	exp := UpdateRequestV1{
		Labels: map[string]string{
			"id":      "test-instance-id",
			"app":     "example-app",
			"version": version.Version,
		},
		Plugins: map[string]*plugins.Status{
			"status": {State: plugins.StateOK},
		},
	}

	status := testStatus()

	fixture.plugin.BulkUpdateBundleStatus(map[string]*bundle.Status{status.Name: status})
	result := <-fixture.server.ch

	exp.Bundles = map[string]*bundle.Status{status.Name: status}

	if !reflect.DeepEqual(result, exp) {
		t.Fatalf("Expected: %v but got: %v", exp, result)
	}
}

func TestPluginStartBulkUpdateMultiple(t *testing.T) {

	fixture := newTestFixture(t, nil)
	fixture.server.ch = make(chan UpdateRequestV1)
	defer fixture.server.stop()

	ctx := context.Background()

	err := fixture.plugin.Start(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer fixture.plugin.Stop(ctx)

	// Ignore the plugin updating its status (tested elsewhere)
	<-fixture.server.ch

	statuses := map[string]*bundle.Status{}
	tDownload, _ := time.Parse(time.RFC3339Nano, "2018-01-01T00:00:00.0000000Z")
	tActivate, _ := time.Parse(time.RFC3339Nano, "2018-01-01T00:00:01.0000000Z")
	for i := 0; i < 20; i++ {
		name := fmt.Sprintf("test-bundle-%d", i)
		statuses[name] = &bundle.Status{
			Name:                     name,
			ActiveRevision:           fmt.Sprintf("v%d", i),
			LastSuccessfulDownload:   tDownload,
			LastSuccessfulActivation: tActivate,
		}
	}

	fixture.plugin.BulkUpdateBundleStatus(statuses)
	result := <-fixture.server.ch

	expLabels := map[string]string{
		"id":      "test-instance-id",
		"app":     "example-app",
		"version": version.Version,
	}

	if !reflect.DeepEqual(result.Labels, expLabels) {
		t.Fatalf("Unexpected status labels: %+v", result.Labels)
	}

	if len(result.Bundles) != len(statuses) {
		t.Fatalf("Expected %d statuses, got %d", len(statuses), len(result.Bundles))
	}

	for name, s := range statuses {
		actualStatus := result.Bundles[name]
		if actualStatus.Name != s.Name ||
			actualStatus.LastSuccessfulActivation != s.LastSuccessfulActivation ||
			actualStatus.LastSuccessfulDownload != s.LastSuccessfulDownload ||
			actualStatus.ActiveRevision != s.ActiveRevision {
			t.Errorf("Bundle %s has unexpected status:\n\n %v\n\nExpected:\n%v\n\n", name, actualStatus, s)
		}
	}
}

func TestPluginStartDiscovery(t *testing.T) {

	fixture := newTestFixture(t, nil)
	fixture.server.ch = make(chan UpdateRequestV1)
	defer fixture.server.stop()

	ctx := context.Background()

	err := fixture.plugin.Start(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer fixture.plugin.Stop(ctx)

	// Ignore the plugin updating its status (tested elsewhere)
	<-fixture.server.ch

	status := testStatus()

	fixture.plugin.UpdateDiscoveryStatus(*status)
	result := <-fixture.server.ch

	exp := UpdateRequestV1{
		Labels: map[string]string{
			"id":      "test-instance-id",
			"app":     "example-app",
			"version": version.Version,
		},
		Discovery: status,
		Plugins: map[string]*plugins.Status{
			"status": {State: plugins.StateOK},
		},
	}

	if !reflect.DeepEqual(result, exp) {
		t.Fatalf("Expected: %+v but got: %+v", exp, result)
	}
}

func TestPluginBadAuth(t *testing.T) {
	fixture := newTestFixture(t, nil)
	ctx := context.Background()
	fixture.server.expCode = 401
	defer fixture.server.stop()
	fixture.plugin.lastBundleStatuses = map[string]*bundle.Status{}
	err := fixture.plugin.oneShot(ctx)
	if err == nil {
		t.Fatal("Expected error")
	}
	if err.Error() != "status update failed, server replied with HTTP 401 Unauthorized" {
		t.Fatalf("Unexpected error contents: %v", err)
	}
}

func TestPluginBadPath(t *testing.T) {
	fixture := newTestFixture(t, nil)
	ctx := context.Background()
	fixture.server.expCode = 404
	defer fixture.server.stop()
	fixture.plugin.lastBundleStatuses = map[string]*bundle.Status{}
	err := fixture.plugin.oneShot(ctx)
	if err == nil {
		t.Fatal("Expected error")
	}
	if err.Error() != "status update failed, server replied with HTTP 404 Not Found" {
		t.Fatalf("Unexpected error contents: %v", err)
	}
}

func TestPluginBadStatus(t *testing.T) {
	fixture := newTestFixture(t, nil)
	ctx := context.Background()
	fixture.server.expCode = 500
	defer fixture.server.stop()
	fixture.plugin.lastBundleStatuses = map[string]*bundle.Status{}
	err := fixture.plugin.oneShot(ctx)
	if err == nil {
		t.Fatal("Expected error")
	}
	if err.Error() != "status update failed, server replied with HTTP 500 Internal Server Error" {
		t.Fatalf("Unexpected error contents: %v", err)
	}
}

func TestPluginNonstandardStatus(t *testing.T) {
	fixture := newTestFixture(t, nil)
	ctx := context.Background()
	fixture.server.expCode = 599
	defer fixture.server.stop()
	fixture.plugin.lastBundleStatuses = map[string]*bundle.Status{}
	err := fixture.plugin.oneShot(ctx)
	if err == nil {
		t.Fatal("Expected error")
	}
	if err.Error() != "status update failed, server replied with HTTP 599 " {
		t.Fatalf("Unexpected error contents: %v", err)
	}
}

func TestPlugin2xxStatus(t *testing.T) {
	fixture := newTestFixture(t, nil)
	ctx := context.Background()
	fixture.server.expCode = 204
	defer fixture.server.stop()
	fixture.plugin.lastBundleStatuses = map[string]*bundle.Status{}
	err := fixture.plugin.oneShot(ctx)
	if err != nil {
		t.Fatal("Expected no error")
	}
}

func TestPluginReconfigure(t *testing.T) {
	ctx := context.Background()
	fixture := newTestFixture(t, nil)
	defer fixture.server.stop()

	if err := fixture.plugin.Start(ctx); err != nil {
		t.Fatal(err)
	}

	pluginConfig := []byte(`{
			"service": "example",
			"partition_name": "test"
		}`)

	config, _ := ParseConfig(pluginConfig, fixture.manager.Services(), nil)

	fixture.plugin.Reconfigure(ctx, config)
	fixture.plugin.Stop(ctx)

	if fixture.plugin.config.PartitionName != "test" {
		t.Fatalf("Expected partition name: test but got %v", fixture.plugin.config.PartitionName)
	}
}

func TestMetrics(t *testing.T) {
	fixture := newTestFixture(t, metrics.New())
	fixture.server.ch = make(chan UpdateRequestV1)
	defer fixture.server.stop()

	ctx := context.Background()

	err := fixture.plugin.Start(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer fixture.plugin.Stop(ctx)

	// Ignore the plugin updating its status (tested elsewhere)
	<-fixture.server.ch

	status := testStatus()

	fixture.plugin.BulkUpdateBundleStatus(map[string]*bundle.Status{"bundle": status})
	result := <-fixture.server.ch

	exp := map[string]interface{}{"<built-in>": map[string]interface{}{}}

	if !reflect.DeepEqual(result.Metrics, exp) {
		t.Fatalf("Expected %v but got %v", exp, result.Metrics)
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

	config, err := ParseConfig(loggerConfig, services, nil)

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

	config, err := ParseConfig(loggerConfig, services, nil)

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
			config:   []byte(`{"trigger": "manual"}`),
			expected: plugins.TriggerManual,
		},
		{
			note:     "trigger mode mismatch",
			config:   []byte(`{"trigger": "manual"}`),
			expected: plugins.TriggerPeriodic,
			wantErr:  true,
			err:      fmt.Errorf("invalid status config: trigger mode mismatch: periodic and manual (hint: check discovery configuration)"),
		},
		{
			note:     "bad trigger mode",
			config:   []byte(`{"trigger": "foo"}`),
			expected: "foo",
			wantErr:  true,
			err:      fmt.Errorf("invalid status config: invalid trigger mode \"foo\" (want \"periodic\" or \"manual\")"),
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

				if *c.Trigger != tc.expected {
					t.Fatalf("Expected trigger mode %v but got %v", tc.expected, *c.Trigger)
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

type testPluginCustomizer func(c *Config)

func newTestFixture(t *testing.T, m metrics.Metrics, options ...testPluginCustomizer) testFixture {

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

	registerMock := &prometheusRegisterMock{
		Collectors: map[prom.Collector]bool{},
	}
	manager, err := plugins.New(managerConfig, "test-instance-id", inmem.New(), plugins.WithPrometheusRegister(registerMock))
	if err != nil {
		t.Fatal(err)
	}

	pluginConfig := []byte(`{
			"service": "example",
		}`)

	config, _ := ParseConfig(pluginConfig, manager.Services(), nil)
	for _, option := range options {
		option(config)
	}

	p := New(config, manager).WithMetrics(m)

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

func testStatus() *bundle.Status {
	tDownload, _ := time.Parse(time.RFC3339Nano, "2018-01-01T00:00:00.0000000Z")
	tActivate, _ := time.Parse(time.RFC3339Nano, "2018-01-01T00:00:01.0000000Z")
	tSuccessfulRequest, _ := time.Parse(time.RFC3339Nano, "2018-01-01T00:00:02.0000000Z")
	tRequest, _ := time.Parse(time.RFC3339Nano, "2018-01-01T00:00:03.0000000Z")

	status := bundle.Status{
		Name:                     "example/authz",
		ActiveRevision:           "quickbrawnfaux",
		LastSuccessfulDownload:   tDownload,
		LastSuccessfulActivation: tActivate,
		LastRequest:              tRequest,
		LastSuccessfulRequest:    tSuccessfulRequest,
	}

	return &status
}

type testPlugin struct {
	reqs []UpdateRequestV1
}

func (*testPlugin) Start(context.Context) error {
	return nil
}

func (p *testPlugin) Stop(context.Context) {
}

func (p *testPlugin) Reconfigure(context.Context, interface{}) {
}

func (p *testPlugin) Log(_ context.Context, req *UpdateRequestV1) error {
	p.reqs = append(p.reqs, *req)
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
	err = plugin.oneShot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	err = plugin.oneShot(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if len(backend.reqs) != 2 {
		t.Fatalf("Unexpected number of reqs: expected 2, got %d: %v", len(backend.reqs), backend.reqs)
	}
}

type prometheusRegisterMock struct {
	Collectors map[prom.Collector]bool
}

func (p prometheusRegisterMock) Register(collector prom.Collector) error {
	p.Collectors[collector] = true
	return nil
}

func (p prometheusRegisterMock) MustRegister(collector ...prom.Collector) {
	for _, c := range collector {
		p.Collectors[c] = true
	}
}

func (p prometheusRegisterMock) Unregister(collector prom.Collector) bool {
	delete(p.Collectors, collector)
	return true
}
