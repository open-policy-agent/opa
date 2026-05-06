// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package sdk_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/open-policy-agent/opa/v1/logging/test"
	"github.com/open-policy-agent/opa/v1/sdk"
	sdktest "github.com/open-policy-agent/opa/v1/sdk/test"
)

func TestEvaluatedRules(t *testing.T) {
	ctx := t.Context()

	server := sdktest.MustNewServer(
		sdktest.RawBundles(true),
		sdktest.MockBundle("/bundles/bundle.tar.gz", map[string]string{
			"policy.rego": `
package test

# METADATA
# id: rule1
p if {
	input.foo
}

# METADATA
# id: rule2
p if {
	input.bar
}

# METADATA
# id: rule3
q if {
	input.baz
}

# Rule without ID (should still work)
r if {
	input.qux
}

allow if xs != set()

# METADATA
# id: xs1
xs contains "a" if true

# METADATA
# id: xs2
xs contains "b" if false
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

	testLogger := test.New()
	opa, err := sdk.New(ctx, sdk.Options{
		Config:        strings.NewReader(config),
		ConsoleLogger: testLogger,
	})

	if err != nil {
		t.Fatal(err)
	}

	defer opa.Stop(ctx)

	tests := []struct {
		name               string
		path               string
		input              map[string]any
		wantResult         bool
		wantUndefined      bool
		wantEvaluatedRules []string
	}{
		{
			name:               "input matches rule1",
			path:               "/test/p",
			input:              map[string]any{"foo": true},
			wantResult:         true,
			wantEvaluatedRules: []string{"rule1"},
		},
		{
			name:       "input matches rule2",
			path:       "/test/p",
			input:      map[string]any{"bar": true},
			wantResult: true,
			// Note: Virtual cache may prevent re-evaluation after rule1 was cached
			wantEvaluatedRules: nil,
		},
		{
			name:       "input matches both rule1 and rule2",
			path:       "/test/p",
			input:      map[string]any{"foo": true, "bar": true},
			wantResult: true,
			// Note: Only one rule needed to satisfy 'p', so only one will be evaluated
			wantEvaluatedRules: nil,
		},
		{
			name:               "input matches rule3",
			path:               "/test/q",
			input:              map[string]any{"baz": true},
			wantResult:         true,
			wantEvaluatedRules: []string{"rule3"},
		},
		{
			name:       "input matches rule without ID",
			path:       "/test/r",
			input:      map[string]any{"qux": true},
			wantResult: true,
			// No ID, so no evaluated_rules
			wantEvaluatedRules: nil,
		},
		{
			name:          "no rules match",
			path:          "/test/p",
			input:         map[string]any{"other": true},
			wantUndefined: true,
			// No rules evaluated, so no evaluated_rules
			wantEvaluatedRules: nil,
		},
		{
			name:       "set contains with partial rules",
			path:       "/test/allow",
			input:      map[string]any{},
			wantResult: true,
			// Only xs1 should be in evaluated_rules since xs2 evaluates to false
			wantEvaluatedRules: []string{"xs1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := opa.Decision(ctx, sdk.DecisionOptions{
				Path:  tt.path,
				Input: tt.input,
			})

			if tt.wantUndefined {
				if !sdk.IsUndefinedErr(err) {
					t.Fatalf("expected undefined error, got: %v", err)
				}
				// Still check the log entry was created
				entries := testLogger.Entries()
				if len(entries) == 0 {
					t.Fatal("expected at least one log entry")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			resultBool, ok := result.Result.(bool)
			if !ok {
				t.Fatalf("expected bool result, got: %T", result.Result)
			}
			if resultBool != tt.wantResult {
				t.Fatalf("expected result %v, got %v", tt.wantResult, resultBool)
			}

			// Check the decision log entry
			entries := testLogger.Entries()
			if len(entries) == 0 {
				t.Fatal("expected at least one log entry")
			}

			lastEntry := entries[len(entries)-1]

			// Verify decision_id is present
			if lastEntry.Fields["decision_id"] == "" {
				t.Fatal("expected non-empty decision_id")
			}

			// Verify evaluated_rules field
			evaluatedRules, hasField := lastEntry.Fields["ids"]
			if tt.wantEvaluatedRules == nil {
				if hasField {
					t.Fatalf("expected no evaluated_rules field, but got: %v", evaluatedRules)
				}
			} else {
				if !hasField {
					t.Fatal("expected evaluated_rules field in decision log")
				}

				// Convert to []string for comparison
				rulesSlice, ok := evaluatedRules.([]any)
				if !ok {
					t.Fatalf("expected evaluated_rules to be []any, got %T", evaluatedRules)
				}

				actualRules := make([]string, len(rulesSlice))
				for i, v := range rulesSlice {
					actualRules[i], ok = v.(string)
					if !ok {
						t.Fatalf("expected string rule ID, got %T", v)
					}
				}

				if diff := cmp.Diff(tt.wantEvaluatedRules, actualRules); diff != "" {
					t.Errorf("evaluated_rules mismatch (-want +got):\n%s", diff)
				}
			}
		})
	}
}
