package util

import (
	"slices"
	"testing"
)

func TestValues(t *testing.T) {
	testMap := map[string]int{"a": 1, "b": 2, "c": 3}
	values := Values(testMap)
	if len(values) != 3 {
		t.Errorf("Expected 3 values, got %d", len(values))
	}
	slices.Sort(values)
	if values[0] != 1 || values[1] != 2 || values[2] != 3 {
		t.Errorf("Expected [1, 2, 3], got %v", values)
	}
}
