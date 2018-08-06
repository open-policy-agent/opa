// Copyright 2018 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package benchstat

import (
	"io/ioutil"
	"log"
	"sort"
	"testing"
)

var file1 = "../cmd/benchstat/testdata/old.txt"
var file2 = "../cmd/benchstat/testdata/new.txt"

func extractRowBenchmark(row *Row) string {
	return row.Benchmark
}
func extractRowDelta(row *Row) float64 {
	return row.PctDelta
}
func extractRowChange(row *Row) int {
	return row.Change
}

func benchmarkSortTest(t *testing.T, sampleTable *Table) {
	numRows := len(sampleTable.Rows)
	benchmarks := make([]string, numRows)
	Sort(sampleTable, ByName)
	for idx, row := range sampleTable.Rows {
		benchmarks[idx] = extractRowBenchmark(row)
	}
	t.Run("BenchSorted", func(t *testing.T) {
		if !sort.StringsAreSorted(benchmarks) {
			t.Error("Table not sorted by names")
		}
	})
	Sort(sampleTable, Reverse(ByName))
	for idx, row := range sampleTable.Rows {
		benchmarks[numRows-idx-1] = extractRowBenchmark(row)
	}
	t.Run("BenchReversed", func(t *testing.T) {
		if !sort.StringsAreSorted(benchmarks) {
			t.Error("Table not reverse sorted by benchmarks")
		}
	})
}

func deltaSortTest(t *testing.T, sampleTable *Table) {
	numRows := len(sampleTable.Rows)
	deltas := make([]float64, numRows)
	Sort(sampleTable, ByDelta)
	for idx, row := range sampleTable.Rows {
		deltas[idx] = -extractRowDelta(row)
	}
	t.Run("DeltaSorted", func(t *testing.T) {
		if !sort.Float64sAreSorted(deltas) {
			t.Errorf("Table not sorted by deltas: %v", deltas)
		}
	})
	Sort(sampleTable, Reverse(ByDelta))
	for idx, row := range sampleTable.Rows {
		deltas[idx] = extractRowDelta(row)
	}
	t.Run("DeltaReversed", func(t *testing.T) {
		if !sort.Float64sAreSorted(deltas) {
			t.Error("Table not reverse sorted by deltas")
		}
	})
}

func TestCompareCollection(t *testing.T) {
	sampleCompareCollection := Collection{Alpha: 0.05, AddGeoMean: false, DeltaTest: UTest}
	file1Data, err := ioutil.ReadFile(file1)
	if err != nil {
		log.Fatal(err)
	}
	file2Data, err := ioutil.ReadFile(file2)
	if err != nil {
		log.Fatal(err)
	}
	sampleCompareCollection.AddConfig(file1, file1Data)
	sampleCompareCollection.AddConfig(file2, file2Data)
	// data has both time and speed tables, test only the speed table
	sampleTable := sampleCompareCollection.Tables()[0]
	t.Run("BenchmarkSort", func(t *testing.T) {
		benchmarkSortTest(t, sampleTable)
	})
	t.Run("DeltaSort", func(t *testing.T) {
		deltaSortTest(t, sampleTable)
	})
}

func TestSingleCollection(t *testing.T) {
	sampleCollection := Collection{Alpha: 0.05, AddGeoMean: false, DeltaTest: UTest}
	file1Data, err1 := ioutil.ReadFile(file1)
	if err1 != nil {
		log.Fatal(err1)
	}
	sampleCollection.AddConfig(file1, file1Data)
	sampleTable := sampleCollection.Tables()[0]
	t.Run("BenchmarkSort", func(t *testing.T) {
		benchmarkSortTest(t, sampleTable)
	})
}
