// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package inmem

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/open-policy-agent/opa/v1/storage"
)

func TestLazyPolicyBasicOperations(t *testing.T) {
	testData := []byte("package test\n\nimport rego.v1\n\ndefault allow := false\n")

	t.Run("newLazyPolicy", func(t *testing.T) {
		lp := newLazyPolicy(testData)
		if lp == nil {
			t.Fatal("expected non-nil lazyPolicy")
		}
		if lp.size() != len(testData) {
			t.Errorf("expected size %d, got %d", len(testData), lp.size())
		}
		if lp.compressedSize() == 0 {
			t.Error("expected non-zero compressed size")
		}
		// Check hash is computed
		var zeroHash [32]byte
		if lp.Hash() == zeroHash {
			t.Error("expected non-zero hash")
		}
	})

	t.Run("get decompresses correctly", func(t *testing.T) {
		lp := newLazyPolicy(testData)

		decompressed, err := lp.get()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !bytes.Equal(decompressed, testData) {
			t.Error("decompressed data does not match original")
		}

		// Compressed data should still be available (not freed)
		if lp.compressedSize() == 0 {
			t.Error("expected compressed data to still be available")
		}
	})

	t.Run("get returns cached data on subsequent calls", func(t *testing.T) {
		lp := newLazyPolicy(testData)

		// First call
		data1, err := lp.get()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Second call should return same data from cache
		data2, err := lp.get()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !bytes.Equal(data1, data2) {
			t.Error("cached data does not match first call")
		}
	})

	t.Run("nil data returns nil lazyPolicy", func(t *testing.T) {
		lp := newLazyPolicy(nil)
		if lp != nil {
			t.Error("expected nil lazyPolicy for nil data")
		}
	})

	t.Run("compression ratio", func(t *testing.T) {
		// Highly compressible data (repeated patterns)
		compressibleData := bytes.Repeat([]byte("allow := true\n"), 100)
		lp := newLazyPolicy(compressibleData)

		ratio := float64(lp.size()) / float64(lp.compressedSize())
		if ratio < 2.0 {
			t.Logf("compression ratio: %.2fx", ratio)
			t.Log("Note: Snappy prioritizes speed over compression ratio")
		}

		// Verify decompression works
		decompressed, err := lp.get()
		if err != nil {
			t.Fatalf("decompression failed: %v", err)
		}
		if !bytes.Equal(decompressed, compressibleData) {
			t.Error("decompressed data mismatch")
		}
	})
}

func TestLazyPolicyConcurrency(t *testing.T) {
	testData := []byte("package test\n\ndefault allow := false\n")
	lp := newLazyPolicy(testData)

	t.Run("concurrent get operations", func(t *testing.T) {
		const goroutines = 100
		var wg sync.WaitGroup
		wg.Add(goroutines)

		errCh := make(chan error, goroutines)

		for range goroutines {
			go func() {
				defer wg.Done()
				data, err := lp.get()
				if err != nil {
					errCh <- err
					return
				}
				if !bytes.Equal(data, testData) {
					errCh <- errors.New("data mismatch")
				}
			}()
		}

		wg.Wait()
		close(errCh)

		for err := range errCh {
			t.Error(err)
		}
	})
}

func TestLazyPolicyHash(t *testing.T) {
	t.Run("same data produces same hash", func(t *testing.T) {
		data := []byte("package test\n\ndefault allow := true\n")
		lp1 := newLazyPolicy(data)
		lp2 := newLazyPolicy(data)

		if lp1.Hash() != lp2.Hash() {
			t.Error("expected same hash for same data")
		}
	})

	t.Run("different data produces different hash", func(t *testing.T) {
		data1 := []byte("package test\n\ndefault allow := true\n")
		data2 := []byte("package test\n\ndefault allow := false\n")
		lp1 := newLazyPolicy(data1)
		lp2 := newLazyPolicy(data2)

		if lp1.Hash() == lp2.Hash() {
			t.Error("expected different hash for different data")
		}
	})

	t.Run("Equal method works correctly", func(t *testing.T) {
		data1 := []byte("package test\n\ndefault allow := true\n")
		data2 := []byte("package test\n\ndefault allow := false\n")

		lp1 := newLazyPolicy(data1)
		lp2 := newLazyPolicy(data1) // same data
		lp3 := newLazyPolicy(data2) // different data

		if !lp1.Equal(lp2) {
			t.Error("expected Equal to return true for same data")
		}

		if lp1.Equal(lp3) {
			t.Error("expected Equal to return false for different data")
		}

		// Test nil cases
		if lp1.Equal(nil) {
			t.Error("expected Equal to return false for nil")
		}

		var nilPolicy *lazyPolicy
		if !nilPolicy.Equal(nil) {
			t.Error("expected Equal to return true for nil == nil")
		}
	})
}

func TestStorePolicyOperations(t *testing.T) {
	ctx := context.Background()
	policyData := []byte("package test\n\nimport rego.v1\n\nallow if input.x == 1\n")

	t.Run("upsert and get policy", func(t *testing.T) {
		store := New()

		txn, err := store.NewTransaction(ctx, storage.WriteParams)
		if err != nil {
			t.Fatal(err)
		}

		// Upsert policy
		if err := store.UpsertPolicy(ctx, txn, "test-policy", policyData); err != nil {
			t.Fatal(err)
		}

		if err := store.Commit(ctx, txn); err != nil {
			t.Fatal(err)
		}

		// Get policy
		txn, err = store.NewTransaction(ctx)
		if err != nil {
			t.Fatal(err)
		}

		retrieved, err := store.GetPolicy(ctx, txn, "test-policy")
		if err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal(retrieved, policyData) {
			t.Error("retrieved policy does not match original")
		}

		store.Abort(ctx, txn)
	})

	t.Run("get non-existent policy", func(t *testing.T) {
		store := New()

		txn, err := store.NewTransaction(ctx)
		if err != nil {
			t.Fatal(err)
		}

		_, err = store.GetPolicy(ctx, txn, "non-existent")
		if err == nil {
			t.Error("expected error for non-existent policy")
		}

		store.Abort(ctx, txn)
	})

	t.Run("delete policy", func(t *testing.T) {
		store := New()

		// Insert policy
		txn, _ := store.NewTransaction(ctx, storage.WriteParams)
		if err := store.UpsertPolicy(ctx, txn, "test-policy", policyData); err != nil {
			t.Fatal(err)
		}
		if err := store.Commit(ctx, txn); err != nil {
			t.Fatal(err)
		}

		// Delete policy
		txn, _ = store.NewTransaction(ctx, storage.WriteParams)
		if err := store.DeletePolicy(ctx, txn, "test-policy"); err != nil {
			t.Fatal(err)
		}
		if err := store.Commit(ctx, txn); err != nil {
			t.Fatal(err)
		}

		// Verify deletion
		txn, _ = store.NewTransaction(ctx)
		_, err := store.GetPolicy(ctx, txn, "test-policy")
		if err == nil {
			t.Error("expected error for deleted policy")
		}
		store.Abort(ctx, txn)
	})

	t.Run("list policies", func(t *testing.T) {
		store := New()

		// Insert multiple policies
		txn, _ := store.NewTransaction(ctx, storage.WriteParams)
		for i := range 5 {
			id := fmt.Sprintf("policy-%d", i)
			if err := store.UpsertPolicy(ctx, txn, id, policyData); err != nil {
				t.Fatal(err)
			}
		}
		if err := store.Commit(ctx, txn); err != nil {
			t.Fatal(err)
		}

		// List policies
		txn, _ = store.NewTransaction(ctx)
		policies, err := store.ListPolicies(ctx, txn)
		if err != nil {
			t.Fatal(err)
		}

		if len(policies) != 5 {
			t.Errorf("expected 5 policies, got %d", len(policies))
		}

		store.Abort(ctx, txn)
	})

	t.Run("update existing policy", func(t *testing.T) {
		store := New()

		// Insert initial policy
		txn, _ := store.NewTransaction(ctx, storage.WriteParams)
		if err := store.UpsertPolicy(ctx, txn, "test-policy", policyData); err != nil {
			t.Fatal(err)
		}
		if err := store.Commit(ctx, txn); err != nil {
			t.Fatal(err)
		}

		// Update policy
		updatedData := []byte("package test\n\nimport rego.v1\n\nallow if input.x == 2\n")
		txn, _ = store.NewTransaction(ctx, storage.WriteParams)
		if err := store.UpsertPolicy(ctx, txn, "test-policy", updatedData); err != nil {
			t.Fatal(err)
		}
		if err := store.Commit(ctx, txn); err != nil {
			t.Fatal(err)
		}

		// Verify update
		txn, _ = store.NewTransaction(ctx)
		retrieved, err := store.GetPolicy(ctx, txn, "test-policy")
		if err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal(retrieved, updatedData) {
			t.Error("retrieved policy does not match updated data")
		}

		store.Abort(ctx, txn)
	})
}

func TestTransactionPolicyUpdates(t *testing.T) {
	ctx := context.Background()
	policyData := []byte("package test\n\nallow := true\n")

	t.Run("transaction isolation", func(t *testing.T) {
		store := New()

		// Insert policy in transaction 1
		txn1, _ := store.NewTransaction(ctx, storage.WriteParams)
		if err := store.UpsertPolicy(ctx, txn1, "policy-1", policyData); err != nil {

			t.Fatal(err)

		}
		// Don't commit yet

		// Transaction 2 should not see uncommitted policy
		txn2, _ := store.NewTransaction(ctx)
		_, err := store.GetPolicy(ctx, txn2, "policy-1")
		if err == nil {
			t.Error("expected error for uncommitted policy")
		}
		store.Abort(ctx, txn2)

		// Commit transaction 1
		if err := store.Commit(ctx, txn1); err != nil {

			t.Fatal(err)

		}

		// Now transaction 3 should see the policy
		txn3, _ := store.NewTransaction(ctx)
		retrieved, err := store.GetPolicy(ctx, txn3, "policy-1")
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(retrieved, policyData) {
			t.Error("retrieved policy mismatch")
		}
		store.Abort(ctx, txn3)
	})

	t.Run("transaction sees own updates", func(t *testing.T) {
		store := New()

		txn, _ := store.NewTransaction(ctx, storage.WriteParams)

		// Upsert policy
		if err := store.UpsertPolicy(ctx, txn, "policy-1", policyData); err != nil {
			t.Fatal(err)
		}

		// Should be able to read own update before commit
		retrieved, err := store.GetPolicy(ctx, txn, "policy-1")
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(retrieved, policyData) {
			t.Error("transaction cannot see own update")
		}

		if err := store.Commit(ctx, txn); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("abort transaction discards updates", func(t *testing.T) {
		store := New()

		txn1, _ := store.NewTransaction(ctx, storage.WriteParams)
		if err := store.UpsertPolicy(ctx, txn1, "policy-1", policyData); err != nil {

			t.Fatal(err)

		}
		store.Abort(ctx, txn1) // Abort instead of commit

		// Verify policy was not stored
		txn2, _ := store.NewTransaction(ctx)
		_, err := store.GetPolicy(ctx, txn2, "policy-1")
		if err == nil {
			t.Error("expected error for aborted policy")
		}
		store.Abort(ctx, txn2)
	})
}

func TestLargePolicyHandling(t *testing.T) {
	ctx := context.Background()

	// Generate large policy (100KB)
	largePolicy := bytes.Repeat([]byte("package test\n\nallow := true\n"), 4000)

	t.Run("large policy compression and decompression", func(t *testing.T) {
		store := New()

		txn, _ := store.NewTransaction(ctx, storage.WriteParams)
		if err := store.UpsertPolicy(ctx, txn, "large-policy", largePolicy); err != nil {
			t.Fatal(err)
		}
		if err := store.Commit(ctx, txn); err != nil {
			t.Fatal(err)
		}

		// Retrieve and verify
		txn, _ = store.NewTransaction(ctx)
		retrieved, err := store.GetPolicy(ctx, txn, "large-policy")
		if err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal(retrieved, largePolicy) {
			t.Error("large policy data mismatch")
		}

		store.Abort(ctx, txn)
	})
}

func TestEmptyAndEdgeCases(t *testing.T) {
	ctx := context.Background()

	t.Run("empty policy", func(t *testing.T) {
		store := New()

		txn, _ := store.NewTransaction(ctx, storage.WriteParams)
		emptyData := []byte("")
		if err := store.UpsertPolicy(ctx, txn, "empty-policy", emptyData); err != nil {
			t.Fatal(err)
		}
		if err := store.Commit(ctx, txn); err != nil {
			t.Fatal(err)
		}

		txn, _ = store.NewTransaction(ctx)
		retrieved, err := store.GetPolicy(ctx, txn, "empty-policy")
		if err != nil {
			t.Fatal(err)
		}
		if len(retrieved) != 0 {
			t.Error("expected empty policy data")
		}
		store.Abort(ctx, txn)
	})

	t.Run("policy with special characters", func(t *testing.T) {
		store := New()

		specialData := []byte("package test\n\n# Comment with UTF-8: café, 日本語, 🎉\nallow := true\n")
		txn, _ := store.NewTransaction(ctx, storage.WriteParams)
		if err := store.UpsertPolicy(ctx, txn, "special-policy", specialData); err != nil {
			t.Fatal(err)
		}
		if err := store.Commit(ctx, txn); err != nil {
			t.Fatal(err)
		}

		txn, _ = store.NewTransaction(ctx)
		retrieved, err := store.GetPolicy(ctx, txn, "special-policy")
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(retrieved, specialData) {
			t.Error("special character policy data mismatch")
		}
		store.Abort(ctx, txn)
	})
}
