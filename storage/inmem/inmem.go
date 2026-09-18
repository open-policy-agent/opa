// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Deprecated: This package is intended for older projects transitioning from OPA v0.x and will remain for the lifetime of OPA v1.x, but its use is not recommended.
// For newer features and behaviours, such as defaulting to the Rego v1 syntax, use the corresponding components in the [github.com/open-policy-agent/opa/v1] package instead.
// See https://www.openpolicyagent.org/docs/latest/v0-compatibility/ for more information.
//
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
func NewFromObject(data map[string]any) storage.Store {
	return v1.NewFromObject(data)
}

// NewFromObjectWithOpts returns a new in-memory store from the supplied data object, with the
// options passed.
func NewFromObjectWithOpts(data map[string]any, opts ...Opt) storage.Store {
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

// An Opt modifies store at instantiation.
type Opt = v1.Opt

// OptRoundTripOnWrite sets whether incoming objects written to store are
// round-tripped through JSON to ensure they are serializable to JSON.
//
// Callers should disable this if they can guarantee all objects passed to
// Write() are serializable to JSON. Failing to do so may result in undefined
// behavior, including panics.
//
// Usually, when only storing objects in the inmem store that have been read
// via encoding/json, this is safe to disable, and comes with an improvement
// in performance and memory use.
//
// If setting to false, callers should deep-copy any objects passed to Write()
// unless they can guarantee the objects will not be mutated after being written,
// and that mutations happening to the objects after they have been passed into
// Write() don't affect their logic.
func OptRoundTripOnWrite(enabled bool) Opt {
	return v1.OptRoundTripOnWrite(enabled)
}

// OptReturnASTValuesOnRead sets whether data values added to the store should be
// eagerly converted to AST values, which are then returned on read.
//
// When enabled, this feature does not sanity check data before converting it to AST values,
// which may result in panics if the data is not valid. Callers should ensure that passed data
// can be serialized to AST values; otherwise, it's recommended to also enable OptRoundTripOnWrite.
func OptReturnASTValuesOnRead(enabled bool) Opt {
	return v1.OptReturnASTValuesOnRead(enabled)
}
