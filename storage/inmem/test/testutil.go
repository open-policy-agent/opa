// Copyright 2022 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package test

import (
	"github.com/open-policy-agent/opa/storage"
	v1 "github.com/open-policy-agent/opa/v1/storage/inmem/test"
)

// New returns an inmem store with some common options set: opt-out of write
// roundtripping.
func New() storage.Store {
	return v1.New()
}

// NewFromObject returns an inmem store from the passed object, with some
// common options set: opt-out of write roundtripping.
func NewFromObject(x map[string]interface{}) storage.Store {
	return v1.NewFromObject(x)
}
