// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cover

import (
	"reflect"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
)

func TestRangeIn(t *testing.T) {
	r := Range{Start: Position{Row: 3}, End: Position{Row: 5}}

	tests := map[string]struct {
		row int
		exp bool
	}{
		"row before start":     {row: 2, exp: false},
		"row at start":         {row: 3, exp: true},
		"row inside":           {row: 4, exp: true},
		"row at end inclusive": {row: 5, exp: true},
		"row after end":        {row: 6, exp: false},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if got := r.In(tc.row); got != tc.exp {
				t.Fatalf("Expected %v but got %v", tc.exp, got)
			}
		})
	}
}

func TestRangeCompare(t *testing.T) {
	mk := func(sr, sc, er, ec int) Range {
		return Range{
			Start: Position{Row: sr, Col: sc},
			End:   Position{Row: er, Col: ec},
		}
	}

	tests := map[string]struct {
		a   Range
		b   Range
		exp int
	}{
		"equal": {
			a:   mk(3, 1, 3, 5),
			b:   mk(3, 1, 3, 5),
			exp: 0,
		},
		"earlier start row sorts first": {
			a:   mk(2, 10, 2, 12),
			b:   mk(3, 1, 3, 1),
			exp: -1,
		},
		"same start, later end col sorts last": {
			a:   mk(3, 1, 3, 4),
			b:   mk(3, 1, 3, 5),
			exp: -1,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := tc.a.Compare(tc.b)
			if (got < 0) != (tc.exp < 0) || (got > 0) != (tc.exp > 0) || (got == 0) != (tc.exp == 0) {
				t.Fatalf("Expected sign matching %d but got %d", tc.exp, got)
			}
		})
	}
}

func TestRangeOf(t *testing.T) {
	loc := &ast.Location{Text: []byte("false"), File: "x.rego", Row: 3, Col: 10}
	got := rangeOf(loc)
	want := Range{
		Start: Position{Row: 3, Col: 10},
		End:   Position{Row: 3, Col: 15},
	}
	if got != want {
		t.Fatalf("Expected %+v but got %+v", want, got)
	}
}

func TestUniqueRowCount(t *testing.T) {
	mkRange := func(startRow, endRow int) Range {
		return Range{
			Start: Position{Row: startRow},
			End:   Position{Row: endRow},
		}
	}

	tests := map[string]struct {
		rs  []Range
		exp int
	}{
		"empty": {
			rs:  nil,
			exp: 0,
		},
		"two ranges sharing a row": {
			rs:  []Range{mkRange(3, 3), mkRange(3, 3)},
			exp: 1,
		},
		"multi-row range plus overlap": {
			rs:  []Range{mkRange(2, 4), mkRange(3, 3), mkRange(7, 7)},
			exp: 4, // rows 2,3,4,7
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if got := uniqueRowCount(tc.rs); got != tc.exp {
				t.Fatalf("Expected %d but got %d", tc.exp, got)
			}
		})
	}
}

func TestRowSpans(t *testing.T) {
	mkRange := func(startRow, endRow int) Range {
		return Range{
			Start: Position{Row: startRow},
			End:   Position{Row: endRow},
		}
	}

	tests := map[string]struct {
		rs  []Range
		exp [][2]int
	}{
		"empty": {
			rs:  nil,
			exp: nil,
		},
		"adjacent rows merge into one span": {
			rs:  []Range{mkRange(3, 3), mkRange(4, 4), mkRange(5, 5)},
			exp: [][2]int{{3, 5}},
		},
		"gaps split into separate spans": {
			rs:  []Range{mkRange(7, 7), mkRange(2, 4), mkRange(3, 3)},
			exp: [][2]int{{2, 4}, {7, 7}},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := rowSpans(tc.rs)
			if !reflect.DeepEqual(got, tc.exp) {
				t.Fatalf("Expected %v but got %v", tc.exp, got)
			}
		})
	}
}
