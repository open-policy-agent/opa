// Copyright 2024 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package debug

import (
	"bytes"
	"fmt"
	"sync"

	"github.com/open-policy-agent/opa/v1/ast/location"
)

type BreakpointID int

type Breakpoint interface {
	ID() BreakpointID
	Location() location.Location
}

type breakpoint struct {
	id       BreakpointID
	location location.Location
}

func (b breakpoint) ID() BreakpointID {
	return b.id
}

func (b breakpoint) Location() location.Location {
	return b.location
}

func (b breakpoint) String() string {
	return fmt.Sprintf("<%d> %s:%d", b.id, b.location.File, b.location.Row)
}

type breakpointList []Breakpoint

func (b breakpointList) String() string {
	if b == nil {
		return "[]"
	}

	buf := new(bytes.Buffer)
	buf.WriteString("[")
	for i, bp := range b {
		if i > 0 {
			buf.WriteString(", ")
		}
		_, _ = fmt.Fprint(buf, bp)
	}
	buf.WriteString("]")
	return buf.String()
}

type breakpointCollection struct {
	breakpoints map[string]breakpointList
	idCounter   BreakpointID
	mtx         sync.Mutex
}

func newBreakpointCollection() *breakpointCollection {
	return &breakpointCollection{
		breakpoints: map[string]breakpointList{},
	}
}

func (bc *breakpointCollection) newID() BreakpointID {
	bc.idCounter++
	return bc.idCounter
}

func (bc *breakpointCollection) add(location location.Location) Breakpoint {
	bc.mtx.Lock()
	defer bc.mtx.Unlock()

	bp := breakpoint{
		id:       bc.newID(),
		location: location,
	}
	bps := bc.breakpoints[bp.location.File]
	bps = append(bps, bp)
	bc.breakpoints[bp.location.File] = bps
	return bp
}

func (bc *breakpointCollection) all() breakpointList {
	bc.mtx.Lock()
	defer bc.mtx.Unlock()

	count := 0
	for _, list := range bc.breakpoints {
		count += len(list)
	}
	bps := make(breakpointList, 0, count)
	for _, list := range bc.breakpoints {
		bps = append(bps, list...)
	}
	return bps
}

func (bc *breakpointCollection) allForFilePath(path string) breakpointList {
	bc.mtx.Lock()
	defer bc.mtx.Unlock()

	return bc.breakpoints[path]
}

func (bc *breakpointCollection) remove(id BreakpointID) Breakpoint {
	bc.mtx.Lock()
	defer bc.mtx.Unlock()

	var removed Breakpoint
	for path, bps := range bc.breakpoints {
		var newBps breakpointList
		for _, bp := range bps {
			if bp.ID() != id {
				newBps = append(newBps, bp)
			} else {
				removed = bp
			}
		}
		bc.breakpoints[path] = newBps
	}

	return removed
}

func (bc *breakpointCollection) clear() {
	bc.mtx.Lock()
	defer bc.mtx.Unlock()

	bc.breakpoints = map[string]breakpointList{}
}

func (bc *breakpointCollection) String() string {
	if bc == nil {
		return "[]"
	}

	buf := new(bytes.Buffer)
	buf.WriteString("[")
	for _, bps := range bc.breakpoints {
		for i, bp := range bps {
			if i > 0 {
				buf.WriteString(", ")
			}
			_, _ = fmt.Fprint(buf, bp)
		}
	}
	buf.WriteString("]")
	return buf.String()
}
