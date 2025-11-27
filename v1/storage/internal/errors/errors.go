// Copyright 2021 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package errors contains reusable error-related code for the storage layer.
package errors

import (
	"fmt"

	"github.com/open-policy-agent/opa/v1/storage"
)

const (
	ArrayIndexTypeMsg      = "array index must be integer"
	DoesNotExistMsg        = "document does not exist"
	OutOfRangeMsg          = "array index out of range"
	RootMustBeObjectMsg    = "root must be object"
	RootCannotBeRemovedMsg = "root cannot be removed"
)

var (
	NotFoundErr            = &storage.Error{Code: storage.NotFoundErr, Message: DoesNotExistMsg}
	RootMustBeObjectErr    = &storage.Error{Code: storage.InvalidPatchErr, Message: RootMustBeObjectMsg}
	RootCannotBeRemovedErr = &storage.Error{Code: storage.InvalidPatchErr, Message: RootCannotBeRemovedMsg}
)

func NewNotFoundErrorWithHint(path storage.Path, hint string) *storage.Error {
	return &storage.Error{
		Code:    storage.NotFoundErr,
		Message: path.String() + ": " + hint,
	}
}

func NewNotFoundErrorf(f string, a ...any) *storage.Error {
	return &storage.Error{
		Code:    storage.NotFoundErr,
		Message: fmt.Sprintf(f, a...),
	}
}

func NewWriteConflictError(p storage.Path) *storage.Error {
	return &storage.Error{
		Code:    storage.WriteConflictErr,
		Message: p.String(),
	}
}

func NewInvalidPatchError(f string, a ...any) *storage.Error {
	return &storage.Error{
		Code:    storage.InvalidPatchErr,
		Message: fmt.Sprintf(f, a...),
	}
}
