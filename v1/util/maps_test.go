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

func TestKeysRecursive(t *testing.T) {
	testMap := map[string]any{
		"a": 1,
		"b": map[string]any{
			"c": 2,
			"d": map[string]any{
				"e": 3,
			},
		},
	}
	keys := KeysRecursive(testMap, make(map[string]struct{}))
	keysSlice := KeysSorted(keys)
	if !slices.Equal(keysSlice, []string{"a", "b", "c", "d", "e"}) {
		t.Errorf("Expected [a b c d e], got %v", keysSlice)
	}
}
