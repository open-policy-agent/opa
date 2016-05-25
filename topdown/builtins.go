// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import "github.com/open-policy-agent/opa/ast"

type builtinFunction func(*Context, *ast.Expr, Iterator) error

var builtinFunctions = map[ast.Var]builtinFunction{
	ast.Equality.Name:      evalEq,
	ast.GreaterThan.Name:   evalIneq(compareGreaterThan),
	ast.GreaterThanEq.Name: evalIneq(compareGreaterThanEq),
	ast.LessThan.Name:      evalIneq(compareLessThan),
	ast.LessThanEq.Name:    evalIneq(compareLessThanEq),
	ast.NotEqual.Name:      evalIneq(compareNotEq),
}
