// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import "github.com/open-policy-agent/opa/ast"

// Op defines the types of tracing events.
type Op string

const (
	// EnterOp is emitted when a new query is about to be evaluated.
	EnterOp Op = "Enter"

	// ExitOp is emitted when a query has evaluated to true.
	ExitOp Op = "Exit"

	// EvalOp is emitted when an expression is about to be evaluated.
	EvalOp Op = "Eval"

	// RedoOp is emitted when an expression, rule, or query is being re-evaluated.
	RedoOp Op = "Redo"

	// FailOp is emitted when an expression evaluates to false.
	FailOp Op = "Fail"
)

// Event contains state associated with a tracing event.
type Event struct {
	Op       Op          // Identifies type of event.
	Node     interface{} // Contains AST node relevant to the event.
	QueryID  uint64      // Identifies the query this event belongs to.
	ParentID uint64      // Identifies the parent query this event belongs to.
}

// Equal returns true if this event is equal to the other event.
func (evt Event) Equal(other Event) bool {
	if evt.Op != other.Op {
		return false
	}
	if evt.QueryID != other.QueryID {
		return false
	}
	if evt.ParentID != other.ParentID {
		return false
	}
	if !equalNodes(evt, other) {
		return false
	}
	return true
}

func (evt Event) String() string {
	return fmt.Sprintf("%v %v (qid=%v, pqid=%v)", evt.Op, evt.Node, evt.QueryID, evt.ParentID)
}

// Tracer defines the interface for tracing in the top-down evaluation engine.
type Tracer interface {
	Enabled() bool
	Trace(ctx *Context, evt Event)
}

func equalNodes(a, b Event) bool {
	switch a := a.Node.(type) {
	case ast.Body:
		if b, ok := b.Node.(ast.Body); ok {
			return a.Equal(b)
		}
	case *ast.Rule:
		if b, ok := b.Node.(*ast.Rule); ok {
			return a.Equal(b)
		}
	case *ast.Expr:
		if b, ok := b.Node.(*ast.Expr); ok {
			return a.Equal(b)
		}
	case nil:
		return b.Node == nil
	}
	return false
}

// StdoutTracer ...
type StdoutTracer struct{}

// Enabled ...
func (t *StdoutTracer) Enabled() bool {
	return true
}

// Trace ...
func (t *StdoutTracer) Trace(ctx *Context, evt Event) {

}
