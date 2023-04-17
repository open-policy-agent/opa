// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

//go:build go1.17
// +build go1.17

// NOTE(sr): Split off of plugin_test.go, because time.UnixMilli doesn't
// exist before go 1.17. Can be merged with the other file once we drop
// support for go 1.16.

package status

import (
	"context"
	"testing"
	"time"

	"github.com/open-policy-agent/opa/plugins/bundle"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestPluginPrometheus(t *testing.T) {
	fixture := newTestFixture(t, nil, func(c *Config) {
		c.Prometheus = true
	})
	fixture.server.ch = make(chan UpdateRequestV1)
	defer fixture.server.stop()

	ctx := context.Background()

	err := fixture.plugin.Start(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer fixture.plugin.Stop(ctx)
	<-fixture.server.ch

	status := testStatus()

	fixture.plugin.BulkUpdateBundleStatus(map[string]*bundle.Status{"bundle": status})
	<-fixture.server.ch

	registerMock := fixture.manager.PrometheusRegister().(*prometheusRegisterMock)
	if registerMock.Collectors[pluginStatus] != true {
		t.Fatalf("Plugin status metric was not registered on prometheus")
	}
	if registerMock.Collectors[loaded] != true {
		t.Fatalf("Loaded metric was not registered on prometheus")
	}
	if registerMock.Collectors[failLoad] != true {
		t.Fatalf("FailLoad metric was not registered on prometheus")
	}
	if registerMock.Collectors[lastRequest] != true {
		t.Fatalf("Last request metric was not registered on prometheus")
	}
	if registerMock.Collectors[lastSuccessfulActivation] != true {
		t.Fatalf("Last Successful Activation metric was not registered on prometheus")
	}
	if registerMock.Collectors[lastSuccessfulDownload] != true {
		t.Fatalf("Last Successful Download metric was not registered on prometheus")
	}
	if registerMock.Collectors[lastSuccessfulRequest] != true {
		t.Fatalf("Last Successful Request metric was not registered on prometheus")
	}
	if registerMock.Collectors[bundleLoadDuration] != true {
		t.Fatalf("Bundle Load Duration metric was not registered on prometheus")
	}
	if len(registerMock.Collectors) != 8 {
		t.Fatalf("Number of collectors expected (%v), got %v", 8, len(registerMock.Collectors))
	}

	lastRequestMetricResult := time.UnixMilli(int64(testutil.ToFloat64(lastRequest) / 1e6))
	if !lastRequestMetricResult.Equal(status.LastRequest) {
		t.Fatalf("Last request expected (%v), got %v", status.LastRequest.UTC(), lastRequestMetricResult.UTC())
	}

	lastSuccessfulRequestMetricResult := time.UnixMilli(int64(testutil.ToFloat64(lastSuccessfulRequest) / 1e6))
	if !lastSuccessfulRequestMetricResult.Equal(status.LastSuccessfulRequest) {
		t.Fatalf("Last request expected (%v), got %v", status.LastSuccessfulRequest.UTC(), lastSuccessfulRequestMetricResult.UTC())
	}

	lastSuccessfulDownloadMetricResult := time.UnixMilli(int64(testutil.ToFloat64(lastSuccessfulDownload) / 1e6))
	if !lastSuccessfulDownloadMetricResult.Equal(status.LastSuccessfulDownload) {
		t.Fatalf("Last request expected (%v), got %v", status.LastSuccessfulDownload.UTC(), lastSuccessfulDownloadMetricResult.UTC())
	}

	lastSuccessfulActivationMetricResult := time.UnixMilli(int64(testutil.ToFloat64(lastSuccessfulActivation) / 1e6))
	if !lastSuccessfulActivationMetricResult.Equal(status.LastSuccessfulActivation) {
		t.Fatalf("Last request expected (%v), got %v", status.LastSuccessfulActivation.UTC(), lastSuccessfulActivationMetricResult.UTC())
	}

	bundlesLoaded := testutil.CollectAndCount(loaded)
	if bundlesLoaded != 1 {
		t.Fatalf("Unexpected number of bundle loads (%v), got %v", 1, bundlesLoaded)
	}

	bundlesFailedToLoad := testutil.CollectAndCount(failLoad)
	if bundlesFailedToLoad != 0 {
		t.Fatalf("Unexpected number of bundle fails load (%v), got %v", 0, bundlesFailedToLoad)
	}

	pluginsStatus := testutil.CollectAndCount(pluginStatus)
	if pluginsStatus != 1 {
		t.Fatalf("Unexpected number of plugins (%v), got %v", 1, pluginsStatus)
	}
}

func TestMetricsBundleWithoutRevision(t *testing.T) {
	fixture := newTestFixture(t, nil, func(c *Config) {
		c.Prometheus = true
	})
	fixture.server.ch = make(chan UpdateRequestV1)
	defer fixture.server.stop()

	ctx := context.Background()

	err := fixture.plugin.Start(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer fixture.plugin.Stop(ctx)
	<-fixture.server.ch

	status := testStatus()
	status.ActiveRevision = ""

	fixture.plugin.BulkUpdateBundleStatus(map[string]*bundle.Status{"bundle": status})
	<-fixture.server.ch

	bundlesLoaded := testutil.CollectAndCount(loaded)
	if bundlesLoaded != 1 {
		t.Fatalf("Unexpected number of bundle loads (%v), got %v", 1, bundlesLoaded)
	}

	bundlesFailedToLoad := testutil.CollectAndCount(failLoad)
	if bundlesFailedToLoad != 0 {
		t.Fatalf("Unexpected number of bundle fails load (%v), got %v", 0, bundlesFailedToLoad)
	}
}
