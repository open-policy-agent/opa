// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
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
}

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

	if text := frameText(loc.Text); text != "" {
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
func frameText(src []byte) string {
	text := strings.Join(strings.Fields(string(src)), " ")

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
		st = append(st, StackFrame{QueryID: curr.queryID, Location: curr.currentLocation()})
	}
	return st
}

// currentLocation returns the location of the expression e is evaluating. Once
// the whole body has succeeded e.index sits one past the end, leaving nothing to
// point at, so the last expression is reported as the nearest position.
func (e *eval) currentLocation() *ast.Location {
	if len(e.query) == 0 {
		return nil
	}
	return e.query[min(e.index, len(e.query)-1)].Location
}

// withStackTrace records the evaluation stack on err, if enabled. Kept small
// enough to inline, so the disabled case costs only a branch.
func (e *eval) withStackTrace(err error) error {
	if err == nil || !e.stackTraces {
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
