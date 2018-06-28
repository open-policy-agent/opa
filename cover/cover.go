// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package cover reports coverage on modules.
package cover

import (
	"sort"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/topdown"
)

// Cover computes and reports on coverage.
type Cover struct {
	hits map[string]map[Position]struct{}
}

// New returns a new Cover object.
func New() *Cover {
	return &Cover{
		hits: map[string]map[Position]struct{}{},
	}
}

// Enabled returns true if coverage is enabled.
func (c *Cover) Enabled() bool {
	return true
}

// Report returns a coverage Report for the given modules.
func (c *Cover) Report(modules map[string]*ast.Module) (report Report) {
	report.Files = map[string]*FileReport{}
	for file, hits := range c.hits {
		covered := make(PositionSlice, 0, len(hits))
		for pos := range hits {
			covered = append(covered, pos)
		}
		covered.Sort()
		fr, ok := report.Files[file]
		if !ok {
			fr = &FileReport{}
			report.Files[file] = fr
		}
		fr.Covered = sortedPositionSliceToRangeSlice(covered)
	}
	for file, module := range modules {
		notCovered := PositionSlice{}
		ast.WalkRules(module, func(x *ast.Rule) bool {
			if hasFileLocation(x.Head.Location) {
				if !report.IsCovered(x.Location.File, x.Location.Row) {
					notCovered = append(notCovered, Position{x.Head.Location.Row})
				}
			}
			return false
		})
		ast.WalkExprs(module, func(x *ast.Expr) bool {
			if hasFileLocation(x.Location) {
				if !report.IsCovered(x.Location.File, x.Location.Row) {
					notCovered = append(notCovered, Position{x.Location.Row})
				}
			}
			return false
		})
		notCovered.Sort()
		fr, ok := report.Files[file]
		if !ok {
			fr = &FileReport{}
			report.Files[file] = fr
		}
		fr.NotCovered = sortedPositionSliceToRangeSlice(notCovered)
	}
	return
}

// Trace updates the coverage state.
func (c *Cover) Trace(event *topdown.Event) {
	switch event.Op {
	case topdown.ExitOp:
		if rule, ok := event.Node.(*ast.Rule); ok {
			c.setHit(rule.Head.Location)
		}
	case topdown.EvalOp:
		if expr := event.Node.(*ast.Expr); expr != nil {
			c.setHit(expr.Location)
		}
	}
}

func (c *Cover) setHit(loc *ast.Location) {
	if hasFileLocation(loc) {
		hits, ok := c.hits[loc.File]
		if !ok {
			hits = map[Position]struct{}{}
			c.hits[loc.File] = hits
		}
		hits[Position{loc.Row}] = struct{}{}
	}
}

// Position represents a file location.
type Position struct {
	Row int `json:"row"`
}

// PositionSlice is a collection of position that can be sorted.
type PositionSlice []Position

// Sort sorts the slice by line number.
func (sl PositionSlice) Sort() {
	sort.Slice(sl, func(i, j int) bool {
		return sl[i].Row < sl[j].Row
	})
}

// Range represents a range of positions in a file.
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

// In returns true if the row is inside the range.
func (r Range) In(row int) bool {
	return row >= r.Start.Row && row <= r.End.Row
}

// FileReport represents a coverage report for a single file.
type FileReport struct {
	Covered    []Range `json:"covered,omitempty"`
	NotCovered []Range `json:"not_covered,omitempty"`
}

// IsCovered returns true if the row is marked as covered in the report.
func (fr *FileReport) IsCovered(row int) bool {
	if fr == nil {
		return false
	}
	for _, r := range fr.Covered {
		if r.In(row) {
			return true
		}
	}
	return false
}

// IsNotCovered returns true if the row is marked as NOT covered in the report.
// This is not the same as simply not being reported. For example, certain
// statements like imports are not included in the report.
func (fr *FileReport) IsNotCovered(row int) bool {
	if fr == nil {
		return false
	}
	for _, r := range fr.NotCovered {
		if r.In(row) {
			return true
		}
	}
	return false
}

// Report represents a coverage report for a set of files.
type Report struct {
	Files map[string]*FileReport `json:"files"`
}

// IsCovered returns true if the row in the given file is covered.
func (r Report) IsCovered(file string, row int) bool {
	return r.Files[file].IsCovered(row)
}

func sortedPositionSliceToRangeSlice(sorted []Position) (result []Range) {
	if len(sorted) == 0 {
		return
	}
	start, end := sorted[0], sorted[0]
	for i := 1; i < len(sorted); i++ {
		curr := sorted[i]
		if curr.Row == end.Row+1 {
			end = curr
		} else {
			result = append(result, Range{start, end})
			start, end = curr, curr
		}
	}
	result = append(result, Range{start, end})
	return
}

func hasFileLocation(loc *ast.Location) bool {
	return loc != nil && loc.File != ""
}
