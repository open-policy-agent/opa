// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package dependencies provides functions for determining the set of ast.Refs that AST
// elements depend on.
//
// Deprecated: This package is intended for older projects transitioning from OPA v0.x and will remain for the lifetime of OPA v1.x, but its use is not recommended.
// For newer features and behaviours, such as defaulting to the Rego v1 syntax, use the corresponding components in the [github.com/open-policy-agent/opa/v1] package instead.
// See https://www.openpolicyagent.org/docs/latest/v0-compatibility/ for more information.
package dependencies

import (
	"github.com/open-policy-agent/opa/ast"
	v1 "github.com/open-policy-agent/opa/v1/dependencies"
)

// All returns the list of data ast.Refs that the given AST element depends on.
func All(x any) (resolved []ast.Ref, err error) {
	return v1.All(x)
}

// Minimal returns the list of data ast.Refs that the given AST element depends on.
// If an AST element depends on a ast.Ref that is a prefix of another dependency, the
// ast.Ref that is the prefix of the other will be the only one in the returned list.
//
// As an example, if an element depends on data.x and data.x.y, only data.x will
// be in the returned list.
func Minimal(x any) (resolved []ast.Ref, err error) {
	return v1.Minimal(x)
}

// Base returns the list of base data documents that the given AST element depends on.
//
// The returned refs are always constant and are truncated at any point where they become
// dynamic. That is, a ref like data.a.b[x] will be truncated to data.a.b.
func Base(compiler *ast.Compiler, x any) ([]ast.Ref, error) {
	return v1.Base(compiler, x)
}

// Virtual returns the list of virtual data documents that the given AST element depends
// on.
//
// The returned refs are always constant and are truncated at any point where they become
// dynamic. That is, a ref like data.a.b[x] will be truncated to data.a.b.
func Virtual(compiler *ast.Compiler, x any) ([]ast.Ref, error) {
	return v1.Virtual(compiler, x)
}
