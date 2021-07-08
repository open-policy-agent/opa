// Copyright 2021 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package disk

import (
	"github.com/open-policy-agent/opa/storage"
)

var errNotFound = &storage.Error{Code: storage.NotFoundErr}

func wrapError(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := err.(*storage.Error); ok {
		return err
	}
	// NOTE(tsandall): we intentionally do not convert badger.ErrKeyNotFound to
	// NotFoundErr code here because the former may not always need to be
	// represented as a NotFoundErr (i.e., it may depend on the call-site.)
	return &storage.Error{Code: storage.InternalErr, Message: err.Error()}
}
