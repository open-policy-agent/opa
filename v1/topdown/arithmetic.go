// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"errors"
	"math/big"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/topdown/builtins"
)

// Arithmetic runs in the narrowest representation that holds the operands exactly: an int
// in the int32 window (so that a product fits an int64), then a big.Int, then a big.Float.
// Each operation spells out its behavior per representation, and the result is handed back
// in the narrowest form it fits.
const (
	minSmallInt = -1 << 31
	maxSmallInt = 1<<31 - 1
)

func smallInt(n ast.Number) (int64, bool) {
	i, ok := n.Int64()
	return i, ok && minSmallInt <= i && i <= maxSmallInt
}

// arithOp is a binary operation on numbers.
type arithOp struct {
	// small operates on ints in the int32 window. It reports false when the result is not
	// an int, which makes the operation continue with the wider representations.
	small func(x, y int64) (int64, bool, error)
	// exact operates on integers of any size. If nil, the operation is not exact on big
	// integers and goes straight to floats.
	exact func(z, x, y *big.Int) (*big.Int, error)
	// float is nil for operations that only exist on integers, noFloat being the error
	// for any other operands.
	float   func(z, x, y *big.Float) (*big.Float, error)
	noFloat error
}

func (op arithOp) eval(n1, n2 ast.Number) (*ast.Term, error) {
	if x, ok := smallInt(n1); ok {
		if y, ok := smallInt(n2); ok {
			r, ok, err := op.small(x, y)
			if err != nil {
				return nil, err
			}
			if ok {
				return ast.IntNumberTerm(int(r)), nil
			}
		}
	}

	if op.exact != nil {
		// Going through floats would round integers needing more significant bits than
		// big.Float's default mantissa holds.
		if x, err := builtins.NumberToInt(n1); err == nil {
			if y, err := builtins.NumberToInt(n2); err == nil {
				r, err := op.exact(new(big.Int), x, y)
				if err != nil {
					return nil, err
				}
				return ast.NewTerm(builtins.IntToNumber(r)), nil
			}
		}
	}

	if op.float == nil {
		return nil, op.noFloat
	}

	r, err := op.float(new(big.Float), builtins.NumberToFloat(n1), builtins.NumberToFloat(n2))
	if err != nil {
		return nil, err
	}
	return ast.NewTerm(builtins.FloatToNumber(r)), nil
}

func (op arithOp) builtin() BuiltinFunc {
	return func(_ BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
		n1, err := builtins.NumberOperand(operands[0].Value, 1)
		if err != nil {
			return err
		}
		n2, err := builtins.NumberOperand(operands[1].Value, 2)
		if err != nil {
			return err
		}
		t, err := op.eval(n1, n2)
		if err != nil {
			return err
		}
		return iter(t)
	}
}

// arithUnary is a unary operation on numbers.
type arithUnary struct {
	// small operates on ints in the int32 window.
	small func(x int64) int64
	float func(x *big.Float) *big.Float
}

func (op arithUnary) builtin() BuiltinFunc {
	return func(_ BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
		n, err := builtins.NumberOperand(operands[0].Value, 1)
		if err != nil {
			return err
		}
		if x, ok := smallInt(n); ok {
			return iter(ast.IntNumberTerm(int(op.small(x))))
		}
		return iter(ast.NewTerm(builtins.FloatToNumber(op.float(builtins.NumberToFloat(n)))))
	}
}

func infallible[T any](f func(z, x, y *T) *T) func(z, x, y *T) (*T, error) {
	return func(z, x, y *T) (*T, error) { return f(z, x, y), nil }
}

func always(f func(x, y int64) int64) func(x, y int64) (int64, bool, error) {
	return func(x, y int64) (int64, bool, error) { return f(x, y), true, nil }
}

var (
	opPlus = arithOp{
		small: always(func(x, y int64) int64 { return x + y }),
		exact: infallible((*big.Int).Add),
		float: infallible((*big.Float).Add),
	}

	opMinus = arithOp{
		small: always(func(x, y int64) int64 { return x - y }),
		exact: infallible((*big.Int).Sub),
		float: infallible((*big.Float).Sub),
	}

	opMultiply = arithOp{
		small: always(func(x, y int64) int64 { return x * y }),
		exact: infallible((*big.Int).Mul),
		float: infallible((*big.Float).Mul),
	}

	// Division is only exact on ints when the divisor divides the dividend. Anything else,
	// including big integers and a zero dividend (whose quotient may be -0), is a float quotient.
	opDivide = arithOp{
		small: func(x, y int64) (int64, bool, error) {
			if y == 0 {
				return 0, false, errDivideByZero
			}
			return x / y, x != 0 && x%y == 0, nil
		},
		float: func(z, x, y *big.Float) (*big.Float, error) {
			// A zero divisor of any form: 0.0 and 0e5 are not caught by the int case.
			if i, acc := y.Int64(); acc == big.Exact && i == 0 {
				return nil, errDivideByZero
			}
			return z.Quo(x, y), nil
		},
	}

	// Modulo only exists for integers.
	opRem = arithOp{
		small: func(x, y int64) (int64, bool, error) {
			if y == 0 {
				return 0, false, errModuloByZero
			}
			return x % y, true, nil
		},
		exact: func(z, x, y *big.Int) (*big.Int, error) {
			// Sign, not Int64: Int64 returns the low 64 bits when y does not fit in an
			// int64, so any nonzero multiple of 2^64 (e.g. 10 % 18446744073709551616)
			// would be misreported as modulo by zero.
			if y.Sign() == 0 {
				return nil, errModuloByZero
			}
			return z.Rem(x, y), nil
		},
		noFloat: errModuloOnFloating,
	}

	opAbs = arithUnary{
		small: func(x int64) int64 { return max(x, -x) },
		float: func(x *big.Float) *big.Float { return x.Abs(x) },
	}

	opRound = arithUnary{small: identity, float: arithRound}
	opCeil  = arithUnary{small: identity, float: arithCeil}
	opFloor = arithUnary{small: identity, float: arithFloor}

	errDivideByZero     = errors.New("divide by zero")
	errModuloByZero     = errors.New("modulo by zero")
	errModuloOnFloating = errors.New("modulo on floating-point number")
)

func identity(x int64) int64 { return x }

var halfAwayFromZero = big.NewFloat(0.5)

func arithRound(a *big.Float) *big.Float {
	var i *big.Int
	if a.Signbit() {
		i, _ = new(big.Float).Sub(a, halfAwayFromZero).Int(nil)
	} else {
		i, _ = new(big.Float).Add(a, halfAwayFromZero).Int(nil)
	}
	return new(big.Float).SetInt(i)
}

func arithCeil(a *big.Float) *big.Float {
	i, _ := a.Int(nil)
	f := new(big.Float).SetInt(i)

	if f.Signbit() || a.Cmp(f) == 0 {
		return f
	}

	return new(big.Float).Add(f, big.NewFloat(1.0))
}

func arithFloor(a *big.Float) *big.Float {
	i, _ := a.Int(nil)
	f := new(big.Float).SetInt(i)

	if !f.Signbit() || a.Cmp(f) == 0 {
		return f
	}

	return new(big.Float).Sub(f, big.NewFloat(1.0))
}

func builtinMinus(_ BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	n1, ok1 := operands[0].Value.(ast.Number)
	n2, ok2 := operands[1].Value.(ast.Number)

	if ok1 && ok2 {
		t, err := opMinus.eval(n1, n2)
		if err != nil {
			return err
		}
		return iter(t)
	}

	s1, ok3 := operands[0].Value.(ast.Set)
	s2, ok4 := operands[1].Value.(ast.Set)

	if ok3 && ok4 {
		diff := s1.Diff(s2)
		if diff.Len() == 0 {
			return iter(ast.InternedEmptySet)
		}
		return iter(ast.NewTerm(diff))
	}

	if !ok1 && !ok3 {
		return builtins.NewOperandTypeErr(1, operands[0].Value, "number", "set")
	}

	if ok2 {
		return builtins.NewOperandTypeErr(2, operands[1].Value, "set")
	}

	return builtins.NewOperandTypeErr(2, operands[1].Value, "number")
}

func init() {
	RegisterBuiltinFunc(ast.Abs.Name, opAbs.builtin())
	RegisterBuiltinFunc(ast.Round.Name, opRound.builtin())
	RegisterBuiltinFunc(ast.Ceil.Name, opCeil.builtin())
	RegisterBuiltinFunc(ast.Floor.Name, opFloor.builtin())
	RegisterBuiltinFunc(ast.Plus.Name, opPlus.builtin())
	RegisterBuiltinFunc(ast.Minus.Name, builtinMinus)
	RegisterBuiltinFunc(ast.Multiply.Name, opMultiply.builtin())
	RegisterBuiltinFunc(ast.Divide.Name, opDivide.builtin())
	RegisterBuiltinFunc(ast.Rem.Name, opRem.builtin())
}
