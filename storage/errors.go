// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package storage

import (
	"fmt"

	"github.com/open-policy-agent/opa/ast"
)

const (
	// InternalErr indicates an unknown, internal error has occurred.
	InternalErr = "storage_internal_error"

	// NotFoundErr indicates the path used in the storage operation does not
	// locate a document.
	NotFoundErr = "storage_not_found_error"

	// InvalidPatchErr indicates an invalid patch/write was issued. The patch
	// was rejected.
	InvalidPatchErr = "storage_invalid_patch_error"

	// MountConflictErr indicates a mount attempt was made on a path that is
	// already used for a mount.
	MountConflictErr = "storage_mount_conflict_error"

	// IndexNotFoundErr indicates the caller attempted to use indexing on a
	// reference that has not been indexed.
	IndexNotFoundErr = "storage_index_not_found_error"

	// IndexingNotSupportedErr indicates the caller attempted to index a
	// reference provided by a store that does not support indexing.
	IndexingNotSupportedErr = "storage_indexing_not_supported_error"

	// TriggersNotSupportedErr indicates the caller attempted to register a
	// trigger against a store that does not support them.
	TriggersNotSupportedErr = "storage_triggers_not_supported_error"

	// WritesNotSupportedErr indicate the caller attempted to perform a write
	// against a store that does not support them.
	WritesNotSupportedErr = "storage_writes_not_supported_error"
)

// Error is the error type returned by the storage layer.
type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (err *Error) Error() string {
	if err.Message != "" {
		return fmt.Sprintf("%v: %v", err.Code, err.Message)
	}
	return string(err.Code)
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
		Code: IndexNotFoundErr,
	}
}

func indexingNotSupportedError() *Error {
	return &Error{
		Code: IndexingNotSupportedErr,
	}
}

func internalError(f string, a ...interface{}) *Error {
	return &Error{
		Code:    InternalErr,
		Message: fmt.Sprintf(f, a...),
	}
}

func invalidPatchErr(f string, a ...interface{}) *Error {
	return &Error{
		Code:    InvalidPatchErr,
		Message: fmt.Sprintf(f, a...),
	}
}

func mountConflictError() *Error {
	return &Error{
		Code: MountConflictErr,
	}
}

func notFoundError(path Path) *Error {
	return notFoundErrorf("%v: %v", path.String(), doesNotExistMsg)
}

func notFoundErrorHint(path Path, hint string) *Error {
	return notFoundErrorf("%v: %v", path.String(), hint)
}

func notFoundRefError(ref ast.Ref) *Error {
	return notFoundErrorf("%v: %v", ref.String(), doesNotExistMsg)
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
		Code: TriggersNotSupportedErr,
	}
}

func writesNotSupportedError() *Error {
	return &Error{
		Code: WritesNotSupportedErr,
	}
}
