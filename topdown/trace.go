// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"fmt"
	"io"
	"strings"

	"github.com/open-policy-agent/opa/ast"
)

// Trace represents a sequence of events emitted during query evaluation.
type Trace []*Event

func (e Trace) String() string {
	depths := depths{}
	buf := make([]string, len(e))
	for i, evt := range e {
		depth := depths.GetOrSet(evt.QueryID, evt.ParentID)
		buf[i] = evt.format(depth)
	}
	return strings.Join(buf, "\n")
}

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

// HasRule returns true if the Event contains an ast.Rule.
func (evt *Event) HasRule() bool {
	_, ok := evt.Node.(*ast.Rule)
	return ok
}

// HasBody returns true if the Event contains an ast.Body.
func (evt *Event) HasBody() bool {
	_, ok := evt.Node.(ast.Body)
	return ok
}

// HasExpr returns true if the Event contains an ast.Expr.
func (evt *Event) HasExpr() bool {
	_, ok := evt.Node.(*ast.Expr)
	return ok
}

// Equal returns true if this event is equal to the other event.
func (evt *Event) Equal(other *Event) bool {
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

func (evt *Event) format(depth int) string {
	padding := evt.getPadding(depth)
	return fmt.Sprintf("%v%v %v", padding, evt.Op, evt.Node)
}

func (evt *Event) getPadding(depth int) string {
	spaces := evt.getSpaces(depth)
	padding := ""
	if spaces > 1 {
		padding += strings.Repeat("| ", spaces-1)
	}
	return padding
}

func (evt *Event) getSpaces(depth int) int {
	switch evt.Op {
	case EnterOp:
		return depth
	case RedoOp:
		if _, ok := evt.Node.(*ast.Expr); !ok {
			return depth
		}
	}
	return depth + 1
}

func (evt *Event) String() string {
	return fmt.Sprintf("%v %v (qid=%v, pqid=%v)", evt.Op, evt.Node, evt.QueryID, evt.ParentID)
}

// Tracer defines the interface for tracing in the top-down evaluation engine.
type Tracer interface {
	Enabled() bool
	Trace(ctx *Context, evt *Event)
}

// LineTracer implements the Tracer interface by writing events to an output
// stream.
type LineTracer struct {
	output io.Writer
	depths depths
}

// NewLineTracer returns a new LineTracer.
func NewLineTracer(output io.Writer) *LineTracer {
	return &LineTracer{
		output: output,
		depths: depths{},
	}
}

// Enabled always returns true.
func (t *LineTracer) Enabled() bool {
	return true
}

// Trace emits evt to the output stream. The event will be formatted based on
// query depth and bindings in ctx.
func (t *LineTracer) Trace(ctx *Context, evt *Event) {
	evt.Node = t.mangleNode(ctx, evt)
	depth := t.depths.GetOrSet(evt.QueryID, evt.ParentID)
	fmt.Fprintln(t.output, evt.format(depth))
}

func (t *LineTracer) mangleNode(ctx *Context, evt *Event) interface{} {
	switch evt.Op {
	case RedoOp:
		switch node := evt.Node.(type) {
		case *ast.Rule:
			return node.Head()
		}
	default:
		switch node := evt.Node.(type) {
		case *ast.Rule:
			return PlugHead(node.Head(), ctx)
		case *ast.Expr:
			return PlugExpr(node, ctx)
		}
	}
	return evt.Node
}

// depths is a helper for computing the depth of an event. Events within the
// same query all have the same depth. The depth of query is
// depth(parent(query))+1.
type depths map[uint64]int

func (ds depths) GetOrSet(qid uint64, pqid uint64) int {
	depth := ds[qid]
	if depth == 0 {
		depth = ds[pqid]
		depth++
		ds[qid] = depth
	}
	return depth
}

func equalNodes(a, b *Event) bool {
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
