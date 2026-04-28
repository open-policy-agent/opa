// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package types

import (
	"encoding/json"
	"testing"
)

func TestDataRequestV1_ExtraFields(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantErr  bool
		validate func(*testing.T, *DataRequestV1)
	}{
		{
			name:  "basic request with extra fields",
			input: `{"input": {"user": "alice"}, "request_id": "123", "metadata": {"source": "test"}}`,
			validate: func(t *testing.T, req *DataRequestV1) {
				if req.Input == nil {
					t.Error("Input should not be nil")
				}
				if len(req.Metadata) != 2 {
					t.Errorf("Expected 2 extra fields, got %d", len(req.Metadata))
				}
				if req.Metadata["request_id"] != "123" {
					t.Errorf("Expected request_id='123', got %v", req.Metadata["request_id"])
				}
				metadata, ok := req.Metadata["metadata"].(map[string]any)
				if !ok {
					t.Error("metadata should be a map")
				} else if metadata["source"] != "test" {
					t.Errorf("Expected metadata.source='test', got %v", metadata["source"])
				}
			},
		},
		{
			name:  "request with no extra fields",
			input: `{"input": {"user": "bob"}}`,
			validate: func(t *testing.T, req *DataRequestV1) {
				if req.Input == nil {
					t.Error("Input should not be nil")
				}
				if req.Metadata != nil {
					t.Errorf("Extra should be nil when no extra fields, got %v", req.Metadata)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req DataRequestV1
			err := json.Unmarshal([]byte(tt.input), &req)
			if (err != nil) != tt.wantErr {
				t.Errorf("Unmarshal() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && tt.validate != nil {
				tt.validate(t, &req)
			}
		})
	}
}

func TestDataResponseV1_ExtraFields(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantErr  bool
		validate func(*testing.T, *DataResponseV1)
	}{
		{
			name:  "response with extra fields",
			input: `{"result": true, "decision_id": "abc123", "custom_field": "value", "trace_id": "xyz"}`,
			validate: func(t *testing.T, resp *DataResponseV1) {
				if resp.DecisionID != "abc123" {
					t.Errorf("Expected decision_id='abc123', got %s", resp.DecisionID)
				}
				if len(resp.Metadata) != 2 {
					t.Errorf("Expected 2 extra fields, got %d", len(resp.Metadata))
				}
				if resp.Metadata["custom_field"] != "value" {
					t.Errorf("Expected custom_field='value', got %v", resp.Metadata["custom_field"])
				}
				if resp.Metadata["trace_id"] != "xyz" {
					t.Errorf("Expected trace_id='xyz', got %v", resp.Metadata["trace_id"])
				}
			},
		},
		{
			name:  "response with no extra fields",
			input: `{"result": {"allowed": true}, "decision_id": "def456"}`,
			validate: func(t *testing.T, resp *DataResponseV1) {
				if resp.DecisionID != "def456" {
					t.Errorf("Expected decision_id='def456', got %s", resp.DecisionID)
				}
				if resp.Metadata != nil {
					t.Errorf("Extra should be nil when no extra fields, got %v", resp.Metadata)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var resp DataResponseV1
			err := json.Unmarshal([]byte(tt.input), &resp)
			if (err != nil) != tt.wantErr {
				t.Errorf("Unmarshal() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && tt.validate != nil {
				tt.validate(t, &resp)
			}
		})
	}
}

func TestDataResponseV1_RoundTrip(t *testing.T) {
	input := `{"result": {"allowed": true}, "decision_id": "xyz", "custom": "data", "metrics": {"timer_rego_query_eval_ns": 1000}}`

	var resp DataResponseV1
	if err := json.Unmarshal([]byte(input), &resp); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	output, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var resp2 DataResponseV1
	if err := json.Unmarshal(output, &resp2); err != nil {
		t.Fatalf("Second unmarshal failed: %v", err)
	}

	if resp.DecisionID != resp2.DecisionID {
		t.Errorf("decision_id not preserved: %s != %s", resp.DecisionID, resp2.DecisionID)
	}

	if resp.Metadata["custom"] != resp2.Metadata["custom"] {
		t.Errorf("custom field not preserved: %v != %v", resp.Metadata["custom"], resp2.Metadata["custom"])
	}

	if resp.Metrics["timer_rego_query_eval_ns"] != resp2.Metrics["timer_rego_query_eval_ns"] {
		t.Error("metrics not preserved")
	}
}

func TestDataResponseV1_ReservedFieldsNotOverridden(t *testing.T) {
	reservedFields := []string{
		"decision_id",
		"provenance",
		"explanation",
		"metrics",
		"result",
		"warning",
	}

	for _, field := range reservedFields {
		t.Run(field, func(t *testing.T) {
			resp := DataResponseV1{
				DecisionID: "real_decision_id",
				Metadata: map[string]any{
					field: "should_be_ignored",
				},
			}

			if field == "result" {
				result := any("real_result")
				resp.Result = &result
			}

			output, err := json.Marshal(resp)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var raw map[string]any
			if err := json.Unmarshal(output, &raw); err != nil {
				t.Fatalf("Unmarshal to map failed: %v", err)
			}

			if field == "decision_id" && raw["decision_id"] != "real_decision_id" {
				t.Errorf("Reserved field '%s' was overridden by extra field", field)
			}
			if field == "result" && raw["result"] != "real_result" {
				t.Errorf("Reserved field 'result' was overridden by extra field, got: %v", raw["result"])
			}

			if field != "decision_id" && field != "result" {
				if _, exists := raw[field]; exists && raw[field] == "should_be_ignored" {
					t.Errorf("Reserved field '%s' from extra should have been ignored but appeared in output", field)
				}
			}
		})
	}
}
