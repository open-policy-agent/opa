// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	v1 "github.com/open-policy-agent/opa/v1/topdown"
)

// Halt is a special error type that built-in function implementations return to indicate
// that policy evaluation should stop immediately.
type Halt = v1.Halt

// Error is the error type returned by the Eval and Query functions when
// an evaluation error occurs.
type Error = v1.Error

const (

	// InternalErr represents an unknown evaluation error.
	InternalErr = v1.InternalErr

	// CancelErr indicates the evaluation process was cancelled.
	CancelErr = v1.CancelErr

	// ConflictErr indicates a conflict was encountered during evaluation. For
	// instance, a conflict occurs if a rule produces multiple, differing values
	// for the same key in an object. Conflict errors indicate the policy does
	// not account for the data loaded into the policy engine.
	ConflictErr = v1.ConflictErr

	// TypeErr indicates evaluation stopped because an expression was applied to
	// a value of an inappropriate type.
	TypeErr = v1.TypeErr

	// BuiltinErr indicates a built-in function received a semantically invalid
	// input or encountered some kind of runtime error, e.g., connection
	// timeout, connection refused, etc.
	BuiltinErr = v1.BuiltinErr

	// WithMergeErr indicates that the real and replacement data could not be merged.
	WithMergeErr = v1.WithMergeErr
)

// IsError returns true if the err is an Error.
func IsError(err error) bool {
	return v1.IsError(err)
}

// IsCancel returns true if err was caused by cancellation.
func IsCancel(err error) bool {
	return v1.IsCancel(err)
}
