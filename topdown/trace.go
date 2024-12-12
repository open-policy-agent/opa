// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"io"

	v1 "github.com/open-policy-agent/opa/v1/topdown"
)

// Op defines the types of tracing events.
type Op = v1.Op

const (
	// EnterOp is emitted when a new query is about to be evaluated.
	EnterOp = v1.EnterOp

	// ExitOp is emitted when a query has evaluated to true.
	ExitOp = v1.ExitOp

	// EvalOp is emitted when an expression is about to be evaluated.
	EvalOp = v1.EvalOp

	// RedoOp is emitted when an expression, rule, or query is being re-evaluated.
	RedoOp = v1.RedoOp

	// SaveOp is emitted when an expression is saved instead of evaluated
	// during partial evaluation.
	SaveOp = v1.SaveOp

	// FailOp is emitted when an expression evaluates to false.
	FailOp = v1.FailOp

	// DuplicateOp is emitted when a query has produced a duplicate value. The search
	// will stop at the point where the duplicate was emitted and backtrack.
	DuplicateOp = v1.DuplicateOp

	// NoteOp is emitted when an expression invokes a tracing built-in function.
	NoteOp = v1.NoteOp

	// IndexOp is emitted during an expression evaluation to represent lookup
	// matches.
	IndexOp = v1.IndexOp

	// WasmOp is emitted when resolving a ref using an external
	// Resolver.
	WasmOp = v1.WasmOp

	// UnifyOp is emitted when two terms are unified.  Node will be set to an
	// equality expression with the two terms.  This Node will not have location
	// info.
	UnifyOp           = v1.UnifyOp
	FailedAssertionOp = v1.FailedAssertionOp
)

// VarMetadata provides some user facing information about
// a variable in some policy.
type VarMetadata = v1.VarMetadata

// Event contains state associated with a tracing event.
type Event = v1.Event

// Tracer defines the interface for tracing in the top-down evaluation engine.
// Deprecated: Use QueryTracer instead.
type Tracer = v1.Tracer

// QueryTracer defines the interface for tracing in the top-down evaluation engine.
// The implementation can provide additional configuration to modify the tracing
// behavior for query evaluations.
type QueryTracer = v1.QueryTracer

// TraceConfig defines some common configuration for Tracer implementations
type TraceConfig = v1.TraceConfig

// WrapLegacyTracer will create a new QueryTracer which wraps an
// older Tracer instance.
func WrapLegacyTracer(tracer Tracer) QueryTracer {
	return v1.WrapLegacyTracer(tracer)
}

// BufferTracer implements the Tracer and QueryTracer interface by
// simply buffering all events received.
type BufferTracer = v1.BufferTracer

// NewBufferTracer returns a new BufferTracer.
func NewBufferTracer() *BufferTracer {
	return v1.NewBufferTracer()
}

// PrettyTrace pretty prints the trace to the writer.
func PrettyTrace(w io.Writer, trace []*Event) {
	v1.PrettyTrace(w, trace)
}

// PrettyTraceWithLocation prints the trace to the writer and includes location information
func PrettyTraceWithLocation(w io.Writer, trace []*Event) {
	v1.PrettyTraceWithLocation(w, trace)
}

type PrettyTraceOptions = v1.PrettyTraceOptions

func PrettyTraceWithOpts(w io.Writer, trace []*Event, opts PrettyTraceOptions) {
	v1.PrettyTraceWithOpts(w, trace, opts)
}

type PrettyEventOpts = v1.PrettyEventOpts

func PrettyEvent(w io.Writer, e *Event, opts PrettyEventOpts) error {
	return v1.PrettyEvent(w, e, opts)
}
