// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"fmt"

	"github.com/open-policy-agent/opa/ast"
)

// Error is the error type returned by the Eval and Query functions when
// an evaluation error occurs.
type Error struct {
	Code     string        `json:"code"`
	Message  string        `json:"message"`
	Location *ast.Location `json:"location,omitempty"`
}

const (

	// InternalErr represents an unknown evaluation error.
	InternalErr string = "eval_internal_error"

	// ConflictErr indicates a conflict was encountered during evaluation. For
	// instance, a conflict occurs if a rule produces multiple, differing values
	// for the same key in an object. Conflict errors indicate the policy does
	// not account for the data loaded into the policy engine.
	ConflictErr string = "eval_conflict_error"

	// TypeErr indicates evaluation stopped because an expression was applied to
	// a value of an inappropriate type.
	TypeErr string = "eval_type_error"
)

// IsError returns true if the err is an Error.
func IsError(err error) bool {
	_, ok := err.(*Error)
	return ok
}

func (e *Error) Error() string {

	msg := fmt.Sprintf("%v: %v", e.Code, e.Message)

	if e.Location != nil {
		msg = e.Location.String() + ": " + msg
	}

	return msg
}

func completeDocConflictErr(loc *ast.Location) error {
	return &Error{
		Code:     ConflictErr,
		Location: loc,
		Message:  "completely defined rules must produce exactly one value",
	}
}

func objectDocKeyConflictErr(loc *ast.Location) error {
	return &Error{
		Code:     ConflictErr,
		Location: loc,
		Message:  "partial rule definitions must produce exactly one value per object key",
	}
}

func unsupportedBuiltinErr(loc *ast.Location) error {
	return &Error{
		Code:     InternalErr,
		Location: loc,
		Message:  "unsupported built-in",
	}
}

func objectDocKeyTypeErr(loc *ast.Location) error {
	return &Error{
		Code:     TypeErr,
		Location: loc,
		Message:  "partial rule definitions must produce string values for object keys",
	}
}
