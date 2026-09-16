// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package storage exposes the policy engine's storage layer.
//
// Deprecated: This package is intended for older projects transitioning from OPA v0.x and will remain for the lifetime of OPA v1.x, but its use is not recommended.
// For newer features and behaviours, such as defaulting to the Rego v1 syntax, use the corresponding components in the [github.com/open-policy-agent/opa/v1] package instead.
// See https://www.openpolicyagent.org/docs/latest/v0-compatibility/ for more information.
package storage

import (
	"context"

	"github.com/open-policy-agent/opa/ast"
	v1 "github.com/open-policy-agent/opa/v1/storage"
)

const (
	// InternalErr indicates an unknown, internal error has occurred.
	InternalErr = v1.InternalErr

	// NotFoundErr indicates the path used in the storage operation does not
	// locate a document.
	NotFoundErr = v1.NotFoundErr

	// WriteConflictErr indicates a write on the path enocuntered a conflicting
	// value inside the transaction.
	WriteConflictErr = v1.WriteConflictErr

	// InvalidPatchErr indicates an invalid patch/write was issued. The patch
	// was rejected.
	InvalidPatchErr = v1.InvalidPatchErr

	// InvalidTransactionErr indicates an invalid operation was performed
	// inside of the transaction.
	InvalidTransactionErr = v1.InvalidTransactionErr

	// TriggersNotSupportedErr indicates the caller attempted to register a
	// trigger against a store that does not support them.
	TriggersNotSupportedErr = v1.TriggersNotSupportedErr

	// WritesNotSupportedErr indicate the caller attempted to perform a write
	// against a store that does not support them.
	WritesNotSupportedErr = v1.WritesNotSupportedErr

	// PolicyNotSupportedErr indicate the caller attempted to perform a policy
	// management operation against a store that does not support them.
	PolicyNotSupportedErr = v1.PolicyNotSupportedErr
)

// Error is the error type returned by the storage layer.
type Error = v1.Error

// IsNotFound returns true if this error is a NotFoundErr.
func IsNotFound(err error) bool {
	return v1.IsNotFound(err)
}

// IsWriteConflictError returns true if this error a WriteConflictErr.
func IsWriteConflictError(err error) bool {
	return v1.IsWriteConflictError(err)
}

// IsInvalidPatch returns true if this error is a InvalidPatchErr.
func IsInvalidPatch(err error) bool {
	return v1.IsInvalidPatch(err)
}

// IsInvalidTransaction returns true if this error is a InvalidTransactionErr.
func IsInvalidTransaction(err error) bool {
	return v1.IsInvalidTransaction(err)
}

// IsIndexingNotSupported is a stub for backwards-compatibility.
//
// Deprecated: We no longer return IndexingNotSupported errors, so it is
// unnecessary to check for them.
func IsIndexingNotSupported(err error) bool {
	return v1.IsIndexingNotSupported(err)
}

// Transaction defines the interface that identifies a consistent snapshot over
// the policy engine's storage layer.
type Transaction = v1.Transaction

// Store defines the interface for the storage layer's backend.
type Store = v1.Store

// MakeDirer defines the interface a Store could realize to override the
// generic MakeDir functionality in storage.MakeDir
type MakeDirer = v1.MakeDirer

// NonEmptyer allows a store implemention to override NonEmpty())
type NonEmptyer = v1.NonEmptyer

// TransactionParams describes a new transaction.
type TransactionParams = v1.TransactionParams

// Context is a simple container for key/value pairs.
type Context = v1.Context

// NewContext returns a new context object.
func NewContext() *Context {
	return v1.NewContext()
}

// WriteParams specifies the TransactionParams for a write transaction.
var WriteParams = v1.WriteParams

// PatchOp is the enumeration of supposed modifications.
type PatchOp = v1.PatchOp

// Patch supports add, remove, and replace operations.
const (
	AddOp     = v1.AddOp
	RemoveOp  = v1.RemoveOp
	ReplaceOp = v1.ReplaceOp
)

// WritesNotSupported provides a default implementation of the write
// interface which may be used if the backend does not support writes.
type WritesNotSupported = v1.WritesNotSupported

// Policy defines the interface for policy module storage.
type Policy = v1.Policy

// PolicyNotSupported provides a default implementation of the policy interface
// which may be used if the backend does not support policy storage.
type PolicyNotSupported = v1.PolicyNotSupported

// PolicyEvent describes a change to a policy.
type PolicyEvent = v1.PolicyEvent

// DataEvent describes a change to a base data document.
type DataEvent = v1.DataEvent

// TriggerEvent describes the changes that caused the trigger to be invoked.
type TriggerEvent = v1.TriggerEvent

// TriggerConfig contains the trigger registration configuration.
type TriggerConfig = v1.TriggerConfig

// Trigger defines the interface that stores implement to register for change
// notifications when the store is changed.
type Trigger = v1.Trigger

// TriggersNotSupported provides default implementations of the Trigger
// interface which may be used if the backend does not support triggers.
type TriggersNotSupported = v1.TriggersNotSupported

// TriggerHandle defines the interface that can be used to unregister triggers that have
// been registered on a Store.
type TriggerHandle = v1.TriggerHandle

// Iterator defines the interface that can be used to read files from a directory starting with
// files at the base of the directory, then sub-directories etc.
type Iterator = v1.Iterator

// Update contains information about a file
type Update = v1.Update

// Path refers to a document in storage.
type Path = v1.Path

// ParsePath returns a new path for the given str.
func ParsePath(str string) (path Path, ok bool) {
	return v1.ParsePath(str)
}

// ParsePathEscaped returns a new path for the given escaped str.
func ParsePathEscaped(str string) (path Path, ok bool) {
	return v1.ParsePathEscaped(str)
}

// NewPathForRef returns a new path for the given ref.
func NewPathForRef(ref ast.Ref) (path Path, err error) {
	return v1.NewPathForRef(ref)
}

// MustParsePath returns a new Path for s. If s cannot be parsed, this function
// will panic. This is mostly for test purposes.
func MustParsePath(s string) Path {
	return v1.MustParsePath(s)
}

// NewTransactionOrDie is a helper function to create a new transaction. If the
// storage layer cannot create a new transaction, this function will panic. This
// function should only be used for tests.
func NewTransactionOrDie(ctx context.Context, store Store, params ...TransactionParams) Transaction {
	return v1.NewTransactionOrDie(ctx, store, params...)
}

// ReadOne is a convenience function to read a single value from the provided Store. It
// will create a new Transaction to perform the read with, and clean up after itself
// should an error occur.
func ReadOne(ctx context.Context, store Store, path Path) (any, error) {
	return v1.ReadOne(ctx, store, path)
}

// WriteOne is a convenience function to write a single value to the provided Store. It
// will create a new Transaction to perform the write with, and clean up after itself
// should an error occur.
func WriteOne(ctx context.Context, store Store, op PatchOp, path Path, value any) error {
	return v1.WriteOne(ctx, store, op, path, value)
}

// MakeDir inserts an empty object at path. If the parent path does not exist,
// MakeDir will create it recursively.
func MakeDir(ctx context.Context, store Store, txn Transaction, path Path) error {
	return v1.MakeDir(ctx, store, txn, path)
}

// Txn is a convenience function that executes f inside a new transaction
// opened on the store. If the function returns an error, the transaction is
// aborted and the error is returned. Otherwise, the transaction is committed
// and the result of the commit is returned.
func Txn(ctx context.Context, store Store, params TransactionParams, f func(Transaction) error) error {
	return v1.Txn(ctx, store, params, f)
}

// NonEmpty returns a function that tests if a path is non-empty. A
// path is non-empty if a Read on the path returns a value or a Read
// on any of the path prefixes returns a non-object value.
func NonEmpty(ctx context.Context, store Store, txn Transaction) func([]string) (bool, error) {
	return v1.NonEmpty(ctx, store, txn)
}
