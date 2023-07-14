// Copyright 2021 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package sdk_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/fortytw2/leaktest"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/config"
	"github.com/open-policy-agent/opa/hooks"
	"github.com/open-policy-agent/opa/logging"
	loggingtest "github.com/open-policy-agent/opa/logging/test"
	"github.com/open-policy-agent/opa/metrics"
	"github.com/open-policy-agent/opa/plugins"
	"github.com/open-policy-agent/opa/profiler"
	"github.com/open-policy-agent/opa/rego"
	"github.com/open-policy-agent/opa/sdk"
	sdktest "github.com/open-policy-agent/opa/sdk/test"
	"github.com/open-policy-agent/opa/server/types"
	"github.com/open-policy-agent/opa/topdown"
	"github.com/open-policy-agent/opa/topdown/builtins"
	"github.com/open-policy-agent/opa/topdown/lineage"
	"github.com/open-policy-agent/opa/version"
)

// Plugin creates an empty plugin to test plugin initialization and shutdown
type plugin struct {
	manager  *plugins.Manager
	shutdown time.Duration // to simulate a shutdown that takes some time
}

type factory struct {
	shutdown time.Duration
}

func (p *plugin) Start(context.Context) error {
	p.manager.UpdatePluginStatus("test_plugin", &plugins.Status{State: plugins.StateOK})
	return nil
}

func (p *plugin) Stop(ctx context.Context) {
	select {
	case <-ctx.Done():
	case <-time.After(p.shutdown):
	}
}

func (*plugin) Reconfigure(context.Context, interface{}) {
}

func (f factory) New(manager *plugins.Manager, _ interface{}) plugins.Plugin {
	return &plugin{
		manager:  manager,
		shutdown: f.shutdown,
	}
}

func (factory) Validate(*plugins.Manager, []byte) (interface{}, error) {
	return nil, nil
}

func TestPlugins(t *testing.T) {
	ctx := context.Background()
	config := `{
        "plugins": {
            "test_plugin": {}
		}
	}`

	opa, err := sdk.New(ctx, sdk.Options{
		Config: strings.NewReader(config),
		Plugins: map[string]plugins.Factory{
			"test_plugin": factory{},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	opa.Stop(ctx)
}

func TestHookOnConfig(t *testing.T) {
	ctx := context.Background()

	// We're setting up two hooks that smuggle in some new labels, and hold on
	// to their config.
	// NOTE: Hook ordering isn't guaranteed, so we cannot rely on their invocation
	// in sequence.
	th0 := &testhook{k: "foo", v: "baz"}
	th1 := &testhook{k: "fox", v: "quz"}
	opa, err := sdk.New(ctx, sdk.Options{
		ID:     "sdk-id-0",
		Config: strings.NewReader(`{}`),
		Hooks:  hooks.New(th0, th1),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer opa.Stop(ctx)

	exp := &config.Config{
		Labels: map[string]string{
			"id":      "sdk-id-0",
			"version": version.Version,
			"foo":     "baz",
			"fox":     "quz",
		},
	}
	act := th1.c // doesn't matter which hook, they only mutate the config via its pointer
	if diff := cmp.Diff(exp, act, cmpopts.IgnoreFields(config.Config{}, "DefaultDecision", "DefaultAuthorizationDecision")); diff != "" {
		t.Errorf("unexpected config: (-want, +got):\n%s", diff)
	}
}

func TestHookOnConfigDiscovery(t *testing.T) {
	ctx := context.Background()
	th0 := &testhook{k: "foo", v: "baz"}
	th1 := &testhook{k: "fox", v: "quz"}
	disco := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/status" {
			return // ignore status plugin POSTs
		}
		http.FileServer(http.Dir("testdata")).ServeHTTP(w, r)
	}))
	opa, err := sdk.New(ctx, sdk.Options{
		ID: "sdk-id-0",
		Config: strings.NewReader(fmt.Sprintf(`{
"discovery": {"service":"disco", "resource": "disco.tar.gz"},
"services": [{"name":"disco", "url": "%[1]s"}]
		}`, disco.URL)),
		Hooks:  hooks.New(th0, th1),
		Logger: logging.New(),
		Plugins: map[string]plugins.Factory{
			"test_plugin": factory{},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer opa.Stop(ctx)

	exp := &config.Config{
		Labels: map[string]string{
			"id":      "sdk-id-0",
			"version": version.Version,
			"foo":     "baz",
			"fox":     "quz",
		},
		Plugins:   map[string]json.RawMessage{"test_plugin": json.RawMessage("{}")},
		Discovery: json.RawMessage(`{"service":"disco", "resource": "disco.tar.gz"}`),
	}
	act := th1.c // doesn't matter which hook, they only mutate the config via its pointer
	if diff := cmp.Diff(exp, act, cmpopts.IgnoreFields(config.Config{}, "DefaultDecision", "DefaultAuthorizationDecision")); diff != "" {
		t.Errorf("unexpected config: (-want, +got):\n%s", diff)
	}
}

type testhook struct {
	k, v string
	c    *config.Config
}

func (h *testhook) OnConfig(_ context.Context, c *config.Config) (*config.Config, error) {
	c.Labels[h.k] = h.v
	h.c = c
	return c, nil
}

func (h *testhook) OnConfigDiscovery(_ context.Context, c *config.Config) (*config.Config, error) {
	c.Labels[h.k] = h.v
	h.c = c
	return c, nil
}

func TestPluginPanic(t *testing.T) {
	ctx := context.Background()

	opa, err := sdk.New(ctx, sdk.Options{})
	if err != nil {
		t.Fatal(err)
	}
	opa.Stop(ctx)
}

func TestSDKConfigurableID(t *testing.T) {
	ctx := context.Background()

	server := sdktest.MustNewServer(
		sdktest.MockBundle("/bundles/bundle.tar.gz", map[string]string{
			"main.rego": `
package system

main = time.now_ns()
`,
		}),
	)

	defer server.Stop()

	config := fmt.Sprintf(`{
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
	}`, server.URL())

	testLogger := loggingtest.New()
	opa, err := sdk.New(ctx, sdk.Options{
		Config:        strings.NewReader(config),
		ConsoleLogger: testLogger,
		ID:            "164031de-e511-11ec-8fea-0242ac120002"})

	if err != nil {
		t.Fatal(err)
	}

	defer opa.Stop(ctx)

	if _, err := opa.Decision(ctx, sdk.DecisionOptions{
		Now: time.Unix(0, 1619868194450288000).UTC(),
	}); err != nil {
		t.Fatal(err)
	}

	entries := testLogger.Entries()

	if entries[0].Fields["labels"].(map[string]interface{})["id"] != "164031de-e511-11ec-8fea-0242ac120002" {
		t.Fatalf("expected %v but got %v", "164031de-e511-11ec-8fea-0242ac120002", entries[0].Fields["labels"].(map[string]interface{})["id"])
	}

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

	config := fmt.Sprintf(`{
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
	}`, server.URL())

	opa, err := sdk.New(ctx, sdk.Options{
		Config: strings.NewReader(config),
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

func TestDecisionWithStrictBuiltinErrors(t *testing.T) {

	ctx := context.Background()

	server := sdktest.MustNewServer(
		sdktest.MockBundle("/bundles/bundle.tar.gz", map[string]string{
			"main.rego": `
package example

erroring_function(number) = output {
	output := number / 0
}

allow {
	erroring_function(1)
}
`,
		}),
	)

	defer server.Stop()

	config := fmt.Sprintf(`{
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
	}`, server.URL())

	opa, err := sdk.New(ctx, sdk.Options{
		Config: strings.NewReader(config),
	})
	if err != nil {
		t.Fatal(err)
	}

	defer opa.Stop(ctx)

	_, err = opa.Decision(ctx, sdk.DecisionOptions{
		StrictBuiltinErrors: true,
		Path:                "/example/allow",
	})
	if err == nil {
		t.Fatal("expected error but got nil")
	}

	actual, ok := err.(*topdown.Error)
	if !ok || actual.Code != "eval_builtin_error" {
		t.Fatalf("expected eval_builtin_error but got %v", actual)
	}

	if exp, act := "div: divide by zero", actual.Message; exp != act {
		t.Fatalf("expected %v but got %v", exp, act)
	}
}

func TestDecisionWithTrace(t *testing.T) {

	ctx := context.Background()

	server := sdktest.MustNewServer(
		sdktest.MockBundle("/bundles/bundle.tar.gz", map[string]string{
			"main.rego": `
package system

main {
	trace("foobar")
	true
}
			`,
		}),
	)

	defer server.Stop()

	config := fmt.Sprintf(`{
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
	}`, server.URL())

	opa, err := sdk.New(ctx, sdk.Options{
		Config: strings.NewReader(config),
	})
	if err != nil {
		t.Fatal(err)
	}

	defer opa.Stop(ctx)

	tracer := topdown.NewBufferTracer()

	if result, err := opa.Decision(ctx, sdk.DecisionOptions{
		Path:   "/system/main",
		Tracer: tracer,
	}); err != nil {
		t.Fatal(err)
	} else if decision, ok := result.Result.(bool); !ok || !decision {
		t.Fatal("expected true but got:", decision, ok)
	}

	events := lineage.Notes(*(tracer))
	if exp, act := 3, len(events); exp != act {
		t.Fatalf("expected %d events, got %d", exp, act)
	}

	if exp, act := "Enter", string(events[0].Op); exp != act {
		t.Errorf("expected %s event, got %s", exp, act)
	}
	if exp, act := "data.system.main", string(events[0].Location.Text); exp != act {
		t.Errorf("expected location %q got %q", exp, act)
	}
	if exp, act := "Enter", string(events[1].Op); exp != act {
		t.Errorf("expected %s event, got %v", exp, act)
	}
	if exp, act := "Note", string(events[2].Op); exp != act {
		t.Errorf("expected %s event, got %v", exp, act)
	}
	if exp, act := "foobar", events[2].Message; exp != act {
		t.Errorf("unexpected message, wanted %q, got %q", exp, act)
	}
}

func TestDecisionWithMetrics(t *testing.T) {

	ctx := context.Background()

	server := sdktest.MustNewServer(
		sdktest.MockBundle("/bundles/bundle.tar.gz", map[string]string{
			"main.rego": `
package system

main = true
`,
		}),
	)

	defer server.Stop()

	config := fmt.Sprintf(`{
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
	}`, server.URL())

	opa, err := sdk.New(ctx, sdk.Options{
		Config: strings.NewReader(config),
	})
	if err != nil {
		t.Fatal(err)
	}

	defer opa.Stop(ctx)

	m := metrics.New()
	if result, err := opa.Decision(ctx, sdk.DecisionOptions{
		Metrics: m,
	}); err != nil {
		t.Fatal(err)
	} else if decision, ok := result.Result.(bool); !ok || !decision {
		t.Fatal("expected true but got:", decision, ok)
	}

	if exp, act := 4, len(m.All()); exp != act {
		t.Fatalf("expected %d metrics, got %d", exp, act)
	}

	expectedRecordedMetricGroups := map[string]bool{
		"timer_rego": false,
		"timer_sdk":  false,
	}
	for k := range m.All() {
		for group, found := range expectedRecordedMetricGroups {
			if found {
				continue
			}
			if strings.HasPrefix(k, group) {
				expectedRecordedMetricGroups[group] = true
			}
		}
	}
	for group, found := range expectedRecordedMetricGroups {
		if !found {
			t.Errorf("expected metric group %s not recorded", group)
		}
	}

}

func TestDecisionWithIntrumentationAndProfile(t *testing.T) {

	ctx := context.Background()

	server := sdktest.MustNewServer(
		sdktest.MockBundle("/bundles/bundle.tar.gz", map[string]string{
			"main.rego": `
package system

main = true
`,
		}),
	)

	defer server.Stop()

	config := fmt.Sprintf(`{
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
	}`, server.URL())

	opa, err := sdk.New(ctx, sdk.Options{
		Config: strings.NewReader(config),
	})
	if err != nil {
		t.Fatal(err)
	}

	defer opa.Stop(ctx)

	m := metrics.New()
	p := profiler.New()
	if result, err := opa.Decision(ctx, sdk.DecisionOptions{
		Metrics:    m,
		Profiler:   p,
		Instrument: true,
	}); err != nil {
		t.Fatal(err)
	} else if decision, ok := result.Result.(bool); !ok || !decision {
		t.Fatal("expected true but got:", decision, ok)
	}

	if exp, act := 25, len(m.All()); exp != act {
		t.Fatalf("expected %d metrics, got %d", exp, act)
	}

	expectedRecordedMetricGroups := map[string]bool{
		"counter_eval":        false,
		"histogram_eval":      false,
		"timer_query_compile": false,
		"timer_eval":          false,
		"timer_rego":          false,
		"timer_sdk":           false,
	}
	for k := range m.All() {
		for group, found := range expectedRecordedMetricGroups {
			if found {
				continue
			}
			if strings.HasPrefix(k, group) {
				expectedRecordedMetricGroups[group] = true
			}
		}
	}
	for group, found := range expectedRecordedMetricGroups {
		if !found {
			t.Errorf("expected metric group %s not recorded", group)
		}
	}

	stats := p.ReportTopNResults(10, []string{"line"})

	if exp, act := 2, len(stats); exp != act {
		t.Fatalf("expected %d stats, got %d", exp, act)
	}
	if exp, act := "true", string(stats[0].Location.Text); exp != act {
		t.Errorf("expected location %q got %q", exp, act)
	}
	if exp, act := "data.system.main", string(stats[1].Location.Text); exp != act {
		t.Errorf("expected location %q got %q", exp, act)
	}

}

func TestDecisionWithProvenance(t *testing.T) {

	ctx := context.Background()

	server := sdktest.MustNewServer(
		sdktest.MockBundle("/bundles/bundle.tar.gz", map[string]string{
			"main.rego": `
package system

main = true
`,
			".manifest": `{"revision": "v1.0.0"}`,
		}),
	)

	defer server.Stop()

	config := fmt.Sprintf(`{
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
	}`, server.URL())

	opa, err := sdk.New(ctx, sdk.Options{
		Config: strings.NewReader(config),
	})
	if err != nil {
		t.Fatal(err)
	}

	defer opa.Stop(ctx)

	result, err := opa.Decision(ctx, sdk.DecisionOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if decision, ok := result.Result.(bool); !ok || !decision {
		t.Fatal("expected true but got:", decision, ok)
	}

	expectedProvenance := types.ProvenanceV1{
		Version: version.Version,
		Bundles: map[string]types.ProvenanceBundleV1{
			"test": {
				Revision: "v1.0.0",
			},
		},
	}

	if result.Provenance.Version == "" {
		t.Error("expected non empty provenance version")
	}

	if !reflect.DeepEqual(result.Provenance, expectedProvenance) {
		t.Fatalf("expected %v but got %v", expectedProvenance, result.Provenance)
	}

}

func TestDecisionWithBundleData(t *testing.T) {

	ctx := context.Background()

	server := sdktest.MustNewServer(
		sdktest.MockBundle("/bundles/bundle.tar.gz", map[string]string{
			"main.rego": `
package system

main = data.foo
`,
			"data.json": `{"foo": "bar"}`,
		}),
	)

	defer server.Stop()

	config := fmt.Sprintf(`{
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
	}`, server.URL())

	opa, err := sdk.New(ctx, sdk.Options{
		Config: strings.NewReader(config),
	})
	if err != nil {
		t.Fatal(err)
	}

	defer opa.Stop(ctx)

	result, err := opa.Decision(ctx, sdk.DecisionOptions{})
	if err != nil {
		t.Fatal(err)
	}

	exp := "bar"
	if act, ok := result.Result.(string); !ok || act != exp {
		t.Fatalf("expected %s but got %s", exp, act)
	}

}

func TestDecisionWithConfigurableID(t *testing.T) {
	ctx := context.Background()

	server := sdktest.MustNewServer(
		sdktest.MockBundle("/bundles/bundle.tar.gz", map[string]string{
			"main.rego": `
package system

main = time.now_ns()
`,
		}),
	)

	defer server.Stop()

	config := fmt.Sprintf(`{
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
	}`, server.URL())

	testLogger := loggingtest.New()
	opa, err := sdk.New(ctx, sdk.Options{
		Config:        strings.NewReader(config),
		ConsoleLogger: testLogger})

	if err != nil {
		t.Fatal(err)
	}

	defer opa.Stop(ctx)

	if _, err := opa.Decision(ctx, sdk.DecisionOptions{
		Now: time.Unix(0, 1619868194450288000).UTC(),
	}); err != nil {
		t.Fatal(err)
	}

	if _, err := opa.Decision(ctx, sdk.DecisionOptions{
		Now:        time.Unix(0, 1619868194450288000).UTC(),
		DecisionID: "164031de-e511-11ec-8fea-0242ac120002",
	}); err != nil {
		t.Fatal(err)
	}

	entries := testLogger.Entries()

	if exp, act := 2, len(entries); exp != act {
		t.Fatalf("expected %d entries, got %d", exp, act)
	}

	if entries[0].Fields["decision_id"] == "" {
		t.Fatalf("expected not empty decision_id")
	}

	if entries[1].Fields["decision_id"] != "164031de-e511-11ec-8fea-0242ac120002" {
		t.Fatalf("expected %v but got %v", "164031de-e511-11ec-8fea-0242ac120002", entries[1].Fields["decision_id"])
	}
}

func TestPartial(t *testing.T) {

	ctx := context.Background()

	server := sdktest.MustNewServer(
		sdktest.MockBundle("/bundles/bundle.tar.gz", map[string]string{
			"main.rego": `
package test

allow {
	data.junk.x = input.y
}
`,
		}),
	)

	defer server.Stop()

	config := fmt.Sprintf(`{
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
	}`, server.URL())

	testLogger := loggingtest.New()
	opa, err := sdk.New(ctx, sdk.Options{
		Config:        strings.NewReader(config),
		ConsoleLogger: testLogger,
	})
	if err != nil {
		t.Fatal(err)
	}

	defer opa.Stop(ctx)

	var result *sdk.PartialResult
	if result, err = opa.Partial(ctx, sdk.PartialOptions{
		Input:    map[string]int{"y": 2},
		Query:    "data.test.allow = true",
		Unknowns: []string{"data.junk.x"},
		Mapper:   &sdk.RawMapper{},
		Now:      time.Unix(0, 1619868194450288000).UTC(),
	}); err != nil {
		t.Fatal(err)
	} else if decision, ok := result.Result.(*rego.PartialQueries); !ok || decision.Queries[0].String() != "2 = data.junk.x" {
		t.Fatal("expected &{[2 = data.junk.x] []} true but got:", decision, ok)
	}

	entries := testLogger.Entries()

	if l := len(entries); l != 1 {
		t.Fatalf("expected %v but got %v", 1, l)
	}

	// just checking for existence, since it's a complex value
	if entries[0].Fields["mapped_result"] == nil {
		t.Fatalf("expected not nil value for mapped_result but got nil")
	}

	if entries[0].Fields["result"] == nil {
		t.Fatalf("expected not nil value for result but got nil")
	}

	if entries[0].Fields["timestamp"] != "2021-05-01T11:23:14.450288Z" {
		t.Fatalf("expected %v but got %v", "2021-05-01T11:23:14.450288Z", entries[0].Fields["timestamp"])
	}

}

func TestPartialWithStrictBuiltinErrors(t *testing.T) {

	ctx := context.Background()

	server := sdktest.MustNewServer(
		sdktest.MockBundle("/bundles/bundle.tar.gz", map[string]string{
			"main.rego": `
package example

erroring_function(number) = output {
	output := number / 0
}

allow {
	erroring_function(1)
}
`,
		}),
	)

	defer server.Stop()

	config := fmt.Sprintf(`{
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
	}`, server.URL())

	testLogger := loggingtest.New()
	opa, err := sdk.New(ctx, sdk.Options{
		Config:        strings.NewReader(config),
		ConsoleLogger: testLogger,
	})
	if err != nil {
		t.Fatal(err)
	}

	defer opa.Stop(ctx)

	_, err = opa.Partial(ctx, sdk.PartialOptions{
		Input:               map[string]interface{}{},
		Query:               "data.example.allow",
		Unknowns:            []string{},
		Mapper:              &sdk.RawMapper{},
		Now:                 time.Unix(0, 1619868194450288000).UTC(),
		StrictBuiltinErrors: true,
	})
	if err == nil {
		t.Fatal("expected error but got nil")
	}

	actual, ok := err.(*topdown.Error)
	if !ok || actual.Code != "eval_builtin_error" {
		t.Fatalf("expected eval_builtin_error but got %v", actual)
	}

	if exp, act := "div: divide by zero", actual.Message; exp != act {
		t.Fatalf("expected %v but got %v", exp, act)
	}
}

func TestPartialWithTrace(t *testing.T) {

	ctx := context.Background()

	server := sdktest.MustNewServer(
		sdktest.MockBundle("/bundles/bundle.tar.gz", map[string]string{
			"main.rego": `
package system

main {
	trace("foobar")
}
`,
		}),
	)

	defer server.Stop()

	config := fmt.Sprintf(`{
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
	}`, server.URL())

	testLogger := loggingtest.New()
	opa, err := sdk.New(ctx, sdk.Options{
		Config:        strings.NewReader(config),
		ConsoleLogger: testLogger,
	})
	if err != nil {
		t.Fatal(err)
	}

	defer opa.Stop(ctx)

	tracer := topdown.NewBufferTracer()
	_, err = opa.Partial(ctx, sdk.PartialOptions{
		Input:    map[string]interface{}{},
		Query:    "data.system.main",
		Unknowns: []string{},
		Mapper:   &sdk.RawMapper{},
		Now:      time.Unix(0, 1619868194450288000).UTC(),
		Tracer:   tracer,
	})
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	events := lineage.Notes(*(tracer))

	if exp, act := 3, len(events); exp != act {
		t.Fatalf("expected %d events, got %d", exp, act)
	}

	if exp, act := "Enter", string(events[0].Op); exp != act {
		t.Errorf("expected %s event, got %s", exp, act)
	}
	if exp, act := "data.system.main", string(events[0].Location.Text); exp != act {
		t.Errorf("expected location %q got %q", exp, act)
	}
	if exp, act := "Enter", string(events[1].Op); exp != act {
		t.Errorf("expected %s event, got %v", exp, act)
	}
	if exp, act := "Note", string(events[2].Op); exp != act {
		t.Errorf("expected %s event, got %v", exp, act)
	}
	if exp, act := "foobar", events[2].Message; exp != act {
		t.Errorf("unexpected message, wanted %q, got %q", exp, act)
	}
}

func TestPartialWithMetrics(t *testing.T) {

	ctx := context.Background()

	server := sdktest.MustNewServer(
		sdktest.MockBundle("/bundles/bundle.tar.gz", map[string]string{
			"main.rego": `
package test

allow {
	data.junk.x = input.y
}
`,
		}),
	)

	defer server.Stop()

	config := fmt.Sprintf(`{
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
	}`, server.URL())

	opa, err := sdk.New(ctx, sdk.Options{
		Config: strings.NewReader(config),
	})
	if err != nil {
		t.Fatal(err)
	}

	defer opa.Stop(ctx)

	m := metrics.New()
	var result *sdk.PartialResult
	if result, err = opa.Partial(ctx, sdk.PartialOptions{
		Input:    map[string]int{"y": 2},
		Query:    "data.test.allow = true",
		Unknowns: []string{"data.junk.x"},
		Mapper:   &sdk.RawMapper{},
		Now:      time.Unix(0, 1619868194450288000).UTC(),
		Metrics:  m,
	}); err != nil {
		t.Fatal(err)
	} else if decision, ok := result.Result.(*rego.PartialQueries); !ok || decision.Queries[0].String() != "2 = data.junk.x" {
		t.Fatal("expected &{[2 = data.junk.x] []} true but got:", decision, ok)
	}

	if exp, act := 5, len(m.All()); exp != act {
		t.Fatalf("expected %d metrics, got %d", exp, act)
	}

	expectedRecordedMetricGroups := map[string]bool{
		"timer_rego": false,
		"timer_sdk":  false,
	}
	for k := range m.All() {
		for group, found := range expectedRecordedMetricGroups {
			if found {
				continue
			}
			if strings.HasPrefix(k, group) {
				expectedRecordedMetricGroups[group] = true
			}
		}
	}
	for group, found := range expectedRecordedMetricGroups {
		if !found {
			t.Errorf("expected metric group %s not recorded", group)
		}
	}

}

func TestPartialWithInstrumentationAndProfile(t *testing.T) {

	ctx := context.Background()

	server := sdktest.MustNewServer(
		sdktest.MockBundle("/bundles/bundle.tar.gz", map[string]string{
			"main.rego": `
package test

allow {
	data.junk.x = input.y
}
`,
		}),
	)

	defer server.Stop()

	config := fmt.Sprintf(`{
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
	}`, server.URL())

	opa, err := sdk.New(ctx, sdk.Options{
		Config: strings.NewReader(config),
	})
	if err != nil {
		t.Fatal(err)
	}

	defer opa.Stop(ctx)

	m := metrics.New()
	p := profiler.New()
	var result *sdk.PartialResult
	if result, err = opa.Partial(ctx, sdk.PartialOptions{
		Input:      map[string]int{"y": 2},
		Query:      "data.test.allow = true",
		Unknowns:   []string{"data.junk.x"},
		Mapper:     &sdk.RawMapper{},
		Now:        time.Unix(0, 1619868194450288000).UTC(),
		Metrics:    m,
		Profiler:   p,
		Instrument: true,
	}); err != nil {
		t.Fatal(err)
	} else if decision, ok := result.Result.(*rego.PartialQueries); !ok || decision.Queries[0].String() != "2 = data.junk.x" {
		t.Fatal("expected &{[2 = data.junk.x] []} true but got:", decision, ok)
	}

	if exp, act := 32, len(m.All()); exp != act {
		t.Fatalf("expected %d metrics, got %d", exp, act)
	}

	expectedRecordedMetricGroups := map[string]bool{
		"histogram_eval":      false,
		"histogram_partial":   false,
		"timer_query_compile": false,
		"timer_eval":          false,
		"timer_partial":       false,
		"timer_rego":          false,
		"timer_sdk":           false,
	}

	for k := range m.All() {
		for group, found := range expectedRecordedMetricGroups {
			if found {
				continue
			}
			if strings.HasPrefix(k, group) {
				expectedRecordedMetricGroups[group] = true
			}
		}
	}
	for group, found := range expectedRecordedMetricGroups {
		if !found {
			t.Errorf("expected metric group %s not recorded", group)
		}
	}

	stats := p.ReportTopNResults(10, []string{"line"})

	if exp, act := 2, len(stats); exp != act {
		t.Fatalf("expected %d stats, got %d", exp, act)
	}
	if exp, act := "data.junk.x = input.y", string(stats[0].Location.Text); exp != act {
		t.Errorf("expected location %q got %q", exp, act)
	}
	if exp, act := "data.test.allow = true", string(stats[1].Location.Text); exp != act {
		t.Errorf("expected location %q got %q", exp, act)
	}

}

func TestPartialWithProvenance(t *testing.T) {

	ctx := context.Background()

	server := sdktest.MustNewServer(
		sdktest.MockBundle("/bundles/bundle.tar.gz", map[string]string{
			"main.rego": `
package test

allow {
	data.junk.x = input.y
}
`,
			".manifest": `{"revision": "v1.0.0"}`,
		}),
	)

	defer server.Stop()

	config := fmt.Sprintf(`{
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
	}`, server.URL())

	opa, err := sdk.New(ctx, sdk.Options{
		Config: strings.NewReader(config),
	})
	if err != nil {
		t.Fatal(err)
	}

	defer opa.Stop(ctx)

	var result *sdk.PartialResult
	if result, err = opa.Partial(ctx, sdk.PartialOptions{
		Input:    map[string]int{"y": 2},
		Query:    "data.test.allow = true",
		Unknowns: []string{"data.junk.x"},
		Mapper:   &sdk.RawMapper{},
		Now:      time.Unix(0, 1619868194450288000).UTC(),
	}); err != nil {
		t.Fatal(err)
	} else if decision, ok := result.Result.(*rego.PartialQueries); !ok || decision.Queries[0].String() != "2 = data.junk.x" {
		t.Fatal("expected &{[2 = data.junk.x] []} true but got:", decision, ok)
	}

	expectedProvenance := types.ProvenanceV1{
		Version:   version.Version,
		Vcs:       version.Vcs,
		Timestamp: version.Timestamp,
		Hostname:  version.Hostname,
		Bundles: map[string]types.ProvenanceBundleV1{
			"test": {
				Revision: "v1.0.0",
			},
		},
	}

	if !reflect.DeepEqual(result.Provenance, expectedProvenance) {
		t.Fatalf("expected %v but got %v", expectedProvenance, result.Provenance)
	}

}

func TestPartialWithConfigurableID(t *testing.T) {

	ctx := context.Background()

	server := sdktest.MustNewServer(
		sdktest.MockBundle("/bundles/bundle.tar.gz", map[string]string{
			"main.rego": `
package test

allow {
	data.junk.x = input.y
}
`,
		}),
	)

	defer server.Stop()

	config := fmt.Sprintf(`{
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
	}`, server.URL())

	testLogger := loggingtest.New()
	opa, err := sdk.New(ctx, sdk.Options{
		Config:        strings.NewReader(config),
		ConsoleLogger: testLogger,
	})
	if err != nil {
		t.Fatal(err)
	}

	defer opa.Stop(ctx)

	if result, err := opa.Partial(ctx, sdk.PartialOptions{
		Input:    map[string]int{"y": 2},
		Query:    "data.test.allow = true",
		Unknowns: []string{"data.junk.x"},
		Mapper:   &sdk.RawMapper{},
		Now:      time.Unix(0, 1619868194450288000).UTC(),
	}); err != nil {
		t.Fatal(err)
	} else if decision, ok := result.Result.(*rego.PartialQueries); !ok || decision.Queries[0].String() != "2 = data.junk.x" {
		t.Fatal("expected &{[2 = data.junk.x] []} true but got:", decision, ok)
	}

	if result, err := opa.Partial(ctx, sdk.PartialOptions{
		Input:      map[string]int{"y": 2},
		Query:      "data.test.allow = true",
		Unknowns:   []string{"data.junk.x"},
		Mapper:     &sdk.RawMapper{},
		Now:        time.Unix(0, 1619868194450288000).UTC(),
		DecisionID: "164031de-e511-11ec-8fea-0242ac120002",
	}); err != nil {
		t.Fatal(err)
	} else if decision, ok := result.Result.(*rego.PartialQueries); !ok || decision.Queries[0].String() != "2 = data.junk.x" {
		t.Fatal("expected &{[2 = data.junk.x] []} true but got:", decision, ok)
	}

	entries := testLogger.Entries()

	if exp, act := 2, len(entries); exp != act {
		t.Fatalf("expected %d entries, got %d", exp, act)
	}

	if entries[0].Fields["decision_id"] == "" {
		t.Fatalf("expected not empty decision_id")
	}

	if entries[1].Fields["decision_id"] != "164031de-e511-11ec-8fea-0242ac120002" {
		t.Fatalf("expected %v but got %v", "164031de-e511-11ec-8fea-0242ac120002", entries[1].Fields["decision_id"])
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

	config := fmt.Sprintf(`{
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
	}`, server.URL())

	opa, err := sdk.New(ctx, sdk.Options{
		Config: strings.NewReader(config),
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
			"main.rego": `
package system

main = time.now_ns()
`,
		}),
	)

	defer server.Stop()

	config := fmt.Sprintf(`{
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
	}`, server.URL())

	testLogger := loggingtest.New()
	opa, err := sdk.New(ctx, sdk.Options{
		Config:        strings.NewReader(config),
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

func TestDecisionLoggingWithMasking(t *testing.T) {

	ctx := context.Background()

	server := sdktest.MustNewServer(
		sdktest.MockBundle("/bundles/bundle.tar.gz", map[string]string{
			"main.rego": `
package system

main = true

str = "foo"

loopback = input
`,
			"log.rego": `
package system.log

mask["/input/secret"]
mask["/input/top/secret"]
mask["/input/dossier/1/highly"]
`,
		}),
	)

	defer server.Stop()

	config := fmt.Sprintf(`{
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
	}`, server.URL())

	testLogger := loggingtest.New()
	opa, err := sdk.New(ctx, sdk.Options{
		Config:        strings.NewReader(config),
		ConsoleLogger: testLogger,
	})
	if err != nil {
		t.Fatal(err)
	}

	defer opa.Stop(ctx)

	if _, err := opa.Decision(ctx, sdk.DecisionOptions{
		Input: map[string]interface{}{
			"secret": "foo",
			"top": map[string]string{
				"secret": "bar",
			},
			"dossier": []map[string]interface{}{
				{
					"very": "private",
				},
				{
					"highly": "classified",
				},
			},
		},
	}); err != nil {
		t.Fatal(err)
	}

	entries := testLogger.Entries()

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry but got %d", len(entries))
	}

	expectedErased := []interface{}{
		"/input/dossier/1/highly",
		"/input/secret",
		"/input/top/secret",
	}
	erased := entries[0].Fields["erased"].([]interface{})
	stringLess := func(a, b string) bool {
		return a < b
	}
	if !cmp.Equal(expectedErased, erased, cmpopts.SortSlices(stringLess)) {
		t.Errorf("Did not get expected result for erased field in decision log:\n%s", cmp.Diff(expectedErased, erased, cmpopts.SortSlices(stringLess)))
	}
	errMsg := `Expected masked field "%s" to be removed, but it was present.`
	input := entries[0].Fields["input"].(map[string]interface{})
	if _, ok := input["secret"]; ok {
		t.Errorf(errMsg, "/input/secret")
	}

	if _, ok := input["top"].(map[string]interface{})["secret"]; ok {
		t.Errorf(errMsg, "/input/top/secret")
	}

	if _, ok := input["dossier"].([]interface{})[1].(map[string]interface{})["highly"]; ok {
		t.Errorf(errMsg, "/input/dossier/1/highly")
	}

}

func TestDecisionLoggingWithNDBCache(t *testing.T) {

	ctx := context.Background()

	server := sdktest.MustNewServer(
		sdktest.MockBundle("/bundles/bundle.tar.gz", map[string]string{
			"main.rego": `
package system

main = time.now_ns()
`,
		}),
	)

	defer server.Stop()

	config := fmt.Sprintf(`{
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
			"console": true,
			"nd_builtin_cache": true
		}
	}`, server.URL())

	testLogger := loggingtest.New()
	opa, err := sdk.New(ctx, sdk.Options{
		Config:        strings.NewReader(config),
		ConsoleLogger: testLogger,
	})
	if err != nil {
		t.Fatal(err)
	}

	defer opa.Stop(ctx)

	// Build ND builtins cache, and populate with an unused builtin.
	ndbc := builtins.NDBCache{}
	ndbc.Put("rand.intn", ast.NewArray(), ast.NewObject([2]*ast.Term{ast.StringTerm("z"), ast.IntNumberTerm(7)}))

	// Verify that timestamp matches time.now_ns() value.
	if _, err := opa.Decision(ctx, sdk.DecisionOptions{
		Now:      time.Unix(0, 1619868194450288000).UTC(),
		NDBCache: ndbc,
	}); err != nil {
		t.Fatal(err)
	}

	entries := testLogger.Entries()

	// Check the contents of the ND builtins cache.
	if cache, ok := entries[0].Fields["nd_builtin_cache"]; ok {
		// Ensure the original cache entry for rand.intn is still there.
		if _, ok := cache.(map[string]interface{})["rand.intn"]; !ok {
			t.Fatalf("ND builtins cache was not preserved during evaluation.")
		}
		// Ensure time.now_ns entry was picked up correctly.
		if _, ok := cache.(map[string]interface{})["time.now_ns"]; !ok {
			t.Fatalf("ND builtins cache did not observe time.now_ns call during evaluation.")
		}
	} else {
		t.Fatalf("ND builtins cache missing.")
	}

}

func TestQueryCaching(t *testing.T) {

	ctx := context.Background()

	server := sdktest.MustNewServer(
		sdktest.MockBundle("/bundles/bundle.tar.gz", map[string]string{
			"main.rego": `
package system

main = 7
`,
		}),
	)

	defer server.Stop()

	config := fmt.Sprintf(`{
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
	}`, server.URL())

	opa, err := sdk.New(ctx, sdk.Options{
		Config: strings.NewReader(config),
	})
	if err != nil {
		t.Fatal(err)
	}

	defer opa.Stop(ctx)

	// Execute two queries with metrics
	m1 := metrics.New()
	if _, err := opa.Decision(ctx, sdk.DecisionOptions{
		Metrics: m1,
	}); err != nil {
		t.Fatal(err)
	}

	m2 := metrics.New()
	if _, err := opa.Decision(ctx, sdk.DecisionOptions{
		Metrics: m2,
	}); err != nil {
		t.Fatal(err)
	}

	// Expect only the metrics from the first query to contain preparation metrics
	if _, ok := m1.All()["timer_rego_query_parse_ns"]; !ok {
		t.Fatal("first query should have preparation metrics")
	}
	if _, ok := m2.All()["timer_rego_query_parse_ns"]; ok {
		t.Fatal("second query should not have preparation metrics")
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

	config := fmt.Sprintf(`{
		"services": {
			"test": {
				"url": %q
			}
		},
		"discovery": {
			"resource": "/bundles/discovery.tar.gz"
		}
	}`, server.URL())

	opa, err := sdk.New(ctx, sdk.Options{
		Config: strings.NewReader(config),
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
			"main.rego": `
package system

main = 7
`,
		}),
	)

	defer server.Stop()

	config := fmt.Sprintf(`{
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
	}`, server.URL())

	opa, err := sdk.New(ctx, sdk.Options{
		Config: strings.NewReader(config),
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

	config := fmt.Sprintf(`{
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
		}`, server.URL())

	// Server will return 404 responses because bundle does not exist. OPA should timeout.
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*100)
	defer cancel()

	_, err := sdk.New(ctx, sdk.Options{
		Config: strings.NewReader(config),
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded error but got %v", err)
	}
}

// TestStopWithDeadline asserts that a graceful shutdown of the SDK is possible.
func TestStopWithDeadline(t *testing.T) {

	ctx := context.Background()
	opa, err := sdk.New(ctx, sdk.Options{
		Config: strings.NewReader(`{
			"plugins": {
				"test_plugin": {}
			}
		}`),
		Plugins: map[string]plugins.Factory{
			"test_plugin": factory{shutdown: time.Second},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	const timeout = 20 * time.Millisecond
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	before := time.Now()
	opa.Stop(ctx) // 1s timeout is ignored

	dur := time.Since(before)
	diff := dur - timeout
	maxDelta := 500 * time.Millisecond
	if diff > maxDelta || diff < -maxDelta {
		t.Errorf("expected shutdown to have %v grace period, measured shutdown in %v (max delta %v)", timeout, dur, maxDelta)
	}
}

func TestConfigAsYAML(t *testing.T) {

	server := sdktest.MustNewServer(
		sdktest.MockBundle("/bundles/bundle.tar.gz", map[string]string{
			"main.rego": `
package system

main = 7
`,
		}),
	)
	defer server.Stop()

	config := fmt.Sprintf(`services:
  test:
    url: %q
bundles:
  test:
    resource: "/bundles/bundle.tar.gz"`, server.URL())

	ctx := context.Background()
	_, err := sdk.New(ctx, sdk.Options{
		Config: strings.NewReader(config),
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestConfigure(t *testing.T) {
	defer leaktest.Check(t)()

	server := sdktest.MustNewServer(
		sdktest.MockBundle("/bundles/bundle1.tar.gz", map[string]string{
			"main.rego": `
package system

main = 7
`,
		}),
		sdktest.MockBundle("/bundles/bundle2.tar.gz", map[string]string{
			"main.rego": `
package system

main = 8
`,
		}),
	)
	defer server.Stop()

	// Startup new OPA with first config.
	config1 := fmt.Sprintf(`{
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
	}`, server.URL())

	ctx := context.Background()
	opa, err := sdk.New(ctx, sdk.Options{
		Config: strings.NewReader(config1),
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
	config2 := fmt.Sprintf(`{
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
	}`, server.URL())

	err = opa.Configure(ctx, sdk.ConfigOptions{
		Config: strings.NewReader(config2),
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
		Config: strings.NewReader(config2),
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

	config := fmt.Sprintf(`{
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
	}`, server.URL())

	opa, err := sdk.New(ctx, sdk.Options{
		Config: strings.NewReader(config),
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

	config := fmt.Sprintf(`{
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
	}`, server.URL(), testBundleResource, testLabel)

	opa, err := sdk.New(ctx, sdk.Options{
		Config: strings.NewReader(config),
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

func TestPrintStatements(t *testing.T) {

	ctx := context.Background()

	s := sdktest.MustNewServer(sdktest.MockBundle("/bundles/b.tar.gz", map[string]string{
		"x.rego": `
package foo

p { print("XXX") }
`,
	}))

	defer s.Stop()

	config := fmt.Sprintf(`{
		"services": {
			"test": {
				"url": %q
			}
		},
		"bundles": {
			"test": {
				"resource": "/bundles/b.tar.gz"
			}
		}
	}`, s.URL())

	logger := loggingtest.New()
	logger.SetLevel(logging.Info)

	opa, err := sdk.New(ctx, sdk.Options{
		Logger: logger,
		Config: strings.NewReader(config),
	})
	if err != nil {
		t.Fatal(err)
	}

	defer opa.Stop(ctx)

	if _, err := opa.Decision(ctx, sdk.DecisionOptions{Path: "/foo/p"}); err != nil {
		t.Fatal(err)
	}

	entries := logger.Entries()
	if len(entries) == 0 {
		t.Fatal("expected logs")
	}

	e := entries[len(entries)-1]

	if e.Message != "XXX" || e.Fields["line"].(string) != "/x.rego:4" {
		t.Fatal("expected print output but got:", e)
	}
}
