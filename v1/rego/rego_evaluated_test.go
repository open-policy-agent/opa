package rego

import (
	"testing"

	"github.com/open-policy-agent/opa/v1/topdown"
)

func TestEvaluatedRules(t *testing.T) {
	module := `package test

# METADATA
# id: rule1
p if input.foo

# METADATA
# id: rule2
p if input.bar`

	t.Run("records matching rule", func(t *testing.T) {
		tracker := &topdown.EvaluatedRuleTracker{}
		rs, err := New(
			Query("data.test.p"),
			Module("test.rego", module),
			Input(map[string]any{"foo": true}),
			EvaluatedRuleTracker(tracker),
		).Eval(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		if len(rs) == 0 {
			t.Fatal("expected result")
		}
		if len(tracker.IDs) != 1 || tracker.IDs[0] != "rule1" {
			t.Fatalf("expected [rule1], got %v", tracker.IDs)
		}
	})

	t.Run("no match yields empty", func(t *testing.T) {
		tracker := &topdown.EvaluatedRuleTracker{}
		_, _ = New(
			Query("data.test.p"),
			Module("test.rego", module),
			Input(map[string]any{"baz": true}),
			EvaluatedRuleTracker(tracker),
		).Eval(t.Context())

		if len(tracker.IDs) != 0 {
			t.Fatalf("expected empty, got %v", tracker.IDs)
		}
	})

	t.Run("nil tracker is safe", func(t *testing.T) {
		_, err := New(
			Query("data.test.p"),
			Module("test.rego", module),
			Input(map[string]any{"foo": true}),
			EvaluatedRuleTracker(nil),
		).Eval(t.Context())
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("prepared query", func(t *testing.T) {
		pq, err := New(
			Query("data.test.p"),
			Module("test.rego", module),
		).PrepareForEval(t.Context())
		if err != nil {
			t.Fatal(err)
		}

		tracker := &topdown.EvaluatedRuleTracker{}
		rs, err := pq.Eval(t.Context(),
			EvalInput(map[string]any{"bar": true}),
			EvalEvaluatedRuleTracker(tracker),
		)
		if err != nil {
			t.Fatal(err)
		}
		if len(rs) == 0 {
			t.Fatal("expected result")
		}
		if len(tracker.IDs) != 1 || tracker.IDs[0] != "rule2" {
			t.Fatalf("expected [rule2], got %v", tracker.IDs)
		}
	})
}
