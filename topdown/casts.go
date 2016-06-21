// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"fmt"
	"strconv"

	"github.com/open-policy-agent/opa/ast"
	"github.com/pkg/errors"
)

func evalToNumber(ctx *Context, expr *ast.Expr, iter Iterator) error {
	ops := expr.Terms.([]*ast.Term)
	a, b := ops[1].Value, ops[2].Value

	x, err := ValueToInterface(a, ctx)
	if err != nil {
		return fmt.Errorf("to_number")
	}

	var n ast.Number

	switch x := x.(type) {
	case string:
		f, err := strconv.ParseFloat(string(x), 64)
		if err != nil {
			return errors.Wrapf(err, "to_number")
		}
		n = ast.Number(f)
	case float64:
		n = ast.Number(x)
	case bool:
		if x {
			n = ast.Number(1)
		} else {
			n = ast.Number(0)
		}
	default:
		return fmt.Errorf("to_number: source must be a string, boolean, or number: %T", a)
	}

	switch b := b.(type) {
	case ast.Var:
		ctx = ctx.BindVar(b, n)
		return iter(ctx)
	default:
		if n.Equal(b) {
			return iter(ctx)
		}
		return nil
	}
}
