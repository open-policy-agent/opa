// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package cover reports coverage on modules.
package cover

import (
	"sync"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/topdown"
)

// Cover computes and reports on coverage.
type Cover struct {
	mu               sync.Mutex
	coveredRanges    fileRangeSets
	notCoveredRanges notCoveredRangesByReason
}

// notCoveredRangesByReason groups per-file not-covered range sets by reason.
type notCoveredRangesByReason struct {
	indexExcluded fileRangeSets
}

// New returns a new Cover object.
func New() *Cover {
	return &Cover{
		coveredRanges: fileRangeSets{},
		notCoveredRanges: notCoveredRangesByReason{
			indexExcluded: fileRangeSets{},
		},
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
		ReportOps:     []topdown.Op{topdown.IndexExcludedOp},
	}
}

// Report returns a coverage Report for the given modules.
func (c *Cover) Report(modules map[string]*ast.Module) (report Report) {
	// No caller reports mid-trace, but guard so that doing so would only be a
	// stale read.
	c.mu.Lock()
	defer c.mu.Unlock()

	report.Files = map[string]*FileReport{}
	for file, coveredRanges := range c.coveredRanges {
		fr, ok := report.Files[file]
		if !ok {
			fr = &FileReport{}
			report.Files[file] = fr
		}
		fr.Covered = coveredRanges.Slice()
	}
	for file, module := range modules {
		notCoveredRanges := rangeSet{}
		fr, ok := report.Files[file]
		if !ok {
			fr = &FileReport{}
			report.Files[file] = fr
		}
		ast.WalkRules(module, func(x *ast.Rule) bool {
			if rng, notCovered := report.notCoveredAt(x.Head.Location); notCovered {
				notCoveredRanges.Add(rng)
			}
			return false
		})
		ast.WalkExprs(module, func(x *ast.Expr) bool {
			if includeExprInCoverage(x) {
				if rng, notCovered := report.notCoveredAt(x.Location); notCovered {
					notCoveredRanges.Add(rng)
				}
			}
			return false
		})
		fr.NotCovered = notCoveredRanges.Slice()

		// Annotate fr.NotCovered with the reason each range wasn't covered,
		// using what Cover recorded during evaluation (e.g. indexer exclusion).
		//
		// module map key can differ from parse-time Location.File (e.g. bundle
		// modules), so look up exclusions by the package's actual source file.
		locFile := module.Package.Location.File
		// Assumes this Cover is used once.
		if excl := c.notCoveredRanges.indexExcluded[locFile]; len(excl) > 0 {
			// Only trust exclusions that are also proven not covered by a real
			// hit, so a rule reached via another path (e.g. a `with` overlay
			// using a different index) can never be mislabeled.
			for i := range fr.NotCovered {
				if excl.Contains(fr.NotCovered[i]) {
					fr.NotCovered[i].Kind = KindIndexExcluded
				}
			}
		}
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
			c.recordCovered(rule.Head.Location)
		}
	case topdown.EvalOp:
		if expr := event.Node.(*ast.Expr); expr != nil {
			c.recordCovered(expr.Location)
		}
	case topdown.IndexExcludedOp:
		if rule, ok := event.Node.(*ast.Rule); ok {
			// The indexer excludes the rule by its head; since the body
			// never runs either, mark its expressions index-excluded too.
			locs := make([]*ast.Location, 0, len(rule.Body)+1)
			locs = append(locs, rule.Head.Location)
			for _, expr := range rule.Body {
				if includeExprInCoverage(expr) {
					locs = append(locs, expr.Location)
				}
			}
			c.recordNotCovered(KindIndexExcluded, locs...)
		}
	}
}

// recordCovered marks loc as covered, unless loc has no file (e.g. generated code).
func (c *Cover) recordCovered(loc *ast.Location) {
	c.record(c.coveredRanges, loc)
}

// recordNotCovered marks locs as not covered for the given reason, unless a
// loc has no file (e.g. generated code).
func (c *Cover) recordNotCovered(kind Kind, locs ...*ast.Location) {
	switch kind {
	case KindIndexExcluded:
		c.record(c.notCoveredRanges.indexExcluded, locs...)
	}
}

// record adds each loc's range to m under a single lock, skipping locs with
// no file (e.g. generated code).
func (c *Cover) record(m fileRangeSets, locs ...*ast.Location) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, loc := range locs {
		if loc.HasFile() {
			m.Add(loc.File, rangeOf(loc))
		}
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

// notCoveredAt returns loc's range and true, unless loc has no file or its
// range is already covered.
func (r Report) notCoveredAt(loc *ast.Location) (Range, bool) {
	if !loc.HasFile() {
		return Range{}, false
	}
	rng := rangeOf(loc)
	return rng, !r.Files[loc.File].isRangeCovered(rng)
}

// Check the expression and return true if it should be included in the coverage report
func includeExprInCoverage(x *ast.Expr) bool {
	_, excludeExprType := x.Terms.(*ast.SomeDecl)

	return !excludeExprType && x.Location.HasFile()
}
