// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package inmem implements an in-memory version of the policy engine's storage
// layer.
//
// The in-memory store is used as the default storage layer implementation. The
// in-memory store supports multi-reader/single-writer concurrency with
// rollback.
//
// Callers should assume the in-memory store does not make copies of written
// data. Once data is written to the in-memory store, it should not be modified
// (outside of calling Store.Write). Furthermore, data read from the in-memory
// store should be treated as read-only.
package inmem

import (
	"io"

	"github.com/open-policy-agent/opa/storage"
	v1 "github.com/open-policy-agent/opa/v1/storage/inmem"
)

// New returns an empty in-memory store.
func New() storage.Store {
	return v1.New()
}

// NewWithOpts returns an empty in-memory store, with extra options passed.
func NewWithOpts(opts ...Opt) storage.Store {
	return v1.NewWithOpts(opts...)
}

// NewFromObject returns a new in-memory store from the supplied data object.
func NewFromObject(data map[string]interface{}) storage.Store {
	return v1.NewFromObject(data)
}

// NewFromObjectWithOpts returns a new in-memory store from the supplied data object, with the
// options passed.
func NewFromObjectWithOpts(data map[string]interface{}, opts ...Opt) storage.Store {
	return v1.NewFromObjectWithOpts(data, opts...)
}

// NewFromReader returns a new in-memory store from a reader that produces a
// JSON serialized object. This function is for test purposes.
func NewFromReader(r io.Reader) storage.Store {
	return v1.NewFromReader(r)
}

// NewFromReader returns a new in-memory store from a reader that produces a
// JSON serialized object, with extra options. This function is for test purposes.
func NewFromReaderWithOpts(r io.Reader, opts ...Opt) storage.Store {
	return v1.NewFromReaderWithOpts(r, opts...)
}
