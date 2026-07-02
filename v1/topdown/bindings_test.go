// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"strconv"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
)

func TestBindingsZeroValues(t *testing.T) {
	t.Parallel()

	var unifier *bindings

	// Plugging
	result := unifier.Plug(term("x"))
	exp := term("x")
	if !result.Equal(exp) {
		t.Fatalf("Expected %v but got %v", exp, result)
	}

	// String
	if unifier.String() != "()" {
		t.Fatalf("Expected empty binding list but got: %v", unifier.String())
	}
}

func term(s string) *ast.Term {
	return ast.MustParseTerm(s)
}

func TestBindingsArrayHashmap(t *testing.T) {
	t.Parallel()

	var bindings bindings
	b := newBindingsArrayHashmap()
	keys := make(map[int]ast.Var)

	for i := range maxLinearScan + 1 {
		b.Put(testBindingKey(i), testBindingValue(&bindings, i))
		keys[i] = testBindingKey(i).Value.(ast.Var)

		testBindingKeys(t, &bindings, &b, keys)
	}

	for i := range maxLinearScan + 1 {
		b.Delete(testBindingKey(i))
		delete(keys, i)

		testBindingKeys(t, &bindings, &b, keys)
	}
}

func testBindingKeys(t *testing.T, bindings *bindings, b *bindingsArrayHashmap, keys map[int]ast.Var) {
	t.Helper()

	for k := range keys {
		value := testBindingValue(bindings, k)
		if v, ok := b.Get(testBindingKey(k)); !ok {
			t.Errorf("value not found: %v", k)
		} else if !v.equal(&value) {
			t.Errorf("value not equal")
		}
	}

	var found []ast.Var
	b.Iter(func(k *ast.Term, v value) bool {
		key := k.Value.(ast.Var)
		if i, _ := strconv.Atoi(string(key)); !testBindingValue(bindings, i).equal(&v) {
			t.Errorf("iteration value note equal")
		}

		found = append(found, key)
		return false
	})

	if len(found) != len(keys) {
		t.Errorf("all keys not found")
	}

next:
	for _, a := range keys {
		for _, b := range found {
			if a == b {
				continue next
			}
		}

		t.Errorf("key not found")
	}
}

func testBindingKey(key int) *ast.Term {
	return ast.VarTerm(strconv.Itoa(key))
}

func testBindingValue(b *bindings, key int) value {
	return value{b, ast.IntNumberTerm(key)}
}

// TestBindingsArrayHashmapDynamicGrowth tests the dynamic growth behavior of the slice-based implementation.
// This validates that the optimization correctly grows the slice incrementally (2 -> 4 -> 8 -> 16).
func TestBindingsArrayHashmapDynamicGrowth(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name         string
		numBindings  int
		expectedCap  int // Expected final capacity
		shouldUseMap bool
	}{
		{"1_binding", 1, 2, false},     // Should allocate cap=2
		{"2_bindings", 2, 2, false},    // Should use cap=2
		{"3_bindings", 3, 4, false},    // Should grow to cap=4
		{"4_bindings", 4, 4, false},    // Should use cap=4
		{"5_bindings", 5, 8, false},    // Should grow to cap=8
		{"8_bindings", 8, 8, false},    // Should use cap=8
		{"9_bindings", 9, 16, false},   // Should grow to cap=16
		{"16_bindings", 16, 16, false}, // Should use cap=16
		{"17_bindings", 17, 0, true},   // Should transition to map
	}

	var bindings bindings
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			b := newBindingsArrayHashmap()

			// Add bindings
			for i := range tc.numBindings {
				b.Put(testBindingKey(i), testBindingValue(&bindings, i))
			}

			// Verify mode (slice vs map)
			if tc.shouldUseMap {
				if b.m == nil {
					t.Errorf("Expected map mode but still in slice mode")
				}
				if b.a != nil {
					t.Errorf("Expected slice to be nil after transition to map")
				}
			} else {
				if b.m != nil {
					t.Errorf("Expected slice mode but transitioned to map")
				}
				if cap(b.a) != tc.expectedCap {
					t.Errorf("Expected capacity %d but got %d", tc.expectedCap, cap(b.a))
				}
			}

			// Verify all values are retrievable
			for i := range tc.numBindings {
				expected := testBindingValue(&bindings, i)
				if v, ok := b.Get(testBindingKey(i)); !ok {
					t.Errorf("Value %d not found", i)
				} else if !v.equal(&expected) {
					t.Errorf("Value %d not equal", i)
				}
			}

			// Verify count via iteration
			count := 0
			b.Iter(func(k *ast.Term, v value) bool {
				count++
				return false
			})
			if count != tc.numBindings {
				t.Errorf("Expected %d bindings but found %d", tc.numBindings, count)
			}
		})
	}
}

// TestBindingsArrayHashmapWithSizeHint tests the pre-allocation behavior with size hints.
func TestBindingsArrayHashmapWithSizeHint(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name         string
		sizeHint     int
		numBindings  int
		expectedCap  int // Expected initial capacity
		shouldUseMap bool
	}{
		{"hint_0", 0, 5, 8, false},        // No hint, dynamic growth
		{"hint_2", 2, 2, 2, false},        // Pre-allocated cap=2
		{"hint_5", 5, 5, 5, false},        // Pre-allocated cap=5
		{"hint_10", 10, 10, 10, false},    // Pre-allocated cap=10
		{"hint_16", 16, 16, 16, false},    // Pre-allocated cap=16
		{"hint_20", 20, 20, 0, true},      // Pre-allocated map
		{"hint_50", 50, 50, 0, true},      // Pre-allocated map
		{"hint_2_grow", 2, 10, 16, false}, // Start with hint=2, grow to 10
	}

	var bindings bindings
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			b := newBindingsArrayHashmapWithSize(tc.sizeHint)

			// Check initial state before any Put operations
			if tc.sizeHint > 0 && tc.sizeHint <= maxLinearScan {
				if cap(b.a) != tc.sizeHint {
					t.Errorf("Expected initial capacity %d but got %d", tc.sizeHint, cap(b.a))
				}
			} else if tc.sizeHint > maxLinearScan {
				if b.m == nil {
					t.Errorf("Expected pre-allocated map but got nil")
				}
			}

			// Add bindings
			for i := range tc.numBindings {
				b.Put(testBindingKey(i), testBindingValue(&bindings, i))
			}

			// Verify mode and capacity
			if tc.shouldUseMap {
				if b.m == nil {
					t.Errorf("Expected map mode but still in slice mode")
				}
			} else {
				if b.m != nil {
					t.Errorf("Expected slice mode but transitioned to map")
				}
				// Only check final capacity for cases that don't transition
				if tc.numBindings <= maxLinearScan && cap(b.a) != tc.expectedCap {
					t.Errorf("Expected final capacity %d but got %d", tc.expectedCap, cap(b.a))
				}
			}

			// Verify all values
			for i := range tc.numBindings {
				expected := testBindingValue(&bindings, i)
				if v, ok := b.Get(testBindingKey(i)); !ok {
					t.Errorf("Value %d not found", i)
				} else if !v.equal(&expected) {
					t.Errorf("Value %d not equal", i)
				}
			}
		})
	}
}

// TestBindingsArrayHashmapUpdateExisting tests updating existing keys.
func TestBindingsArrayHashmapUpdateExisting(t *testing.T) {
	t.Parallel()

	var bindings bindings
	b := newBindingsArrayHashmap()

	// Add initial bindings
	for i := range 5 {
		b.Put(testBindingKey(i), testBindingValue(&bindings, i))
	}

	initialCap := cap(b.a)

	// Update existing keys - should not grow capacity
	for i := range 5 {
		b.Put(testBindingKey(i), testBindingValue(&bindings, i*10))
	}

	if cap(b.a) != initialCap {
		t.Errorf("Capacity changed from %d to %d on update", initialCap, cap(b.a))
	}

	// Verify updated values
	for i := range 5 {
		expected := testBindingValue(&bindings, i*10)
		if v, ok := b.Get(testBindingKey(i)); !ok {
			t.Errorf("Value %d not found", i)
		} else if !v.equal(&expected) {
			t.Errorf("Value %d not updated correctly", i)
		}
	}
}
