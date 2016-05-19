// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/util"
)

type compareFunc func(a, b interface{}) bool

func compareGreaterThan(a, b interface{}) bool {
	return util.Compare(a, b) > 0
}

func compareGreaterThanEq(a, b interface{}) bool {
	return util.Compare(a, b) >= 0
}

func compareLessThan(a, b interface{}) bool {
	return util.Compare(a, b) < 0
}

func compareLessThanEq(a, b interface{}) bool {
	return util.Compare(a, b) <= 0
}

func compareNotEq(a, b interface{}) bool {
	return util.Compare(a, b) != 0
}

func evalIneq(cmp compareFunc) builtinFunction {
	return func(ctx *Context, expr *ast.Expr, iter Iterator) error {
		ops := expr.Terms.([]*ast.Term)
		a, b := ops[1].Value, ops[2].Value
		av, err := ValueToInterface(a, ctx)
		if err != nil {
			return err
		}
		bv, err := ValueToInterface(b, ctx)
		if err != nil {
			return err
		}
		if cmp(av, bv) {
			return iter(ctx)
		}
		return nil
	}
}
