// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package profiler computes and reports on the time spent on expressions.
package profiler

import (
	"sort"
	"time"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/metrics"
	"github.com/open-policy-agent/opa/topdown"
)

// Profiler computes and reports on the time spent on expressions.
type Profiler struct {
	hits        map[string]map[int]ExprStats
	activeTimer time.Time
	prevExpr    exprInfo
}

// exprInfo stores information about an expression.
type exprInfo struct {
	location *ast.Location
	op       topdown.Op
}

// New returns a new Profiler object.
func New() *Profiler {
	return &Profiler{
		hits: map[string]map[int]ExprStats{},
	}
}

// Enabled returns true if profiler is enabled.
func (*Profiler) Enabled() bool {
	return true
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

	report := Report{Files: map[string]*FileReport{}}
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

// ReportTopNResults returns the top N results based on the given
// criteria. If N <= 0, all the results based on the criteria are returned.
func (p *Profiler) ReportTopNResults(numResults int, criteria []string) []ExprStats {
	p.processLastExpr()
	stats := []ExprStats{}

	for _, hits := range p.hits {
		for _, stat := range hits {
			stats = append(stats, stat)
		}
	}

	// allowed criteria for sorting results
	allowedCriteria := map[string]lessFunc{}
	allowedCriteria["total_time_ns"] = func(stat1, stat2 *ExprStats) bool {
		return stat1.ExprTimeNs > stat2.ExprTimeNs
	}
	allowedCriteria["num_eval"] = func(stat1, stat2 *ExprStats) bool {
		return stat1.NumEval > stat2.NumEval
	}
	allowedCriteria["num_redo"] = func(stat1, stat2 *ExprStats) bool {
		return stat1.NumRedo > stat2.NumRedo
	}
	allowedCriteria["file"] = func(stat1, stat2 *ExprStats) bool {
		return stat1.Location.File > stat2.Location.File
	}
	allowedCriteria["line"] = func(stat1, stat2 *ExprStats) bool {
		return stat1.Location.Row > stat2.Location.Row
	}

	sortFuncs := []lessFunc{}

	for _, cr := range criteria {
		if fn, ok := allowedCriteria[cr]; ok {
			sortFuncs = append(sortFuncs, fn)
		}
	}

	// if no criteria return all the stats
	if len(sortFuncs) == 0 {
		return stats
	}

	orderedBy(sortFuncs).Sort(stats)

	// if desired number of results to be returned is less than or
	// equal to 0 or exceed total available results,
	// return all the stats
	if numResults <= 0 || numResults > len(stats) {
		return stats
	}
	return stats[:numResults]

}

// Trace updates the profiler state.
// Deprecated: Use TraceEvent instead.
func (p *Profiler) Trace(event *topdown.Event) {
	p.TraceEvent(*event)
}

// TraceEvent updates the coverage state.
func (p *Profiler) TraceEvent(event topdown.Event) {
	switch event.Op {
	case topdown.EvalOp:
		if expr, ok := event.Node.(*ast.Expr); ok && expr != nil {
			p.processExpr(expr, event.Op)
		}
	case topdown.RedoOp:
		if expr, ok := event.Node.(*ast.Expr); ok && expr != nil {
			p.processExpr(expr, event.Op)
		}
	}
}

func (p *Profiler) processExpr(expr *ast.Expr, eventType topdown.Op) {
	if expr.Location == nil {
		// add fake location to group expressions without a location
		expr.Location = ast.NewLocation([]byte("???"), "", 0, 0)
	}

	// set the active timer on the first expression
	if p.activeTimer.IsZero() {
		p.activeTimer = time.Now()
		p.prevExpr = exprInfo{
			op:       eventType,
			location: expr.Location,
		}
		return
	}

	// record the profiler results for the previous expression
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
	}
}

func (p *Profiler) processLastExpr() {
	expr := ast.Expr{
		Location: p.prevExpr.location,
	}
	p.processExpr(&expr, p.prevExpr.op)
}

func getProfilerStats(expr exprInfo, timer time.Time) ExprStats {
	profilerStats := ExprStats{}
	profilerStats.ExprTimeNs = time.Since(timer).Nanoseconds()
	profilerStats.Location = expr.location

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
	Location   *ast.Location `json:"location"`
}

// ExprStatsAggregated represents the result of profiling an expression
// by aggregating `n` profiles.
type ExprStatsAggregated struct {
	ExprTimeNsStats interface{}   `json:"total_time_ns_stats"`
	NumEval         int           `json:"num_eval"`
	NumRedo         int           `json:"num_redo"`
	Location        *ast.Location `json:"location"`
}

func aggregate(stats ...ExprStats) ExprStatsAggregated {
	if len(stats) == 0 {
		return ExprStatsAggregated{}
	}
	res := ExprStatsAggregated{
		NumEval:  stats[0].NumEval,
		NumRedo:  stats[0].NumRedo,
		Location: stats[0].Location,
	}
	timeNs := make([]int64, 0, len(stats))
	for _, s := range stats {
		timeNs = append(timeNs, s.ExprTimeNs)
	}
	res.ExprTimeNsStats = metrics.Statistics(timeNs...)
	return res
}

func AggregateProfiles(profiles ...[]ExprStats) []ExprStatsAggregated {
	if len(profiles) == 0 {
		return []ExprStatsAggregated{}
	}
	res := make([]ExprStatsAggregated, len(profiles[0]))
	for j := 0; j < len(profiles[0]); j++ {
		var s []ExprStats
		for _, p := range profiles {
			s = append(s, p[j])
		}
		res[j] = aggregate(s...)
	}
	return res
}

func sortStatsByRow(ps []ExprStats) {
	sort.Slice(ps, func(i, j int) bool {
		return ps[i].Location.Row < ps[j].Location.Row
	})
}

// Report represents the profiler report for a set of files.
type Report struct {
	Files map[string]*FileReport `json:"files"`
}

// FileReport represents a profiler report for a single file.
type FileReport struct {
	Result []ExprStats `json:"result"`
}

// Helper interfaces and methods for sorting a slice of ExprStats structs
// based on multiple fields.

type lessFunc func(p1, p2 *ExprStats) bool

// multiSorter implements the Sort interface, sorting the changes within.
type multiSorter struct {
	stats []ExprStats
	less  []lessFunc
}

// Sort sorts the argument slice according to the less functions passed to OrderedBy.
func (ms *multiSorter) Sort(stats []ExprStats) {
	ms.stats = stats
	sort.Sort(ms)
}

// orderedBy returns a Sorter that sorts using the less functions, in order.
func orderedBy(less []lessFunc) *multiSorter {
	return &multiSorter{
		less: less,
	}
}

// Len is part of sort.Interface.
func (ms *multiSorter) Len() int {
	return len(ms.stats)
}

// Swap is part of sort.Interface.
func (ms *multiSorter) Swap(i, j int) {
	ms.stats[i], ms.stats[j] = ms.stats[j], ms.stats[i]
}

// Less is part of sort.Interface. It is implemented by looping along the
// less functions until it finds a comparison that discriminates between
// the two items.
func (ms *multiSorter) Less(i, j int) bool {
	p, q := &ms.stats[i], &ms.stats[j]
	// Try all but the last comparison.
	var k int
	for k = 0; k < len(ms.less)-1; k++ {
		less := ms.less[k]
		switch {
		case less(p, q):
			return true
		case less(q, p):
			return false
		}
		// p == q; try the next comparison.
	}
	return ms.less[k](p, q)
}
