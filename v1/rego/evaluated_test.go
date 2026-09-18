// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package rego

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/topdown"
)

func partialWithTracker(t *testing.T, src string, unknowns []string, input map[string]any) *topdown.EvaluatedRuleTracker {
	t.Helper()

	tracker := &topdown.EvaluatedRuleTracker{}
	pq, err := New(
		Query("data.test.allow"),
		ParsedModule(ast.MustParseModuleWithOpts(src, ast.ParserOptions{ProcessAnnotation: true})),
		Unknowns(unknowns),
		EvaluatedRuleTracker(tracker),
	).PrepareForPartial(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pq.Partial(t.Context(), EvalInput(input)); err != nil {
		t.Fatal(err)
	}
	return tracker
}

// topdown.Query.PartialRun resolves an annotation set for the tracker, but the
// rego layer used to build its partial query without passing one, so the
// EvaluatedRuleTracker options were silently ignored. Rules resolved during
// partial evaluation are now recorded, as they are during evaluation.
func TestEvaluatedRuleTrackerPartial(t *testing.T) {
	const module = `package test

# METADATA
# labels:
#   id: allow-admin
allow if input.role == "admin"
`

	tracker := partialWithTracker(t, module, []string{"input.resource"}, map[string]any{"role": "admin"})

	exp := []map[string]any{{"id": "allow-admin"}}
	if diff := cmp.Diff(exp, tracker.Labels, cmpopts.EquateEmpty()); diff != "" {
		t.Errorf("unexpected labels (-want, +got):\n%s", diff)
	}
}

// The per-evaluation tracker takes precedence over the one set on the Rego
// object, matching how the eval path resolves the two.
func TestEvaluatedRuleTrackerPartialEvalOptionWins(t *testing.T) {
	module := ast.MustParseModuleWithOpts(`package test

# METADATA
# labels:
#   id: allow-admin
allow if input.role == "admin"
`, ast.ParserOptions{ProcessAnnotation: true})

	onRego := &topdown.EvaluatedRuleTracker{}
	onEval := &topdown.EvaluatedRuleTracker{}

	pq, err := New(
		Query("data.test.allow"),
		ParsedModule(module),
		Unknowns([]string{"input.resource"}),
		EvaluatedRuleTracker(onRego),
	).PrepareForPartial(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pq.Partial(t.Context(),
		EvalInput(map[string]any{"role": "admin"}),
		EvalEvaluatedRuleTracker(onEval)); err != nil {
		t.Fatal(err)
	}

	if len(onEval.Labels) != 1 {
		t.Fatalf("eval-scoped tracker: got %v, want one label set", onEval.Labels)
	}
	if len(onRego.Labels) != 0 {
		t.Fatalf("rego-scoped tracker should be unused: got %v", onRego.Labels)
	}
}

// Rules left as residuals are recorded too: the label identifies a rule that
// contributed to the result, and under partial evaluation a residual disjunct
// is a contribution. Rules that cannot match are still left out.
func TestEvaluatedRuleTrackerPartialResidual(t *testing.T) {
	const module = `package test

# METADATA
# labels:
#   id: allow-x
allow if input.resource == "x"

# METADATA
# labels:
#   id: allow-y
allow if input.resource == "y"

# METADATA
# labels:
#   id: allow-nobody
allow if input.role == "nobody"
`

	got := partialWithTracker(t, module, []string{"input.resource"}, map[string]any{"role": "admin"}).Labels

	exp := []map[string]any{{"id": "allow-x"}, {"id": "allow-y"}}
	less := func(a, b map[string]any) bool { return a["id"].(string) < b["id"].(string) }
	if diff := cmp.Diff(exp, got, cmpopts.SortSlices(less), cmpopts.EquateEmpty()); diff != "" {
		t.Errorf("unexpected labels (-want, +got):\n%s", diff)
	}
}

// A default rule sends partial evaluation down the support-module path; the
// rules folded into that support module are recorded as well.
func TestEvaluatedRuleTrackerPartialSupport(t *testing.T) {
	const module = `package test

default allow := false

# METADATA
# labels:
#   id: allow-x
allow if input.resource == "x"
`

	got := partialWithTracker(t, module, []string{"input.resource"}, map[string]any{}).Labels

	exp := []map[string]any{{"id": "allow-x"}}
	if diff := cmp.Diff(exp, got, cmpopts.EquateEmpty()); diff != "" {
		t.Errorf("unexpected labels (-want, +got):\n%s", diff)
	}
}
