// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package storage

import (
	"context"

	"github.com/open-policy-agent/opa/ast"
)

// Transaction defines the interface that identifies a consistent snapshot over
// the policy engine's storage layer.
type Transaction interface {
	ID() uint64
}

// Store defines the interface for the storage layer's backend.
type Store interface {
	Trigger
	Policy
	Indexing

	// NewTransaction is called create a new transaction in the store.
	NewTransaction(ctx context.Context, params ...TransactionParams) (Transaction, error)

	// Read is called to fetch a document referred to by path.
	Read(ctx context.Context, txn Transaction, path Path) (interface{}, error)

	// Write is called to modify a document referred to by path.
	Write(ctx context.Context, txn Transaction, op PatchOp, path Path, value interface{}) error

	// Commit is called to finish the transaction.
	Commit(ctx context.Context, txn Transaction) error

	// Abort is called to cancel the transaction.
	Abort(ctx context.Context, txn Transaction)
}

// TransactionParams describes a new transaction.
type TransactionParams struct {
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

// Policy defines the interface for policy module storage.
type Policy interface {
	ListPolicies(context.Context, Transaction) ([]string, error)
	GetPolicy(context.Context, Transaction, string) ([]byte, error)
	UpsertPolicy(context.Context, Transaction, string, []byte) error
	DeletePolicy(context.Context, Transaction, string) error
}

// PolicyNotSupported provides a default implementation of the policy interface
// which may be used if the backend does not support policy storage.
type PolicyNotSupported struct{}

// ListPolicies always returns a PolicyNotSupportedErr.
func (PolicyNotSupported) ListPolicies(context.Context, Transaction) ([]string, error) {
	return nil, policyNotSupportedError()
}

// GetPolicy always returns a PolicyNotSupportedErr.
func (PolicyNotSupported) GetPolicy(context.Context, Transaction, string) ([]byte, error) {
	return nil, policyNotSupportedError()
}

// UpsertPolicy always returns a PolicyNotSupportedErr.
func (PolicyNotSupported) UpsertPolicy(context.Context, Transaction, string, []byte) error {
	return policyNotSupportedError()
}

// DeletePolicy always returns a PolicyNotSupportedErr.
func (PolicyNotSupported) DeletePolicy(context.Context, Transaction, string) error {
	return policyNotSupportedError()
}

// TriggerCallback defines the interface that callers can implement to handle
// changes in the stores.
type TriggerCallback func(ctx context.Context, txn Transaction, op PatchOp, path Path, value interface{}) error

// TriggerConfig contains the trigger registration configuration.
type TriggerConfig struct {

	// Before is called before the change is applied to the store.
	Before TriggerCallback

	// After is called after the change is applied to the store.
	After TriggerCallback

	// TODO(tsandall): include callbacks for aborted changes
}

// Trigger defines the interface that stores implement to register for change
// notifications when the store is changed.
type Trigger interface {
	Register(id string, config TriggerConfig) error
	Unregister(id string)
}

// TriggersNotSupported provides default implementations of the Trigger
// interface which may be used if the backend does not support triggers.
type TriggersNotSupported struct{}

// Register always returns an error indicating triggers are not supported.
func (TriggersNotSupported) Register(string, TriggerConfig) error {
	return triggersNotSupportedError()
}

// Unregister is a no-op.
func (TriggersNotSupported) Unregister(string) {
}

// IndexIterator defines the interface for iterating over index results.
type IndexIterator func(*ast.ValueMap) error

// Indexing defines the interface for building and searching storage indices.
type Indexing interface {
	Build(context.Context, Transaction, ast.Ref) error
	Index(context.Context, Transaction, ast.Ref, interface{}, IndexIterator) error
}

// IndexingNotSupported provides default implementations of the Indexing
// interface which may be used if the backend does not support indexing.
type IndexingNotSupported struct{}

// Build always returns an error indicating indexing is not supported.
func (IndexingNotSupported) Build(context.Context, Transaction, ast.Ref) error {
	return indexingNotSupportedError()
}

// Index always returns an error indicating indexing is not supported.
func (IndexingNotSupported) Index(context.Context, Transaction, ast.Ref, interface{}, IndexIterator) error {
	return indexingNotSupportedError()
}
