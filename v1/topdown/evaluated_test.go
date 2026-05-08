// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/storage"
	inmem "github.com/open-policy-agent/opa/v1/storage/inmem/test"
	"github.com/open-policy-agent/opa/v1/topdown"
)

func TestEvaluatedRuleTracker(t *testing.T) {
	t.Run("collects labels", func(t *testing.T) {
		tracker := &topdown.EvaluatedRuleTracker{}
		tracker.Record(&ast.Rule{
			Annotations: []*ast.Annotations{
				{Labels: map[string]any{"severity": "low"}},
			},
		})
		if len(tracker.Labels) != 1 {
			t.Fatalf("expected 1 label set, got %d", len(tracker.Labels))
		}
		if tracker.Labels[0]["severity"] != "low" {
			t.Fatalf("expected severity=low, got %v", tracker.Labels[0])
		}
	})

	t.Run("deduplicates identical labels", func(t *testing.T) {
		tracker := &topdown.EvaluatedRuleTracker{}
		labels := map[string]any{"team": "platform", "severity": "high"}
		tracker.Record(&ast.Rule{
			Annotations: []*ast.Annotations{{Labels: labels}},
		})
		tracker.Record(&ast.Rule{
			Annotations: []*ast.Annotations{{Labels: labels}},
		})
		if len(tracker.Labels) != 1 {
			t.Fatalf("expected 1 label set (deduplicated), got %d", len(tracker.Labels))
		}
	})

	t.Run("keeps distinct labels", func(t *testing.T) {
		tracker := &topdown.EvaluatedRuleTracker{}
		tracker.Record(&ast.Rule{
			Annotations: []*ast.Annotations{
				{Labels: map[string]any{"severity": "low"}},
			},
		})
		tracker.Record(&ast.Rule{
			Annotations: []*ast.Annotations{
				{Labels: map[string]any{"severity": "high"}},
			},
		})
		if len(tracker.Labels) != 2 {
			t.Fatalf("expected 2 label sets, got %d", len(tracker.Labels))
		}
	})

	t.Run("nil tracker is safe", func(t *testing.T) {
		var tracker *topdown.EvaluatedRuleTracker
		tracker.Record(&ast.Rule{
			Annotations: []*ast.Annotations{
				{Labels: map[string]any{"x": "y"}},
			},
		})
	})

	t.Run("skips rules without labels", func(t *testing.T) {
		tracker := &topdown.EvaluatedRuleTracker{}
		tracker.Record(&ast.Rule{
			Annotations: []*ast.Annotations{
				{Custom: map[string]any{"foo": "bar"}},
			},
		})
		if len(tracker.Labels) != 0 {
			t.Fatalf("expected 0 label sets, got %d", len(tracker.Labels))
		}
	})

	t.Run("skips rules without annotations", func(t *testing.T) {
		tracker := &topdown.EvaluatedRuleTracker{}
		tracker.Record(&ast.Rule{})
		if len(tracker.Labels) != 0 {
			t.Fatalf("expected 0 label sets, got %d", len(tracker.Labels))
		}
	})
}

func TestEvaluatedRuleLabelsScopes(t *testing.T) {
	tests := []struct {
		note   string
		module string
		query  string
		input  string
		exp    []map[string]any
	}{
		{
			note: "rule scope labels",
			module: `package test

# METADATA
# labels:
#   severity: high
#   team: security
allow if input.role == "admin"
`,
			query: "data.test.allow",
			exp: []map[string]any{
				{"severity": "high", "team": "security"},
			},
		},
		{
			note: "document scope labels apply to all rules in document",
			module: `package test

# METADATA
# scope: document
# labels:
#   component: authz
allow if input.role == "admin"

allow if input.role == "superuser"
`,
			query: "data.test.allow",
			exp: []map[string]any{
				{"component": "authz"},
			},
		},
		{
			note: "rule and document scope combine",
			module: `package test

# METADATA
# scope: document
# labels:
#   component: authz

# METADATA
# labels:
#   severity: high
allow if input.role == "admin"

# METADATA
# labels:
#   severity: low
allow if input.role == "viewer"
`,
			query: "data.test.allow",
			exp: []map[string]any{
				{"component": "authz"},
				{"severity": "high"},
			},
		},
		{
			note: "no labels when rule not satisfied",
			module: `package test

# METADATA
# labels:
#   severity: high
allow if input.role == "admin"
`,
			query: "data.test.allow",
			input: `{"role": "guest"}`,
			exp:   nil,
		},
		{
			note: "multiple rules each contribute labels",
			module: `package test

# METADATA
# labels:
#   id: allow-admin
allow if input.role == "admin"

# METADATA
# labels:
#   id: allow-viewer
allow if input.role == "viewer"
`,
			query: "data.test.allow",
			exp: []map[string]any{
				{"id": "allow-admin"},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			mod := ast.MustParseModuleWithOpts(tc.module, ast.ParserOptions{ProcessAnnotation: true})
			c := ast.NewCompiler()
			c.Compile(map[string]*ast.Module{"test": mod})
			if c.Failed() {
				t.Fatal(c.Errors)
			}

			inputStr := tc.input
			if inputStr == "" {
				inputStr = `{"role": "admin"}`
			}
			input := ast.MustParseTerm(inputStr)
			store := inmem.New()
			ctx := t.Context()
			txn := storage.NewTransactionOrDie(ctx, store)
			defer store.Abort(ctx, txn)

			tracker := &topdown.EvaluatedRuleTracker{}
			query := topdown.NewQuery(ast.MustParseBody(tc.query)).
				WithCompiler(c).
				WithStore(store).
				WithTransaction(txn).
				WithInput(input).
				WithEvaluatedRuleTracker(tracker)

			_, err := query.Run(ctx)
			if err != nil {
				t.Fatal(err)
			}

			if diff := cmp.Diff(tc.exp, tracker.Labels, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("unexpected labels (-want, +got):\n%s", diff)
			}
		})
	}
}
