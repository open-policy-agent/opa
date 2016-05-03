// Copyright 2012 Apcera Inc. All rights reserved.

package termtables

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

var (
	// Must match SGR escape sequence, which is "CSI Pm m", where the Control
	// Sequence Introducer (CSI) is "ESC ["; where Pm is "A multiple numeric
	// parameter composed of any number of single numeric parameters, separated
	// by ; character(s).  Individual values for the parameters are listed with
	// Ps" and where Ps is A single (usually optional) numeric parameter,
	// composed of one of [sic] more digits."
	//
	// In practice, the end sequence is usually given as \e[0m but reading that
	// definition, it's clear that the 0 is optional and some testing confirms
	// that it is certainly optional with MacOS Terminal 2.3, so we need to
	// support the string \e[m as a terminator too.
	colorFilter = regexp.MustCompile(`\033\[(?:\d+(?:;\d+)*)?m`)
)

// A Cell denotes one cell of a table; it spans one row and a variable number
// of columns.  A given Cell can only be used at one place in a table; the act
// of adding the Cell to the table mutates it with position information, so
// do not create one "const" Cell to add it multiple times.
type Cell struct {
	column         int
	formattedValue string
	alignment      *tableAlignment
	colSpan        int
}

// CreateCell returns a Cell where the content is the supplied value, with the
// optional supplied style (which may be given as nil).  The style can include
// a non-zero ColSpan to cause the cell to become column-spanning.  Changing
// the style afterwards will not adjust the column-spanning state of the cell
// itself.
func CreateCell(v interface{}, style *CellStyle) *Cell {
	return createCell(0, v, style)
}

func createCell(column int, v interface{}, style *CellStyle) *Cell {
	cell := &Cell{column: column, formattedValue: renderValue(v), colSpan: 1}
	if style != nil {
		cell.alignment = &style.Alignment
		if style.ColSpan != 0 {
			cell.colSpan = style.ColSpan
		}
	}
	return cell
}

// Width returns the width of the content of the cell, measured in runes; if
// each rune is a single rendering glyph and not "wide", then this is
// sufficient to calculate the width for rendering purposes.  This will fail
// on more sophisticated Unicode; in which case, this is the place to plug in
// better logic for "measuring" the display width.  Around about then, you
// run into some fundamental limitations of a cell grid display model as is
// used in ttys.
func (c *Cell) Width() int {
	return utf8.RuneCountInString(filterColorCodes(c.formattedValue))
}

// Filter out terminal bold/color sequences in a string.
// This supports only basic bold/color escape sequences.
func filterColorCodes(s string) string {
	return colorFilter.ReplaceAllString(s, "")
}

// Render returns a string representing the content of the cell, together with
// padding (to the widths specified) and handling any alignment.
func (c *Cell) Render(style *renderStyle) (buffer string) {
	// if no alignment is set, import the table's default
	if c.alignment == nil {
		c.alignment = &style.Alignment
	}

	// left padding
	buffer += strings.Repeat(" ", style.PaddingLeft)

	// append the main value and handle alignment
	buffer += c.alignCell(style)

	// right padding
	buffer += strings.Repeat(" ", style.PaddingRight)

	// this handles escaping for, eg, Markdown, where we don't care about the
	// alignment quite as much
	if style.replaceContent != nil {
		buffer = style.replaceContent(buffer)
	}

	return buffer
}

func (c *Cell) alignCell(style *renderStyle) string {
	buffer := ""
	width := style.CellWidth(c.column)

	if c.colSpan > 1 {
		for i := 1; i < c.colSpan; i++ {
			w := style.CellWidth(c.column + i)
			if w == 0 {
				break
			}
			width += style.PaddingLeft + w + style.PaddingRight + utf8.RuneCountInString(style.BorderY)
		}
	}

	switch *c.alignment {

	default:
		buffer += c.formattedValue
		if l := width - c.Width(); l > 0 {
			buffer += strings.Repeat(" ", l)
		}

	case AlignLeft:
		buffer += c.formattedValue
		if l := width - c.Width(); l > 0 {
			buffer += strings.Repeat(" ", l)
		}

	case AlignRight:
		if l := width - c.Width(); l > 0 {
			buffer += strings.Repeat(" ", l)
		}
		buffer += c.formattedValue

	case AlignCenter:
		left, right := 0, 0
		if l := width - c.Width(); l > 0 {
			lf := float64(l)
			left = int(math.Floor(lf / 2))
			right = int(math.Ceil(lf / 2))
		}
		buffer += strings.Repeat(" ", left)
		buffer += c.formattedValue
		buffer += strings.Repeat(" ", right)
	}

	return buffer
}

// Format the raw value as a string depending on the type
func renderValue(v interface{}) string {
	switch vv := v.(type) {
	case string:
		return vv
	case bool:
		return strconv.FormatBool(vv)
	case int:
		return strconv.Itoa(vv)
	case int64:
		return strconv.FormatInt(vv, 10)
	case uint64:
		return strconv.FormatUint(vv, 10)
	case float64:
		return strconv.FormatFloat(vv, 'f', 2, 64)
	case fmt.Stringer:
		return vv.String()
	}
	return fmt.Sprintf("%v", v)
}
