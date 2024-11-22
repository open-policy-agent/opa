// Copyright 2021 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package refactor implements different refactoring operations over Rego modules.
package refactor

import (
	v1 "github.com/open-policy-agent/opa/v1/refactor"
)

// Error defines the structure of errors returned by refactor.
type Error = v1.Error

// Refactor implements different refactoring operations over Rego modules eg. renaming packages.
type Refactor = v1.Refactor

// New returns a new Refactor object.
func New() *Refactor {
	return v1.New()
}

// MoveQuery holds the set of Rego modules whose package paths and other references are to be rewritten
// as per the mapping defined in SrcDstMapping.
// If validate is true, the moved modules will be compiled to ensure they are valid.
type MoveQuery = v1.MoveQuery

// MoveQueryResult defines the output of a move query and holds the rewritten modules with updated packages paths
// and references.
type MoveQueryResult = v1.MoveQueryResult
