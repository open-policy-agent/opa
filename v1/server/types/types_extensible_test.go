// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package types

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/v1/util"
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

func TestDataRequestV1_MarshalRoundTrip(t *testing.T) {
	src := `{"input":{"user":"alice"},"trace_id":"abc","tenant":{"id":"t-1"}}`

	var req DataRequestV1
	if err := json.Unmarshal([]byte(src), &req); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	input, ok := got["input"].(map[string]any)
	if !ok || input["user"] != "alice" {
		t.Errorf("input not preserved: %v", got["input"])
	}
	if got["trace_id"] != "abc" {
		t.Errorf("extra trace_id not preserved: %v", got["trace_id"])
	}
	tenant, ok := got["tenant"].(map[string]any)
	if !ok || tenant["id"] != "t-1" {
		t.Errorf("extra tenant not preserved: %v", got["tenant"])
	}
}

func TestDataRequestV1_MarshalDoesNotOverrideInput(t *testing.T) {
	inp := any(map[string]any{"user": "alice"})
	req := DataRequestV1{
		Input:    &inp,
		Metadata: map[string]any{"input": "should_be_ignored"},
	}

	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	input, ok := got["input"].(map[string]any)
	if !ok || input["user"] != "alice" {
		t.Errorf("input overridden by metadata: %v", got["input"])
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
		"rule_labels",
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

func TestTopLevelKeysKnown(t *testing.T) {
	known := map[string]bool{"input": true, "decision_id": true}
	tests := []struct {
		data string
		want bool
	}{
		{`{}`, true},
		{`null`, true},
		{`{"input": {"user": "alice"}}`, true},
		{`{"input": 1, "decision_id": "x"}`, true},
		{`{"input": {"extra": {"deep": [{"other": 1}]}}}`, true},
		{`{"input": ["a", {"b": "c"}, "d"], "decision_id": "x"}`, true},
		{`{"input": "has \"quotes\" and {braces}, commas"}`, true},
		{`{"input": "\\", "decision_id": "x"}`, true},
		{`{"input": {}, "request_id": "123"}`, false},
		{`{"request_id": "123", "input": {}}`, false},
		{`{"input": [1, 2], "request_id": "123"}`, false},
		{`{"Input": {}}`, false},
		{`{"\u0069nput": {}}`, false},
		{`{"input": "unterminated`, false},
	}
	for _, tc := range tests {
		t.Run(tc.data, func(t *testing.T) {
			if got := topLevelKeysKnown([]byte(tc.data), known); got != tc.want {
				t.Errorf("topLevelKeysKnown(%s) = %v, want %v", tc.data, got, tc.want)
			}
		})
	}
}

func TestDataRequestV1_EscapedKeyExtras(t *testing.T) {
	var req DataRequestV1
	if err := json.Unmarshal([]byte(`{"input": {}, "request\u005fid": "123"}`), &req); err != nil {
		t.Fatal(err)
	}
	if req.Metadata["request_id"] != "123" {
		t.Errorf("expected request_id='123' in Metadata, got %v", req.Metadata)
	}
}

// no_extras/10-16       94254     12804 ns/op     18344 B/op    216 allocs/op // decoding extras on every call
// extras/10-16          88120     13597 ns/op     21176 B/op    228 allocs/op // decoding extras on every call
// no_extras/1000-16       992   1209217 ns/op   1731869 B/op  17070 allocs/op // decoding extras on every call
// extras/1000-16          991   1210239 ns/op   1734701 B/op  17082 allocs/op // decoding extras on every call
// no_extras/10-16      121662      9955 ns/op     14600 B/op    197 allocs/op // scanning top-level keys first
// extras/10-16          85192     14239 ns/op     21176 B/op    228 allocs/op // scanning top-level keys first
// no_extras/1000-16      1324    905507 ns/op   1396978 B/op  17045 allocs/op // scanning top-level keys first
// extras/1000-16          928   1258173 ns/op   1734700 B/op  17082 allocs/op // scanning top-level keys first
func BenchmarkDataRequestV1Unmarshal(b *testing.B) {
	for _, n := range []int{10, 1000} {
		var sb strings.Builder
		sb.WriteString(`{"input":{"items":[`)
		for i := range n {
			if i > 0 {
				sb.WriteByte(',')
			}
			fmt.Fprintf(&sb, `{"id":%d,"name":"user-%d","roles":["a","b"],"attrs":{"x":true}}`, i, i)
		}
		sb.WriteString(`]}`)
		noExtras := []byte(sb.String() + `}`)
		withExtras := []byte(sb.String() + `,"request_id":"123"}`)

		for name, body := range map[string][]byte{"no_extras": noExtras, "extras": withExtras} {
			b.Run(fmt.Sprintf("%s/%d", name, n), func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					var req DataRequestV1
					if err := util.UnmarshalJSON(body, &req); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}
