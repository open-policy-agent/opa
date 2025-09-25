// Copyright 2025 The OPA Authors
// SPDX-License-Identifier: Apache-2.0

package compile

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/open-policy-agent/opa/v1/logging/test"
	"github.com/open-policy-agent/opa/v1/runtime"
	"github.com/open-policy-agent/opa/v1/test/e2e"
)

var ignores = []string{
	"timestamp",
	"metrics",
	"labels",
	"intermediate",
	"requested_by",
}

var stdIgnores = cmpopts.IgnoreMapEntries(func(k string, _ any) bool {
	return slices.Contains(ignores, k)
})

func TestDecisionLogsCompileAPIResult(t *testing.T) {
	policy := `
package filters

# METADATA
# compile:
#   unknowns: [input.fruits]
#   mask_rule: data.filters.mask
include if input.fruits.name in input.favorites

default mask.fruits.supplier := {"replace": {"value": "***"}}
`

	path := "filters/include"

	params := e2e.NewAPIServerTestParams()
	params.ConfigOverrides = []string{
		"decision_logs.console=true",
	}
	params.Logging = runtime.LoggingConfig{Level: "error"}
	consoleLogger := test.New()
	params.ConsoleLogger = consoleLogger

	testRuntime, err := e2e.NewTestRuntime(params)
	if err != nil {
		t.Fatal(err)
	}
	testRuntime.ConsoleLogger = consoleLogger
	ctx, cancel := context.WithCancel(t.Context())

	done := make(chan bool)
	go func() {
		err := testRuntime.Runtime.Serve(ctx)
		if err != nil {
			t.Errorf("Unexpected error: %s", err)
		}
		done <- true
	}()
	t.Cleanup(cancel)
	if err := testRuntime.WaitForServer(); err != nil {
		t.Fatal(err)
	}
	opaURL := testRuntime.URL()

	{ // store policy
		req, err := http.NewRequest("PUT", opaURL+"/v1/policies/policy.rego", strings.NewReader(policy))
		if err != nil {
			t.Fatalf("failed to create request: %v", err)
		}
		if _, err := http.DefaultClient.Do(req); err != nil {
			t.Fatalf("put policy: %v", err)
		}
	}

	{ // act: send Compile API request
		input := map[string]any{"favorites": []string{"banana", "orange"}}
		payload := map[string]any{
			"input": input,
			"options": map[string]any{
				"targetSQLTableMappings": map[string]any{
					"postgresql": map[string]any{
						"fruits": map[string]string{
							"$self": "f",
							"name":  "n",
						},
					},
				},
			},
		}

		queryBytes, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("Failed to marshal JSON: %v", err)
		}
		req, err := http.NewRequest("POST",
			fmt.Sprintf("%s/v1/compile/%s", opaURL, path),
			strings.NewReader(string(queryBytes)))
		if err != nil {
			t.Fatalf("failed to create request: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/vnd.opa.sql.postgresql+json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("failed to execute request: %v", err)
		}
		defer resp.Body.Close()
		var respPayload struct {
			Result struct {
				Query any            `json:"query"`
				Masks map[string]any `json:"masks,omitempty"`
			} `json:"result"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&respPayload); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if exp, act := "WHERE f.n IN (E'banana', E'orange')", respPayload.Result.Query; exp != act {
			t.Errorf("response: expected %v, got %v (response: %v)", exp, act, respPayload)
		}
		exp, act := map[string]any{"fruits": map[string]any{"supplier": map[string]any{"replace": map[string]any{"value": "***"}}}}, respPayload.Result.Masks
		if diff := cmp.Diff(exp, act); diff != "" {
			t.Errorf("response: expected %v, got %v (response: %v)", exp, act, respPayload)
		}
	}

	var entry test.LogEntry
	for _, entry = range testRuntime.ConsoleLogger.Entries() {
		if entry.Message == "Decision Log" {
			break
		}
	}

	if entry.Message == "" {
		t.Fatal("no DL messages logged")
	}

	dl := map[string]any{
		"decision_id": "",
		"path":        path,
		"result": map[string]any{
			"query": "WHERE f.n IN (E'banana', E'orange')",
			"masks": map[string]any{
				"fruits": map[string]any{
					"supplier": map[string]any{
						"replace": map[string]any{"value": "***"},
					},
				},
			},
		},
		"input": map[string]any{
			"favorites": []any{"banana", "orange"},
		},
		"req_id": json.Number("3"),
		"type":   "openpolicyagent.org/decision_logs",
		"custom": map[string]any{
			"options": map[string]any{
				"targetSQLTableMappings": map[string]any{
					"postgresql": map[string]any{
						"fruits": map[string]any{
							"$self": "f",
							"name":  "n",
						},
					},
				},
			},
			"unknowns":  []any{"input.fruits"},
			"type":      "open-policy-agent/compile",
			"mask_rule": "data.filters.mask",
		},
	}
	if diff := cmp.Diff(dl, entry.Fields, stdIgnores); diff != "" {
		t.Errorf("diff: (-want +got):\n%s", diff)
	}
}
