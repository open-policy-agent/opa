// Copyright 2021 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package sdk

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"

	"github.com/open-policy-agent/opa/compile"
	"github.com/open-policy-agent/opa/logging"
	"github.com/open-policy-agent/opa/util/test"
)

func TestQuery(t *testing.T) {
	files := map[string]string{
		"/bundle/policy.rego": `package http.example.authz
		
		default allow = false

		allow {
			input.roles[_] == "admin"
		}`,
		"/bundle/.manifest": `{
		    "roots": ["http/example/authz"]
		}`,
		"/policy.rego": `package policy

		allow {
			input.method == "GET"
			input.path == ["some", "path"]
			input.roles[_] == "developer"
		}`,
	}

	test.WithTempFS(files, func(path string) {
		buf := bytes.NewBuffer(nil)
		ctx := context.Background()
		err := compile.New().WithAsBundle(true).WithPaths(filepath.Join(path, "bundle")).WithOutput(buf).Build(ctx)
		if err != nil {
			t.Fatalf("unexpected error %v", err)
		}

		server := test.NewTestManagementServer(t).WithBundleAtEndpoint(buf, "/bundles/http/example/authz.tar.gz")
		server.Start()

		config := `{
		"services": {
			"test": {
				"url": "` + server.HTTPServer.URL + `"
			}
		},
		"bundles": {
			"test": {
				"service": "test",
				"resource": "bundles/http/example/authz.tar.gz"
			}
		},
		"decision_logs": {
			"service": "test"
		}
	}`

		args := []func(*OPA) error{
			Paths([]string{filepath.Join(path, "policy.rego")}),
			Config(config),
			Logger(logging.NewNoOpLogger()),
		}
		opa, err := StartOPAWaitUntilReady(ctx, args...)
		if err != nil {
			t.Fatalf("unexpected error %v", err)
		}

		defer opa.Stop(ctx)
		defer server.Cleanup()
		defer server.Stop()

		if !opa.AwaitReady(ctx) {
			t.Fatalf("expected plugins and bundles to have loaded")
		}

		tests := []struct {
			note                  string
			input                 map[string]interface{}
			decisionPath          string
			decisionAssertionFunc func(interface{})
		}{
			{
				note: "policy from filesystem path",
				input: map[string]interface{}{
					"method": "GET",
					"path":   []string{"some", "path"},
					"roles":  []string{"developer"},
				},
				decisionPath: "data.policy.allow",
				decisionAssertionFunc: func(decision interface{}) {
					if decision.(bool) != true {
						t.Errorf("Expected data.policy.allow = true")
					}
				},
			},
			{
				note: "policy from remote bundle",
				input: map[string]interface{}{
					"method": "GET",
					"roles":  []string{"developer"},
				},
				decisionPath: "data.http.example.authz.allow",
				decisionAssertionFunc: func(decision interface{}) {
					if decision.(bool) != false {
						t.Errorf("Expected data.http.example.authz.allow = false")
					}
				},
			},
		}

		for _, tc := range tests {
			t.Run(tc.note, func(t *testing.T) {
				decision, err := opa.Query(ctx, tc.decisionPath, tc.input)
				if err != nil {
					t.Fatalf("unexpected error %v", err)
				}
				tc.decisionAssertionFunc(decision.Data)
			})
		}
	})
}

func TestQueryDefaultDecision(t *testing.T) {
	files := map[string]string{
		"/policy.rego": `package system

		main = true
	`}

	test.WithTempFS(files, func(path string) {
		ctx := context.Background()

		opa, err := StartOPAWaitUntilReady(ctx, Paths([]string{path}), Logger(logging.NewNoOpLogger()))
		if err != nil {
			t.Fatalf("unexpected error %v", err)
		}
		defer opa.Stop(ctx)

		decision, err := opa.QueryDefault(ctx, map[string]interface{}{})
		if err != nil {
			t.Fatalf("unexpected error %v", err)
		}
		if decision.Data.(bool) != true {
			t.Errorf("Expected data.main.allow = true")
		}
	})
}

func TestQueryCollectsMetrics(t *testing.T) {
	files := map[string]string{
		"/bundle/policy.rego": `package http.example.authz

		allow = true
	`}

	test.WithTempFS(files, func(path string) {
		buf := bytes.NewBuffer(nil)
		ctx := context.Background()

		err := compile.New().WithPaths(filepath.Join(path, "bundle")).WithOutput(buf).Build(ctx)
		if err != nil {
			t.Fatalf("unexpected error %v", err)
		}

		opa, err := StartOPAWaitUntilReady(ctx, Paths([]string{path}), Logger(logging.NewNoOpLogger()))
		if err != nil {
			t.Fatalf("unexpected error %v", err)
		}
		defer opa.Stop(ctx)

		decision, err := opa.Query(ctx, "data.http.example.authz.allow", map[string]interface{}{})
		if err != nil {
			t.Fatalf("unexpected error %v", err)
		}
		if decision.Data.(bool) != true {
			t.Errorf("Expected data.http.example.authz.allow = true")
		}

		if _, ok := decision.Metrics.All()["timer_sdk_query_ns"]; !ok {
			t.Errorf("Expected timer_sdk_query_ns to have been collected")
		}

		if decision.Metrics.All()["timer_sdk_query_ns"].(int64) <= 0 {
			t.Errorf("Expected timer_sdk_query_ns to be > 0")
		}
	})
}
