// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"github.com/open-policy-agent/opa/ast"
	"github.com/pkg/errors"
)

func evalPlus(ctx *Context, expr *ast.Expr, iter Iterator) error {
	ops := expr.Terms.([]*ast.Term)

	a, err := ValueToFloat64(ops[1].Value, ctx)
	if err != nil {
		return errors.Wrapf(err, "plus")
	}

	b, err := ValueToFloat64(ops[2].Value, ctx)
	if err != nil {
		return errors.Wrapf(err, "plus")
	}

	c := ops[3].Value
	r := ast.Number(a + b)

	switch c := c.(type) {
	case ast.Var:
		ctx = ctx.BindVar(c, r)
		return iter(ctx)
	default:
		if r.Equal(c) {
			return iter(ctx)
		}
		return nil
	}
}
