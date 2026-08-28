// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Command bench-nightly runs the nightly benchlab experiment and turns it into
// the data the benchmarks site charts.
//
// Each night a curated set of benchmarks (see set.json) is measured at three
// commits -- a fixed baseline, the previous night's commit, and HEAD -- with all
// three passed to a single benchlab invocation per package. benchlab repeats the
// whole commit list once per rep, so the arms are round-robin interleaved on one
// machine and drift is spread across them instead of biasing whichever ran
// first. That interleaving is the only thing that makes the arms comparable, so
// commits that need comparing must go into the same invocation.
//
// Deltas measured on different machines are not comparable, so a trend line
// cannot be built by chaining each night's "yesterday vs today" measurement:
// the errors compound and there is no way to re-anchor. Re-measuring a fixed
// baseline every night keeps "% vs baseline" comparable across nights even
// though the machines are not. Carrying the previous night's commit along costs
// one extra arm and buys two things: the "what landed today" delta, and a
// calibration signal -- (baseline, prev) is measured tonight and was measured
// last night on a different machine, and since the commits are identical any
// disagreement is pure measurement error.
//
// Run mode measures one shard and writes its results as JSON. Merge mode folds
// one night's shards into a single record, computes that night's calibration
// against the previous night, and appends it to benchlab.json.
package main

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
)

const opaModulePath = "github.com/open-policy-agent/opa"

// measureForUnit maps benchstat's units onto the measure names benchmarks.json
// already uses, so both data sources can be charted on the same axes.
var measureForUnit = map[string]string{
	"sec/op":    "NsPerOp",
	"ns/op":     "NsPerOp",
	"B/op":      "BytesPerOp",
	"allocs/op": "AllocsPerOp",
}

// target is one curated entry: a package and the benchmarks to run within it.
type target struct {
	Shard string `json:"shard"`
	Pkg   string `json:"pkg"`
	Bench string `json:"bench"`
}

type benchmarkSet struct {
	Targets []target `json:"targets"`
}

// delta is one pairwise comparison between two arms of the same experiment.
type delta struct {
	Pct         float64 `json:"pct"`
	P           float64 `json:"p"`
	N           int     `json:"n"`
	Significant bool    `json:"significant"`
}

// result is one benchmark/measure pair's comparisons for one night.
type result struct {
	Pkg     string `json:"pkg"`
	Name    string `json:"name"`
	Measure string `json:"measure"`

	BaselineValue float64 `json:"baseline_value"`
	HeadValue     float64 `json:"head_value"`
	BaselineCIPct float64 `json:"baseline_ci_pct"`
	HeadCIPct     float64 `json:"head_ci_pct"`

	// VsBaseline is the trend datapoint. VsPrev is the "what landed today"
	// signal. PrevVsBaseline is tonight's measurement of a pair that last
	// night also measured, and feeds the calibration summary.
	VsBaseline     *delta `json:"vs_baseline,omitempty"`
	VsPrev         *delta `json:"vs_prev,omitempty"`
	PrevVsBaseline *delta `json:"prev_vs_baseline,omitempty"`
}

// calibration summarises how far tonight's measurement of (baseline, prev)
// drifted from last night's measurement of the same two commits. The commits
// are identical, so this is measurement error and nothing else -- a night with
// large drift should not be read as a real change.
type calibration struct {
	ComparedTo   string  `json:"compared_to"`
	N            int     `json:"n"`
	MedianAbsPct float64 `json:"median_abs_drift_pct"`
	P90AbsPct    float64 `json:"p90_abs_drift_pct"`
	MaxAbsPct    float64 `json:"max_abs_drift_pct"`
}

// night is one nightly experiment, as stored in benchlab.json.
type night struct {
	Date        int64    `json:"date"`
	Head        string   `json:"head"`
	Prev        string   `json:"prev,omitempty"`
	BaselineSHA string   `json:"baseline_sha"`
	BaselineTag string   `json:"baseline_tag"`
	Shards      []string `json:"shards"`

	Reps       int     `json:"reps"`
	Count      int     `json:"count"`
	Benchtime  string  `json:"benchtime"`
	Alpha      float64 `json:"alpha"`
	Confidence float64 `json:"confidence"`

	Calibration *calibration `json:"calibration,omitempty"`
	Results     []result     `json:"results"`
}

func main() {
	log.SetPrefix("bench-nightly: ")
	log.SetFlags(0)

	var (
		mergeDir = flag.String("merge", "", "merge mode: fold the shard JSON files in this `dir` into -history")
		history  = flag.String("history", "", "path to benchlab.json; read for -prev in run mode, read and rewritten in merge mode")
		keep     = flag.Int("keep", 120, "merge mode: retain at most `N` nights of history")

		setPath  = flag.String("set", "build/bench-nightly/set.json", "path to the curated benchmark `set`")
		shard    = flag.String("shard", "", "run only the targets with this `shard` label")
		baseline = flag.String("baseline", "", "baseline `commit`; defaults to the newest v* tag")
		prev     = flag.String("prev", "", "previous night's `commit`; defaults to the newest night in -history")
		out      = flag.String("out", "", "write results to this `file` (default benchlab.<shard>.json)")

		reps       = flag.Int("reps", 15, "benchlab -reps")
		count      = flag.Int("count", 1, "benchlab -count")
		benchtime  = flag.String("benchtime", "2s", "benchlab -benchtime")
		tags       = flag.String("tags", "opa_wasm", "build `tags` for the benchlab host spec")
		alpha      = flag.Float64("alpha", 0.001, "benchstat -alpha")
		confidence = flag.Float64("confidence", 0.95, "benchstat -confidence")
	)
	flag.Parse()

	if *mergeDir != "" {
		if *history == "" {
			log.Fatal("-history is required with -merge")
		}
		runMerge(*mergeDir, *history, *keep)
		return
	}

	if *shard == "" {
		log.Fatal("-shard is required")
	}

	targets := loadTargets(*setPath, *shard)
	if len(targets) == 0 {
		log.Fatalf("no targets with shard %q in %s", *shard, *setPath)
	}

	head := gitOutput("rev-parse", "HEAD")
	baseSHA, baseTag := resolveBaseline(*baseline)
	prevSHA := resolvePrev(*prev, *history)

	if prevSHA == head {
		// Nothing landed since the last night. The prev arm would measure the
		// same commit twice under two labels: pure cost, no signal.
		log.Printf("prev (%s) is HEAD; dropping the prev arm", short(prevSHA))
		prevSHA = ""
	}

	n := night{
		Date:        time.Now().Unix(),
		Head:        head,
		Prev:        prevSHA,
		BaselineSHA: baseSHA,
		BaselineTag: baseTag,
		Shards:      []string{*shard},
		Reps:        *reps,
		Count:       *count,
		Benchtime:   *benchtime,
		Alpha:       *alpha,
		Confidence:  *confidence,
	}

	log.Printf("shard %s: baseline %s (%s), prev %s, head %s",
		*shard, short(baseSHA), baseTag, short(prevSHA), short(head))

	for _, t := range targets {
		rs, err := measure(t, n, *tags)
		if err != nil {
			log.Fatalf("%s: %v", t.Pkg, err)
		}
		if len(rs) == 0 {
			log.Printf("warning: %s matched no benchmarks for -bench %q", t.Pkg, t.Bench)
		}
		n.Results = append(n.Results, rs...)
	}

	path := *out
	if path == "" {
		path = "benchlab." + *shard + ".json"
	}
	writeJSON(path, n)
	log.Printf("wrote %s (%d results)", path, len(n.Results))
}

func loadTargets(path, shard string) []target {
	var set benchmarkSet
	readJSON(path, &set)
	var out []target
	for _, t := range set.Targets {
		if t.Shard == shard {
			out = append(out, t)
		}
	}
	return out
}

// resolveBaseline returns the baseline commit and the tag naming it. The tag
// matters because the notebook anchors its charts to a release tag too; if the
// two disagree the traces are in different units and are silently
// incomparable, so the tag is recorded for the notebook to check.
func resolveBaseline(want string) (sha, tag string) {
	if want == "" {
		tag = gitOutput("describe", "--tags", "--abbrev=0", "--match", "v*")
		return gitOutput("rev-parse", tag+"^{commit}"), tag
	}
	sha = gitOutput("rev-parse", want+"^{commit}")
	// Best-effort: an explicitly passed baseline may not be a tag at all.
	if out, err := exec.Command("git", "describe", "--tags", "--exact-match", sha).Output(); err == nil {
		tag = strings.TrimSpace(string(out))
	}
	return sha, tag
}

// resolvePrev returns the commit the previous night measured as its HEAD.
func resolvePrev(want, history string) string {
	if want != "" {
		return gitOutput("rev-parse", want+"^{commit}")
	}
	if history == "" {
		return ""
	}
	nights := readHistory(history)
	if len(nights) == 0 {
		return ""
	}
	return nights[len(nights)-1].Head
}

// measure runs one benchlab invocation covering every arm, then re-analyses its
// raw output with benchstat to extract the pairwise comparisons.
func measure(t target, n night, tags string) ([]result, error) {
	commits := []string{n.BaselineSHA}
	if n.Prev != "" {
		commits = append(commits, n.Prev)
	}
	commits = append(commits, n.Head)

	raw, err := runBenchlab(t, n, tags, commits)
	if err != nil {
		return nil, err
	}

	// One invocation covering (baseline, prev, head) already yields both
	// deltas against the baseline, because benchstat compares every column
	// with the first one. Only head-vs-prev needs a second pass, and it is
	// pure re-analysis of the same file rather than more measurement.
	vsBase, err := benchstatTables(raw, commits, n)
	if err != nil {
		return nil, err
	}
	var vsPrev map[string]*csvTable
	if n.Prev != "" {
		if vsPrev, err = benchstatTables(raw, []string{n.Prev, n.Head}, n); err != nil {
			return nil, err
		}
	}

	return assemble(t, n, vsBase, vsPrev)
}

// runBenchlab runs benchlab for one target and returns the raw output file it
// wrote. benchlab picks a fresh .benchlab/bench.<date>[.N].txt each invocation,
// so the new file is found by diffing the directory rather than guessed at.
//
// -run '^$' skips the test phase benchlab would otherwise use to avoid
// benchmarking broken code; the per-push job already establishes that HEAD's
// tests pass, and paying for them again at three commits is not worth the
// wall-clock. This mirrors what build/bench-comment does.
func runBenchlab(t target, n night, tags string, commits []string) (string, error) {
	before := benchFiles()

	args := []string{
		"-commit", strings.Join(commits, ","),
		"-pkg", t.Pkg,
		"-host", "local:tags=" + tags,
		"-reps", strconv.Itoa(n.Reps),
		"-count", strconv.Itoa(n.Count),
		"-benchtime", n.Benchtime,
		"-run", "^$",
	}
	if t.Bench != "" {
		args = append(args, "-bench", t.Bench)
	}

	cmd := exec.Command("benchlab", args...)
	cmd.Stdout = os.Stderr // benchlab's progress output is not our data
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("benchlab: %w", err)
	}

	added := []string{}
	for _, f := range benchFiles() {
		if !slices.Contains(before, f) {
			added = append(added, f)
		}
	}
	switch len(added) {
	case 1:
		return added[0], nil
	case 0:
		return "", fmt.Errorf("benchlab wrote no new raw output file")
	default:
		return "", fmt.Errorf("benchlab wrote %d new raw output files: %v", len(added), added)
	}
}

func benchFiles() []string {
	files, _ := filepath.Glob(".benchlab/bench.*.txt")
	slices.Sort(files)
	return files
}

// benchstatTables re-analyses a benchlab raw output file, splitting it into one
// column per commit. benchlab tags every section it writes with a "commit:"
// configuration key precisely so that this is possible.
func benchstatTables(raw string, commits []string, n night) (map[string]*csvTable, error) {
	quoted := make([]string, len(commits))
	for i, c := range commits {
		quoted[i] = strconv.Quote(c)
	}
	cmd := exec.Command("benchstat",
		"-format=csv",
		fmt.Sprintf("-alpha=%g", n.Alpha),
		fmt.Sprintf("-confidence=%g", n.Confidence),
		fmt.Sprintf("-col=commit@(%s)", strings.Join(quoted, " ")),
		raw,
	)
	cmd.Stderr = os.Stderr // benchstat reports warnings here
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("benchstat: %w", err)
	}
	return parseBenchstatCSV(string(out))
}

// csvColumn is one benchstat CSV column: a named group of samples, plus, for
// every column after the first, its comparison against the first column.
type csvColumn struct {
	name     string
	valueIdx int
	ciIdx    int
	deltaIdx int
	pIdx     int
}

// csvTable is one benchstat CSV table, holding every row for a single unit.
type csvTable struct {
	pkg     string
	unit    string
	columns []csvColumn
	rows    map[string][]string
}

func (t *csvTable) column(name string) (csvColumn, bool) {
	for _, c := range t.columns {
		if c.name == name {
			return c, true
		}
	}
	return csvColumn{}, false
}

func (t *csvTable) cell(row []string, idx int) string {
	if idx < 0 || idx >= len(row) {
		return ""
	}
	return row[idx]
}

// parseBenchstatCSV parses benchstat -format=csv into one table per unit, keyed
// by measure name. The layout is a preamble of "key: value" lines followed by
// blank-line separated tables, each opening with a row of column names and a row
// describing that column's sub-columns.
func parseBenchstatCSV(out string) (map[string]*csvTable, error) {
	lines := strings.Split(out, "\n")

	preamble := map[string]string{}
	i := 0
	for ; i < len(lines); i++ {
		line := strings.TrimRight(lines[i], "\r")
		if strings.HasPrefix(line, ",") {
			break
		}
		if key, value, ok := strings.Cut(line, ": "); ok {
			preamble[strings.TrimSpace(key)] = strings.TrimSpace(value)
		}
	}

	pkg := strings.Replace(preamble["pkg"], opaModulePath, ".", 1)

	tables := map[string]*csvTable{}
	for i < len(lines) {
		if strings.TrimSpace(lines[i]) == "" {
			i++
			continue
		}
		var block []string
		for i < len(lines) && strings.TrimSpace(lines[i]) != "" {
			block = append(block, lines[i])
			i++
		}
		table, err := parseTableBlock(block, pkg)
		if err != nil {
			return nil, err
		}
		if table == nil {
			continue
		}
		measure, ok := measureForUnit[table.unit]
		if !ok {
			log.Printf("warning: ignoring benchstat table with unrecognised unit %q", table.unit)
			continue
		}
		tables[measure] = table
	}
	return tables, nil
}

func parseTableBlock(block []string, pkg string) (*csvTable, error) {
	if len(block) < 3 {
		return nil, nil // a header with no data rows carries nothing
	}
	r := csv.NewReader(strings.NewReader(strings.Join(block, "\n")))
	r.FieldsPerRecord = -1
	records, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("parsing benchstat csv: %w", err)
	}
	if len(records) < 3 {
		return nil, nil
	}

	names, units := records[0], records[1]
	table := &csvTable{pkg: pkg, rows: map[string][]string{}}

	// Sub-columns run [unit, CI] for the first column and [unit, CI, vs base,
	// P] for the rest. Walking the header rather than assuming fixed offsets
	// keeps this working whether or not a comparison column is present.
	for j := 1; j < len(units); {
		if units[j] == "" {
			j++
			continue
		}
		col := csvColumn{valueIdx: j, ciIdx: -1, deltaIdx: -1, pIdx: -1}
		if j < len(names) {
			col.name = names[j]
		}
		if table.unit == "" {
			table.unit = units[j]
		}
		j++
		if j < len(units) && units[j] == "CI" {
			col.ciIdx = j
			j++
		}
		if j < len(units) && units[j] == "vs base" {
			col.deltaIdx = j
			j++
		}
		if j < len(units) && units[j] == "P" {
			col.pIdx = j
			j++
		}
		table.columns = append(table.columns, col)
	}

	for _, row := range records[2:] {
		if len(row) == 0 || row[0] == "" || row[0] == "geomean" {
			continue
		}
		table.rows[row[0]] = row
	}
	return table, nil
}

// assemble turns the parsed benchstat tables into results.
func assemble(t target, n night, vsBase, vsPrev map[string]*csvTable) ([]result, error) {
	var out []result

	measures := make([]string, 0, len(vsBase))
	for m := range vsBase {
		measures = append(measures, m)
	}
	sort.Strings(measures)

	for _, measure := range measures {
		table := vsBase[measure]

		baseCol, ok := table.column(n.BaselineSHA)
		if !ok {
			return nil, fmt.Errorf("%s: no benchstat column for baseline %s", measure, short(n.BaselineSHA))
		}
		headCol, ok := table.column(n.Head)
		if !ok {
			return nil, fmt.Errorf("%s: no benchstat column for head %s", measure, short(n.Head))
		}
		prevCol, hasPrev := table.column(n.Prev)

		names := make([]string, 0, len(table.rows))
		for name := range table.rows {
			names = append(names, name)
		}
		sort.Strings(names)

		for _, name := range names {
			row := table.rows[name]

			baseVal, okB := parseFloat(table.cell(row, baseCol.valueIdx))
			headVal, okH := parseFloat(table.cell(row, headCol.valueIdx))
			if !okB || !okH {
				continue
			}
			baseVal = round4(toMeasureUnits(table.unit, baseVal))
			headVal = round4(toMeasureUnits(table.unit, headVal))

			r := result{
				// benchstat strips the "Benchmark" prefix from names in its
				// tables, but the published chart pages are keyed by the full
				// Go benchmark name, so it has to go back on.
				Pkg:           t.Pkg,
				Name:          "Benchmark" + name,
				Measure:       measure,
				BaselineValue: baseVal,
				HeadValue:     headVal,
				BaselineCIPct: parsePct(table.cell(row, baseCol.ciIdx)),
				HeadCIPct:     parsePct(table.cell(row, headCol.ciIdx)),
				VsBaseline: newDelta(baseVal, headVal,
					table.cell(row, headCol.deltaIdx), table.cell(row, headCol.pIdx)),
			}

			if hasPrev {
				if prevVal, ok := parseFloat(table.cell(row, prevCol.valueIdx)); ok {
					r.PrevVsBaseline = newDelta(baseVal, toMeasureUnits(table.unit, prevVal),
						table.cell(row, prevCol.deltaIdx), table.cell(row, prevCol.pIdx))
				}
			}

			if pt := vsPrev[measure]; pt != nil {
				r.VsPrev = deltaFromTable(pt, n.Prev, n.Head, name)
			}

			out = append(out, r)
		}
	}
	return out, nil
}

func deltaFromTable(t *csvTable, from, to, name string) *delta {
	row, ok := t.rows[name]
	if !ok {
		return nil
	}
	fromCol, ok1 := t.column(from)
	toCol, ok2 := t.column(to)
	if !ok1 || !ok2 {
		return nil
	}
	fromVal, okF := parseFloat(t.cell(row, fromCol.valueIdx))
	toVal, okT := parseFloat(t.cell(row, toCol.valueIdx))
	if !okF || !okT {
		return nil
	}
	return newDelta(toMeasureUnits(t.unit, fromVal), toMeasureUnits(t.unit, toVal),
		t.cell(row, toCol.deltaIdx), t.cell(row, toCol.pIdx))
}

// newDelta builds a comparison from two group medians plus benchstat's verdict.
//
// The point estimate is computed from the medians rather than read from
// benchstat's "vs base" cell, because benchstat prints "~" there whenever a
// difference is not distinguishable from noise. On a quiet night that is most
// benchmarks, and taking the cell at face value would punch holes in the trend
// line exactly where it is flat. The cell is still used for the significance
// verdict, which is the part benchstat is authoritative about.
func newDelta(from, to float64, deltaCell, pCell string) *delta {
	d := &delta{
		Significant: strings.HasPrefix(deltaCell, "+") || strings.HasPrefix(deltaCell, "-"),
		P:           math.NaN(),
	}
	if from != 0 {
		d.Pct = round4((to/from - 1) * 100)
	}
	for _, field := range strings.Fields(pCell) {
		if v, ok := strings.CutPrefix(field, "p="); ok {
			if f, err := strconv.ParseFloat(v, 64); err == nil {
				d.P = round4(f)
			}
		}
		if v, ok := strings.CutPrefix(field, "n="); ok {
			// Unequal sample counts are reported as "n=15+14".
			if f, err := strconv.Atoi(strings.SplitN(v, "+", 2)[0]); err == nil {
				d.N = f
			}
		}
	}
	if math.IsNaN(d.P) {
		d.P = 0
	}
	return d
}

// toMeasureUnits converts benchstat's seconds to the nanoseconds
// benchmarks.json records, leaving byte and allocation counts alone.
func toMeasureUnits(unit string, v float64) float64 {
	if unit == "sec/op" {
		return v * 1e9
	}
	return v
}

// round4 trims float noise. A median ratio over fifteen samples does not carry
// seventeen significant digits, and every excess character is paid for ~900
// times a night in a file the notebook reads whole.
func round4(v float64) float64 {
	return math.Round(v*1e4) / 1e4
}

func parseFloat(s string) (float64, bool) {
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return v, err == nil
}

// parsePct reads benchstat's CI cell, which is a percentage such as "2%".
func parsePct(s string) float64 {
	v, _ := parseFloat(strings.TrimSuffix(strings.TrimSpace(s), "%"))
	return v
}

// runMerge folds one night's shards into a single record and appends it to the
// history, computing the night's calibration while last night's numbers are at
// hand.
func runMerge(dir, historyPath string, keep int) {
	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		log.Fatal(err)
	}
	slices.Sort(files)
	if len(files) == 0 {
		log.Fatalf("no shard JSON files in %s", dir)
	}

	var merged night
	for i, f := range files {
		var shard night
		readJSON(f, &shard)
		if i == 0 {
			merged = shard
			merged.Results = slices.Clone(shard.Results)
			continue
		}
		// Shards of one night must describe the same experiment, or the
		// results are not commensurable and the record would be a fiction.
		if shard.Head != merged.Head || shard.BaselineSHA != merged.BaselineSHA || shard.Prev != merged.Prev {
			log.Fatalf("%s disagrees with %s about the experiment (head/baseline/prev)", f, files[0])
		}
		merged.Shards = append(merged.Shards, shard.Shards...)
		merged.Results = append(merged.Results, shard.Results...)
	}
	slices.Sort(merged.Shards)

	nights := readHistory(historyPath)
	if len(nights) > 0 {
		merged.Calibration = calibrate(nights[len(nights)-1], merged)
	}

	nights = append(nights, merged)
	if keep > 0 && len(nights) > keep {
		nights = nights[len(nights)-keep:]
	}
	compactHistory(nights)
	writeCompactJSON(historyPath, nights)

	log.Printf("merged %d shard(s), %d results; history now %d night(s)",
		len(files), len(merged.Results), len(nights))
	if c := merged.Calibration; c != nil {
		log.Printf("calibration vs %s: median %.2f%%, p90 %.2f%%, max %.2f%% over %d benchmarks",
			short(c.ComparedTo), c.MedianAbsPct, c.P90AbsPct, c.MaxAbsPct, c.N)
	}
}

// compactHistory drops from every night but the newest the two comparisons only
// the newest one needs.
//
// vs_prev is the "what landed today" signal, which is only actionable for the
// most recent night. prev_vs_baseline exists solely to be compared against the
// previous night's vs_baseline, and that has already happened by the time a
// night is written -- the outcome is its calibration summary. Retaining either
// on older nights roughly triples the size of a file the notebook slurps whole
// on every render.
func compactHistory(nights []night) {
	if len(nights) < 2 {
		return
	}
	for i := range nights[:len(nights)-1] {
		for j := range nights[i].Results {
			nights[i].Results[j].VsPrev = nil
			nights[i].Results[j].PrevVsBaseline = nil
		}
	}
}

// calibrate compares tonight's measurement of (baseline, prev) with last
// night's measurement of (baseline, its own head) -- the same two commits, a
// different machine. Any disagreement is measurement error.
func calibrate(last, tonight night) *calibration {
	if tonight.Prev == "" || last.Head != tonight.Prev || last.BaselineSHA != tonight.BaselineSHA {
		return nil
	}

	type key struct{ pkg, name, measure string }
	lastByKey := map[key]*delta{}
	for _, r := range last.Results {
		if r.VsBaseline != nil {
			lastByKey[key{r.Pkg, r.Name, r.Measure}] = r.VsBaseline
		}
	}

	var drifts []float64
	for _, r := range tonight.Results {
		if r.PrevVsBaseline == nil {
			continue
		}
		if prior, ok := lastByKey[key{r.Pkg, r.Name, r.Measure}]; ok {
			drifts = append(drifts, math.Abs(r.PrevVsBaseline.Pct-prior.Pct))
		}
	}
	if len(drifts) == 0 {
		return nil
	}
	sort.Float64s(drifts)

	return &calibration{
		ComparedTo:   last.Head,
		N:            len(drifts),
		MedianAbsPct: quantile(drifts, 0.5),
		P90AbsPct:    quantile(drifts, 0.9),
		MaxAbsPct:    drifts[len(drifts)-1],
	}
}

// quantile returns the q-th quantile of a sorted slice by nearest rank.
func quantile(sorted []float64, q float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	i := int(math.Round(q * float64(len(sorted)-1)))
	return sorted[max(0, min(i, len(sorted)-1))]
}

func readHistory(path string) []night {
	var nights []night
	if _, err := os.Stat(path); err != nil {
		return nil // first run: no history yet
	}
	readJSON(path, &nights)
	return nights
}

func readJSON(path string, v any) {
	b, err := os.ReadFile(path)
	if err != nil {
		log.Fatal(err)
	}
	if err := json.Unmarshal(b, v); err != nil {
		log.Fatalf("parsing %s: %v", path, err)
	}
}

// writeJSON writes v indented, for the per-shard files a human may read.
func writeJSON(path string, v any) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	write(path, b)
}

// writeCompactJSON writes v without indentation. benchlab.json accumulates
// roughly nine hundred results a night and the notebook reads it whole, so the
// whitespace is not free.
func writeCompactJSON(path string, v any) {
	b, err := json.Marshal(v)
	if err != nil {
		log.Fatal(err)
	}
	write(path, b)
}

func write(path string, b []byte) {
	if err := os.WriteFile(path, append(b, '\n'), 0o644); err != nil {
		log.Fatal(err)
	}
}

func gitOutput(args ...string) string {
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		log.Fatalf("git %s: %v", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(out))
}

func short(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}
