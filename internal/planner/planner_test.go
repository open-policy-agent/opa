// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package planner

import (
	"testing"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/internal/ir"
)

func TestPlannerHelloWorld(t *testing.T) {

	tests := []struct {
		note    string
		queries []string
		exp     ir.Policy
	}{
		{
			note:    "empty",
			queries: []string{},
		},
		{
			note:    "hello world",
			queries: []string{"input.a = 1"},
		},
		{
			note:    "conjunction",
			queries: []string{"1 = 1; 2 = 2"},
		},
		{
			note:    "disjunction",
			queries: []string{"input.a = 1", "input.b = 2"},
		},
		{
			note:    "iteration",
			queries: []string{"input.a[i] = 1; input.b = 2"},
		},
		{
			note:    "iteration: compare key",
			queries: []string{"input.a[i] = 1; input.b = i"},
		},
		{
			note:    "iteration: nested",
			queries: []string{"input.a[i] = 1; input.b[j] = 2"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			queries := make([]ast.Body, len(tc.queries))
			for i := range queries {
				queries[i] = ast.MustParseBody(tc.queries[i])
			}
			planner := New().WithQueries(queries)
			policy, err := planner.Plan()
			if err != nil {
				t.Fatal(err)
			}
			_ = policy
		})
	}
}
