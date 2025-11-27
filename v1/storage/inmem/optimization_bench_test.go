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
	txn, err := store.NewTransaction(ctx, storage.WriteParams)
	if err != nil {
		b.Fatal(err)
	}
	testData := map[string]any{
		"users": map[string]any{
			"alice": map[string]any{"role": "admin"},
			"bob":   map[string]any{"role": "user"},
		},
		"policies": map[string]any{
			"allow": true,
		},
	}
	if err := store.Write(ctx, txn, storage.AddOp, storage.Path{}, testData); err != nil {
		b.Fatal(err)
	}
	if err := store.Commit(ctx, txn); err != nil {
		b.Fatal(err)
	}

	path, ok := storage.ParsePathEscaped("/users/alice")
	if !ok {
		b.Fatal("failed to parse path")
	}

	b.ResetTimer()
	for range b.N {
		txn, err := store.NewTransaction(ctx)
		if err != nil {
			b.Fatal(err)
		}
		if _, err := store.Read(ctx, txn, path); err != nil {
			b.Fatal(err)
		}
		store.Abort(ctx, txn)
	}
}

// BenchmarkStoreReadWrite benchmarks the optimized Read operation in write transaction
func BenchmarkStoreReadWrite(b *testing.B) {
	ctx := context.Background()
	store := New()

	// Setup test data
	txn, err := store.NewTransaction(ctx, storage.WriteParams)
	if err != nil {
		b.Fatal(err)
	}
	testData := map[string]any{
		"users": map[string]any{
			"alice": map[string]any{"role": "admin"},
			"bob":   map[string]any{"role": "user"},
		},
	}
	if err := store.Write(ctx, txn, storage.AddOp, storage.Path{}, testData); err != nil {
		b.Fatal(err)
	}
	if err := store.Commit(ctx, txn); err != nil {
		b.Fatal(err)
	}

	path, ok := storage.ParsePathEscaped("/users/alice")
	if !ok {
		b.Fatal("failed to parse path")
	}

	b.ResetTimer()
	for range b.N {
		txn, err := store.NewTransaction(ctx, storage.WriteParams)
		if err != nil {
			b.Fatal(err)
		}
		if _, err := store.Read(ctx, txn, path); err != nil {
			b.Fatal(err)
		}
		store.Abort(ctx, txn)
	}
}

// BenchmarkStoreWrite benchmarks the optimized Write operation
func BenchmarkStoreWrite(b *testing.B) {
	ctx := context.Background()

	for range b.N {
		store := New()
		txn, err := store.NewTransaction(ctx, storage.WriteParams)
		if err != nil {
			b.Fatal(err)
		}

		path := storage.MustParsePath("/users")
		value := map[string]any{"alice": map[string]any{"role": "admin", "active": true}}

		if err := store.Write(ctx, txn, storage.AddOp, path, value); err != nil {
			b.Fatal(err)
		}
		store.Abort(ctx, txn)
	}
}

// BenchmarkStoreListPolicies benchmarks the optimized ListPolicies operation
func BenchmarkStoreListPolicies(b *testing.B) {
	ctx := context.Background()
	store := New()

	// Setup test policies
	txn, err := store.NewTransaction(ctx, storage.WriteParams)
	if err != nil {
		b.Fatal(err)
	}
	for i := range 50 {
		policyID := "policy" + string(rune('0'+i%10)) + string(rune('a'+i/10))
		policyData := []byte(`package test`)
		if err := store.UpsertPolicy(ctx, txn, policyID, policyData); err != nil {
			b.Fatal(err)
		}
	}
	if err := store.Commit(ctx, txn); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for range b.N {
		txn, err := store.NewTransaction(ctx)
		if err != nil {
			b.Fatal(err)
		}
		if _, err := store.ListPolicies(ctx, txn); err != nil {
			b.Fatal(err)
		}
		store.Abort(ctx, txn)
	}
}

// BenchmarkStoreArrayUpdate benchmarks array update operations (uses newUpdateArrayAST)
func BenchmarkStoreArrayUpdate(b *testing.B) {
	ctx := context.Background()
	store := NewWithOpts(OptReturnASTValuesOnRead(true))

	// Setup test data with array
	txn, err := store.NewTransaction(ctx, storage.WriteParams)
	if err != nil {
		b.Fatal(err)
	}
	testData := map[string]any{
		"items": []any{"a", "b", "c", "d", "e"},
	}
	if err := store.Write(ctx, txn, storage.AddOp, storage.Path{}, testData); err != nil {
		b.Fatal(err)
	}
	if err := store.Commit(ctx, txn); err != nil {
		b.Fatal(err)
	}

	path, ok := storage.ParsePathEscaped("/items/2")
	if !ok {
		b.Fatal("failed to parse path")
	}

	b.ResetTimer()
	for range b.N {
		txn, err := store.NewTransaction(ctx, storage.WriteParams)
		if err != nil {
			b.Fatal(err)
		}
		if err := store.Write(ctx, txn, storage.ReplaceOp, path, "updated"); err != nil {
			b.Fatal(err)
		}
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
		txn, err := store.NewTransaction(ctx, storage.WriteParams)
		if err != nil {
			b.Fatal(err)
		}
		testData := map[string]any{
			"users": map[string]any{
				"alice":   map[string]any{"role": "admin"},
				"bob":     map[string]any{"role": "user"},
				"charlie": map[string]any{"role": "guest"},
			},
		}
		if err := store.Write(ctx, txn, storage.AddOp, storage.Path{}, testData); err != nil {
			b.Fatal(err)
		}

		path, ok := storage.ParsePathEscaped("/users/bob")
		if !ok {
			b.Fatal("failed to parse path")
		}
		if err := store.Write(ctx, txn, storage.RemoveOp, path, nil); err != nil {
			b.Fatal(err)
		}
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
		txn, err := store.NewTransaction(ctx, storage.WriteParams)
		if err != nil {
			b.Fatal(err)
		}
		testData := map[string]any{
			"items": []any{"a", "b", "c", "d", "e", "f", "g"},
		}
		if err := store.Write(ctx, txn, storage.AddOp, storage.Path{}, testData); err != nil {
			b.Fatal(err)
		}

		path, ok := storage.ParsePathEscaped("/items/3")
		if !ok {
			b.Fatal("failed to parse path")
		}
		if err := store.Write(ctx, txn, storage.RemoveOp, path, nil); err != nil {
			b.Fatal(err)
		}
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
		db := New()
		for range b.N {
			txn, err := db.NewTransaction(ctx, storage.WriteParams)
			if err != nil {
				b.Fatal(err)
			}
			// Add multiple data updates
			for i := range 10 {
				path := storage.MustParsePath("/test" + string(rune('0'+i)))
				if err := db.Write(ctx, txn, storage.AddOp, path, map[string]any{"value": i}); err != nil {
					b.Fatal(err)
				}
			}
			if err := db.Commit(ctx, txn); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("policies_only", func(b *testing.B) {
		db := New()
		for range b.N {
			txn, err := db.NewTransaction(ctx, storage.WriteParams)
			if err != nil {
				b.Fatal(err)
			}
			// Add multiple policy updates
			for i := range 10 {
				if err := db.UpsertPolicy(ctx, txn, "policy"+string(rune('0'+i)), []byte("package test")); err != nil {
					b.Fatal(err)
				}
			}
			if err := db.Commit(ctx, txn); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("mixed", func(b *testing.B) {
		db := New()
		for range b.N {
			txn, err := db.NewTransaction(ctx, storage.WriteParams)
			if err != nil {
				b.Fatal(err)
			}
			// Add data updates
			for i := range 5 {
				path := storage.MustParsePath("/test" + string(rune('0'+i)))
				if err := db.Write(ctx, txn, storage.AddOp, path, map[string]any{"value": i}); err != nil {
					b.Fatal(err)
				}
			}
			// Add policy updates
			for i := range 5 {
				if err := db.UpsertPolicy(ctx, txn, "policy"+string(rune('0'+i)), []byte("package test")); err != nil {
					b.Fatal(err)
				}
			}
			if err := db.Commit(ctx, txn); err != nil {
				b.Fatal(err)
			}
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
		for range b.N {
			count := 0
			for range underlying.All() {
				count++
			}
		}
	})

	b.Run("traditional_list", func(b *testing.B) {
		b.ResetTimer()
		for b.Loop() {
			count := 0
			for curr := underlying.updates.Front(); curr != nil; curr = curr.Next() {
				_ = curr.Value.(dataUpdate)
				count++
			}
		}
	})

	db.Abort(ctx, txn)
}
