// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"bytes"
	"slices"
	"strconv"
	"strings"

	"github.com/open-policy-agent/opa/v1/ast"
)

// maxStackFrameTextLength keeps frames with large literals in them readable.
const maxStackFrameTextLength = 80

// StackFrame is one query in a StackTrace.
type StackFrame struct {
	// QueryID matches the QueryID of the trace events for the same query.
	QueryID uint64 `json:"query_id"`

	// Location is the expression being evaluated, nil if the query has no
	// location information.
	Location *ast.Location `json:"location,omitempty"`

	// text is the source at Location with the values bound to its variables
	// spliced in, empty when nothing was bound. Unexported to keep the policy
	// source out of the marshaled frame; String reports it.
	text string
}

// String returns the frame's position and, when the query has source, the
// expression at it with the values its variables were bound to spliced in.
func (f StackFrame) String() string {
	s := strings.Builder{}
	f.writeTo(&s)
	return s.String()
}

func (f StackFrame) writeTo(s *strings.Builder) {
	loc := f.Location
	if loc == nil {
		s.WriteString("<unknown>")
		return
	}

	if loc.File != "" {
		s.WriteString(loc.File)
		s.WriteByte(':')
		s.WriteString(strconv.Itoa(loc.Row))
	} else {
		s.WriteString(strconv.Itoa(loc.Row))
		s.WriteByte(':')
		s.WriteString(strconv.Itoa(loc.Col))
	}

	src := f.text
	if src == "" && loc.Text != nil {
		src = string(loc.Text)
	}

	if text := frameText(src); text != "" {
		s.WriteString(": ")
		s.WriteString(text)
	}
}

// StackTrace is the stack of queries being evaluated when an error occurred,
// innermost first.
type StackTrace []StackFrame

// String returns one indented frame per line.
func (st StackTrace) String() string {
	s := strings.Builder{}
	for i := range st {
		if i > 0 {
			s.WriteByte('\n')
		}
		s.WriteString("  ")
		st[i].writeTo(&s)
	}
	return s.String()
}

// frameText collapses src onto one line and truncates it.
func frameText(src string) string {
	text := strings.Join(strings.Fields(src), " ")

	var runes int
	for i := range text {
		if runes == maxStackFrameTextLength {
			return text[:i] + "..."
		}
		runes++
	}

	return text
}

// stackTrace captures the evaluation stack, innermost first. Enclosing queries
// are still suspended on the call stack here, so their expression indices have
// not been unwound yet.
//
// The chain is walked twice to size the slice: append would cost an allocation
// per doubling, on every error a deep stack raises.
func (e *eval) stackTrace() StackTrace {
	var depth int
	for curr := e; curr != nil; curr = curr.parent {
		depth++
	}

	st := make(StackTrace, 0, depth)
	for curr := e; curr != nil; curr = curr.parent {
		expr := curr.currentExpr()
		if expr == nil {
			st = append(st, StackFrame{QueryID: curr.queryID})
			continue
		}
		st = append(st, StackFrame{
			QueryID:  curr.queryID,
			Location: expr.Location,
			text:     curr.resolvedText(expr),
		})
	}
	return st
}

// currentExpr returns the expression e is evaluating. Once the whole body has
// succeeded e.index sits one past the end, leaving nothing to point at, so the
// last expression is reported as the nearest position.
func (e *eval) currentExpr() *ast.Expr {
	if len(e.query) == 0 {
		return nil
	}
	return e.query[min(e.index, len(e.query)-1)]
}

// resolvedText renders the source of expr with the values its variables are
// bound to spliced in, so a frame reads fn(1) where the policy wrote fn(x) - the
// argument a call failed on is usually the reason it failed. Returns "" when
// nothing was substituted and the source stands on its own.
//
// Bindings are unwound as evaluation backtracks, so this has to run while the
// error is being raised rather than when the frame is rendered.
func (e *eval) resolvedText(expr *ast.Expr) string {
	loc := expr.Location
	if loc == nil || len(loc.Text) == 0 {
		return ""
	}

	capture := e.stackCapture
	capture.vars = appendReadVars(capture.vars[:0], expr)
	subs := capture.subs[:0]

	for _, t := range capture.vars {
		start, ok := sourceSpan(loc, t.Location)
		if !ok {
			continue
		}

		// An unbound var plugs to itself, and a value too long to fit in a frame
		// is better left as the name the policy gave it.
		bound := e.bindings.Plug(t)
		if bound == t || !bound.IsGround() || bound.StringLength() > maxStackFrameTextLength {
			continue
		}

		subs = append(subs, textSub{start: start, end: start + len(t.Location.Text), value: bound.String()})
	}

	capture.subs = subs

	if len(subs) == 0 {
		return ""
	}

	slices.SortFunc(subs, func(a, b textSub) int { return a.start - b.start })

	s := strings.Builder{}
	prev := 0
	for _, sub := range subs {
		// A var reached twice, or one nested in the span of another, resolves to
		// the same text the first one already wrote.
		if sub.start < prev {
			continue
		}
		s.Write(loc.Text[prev:sub.start])
		s.WriteString(sub.value)
		prev = sub.end
	}
	s.Write(loc.Text[prev:])

	return s.String()
}

// textSub is a span of an expression's source to replace with a value.
type textSub struct {
	start, end int
	value      string
}

// stackTraceCapture is the state Query.WithStackTraces turns on. A non-nil one
// on an eval means capture is enabled, so the feature adds a single field to a
// struct that is copied for every query.
type stackTraceCapture struct {
	// builtinErrors records whether collected built-in errors have a consumer.
	// Without one query.go drops them, and a policy over messy data reaches that
	// path for every row.
	builtinErrors bool

	// Working space for resolvedText, shared by every eval of the query -
	// evaluation is single threaded - rather than reallocated per stack.
	vars []*ast.Term
	subs []textSub
}

// newStackCapture returns the capture state to share across q's evals, nil when
// stack traces are off and nothing will ask for it.
func (q *Query) newStackCapture() *stackTraceCapture {
	if !q.stackTraces {
		return nil
	}
	return &stackTraceCapture{builtinErrors: q.strictBuiltinErrors || q.builtinErrorList != nil}
}

// appendReadVars appends every variable expr reads to dst. ast.WalkTerms would
// find the same ones, but it allocates a visitor and a closure per call and this
// runs once per frame of every captured stack.
//
// Positions that declare a variable rather than read one are skipped, so an
// every keeps its own name in the head instead of reading `every 1 in [1]`. So
// are comprehension bodies: their variables belong to a child query's bindings,
// which the frame cannot resolve.
func appendReadVars(dst []*ast.Term, expr *ast.Expr) []*ast.Term {
	switch terms := expr.Terms.(type) {
	case *ast.Term:
		dst = appendVarsInTerm(dst, terms)
	case []*ast.Term:
		// terms[0] is the operator, a ref to a built-in or to a rule.
		for _, t := range terms[1:] {
			dst = appendVarsInTerm(dst, t)
		}
	case *ast.Every:
		// Key and Value are the every's to declare, but its body reads them.
		dst = appendVarsInTerm(dst, terms.Domain)
		for _, e := range terms.Body {
			dst = appendReadVars(dst, e)
		}
	}

	for _, w := range expr.With {
		dst = appendVarsInTerm(dst, w.Value)
	}

	return dst
}

func appendVarsInTerm(dst []*ast.Term, t *ast.Term) []*ast.Term {
	// Composites track their groundness, so this prunes most of the walk for
	// the cost of a field read.
	if t.IsGround() {
		return dst
	}

	switch v := t.Value.(type) {
	case ast.Var:
		dst = append(dst, t)
	case ast.Ref:
		for _, t := range v {
			dst = appendVarsInTerm(dst, t)
		}
	case ast.Call:
		for _, t := range v[1:] {
			dst = appendVarsInTerm(dst, t)
		}
	case *ast.Array:
		for i := range v.Len() {
			dst = appendVarsInTerm(dst, v.Elem(i))
		}
	case ast.Set:
		// Slice and Keys allocate where Foreach wouldn't, but a closure over dst
		// would put it on the heap for every call, ground terms included.
		for _, t := range v.Slice() {
			dst = appendVarsInTerm(dst, t)
		}
	case ast.Object:
		for _, k := range v.Keys() {
			dst = appendVarsInTerm(dst, k)
			dst = appendVarsInTerm(dst, v.Get(k))
		}
	}

	return dst
}

// sourceSpan returns where term's source sits within expr's, and false when the
// two don't line up. Terms shared across a policy - the data root document, for
// one - carry the location of wherever they were first parsed, so matching the
// offset alone isn't enough.
func sourceSpan(expr, term *ast.Location) (int, bool) {
	if term == nil || len(term.Text) == 0 || term.File != expr.File {
		return 0, false
	}

	start := term.Offset - expr.Offset
	end := start + len(term.Text)
	if start < 0 || end > len(expr.Text) || !bytes.Equal(expr.Text[start:end], term.Text) {
		return 0, false
	}

	return start, true
}

// withStackTrace records the evaluation stack on err, if enabled. Kept small
// enough to inline, so the disabled case costs only a branch.
func (e *eval) withStackTrace(err error) error {
	if err == nil || e.stackCapture == nil {
		return err
	}
	return e.attachStackTrace(err)
}

// attachStackTrace returns err carrying the evaluation stack, or unchanged if it
// already has one - the innermost stack wins. err is copied, not annotated in
// place: shared errors like errInScopeWithStmt would otherwise race, and leak
// one query's stack into every later occurrence.
func (e *eval) attachStackTrace(err error) error {
	if tdErr, ok := err.(*Error); ok && tdErr.StackTrace == nil {
		cpy := *tdErr
		cpy.StackTrace = e.stackTrace()
		return &cpy
	}
	return err
}
