// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Deprecated: This package is intended for older projects transitioning from OPA v0.x and will remain for the lifetime of OPA v1.x, but its use is not recommended.
// For newer features and behaviours, such as defaulting to the Rego v1 syntax, use the corresponding components in the [github.com/open-policy-agent/opa/v1] package instead.
// See https://www.openpolicyagent.org/docs/latest/v0-compatibility/ for more information.
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
func NewFromObject(x map[string]any) storage.Store {
	return v1.NewFromObject(x)
}
