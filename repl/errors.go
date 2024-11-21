// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package repl

import v1 "github.com/open-policy-agent/opa/v1/repl"

// Error is the error type returned by the REPL.
type Error = v1.Error

const (
	// BadArgsErr indicates bad arguments were provided to a built-in REPL
	// command.
	BadArgsErr string = v1.BadArgsErr
)
