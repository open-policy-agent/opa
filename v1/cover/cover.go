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
	mu                sync.Mutex
	coveredRanges     fileRangeSets
	supplementaryRuns []supplementaryRun
}

// supplementaryRun pairs a supplementary Cover with the Kind that explains
// why evaluating under that configuration produced coverage the baseline
// didn't.
type supplementaryRun struct {
	kind          Kind
	supplementary *Cover
}

// New returns a new Cover object.
func New() *Cover {
	return &Cover{
		coveredRanges: fileRangeSets{},
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

// AddRun registers supplementary as a Cover populated by evaluating the
// same query with e.g. indexing or early exit disabled. In Report, any
// range that's NotCovered but Covered in supplementary is tagged with kind.
func (c *Cover) AddRun(kind Kind, supplementary *Cover) {
	if supplementary == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.supplementaryRuns = append(c.supplementaryRuns, supplementaryRun{kind: kind, supplementary: supplementary})
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

	// Precompute each supplementary run's own Report once, rather than per
	// file below — Report() re-walks every module, so doing it per file
	// would be quadratic in the number of files.
	supplementaryReports := make([]Report, len(c.supplementaryRuns))
	for i, run := range c.supplementaryRuns {
		supplementaryReports[i] = run.supplementary.Report(modules)
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

		// Tag ranges that supplementary runs annotated with kind.
		locFile := file
		if module.Package != nil && module.Package.Location != nil {
			locFile = module.Package.Location.File
		}
		for i, run := range c.supplementaryRuns {
			supplementaryFR := supplementaryReports[i].Files[locFile]
			for j := range fr.NotCovered {
				if supplementaryFR.isRangeCovered(fr.NotCovered[j]) {
					fr.NotCovered[j].Kinds = append(fr.NotCovered[j].Kinds, run.kind)
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
	}
}

// recordCovered marks loc as covered, unless loc has no file (e.g. generated code).
func (c *Cover) recordCovered(loc *ast.Location) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if loc.HasFile() {
		c.coveredRanges.Add(loc.File, rangeOf(loc))
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
