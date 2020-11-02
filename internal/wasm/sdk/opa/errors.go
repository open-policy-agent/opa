// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package opa

import (
	"errors"
)

var (
	// ErrInvalidConfig is the error returned if the OPA initialization fails due to an invalid config.
	ErrInvalidConfig = errors.New("invalid config")
	// ErrInvalidPolicyOrData is the error returned if either policy or data is invalid.
	ErrInvalidPolicyOrData = errors.New("invalid policy or data")
	// ErrInvalidBundle is the error returned if the bundle loaded is corrupted.
	ErrInvalidBundle = errors.New("invalid bundle")
	// ErrNotReady is the error returned if the OPA instance is not initialized.
	ErrNotReady = errors.New("not ready")
	// ErrUndefined is the error returned if the evaluation result is undefined.
	ErrUndefined = errors.New("undefined decision")
	// ErrNonBoolean is the error returned if the evaluation result is not of boolean value.
	ErrNonBoolean = errors.New("non-boolean decision")
	// ErrInternal is the error returned if the evaluation fails due to an internal error.
	ErrInternal = errors.New("internal error")
)
