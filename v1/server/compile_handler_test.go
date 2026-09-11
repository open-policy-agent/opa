// Copyright 2025 The OPA Authors
// SPDX-License-Identifier: Apache-2.0

package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/storage"
	"github.com/open-policy-agent/opa/v1/storage/inmem"
)

type Query struct {
	Query any `json:"query,omitempty"`
	Masks any `json:"masks,omitempty"`
}
type Response struct {
	Result struct {
		Query    any   `json:"query,omitempty"`
		Masks    any   `json:"masks,omitempty"`
		UCAST    Query `json:"ucast"` // NB: omitempty has no effect on nested struct fields (so the linter tells me)
		Postgres Query `json:"postgresql"`
		MySQL    Query `json:"mysql"`
		MSSQL    Query `json:"sqlserver"`
		SQLite   Query `json:"sqlite"`
	} `json:"result"`
	Metrics map[string]float64 `json:"metrics"`
	Hints   []map[string]any   `json:"hints"`
}

var ignoreMetrics = cmpopts.IgnoreMapEntries(func(k string, _ any) bool { return k == "metrics" })

func setup(t testing.TB, rego string, data any) *fixture {
	ctx := t.Context()
	store := inmem.New()
	txn := storage.NewTransactionOrDie(ctx, store, storage.WriteParams)
	if err := store.UpsertPolicy(ctx, txn, "filters.rego", []byte(rego)); err != nil {
		t.Fatalf("upsert policy: %v", err)
	}
	if data != nil {
		if err := store.Write(ctx, txn, storage.AddOp, storage.Path{}, data); err != nil {
			t.Fatalf("write data: %v", err)
		}
	}
	if err := store.Commit(ctx, txn); err != nil {
		t.Fatalf("store policy: %v", err)
	}
	return newFixtureWithStore(t, store,
		func(s *Server) {
			_ = s.WithRuntime(ast.MustParseTerm(`{"foo": "bar", "fox": 100}`))
		},
	)
}

func TestCompileHandlerMultiTarget(t *testing.T) {
	t.Parallel()
	var roles map[string]any
	if err := json.Unmarshal(rolesJSON, &roles); err != nil {
		t.Fatalf("unmarshal roles: %v", err)
	}

	f := setup(t, string(benchRego), map[string]any{"roles": roles})

	input := map[string]any{
		"user": "caesar",
		"tenant": map[string]any{
			"id":   2,
			"name": "acmecorp",
		},
	}
	path := "filters/include"
	target := "application/vnd.opa.multitarget+json"

	payload := map[string]any{ // NB(sr): unknowns are taken from metadata
		"input": input,
		"options": map[string]any{
			"targetDialects": []string{
				"sql+postgresql",
				"sql+mysql",
				"sql+sqlserver",
				"sql+sqlite",
				"ucast+prisma",
			},
		},
	}

	expCode := http.StatusOK
	expBody, _ := json.Marshal(map[string]any{
		"result": map[string]any{
			"postgresql": map[string]any{
				"masks": map[string]any{"tickets": map[string]any{"id": map[string]any{"replace": map[string]any{"value": string("***")}}}},
				"query": "WHERE ((tickets.tenant = E'2' AND users.name = E'caesar') OR (tickets.tenant = E'2' AND tickets.assignee IS NULL AND tickets.resolved = FALSE))",
			},
			"mysql": map[string]any{
				"masks": map[string]any{"tickets": map[string]any{"id": map[string]any{"replace": map[string]any{"value": string("***")}}}},
				"query": "WHERE ((tickets.tenant = '2' AND users.name = 'caesar') OR (tickets.tenant = '2' AND tickets.assignee IS NULL AND tickets.resolved = FALSE))",
			},
			"sqlserver": map[string]any{
				"masks": map[string]any{"tickets": map[string]any{"id": map[string]any{"replace": map[string]any{"value": string("***")}}}},
				"query": "WHERE ((tickets.tenant = N'2' AND users.name = N'caesar') OR (tickets.tenant = N'2' AND tickets.assignee IS NULL AND tickets.resolved = FALSE))",
			},
			"sqlite": map[string]any{
				"masks": map[string]any{"tickets": map[string]any{"id": map[string]any{"replace": map[string]any{"value": string("***")}}}},
				"query": "WHERE ((tickets.tenant = '2' AND users.name = 'caesar') OR (tickets.tenant = '2' AND tickets.assignee IS NULL AND tickets.resolved = FALSE))",
			},
			"ucast": map[string]any{
				"masks": map[string]any{"tickets": map[string]any{"id": map[string]any{"replace": map[string]any{"value": string("***")}}}},
				"query": map[string]any{
					"operator": "or",
					"type":     "compound",
					"value": []any{
						map[string]any{
							"operator": "and",
							"type":     "compound",
							"value": []any{
								map[string]any{"field": "tickets.tenant", "operator": "eq", "type": "field", "value": float64(2)},
								map[string]any{"field": "users.name", "operator": "eq", "type": "field", "value": "caesar"},
							},
						},
						map[string]any{
							"operator": "and",
							"type":     "compound",
							"value": []any{
								map[string]any{"field": "tickets.tenant", "operator": "eq", "type": "field", "value": float64(2)},
								map[string]any{"field": "tickets.assignee", "operator": "eq", "type": "field", "value": nil},
								map[string]any{"field": "tickets.resolved", "operator": "eq", "type": "field", "value": false},
							},
						},
					},
				},
			},
		},
	})

	jsonData, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Failed to marshal JSON: %v", err)
	}

	req, _ := http.NewRequest("POST", "/v1/compile/"+path, bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", target)
	if err := f.executeRequest(req, expCode, string(expBody), ignoreMetrics); err != nil {
		t.Error(err)
	}
}

func TestCompileHandlerMetrics(t *testing.T) {
	t.Parallel()
	var roles map[string]any
	if err := json.Unmarshal(rolesJSON, &roles); err != nil {
		t.Fatalf("unmarshal roles: %v", err)
	}

	input := map[string]any{
		"user": "caesar",
		"tenant": map[string]any{
			"id":   2,
			"name": "acmecorp",
		},
	}
	path := "filters/include"
	targets := []string{
		"application/vnd.opa.sql.postgresql+json",
		"application/vnd.opa.ucast.prisma+json",
	}

	for _, target := range targets {
		f := setup(t, string(benchRego), map[string]any{"roles": roles})
		t.Run(strings.Split(target, "/")[1], func(t *testing.T) {
			payload := map[string]any{ // NB(sr): unknowns+mask_rule are taken from metadata
				"input": input,
			}
			{ // check metrics
				req := evalReq(t, path, payload, target)
				if err := f.executeRequest(req, http.StatusOK, ""); err != nil {
					t.Fatal(err)
				}
				var resp Response
				if err := json.NewDecoder(f.recorder.Result().Body).Decode(&resp); err != nil {
					t.Error(err)
				}

				if exp, act := map[string]float64{
					"timer_compile_eval_constraints_ns":             0,
					"timer_compile_eval_mask_rule_ns":               0,
					"timer_compile_extract_annotations_unknowns_ns": 0,
					"timer_compile_extract_annotations_mask_ns":     0,
					"timer_compile_prep_partial_ns":                 0,
					"timer_rego_external_resolve_ns":                0,
					"timer_rego_partial_eval_ns":                    0,
					"timer_rego_query_compile_ns":                   0,
					"timer_rego_query_parse_ns":                     0,
					"timer_rego_query_eval_ns":                      0,
					"timer_server_handler_ns":                       0,
					"timer_compile_translate_queries_ns":            0,
				}, resp.Metrics; !compareMetrics(exp, act) {
					t.Fatalf("unexpected metrics: want %v, got %v", exp, act)
				}
			}

			{ // Redo without resetting the cache: no extraction happens
				req := evalReq(t, path, payload, target)
				if err := f.executeRequest(req, http.StatusOK, ""); err != nil {
					t.Fatal(err)
				}
				var resp Response
				if err := json.NewDecoder(f.recorder.Result().Body).Decode(&resp); err != nil {
					t.Error(err)
				}
				if n, ok := resp.Metrics["timer_compile_extract_annotations_unknowns_ns"]; ok {
					t.Errorf("unexpected metric 'timer_compile_extract_annotations_unknowns_ns': %v", n)
				}
				if n, ok := resp.Metrics["timer_compile_extract_annotations_mask_ns"]; ok {
					t.Errorf("unexpected metric 'timer_compile_extract_annotations_mask_ns': %v", n)
				}
			}
		})
	}
}

// compareMetrics only checks that the keys of `exp` and `act` are the same.
func compareMetrics(exp, act map[string]float64) bool {
	return maps.EqualFunc(exp, act, func(_, _ float64) bool {
		return true
	})
}

func TestCompileHandlerHints(t *testing.T) {
	t.Parallel()
	typoRego := `package filters
# METADATA
# scope: document
# compile:
#   unknowns: [input.fruits]
include if input.fruits.name == "apple"
include if input.fruit.cost < input.max
`
	f := setup(t, typoRego, nil)
	input := map[string]any{
		"max": 1,
	}
	path := "filters/include"
	target := "application/vnd.opa.sql.postgresql+json"

	payload := map[string]any{ // NB(sr): unknowns are taken from metadata
		"input": input,
	}
	req := evalReq(t, path, payload, target)

	expCode := http.StatusOK
	expResp := map[string]any{
		"result": map[string]any{
			"query": "WHERE fruits.name = E'apple'",
		},
		"hints": []map[string]any{
			{
				"location": map[string]any{
					"col":  float64(12),
					"row":  float64(7),
					"file": "filters.rego",
				},
				"message": "input.fruit.cost undefined, did you mean input.fruits.cost?",
			},
		},
	}
	expBodyJSON, _ := json.Marshal(expResp)
	if err := f.executeRequest(req, expCode, string(expBodyJSON), ignoreMetrics); err != nil {
		t.Error(err)
	}
}

func TestCompileHandlerMaskingRules(t *testing.T) {
	t.Parallel()
	var roles map[string]any
	if err := json.Unmarshal(rolesJSON, &roles); err != nil {
		t.Fatalf("unmarshal roles: %v", err)
	}

	input := map[string]any{
		"user": "caesar",
		"tenant": map[string]any{
			"id":   2,
			"name": "acmecorp",
		},
	}
	path := "filters/include"
	target := "application/vnd.opa.sql.postgresql+json"

	t.Run("mask rule from payload parameter", func(t *testing.T) {
		t.Parallel()
		f := setup(t, string(benchRego), map[string]any{"roles": roles})
		payload := map[string]any{ // NB(sr): unknowns are taken from metadata
			"input": input,
			"options": map[string]any{
				"maskRule": "data.filters.masks",
			},
		}
		req := evalReq(t, path, payload, target)

		expBodyJSON, _ := json.Marshal(map[string]any{
			"result": map[string]any{
				"query": "WHERE ((tickets.tenant = E'2' AND users.name = E'caesar') OR (tickets.tenant = E'2' AND tickets.assignee IS NULL AND tickets.resolved = FALSE))",
				"masks": map[string]any{"tickets": map[string]any{"description": map[string]any{"replace": map[string]any{"value": "***"}}}},
			},
		})
		if err := f.executeRequest(req, http.StatusOK, string(expBodyJSON), ignoreMetrics); err != nil {
			t.Error(err)
		}
	})
	t.Run("mask rule from payload parameter + package-local matching", func(t *testing.T) {
		t.Parallel()
		f := setup(t, string(benchRego), map[string]any{"roles": roles})
		payload := map[string]any{ // NB(sr): unknowns are taken from metadata
			"input": input,
			"options": map[string]any{
				"maskRule": "masks",
			},
		}
		req := evalReq(t, path, payload, target)
		expBodyJSON, _ := json.Marshal(map[string]any{
			"result": map[string]any{
				"query": "WHERE ((tickets.tenant = E'2' AND users.name = E'caesar') OR (tickets.tenant = E'2' AND tickets.assignee IS NULL AND tickets.resolved = FALSE))",
				"masks": map[string]any{"tickets": map[string]any{"description": map[string]any{"replace": map[string]any{"value": "***"}}}},
			},
		})
		if err := f.executeRequest(req, http.StatusOK, string(expBodyJSON), ignoreMetrics); err != nil {
			t.Error(err)
		}
	})
	t.Run("mask rule from rule annotation", func(t *testing.T) {
		t.Parallel()
		f := setup(t, string(benchRego), map[string]any{"roles": roles})
		payload := map[string]any{
			"input": input,
		}
		req := evalReq(t, path, payload, target)
		expBodyJSON, _ := json.Marshal(map[string]any{
			"result": map[string]any{
				"query": "WHERE ((tickets.tenant = E'2' AND users.name = E'caesar') OR (tickets.tenant = E'2' AND tickets.assignee IS NULL AND tickets.resolved = FALSE))",
				"masks": map[string]any{"tickets": map[string]any{"id": map[string]any{"replace": map[string]any{"value": "***"}}}},
			},
		})
		if err := f.executeRequest(req, http.StatusOK, string(expBodyJSON), ignoreMetrics); err != nil {
			t.Error(err)
		}
	})
	t.Run("mask rule from rule annotation + package-local matching", func(t *testing.T) {
		t.Parallel()
		// Mangle the mask_rule annotation to make it package-local:
		benchRego := bytes.Replace(benchRego, []byte("mask_rule: data.filters.mask_from_annotation"), []byte("mask_rule: mask_from_annotation"), 1)
		f := setup(t, string(benchRego), map[string]any{"roles": roles})

		payload := map[string]any{
			"input": input,
		}
		req := evalReq(t, path, payload, target)
		expBodyJSON, _ := json.Marshal(map[string]any{
			"result": map[string]any{
				"query": "WHERE ((tickets.tenant = E'2' AND users.name = E'caesar') OR (tickets.tenant = E'2' AND tickets.assignee IS NULL AND tickets.resolved = FALSE))",
				"masks": map[string]any{"tickets": map[string]any{"id": map[string]any{"replace": map[string]any{"value": "***"}}}},
			},
		})
		if err := f.executeRequest(req, http.StatusOK, string(expBodyJSON), ignoreMetrics); err != nil {
			t.Error(err)
		}
	})
}

func TestCompileFiltersRequestUnmarshalMetadata(t *testing.T) {
	t.Parallel()

	body := `{
		"input": {"user": "alice"},
		"unknowns": ["input.resources"],
		"options": {"maskRule": "masks"},
		"snapshot_id": "t|2026-05-13T00:00:00Z|2026-05-13T00:10:00Z|s",
		"com.example.opa/metadata": {"trace_id": "xyz-789"}
	}`

	var req CompileFiltersRequestV1
	if err := json.Unmarshal([]byte(body), &req); err != nil {
		t.Fatal(err)
	}

	if req.Input == nil {
		t.Fatal("expected input")
	}
	if req.Unknowns == nil || len(*req.Unknowns) != 1 || (*req.Unknowns)[0] != "input.resources" {
		t.Fatalf("unexpected unknowns: %v", req.Unknowns)
	}
	if req.Options.MaskRule != "masks" {
		t.Fatalf("unexpected maskRule: %v", req.Options.MaskRule)
	}

	if req.Metadata == nil {
		t.Fatal("expected metadata")
	}
	if req.Metadata["snapshot_id"] != "t|2026-05-13T00:00:00Z|2026-05-13T00:10:00Z|s" {
		t.Fatalf("unexpected snapshot_id: %v", req.Metadata["snapshot_id"])
	}
	md, ok := req.Metadata["com.example.opa/metadata"].(map[string]any)
	if !ok {
		t.Fatal("expected com.example.opa/metadata in metadata")
	}
	if md["trace_id"] != "xyz-789" {
		t.Fatalf("unexpected trace_id: %v", md["trace_id"])
	}

	// Known fields must not appear in metadata
	for _, key := range []string{"input", "unknowns", "options", "query"} {
		if _, ok := req.Metadata[key]; ok {
			t.Fatalf("'%s' should not be in metadata", key)
		}
	}
}

func TestCompileResponseMarshalMetadata(t *testing.T) {
	t.Parallel()

	result := any("WHERE name = 'alice'")
	resp := CompileResponseV1{
		Result:   &result,
		Metadata: map[string]any{"snapshot_id": "t|new-snapshot"},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatal(err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}

	if decoded["snapshot_id"] != "t|new-snapshot" {
		t.Fatalf("expected snapshot_id in response JSON, got %v", decoded["snapshot_id"])
	}
	if decoded["result"] == nil {
		t.Fatal("expected result in response JSON")
	}
}

func TestCompileResponseMarshalMetadataNoOverride(t *testing.T) {
	t.Parallel()

	result := any("WHERE name = 'alice'")
	resp := CompileResponseV1{
		Result: &result,
		Metadata: map[string]any{
			"result":      "should-not-override",
			"snapshot_id": "t|valid",
		},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatal(err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}

	// "result" must not be overridden by metadata
	if decoded["result"] != "WHERE name = 'alice'" {
		t.Fatalf("result should not be overridden, got %v", decoded["result"])
	}
	if decoded["snapshot_id"] != "t|valid" {
		t.Fatalf("expected snapshot_id, got %v", decoded["snapshot_id"])
	}
}

func TestCompileResponseMarshalNoMetadata(t *testing.T) {
	t.Parallel()

	result := any("WHERE name = 'alice'")
	resp := CompileResponseV1{
		Result: &result,
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatal(err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}

	if _, ok := decoded["snapshot_id"]; ok {
		t.Fatal("snapshot_id should not appear without metadata")
	}
}

func TestCompileHandlerRequestMetadata(t *testing.T) {
	t.Parallel()

	rego := `package filters
# METADATA
# scope: document
# compile:
#   unknowns: [input.fruits]
include if input.fruits.name == "apple"
`

	var logged *Info
	f := setup(t, rego, nil)
	f.server = f.server.WithDecisionLoggerWithErr(func(_ context.Context, info *Info) error {
		logged = info
		return nil
	})

	payload := map[string]any{
		"input":                    map[string]any{"max": 1},
		"snapshot_id":              "t|2026-05-13T00:00:00Z|2026-05-13T00:10:00Z|s",
		"com.example.opa/metadata": map[string]any{"trace_id": "abc-123"},
	}

	req := evalReq(t, "filters/include", payload, "application/vnd.opa.sql.postgresql+json")
	if err := f.executeRequest(req, http.StatusOK, ""); err != nil {
		t.Fatal(err)
	}

	if logged == nil {
		t.Fatal("expected decision log entry")
	}
	if logged.Custom == nil {
		t.Fatal("expected Custom in decision log")
	}

	incoming, ok := logged.Custom["request_metadata"].(map[string]any)
	if !ok {
		t.Fatalf("expected request_metadata in Custom, got %v", logged.Custom)
	}

	if incoming["snapshot_id"] != "t|2026-05-13T00:00:00Z|2026-05-13T00:10:00Z|s" {
		t.Fatalf("expected snapshot_id in request_metadata, got %v", incoming["snapshot_id"])
	}

	md, ok := incoming["com.example.opa/metadata"].(map[string]any)
	if !ok {
		t.Fatal("expected com.example.opa/metadata in request_metadata")
	}
	if md["trace_id"] != "abc-123" {
		t.Fatalf("expected trace_id='abc-123', got %v", md["trace_id"])
	}

	// Known fields must not leak into metadata
	for _, key := range []string{"input", "unknowns", "options", "query"} {
		if _, ok := incoming[key]; ok {
			t.Fatalf("'%s' should not appear in request_metadata", key)
		}
	}

	if _, ok := logged.Custom["response_metadata"]; ok {
		t.Fatal("response_metadata should be absent when nothing populates it")
	}
}

func TestCompileHandlerResponseMetadata(t *testing.T) {
	t.Parallel()

	rego := `package filters
# METADATA
# scope: document
# compile:
#   unknowns: [input.fruits]
include if {
    test.set_outgoing()
    input.fruits.name == "apple"
}
`

	var logged *Info
	f := setup(t, rego, nil)
	f.server = f.server.WithDecisionLoggerWithErr(func(_ context.Context, info *Info) error {
		logged = info
		return nil
	})

	payload := map[string]any{
		"input":       map[string]any{"max": 1},
		"snapshot_id": "t|2026-05-13T00:00:00Z|2026-05-13T00:10:00Z|s",
	}

	req := evalReq(t, "filters/include", payload, "application/vnd.opa.sql.postgresql+json")
	if err := f.executeRequest(req, http.StatusOK, ""); err != nil {
		t.Fatal(err)
	}

	// Check the HTTP response body contains response metadata
	var respBody map[string]any
	if err := json.NewDecoder(f.recorder.Result().Body).Decode(&respBody); err != nil {
		t.Fatal(err)
	}

	if respBody["version"] != "1.0" {
		t.Fatalf("expected version='1.0' in response body, got %v", respBody["version"])
	}

	// Check decision log
	if logged == nil {
		t.Fatal("expected decision log entry")
	}
	if logged.Custom == nil {
		t.Fatal("expected Custom in decision log")
	}

	outgoing, ok := logged.Custom["response_metadata"].(map[string]any)
	if !ok {
		t.Fatalf("expected response_metadata in Custom, got %v", logged.Custom)
	}
	if outgoing["version"] != "1.0" {
		t.Fatalf("expected version='1.0' in response_metadata, got %v", outgoing["version"])
	}

	// Request metadata should also be present
	if _, ok := logged.Custom["request_metadata"].(map[string]any); !ok {
		t.Fatal("expected request_metadata in Custom")
	}
}

func evalReq(t testing.TB, path string, payload map[string]any, target string) *http.Request {
	t.Helper()

	jsonData, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Failed to marshal JSON: %v", err)
	}

	// CAVEAT(sr): We're using the httptest machinery to simulate a request, so the actual
	// request path is ignored.
	req, _ := http.NewRequest("POST", fmt.Sprintf("/v1/compile/%s?metrics=true", path), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", target)
	return req
}
