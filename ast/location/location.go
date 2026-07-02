// Package location defines locations in Rego source code.
package location

import (
	v1 "github.com/open-policy-agent/opa/v1/ast"
)

// Location records a position in source code
type Location = v1.Location

// NewLocation returns a new Location object.
func NewLocation(text []byte, file string, row int, col int) *Location {
	return v1.NewLocation(text, file, row, col)
}
