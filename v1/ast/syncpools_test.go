// Copyright 2024 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"testing"
)

// TestMapStringAnySlicePool verifies that maps are properly cleared when returned to the pool.
func TestMapStringAnySlicePool(t *testing.T) {
	// Get a slice of 3 maps
	s1 := mapStringAnySlicePool.Get(3)

	// Populate the maps with data
	(*s1)[0]["key1"] = "value1"
	(*s1)[0]["key2"] = "value2"
	(*s1)[1]["foo"] = "bar"
	(*s1)[2]["test"] = 123

	// Return to pool
	mapStringAnySlicePool.Put(s1)

	// Get again - should be the same underlying slice but with cleared maps
	s2 := mapStringAnySlicePool.Get(3)

	// Verify all maps are empty
	for i, m := range *s2 {
		if len(m) != 0 {
			t.Errorf("Map at index %d should be empty but has %d elements", i, len(m))
		}
	}

	// Verify maps are not nil
	for i, m := range *s2 {
		if m == nil {
			t.Errorf("Map at index %d should not be nil", i)
		}
	}

	// Return to pool
	mapStringAnySlicePool.Put(s2)
}

// TestMapStringAnySlicePoolGrowth verifies that the pool handles growth correctly.
func TestMapStringAnySlicePoolGrowth(t *testing.T) {
	// Get a small slice
	s1 := mapStringAnySlicePool.Get(2)
	mapStringAnySlicePool.Put(s1)

	// Get a larger slice - should allocate new if capacity is insufficient
	s2 := mapStringAnySlicePool.Get(10)

	if len(*s2) != 10 {
		t.Errorf("Expected slice length 10, got %d", len(*s2))
	}

	// All maps should be initialized and empty
	for i, m := range *s2 {
		if m == nil {
			t.Errorf("Map at index %d should not be nil", i)
		}
		if len(m) != 0 {
			t.Errorf("Map at index %d should be empty", i)
		}
	}

	mapStringAnySlicePool.Put(s2)
}
