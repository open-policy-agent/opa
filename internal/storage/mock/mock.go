// Copyright 2019 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package mock defines a fake storage implementation for use in testing.
package mock

import (
	"context"
	"fmt"
	"testing"

	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/storage/inmem"
)

// Transaction is a mock storage.Transaction implementation for use in testing.
// It uses an internal storage.Transaction pointer with some added functionality.
type Transaction struct {
	txn       storage.Transaction
	Committed int
	Aborted   int
}

// ID returns the underlying transaction ID
func (t *Transaction) ID() uint64 {
	return t.txn.ID()
}

// Validate returns an error if the transaction is in an invalid state
func (t *Transaction) Validate() error {
	if t.Committed > 1 {
		return fmt.Errorf("transaction %d has too many commits (%d)", t.ID(), t.Committed)
	}
	if t.Aborted > 1 {
		return fmt.Errorf("transaction %d has too many aborts (%d)", t.ID(), t.Committed)
	}
	return nil
}

func (t *Transaction) safeToUse() bool {
	return t.Committed == 0 && t.Aborted == 0
}

// Store is a mock storage.Store implementation for use in testing.
type Store struct {
	inmem        storage.Store
	baseData     map[string]interface{}
	Transactions []*Transaction
	Reads        []*ReadCall
	Writes       []*WriteCall
}

// ReadCall captures the parameters for a Read call
type ReadCall struct {
	Transaction *Transaction
	Path        storage.Path
	Error       error
	Safe        bool
}

// WriteCall captures the parameters for a write call
type WriteCall struct {
	Transaction *Transaction
	Op          storage.PatchOp
	Path        storage.Path
	Error       error
	Safe        bool
}

// New creates a new mock Store
func New() *Store {
	s := &Store{}
	s.Reset()
	return s
}

// NewWithData creates a store with some initial data
func NewWithData(data map[string]interface{}) *Store {
	s := &Store{
		baseData: data,
	}
	s.Reset()
	return s
}

// Reset the store
func (s *Store) Reset() {
	s.Transactions = []*Transaction{}
	s.Reads = []*ReadCall{}
	s.Writes = []*WriteCall{}
	if s.baseData != nil {
		s.inmem = inmem.NewFromObject(s.baseData)
	} else {
		s.inmem = inmem.New()
	}
}

// GetTransaction will a transaction with a specific ID
// that was associated with this Store.
func (s *Store) GetTransaction(id uint64) *Transaction {
	for _, txn := range s.Transactions {
		if txn.ID() == id {
			return txn
		}
	}
	return nil
}

// Errors returns a list of errors for each invalid state found.
// If any Transactions are invalid or reads/writes were
// unsafe an error will be returned for each problem.
func (s *Store) Errors() []error {
	var errs []error
	for _, txn := range s.Transactions {
		err := txn.Validate()
		if err != nil {
			errs = append(errs, err)
		}
	}

	for _, read := range s.Reads {
		if !read.Safe {
			errs = append(errs, fmt.Errorf("unsafe Read call %+v", *read))
		}
	}

	for _, write := range s.Writes {
		if !write.Safe {
			errs = append(errs, fmt.Errorf("unsafe Write call %+v", *write))
		}
	}

	return errs
}

// AssertValid will raise an error with the provided testing.T if
// there are any errors on the store.
func (s *Store) AssertValid(t *testing.T) {
	t.Helper()
	for _, err := range s.Errors() {
		t.Errorf("Error detected on store: %s", err)
	}
}

// storage.Store interface implementation

// Register just shims the call to the underlying inmem store
func (s *Store) Register(ctx context.Context, txn storage.Transaction, config storage.TriggerConfig) (storage.TriggerHandle, error) {
	return s.inmem.Register(ctx, getRealTxn(txn), config)
}

// ListPolicies just shims the call to the underlying inmem store
func (s *Store) ListPolicies(ctx context.Context, txn storage.Transaction) ([]string, error) {
	return s.inmem.ListPolicies(ctx, getRealTxn(txn))
}

// GetPolicy just shims the call to the underlying inmem store
func (s *Store) GetPolicy(ctx context.Context, txn storage.Transaction, name string) ([]byte, error) {
	return s.inmem.GetPolicy(ctx, getRealTxn(txn), name)
}

// UpsertPolicy just shims the call to the underlying inmem store
func (s *Store) UpsertPolicy(ctx context.Context, txn storage.Transaction, name string, policy []byte) error {
	return s.inmem.UpsertPolicy(ctx, getRealTxn(txn), name, policy)
}

// DeletePolicy just shims the call to the underlying inmem store
func (s *Store) DeletePolicy(ctx context.Context, txn storage.Transaction, name string) error {
	return s.inmem.DeletePolicy(ctx, getRealTxn(txn), name)
}

// NewTransaction will create a new transaction on the underlying inmem store
// but wraps it with a mock Transaction. These are then tracked on the store.
func (s *Store) NewTransaction(ctx context.Context, params ...storage.TransactionParams) (storage.Transaction, error) {
	realTxn, err := s.inmem.NewTransaction(ctx, params...)
	if err != nil {
		return nil, err
	}
	txn := &Transaction{
		txn:       realTxn,
		Committed: 0,
		Aborted:   0,
	}
	s.Transactions = append(s.Transactions, txn)
	return txn, nil
}

// Read will make a read from the underlying inmem store and
// add a new entry to the mock store Reads list. If there
// is an error are the read is unsafe it will be noted in
// the ReadCall.
func (s *Store) Read(ctx context.Context, txn storage.Transaction, path storage.Path) (interface{}, error) {
	mockTxn := txn.(*Transaction)

	data, err := s.inmem.Read(ctx, mockTxn.txn, path)

	s.Reads = append(s.Reads, &ReadCall{
		Transaction: mockTxn,
		Path:        path,
		Error:       err,
		Safe:        mockTxn.safeToUse(),
	})

	return data, err
}

// Write will make a read from the underlying inmem store and
// add a new entry to the mock store Writes list. If there
// is an error are the write is unsafe it will be noted in
// the WriteCall.
func (s *Store) Write(ctx context.Context, txn storage.Transaction, op storage.PatchOp, path storage.Path, value interface{}) error {
	mockTxn := txn.(*Transaction)

	err := s.inmem.Write(ctx, mockTxn.txn, op, path, value)

	s.Writes = append(s.Writes, &WriteCall{
		Transaction: mockTxn,
		Op:          op,
		Path:        path,
		Error:       err,
		Safe:        mockTxn.safeToUse(),
	})

	return nil
}

// Commit will commit the underlying transaction while
// also updating the mock Transaction
func (s *Store) Commit(ctx context.Context, txn storage.Transaction) error {
	mockTxn := txn.(*Transaction)

	err := s.inmem.Commit(ctx, mockTxn.txn)
	if err != nil {
		return err
	}

	mockTxn.Committed++
	return nil
}

// Abort will abort the underlying transaction while
// also updating the mock Transaction
func (s *Store) Abort(ctx context.Context, txn storage.Transaction) {
	mockTxn := txn.(*Transaction)
	s.inmem.Abort(ctx, mockTxn.txn)
	mockTxn.Aborted++
}

func (s *Store) Truncate(ctx context.Context, txn storage.Transaction, params storage.TransactionParams, it storage.Iterator) error {
	return s.inmem.Truncate(ctx, getRealTxn(txn), params, it)
}

func getRealTxn(txn storage.Transaction) storage.Transaction {
	return txn.(*Transaction).txn
}
