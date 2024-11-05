// Copyright 2024 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package compile

import (
	"github.com/open-policy-agent/opa/ast"
)

// NewCompiler returns a new empty [ast.Compiler].
//
// This is a convenience function for [ast.NewCompiler] that sets the default [ast.RegoVersion] to [ast.RegoV1].
func NewCompiler() *ast.Compiler {
	return ast.NewCompiler().WithDefaultRegoVersion(ast.RegoV1)
}
