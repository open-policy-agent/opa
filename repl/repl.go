// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Deprecated: This package is intended for older projects transitioning from OPA v0.x and will remain for the lifetime of OPA v1.x, but its use is not recommended.
// For newer features and behaviours, such as defaulting to the Rego v1 syntax, use the corresponding components in the [github.com/open-policy-agent/opa/v1] package instead.
// See https://www.openpolicyagent.org/docs/latest/v0-compatibility/ for more information.
//
// Package repl implements a Read-Eval-Print-Loop (REPL) for interacting with the policy engine.
//
// The REPL is typically used from the command line, however, it can also be used as a library.
package repl

import (
	"io"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/storage"
	v1 "github.com/open-policy-agent/opa/v1/repl"
)

// Error is the error type returned by the REPL.
type Error = v1.Error

const (
	// BadArgsErr indicates bad arguments were provided to a built-in REPL
	// command.
	BadArgsErr string = v1.BadArgsErr
)

// REPL represents an instance of the interactive shell.
type REPL = v1.REPL

// New returns a new instance of the REPL.
func New(store storage.Store, historyPath string, output io.Writer, outputFormat string, errLimit int, banner string) *REPL {
	return v1.New(store, historyPath, output, outputFormat, errLimit, banner).
		WithRegoVersion(ast.DefaultRegoVersion)
}
