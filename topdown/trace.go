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

// LineTracer implements the Tracer interface by writing events to an output
// stream.
type LineTracer struct {
	output io.Writer
	depths map[uint64]int
}

// NewLineTracer returns a new LineTracer.
func NewLineTracer(output io.Writer) *LineTracer {
	return &LineTracer{
		output: output,
		depths: map[uint64]int{},
	}
}

// Enabled always returns true.
func (t *LineTracer) Enabled() bool {
	return true
}

// Trace formats the event as a string and writes it to the output stream.
// Expression and rule AST nodes will be plugged with bindings from the context.
func (t *LineTracer) Trace(ctx *Context, evt Event) {
	msg := t.formatEvent(ctx, evt)
	fmt.Fprintln(t.output, msg)
}

func (t *LineTracer) formatEvent(ctx *Context, evt Event) string {
	padding := t.getPadding(evt)
	node := t.getNode(ctx, evt)
	return fmt.Sprintf("%v%v %v", padding, evt.Op, node)
}

func (t *LineTracer) getNode(ctx *Context, evt Event) interface{} {
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

func (t *LineTracer) getPadding(evt Event) string {
	d := t.depths[evt.QueryID]
	if d == 0 {
		d = t.depths[evt.ParentID]
		d++
		if t.depths == nil {
			t.depths = map[uint64]int{}
		}
		t.depths[evt.QueryID] = d
	}
	var spaces int
	switch evt.Op {
	case EnterOp:
		spaces = d
	case RedoOp:
		if _, ok := evt.Node.(*ast.Expr); !ok {
			spaces = d
			break
		}
		fallthrough
	default:
		spaces = d + 1
	}
	padding := ""
	if spaces > 1 {
		padding += strings.Repeat("| ", spaces-1)
	}
	return padding
}
