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
	Code     int
	Message  string
	Location *ast.Location
}

const (

	// InternalErr represents an unknown evaluation error.
	InternalErr = iota

	// ConflictErr indicates a conflict was encountered during evaluation. For
	// instance, a conflict occurs if a rule produces multiple, differing values
	// for the same key in an object. Conflict errors indicate the policy does
	// not account for the data loaded into the policy engine.
	ConflictErr = iota

	// TypeErr indicates evaluation stopped because an expression was applied to
	// a value of an inappropriate type.
	TypeErr = iota
)

func (e *Error) Error() string {

	msg := fmt.Sprintf("evaluation error (code: %v): %v", e.Code, e.Message)

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

func setDereferenceTypeErr(loc *ast.Location) error {
	return &Error{
		Code:     TypeErr,
		Location: loc,
		Message:  "set documents cannot be dereferenced",
	}
}
