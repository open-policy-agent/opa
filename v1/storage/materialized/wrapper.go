// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package materialized

import (
	"context"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/storage"
)

// TransactionWrapper provides a convenient API for updating data and materialized views atomically
type TransactionWrapper struct {
	ctx      context.Context
	store    storage.Store
	txn      storage.Transaction
	mgr      *Manager
	written  bool
	autoEval bool
}

// NewTransaction creates a transaction wrapper that can automatically refresh materialized views
//
// Usage:
//
//	wrapper := materialized.NewTransaction(ctx, store, compiler, storage.WriteParams)
//	defer wrapper.Abort()
//
//	// Write data
//	wrapper.Write(storage.AddOp, path, data)
//
//	// Commit with automatic view refresh
//	if err := wrapper.CommitWithRefresh(); err != nil {
//	    return err
//	}
func NewTransaction(ctx context.Context, store storage.Store, compiler *ast.Compiler, params storage.TransactionParams) (*TransactionWrapper, error) {
	txn, err := store.NewTransaction(ctx, params)
	if err != nil {
		return nil, err
	}

	return &TransactionWrapper{
		ctx:      ctx,
		store:    store,
		txn:      txn,
		mgr:      NewManager(compiler, store),
		autoEval: true, // Auto-refresh enabled by default
	}, nil
}

// Write wraps store.Write and marks transaction as having data writes
func (w *TransactionWrapper) Write(op storage.PatchOp, path storage.Path, value any) error {
	w.written = true
	return w.store.Write(w.ctx, w.txn, op, path, value)
}

// Read wraps store.Read
func (w *TransactionWrapper) Read(path storage.Path) (any, error) {
	return w.store.Read(w.ctx, w.txn, path)
}

// SetAutoRefresh enables or disables automatic view refresh on commit
func (w *TransactionWrapper) SetAutoRefresh(enabled bool) {
	w.autoEval = enabled
}

// CommitWithRefresh commits the transaction, refreshing materialized views if data was written
func (w *TransactionWrapper) CommitWithRefresh() error {
	// Refresh views if data was written and auto-refresh is enabled
	if w.written && w.autoEval {
		if err := w.mgr.EvaluateSystemStore(w.ctx, w.txn); err != nil {
			return err
		}
	}

	return w.store.Commit(w.ctx, w.txn)
}

// Commit commits without refreshing views (even if data was written)
func (w *TransactionWrapper) Commit() error {
	return w.store.Commit(w.ctx, w.txn)
}

// Abort aborts the transaction
func (w *TransactionWrapper) Abort() {
	w.store.Abort(w.ctx, w.txn)
}

// Transaction returns the underlying storage transaction
func (w *TransactionWrapper) Transaction() storage.Transaction {
	return w.txn
}

// Example usage patterns:

// Example_wrapperBasic demonstrates basic usage of transaction wrapper
func Example_wrapperBasic() {
	ctx := context.Background()
	store := storage.Store(nil) // Your store
	compiler := &ast.Compiler{} // Your compiler

	// Create wrapper
	wrapper, _ := NewTransaction(ctx, store, compiler, storage.WriteParams)
	defer wrapper.Abort()

	// Write data - automatically tracked
	_ = wrapper.Write(storage.AddOp, storage.MustParsePath("/users"), []any{
		map[string]any{"name": "alice", "active": true},
	})

	// Commit with automatic view refresh
	_ = wrapper.CommitWithRefresh()
}

// Example_wrapperManualControl demonstrates manual control over refresh
func Example_wrapperManualControl() {
	ctx := context.Background()
	store := storage.Store(nil)
	compiler := &ast.Compiler{}

	wrapper, _ := NewTransaction(ctx, store, compiler, storage.WriteParams)
	defer wrapper.Abort()

	// Disable auto-refresh for this transaction
	wrapper.SetAutoRefresh(false)

	_ = wrapper.Write(storage.AddOp, storage.MustParsePath("/temp"), "data")

	// Commit without refresh (faster for temporary data)
	_ = wrapper.Commit()
}
