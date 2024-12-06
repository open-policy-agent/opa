// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	v1 "github.com/open-policy-agent/opa/v1/topdown"
)

// Cancel defines the interface for cancelling topdown queries. Cancel
// operations are thread-safe and idempotent.
type Cancel = v1.Cancel

// NewCancel returns a new Cancel object.
func NewCancel() Cancel {
	return v1.NewCancel()
}
