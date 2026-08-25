// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import "github.com/open-policy-agent/opa/v1/ast"

func builtinGreaterThan(_ BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	return iter(ast.InternedTerm(operands[0].Value.Compare(operands[1].Value) > 0))
}

func builtinGreaterThanEq(_ BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	return iter(ast.InternedTerm(operands[0].Value.Compare(operands[1].Value) >= 0))
}

func builtinLessThan(_ BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	return iter(ast.InternedTerm(operands[0].Value.Compare(operands[1].Value) < 0))
}

func builtinLessThanEq(_ BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	return iter(ast.InternedTerm(operands[0].Value.Compare(operands[1].Value) <= 0))
}

func builtinNotEqual(_ BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	return iter(ast.InternedTerm(!operands[0].Equal(operands[1])))
}

func builtinEqual(_ BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	return iter(ast.InternedTerm(operands[0].Equal(operands[1])))
}

func init() {
	RegisterBuiltinFunc(ast.GreaterThan.Name, builtinGreaterThan)
	RegisterBuiltinFunc(ast.GreaterThanEq.Name, builtinGreaterThanEq)
	RegisterBuiltinFunc(ast.LessThan.Name, builtinLessThan)
	RegisterBuiltinFunc(ast.LessThanEq.Name, builtinLessThanEq)
	RegisterBuiltinFunc(ast.NotEqual.Name, builtinNotEqual)
	RegisterBuiltinFunc(ast.Equal.Name, builtinEqual)
}
