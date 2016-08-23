// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package storage

import "github.com/open-policy-agent/opa/ast"

// Store defines the interface for plugging into the policy engine's storage
// layer. Users can implement this interface to provide the policy engine access
// to data stored outside the default, built-in store.
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

	// Read is called to fetch a document referred to by path.
	Read(txn Transaction, ref ast.Ref) (interface{}, error)

	// Finished is called to indicate that a transaction has finished. The
	// store can use the call to clean up any resources that may have been
	// allocated for the transaction.
	Finished(txn Transaction)
}
