// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package topdown provides low-level query evaluation support.
//
// The topdown implementation is a modified version of the standard top-down
// evaluation algorithm used in Datalog. References and comprehensions are
// evaluated eagerly while all other terms are evaluated lazily.
//
// Deprecated: This package is intended for older projects transitioning from OPA v0.x and will remain for the lifetime of OPA v1.x, but its use is not recommended.
// For newer features and behaviours, such as defaulting to the Rego v1 syntax, use the corresponding components in the [github.com/open-policy-agent/opa/v1] package instead.
// See https://www.openpolicyagent.org/docs/latest/v0-compatibility/ for more information.
package topdown

import (
	"io"

	"github.com/open-policy-agent/opa/topdown/print"
	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/metrics"
	v1 "github.com/open-policy-agent/opa/v1/topdown"
)

type (
	// Deprecated: Functional-style builtins are deprecated. Use BuiltinFunc instead.
	FunctionalBuiltin1 = v1.FunctionalBuiltin1

	// Deprecated: Functional-style builtins are deprecated. Use BuiltinFunc instead.
	FunctionalBuiltin2 = v1.FunctionalBuiltin2

	// Deprecated: Functional-style builtins are deprecated. Use BuiltinFunc instead.
	FunctionalBuiltin3 = v1.FunctionalBuiltin3

	// Deprecated: Functional-style builtins are deprecated. Use BuiltinFunc instead.
	FunctionalBuiltin4 = v1.FunctionalBuiltin4

	// BuiltinContext contains context from the evaluator that may be used by
	// built-in functions.
	BuiltinContext = v1.BuiltinContext

	// BuiltinFunc defines an interface for implementing built-in functions.
	// The built-in function is called with the plugged operands from the call
	// (including the output operands.) The implementation should evaluate the
	// operands and invoke the iterator for each successful/defined output
	// value.
	BuiltinFunc = v1.BuiltinFunc
)

// RegisterBuiltinFunc adds a new built-in function to the evaluation engine.
func RegisterBuiltinFunc(name string, f BuiltinFunc) {
	v1.RegisterBuiltinFunc(name, f)
}

// Deprecated: Functional-style builtins are deprecated. Use RegisterBuiltinFunc instead.
func RegisterFunctionalBuiltin1(name string, fun FunctionalBuiltin1) {
	v1.RegisterFunctionalBuiltin1(name, fun)
}

// Deprecated: Functional-style builtins are deprecated. Use RegisterBuiltinFunc instead.
func RegisterFunctionalBuiltin2(name string, fun FunctionalBuiltin2) {
	v1.RegisterFunctionalBuiltin2(name, fun)
}

// Deprecated: Functional-style builtins are deprecated. Use RegisterBuiltinFunc instead.
func RegisterFunctionalBuiltin3(name string, fun FunctionalBuiltin3) {
	v1.RegisterFunctionalBuiltin3(name, fun)
}

// Deprecated: Functional-style builtins are deprecated. Use RegisterBuiltinFunc instead.
func RegisterFunctionalBuiltin4(name string, fun FunctionalBuiltin4) {
	v1.RegisterFunctionalBuiltin4(name, fun)
}

// GetBuiltin returns a built-in function implementation, nil if no built-in found.
func GetBuiltin(name string) BuiltinFunc {
	return v1.GetBuiltin(name)
}

// Deprecated: The BuiltinEmpty type is no longer needed. Use nil return values instead.
type BuiltinEmpty = v1.Builtin

// VirtualCache defines the interface for a cache that stores the results of
// evaluated virtual documents (rules).
// The cache is a stack of frames, where each frame is a mapping from references
// to values.
type VirtualCache = v1.VirtualCache

func NewVirtualCache() VirtualCache {
	return v1.NewVirtualCache()
}

// Cancel defines the interface for cancelling topdown queries. Cancel
// operations are thread-safe and idempotent.
type Cancel = v1.Cancel

// NewCancel returns a new Cancel object.
func NewCancel() Cancel {
	return v1.NewCancel()
}

// Halt is a special error type that built-in function implementations return to indicate
// that policy evaluation should stop immediately.
type Halt = v1.Halt

// Error is the error type returned by the Eval and Query functions when
// an evaluation error occurs.
type Error = v1.Error

// StackFrame is a single query in the stack of queries that were being evaluated
// when an error occurred.
type StackFrame = v1.StackFrame

// StackTrace is the stack of queries that were being evaluated when an error
// occurred, ordered from the innermost query outwards.
type StackTrace = v1.StackTrace

const (

	// InternalErr represents an unknown evaluation error.
	InternalErr = v1.InternalErr

	// CancelErr indicates the evaluation process was cancelled.
	CancelErr = v1.CancelErr

	// ConflictErr indicates a conflict was encountered during evaluation. For
	// instance, a conflict occurs if a rule produces multiple, differing values
	// for the same key in an object. Conflict errors indicate the policy does
	// not account for the data loaded into the policy engine.
	ConflictErr = v1.ConflictErr

	// TypeErr indicates evaluation stopped because an expression was applied to
	// a value of an inappropriate type.
	TypeErr = v1.TypeErr

	// BuiltinErr indicates a built-in function received a semantically invalid
	// input or encountered some kind of runtime error, e.g., connection
	// timeout, connection refused, etc.
	BuiltinErr = v1.BuiltinErr

	// WithMergeErr indicates that the real and replacement data could not be merged.
	WithMergeErr = v1.WithMergeErr
)

// IsError returns true if the err is an Error.
func IsError(err error) bool {
	return v1.IsError(err)
}

// IsCancel returns true if err was caused by cancellation.
func IsCancel(err error) bool {
	return v1.IsCancel(err)
}

const (
	// HTTPSendInternalErr represents a runtime evaluation error.
	HTTPSendInternalErr = v1.HTTPSendInternalErr

	// HTTPSendNetworkErr represents a network error.
	HTTPSendNetworkErr = v1.HTTPSendNetworkErr
)

// Instrumentation implements helper functions to instrument query evaluation
// to diagnose performance issues. Instrumentation may be expensive in some
// cases, so it is disabled by default.
type Instrumentation = v1.Instrumentation

// NewInstrumentation returns a new Instrumentation object. Performance
// diagnostics recorded on this Instrumentation object will stored in m.
func NewInstrumentation(m metrics.Metrics) *Instrumentation {
	return v1.NewInstrumentation(m)
}

func NewPrintHook(w io.Writer) print.Hook {
	return v1.NewPrintHook(w)
}

// QueryResultSet represents a collection of results returned by a query.
type QueryResultSet = v1.QueryResultSet

// QueryResult represents a single result returned by a query. The result
// contains bindings for all variables that appear in the query.
type QueryResult = v1.QueryResult

// Query provides a configurable interface for performing query evaluation.
type Query = v1.Query

// Builtin represents a built-in function that queries can call.
type Builtin = v1.Builtin

// NewQuery returns a new Query object that can be run.
func NewQuery(query ast.Body) *Query {
	return v1.NewQuery(query)
}

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
//
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
