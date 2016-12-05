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

func evalIneq(cmp compareFunc) BuiltinFunc {
	return func(t *Topdown, expr *ast.Expr, iter Iterator) error {
		ops := expr.Terms.([]*ast.Term)
		a, err := ResolveRefs(ops[1].Value, t)
		if err != nil {
			return err
		}
		b, err := ResolveRefs(ops[2].Value, t)
		if err != nil {
			return err
		}
		if cmp(a, b) {
			return iter(t)
		}
		return nil
	}
}
