// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package storage

import "github.com/open-policy-agent/opa/ast"

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
	// transaction. The caller will provide refs hinting the paths that may be
	// read during the transaction.
	Begin(txn Transaction, refs []ast.Ref) error

	// Read is called to fetch a document referred to by ref.
	Read(txn Transaction, ref ast.Ref) (interface{}, error)

	// Write is called to modify a document referred to by ref.
	Write(txn Transaction, op PatchOp, ref ast.Ref, value interface{}) error

	// Close indicates a transaction has finished. The store can use the call to
	// release any resources temporarily allocated for the transaction.
	Close(txn Transaction)
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

func (WritesNotSupported) Write(txn Transaction, op PatchOp, ref ast.Ref, value interface{}) error {
	return writesNotSupportedError()
}
