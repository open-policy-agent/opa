// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cover

import (
	"slices"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/util"
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

// Range represents a range of positions in a file. Kinds categorizes why a
// range was not covered, e.g. KindIndexExcluded; it's empty when covered or
// the reason is unknown, and can hold more than one Kind.
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
	Kinds []Kind   `json:"kinds,omitempty"`
}

// Kind categorizes why a Range was not covered.
type Kind string

const (
	// KindIndexExcluded marks a not-covered range that the rule indexer
	// excluded without attempting it.
	KindIndexExcluded Kind = "index_excluded"

	// KindEarlyExit marks a not-covered range that early-exit optimizations
	// skipped without attempting it.
	KindEarlyExit Kind = "early_exit"
)

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

// rangeKey identifies a Range by position only, so Kinds never participates
// in a rangeSet's identity.
type rangeKey struct {
	Start Position
	End   Position
}

func (r Range) key() rangeKey {
	return rangeKey{Start: r.Start, End: r.End}
}

// rangeSet is a set of ranges, keyed by position only (see Range.key).
type rangeSet map[rangeKey]Range

// Add inserts r into the set.
func (s rangeSet) Add(r Range) {
	s[r.key()] = r
}

// Slice returns the ranges in the set as a slice, sorted by Range.Compare.
func (s rangeSet) Slice() []Range {
	rs := make([]Range, 0, len(s))
	for _, r := range s {
		rs = append(rs, r)
	}
	return util.SortedFunc(rs, Range.Compare)
}

// fileRangeSets maps a file to the set of ranges recorded for it.
type fileRangeSets map[string]rangeSet

// Add records r against file, creating its rangeSet on first use.
func (m fileRangeSets) Add(file string, r Range) {
	s, ok := m[file]
	if !ok {
		s = rangeSet{}
		m[file] = s
	}
	s.Add(r)
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
