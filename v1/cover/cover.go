// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package cover reports coverage on modules.
package cover

import (
	"slices"
	"sync"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/topdown"
)

// Cover computes and reports on coverage.
type Cover struct {
	mu   sync.Mutex
	hits map[string]map[Range]struct{}
}

// New returns a new Cover object.
func New() *Cover {
	return &Cover{
		hits: map[string]map[Range]struct{}{},
	}
}

// Enabled returns true if coverage is enabled.
func (*Cover) Enabled() bool {
	return true
}

// Config returns the standard Tracer configuration for the Cover tracer
func (*Cover) Config() topdown.TraceConfig {
	return topdown.TraceConfig{
		PlugLocalVars: false, // Event variable metadata is not required for the Coverage report
	}
}

// Report returns a coverage Report for the given modules.
func (c *Cover) Report(modules map[string]*ast.Module) (report Report) {
	report.Files = map[string]*FileReport{}
	for file, hits := range c.hits {
		covered := make([]Range, 0, len(hits))
		for r := range hits {
			covered = append(covered, r)
		}
		slices.SortFunc(covered, Range.Compare)
		fr, ok := report.Files[file]
		if !ok {
			fr = &FileReport{}
			report.Files[file] = fr
		}
		fr.Covered = covered
	}
	for file, module := range modules {
		notCovered := map[Range]struct{}{}
		ast.WalkRules(module, func(x *ast.Rule) bool {
			if x.Head.Location.HasFile() {
				headRange := rangeOf(x.Head.Location)
				if !report.Files[x.Head.Location.File].isRangeCovered(headRange) {
					notCovered[headRange] = struct{}{}
				}
			}
			return false
		})
		ast.WalkExprs(module, func(x *ast.Expr) bool {
			if includeExprInCoverage(x) {
				exprRange := rangeOf(x.Location)
				if !report.Files[x.Location.File].isRangeCovered(exprRange) {
					notCovered[exprRange] = struct{}{}
				}
			}
			return false
		})
		ranges := make([]Range, 0, len(notCovered))
		for r := range notCovered {
			ranges = append(ranges, r)
		}
		slices.SortFunc(ranges, Range.Compare)
		fr, ok := report.Files[file]
		if !ok {
			fr = &FileReport{}
			report.Files[file] = fr
		}
		fr.NotCovered = ranges
	}

	var coveredLoc, notCoveredLoc int
	var overallCoverage float64

	for _, fr := range report.Files {
		fr.Coverage = fr.computeCoveragePercentage()
		fr.CoveredLines = fr.locCovered()
		fr.NotCoveredLines = fr.locNotCovered()
		coveredLoc += fr.CoveredLines
		notCoveredLoc += fr.NotCoveredLines
	}
	totalLoc := coveredLoc + notCoveredLoc

	if totalLoc != 0 {
		overallCoverage = 100.0 * float64(coveredLoc) / float64(totalLoc)
	}
	report.CoveredLines = coveredLoc
	report.NotCoveredLines = notCoveredLoc
	report.Coverage = overallCoverage

	return
}

// Trace updates the coverage state.
//
// Deprecated: Use TraceEvent instead.
func (c *Cover) Trace(event *topdown.Event) {
	c.TraceEvent(*event)
}

// TraceEvent updates the coverage state.
func (c *Cover) TraceEvent(event topdown.Event) {
	switch event.Op {
	case topdown.ExitOp:
		if rule, ok := event.Node.(*ast.Rule); ok {
			c.setHit(rule.Head.Location)
		}
	case topdown.EvalOp:
		if expr := event.Node.(*ast.Expr); expr != nil {
			c.setHit(expr.Location)
		}
	}
}

func (c *Cover) setHit(loc *ast.Location) {
	if loc.HasFile() {
		c.mu.Lock()
		defer c.mu.Unlock()
		hits, ok := c.hits[loc.File]
		if !ok {
			hits = map[Range]struct{}{}
			c.hits[loc.File] = hits
		}
		hits[rangeOf(loc)] = struct{}{}
	}
}

// Report represents a coverage report for a set of files.
type Report struct {
	Files           map[string]*FileReport `json:"files"`
	CoveredLines    int                    `json:"covered_lines"`
	NotCoveredLines int                    `json:"not_covered_lines"`
	Coverage        float64                `json:"coverage"`
}

// IsCovered returns true if the row in the given file is covered.
func (r Report) IsCovered(file string, row int) bool {
	return r.Files[file].IsCovered(row)
}

// Check the expression and return true if it should be included in the coverage report
func includeExprInCoverage(x *ast.Expr) bool {
	_, excludeExprType := x.Terms.(*ast.SomeDecl)

	return !excludeExprType && x.Location.HasFile()
}
