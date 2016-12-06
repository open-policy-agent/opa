// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package storage

import "context"

// Store defines the interface for the storage layer's backend. Users can
// implement their own stores and mount them into the storage layer to provide
// the policy engine access to external data sources.
type Store interface {
	Trigger

	// Returns a unique identifier for this store. The function should namespace
	// the identifier to avoid potential conflicts, e.g.,
	// com.example/foo-service.
	ID() string

	// Begin is called to indicate that a new transaction has started. The store
	// can use the call to initialize any resources that may be required for the
	// transaction.
	Begin(ctx context.Context, txn Transaction, params TransactionParams) error

	// Read is called to fetch a document referred to by path.
	Read(ctx context.Context, txn Transaction, path Path) (interface{}, error)

	// Write is called to modify a document referred to by path.
	Write(ctx context.Context, txn Transaction, op PatchOp, path Path, value interface{}) error

	// Close indicates a transaction has finished. The store can use the call to
	// release any resources temporarily allocated for the transaction.
	Close(ctx context.Context, txn Transaction)
}

// TransactionParams describes a new transaction.
type TransactionParams struct {

	// Paths represents a set of document paths that may be read during the
	// transaction. The paths may be provided by the caller to hint to the
	// storage layer that certain documents could be pre-loaded.
	Paths []Path
}

// NewTransactionParams returns a new TransactionParams object.
func NewTransactionParams() TransactionParams {
	return TransactionParams{}
}

// WithPaths returns a new TransactionParams object with the paths set.
func (params TransactionParams) WithPaths(paths []Path) TransactionParams {
	params.Paths = paths
	return params
}

// PatchOp is the enumeration of supposed modifications.
type PatchOp int

// Patch supports add, remove, and replace operations.
const (
	AddOp     PatchOp = iota
	RemoveOp          = iota
	ReplaceOp         = iota
)

// WritesNotSupported provides a default implementation of the write
// interface which may be used if the backend does not support writes.
type WritesNotSupported struct{}

func (WritesNotSupported) Write(ctx context.Context, txn Transaction, op PatchOp, path Path, value interface{}) error {
	return writesNotSupportedError()
}
