// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"fmt"
	"math"

	"github.com/open-policy-agent/opa/ast"
)

type arithArity1 func(a float64) (ast.Number, error)

func arithAbs(a float64) (ast.Number, error) {
	return ast.Number(math.Abs(a)), nil
}

func arithRound(a float64) (ast.Number, error) {
	return ast.Number(math.Floor(a + 0.5)), nil
}

type arithArity2 func(a, b float64) (ast.Number, error)

func arithPlus(a, b float64) (ast.Number, error) {
	return ast.Number(a + b), nil
}

func arithMinus(a, b float64) (ast.Number, error) {
	return ast.Number(a - b), nil
}

func arithMultiply(a, b float64) (ast.Number, error) {
	return ast.Number(a * b), nil
}

func arithDivide(a, b float64) (ast.Number, error) {
	if b == 0 {
		return 0, fmt.Errorf("divide: by zero")
	}
	return ast.Number(a / b), nil
}

func evalArithArity1(f arithArity1) BuiltinFunc {
	return func(ctx *Context, expr *ast.Expr, iter Iterator) error {
		ops := expr.Terms.([]*ast.Term)
		a, err := ValueToFloat64(ops[1].Value, ctx)
		if err != nil {
			return expr.Location.Wrapf(err, "expected number (operand %s is not a number)", ops[0].Location.Text)
		}

		r, err := f(a)
		if err != nil {
			return err
		}

		b := ops[2].Value

		switch b := b.(type) {
		case ast.Var:
			ctx = ctx.BindValue(b, r)
			return iter(ctx)
		default:
			if b.Equal(r) {
				return iter(ctx)
			}
			return nil
		}
	}
}

func evalArithArity2(f arithArity2) BuiltinFunc {
	return func(ctx *Context, expr *ast.Expr, iter Iterator) error {
		ops := expr.Terms.([]*ast.Term)

		a, err := ValueToFloat64(ops[1].Value, ctx)
		if err != nil {
			return expr.Location.Wrapf(err, "expected number (first operand %s is not a number)", ops[0].Location.Text)
		}

		b, err := ValueToFloat64(ops[2].Value, ctx)
		if err != nil {
			return expr.Location.Wrapf(err, "expected number (second operand %s is not a number)", ops[2].Location.Text)
		}

		c, err := f(a, b)
		if err != nil {
			return err
		}

		cv := ops[3].Value

		switch cv := cv.(type) {
		case ast.Var:
			ctx = ctx.BindValue(cv, c)
			return iter(ctx)
		default:
			if cv.Equal(c) {
				return iter(ctx)
			}
			return nil
		}
	}
}
