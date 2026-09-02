// Copyright 2024 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package debug

import (
	"bytes"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/topdown"
)

const maxFrameNameLength = 80

// frameStore holds every StackFrame created during a session, and is the resolver for
// the FrameID handed out to clients. Frames are 1-indexed.
type frameStore struct {
	frames []*stackFrame
}

func newFrameStore() *frameStore {
	return &frameStore{}
}

func (fs *frameStore) add(f *stackFrame) *stackFrame {
	f.id = FrameID(len(fs.frames) + 1)
	fs.frames = append(fs.frames, f)
	return f
}

func (fs *frameStore) get(id FrameID) (*stackFrame, error) {
	index := int(id) - 1
	if index < 0 || index >= len(fs.frames) {
		return nil, fmt.Errorf("invalid frame id: %d", id)
	}
	return fs.frames[index], nil
}

// framer groups the trace events of a thread into the frames of a StackTrace.
type framer interface {
	// consume records the event at stackIndex in the thread's event history.
	// Events are consumed once, in order.
	consume(stackIndex int, e *topdown.Event, t *thread)

	// stackTrace returns the frames of the thread's stack, most recent frame first.
	stackTrace(t *thread) StackTrace
}

func newFramer(props LaunchProperties, frames *frameStore) framer {
	if props.StackTraceMode == StackTraceModeEvent {
		return &eventFramer{store: frames}
	}
	return &queryFramer{store: frames, skipOps: props.SkipOps, queries: newQueryIndex()}
}

// eventFramer creates one frame per consumed trace event, including events filtered
// out by LaunchProperties.SkipOps.
type eventFramer struct {
	store *frameStore
	// frames is this thread's stack, in event order.
	frames []*stackFrame
}

func (f *eventFramer) consume(stackIndex int, e *topdown.Event, t *thread) {
	var expl string
	if e.Node != nil {
		pretty := new(bytes.Buffer)
		topdown.PrettyTrace(pretty, []*topdown.Event{e})
		expl = strings.Trim(pretty.String(), "\n")
	} else {
		expl = fmt.Sprintf("%s, %s", e.Op, e.Location)
	}

	frame := f.store.add(&stackFrame{
		location:   e.Location,
		thread:     t.id,
		stackIndex: stackIndex,
		e:          e,
	})
	frame.name = fmt.Sprintf("#%d: %d %s", frame.id, e.QueryID, expl)

	f.frames = append(f.frames, frame)
}

func (f *eventFramer) stackTrace(*thread) StackTrace {
	frames := make(StackTrace, 0, len(f.frames))
	for i := len(f.frames) - 1; i >= 0; i-- {
		frames = append(frames, f.frames[i])
	}

	return frames
}

// queryFramer creates one frame per query scope: the base query, a rule or function
// body, a comprehension, or an every expression and its body.
type queryFramer struct {
	store   *frameStore
	skipOps []topdown.Op
	queries *queryIndex
}

func (f *queryFramer) consume(stackIndex int, e *topdown.Event, _ *thread) {
	f.queries.add(stackIndex, e, slices.Contains(f.skipOps, e.Op))
}

// stackTrace walks the QueryID -> ParentID chain up from the most recently consumed
// event. Query IDs are allocated in creation order, so a parent always has a lower ID
// than its child, which makes the walk terminating.
func (f *queryFramer) stackTrace(t *thread) StackTrace {
	frames := StackTrace{}

	stackIndex := t.consumed - 1
	e := t.stack.Event(stackIndex)
	if e == nil {
		return frames
	}

	queryID := e.QueryID

	for {
		frames = append(frames, f.newFrame(t, queryID, stackIndex))

		if queryID == 0 {
			break
		}

		parent, ok := f.queries.parent[queryID]
		if !ok || parent >= queryID {
			break
		}

		if stackIndex, ok = f.queries.anchor(parent); !ok {
			break
		}

		queryID = parent
	}

	return frames
}

func (f *queryFramer) newFrame(t *thread, queryID uint64, stackIndex int) *stackFrame {
	e := t.stack.Event(stackIndex)

	frame := &stackFrame{
		name:       f.name(t, queryID),
		thread:     t.id,
		stackIndex: stackIndex,
		e:          e,
	}

	if e != nil {
		frame.location = e.Location
	}

	return f.store.add(frame)
}

// name names a frame for the query it represents, prefixed with the query ID to let
// users correlate frames with the query IDs reported in OPA's traces.
func (f *queryFramer) name(t *thread, queryID uint64) string {
	if i, ok := f.queries.enter[queryID]; ok {
		if name := queryNodeName(t.stack.Event(i)); name != "" {
			return fmt.Sprintf("%d: %s", queryID, name)
		}
	}

	return strconv.FormatUint(queryID, 10)
}

// queryIndex tracks, per query ID, where in the event history that query was entered,
// which query entered it, and the events it has most recently emitted.
type queryIndex struct {
	parent           map[uint64]uint64
	enter            map[uint64]int
	latestAny        map[uint64]int
	latestNonSkipped map[uint64]int
}

func newQueryIndex() *queryIndex {
	return &queryIndex{
		parent:           map[uint64]uint64{},
		enter:            map[uint64]int{},
		latestAny:        map[uint64]int{},
		latestNonSkipped: map[uint64]int{},
	}
}

func (qi *queryIndex) add(stackIndex int, e *topdown.Event, skipped bool) {
	if e == nil {
		return
	}

	if _, ok := qi.parent[e.QueryID]; !ok {
		qi.parent[e.QueryID] = e.ParentID
	}

	if e.Op == topdown.EnterOp {
		if _, ok := qi.enter[e.QueryID]; !ok {
			qi.enter[e.QueryID] = stackIndex
		}
	}

	qi.latestAny[e.QueryID] = stackIndex
	if !skipped {
		qi.latestNonSkipped[e.QueryID] = stackIndex
	}
}

// anchor returns the index of the event that a frame for queryID is anchored to.
func (qi *queryIndex) anchor(queryID uint64) (int, bool) {
	if i, ok := qi.latestNonSkipped[queryID]; ok {
		return i, true
	}
	if i, ok := qi.enter[queryID]; ok {
		return i, true
	}

	i, ok := qi.latestAny[queryID]
	return i, ok
}

func queryNodeName(e *topdown.Event) string {
	if e == nil {
		return ""
	}

	switch n := e.Node.(type) {
	case *ast.Rule:
		if n.Module != nil {
			return truncatedString(n.Ref().String(), maxFrameNameLength)
		}
		return truncatedString(n.Head.Ref().String(), maxFrameNameLength)
	case *ast.Expr:
		return truncatedString(n.String(), maxFrameNameLength)
	case ast.Body:
		return truncatedString(n.String(), maxFrameNameLength)
	}

	return ""
}
