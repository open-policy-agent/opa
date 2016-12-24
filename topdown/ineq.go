// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import "github.com/open-policy-agent/opa/ast"

type compareFunc func(a, b ast.Value) bool

func compareGreaterThan(a, b ast.Value) bool {
	return ast.Compare(a, b) > 0
}

func compareGreaterThanEq(a, b ast.Value) bool {
	return ast.Compare(a, b) >= 0
}

func compareLessThan(a, b ast.Value) bool {
	return ast.Compare(a, b) < 0
}

func compareLessThanEq(a, b ast.Value) bool {
	return ast.Compare(a, b) <= 0
}

func compareNotEq(a, b ast.Value) bool {
	return ast.Compare(a, b) != 0
}

func builtinCompare(cmp compareFunc) FunctionalBuiltinVoid2 {
	return func(a, b ast.Value) error {
		if !cmp(a, b) {
			return BuiltinEmpty{}
		}
		return nil
	}
}

func init() {
	RegisterFunctionalBuiltinVoid2(ast.GreaterThan.Name, builtinCompare(compareGreaterThan))
	RegisterFunctionalBuiltinVoid2(ast.GreaterThanEq.Name, builtinCompare(compareGreaterThanEq))
	RegisterFunctionalBuiltinVoid2(ast.LessThan.Name, builtinCompare(compareLessThan))
	RegisterFunctionalBuiltinVoid2(ast.LessThanEq.Name, builtinCompare(compareLessThanEq))
	RegisterFunctionalBuiltinVoid2(ast.NotEqual.Name, builtinCompare(compareNotEq))
}
