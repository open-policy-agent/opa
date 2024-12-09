// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package profiler computes and reports on the time spent on expressions.
package profiler

import (
	v1 "github.com/open-policy-agent/opa/v1/profiler"
)

// Profiler computes and reports on the time spent on expressions.
type Profiler = v1.Profiler

// New returns a new Profiler object.
func New() *Profiler {
	return v1.New()
}

// ExprStats represents the result of profiling an expression.
type ExprStats = v1.ExprStats

// ExprStatsAggregated represents the result of profiling an expression
// by aggregating `n` profiles.
type ExprStatsAggregated = v1.ExprStatsAggregated

func AggregateProfiles(profiles ...[]ExprStats) []ExprStatsAggregated {
	return v1.AggregateProfiles(profiles...)
}

// Report represents the profiler report for a set of files.
type Report = v1.Report

// FileReport represents a profiler report for a single file.
type FileReport = v1.FileReport
