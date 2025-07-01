// Package tw defines types and constants for table formatting and configuration,
// including validation logic for various table properties.
package tw

import (
	"fmt"
	"github.com/olekukonko/errors"
	"strings"
) // Custom error handling library

// Position defines where formatting applies in the table (e.g., header, footer, or rows).
type Position string

// Validate checks if the Position is one of the allowed values: Header, Footer, or Row.
func (pos Position) Validate() error {
	switch pos {
	case Header, Footer, Row:
		return nil // Valid position
	}
	// Return an error for any unrecognized position
	return errors.New("invalid position")
}

// Filter defines a function type for processing cell content.
// It takes a slice of strings (representing cell data) and returns a processed slice.
type Filter func([]string) []string

// Formatter defines an interface for types that can format themselves into a string.
// Used for custom formatting of table cell content.
type Formatter interface {
	Format() string // Returns the formatted string representation
}

// Align specifies the text alignment within a table cell.
type Align string

// Validate checks if the Align is one of the allowed values: None, Center, Left, or Right.
func (a Align) Validate() error {
	switch a {
	case AlignNone, AlignCenter, AlignLeft, AlignRight:
		return nil // Valid alignment
	}
	// Return an error for any unrecognized alignment
	return errors.New("invalid align")
}

type Alignment []Align

func (a Alignment) String() string {
	var str strings.Builder
	for i, a := range a {
		if i > 0 {
			str.WriteString("; ")
		}
		str.WriteString(fmt.Sprint(i))
		str.WriteString("=")
		str.WriteString(string(a))
	}
	return str.String()
}

func (a Alignment) Add(aligns ...Align) Alignment {
	aa := make(Alignment, len(aligns))
	copy(aa, aligns)
	return aa
}

func (a Alignment) Set(col int, align Align) Alignment {
	if col >= 0 && col < len(a) {
		a[col] = align
	}
	return a
}

// Copy creates a new independent copy of the Alignment
func (a Alignment) Copy() Alignment {
	aa := make(Alignment, len(a))
	copy(aa, a)
	return aa
}

// Level indicates the vertical position of a line in the table (e.g., header, body, or footer).
type Level int

// Validate checks if the Level is one of the allowed values: Header, Body, or Footer.
func (l Level) Validate() error {
	switch l {
	case LevelHeader, LevelBody, LevelFooter:
		return nil // Valid level
	}
	// Return an error for any unrecognized level
	return errors.New("invalid level")
}

// Location specifies the horizontal position of a cell or column within a table row.
type Location string

// Validate checks if the Location is one of the allowed values: First, Middle, or End.
func (l Location) Validate() error {
	switch l {
	case LocationFirst, LocationMiddle, LocationEnd:
		return nil // Valid location
	}
	// Return an error for any unrecognized location
	return errors.New("invalid location")
}

type Caption struct {
	Text  string
	Spot  Spot
	Align Align
	Width int
}

func (c Caption) WithText(text string) Caption {
	c.Text = text
	return c
}

func (c Caption) WithSpot(spot Spot) Caption {
	c.Spot = spot
	return c
}

func (c Caption) WithAlign(align Align) Caption {
	c.Align = align
	return c
}

func (c Caption) WithWidth(width int) Caption {
	c.Width = width
	return c
}

type Control struct {
	Hide State
}

// Compact configures compact width optimization for merged cells.
type Compact struct {
	Merge State // Merge enables compact width calculation during cell merging, optimizing space allocation.
}

// Behavior defines settings that control table rendering behaviors, such as column visibility and content formatting.
type Behavior struct {
	AutoHide  State // AutoHide determines whether empty columns are hidden. Ignored in streaming mode.
	TrimSpace State // TrimSpace enables trimming of leading and trailing spaces from cell content.

	Header Control // Header specifies control settings for the table header.
	Footer Control // Footer specifies control settings for the table footer.

	// Compact enables optimized width calculation for merged cells, such as in horizontal merges,
	// by systematically determining the most efficient width instead of scaling by the number of columns.
	Compact Compact
}

// Padding defines the spacing characters around cell content in all four directions.
// A zero-value Padding struct will use the table's default padding unless Overwrite is true.
type Padding struct {
	Left   string
	Right  string
	Top    string
	Bottom string

	// Overwrite forces tablewriter to use this padding configuration exactly as specified,
	// even when empty. When false (default), empty Padding fields will inherit defaults.
	//
	// For explicit no-padding, use the PaddingNone constant instead of setting Overwrite.
	Overwrite bool
}

// Common padding configurations for convenience

// Equals reports whether two Padding configurations are identical in all fields.
// This includes comparing the Overwrite flag as part of the equality check.
func (p Padding) Equals(padding Padding) bool {
	return p.Left == padding.Left &&
		p.Right == padding.Right &&
		p.Top == padding.Top &&
		p.Bottom == padding.Bottom &&
		p.Overwrite == padding.Overwrite
}

// Empty reports whether all padding strings are empty (all fields == "").
// Note that an Empty padding may still take effect if Overwrite is true.
func (p Padding) Empty() bool {
	return p.Left == "" && p.Right == "" && p.Top == "" && p.Bottom == ""
}

// Paddable reports whether this Padding configuration should override existing padding.
// Returns true if either:
//   - Any padding string is non-empty (!p.Empty())
//   - Overwrite flag is true (even with all strings empty)
//
// This is used internally during configuration merging to determine whether to
// apply the padding settings.
func (p Padding) Paddable() bool {
	return !p.Empty() || p.Overwrite
}
