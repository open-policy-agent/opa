// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"context"
	"reflect"
	"testing"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/storage"
	inmem "github.com/open-policy-agent/opa/storage/inmem/test"
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

			query := initTracerTestQuery()

			var tracers []*testQueryTracer
			for _, conf := range tc.tracerConfs {
				tt := &testQueryTracer{
					events:  []*Event{},
					conf:    conf,
					enabled: true,
					t:       t,
				}
				tracers = append(tracers, tt)

				query = query.WithQueryTracer(tt)
			}

			_, err := query.Run(context.Background())
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

func TestLegacyTracerUpgrade(t *testing.T) {

	query := initTracerTestQuery()

	tracer := &testQueryTracer{
		events:  []*Event{},
		conf:    TraceConfig{PlugLocalVars: false},
		enabled: true,
		t:       t,
	}

	// Call with older API, expect to be "upgraded" to QueryTracer
	// If the deprecated Trace() API is called the test will fail.
	query.WithTracer(tracer)

	_, err := query.Run(context.Background())
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
}

func TestLegacyTracerBackwardsCompatibility(t *testing.T) {
	t.Helper()
	query := initTracerTestQuery()

	// Using a tracer that does _not_ implement the newer
	// QueryTracer interface, only the deprecated Tracer one.
	tracer := &testLegacyTracer{
		events: []*Event{},
	}

	query.WithTracer(tracer)

	// For comparison use a buffer tracer and the new interface
	bt := NewBufferTracer()
	query.WithQueryTracer(bt)

	_, err := query.Run(context.Background())
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(*bt) != len(tracer.events) {
		t.Fatalf("Expected %d events on the test tracer, got %d", len(*bt), len(tracer.events))
	}

	if !reflect.DeepEqual([]*Event(*bt), tracer.events) {
		t.Fatalf("Expected same events on test tracer and BufferTracer")
	}
}

func TestDisabledTracer(t *testing.T) {
	query := initTracerTestQuery()

	tracer := &testQueryTracer{
		events:  []*Event{},
		conf:    TraceConfig{PlugLocalVars: false},
		enabled: false,
		t:       t,
	}

	// Both API's should ignore the disabled tracer
	query.WithTracer(tracer)
	query.WithQueryTracer(tracer)

	_, err := query.Run(context.Background())
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(tracer.events) > 0 {
		t.Fatalf("Expected no events on test tracer, got %d", len(tracer.events))
	}
}

func TestRegoMetadataBuiltinCall(t *testing.T) {
	tests := []struct {
		note          string
		query         string
		expectedError string
	}{
		{
			note:          "rego.metadata.chain() call",
			query:         "rego.metadata.chain()",
			expectedError: "rego.metadata.chain(): eval_builtin_error: rego.metadata.chain: the rego.metadata.chain function must only be called within the scope of a rule",
		},
		{
			note:          "rego.metadata.rule() call",
			query:         "rego.metadata.rule()",
			expectedError: "rego.metadata.rule(): eval_builtin_error: rego.metadata.rule: the rego.metadata.rule function must only be called within the scope of a rule",
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			c := ast.NewCompiler()
			q := NewQuery(ast.MustParseBody(tc.query)).WithCompiler(c).
				WithStrictBuiltinErrors(true)
			_, err := q.Run(context.Background())

			if err == nil {
				t.Fatalf("expected error")
			}

			if tc.expectedError != err.Error() {
				t.Fatalf("expected error:\n\n%s\n\ngot:\n\n%s", tc.expectedError, err.Error())
			}
		})
	}
}

func initTracerTestQuery() *Query {
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

	return NewQuery(ast.MustParseBody("data.x.p")).
		WithCompiler(compiler).
		WithStore(store).
		WithTransaction(txn).
		WithInput(inputTerm)
}

type testQueryTracer struct {
	events  []*Event
	conf    TraceConfig
	enabled bool
	t       *testing.T
}

func (n *testQueryTracer) Enabled() bool {
	return n.enabled
}

func (n *testQueryTracer) Trace(e *Event) {
	n.t.Errorf("Unexpected call to Trace() with event %v", e)
}

func (n *testQueryTracer) TraceEvent(e Event) {
	n.events = append(n.events, &e)
}

func (n *testQueryTracer) Config() TraceConfig {
	return n.conf
}

type testLegacyTracer struct {
	events []*Event
}

func (n *testLegacyTracer) Enabled() bool {
	return true
}

func (n *testLegacyTracer) Trace(e *Event) {
	n.events = append(n.events, e)
}
