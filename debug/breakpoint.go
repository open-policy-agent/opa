// Copyright 2024 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package debug

import (
	"bytes"
	"fmt"

	"github.com/open-policy-agent/opa/ast/location"
)

type Breakpoint interface {
	Id() int
	Location() location.Location
}

type breakpoint struct {
	id       int
	location location.Location
}

func (b breakpoint) Id() int {
	return b.id
}

func (b breakpoint) Location() location.Location {
	return b.location
}

func (b breakpoint) String() string {
	return fmt.Sprintf("<%d> %s:%d", b.id, b.location.File, b.location.Row)
}

type breakpointList []breakpoint

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
		_, _ = fmt.Fprintf(buf, "%s:%d", bp.location.File, bp.location.Row)
	}
	buf.WriteString("]")
	return buf.String()
}

type breakpointCollection struct {
	breakpoints map[string]breakpointList
	idCounter   int
}

func newBreakpointCollection() *breakpointCollection {
	return &breakpointCollection{
		breakpoints: map[string]breakpointList{},
	}
}

func (bc *breakpointCollection) newId() int {
	bc.idCounter++
	return bc.idCounter
}

func (bc *breakpointCollection) add(location location.Location) Breakpoint {
	bp := breakpoint{
		id:       bc.newId(),
		location: location,
	}
	bps := bc.breakpoints[bp.location.File]
	bps = append(bps, bp)
	bc.breakpoints[bp.location.File] = bps
	return bp
}

func (bc *breakpointCollection) allForFilePath(path string) breakpointList {
	return bc.breakpoints[path]
}

func (bc *breakpointCollection) clear() {
	bc.breakpoints = map[string]breakpointList{}
}

func (bc *breakpointCollection) String() string {
	if bc == nil {
		return "[]"
	}

	buf := new(bytes.Buffer)
	buf.WriteString("[")
	for path, bps := range bc.breakpoints {
		for i, bp := range bps {
			if i > 0 {
				buf.WriteString(", ")
			}
			_, _ = fmt.Fprintf(buf, "%s:%d\n", path, bp.location.Row)
		}
	}
	buf.WriteString("]")
	return buf.String()
}
