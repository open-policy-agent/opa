// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package repl implements a Read-Eval-Print-Loop (REPL) for interacting with the policy engine.
//
// The REPL is typically used from the command line, however, it can also be used as a library.
// nolint: goconst // String reuse here doesn't make sense to deduplicate.
package repl

import (
	"io"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/storage"
	v1 "github.com/open-policy-agent/opa/v1/repl"
)

// REPL represents an instance of the interactive shell.
type REPL = v1.REPL

// New returns a new instance of the REPL.
func New(store storage.Store, historyPath string, output io.Writer, outputFormat string, errLimit int, banner string) *REPL {
	return v1.New(store, historyPath, output, outputFormat, errLimit, banner).
		WithRegoVersion(ast.DefaultRegoVersion)
}
