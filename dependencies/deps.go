// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package dependencies

import (
	"github.com/open-policy-agent/opa/ast"
	v1 "github.com/open-policy-agent/opa/v1/dependencies"
)

// All returns the list of data ast.Refs that the given AST element depends on.
func All(x interface{}) (resolved []ast.Ref, err error) {
	return v1.All(x)
}

// Minimal returns the list of data ast.Refs that the given AST element depends on.
// If an AST element depends on a ast.Ref that is a prefix of another dependency, the
// ast.Ref that is the prefix of the other will be the only one in the returned list.
//
// As an example, if an element depends on data.x and data.x.y, only data.x will
// be in the returned list.
func Minimal(x interface{}) (resolved []ast.Ref, err error) {
	return v1.Minimal(x)
}

// Base returns the list of base data documents that the given AST element depends on.
//
// The returned refs are always constant and are truncated at any point where they become
// dynamic. That is, a ref like data.a.b[x] will be truncated to data.a.b.
func Base(compiler *ast.Compiler, x interface{}) ([]ast.Ref, error) {
	return v1.Base(compiler, x)
}

// Virtual returns the list of virtual data documents that the given AST element depends
// on.
//
// The returned refs are always constant and are truncated at any point where they become
// dynamic. That is, a ref like data.a.b[x] will be truncated to data.a.b.
func Virtual(compiler *ast.Compiler, x interface{}) ([]ast.Ref, error) {
	return v1.Virtual(compiler, x)
}
