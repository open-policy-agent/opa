// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package profiler computes and reports on the time spent on expressions.
package profiler

import (
	"cmp"
	"time"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/metrics"
	"github.com/open-policy-agent/opa/v1/topdown"
	"github.com/open-policy-agent/opa/v1/util"
)

var unknownLocation = ast.NewLocation([]byte("???"), "", 0, 0)

// Profiler computes and reports on the time spent on expressions.
type Profiler struct {
	hits            map[string]map[int]ExprStats
	hitsByExprIndex map[string]map[int]map[int]ExprStats
	activeTimer     time.Time
	prevExpr        exprInfo
}

// exprInfo stores information about an expression.
type exprInfo struct {
	index    int
	location *ast.Location
	op       topdown.Op
}

// New returns a new Profiler object.
func New() *Profiler {
	return &Profiler{
		hits:            map[string]map[int]ExprStats{},
		hitsByExprIndex: map[string]map[int]map[int]ExprStats{},
	}
}

// Enabled returns true if profiler is enabled.
func (p *Profiler) Enabled() bool {
	return p != nil
}

// Config returns the standard Tracer configuration for the profiler
func (*Profiler) Config() topdown.TraceConfig {
	return topdown.TraceConfig{
		PlugLocalVars: false, // Event variable metadata is not required for the Profiler
	}
}

// ReportByFile returns a profiler report for expressions grouped by the
// file name. For each file the results are sorted by increasing row number.
func (p *Profiler) ReportByFile() Report {
	p.processLastExpr()

	report := Report{Files: make(map[string]*FileReport, len(p.hits))}

	for file, hits := range p.hits {
		stats := make([]ExprStats, 0, len(hits))
		for row, stat := range hits {
			if entry, ok := p.hitsByExprIndex[file][row]; ok {
				stat.NumGenExpr = len(entry)
			}
			stats = append(stats, stat)
		}

		fr, ok := report.Files[file]
		if !ok {
			fr = &FileReport{}
			report.Files[file] = fr
		}
		fr.Result = util.SortedFunc(stats, cmpLineAsc)
	}

	return report
}

// ReportTopNResults returns the top N results based on the given
// criteria. If N <= 0, all the results based on the criteria are returned.
func (p *Profiler) ReportTopNResults(numResults int, criteria []string) []ExprStats {
	p.processLastExpr()

	n := 0
	for _, hits := range p.hits {
		n += len(hits)
	}

	stats := make(exprStatsSlice, 0, n)
	for file, hits := range p.hits {
		for row, stat := range hits {
			if entry, ok := p.hitsByExprIndex[file][row]; ok {
				stat.NumGenExpr = len(entry)
			}
			stats = append(stats, stat)
		}
	}

	return stats.orderedBy(criteria...).limit(numResults)
}

// Trace updates the profiler state.
//
// Deprecated: Use TraceEvent instead.
func (p *Profiler) Trace(event *topdown.Event) {
	p.TraceEvent(*event)
}

// TraceEvent updates the coverage state.
func (p *Profiler) TraceEvent(event topdown.Event) {
	switch event.Op {
	case topdown.EvalOp, topdown.RedoOp:
		if expr, ok := event.Node.(*ast.Expr); ok && expr != nil {
			p.processExpr(expr, event.Op)
		}
	}
}

func (p *Profiler) processExpr(expr *ast.Expr, eventType topdown.Op) {
	if expr.Location == nil {
		// add fake location to group expressions without a location
		expr.Location = unknownLocation
	}

	// set the active timer on the first expression
	if p.activeTimer.IsZero() {
		p.activeTimer = time.Now()
		p.prevExpr = exprInfo{
			op:       eventType,
			location: expr.Location,
			index:    expr.Index,
		}
		return
	}

	// record the profiler results for the previous expression
	p.calculateHitsByExprIndex()

	file := p.prevExpr.location.File
	hits, ok := p.hits[file]
	if !ok {
		hits = map[int]ExprStats{}
		hits[p.prevExpr.location.Row] = getProfilerStats(p.prevExpr, p.activeTimer)
		p.hits[file] = hits
	} else {
		pos := p.prevExpr.location.Row
		pStats, ok := hits[pos]
		if !ok {
			hits[pos] = getProfilerStats(p.prevExpr, p.activeTimer)
		} else {
			pStats.ExprTimeNs += time.Since(p.activeTimer).Nanoseconds()

			switch p.prevExpr.op {
			case topdown.EvalOp:
				pStats.NumEval++
			case topdown.RedoOp:
				pStats.NumRedo++
			}
			hits[pos] = pStats
		}
	}

	// reset active timer and expression
	p.activeTimer = time.Now()
	p.prevExpr = exprInfo{
		op:       eventType,
		location: expr.Location,
		index:    expr.Index,
	}
}

func (p *Profiler) processLastExpr() {
	expr := ast.Expr{
		Location: p.prevExpr.location,
		Index:    p.prevExpr.index,
	}
	p.processExpr(&expr, p.prevExpr.op)
}

func (p *Profiler) calculateHitsByExprIndex() {
	file := p.prevExpr.location.File

	hitsUnique, ok := p.hitsByExprIndex[file]
	if !ok {
		p.hitsByExprIndex[file] = map[int]map[int]ExprStats{
			p.prevExpr.location.Row: {p.prevExpr.index: getProfilerStats(p.prevExpr, p.activeTimer)},
		}
	} else {
		row := p.prevExpr.location.Row
		idx := p.prevExpr.index

		pStats, ok := hitsUnique[row]
		if !ok {
			hitsUnique[row] = map[int]ExprStats{idx: getProfilerStats(p.prevExpr, p.activeTimer)}
		} else {
			pStatsIdx, ok := pStats[idx]
			if !ok {
				hitsUnique[row][idx] = getProfilerStats(p.prevExpr, p.activeTimer)
			} else {
				pStatsIdx.ExprTimeNs += time.Since(p.activeTimer).Nanoseconds()

				switch p.prevExpr.op {
				case topdown.EvalOp:
					pStatsIdx.NumEval++
				case topdown.RedoOp:
					pStatsIdx.NumRedo++
				}

				hitsUnique[row][idx] = pStatsIdx
			}
		}
	}
}

func getProfilerStats(expr exprInfo, timer time.Time) ExprStats {
	profilerStats := ExprStats{
		ExprTimeNs: time.Since(timer).Nanoseconds(),
		Location:   expr.location,
	}

	switch expr.op {
	case topdown.EvalOp:
		profilerStats.NumEval = 1
	case topdown.RedoOp:
		profilerStats.NumRedo = 1
	}
	return profilerStats
}

// ExprStats represents the result of profiling an expression.
type ExprStats struct {
	ExprTimeNs int64         `json:"total_time_ns"`
	NumEval    int           `json:"num_eval"`
	NumRedo    int           `json:"num_redo"`
	NumGenExpr int           `json:"num_gen_expr"`
	Location   *ast.Location `json:"location"`
}

type exprStatsSlice []ExprStats

// ExprStatsAggregated represents the result of profiling an expression
// by aggregating `n` profiles.
type ExprStatsAggregated struct {
	ExprTimeNsStats any           `json:"total_time_ns_stats"`
	NumEval         int           `json:"num_eval"`
	NumRedo         int           `json:"num_redo"`
	NumGenExpr      int           `json:"num_gen_expr"`
	Location        *ast.Location `json:"location"`
}

func aggregate(stats ...ExprStats) ExprStatsAggregated {
	if len(stats) == 0 {
		return ExprStatsAggregated{}
	}
	timeNs := make([]int64, 0, len(stats))
	for _, s := range stats {
		timeNs = append(timeNs, s.ExprTimeNs)
	}
	return ExprStatsAggregated{
		NumEval:         stats[0].NumEval,
		NumRedo:         stats[0].NumRedo,
		NumGenExpr:      stats[0].NumGenExpr,
		Location:        stats[0].Location,
		ExprTimeNsStats: metrics.Statistics(timeNs...),
	}
}

func AggregateProfiles(profiles ...[]ExprStats) (res []ExprStatsAggregated) {
	if len(profiles) > 0 {
		res = make([]ExprStatsAggregated, len(profiles[0]))
		for j := range profiles[0] {
			s := make(exprStatsSlice, 0, len(profiles))
			for _, p := range profiles {
				s = append(s, p[j])
			}
			res[j] = aggregate(s...)
		}
	}
	return res
}

// Report represents the profiler report for a set of files.
type Report struct {
	Files map[string]*FileReport `json:"files"`
}

// FileReport represents a profiler report for a single file.
type FileReport struct {
	Result []ExprStats `json:"result"`
}

func (e exprStatsSlice) orderedBy(criteria ...string) exprStatsSlice {
	if len(criteria) == 0 {
		return e
	}

	allComparers := map[string]func(p1, p2 ExprStats) int{
		"total_time_ns": cmpTotalTimeNs,
		"num_eval":      cmpNumEval,
		"num_redo":      cmpNumRedo,
		"num_gen_expr":  cmpNumGenExpr,
		"file":          cmpFile,
		"line":          cmpLineDsc,
	}

	criteriaComparers := make([]func(ExprStats, ExprStats) int, 0, len(criteria))
	for _, c := range criteria {
		if fn, ok := allComparers[c]; ok {
			criteriaComparers = append(criteriaComparers, fn)
		}
	}

	return util.SortedFunc(e, func(a, b ExprStats) int {
		for _, comparer := range criteriaComparers {
			if res := comparer(a, b); res != 0 {
				return res
			}
		}
		return 0
	})
}

func (e exprStatsSlice) limit(n int) exprStatsSlice {
	if n <= 0 {
		return e
	}
	return e[:min(n, len(e))]
}

func cmpTotalTimeNs(stat1, stat2 ExprStats) int {
	return int(stat2.ExprTimeNs - stat1.ExprTimeNs)
}

func cmpNumEval(stat1, stat2 ExprStats) int {
	return stat2.NumEval - stat1.NumEval
}

func cmpNumRedo(stat1, stat2 ExprStats) int {
	return stat2.NumRedo - stat1.NumRedo
}

func cmpNumGenExpr(stat1, stat2 ExprStats) int {
	return stat2.NumGenExpr - stat1.NumGenExpr
}

func cmpFile(stat1, stat2 ExprStats) int {
	return cmp.Compare(stat2.Location.File, stat1.Location.File)
}

func cmpLineDsc(stat1, stat2 ExprStats) int {
	return cmp.Compare(stat2.Location.Row, stat1.Location.Row)
}

func cmpLineAsc(stat1, stat2 ExprStats) int {
	return cmp.Compare(stat1.Location.Row, stat2.Location.Row)
}
