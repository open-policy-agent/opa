// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/open-policy-agent/opa/ast"
)

func jsonNumberToFloat(n json.Number) *big.Float {
	r, ok := new(big.Float).SetString(string(n))
	if !ok {
		panic("illegal value")
	}
	return r
}

func floatToJSONNumber(f *big.Float) json.Number {
	return json.Number(f.String())
}

func floatToASTNumber(f *big.Float) ast.Number {
	return ast.Number(floatToJSONNumber(f))
}

type arithArity1 func(a *big.Float) (*big.Float, error)

func arithAbs(a *big.Float) (*big.Float, error) {
	return a.Abs(a), nil
}

var halfAwayFromZero = big.NewFloat(0.5)

func arithRound(a *big.Float) (*big.Float, error) {
	var i *big.Int
	if a.Signbit() {
		i, _ = new(big.Float).Sub(a, halfAwayFromZero).Int(nil)
	} else {
		i, _ = new(big.Float).Add(a, halfAwayFromZero).Int(nil)
	}
	return new(big.Float).SetInt(i), nil
}

type arithArity2 func(a, b *big.Float) (*big.Float, error)

func arithPlus(a, b *big.Float) (*big.Float, error) {
	return new(big.Float).Add(a, b), nil
}

func arithMinus(a, b *big.Float) (*big.Float, error) {
	return new(big.Float).Sub(a, b), nil
}

func arithMultiply(a, b *big.Float) (*big.Float, error) {
	return new(big.Float).Mul(a, b), nil
}

func arithDivide(a, b *big.Float) (*big.Float, error) {
	i, acc := b.Int64()
	if acc == big.Exact && i == 0 {
		return nil, fmt.Errorf("divide: by zero")
	}
	return new(big.Float).Quo(a, b), nil
}

func evalArithArity1(f arithArity1) BuiltinFunc {
	return func(ctx *Context, expr *ast.Expr, iter Iterator) error {
		ops := expr.Terms.([]*ast.Term)

		a, err := ValueToJSONNumber(ops[1].Value, ctx)
		if err != nil {
			return expr.Location.Wrapf(err, "expected number (operand %s is not a number)", ops[0].Location.Text)
		}

		x, err := f(jsonNumberToFloat(a))
		if err != nil {
			return err
		}

		b := ops[2].Value
		undo, err := evalEqUnify(ctx, floatToASTNumber(x), b, nil, iter)
		ctx.Unbind(undo)
		return err
	}
}

func evalArithArity2(f arithArity2) BuiltinFunc {
	return func(ctx *Context, expr *ast.Expr, iter Iterator) error {
		ops := expr.Terms.([]*ast.Term)

		a, err := ValueToJSONNumber(ops[1].Value, ctx)
		if err != nil {
			return expr.Location.Wrapf(err, "expected number (first operand %s is not a number)", ops[0].Location.Text)
		}

		b, err := ValueToJSONNumber(ops[2].Value, ctx)
		if err != nil {
			return expr.Location.Wrapf(err, "expected number (second operand %s is not a number)", ops[2].Location.Text)
		}

		c, err := f(jsonNumberToFloat(a), jsonNumberToFloat(b))
		if err != nil {
			return err
		}

		cv := ops[3].Value

		undo, err := evalEqUnify(ctx, floatToASTNumber(c), cv, nil, iter)
		ctx.Unbind(undo)
		return err
	}
}
