// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package bundle

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/open-policy-agent/opa/v1/config"
	"github.com/open-policy-agent/opa/v1/download"
	"github.com/open-policy-agent/opa/v1/plugins"
)

// countingServer answers bundle downloads with a 304, which is enough to see
// which endpoint the downloader is talking to without building a bundle.
func countingServer(t *testing.T) (*httptest.Server, *atomic.Int64) {
	t.Helper()

	var hits atomic.Int64
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusNotModified)
	}))
	t.Cleanup(ts.Close)

	return ts, &hits
}

func reconfigureTestPlugin(t *testing.T, url string) (*Plugin, *plugins.Manager, *Config) {
	t.Helper()

	manager := getTestManager()
	t.Cleanup(func() { manager.Stop(t.Context()) })

	if err := manager.Reconfigure(&config.Config{
		Services: fmt.Appendf(nil, `{"acme": {"url": %q}}`, url),
	}); err != nil {
		t.Fatalf("configure manager: %v", err)
	}

	trigger := plugins.TriggerManual
	cfg := &Config{
		Bundles: map[string]*Source{
			"b": {
				Service:  "acme",
				Resource: "/bundles/b.tar.gz",
				Config:   download.Config{Trigger: &trigger},
			},
		},
	}

	plugin := New(cfg, manager)
	t.Cleanup(func() { plugin.Stop(t.Context()) })

	// Stand in for Start, which would also begin polling.
	plugin.downloaders["b"] = plugin.newDownloader("b", cfg.Bundles["b"], cfg.Bundles)

	return plugin, manager, cfg
}

func reconfigureServices(t *testing.T, manager *plugins.Manager, url string) {
	t.Helper()

	if err := manager.Reconfigure(&config.Config{
		Services: fmt.Appendf(nil, `{"acme": {"url": %q}}`, url),
	}); err != nil {
		t.Fatalf("reconfigure manager: %v", err)
	}
}

// The downloader holds the client it was built with, so re-registering the
// service a bundle points at has to restart it even though the bundle's own
// configuration is untouched.
func TestPluginReconfigurePicksUpChangedService(t *testing.T) {
	first, firstHits := countingServer(t)
	second, secondHits := countingServer(t)

	plugin, manager, cfg := reconfigureTestPlugin(t, first.URL)

	if err := plugin.Trigger(t.Context()); err != nil {
		t.Fatalf("trigger: %v", err)
	}
	if firstHits.Load() != 1 || secondHits.Load() != 0 {
		t.Fatalf("expected the first server to be polled, got first=%d second=%d", firstHits.Load(), secondHits.Load())
	}

	// Same bundle config as before: only the service moved.
	reconfigureServices(t, manager, second.URL)
	plugin.Reconfigure(t.Context(), cfg)

	if err := plugin.Trigger(t.Context()); err != nil {
		t.Fatalf("trigger: %v", err)
	}
	if secondHits.Load() != 1 {
		t.Errorf("expected the download to move to the second server, got first=%d second=%d", firstHits.Load(), secondHits.Load())
	}
}

// The converse, and the reason this is worth a test: restarting a downloader
// drops its etag, so a spurious restart re-downloads every bundle in full.
func TestPluginReconfigureUnchangedServiceIsANoOp(t *testing.T) {
	first, _ := countingServer(t)

	plugin, manager, cfg := reconfigureTestPlugin(t, first.URL)

	before := plugin.downloaders["b"]

	reconfigureServices(t, manager, first.URL)
	plugin.Reconfigure(t.Context(), cfg)

	if plugin.downloaders["b"] != before {
		t.Error("expected the downloader to be left alone when nothing changed")
	}
}
