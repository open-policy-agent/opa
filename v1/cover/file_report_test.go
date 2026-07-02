package cover

import "testing"

func TestFileReportIsRangeCovered(t *testing.T) {
	t.Parallel()

	fr := &FileReport{
		Covered: []Range{
			{Start: Position{Row: 3, Col: 8}, End: Position{Row: 3, Col: 13}},
		},
		NotCovered: []Range{
			{Start: Position{Row: 3, Col: 1}, End: Position{Row: 3, Col: 4}},
		},
	}

	covered := Range{Start: Position{Row: 3, Col: 8}, End: Position{Row: 3, Col: 13}}
	if !fr.isRangeCovered(covered) {
		t.Errorf("expected %v to be covered", covered)
	}

	subCovered := Range{Start: Position{Row: 3, Col: 9}, End: Position{Row: 3, Col: 11}}
	if !fr.isRangeCovered(subCovered) {
		t.Errorf("expected sub-range %v to be covered", subCovered)
	}

	notCovered := Range{Start: Position{Row: 3, Col: 1}, End: Position{Row: 3, Col: 4}}
	if !fr.isRangeNotCovered(notCovered) {
		t.Errorf("expected %v to be not covered", notCovered)
	}

	absent := Range{Start: Position{Row: 5, Col: 1}, End: Position{Row: 5, Col: 4}}
	if fr.isRangeCovered(absent) {
		t.Errorf("expected %v to not be covered", absent)
	}
	if fr.isRangeNotCovered(absent) {
		t.Errorf("expected %v to not be in not_covered", absent)
	}
}
