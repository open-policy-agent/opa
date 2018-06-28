// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package profiler computes and reports on the time spent on expressions
package profiler

import (
	"sort"
	"time"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/topdown"
)

// Profiler computes and reports on the time spent on expressions
type Profiler struct {
	hits        map[string]map[Position]ExprStats
	activeTimer time.Time
	prevExpr    ExprInfo
}

// ExprInfo stores information about an expression
type ExprInfo struct {
	index int
	file  string
	row   int
	col   int
	text  []byte
	op    string
}

// New returns a new Profiler object
func New() *Profiler {
	return &Profiler{
		hits: map[string]map[Position]ExprStats{},
	}
}

// Enabled returns true if profiler is enabled.
func (p *Profiler) Enabled() bool {
	return true
}

// Report returns a profiler report
func (p *Profiler) Report() (report Report) {
	p.processLastExpr()
	report.Files = map[string]*FileReport{}
	for file, hits := range p.hits {
		stats := []ExprStats{}
		for _, stat := range hits {
			stats = append(stats, stat)
		}

		sortStatsByRow(stats)
		fr, ok := report.Files[file]
		if !ok {
			fr = &FileReport{}
			report.Files[file] = fr
		}
		fr.Result = stats
	}
	return report
}

// Trace updates the profiler state
func (p *Profiler) Trace(event *topdown.Event) {
	switch event.Op {
	case topdown.EvalOp:
		if expr, ok := event.Node.(*ast.Expr); ok && expr != nil {
			p.processExpr(expr, "Eval")
		}
	case topdown.RedoOp:
		if expr, ok := event.Node.(*ast.Expr); ok && expr != nil {
			p.processExpr(expr, "Redo")
		}
	}
}

func (p *Profiler) processExpr(expr *ast.Expr, op string) {

	// set the active timer on the first expression
	if p.activeTimer.IsZero() {
		p.activeTimer = time.Now()
		p.prevExpr = ExprInfo{
			index: expr.Index,
			file:  expr.Location.File,
			row:   expr.Location.Row,
			col:   expr.Location.Col,
			op:    op,
			text:  expr.Location.Text,
		}
		return
	}

	// record the profiler results for the previous expression
	hits, ok := p.hits[p.prevExpr.file]
	if !ok {
		hits = map[Position]ExprStats{}
		hits[Position{p.prevExpr.row}] = getProfilerStats(p.prevExpr, p.activeTimer)
		p.hits[p.prevExpr.file] = hits
	} else {
		pStats, ok := hits[Position{p.prevExpr.row}]
		if !ok {
			hits[Position{p.prevExpr.row}] = getProfilerStats(p.prevExpr, p.activeTimer)
		} else {
			pStats.TotalTimeNs += time.Since(p.activeTimer).Nanoseconds()
			if p.prevExpr.op == "Eval" {
				pStats.NumEval++
			} else {
				pStats.NumRedo++
			}
			hits[Position{p.prevExpr.row}] = pStats
		}
	}

	// reset active timer and expression
	p.activeTimer = time.Now()
	p.prevExpr = ExprInfo{
		index: expr.Index,
		file:  expr.Location.File,
		row:   expr.Location.Row,
		col:   expr.Location.Col,
		op:    op,
		text:  expr.Location.Text,
	}
}

func (p *Profiler) processLastExpr() {
	loc := ast.NewLocation(p.prevExpr.text, p.prevExpr.file, p.prevExpr.row, p.prevExpr.col)
	expr := ast.Expr{
		Location: loc,
		Index:    p.prevExpr.index,
	}
	p.processExpr(&expr, p.prevExpr.op)
}

// Position represents a file location.
type Position struct {
	Row int `json:"row"`
}

func getProfilerStats(expr ExprInfo, timer time.Time) ExprStats {
	profilerStats := ExprStats{}
	profilerStats.TotalTimeNs = time.Since(timer).Nanoseconds()
	profilerStats.Index = expr.index

	profilerStats.Row = expr.row
	profilerStats.Col = expr.col
	profilerStats.Text = string(expr.text)

	if expr.op == "Eval" {
		profilerStats.NumEval = 1
	} else {
		profilerStats.NumRedo = 1
	}

	return profilerStats
}

// ExprStats represents the result of profiling an expression
type ExprStats struct {
	Index       int    `json:"index"`
	TotalTimeNs int64  `json:"totalTimeNs"`
	NumEval     int    `json:"numEval"`
	NumRedo     int    `json:"numRedo"`
	Row         int    `json:"row"`
	Col         int    `json:"col"`
	Text        string `json:"text"`
}

func sortStatsByRow(ps []ExprStats) {
	sort.Slice(ps, func(i, j int) bool {
		return ps[i].Row < ps[j].Row
	})
}

// Report represents the profiler report for a set of files.
type Report struct {
	Files map[string]*FileReport `json:"files"`
}

// FileReport represents a profiler report for a single file
type FileReport struct {
	Result []ExprStats `json:"result"`
}
