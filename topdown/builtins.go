// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import "github.com/open-policy-agent/opa/ast"

// BuiltinFunc defines the interface that the evaluation engine uses to
// invoke built-in functions. Users can implement their own built-in functions
// and register them with the evaluation engine.
//
// Callers are given the current evaluation Context ctx with the expression
// expr to be evaluated. Callers can assume that the expression has been plugged
// with bindings from the current context. If the built-in function determines
// that the expression has evaluated successfully it should bind any output variables
// and invoke the iterator with the context produced by binding the output variables.
type BuiltinFunc func(ctx *Context, expr *ast.Expr, iter Iterator) (err error)

// RegisterBuiltinFunc adds a new built-in function to the evaluation engine.
func RegisterBuiltinFunc(name ast.Var, fun BuiltinFunc) {
	builtinFunctions[name] = fun
}

var builtinFunctions map[ast.Var]BuiltinFunc

var defaultBuiltinFuncs = map[ast.Var]BuiltinFunc{
	ast.Equality.Name:      evalEq,
	ast.GreaterThan.Name:   evalIneq(compareGreaterThan),
	ast.GreaterThanEq.Name: evalIneq(compareGreaterThanEq),
	ast.LessThan.Name:      evalIneq(compareLessThan),
	ast.LessThanEq.Name:    evalIneq(compareLessThanEq),
	ast.NotEqual.Name:      evalIneq(compareNotEq),
}

func init() {
	builtinFunctions = map[ast.Var]BuiltinFunc{}
	for name, fun := range defaultBuiltinFuncs {
		RegisterBuiltinFunc(name, fun)
	}
}
