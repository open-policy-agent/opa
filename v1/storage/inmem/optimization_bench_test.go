// Copyright 2025 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package inmem

import (
	"context"
	"testing"

	"github.com/open-policy-agent/opa/v1/storage"
)

// BenchmarkStoreRead benchmarks the optimized Read operation
func BenchmarkStoreRead(b *testing.B) {
	ctx := context.Background()
	store := New()

	// Setup test data
	txn, _ := store.NewTransaction(ctx, storage.WriteParams)
	testData := map[string]any{
		"users": map[string]any{
			"alice": map[string]any{"role": "admin"},
			"bob":   map[string]any{"role": "user"},
		},
		"policies": map[string]any{
			"allow": true,
		},
	}
	_ = store.Write(ctx, txn, storage.AddOp, storage.Path{}, testData)
	_ = store.Commit(ctx, txn)

	path, _ := storage.ParsePathEscaped("/users/alice")

	b.ResetTimer()
	for range b.N {
		txn, _ := store.NewTransaction(ctx)
		_, _ = store.Read(ctx, txn, path)
		store.Abort(ctx, txn)
	}
}

// BenchmarkStoreReadWrite benchmarks the optimized Read operation in write transaction
func BenchmarkStoreReadWrite(b *testing.B) {
	ctx := context.Background()
	store := New()

	// Setup test data
	txn, _ := store.NewTransaction(ctx, storage.WriteParams)
	testData := map[string]any{
		"users": map[string]any{
			"alice": map[string]any{"role": "admin"},
			"bob":   map[string]any{"role": "user"},
		},
	}
	_ = store.Write(ctx, txn, storage.AddOp, storage.Path{}, testData)
	_ = store.Commit(ctx, txn)

	path, _ := storage.ParsePathEscaped("/users/alice")

	b.ResetTimer()
	for range b.N {
		txn, _ := store.NewTransaction(ctx, storage.WriteParams)
		_, _ = store.Read(ctx, txn, path)
		store.Abort(ctx, txn)
	}
}

// BenchmarkStoreWrite benchmarks the optimized Write operation
func BenchmarkStoreWrite(b *testing.B) {
	ctx := context.Background()

	b.ResetTimer()
	for range b.N {
		store := New()
		txn, _ := store.NewTransaction(ctx, storage.WriteParams)

		path, _ := storage.ParsePathEscaped("/users/alice")
		value := map[string]any{"role": "admin", "active": true}

		_ = store.Write(ctx, txn, storage.AddOp, path, value)
		store.Abort(ctx, txn)
	}
}

// BenchmarkStoreListPolicies benchmarks the optimized ListPolicies operation
func BenchmarkStoreListPolicies(b *testing.B) {
	ctx := context.Background()
	store := New()

	// Setup test policies
	txn, _ := store.NewTransaction(ctx, storage.WriteParams)
	for i := range 50 {
		policyID := "policy" + string(rune('0'+i%10)) + string(rune('a'+i/10))
		policyData := []byte(`package test`)
		_ = store.UpsertPolicy(ctx, txn, policyID, policyData)
	}
	_ = store.Commit(ctx, txn)

	b.ResetTimer()
	for range b.N {
		txn, _ := store.NewTransaction(ctx)
		_, _ = store.ListPolicies(ctx, txn)
		store.Abort(ctx, txn)
	}
}

// BenchmarkStoreArrayUpdate benchmarks array update operations (uses newUpdateArrayAST)
func BenchmarkStoreArrayUpdate(b *testing.B) {
	ctx := context.Background()
	store := NewWithOpts(OptReturnASTValuesOnRead(true))

	// Setup test data with array
	txn, _ := store.NewTransaction(ctx, storage.WriteParams)
	testData := map[string]any{
		"items": []any{"a", "b", "c", "d", "e"},
	}
	_ = store.Write(ctx, txn, storage.AddOp, storage.Path{}, testData)
	_ = store.Commit(ctx, txn)

	path, _ := storage.ParsePathEscaped("/items/2")

	b.ResetTimer()
	for range b.N {
		txn, _ := store.NewTransaction(ctx, storage.WriteParams)
		_ = store.Write(ctx, txn, storage.ReplaceOp, path, "updated")
		store.Abort(ctx, txn)
	}
}

// BenchmarkStoreObjectRemove benchmarks object remove operations (uses removeInAstObject)
func BenchmarkStoreObjectRemove(b *testing.B) {
	ctx := context.Background()
	store := NewWithOpts(OptReturnASTValuesOnRead(true))

	b.ResetTimer()
	for range b.N {
		// Setup fresh data for each iteration
		txn, _ := store.NewTransaction(ctx, storage.WriteParams)
		testData := map[string]any{
			"users": map[string]any{
				"alice":   map[string]any{"role": "admin"},
				"bob":     map[string]any{"role": "user"},
				"charlie": map[string]any{"role": "guest"},
			},
		}
		_ = store.Write(ctx, txn, storage.AddOp, storage.Path{}, testData)

		path, _ := storage.ParsePathEscaped("/users/bob")
		_ = store.Write(ctx, txn, storage.RemoveOp, path, nil)
		store.Abort(ctx, txn)
	}
}

// BenchmarkStoreArrayRemove benchmarks array remove operations (uses removeInAstArray)
func BenchmarkStoreArrayRemove(b *testing.B) {
	ctx := context.Background()
	store := NewWithOpts(OptReturnASTValuesOnRead(true))

	b.ResetTimer()
	for range b.N {
		// Setup fresh data for each iteration
		txn, _ := store.NewTransaction(ctx, storage.WriteParams)
		testData := map[string]any{
			"items": []any{"a", "b", "c", "d", "e", "f", "g"},
		}
		_ = store.Write(ctx, txn, storage.AddOp, storage.Path{}, testData)

		path, _ := storage.ParsePathEscaped("/items/3")
		_ = store.Write(ctx, txn, storage.RemoveOp, path, nil)
		store.Abort(ctx, txn)
	}
}

// BenchmarkMktree benchmarks the optimized mktree function
func BenchmarkMktree(b *testing.B) {
	tests := []struct {
		name  string
		path  []string
		value any
	}{
		{"single", []string{"a"}, "value"},
		{"double", []string{"a", "b"}, "value"},
		{"triple", []string{"a", "b", "c"}, "value"},
		{"deep", []string{"a", "b", "c", "d", "e"}, "value"},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			b.ResetTimer()
			for range b.N {
				_, _ = mktree(tt.path, tt.value)
			}
		})
	}
}

// BenchmarkCommit benchmarks transaction commit with pre-allocated slices
func BenchmarkCommit(b *testing.B) {
	ctx := context.Background()

	b.Run("data_only", func(b *testing.B) {
		for range b.N {
			b.StopTimer()
			db := New()
			txn, _ := db.NewTransaction(ctx, storage.WriteParams)
			// Add multiple data updates
			for i := range 10 {
				path := storage.MustParsePath("/data/test" + string(rune('0'+i)))
				_ = db.Write(ctx, txn, storage.AddOp, path, map[string]any{"value": i})
			}
			b.StartTimer()
			_ = db.Commit(ctx, txn)
		}
	})

	b.Run("policies_only", func(b *testing.B) {
		for range b.N {
			b.StopTimer()
			db := New()
			txn, _ := db.NewTransaction(ctx, storage.WriteParams)
			// Add multiple policy updates
			for i := range 10 {
				_ = db.UpsertPolicy(ctx, txn, "policy"+string(rune('0'+i)), []byte("package test"))
			}
			b.StartTimer()
			_ = db.Commit(ctx, txn)
		}
	})

	b.Run("mixed", func(b *testing.B) {
		for range b.N {
			b.StopTimer()
			db := New()
			txn, _ := db.NewTransaction(ctx, storage.WriteParams)
			// Add data updates
			for i := range 5 {
				path := storage.MustParsePath("/data/test" + string(rune('0'+i)))
				_ = db.Write(ctx, txn, storage.AddOp, path, map[string]any{"value": i})
			}
			// Add policy updates
			for i := range 5 {
				_ = db.UpsertPolicy(ctx, txn, "policy"+string(rune('0'+i)), []byte("package test"))
			}
			b.StartTimer()
			_ = db.Commit(ctx, txn)
		}
	})
}

// BenchmarkLookup benchmarks the lookup function
func BenchmarkLookup(b *testing.B) {
	data := map[string]any{
		"a": map[string]any{
			"b": map[string]any{
				"c": map[string]any{
					"d": "value",
				},
			},
		},
	}

	tests := []struct {
		name string
		path storage.Path
	}{
		{"shallow", storage.Path{"a"}},
		{"medium", storage.Path{"a", "b"}},
		{"deep", storage.Path{"a", "b", "c", "d"}},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			b.ResetTimer()
			for range b.N {
				_, _ = lookup(tt.path, data)
			}
		})
	}
}

// BenchmarkIterator benchmarks the new iterator pattern vs traditional list iteration
func BenchmarkIterator(b *testing.B) {
	ctx := context.Background()
	db := New()

	// Setup: create transaction with many updates
	txn, _ := db.NewTransaction(ctx, storage.WriteParams)
	for i := range 50 {
		path := storage.MustParsePath("/data/item" + string(rune('0'+i%10)))
		_ = db.Write(ctx, txn, storage.AddOp, path, map[string]any{"value": i})
	}

	underlying := txn.(*transaction)

	b.Run("iterator_pattern", func(b *testing.B) {
		b.ResetTimer()
		for range b.N {
			count := 0
			for range underlying.All() {
				count++
			}
		}
	})

	b.Run("traditional_list", func(b *testing.B) {
		b.ResetTimer()
		for range b.N {
			count := 0
			for curr := underlying.updates.Front(); curr != nil; curr = curr.Next() {
				_ = curr.Value.(dataUpdate)
				count++
			}
		}
	})

	db.Abort(ctx, txn)
}
