// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"context"
	"testing"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/storage/inmem"
)

func TestQueryTracerDontPlugLocalVars(t *testing.T) {

	cases := []struct {
		note         string
		tracerConfs  []TraceConfig
		expectLocals bool
	}{
		{
			note: "plug locals single tracer",
			tracerConfs: []TraceConfig{
				{PlugLocalVars: true},
			},
			expectLocals: true,
		},
		{
			note: "dont plug locals single tracer",
			tracerConfs: []TraceConfig{
				{PlugLocalVars: false},
			},
			expectLocals: false,
		},
		{
			note: "plug locals multiple tracers",
			tracerConfs: []TraceConfig{
				{PlugLocalVars: true},
				{PlugLocalVars: true},
				{PlugLocalVars: true},
			},
			expectLocals: true,
		},
		{
			note: "dont plug locals multiple tracers",
			tracerConfs: []TraceConfig{
				{PlugLocalVars: false},
				{PlugLocalVars: false},
				{PlugLocalVars: false},
			},
			expectLocals: false,
		},
		{
			note: "plug locals multiple plugins mixed",
			tracerConfs: []TraceConfig{
				{PlugLocalVars: false},
				{PlugLocalVars: true},
				{PlugLocalVars: false},
			},
			expectLocals: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {

			ctx := context.Background()
			store := inmem.New()
			inputTerm := &ast.Term{}
			txn := storage.NewTransactionOrDie(ctx, store)
			defer store.Abort(ctx, txn)

			compiler := compileModules([]string{
				`package x
	
	p {
		a := [1, 2, 3]
		f(a[_])
	}
	
	f(x) {
		x == 3
	}
	
	`})

			query := NewQuery(ast.MustParseBody("data.x.p")).
				WithCompiler(compiler).
				WithStore(store).
				WithTransaction(txn).
				WithInput(inputTerm)

			var tracers []*testTracer
			for _, conf := range tc.tracerConfs {
				tt := &testTracer{
					events: []*Event{},
					conf:   conf,
				}
				tracers = append(tracers, tt)
				query = query.WithTracer(tt)
			}

			_, err := query.Run(ctx)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			// Even if the individual tracer didn't specify for local metadata
			// they will _all_ either have it or not.
			for _, tt := range tracers {
				for _, e := range tt.events {
					if !tc.expectLocals && e.LocalMetadata != nil {
						t.Fatalf("Expected event LocalMetadata to nil")
					}
					if tc.expectLocals && e.LocalMetadata == nil {
						t.Fatalf("Expected event LocalMetadata to be non-nil")
					}
				}
			}
		})
	}
}

type testTracer struct {
	events []*Event
	conf   TraceConfig
}

func (n *testTracer) Enabled() bool {
	return true
}

func (n *testTracer) Trace(e *Event) {
	n.events = append(n.events, e)
}

func (n *testTracer) Config() TraceConfig {
	return n.conf
}
