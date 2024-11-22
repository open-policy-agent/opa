// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ir

import (
	"io"

	v1 "github.com/open-policy-agent/opa/v1/ir"
)

// Pretty writes a human-readable representation of an IR object to w.
func Pretty(w io.Writer, x interface{}) error {
	return v1.Pretty(w, x)
}
