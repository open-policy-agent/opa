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

	status := testStatus()

	fixture.plugin.UpdateBundleStatus(*status)
	result := <-fixture.server.ch

	exp := UpdateRequestV1{
		Labels: map[string]string{
			"id":      "test-instance-id",
			"app":     "example-app",
			"version": version.Version,
		},
		Bundle: status,
	}

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

	status := testStatus()

	fixture.plugin.BulkUpdateBundleStatus(map[string]*bundle.Status{status.Name: status})
	result := <-fixture.server.ch

	exp := UpdateRequestV1{
		Labels: map[string]string{
			"id":      "test-instance-id",
			"app":     "example-app",
			"version": version.Version,
		},
		Bundles: map[string]*bundle.Status{status.Name: status},
	}

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

	config, _ := ParseConfig(pluginConfig, fixture.manager.Services())

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

	status := testStatus()

	fixture.plugin.BulkUpdateBundleStatus(map[string]*bundle.Status{"bundle": status})
	result := <-fixture.server.ch

	exp := map[string]interface{}{"<built-in>": map[string]interface{}{}}

	if !reflect.DeepEqual(result.Metrics, exp) {
		t.Fatalf("Expected %v but got %v", exp, result.Metrics)
	}
}

type testFixture struct {
	manager *plugins.Manager
	plugin  *Plugin
	server  *testServer
}

func newTestFixture(t *testing.T, m metrics.Metrics) testFixture {

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

	config, _ := ParseConfig(pluginConfig, manager.Services())

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
