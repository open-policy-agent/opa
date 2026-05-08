// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown_test

import (
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
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
