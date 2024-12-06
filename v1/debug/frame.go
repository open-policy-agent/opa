// Copyright 2024 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package debug

import (
	"fmt"

	"github.com/open-policy-agent/opa/v1/ast/location"
	"github.com/open-policy-agent/opa/v1/topdown"
)

type FrameID int

type StackFrame interface {
	// ID returns the unique identifier for the frame.
	ID() FrameID

	// Name returns the human-readable name of the frame.
	Name() string

	// Location returns the location of the frame in the source code.
	Location() *location.Location

	// Thread returns the ID of the thread that the frame is associated with.
	Thread() ThreadID

	// String returns a human-readable string representation of the frame.
	String() string

	// Equal returns true if the frame is equal to the other frame.
	Equal(other StackFrame) bool
}

type stackFrame struct {
	id       FrameID
	name     string
	location *location.Location
	thread   ThreadID

	e          *topdown.Event
	stackIndex int
}

func (f *stackFrame) ID() FrameID {
	return f.id
}

func (f *stackFrame) Name() string {
	return f.name
}

func (f *stackFrame) Location() *location.Location {
	return f.location
}

func (f *stackFrame) Thread() ThreadID {
	return f.thread
}

func (f *stackFrame) String() string {
	return fmt.Sprintf("{id: %d, name: %v, location: %v}", f.id, f.name, f.location)
}

func (f *stackFrame) Equal(other StackFrame) bool {
	if f.ID() != other.ID() {
		return false
	}
	if f.Name() != other.Name() {
		return false
	}
	if !f.Location().Equal(other.Location()) {
		return false
	}
	if f.Thread() != other.Thread() {
		return false
	}
	return true
}

// StackTrace represents a StackFrame stack.
type StackTrace []StackFrame

func (s StackTrace) Equal(other StackTrace) bool {
	if len(s) != len(other) {
		return false
	}
	for i := range s {
		if !s[i].Equal(other[i]) {
			return false
		}
	}
	return true
}
