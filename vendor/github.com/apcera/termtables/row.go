// Copyright 2012 Apcera Inc. All rights reserved.

package termtables

import "strings"

// A Row represents one row of a Table, consisting of some number of Cell
// items.
type Row struct {
	cells []*Cell
}

// CreateRow returns a Row where the cells are created as needed to hold each
// item given; each item can be a Cell or content to go into a Cell created
// to hold it.
func CreateRow(items []interface{}) *Row {
	row := &Row{cells: []*Cell{}}
	for _, item := range items {
		row.AddCell(item)
	}
	return row
}

// AddCell adds one item to a row as a new cell, where the item is either a
// Cell or content to be put into a cell.
func (r *Row) AddCell(item interface{}) {
	if c, ok := item.(*Cell); ok {
		c.column = len(r.cells)
		r.cells = append(r.cells, c)
	} else {
		r.cells = append(r.cells, createCell(len(r.cells), item, nil))
	}
}

// Render returns a string representing the content of one row of a table, where
// the Row contains Cells (not Separators) and the representation includes any
// vertical borders needed.
func (r *Row) Render(style *renderStyle) string {
	// pre-render and shove into an array... helps with cleanly adding borders
	renderedCells := []string{}
	for _, c := range r.cells {
		renderedCells = append(renderedCells, c.Render(style))
	}

	// format final output
	return style.BorderY + strings.Join(renderedCells, style.BorderY) + style.BorderY
}
