// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package storage

import (
	"context"
)

// NewTransactionOrDie is a helper function to create a new transaction. If the
// storage layer cannot create a new transaction, this function will panic. This
// function should only be used for tests.
func NewTransactionOrDie(ctx context.Context, store Store) Transaction {
	txn, err := store.NewTransaction(ctx, TransactionParams{})
	if err != nil {
		panic(err)
	}
	return txn
}
