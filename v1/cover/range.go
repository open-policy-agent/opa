// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cover

import (
	"slices"

	"github.com/open-policy-agent/opa/v1/ast"
)

// Position represents a file location.
type Position struct {
	Row int `json:"row"`
	Col int `json:"col,omitempty"`
}

// PositionSlice is a collection of position that can be sorted.
//
// Deprecated: PositionSlice is unused inside OPA and will be removed in a
// future release.
type PositionSlice []Position

// Sort sorts the slice by row, then column.
//
// Deprecated: see PositionSlice.
func (sl PositionSlice) Sort() {
	slices.SortFunc(sl, func(a, b Position) int {
		if a.Row != b.Row {
			return a.Row - b.Row
		}
		return a.Col - b.Col
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

// Compare orders ranges by start, then end, comparing row before col.
func (r Range) Compare(other Range) int {
	if r.Start.Row != other.Start.Row {
		return r.Start.Row - other.Start.Row
	}
	if r.Start.Col != other.Start.Col {
		return r.Start.Col - other.Start.Col
	}
	if r.End.Row != other.End.Row {
		return r.End.Row - other.End.Row
	}
	return r.End.Col - other.End.Col
}

// contains returns true if other is fully contained within r.
func (r Range) contains(other Range) bool {
	otherStartsWithin := r.Start.Row < other.Start.Row ||
		(r.Start.Row == other.Start.Row && r.Start.Col <= other.Start.Col)

	otherEndsWithin := r.End.Row > other.End.Row ||
		(r.End.Row == other.End.Row && r.End.Col >= other.End.Col)

	return otherStartsWithin && otherEndsWithin
}

// rangeOf returns a Range for loc, deriving the end row/col from loc.Text via
// (*ast.Location).End.
func rangeOf(loc *ast.Location) Range {
	endRow, endCol := loc.End()
	return Range{
		Start: Position{Row: loc.Row, Col: loc.Col},
		End:   Position{Row: endRow, Col: endCol},
	}
}

// uniqueRowCount returns the number of distinct rows touched by any range
// in rs. Used for line-level coverage statistics, where overlapping
// per-expression ranges must not double-count.
func uniqueRowCount(rs []Range) int {
	if len(rs) == 0 {
		return 0
	}
	rows := make(map[int]struct{}, len(rs))
	for _, r := range rs {
		for row := r.Start.Row; row <= r.End.Row; row++ {
			rows[row] = struct{}{}
		}
	}
	return len(rows)
}

// rowSpans returns sorted [start, end] row pairs covering the same set of
// rows as rs, with adjacent rows collapsed into a single span. Intended
// for line-oriented output (e.g. "file.rego:3-5") where overlapping or
// touching ranges should print as one entry.
func rowSpans(rs []Range) [][2]int {
	if len(rs) == 0 {
		return nil
	}
	rows := make([]int, 0, len(rs))
	seen := make(map[int]struct{}, len(rs))
	for _, r := range rs {
		for row := r.Start.Row; row <= r.End.Row; row++ {
			if _, ok := seen[row]; ok {
				continue
			}
			seen[row] = struct{}{}
			rows = append(rows, row)
		}
	}
	slices.Sort(rows)
	out := make([][2]int, 0, len(rows))
	start, end := rows[0], rows[0]
	for i := 1; i < len(rows); i++ {
		if rows[i] == end+1 {
			end = rows[i]
			continue
		}
		out = append(out, [2]int{start, end})
		start, end = rows[i], rows[i]
	}
	return append(out, [2]int{start, end})
}
