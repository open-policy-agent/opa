// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package cover reports coverage on modules.
//
// Deprecated: This package is intended for older projects transitioning from OPA v0.x and will remain for the lifetime of OPA v1.x, but its use is not recommended.
// For newer features and behaviours, such as defaulting to the Rego v1 syntax, use the corresponding components in the [github.com/open-policy-agent/opa/v1] package instead.
// See https://www.openpolicyagent.org/docs/latest/v0-compatibility/ for more information.
package cover

import (
	v1 "github.com/open-policy-agent/opa/v1/cover"
)

// Cover computes and reports on coverage.
type Cover = v1.Cover

// New returns a new Cover object.
func New() *Cover {
	return v1.New()
}

// Position represents a file location.
type Position = v1.Position

// PositionSlice is a collection of position that can be sorted.
//
// Deprecated: PositionSlice is unused inside OPA and will be removed in a
// future release.
type PositionSlice = v1.PositionSlice

// Range represents a range of positions in a file.
type Range = v1.Range

// FileReport represents a coverage report for a single file.
type FileReport = v1.FileReport

// Report represents a coverage report for a set of files.
type Report = v1.Report

// CoverageThresholdError represents an error raised when the global
// code coverage percentage is lower than the specified threshold.
type CoverageThresholdError = v1.CoverageThresholdError
