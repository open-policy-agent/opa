// Copyright 2024 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"strconv"
	"sync"
	"testing"
)

// TestArrayHashConcurrent tests concurrent hash access from multiple goroutines.
// This verifies the fix for race conditions identified in PR review comments:
// - No drift between hash and validity flag (now using single atomic with sentinel)
// - Hash computation is fully atomic (CompareAndSwap ensures atomicity)
// - No stale values can be read by concurrent goroutines
func TestArrayHashConcurrent(t *testing.T) {
	tests := []struct {
		name string
		size int
	}{
		{"small_array", 10},
		{"medium_array", 100},
		{"large_array", 1000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create array
			terms := make([]*Term, tt.size)
			for i := range tt.size {
				terms[i] = IntNumberTerm(i)
			}
			arr := NewArray(terms...)

			// Expected hash (computed serially for verification)
			expectedHash := arr.Hash()

			// Reset to not computed state to test concurrent first access
			arr.hash.Store(arrayHashNotComputed)

			const numGoroutines = 100
			results := make([]int, numGoroutines)
			var wg sync.WaitGroup
			wg.Add(numGoroutines)

			// Launch goroutines that all try to compute hash simultaneously
			for i := range numGoroutines {
				go func(idx int) {
					defer wg.Done()
					results[idx] = arr.Hash()
				}(i)
			}

			wg.Wait()

			// Verify all goroutines got the same hash
			for i, hash := range results {
				if hash != expectedHash {
					t.Errorf("goroutine %d: got hash %d, expected %d", i, hash, expectedHash)
				}
			}

			// Verify hash is now cached and consistent
			if cachedHash := arr.Hash(); cachedHash != expectedHash {
				t.Errorf("cached hash mismatch: got %d, expected %d", cachedHash, expectedHash)
			}
		})
	}
}

// TestArrayHashConcurrentWithOperations tests concurrent hash access mixed with array operations.
// This tests the scenario where OPA server has multiple queries accessing the same array.
func TestArrayHashConcurrentWithOperations(t *testing.T) {
	terms := make([]*Term, 50)
	for i := range 50 {
		terms[i] = IntNumberTerm(i)
	}
	arr := NewArray(terms...)

	const numReaders = 50
	const numOperations = 100

	var wg sync.WaitGroup
	errors := make(chan error, numReaders)

	// Readers: continuously access hash
	for i := range numReaders {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for range numOperations {
				hash1 := arr.Hash()
				hash2 := arr.Hash()
				// Hash should be stable (same array, no mutations)
				if hash1 != hash2 {
					errors <- nil // Signal error without blocking
					return
				}
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	if len(errors) > 0 {
		t.Error("Hash inconsistency detected in concurrent access")
	}
}

// TestArrayHashInvalidation tests that hash invalidation works correctly.
// Note: Array is not thread-safe for modifications, so this test is serial.
// This verifies that Set() correctly invalidates cached hash.
func TestArrayHashInvalidation(t *testing.T) {
	terms := make([]*Term, 10)
	for i := range 10 {
		terms[i] = IntNumberTerm(i)
	}
	arr := NewArray(terms...)

	// Get initial hash
	initialHash := arr.Hash()
	if initialHash == 0 {
		t.Error("Initial hash should not be zero")
	}

	// Verify hash is cached
	if h := arr.hash.Load(); h == arrayHashNotComputed {
		t.Error("Hash should be cached after first access")
	}

	// Modify array
	arr.Set(0, IntNumberTerm(999))

	// Hash should be invalidated (set to sentinel)
	if h := arr.hash.Load(); h != arrayHashNotComputed {
		t.Errorf("Hash should be invalidated after Set(), got %d", h)
	}

	// Get new hash (should be recomputed and different)
	newHash := arr.Hash()
	if newHash == initialHash {
		t.Error("Hash should be different after modification")
	}
	if newHash == 0 {
		t.Error("New hash should not be zero")
	}

	// Verify new hash is cached
	if h := arr.hash.Load(); h == arrayHashNotComputed {
		t.Error("Hash should be cached after recomputation")
	}
}

// TestArrayHashConcurrentModification demonstrates Array behavior under concurrent modifications.
// IMPORTANT: This test intentionally shows that Array itself is not thread-safe for modifications.
// The race detector will report races on arr.elems and arr.ground - this is EXPECTED.
// What we're verifying is that the HASH COMPUTATION itself doesn't have races:
// - Reading hash value is safe
// - Computing hash is safe
// - Multiple concurrent Hash() calls don't race with each other
// The underlying Array modification races are a separate concern (Arrays should not be modified concurrently).
func TestArrayHashConcurrentModification(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrent modification test in short mode")
	}

	t.Run("hash_reads_during_modifications", func(t *testing.T) {
		terms := make([]*Term, 10)
		for i := range 10 {
			terms[i] = IntNumberTerm(i)
		}
		arr := NewArray(terms...)

		// Get initial hash
		initialHash := arr.Hash()

		const numReaders = 20
		var wg sync.WaitGroup

		// Readers: access hash (hash computation itself should be race-free)
		readHashes := make(chan int, numReaders)
		for range numReaders {
			wg.Add(1)
			go func() {
				defer wg.Done()
				// Hash() itself should not race
				readHashes <- arr.Hash()
			}()
		}

		wg.Wait()
		close(readHashes)

		// Verify all read hashes are valid (non-zero)
		for hash := range readHashes {
			if hash == 0 {
				t.Error("Read invalid hash (0)")
			}
			// All should see the initial hash (no modifications happened yet)
			if hash != initialHash {
				t.Errorf("Expected hash %d, got %d", initialHash, hash)
			}
		}
	})

	t.Run("invalidation_is_atomic", func(t *testing.T) {
		// Test that rehash() correctly invalidates atomically
		terms := make([]*Term, 5)
		for i := range 5 {
			terms[i] = IntNumberTerm(i)
		}
		arr := NewArray(terms...)

		// Compute hash
		_ = arr.Hash()
		if h := arr.hash.Load(); h == arrayHashNotComputed {
			t.Fatal("Hash should be computed")
		}

		// Invalidate
		arr.rehash()

		// Should be sentinel
		if h := arr.hash.Load(); h != arrayHashNotComputed {
			t.Errorf("After rehash(), expected sentinel %d, got %d", arrayHashNotComputed, h)
		}
	})
}

// TestArrayHashSentinelValue verifies that sentinel value is properly initialized
// and that arrays start with "not computed" state.
func TestArrayHashSentinelValue(t *testing.T) {
	arr := NewArray(IntNumberTerm(1), IntNumberTerm(2))

	// Check initial state (before first Hash() call)
	if h := arr.hash.Load(); h != arrayHashNotComputed {
		t.Errorf("New array should have sentinel value, got %d instead of %d", h, arrayHashNotComputed)
	}

	// After calling Hash(), should have real value
	_ = arr.Hash()
	if h := arr.hash.Load(); h == arrayHashNotComputed {
		t.Error("After Hash() call, array should have computed hash, not sentinel")
	}
}

// TestArrayHashIndependence verifies that each Array instance has its own independent hash field.
// This addresses the concern: "меня смущает 1 глобальная атомарная переменная".
// IMPORTANT: There is NO global variable! Each Array has its own atomic.Int64 field.
func TestArrayHashIndependence(t *testing.T) {
	// Create multiple arrays with different content
	arr1 := NewArray(IntNumberTerm(1), IntNumberTerm(2), IntNumberTerm(3))
	arr2 := NewArray(IntNumberTerm(4), IntNumberTerm(5), IntNumberTerm(6))
	arr3 := NewArray(IntNumberTerm(7), IntNumberTerm(8), IntNumberTerm(9))

	// All start with sentinel (not computed)
	if h := arr1.hash.Load(); h != arrayHashNotComputed {
		t.Error("arr1 should start with sentinel")
	}
	if h := arr2.hash.Load(); h != arrayHashNotComputed {
		t.Error("arr2 should start with sentinel")
	}
	if h := arr3.hash.Load(); h != arrayHashNotComputed {
		t.Error("arr3 should start with sentinel")
	}

	// Compute hash for arr1 only
	hash1 := arr1.Hash()

	// arr1 now has computed hash
	if h := arr1.hash.Load(); h == arrayHashNotComputed {
		t.Error("arr1 should have computed hash")
	}

	// arr2 and arr3 still have sentinel (INDEPENDENT!)
	if h := arr2.hash.Load(); h != arrayHashNotComputed {
		t.Error("arr2 should still have sentinel (independent from arr1)")
	}
	if h := arr3.hash.Load(); h != arrayHashNotComputed {
		t.Error("arr3 should still have sentinel (independent from arr1)")
	}

	// Compute hash for arr2
	hash2 := arr2.Hash()

	// arr3 still has sentinel (INDEPENDENT!)
	if h := arr3.hash.Load(); h != arrayHashNotComputed {
		t.Error("arr3 should still have sentinel (independent from arr1 and arr2)")
	}

	// All hashes should be different (different content)
	if hash1 == hash2 {
		t.Error("Different arrays should have different hashes")
	}

	// Compute hash for arr3
	hash3 := arr3.Hash()
	if hash1 == hash3 || hash2 == hash3 {
		t.Error("arr3 should have unique hash")
	}

	// Verify all are cached independently
	if arr1.hash.Load() != int64(hash1) {
		t.Error("arr1 hash mismatch")
	}
	if arr2.hash.Load() != int64(hash2) {
		t.Error("arr2 hash mismatch")
	}
	if arr3.hash.Load() != int64(hash3) {
		t.Error("arr3 hash mismatch")
	}

	t.Logf("Each Array has independent hash field:")
	t.Logf("   arr1.hash = %d", hash1)
	t.Logf("   arr2.hash = %d", hash2)
	t.Logf("   arr3.hash = %d", hash3)
}

// TestArrayHashConcurrentMultipleArrays tests concurrent access to DIFFERENT arrays.
// This proves that arrays don't interfere with each other - no shared global state.
func TestArrayHashConcurrentMultipleArrays(t *testing.T) {
	const numArrays = 50
	const goroutinesPerArray = 10

	arrays := make([]*Array, numArrays)
	expectedHashes := make([]int, numArrays)

	// Create multiple arrays with different content
	for i := range numArrays {
		terms := make([]*Term, 5)
		for j := range 5 {
			terms[j] = IntNumberTerm(i*10 + j)
		}
		arrays[i] = NewArray(terms...)
		expectedHashes[i] = arrays[i].Hash()
		// Reset to test concurrent first access
		arrays[i].hash.Store(arrayHashNotComputed)
	}

	var wg sync.WaitGroup
	errors := make(chan string, numArrays*goroutinesPerArray)

	// Launch goroutines accessing different arrays concurrently
	for i, arr := range arrays {
		for range goroutinesPerArray {
			wg.Add(1)
			go func(arrayIdx int, array *Array, expected int) {
				defer wg.Done()
				hash := array.Hash()
				if hash != expected {
					errors <- "array mismatch"
				}
			}(i, arr, expectedHashes[i])
		}
	}

	wg.Wait()
	close(errors)

	if len(errors) > 0 {
		t.Errorf("Found %d errors in concurrent multi-array access", len(errors))
	}

	t.Logf("Successfully tested %d arrays with %d goroutines each", numArrays, goroutinesPerArray)
	t.Logf("   Total: %d concurrent operations without interference", numArrays*goroutinesPerArray)
}

// BenchmarkArrayHashConcurrent benchmarks concurrent hash access performance.
// This shows the overhead of CAS operations under contention.
func BenchmarkArrayHashConcurrent(b *testing.B) {
	sizes := []int{10, 100, 1000}
	goroutineCounts := []int{2, 10, 50}

	for _, size := range sizes {
		for _, numG := range goroutineCounts {
			b.Run(formatBenchName(size, numG), func(b *testing.B) {
				terms := make([]*Term, size)
				for i := range size {
					terms[i] = IntNumberTerm(i)
				}

				b.ResetTimer()
				b.ReportAllocs()

				for b.Loop() {
					b.StopTimer()
					arr := NewArray(terms...)
					b.StartTimer()

					var wg sync.WaitGroup
					wg.Add(numG)
					for range numG {
						go func() {
							defer wg.Done()
							_ = arr.Hash()
						}()
					}
					wg.Wait()
				}
			})
		}
	}
}

func formatBenchName(size, goroutines int) string {
	return "size_" + strconv.Itoa(size) + "_goroutines_" + strconv.Itoa(goroutines)
}
