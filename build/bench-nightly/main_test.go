// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package main

import (
	"math"
	"os"
	"testing"
)

// The testdata CSV files are real `benchstat -format=csv` output, produced from
// a synthetic benchlab raw file in which prev was built to be 2% slower than
// the baseline and head 8% slower, with allocations going 12 -> 12 -> 14 and
// bytes held constant. Those known quantities are what the assertions below
// check, so a change in benchstat's output shape fails here rather than
// silently producing wrong charts.
const (
	baseSHA = "basesha"
	prevSHA = "prevsha"
	headSHA = "headsha"
)

func loadTable(t *testing.T, path string) map[string]*csvTable {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	tables, err := parseBenchstatCSV(string(b))
	if err != nil {
		t.Fatal(err)
	}
	return tables
}

func TestParseBenchstatCSV(t *testing.T) {
	tables := loadTable(t, "testdata/vsbase.csv")

	for _, measure := range []string{"NsPerOp", "BytesPerOp", "AllocsPerOp"} {
		if _, ok := tables[measure]; !ok {
			t.Errorf("no table for measure %q", measure)
		}
	}
	if len(tables) != 3 {
		t.Errorf("got %d tables, want 3", len(tables))
	}

	table := tables["NsPerOp"]
	if table.pkg != "./v1/topdown" {
		t.Errorf("pkg = %q, want ./v1/topdown", table.pkg)
	}
	if len(table.columns) != 3 {
		t.Fatalf("got %d columns, want 3", len(table.columns))
	}
	for _, sha := range []string{baseSHA, prevSHA, headSHA} {
		if _, ok := table.column(sha); !ok {
			t.Errorf("no column for %q", sha)
		}
	}

	// The first column carries no comparison; later ones do.
	if c, _ := table.column(baseSHA); c.deltaIdx != -1 {
		t.Errorf("baseline column has a delta index %d, want -1", c.deltaIdx)
	}
	if c, _ := table.column(headSHA); c.deltaIdx == -1 || c.pIdx == -1 {
		t.Errorf("head column missing delta/P indices: %+v", c)
	}

	// geomean is a summary row, not a benchmark.
	if len(table.rows) != 2 {
		t.Errorf("got %d rows, want 2: %v", len(table.rows), table.rows)
	}
	if _, ok := table.rows["geomean"]; ok {
		t.Error("geomean row was not skipped")
	}
}

func TestAssemble(t *testing.T) {
	n := night{BaselineSHA: baseSHA, Prev: prevSHA, Head: headSHA}
	results, err := assemble(
		target{Pkg: "./v1/topdown"}, n,
		loadTable(t, "testdata/vsbase.csv"),
		loadTable(t, "testdata/vsprev.csv"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 6 { // 2 benchmarks x 3 measures
		t.Fatalf("got %d results, want 6", len(results))
	}

	find := func(name, measure string) result {
		t.Helper()
		for _, r := range results {
			if r.Name == name && r.Measure == measure {
				return r
			}
		}
		t.Fatalf("no result for %s/%s", name, measure)
		return result{}
	}

	t.Run("time", func(t *testing.T) {
		r := find("BenchmarkGlob/100-4", "NsPerOp")

		// benchstat reports seconds; benchmarks.json records nanoseconds.
		approx(t, "baseline value", r.BaselineValue, 1002, 0.5)
		approx(t, "head value", r.HeadValue, 1080.7, 0.5)

		approx(t, "vs baseline", r.VsBaseline.Pct, 7.85, 0.01)
		approx(t, "prev vs baseline", r.PrevVsBaseline.Pct, 2.13, 0.01)
		approx(t, "vs prev", r.VsPrev.Pct, 5.61, 0.01)

		for label, d := range map[string]*delta{
			"vs baseline": r.VsBaseline, "prev vs baseline": r.PrevVsBaseline, "vs prev": r.VsPrev,
		} {
			if !d.Significant {
				t.Errorf("%s: not significant, want significant", label)
			}
			if d.N != 15 {
				t.Errorf("%s: n = %d, want 15", label, d.N)
			}
		}
	})

	t.Run("allocations", func(t *testing.T) {
		r := find("BenchmarkGlob/100-4", "AllocsPerOp")
		approx(t, "baseline value", r.BaselineValue, 12, 0.001)
		approx(t, "head value", r.HeadValue, 14, 0.001)
		approx(t, "vs baseline", r.VsBaseline.Pct, 16.67, 0.01)
		// prev and baseline both allocate 12, so benchstat prints "~".
		if r.PrevVsBaseline.Significant {
			t.Error("prev vs baseline: significant, want not significant")
		}
		approx(t, "prev vs baseline", r.PrevVsBaseline.Pct, 0, 0.001)
	})

	t.Run("bytes are unchanged and not significant", func(t *testing.T) {
		r := find("BenchmarkGlob/100-4", "BytesPerOp")
		approx(t, "head value", r.HeadValue, 512, 0.001)
		if r.VsBaseline.Significant {
			t.Error("vs baseline: significant, want not significant")
		}
	})
}

// A "~" verdict means "not distinguishable from noise", not "no measurement".
// Dropping the point estimate along with the verdict would leave holes in the
// trend line on exactly the nights where nothing changed.
func TestInsignificantDeltaKeepsPointEstimate(t *testing.T) {
	d := newDelta(100, 103, "~", "p=0.400 n=15")
	if d.Significant {
		t.Error("Significant = true, want false")
	}
	approx(t, "pct", d.Pct, 3, 0.001)
	approx(t, "p", d.P, 0.4, 0.001)
	if d.N != 15 {
		t.Errorf("n = %d, want 15", d.N)
	}
}

func TestNewDeltaUnequalSampleCounts(t *testing.T) {
	if d := newDelta(1, 1, "~", "p=0.500 n=15+14"); d.N != 15 {
		t.Errorf("n = %d, want 15", d.N)
	}
}

func TestCalibrate(t *testing.T) {
	key := func(pct float64) result {
		return result{
			Pkg: "./v1/topdown", Name: "BenchmarkGlob-4", Measure: "NsPerOp",
			VsBaseline:     &delta{Pct: pct},
			PrevVsBaseline: &delta{Pct: pct},
		}
	}

	last := night{Head: prevSHA, BaselineSHA: baseSHA, Results: []result{key(2.0)}}
	// Tonight re-measures (baseline, prev) and gets 2.5% where last night got
	// 2.0%: the commits are identical, so the 0.5 point difference is noise.
	tonight := night{Head: headSHA, Prev: prevSHA, BaselineSHA: baseSHA, Results: []result{key(2.5)}}

	c := calibrate(last, tonight)
	if c == nil {
		t.Fatal("calibrate returned nil")
	}
	if c.ComparedTo != prevSHA {
		t.Errorf("ComparedTo = %q, want %q", c.ComparedTo, prevSHA)
	}
	if c.N != 1 {
		t.Errorf("N = %d, want 1", c.N)
	}
	approx(t, "median drift", c.MedianAbsPct, 0.5, 0.001)

	t.Run("refuses to compare unrelated nights", func(t *testing.T) {
		// Last night's head is not what tonight measured as prev, so the two
		// numbers are not measurements of the same pair of commits.
		mismatched := night{Head: "other", BaselineSHA: baseSHA, Results: []result{key(2.0)}}
		if c := calibrate(mismatched, tonight); c != nil {
			t.Errorf("got %+v, want nil", c)
		}
	})

	t.Run("refuses when the baseline was re-anchored", func(t *testing.T) {
		rebased := night{Head: prevSHA, BaselineSHA: "newbase", Results: []result{key(2.0)}}
		if c := calibrate(rebased, tonight); c != nil {
			t.Errorf("got %+v, want nil", c)
		}
	})
}

// The workflow derives its shard matrix from set.json with jq while the tool
// reads the same file with -shard, so the two have to agree about its shape.
func TestLoadTargetsFromSet(t *testing.T) {
	for _, shard := range []string{"topdown", "ast", "rego", "inmem"} {
		targets := loadTargets("set.json", shard)
		if len(targets) == 0 {
			t.Errorf("shard %q resolved no targets", shard)
			continue
		}
		for _, target := range targets {
			if target.Pkg == "" || target.Bench == "" {
				t.Errorf("shard %q has an incomplete target: %+v", shard, target)
			}
		}
	}
	if targets := loadTargets("set.json", "nonexistent"); len(targets) != 0 {
		t.Errorf("unknown shard resolved %d targets, want 0", len(targets))
	}
}

// Only the newest night keeps vs_prev/prev_vs_baseline; older nights have had
// everything they contribute (alerting, calibration) already extracted.
func TestCompactHistory(t *testing.T) {
	mk := func(head string) night {
		return night{Head: head, Results: []result{{
			Pkg: "./v1/topdown", Name: "BenchmarkGlob-4", Measure: "NsPerOp",
			VsBaseline:     &delta{Pct: 1},
			VsPrev:         &delta{Pct: 2},
			PrevVsBaseline: &delta{Pct: 3},
		}}}
	}
	nights := []night{mk("h1"), mk("h2"), mk("h3")}
	compactHistory(nights)

	for i, n := range nights[:2] {
		r := n.Results[0]
		if r.VsPrev != nil || r.PrevVsBaseline != nil {
			t.Errorf("night %d (%s) kept a transient comparison: %+v", i, n.Head, r)
		}
		// The trend datapoint and the next night's calibration input must survive.
		if r.VsBaseline == nil {
			t.Errorf("night %d (%s) lost vs_baseline", i, n.Head)
		}
	}
	newest := nights[2].Results[0]
	if newest.VsPrev == nil || newest.PrevVsBaseline == nil {
		t.Errorf("newest night was stripped: %+v", newest)
	}

	t.Run("a single night is left alone", func(t *testing.T) {
		one := []night{mk("h1")}
		compactHistory(one)
		if one[0].Results[0].VsPrev == nil {
			t.Error("the only night was stripped")
		}
	})

	t.Run("is idempotent", func(t *testing.T) {
		again := []night{mk("h1"), mk("h2")}
		compactHistory(again)
		compactHistory(again)
		if again[1].Results[0].VsPrev == nil {
			t.Error("second pass stripped the newest night")
		}
	})
}

func TestRound4(t *testing.T) {
	approx(t, "trims float noise", round4(16.666666666666675), 16.6667, 1e-9)
	approx(t, "leaves short values alone", round4(0.5), 0.5, 1e-9)
}

func TestQuantile(t *testing.T) {
	sorted := []float64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	approx(t, "median", quantile(sorted, 0.5), 5, 0.001)
	approx(t, "p90", quantile(sorted, 0.9), 8, 0.001)
	if got := quantile(nil, 0.5); got != 0 {
		t.Errorf("quantile(nil) = %v, want 0", got)
	}
}

func approx(t *testing.T, label string, got, want, tolerance float64) {
	t.Helper()
	if math.Abs(got-want) > tolerance {
		t.Errorf("%s = %v, want %v (+/- %v)", label, got, want, tolerance)
	}
}
