// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package storage

import (
	"fmt"

	"github.com/open-policy-agent/opa/ast"
)

// ErrCode represents the collection of errors that may be returned by the
// storage layer.
type ErrCode int

const (
	// InternalErr indicates an unknown, internal error has occurred.
	InternalErr ErrCode = iota

	// NotFoundErr indicates the path used in the storage operation does not
	// locate a document.
	NotFoundErr = iota

	// InvalidPatchErr indicates an invalid patch/write was issued. The patch
	// was rejected.
	InvalidPatchErr = iota

	// MountConflictErr indicates a mount attempt was made on a path that is
	// already used for a mount.
	MountConflictErr = iota

	// IndexNotFoundErr indicates the caller attempted to use indexing on a
	// reference that has not been indexed.
	IndexNotFoundErr = iota

	// IndexingNotSupportedErr indicates the caller attempted to index a
	// reference provided by a store that does not support indexing.
	IndexingNotSupportedErr = iota

	// TriggersNotSupportedErr indicates the caller attempted to register a
	// trigger against a store that does not support them.
	TriggersNotSupportedErr = iota

	// WritesNotSupportedErr indicate the caller attempted to perform a write
	// against a store that does not support them.
	WritesNotSupportedErr = iota
)

// Error is the error type returned by the storage layer.
type Error struct {
	Code    ErrCode
	Message string
}

func (err *Error) Error() string {
	return fmt.Sprintf("storage error (code: %d): %v", err.Code, err.Message)
}

// IsNotFound returns true if this error is a NotFoundErr.
func IsNotFound(err error) bool {
	switch err := err.(type) {
	case *Error:
		return err.Code == NotFoundErr
	}
	return false
}

// IsInvalidPatch returns true if this error is a InvalidPatchErr.
func IsInvalidPatch(err error) bool {
	switch err := err.(type) {
	case *Error:
		return err.Code == InvalidPatchErr
	}
	return false
}

var doesNotExistMsg = "document does not exist"
var rootMustBeObjectMsg = "root must be object"
var rootCannotBeRemovedMsg = "root cannot be removed"
var outOfRangeMsg = "array index out of range"
var arrayIndexTypeMsg = "array index must be integer"

func indexNotFoundError() *Error {
	return &Error{
		Code:    IndexNotFoundErr,
		Message: "index not found",
	}
}

func indexingNotSupportedError() *Error {
	return &Error{
		Code:    IndexingNotSupportedErr,
		Message: "indexing not supported",
	}
}

func internalError(f string, a ...interface{}) *Error {
	return &Error{
		Code:    InternalErr,
		Message: fmt.Sprintf(f, a...),
	}
}

func invalidPatchErr(f string, a ...interface{}) *Error {
	msg := fmt.Sprintf("bad patch")
	if len(f) > 0 {
		msg += ": " + fmt.Sprintf(f, a...)
	}
	return &Error{
		Code:    InvalidPatchErr,
		Message: msg,
	}
}

func mountConflictError() *Error {
	return &Error{
		Code:    MountConflictErr,
		Message: "mount conflict",
	}
}

func notFoundError(path Path, f string, a ...interface{}) *Error {
	msg := fmt.Sprintf("bad path: %v", path)
	if len(f) > 0 {
		msg += ", " + fmt.Sprintf(f, a...)
	}
	return notFoundErrorf(msg)
}

func notFoundRefError(ref ast.Ref, f string, a ...interface{}) *Error {
	msg := fmt.Sprintf("bad path: %v", ref)
	if len(f) > 0 {
		msg += ", " + fmt.Sprintf(f, a...)
	}
	return notFoundErrorf(msg)
}

func notFoundErrorf(f string, a ...interface{}) *Error {
	msg := fmt.Sprintf(f, a...)
	return &Error{
		Code:    NotFoundErr,
		Message: msg,
	}
}

func triggersNotSupportedError() *Error {
	return &Error{
		Code:    TriggersNotSupportedErr,
		Message: "triggers not supported",
	}
}

func writesNotSupportedError() *Error {
	return &Error{
		Code:    WritesNotSupportedErr,
		Message: "writes not supported",
	}
}
