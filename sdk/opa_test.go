// Copyright 2021 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package sdk_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/fortytw2/leaktest"
	loggingtest "github.com/open-policy-agent/opa/logging/test"
	"github.com/open-policy-agent/opa/plugins"
	"github.com/open-policy-agent/opa/sdk"
	sdktest "github.com/open-policy-agent/opa/sdk/test"
	"github.com/open-policy-agent/opa/version"
)

// Plugin creates an empty plugin to test plugin initialization
type plugin struct {
	manager *plugins.Manager
}

type factory struct{}

func (p *plugin) Start(ctx context.Context) error {
	p.manager.UpdatePluginStatus("test_plugin", &plugins.Status{State: plugins.StateOK})
	return nil
}

func (p *plugin) Stop(ctx context.Context) {
}

func (p *plugin) Reconfigure(ctx context.Context, config interface{}) {
}

func (factory) New(manager *plugins.Manager, config interface{}) plugins.Plugin {
	return &plugin{
		manager: manager,
	}
}

func (factory) Validate(manager *plugins.Manager, config []byte) (interface{}, error) {
	return nil, nil
}

func TestPlugins(t *testing.T) {

	ctx := context.Background()

	config := []byte(`{
        "plugins": {
            "test_plugin": {}
		}
	}`)

	opa, err := sdk.New(ctx, sdk.Options{
		Config: bytes.NewReader(config),
		Plugins: map[string]plugins.Factory{
			"test_plugin": factory{},
		},
	})

	if err != nil {
		t.Fatal(err)
	}

	defer opa.Stop(ctx)
}

func TestDecision(t *testing.T) {

	ctx := context.Background()

	server := sdktest.MustNewServer(
		sdktest.MockBundle("/bundles/bundle.tar.gz", map[string]string{
			"main.rego": `
				package system

				main = true

				str = "foo"

				loopback = input
			`,
		}),
	)

	defer server.Stop()

	config := []byte(fmt.Sprintf(`{
		"services": {
			"test": {
				"url": %q
			}
		},
		"bundles": {
			"test": {
				"resource": "/bundles/bundle.tar.gz"
			}
		}
	}`, server.URL()))

	opa, err := sdk.New(ctx, sdk.Options{
		Config: bytes.NewReader(config),
	})
	if err != nil {
		t.Fatal(err)
	}

	defer opa.Stop(ctx)

	if result, err := opa.Decision(ctx, sdk.DecisionOptions{}); err != nil {
		t.Fatal(err)
	} else if decision, ok := result.Result.(bool); !ok || !decision {
		t.Fatal("expected true but got:", decision, ok)
	}

	if result, err := opa.Decision(ctx, sdk.DecisionOptions{Path: "/system/str"}); err != nil {
		t.Fatal(err)
	} else if decision, ok := result.Result.(string); !ok || decision != "foo" {
		t.Fatal(`expected "foo" but got:`, decision)
	}

	exp := map[string]interface{}{"foo": "bar"}

	if result, err := opa.Decision(ctx, sdk.DecisionOptions{Path: "/system/loopback", Input: map[string]interface{}{"foo": "bar"}}); err != nil {
		t.Fatal(err)
	} else if !reflect.DeepEqual(result.Result, exp) {
		t.Fatalf("expected %v but got %v", exp, result.Result)
	}
}

func TestUndefinedError(t *testing.T) {

	ctx := context.Background()

	server := sdktest.MustNewServer(
		sdktest.MockBundle("/bundles/bundle.tar.gz", map[string]string{
			"main.rego": "package system",
		}),
	)

	defer server.Stop()

	config := []byte(fmt.Sprintf(`{
		"services": {
			"test": {
				"url": %q
			}
		},
		"bundles": {
			"test": {
				"resource": "/bundles/bundle.tar.gz"
			}
		}
	}`, server.URL()))

	opa, err := sdk.New(ctx, sdk.Options{
		Config: bytes.NewReader(config),
	})
	if err != nil {
		t.Fatal(err)
	}

	defer opa.Stop(ctx)

	_, err = opa.Decision(ctx, sdk.DecisionOptions{})
	if err == nil {
		t.Fatal("expected error")
	}

	if actual, ok := err.(*sdk.Error); !ok || actual.Code != sdk.UndefinedErr {
		t.Fatalf("expected undefined error but got %v", actual)
	}

}

func TestDecisionLogging(t *testing.T) {

	ctx := context.Background()

	server := sdktest.MustNewServer(
		sdktest.MockBundle("/bundles/bundle.tar.gz", map[string]string{
			"main.rego": "package system\nmain = time.now_ns()",
		}),
	)

	defer server.Stop()

	config := []byte(fmt.Sprintf(`{
		"services": {
			"test": {
				"url": %q
			}
		},
		"bundles": {
			"test": {
				"resource": "/bundles/bundle.tar.gz"
			}
		},
		"decision_logs": {
			"console": true
		}
	}`, server.URL()))

	testLogger := loggingtest.New()
	opa, err := sdk.New(ctx, sdk.Options{
		Config:        bytes.NewReader(config),
		ConsoleLogger: testLogger,
	})
	if err != nil {
		t.Fatal(err)
	}

	defer opa.Stop(ctx)

	// Verify that timestamp matches time.now_ns() value.
	if _, err := opa.Decision(ctx, sdk.DecisionOptions{
		Now: time.Unix(0, 1619868194450288000).UTC(),
	}); err != nil {
		t.Fatal(err)
	}

	entries := testLogger.Entries()
	exp := json.Number("1619868194450288000")

	if len(entries) != 1 || entries[0].Fields["result"] != exp || entries[0].Fields["timestamp"] != "2021-05-01T11:23:14.450288Z" {
		t.Fatalf("expected %v but got %v", exp, entries[0].Fields["result"])
	}

	// Verify that timestamp matches time.now_ns() value.
	if _, err := opa.Decision(ctx, sdk.DecisionOptions{
		Now: time.Unix(0, 1619868194450288000).UTC(),
	}); err != nil {
		t.Fatal(err)
	}

}

func TestQueryCaching(t *testing.T) {

	ctx := context.Background()

	server := sdktest.MustNewServer(
		sdktest.MockBundle("/bundles/bundle.tar.gz", map[string]string{
			"main.rego": "package system\nmain = 7",
		}),
	)

	defer server.Stop()

	config := []byte(fmt.Sprintf(`{
		"services": {
			"test": {
				"url": %q
			}
		},
		"bundles": {
			"test": {
				"resource": "/bundles/bundle.tar.gz"
			}
		},
		"decision_logs": {
			"console": true
		}
	}`, server.URL()))

	testLogger := loggingtest.New()
	opa, err := sdk.New(ctx, sdk.Options{
		Config:        bytes.NewReader(config),
		ConsoleLogger: testLogger,
	})
	if err != nil {
		t.Fatal(err)
	}

	defer opa.Stop(ctx)

	// Execute two queries.
	if _, err := opa.Decision(ctx, sdk.DecisionOptions{}); err != nil {
		t.Fatal(err)
	}

	if _, err := opa.Decision(ctx, sdk.DecisionOptions{}); err != nil {
		t.Fatal(err)
	}

	// Expect two log entries, one with timers for query preparation and the other without.
	entries := testLogger.Entries()

	if len(entries) != 2 {
		t.Fatal("expected two log entries but got:", entries)
	}

	_, ok1 := entries[0].Fields["metrics"].(map[string]interface{})["timer_rego_query_parse_ns"]
	_, ok2 := entries[1].Fields["metrics"].(map[string]interface{})["timer_rego_query_parse_ns"]

	if !ok1 || ok2 {
		t.Fatal("expected first query to require preparation but not the second")
	}

}

func TestDiscovery(t *testing.T) {

	ctx := context.Background()

	server := sdktest.MustNewServer(
		sdktest.MockBundle("/bundles/discovery.tar.gz", map[string]string{
			"bundles.rego": `
				package bundles

				test := {"resource": "/bundles/bundle.tar.gz"}
			`,
		}),
		sdktest.MockBundle("/bundles/bundle.tar.gz", map[string]string{
			"main.rego": `
				package system

				main = 7
			`,
		}),
	)

	defer server.Stop()

	config := []byte(fmt.Sprintf(`{
		"services": {
			"test": {
				"url": %q
			}
		},
		"discovery": {
			"resource": "/bundles/discovery.tar.gz"
		}
	}`, server.URL()))

	opa, err := sdk.New(ctx, sdk.Options{
		Config: bytes.NewReader(config),
	})
	if err != nil {
		t.Fatal(err)
	}

	defer opa.Stop(ctx)

	exp := json.Number("7")

	if result, err := opa.Decision(ctx, sdk.DecisionOptions{}); err != nil {
		t.Fatal(err)
	} else if result.Result != exp {
		t.Fatalf("expected %v but got %v", exp, result.Result)
	}

}

func TestAsync(t *testing.T) {

	ctx := context.Background()

	callerReadyCh := make(chan struct{})
	readyCh := make(chan struct{})

	server := sdktest.MustNewServer(
		sdktest.Ready(callerReadyCh),
		sdktest.MockBundle("/bundles/bundle.tar.gz", map[string]string{
			"main.rego": "package system\nmain = 7",
		}),
	)

	defer server.Stop()

	config := []byte(fmt.Sprintf(`{
		"services": {
			"test": {
				"url": %q
			}
		},
		"bundles": {
			"test": {
				"resource": "/bundles/bundle.tar.gz"
			}
		}
	}`, server.URL()))

	opa, err := sdk.New(ctx, sdk.Options{
		Config: bytes.NewReader(config),
		Ready:  readyCh,
	})
	if err != nil {
		t.Fatal(err)
	}

	if result, err := opa.Decision(ctx, sdk.DecisionOptions{}); !sdk.IsUndefinedErr(err) {
		t.Fatal("expected undefined error but got", result, "err:", err)
	}

	defer opa.Stop(ctx)

	// Signal the server to become ready. By controlling server readiness, we
	// can avoid a race condition above when expecting an undefined decision.
	close(callerReadyCh)

	<-readyCh

	exp := json.Number("7")

	if result, err := opa.Decision(ctx, sdk.DecisionOptions{}); err != nil || !reflect.DeepEqual(result.Result, exp) {
		t.Fatal("expected 7 but got", result, "err:", err)
	}
}

func TestCancelStartup(t *testing.T) {

	server := sdktest.MustNewServer()
	defer server.Stop()

	config := []byte(fmt.Sprintf(`{
		"services": {
			"test": {
				"url": %q
			}
		},
		"bundles": {
			"test": {
				"resource": "/bundles/doesnotexist.tar.gz"
			}
		}
		}`, server.URL()))

	// Server will return 404 responses because bundle does not exist. OPA should timeout.
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*100)
	defer cancel()

	_, err := sdk.New(ctx, sdk.Options{
		Config: bytes.NewReader(config),
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded error but got %v", err)
	}
}

func TestConfigAsYAML(t *testing.T) {

	server := sdktest.MustNewServer(
		sdktest.MockBundle("/bundles/bundle.tar.gz", map[string]string{
			"main.rego": "package system\nmain = 7",
		}),
	)
	defer server.Stop()

	config := []byte(fmt.Sprintf(`services:
  test:
    url: %q
bundles:
  test:
    resource: "/bundles/bundle.tar.gz"`, server.URL()))

	ctx := context.Background()
	_, err := sdk.New(ctx, sdk.Options{
		Config: bytes.NewReader(config),
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestConfigure(t *testing.T) {
	defer leaktest.Check(t)()

	server := sdktest.MustNewServer(
		sdktest.MockBundle("/bundles/bundle1.tar.gz", map[string]string{
			"main.rego": "package system\nmain = 7",
		}),
		sdktest.MockBundle("/bundles/bundle2.tar.gz", map[string]string{
			"main.rego": "package system\nmain = 8",
		}),
	)
	defer server.Stop()

	// Startup new OPA with first config.
	config1 := []byte(fmt.Sprintf(`{
		"services": {
			"test": {
				"url": %q
			}
		},
		"bundles": {
			"test": {
				"resource": "/bundles/bundle1.tar.gz"
			}
		}
	}`, server.URL()))

	ctx := context.Background()
	opa, err := sdk.New(ctx, sdk.Options{
		Config: bytes.NewReader(config1),
	})
	if err != nil {
		t.Fatal(err)
	}

	defer opa.Stop(ctx)

	exp := json.Number("7")

	if result, err := opa.Decision(ctx, sdk.DecisionOptions{}); err != nil {
		t.Fatal(err)
	} else if result.Result != exp {
		t.Fatalf("expected %v but got %v", exp, result.Result)
	}

	// Reconfigure with new config to make sure update is picked up.
	config2 := []byte(fmt.Sprintf(`{
		"services": {
			"test": {
				"url": %q
			}
		},
		"bundles": {
			"test": {
				"resource": "/bundles/bundle2.tar.gz"
			}
		}
	}`, server.URL()))

	err = opa.Configure(ctx, sdk.ConfigOptions{
		Config: bytes.NewReader(config2),
	})
	if err != nil {
		t.Fatal(err)
	}

	exp = json.Number("8")

	if result, err := opa.Decision(ctx, sdk.DecisionOptions{}); err != nil {
		t.Fatal(err)
	} else if result.Result != exp {
		t.Fatalf("expected %v but got %v", exp, result.Result)
	}

	// Reconfigure w/ same config to verify that readiness channel is closed.
	ch := make(chan struct{})
	err = opa.Configure(ctx, sdk.ConfigOptions{
		Config: bytes.NewReader(config2),
		Ready:  ch,
	})
	if err != nil {
		t.Fatal(err)
	}

	<-ch
}

func TestOpaVersion(t *testing.T) {
	ctx := context.Background()

	server := sdktest.MustNewServer(
		sdktest.MockBundle("/bundles/bundle.tar.gz", map[string]string{
			"main.rego": `
				package system
				opa_version := opa.runtime().version
			`,
		}),
	)

	defer server.Stop()

	config := []byte(fmt.Sprintf(`{
		"services": {
			"test": {
				"url": %q
			}
		},
		"bundles": {
			"test": {
				"resource": "/bundles/bundle.tar.gz"
			}
		}
	}`, server.URL()))

	opa, err := sdk.New(ctx, sdk.Options{
		Config: bytes.NewReader(config),
	})
	if err != nil {
		t.Fatal(err)
	}

	defer opa.Stop(ctx)

	exp := version.Version

	if result, err := opa.Decision(ctx, sdk.DecisionOptions{Path: "/system/opa_version"}); err != nil {
		t.Fatal(err)
	} else if !reflect.DeepEqual(result.Result, exp) {
		t.Fatalf("expected %v but got %v", exp, result.Result)
	}
}

func TestOpaRuntimeConfig(t *testing.T) {
	ctx := context.Background()

	server := sdktest.MustNewServer(
		sdktest.MockBundle("/bundles/bundle.tar.gz", map[string]string{
			"main.rego": `
				package system
				rt := opa.runtime()

				result := {
					"service_url": rt.config.services.test.url,
					"bundle_resource": rt.config.bundles.test.resource,
					"test_label": rt.config.labels.test
				}
			`,
		}),
	)

	defer server.Stop()

	testBundleResource := "/bundles/bundle.tar.gz"
	testLabel := "a label"

	config := []byte(fmt.Sprintf(`{
		"services": {
			"test": {
				"url": %q
			}
		},
		"bundles": {
			"test": {
				"resource": %q
			}
		},
		"labels": {
			"test": %q
		}
	}`, server.URL(), testBundleResource, testLabel))

	opa, err := sdk.New(ctx, sdk.Options{
		Config: bytes.NewReader(config),
	})
	if err != nil {
		t.Fatal(err)
	}

	defer opa.Stop(ctx)

	exp := map[string]interface{}{
		"service_url":     server.URL(),
		"bundle_resource": testBundleResource,
		"test_label":      testLabel,
	}

	if result, err := opa.Decision(ctx, sdk.DecisionOptions{Path: "/system/result"}); err != nil {
		t.Fatal(err)
	} else if !reflect.DeepEqual(result.Result, exp) {
		t.Fatalf("expected %v but got %v", exp, result.Result)
	}
}
