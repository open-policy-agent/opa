// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package pluginset

import (
	"testing"

	"github.com/open-policy-agent/opa/v1/config"
	"github.com/open-policy-agent/opa/v1/logging"
	"github.com/open-policy-agent/opa/v1/metrics"
	"github.com/open-policy-agent/opa/v1/plugins"
	"github.com/open-policy-agent/opa/v1/plugins/bundle"
	inmemtst "github.com/open-policy-agent/opa/v1/storage/inmem/test"
)

const batchBundleActivationConfig = `
services:
  acme:
    url: https://example.com
bundles:
  b:
    service: acme
    resource: /bundles/b.tar.gz
batch_bundle_activation: true
`

// The option sits at the top level of the configuration rather than inside the
// `bundles` map, which holds nothing but the sources, so it has to be read off
// the top level and handed to the plugin.
func TestParseBatchBundleActivation(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	manager, err := plugins.New([]byte(batchBundleActivationConfig), "test-instance-id", inmemtst.New())
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Stop(ctx)

	configs, err := Parse(map[string]plugins.Factory{}, manager, manager.GetConfig(), metrics.New(), logging.NewNoOpLogger(), nil)
	if err != nil {
		t.Fatal(err)
	}
	configs.Set(manager)

	plugin := bundle.Lookup(manager)
	if plugin == nil {
		t.Fatal("expected the bundle plugin to be registered")
	}
	if !plugin.Config().BatchBundleActivation {
		t.Error("expected batch_bundle_activation to be set on the bundle plugin configuration")
	}
	if _, ok := plugin.Config().Bundles["b"]; !ok {
		t.Error("expected the configured bundle to be present")
	}
}

// Without the option the plugin keeps activating bundles as they arrive.
func TestParseBatchBundleActivationNotSet(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	manager, err := plugins.New([]byte(`
services:
  acme:
    url: https://example.com
bundles:
  b:
    service: acme
    resource: /bundles/b.tar.gz
`), "test-instance-id", inmemtst.New())
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Stop(ctx)

	configs, err := Parse(map[string]plugins.Factory{}, manager, manager.GetConfig(), metrics.New(), logging.NewNoOpLogger(), nil)
	if err != nil {
		t.Fatal(err)
	}
	configs.Set(manager)

	plugin := bundle.Lookup(manager)
	if plugin == nil {
		t.Fatal("expected the bundle plugin to be registered")
	}
	if plugin.Config().BatchBundleActivation {
		t.Error("expected batch_bundle_activation to default to false")
	}
}

// The key is in the schema of known options, so setting it does not draw an
// unrecognized option warning.
func TestParseBatchBundleActivationIsARecognizedOption(t *testing.T) {
	t.Parallel()

	parsed, err := config.ParseConfig([]byte(batchBundleActivationConfig), "test-instance-id")
	if err != nil {
		t.Fatal(err)
	}

	for _, warning := range parsed.Warnings {
		t.Errorf("unexpected warning: %s", warning)
	}
}
