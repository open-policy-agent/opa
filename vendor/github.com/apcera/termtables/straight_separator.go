// Copyright 2012 Apcera Inc. All rights reserved.

package termtables

import (
	"strings"
	"unicode/utf8"
)

// A StraightSeparator is a horizontal line with associated information about
// what sort of position it takes in the table, so as to control which shapes
// will be used where vertical lines are expected to touch this horizontal
// line.
type StraightSeparator struct {
	where lineType
}

// Render returns a string representing this separator, with all border
// crossings appropriately chosen.
func (s *StraightSeparator) Render(style *renderStyle) string {
	// loop over getting dashes
	width := 0
	for i := 0; i < style.columns; i++ {
		width += style.PaddingLeft + style.CellWidth(i) + style.PaddingRight + utf8.RuneCountInString(style.BorderI)
	}

	switch s.where {
	case LINE_TOP:
		return style.BorderTopLeft + strings.Repeat(style.BorderX, width-1) + style.BorderTopRight
	case LINE_INNER, LINE_SUBTOP:
		return style.BorderLeft + strings.Repeat(style.BorderX, width-1) + style.BorderRight
	case LINE_BOTTOM:
		return style.BorderBottomLeft + strings.Repeat(style.BorderX, width-1) + style.BorderBottomRight
	}
	panic("not reached")
}
