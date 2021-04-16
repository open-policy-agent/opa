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
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"

	"github.com/open-policy-agent/opa/metrics"
	"github.com/open-policy-agent/opa/plugins"
	"github.com/open-policy-agent/opa/plugins/bundle"
	"github.com/open-policy-agent/opa/storage/inmem"
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

	fixture.plugin.Start(ctx)
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

	fixture.plugin.UpdateBundleStatus(*status)
	result = <-fixture.server.ch

	exp.Bundle = status

	if !reflect.DeepEqual(result, exp) {
		t.Fatalf("Expected: %v but got: %v", exp, result)
	}
}

func TestPluginStartBulkUpdate(t *testing.T) {

	fixture := newTestFixture(t, nil)
	fixture.server.ch = make(chan UpdateRequestV1)
	defer fixture.server.stop()

	ctx := context.Background()

	fixture.plugin.Start(ctx)
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

	status := testStatus()

	fixture.plugin.BulkUpdateBundleStatus(map[string]*bundle.Status{status.Name: status})
	result = <-fixture.server.ch

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

	fixture.plugin.Start(ctx)
	defer fixture.plugin.Stop(ctx)

	// Ignore the plugin updating its status (tested elsewhere)
	<-fixture.server.ch

	statuses := map[string]*bundle.Status{}
	tDownload, _ := time.Parse("2018-01-01T00:00:00.0000000Z", time.RFC3339Nano)
	tActivate, _ := time.Parse("2018-01-01T00:00:01.0000000Z", time.RFC3339Nano)
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

	fixture.plugin.Start(ctx)
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

func TestPluginConsoleLogging(t *testing.T) {
	logLevel := logrus.GetLevel()
	defer logrus.SetLevel(logLevel)

	// Ensure that status messages are printed to console even with the standard logger configured to log errors only
	logrus.SetLevel(logrus.ErrorLevel)

	hook := test.NewLocal(plugins.GetConsoleLogger())

	fixture := newTestFixture(t, nil, func(c *Config) {
		c.ConsoleLogs = true
		c.Service = ""
	})

	ctx := context.Background()
	_ = fixture.plugin.Start(ctx)
	defer fixture.plugin.Stop(ctx)

	status := testStatus()

	fixture.plugin.UpdateDiscoveryStatus(*status)

	// Give the logger / console some time to process and print the events
	time.Sleep(10 * time.Millisecond)

	// Skip the first entry as it is about the plugin getting updated
	e := hook.AllEntries()[1]

	if e.Message != "Status Log" {
		t.Fatal("Expected status log to console")
	}
	if _, ok := e.Data["discovery"]; !ok {
		t.Fatal("Expected discovery status update")
	}
}

func TestPluginBadAuth(t *testing.T) {
	fixture := newTestFixture(t, nil)
	ctx := context.Background()
	fixture.server.expCode = 401
	defer fixture.server.stop()
	fixture.plugin.lastBundleStatus = &bundle.Status{}
	err := fixture.plugin.oneShot(ctx)
	if err == nil {
		t.Fatal("Expected error")
	}
}

func TestPluginBadPath(t *testing.T) {
	fixture := newTestFixture(t, nil)
	ctx := context.Background()
	fixture.server.expCode = 404
	defer fixture.server.stop()
	fixture.plugin.lastBundleStatus = &bundle.Status{}
	err := fixture.plugin.oneShot(ctx)
	if err == nil {
		t.Fatal("Expected error")
	}
}

func TestPluginBadStatus(t *testing.T) {
	fixture := newTestFixture(t, nil)
	ctx := context.Background()
	fixture.server.expCode = 500
	defer fixture.server.stop()
	fixture.plugin.lastBundleStatus = &bundle.Status{}
	err := fixture.plugin.oneShot(ctx)
	if err == nil {
		t.Fatal("Expected error")
	}
}

func TestPluginReconfigure(t *testing.T) {
	ctx := context.Background()
	fixture := newTestFixture(t, nil)
	defer fixture.server.stop()

	if err := fixture.plugin.Start(ctx); err != nil {
		t.Fatal(err)
	}

	pluginConfig := []byte(fmt.Sprintf(`{
			"service": "example",
			"partition_name": "test"
		}`))

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

	fixture.plugin.Start(ctx)
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

	manager, err := plugins.New(managerConfig, "test-instance-id", inmem.New())
	if err != nil {
		t.Fatal(err)
	}

	pluginConfig := []byte(fmt.Sprintf(`{
			"service": "example",
		}`))

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
	plugin.oneShot(ctx)
	plugin.oneShot(ctx)

	if len(backend.reqs) != 2 {
		t.Fatalf("Unexpected number of reqs: expected 2, got %d: %v", len(backend.reqs), backend.reqs)
	}
}
