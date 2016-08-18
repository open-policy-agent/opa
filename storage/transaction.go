// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package storage

// Transaction defines the interface that identifies a consistent snapshot over
// the policy engine's storage layer.
type Transaction interface {

	// ID returns a unique identifier for this transaction.
	ID() uint64
}

type transaction uint64

const (
	invalidTXN = transaction(0)
)

func (t transaction) ID() uint64 {
	return uint64(t)
}
