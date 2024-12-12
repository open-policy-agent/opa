// Copyright 2021 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"io"

	"github.com/open-policy-agent/opa/topdown/print"
	v1 "github.com/open-policy-agent/opa/v1/topdown"
)

func NewPrintHook(w io.Writer) print.Hook {
	return v1.NewPrintHook(w)
}
