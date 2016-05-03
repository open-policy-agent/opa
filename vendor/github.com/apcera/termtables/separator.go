// Copyright 2012 Apcera Inc. All rights reserved.

package termtables

import "strings"

type lineType int

// These lines are for horizontal rules; these indicate desired styling,
// but simplistic (pure ASCII) markup characters may end up leaving the
// variant lines indistinguishable from LINE_INNER.
const (
	// LINE_INNER *must* be the default; where there are vertical lines drawn
	// across an inner line, the character at that position should indicate
	// that the vertical line goes both up and down from this horizontal line.
	LINE_INNER lineType = iota

	// LINE_TOP has only descenders
	LINE_TOP

	// LINE_SUBTOP has only descenders in the middle, but goes both up and
	// down at the far left & right edges.
	LINE_SUBTOP

	// LINE_BOTTOM has only ascenders.
	LINE_BOTTOM
)

// A Separator is a horizontal rule line, with associated information which
// indicates where in a table it is, sufficient for simple cases to let
// clean tables be drawn.  If a row-spanning cell is created, then this will
// be insufficient: we can get away with hand-waving of "well, it's showing
// where the border would be" but a more capable handling will require
// structure reworking.  Patches welcome.
type Separator struct {
	where lineType
}

// Render returns the string representation of a horizontal rule line in the
// table.
func (s *Separator) Render(style *renderStyle) string {
	// loop over getting dashes
	parts := []string{}
	for i := 0; i < style.columns; i++ {
		w := style.PaddingLeft + style.CellWidth(i) + style.PaddingRight
		parts = append(parts, strings.Repeat(style.BorderX, w))
	}

	switch s.where {
	case LINE_TOP:
		return style.BorderTopLeft + strings.Join(parts, style.BorderTop) + style.BorderTopRight
	case LINE_SUBTOP:
		return style.BorderLeft + strings.Join(parts, style.BorderTop) + style.BorderRight
	case LINE_BOTTOM:
		return style.BorderBottomLeft + strings.Join(parts, style.BorderBottom) + style.BorderBottomRight
	case LINE_INNER:
		return style.BorderLeft + strings.Join(parts, style.BorderI) + style.BorderRight
	}
	panic("not reached")
}
