// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package planner

import (
	"os"
	"testing"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/internal/ir"
)

func TestPlannerHelloWorld(t *testing.T) {

	// NOTE(tsandall): These tests are not meant to give comprehensive coverage
	// of the planner. Currently we have a suite of end-to-end tests in the
	// test/wasm/ directory that are specified in YAML, compiled into Wasm, and
	// executed inside of a node program. For the time being, the planner is
	// simple enough that exhaustive unit testing is not as valuable as
	// end-to-end testing. These tests provide a quick sanity check that the
	// planner is not failing on simple inputs.
	tests := []struct {
		note    string
		queries []string
		modules []string
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
		{
			note:    "iteration: chained",
			queries: []string{"input.a[i][j] = 1"},
		},
		{
			note:    "negation",
			queries: []string{"not input.x.y = 1"},
		},
		{
			note:    "array ref pattern match",
			queries: []string{"input.x = [1, [y]]"},
		},
		{
			note:    "arrays pattern match",
			queries: []string{"[x, 3, [2]] = [1, 3, [y]]"},
		},
		{
			note:    "sets",
			queries: []string{"x = {1,2,3}; x[y]"},
		},
		{
			note:    "vars",
			queries: []string{"x = 1"},
		},
		{
			note:    "complete rules",
			queries: []string{"true"},
			modules: []string{`
				package test
				p = x { x = 1 }
				p = y { y = 2 }
			`},
		},
		{
			note:    "complete rule reference",
			queries: []string{"data.test.p = 10"},
			modules: []string{`
				package test
				p = x { x = 10 }
			`},
		},
		{
			note:    "functions",
			queries: []string{"data.test.f([1,x])"},
			modules: []string{`
				package test
				f([a, b]) {
					a = b
				}
			`},
		},
		{
			note:    "else",
			queries: []string{"data.test.p = 1"},
			modules: []string{`
				package test
				p = 0 {
					false
				} else = 1 {
					true
				}
			`},
		},
		{
			note:    "partial set",
			queries: []string{"data.test.p = {1,2}"},
			modules: []string{`
				package test
				p[1]
				p[2]
			`},
		},
		{
			note:    "partial object",
			queries: []string{`data.test.p = {"a": 1, "b": 2}`},
			modules: []string{`
				package test
				p["a"] = 1
				p["b"] = 2
			`},
		},
		{
			note:    "virtual extent",
			queries: []string{`data`},
			modules: []string{`
				package test

				p = 1
				q = 2 { false }
			`},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			queries := make([]ast.Body, len(tc.queries))
			for i := range queries {
				queries[i] = ast.MustParseBody(tc.queries[i])
			}
			modules := make([]*ast.Module, len(tc.modules))
			for i := range modules {
				modules[i] = ast.MustParseModule(tc.modules[i])
			}
			planner := New().WithQueries(queries).WithModules(modules)
			policy, err := planner.Plan()
			if err != nil {
				t.Fatal(err)
			}
			_ = policy
			ir.Pretty(os.Stderr, policy)
		})
	}
}
