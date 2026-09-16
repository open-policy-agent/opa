// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Deprecated: This package is intended for older projects transitioning from OPA v0.x and will remain for the lifetime of OPA v1.x, but its use is not recommended.
// For newer features and behaviours, such as defaulting to the Rego v1 syntax, use the corresponding components in the [github.com/open-policy-agent/opa/v1] package instead.
// See https://www.openpolicyagent.org/docs/latest/v0-compatibility/ for more information.
//
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
