// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cover

// FileReport represents a coverage report for a single file.
type FileReport struct {
	Covered         []Range `json:"covered,omitempty"`
	NotCovered      []Range `json:"not_covered,omitempty"`
	CoveredLines    int     `json:"covered_lines,omitempty"`
	NotCoveredLines int     `json:"not_covered_lines,omitempty"`
	Coverage        float64 `json:"coverage,omitempty"`
}

// IsCovered returns true if the row is marked as covered in the report.
func (fr *FileReport) IsCovered(row int) bool {
	if fr == nil {
		return false
	}
	for _, r := range fr.Covered {
		if r.In(row) {
			return true
		}
	}
	return false
}

// IsNotCovered returns true if the row is marked as NOT covered in the report.
// This is not the same as simply not being reported. For example, certain
// statements like imports are not included in the report.
func (fr *FileReport) IsNotCovered(row int) bool {
	if fr == nil {
		return false
	}
	for _, r := range fr.NotCovered {
		if r.In(row) {
			return true
		}
	}
	return false
}

// isRangeCovered returns true if r is contained within any covered range.
func (fr *FileReport) isRangeCovered(r Range) bool {
	if fr == nil {
		return false
	}
	return rangeContainedIn(r, fr.Covered)
}

// isRangeNotCovered returns true if r is contained within any not-covered range.
func (fr *FileReport) isRangeNotCovered(r Range) bool {
	if fr == nil {
		return false
	}
	return rangeContainedIn(r, fr.NotCovered)
}

func rangeContainedIn(r Range, ranges []Range) bool {
	for _, candidate := range ranges {
		if candidate.contains(r) {
			return true
		}
	}
	return false
}

// locCovered returns the number of unique rows of code covered by tests
func (fr *FileReport) locCovered() int {
	return uniqueRowCount(fr.Covered)
}

// locNotCovered returns the number of unique rows of code not covered by tests
func (fr *FileReport) locNotCovered() int {
	return uniqueRowCount(fr.NotCovered)
}

// computeCoveragePercentage returns the code coverage percentage of the file
func (fr *FileReport) computeCoveragePercentage() float64 {
	coveredLoc := fr.locCovered()
	notCoveredLoc := fr.locNotCovered()
	totalLoc := coveredLoc + notCoveredLoc

	if totalLoc == 0 {
		return 0.0
	}

	return 100.0 * float64(coveredLoc) / float64(totalLoc)
}
