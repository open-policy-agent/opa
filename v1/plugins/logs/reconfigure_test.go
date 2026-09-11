// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package logs

import (
	"fmt"
	"testing"

	"github.com/open-policy-agent/opa/v1/config"
	"github.com/open-policy-agent/opa/v1/plugins"
	inmem "github.com/open-policy-agent/opa/v1/storage/inmem/test"
)

func servicesConfig(url string) []byte {
	return fmt.Appendf(nil, `{"acme": {"url": %q}}`, url)
}

func reconfigureTestPlugin(t *testing.T, url string) (*Plugin, *plugins.Manager) {
	t.Helper()

	manager, err := plugins.New(nil, "test-instance-id", inmem.New())
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	if err := manager.Reconfigure(&config.Config{Services: servicesConfig(url)}); err != nil {
		t.Fatalf("configure manager: %v", err)
	}

	cfg := &Config{Service: "acme"}
	trigger := plugins.DefaultTriggerMode
	if err := cfg.validateAndInjectDefaults([]string{"acme"}, nil, &trigger, nil); err != nil {
		t.Fatalf("validate config: %v", err)
	}

	return New(cfg, manager), manager
}

// The buffer holds the client it was built with, so re-registering the service
// the plugin already points at has to reach the uploader even though the
// plugin's own configuration is untouched.
func TestPluginReconfigurePicksUpChangedService(t *testing.T) {
	plugin, manager := reconfigureTestPlugin(t, "https://first.example.com")

	if got := plugin.b.(*sizeBuffer).client.Config().URL; got != "https://first.example.com" {
		t.Fatalf("expected the buffer to start on the first URL, got %q", got)
	}

	if err := manager.Reconfigure(&config.Config{
		Services: servicesConfig("https://second.example.com"),
	}); err != nil {
		t.Fatalf("reconfigure manager: %v", err)
	}

	// Same plugin config as before: only the service moved.
	plugin.reconfigure(t.Context(), plugin.Config())

	if got := plugin.b.(*sizeBuffer).client.Config().URL; got != "https://second.example.com" {
		t.Errorf("expected the buffer to be rebuilt on the second URL, got %q", got)
	}
}

// The converse: an unchanged service must not rebuild the buffer, which would
// flush and re-upload on every reconfigure.
func TestPluginReconfigureUnchangedServiceIsANoOp(t *testing.T) {
	plugin, manager := reconfigureTestPlugin(t, "https://first.example.com")

	before := plugin.b

	if err := manager.Reconfigure(&config.Config{
		Services: servicesConfig("https://first.example.com"),
	}); err != nil {
		t.Fatalf("reconfigure manager: %v", err)
	}

	plugin.reconfigure(t.Context(), plugin.Config())

	if plugin.b != before {
		t.Error("expected the buffer to be left alone when nothing changed")
	}
}
