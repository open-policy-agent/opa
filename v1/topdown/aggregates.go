// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"math"
	"math/big"
	"slices"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/topdown/builtins"
)

func builtinCount(_ BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	switch a := operands[0].Value.(type) {
	case *ast.Array:
		return iter(ast.InternedTerm(a.Len()))
	case ast.Object:
		return iter(ast.InternedTerm(a.Len()))
	case ast.Set:
		return iter(ast.InternedTerm(a.Len()))
	case ast.String:
		return iter(ast.InternedTerm(len([]rune(a))))
	}
	return builtins.NewOperandTypeErr(1, operands[0].Value, "array", "object", "set", "string")
}

// termIterable is satisfied by both *ast.Array and ast.Set.
type termIterable interface {
	Iter(func(*ast.Term) error) error
}

// exactIntAccumulate accumulates the numbers in a with op on exact big.Ints, reporting false if
// any element is not an integer, in which case the caller falls back to the float path.
//
// That float path accumulates in a big.Float carrying the default mantissa, so integers needing
// more significant bits are silently rounded.
func exactIntAccumulate(a termIterable, init int64, op func(z, x, y *big.Int) *big.Int) (ast.Number, bool) {
	acc := big.NewInt(init)
	exact := true

	_ = a.Iter(func(x *ast.Term) error {
		if !exact {
			return nil
		}
		n, ok := x.Value.(ast.Number)
		if !ok {
			exact = false
			return nil
		}
		i, err := builtins.NumberToInt(n)
		if err != nil {
			exact = false
			return nil
		}
		op(acc, acc, i)
		return nil
	})

	if !exact {
		return "", false
	}
	return builtins.IntToNumber(acc), true
}

// addInt returns x+y, reporting false if the sum overflows an int so the caller
// can fall back to exact big.Int accumulation instead of wrapping silently.
func addInt(x, y int) (int, bool) {
	if (y > 0 && x > math.MaxInt-y) || (y < 0 && x < math.MinInt-y) {
		return 0, false
	}
	return x + y, true
}

func builtinSum(_ BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	switch a := operands[0].Value.(type) {
	case *ast.Array:
		// Fast path for arrays of integers
		is := 0
		bail := a.Until(func(x *ast.Term) bool {
			if n, ok := x.Value.(ast.Number); ok {
				if i, ok := n.Int(); ok {
					if s, ok := addInt(is, i); ok {
						is = s
						return false
					}
				}
			}
			return true
		})
		if !bail {
			return iter(ast.InternedTerm(is))
		}

		// A non-integer element, or an integer sum that would overflow the
		// machine int: accumulate on exact big.Ints, falling back to floats for
		// genuinely non-integer input.
		if n, ok := exactIntAccumulate(a, 0, (*big.Int).Add); ok {
			return iter(ast.NewTerm(n))
		}

		sum := new(big.Float)
		tmp := new(big.Float)
		err := a.Iter(func(x *ast.Term) error {
			n, ok := x.Value.(ast.Number)
			if !ok {
				return builtins.NewOperandElementErr(1, a, x.Value, "number")
			}
			sum = sum.Add(sum, builtins.NumberToFloatInto(tmp, n))
			return nil
		})
		if err != nil {
			return err
		}
		return iter(ast.NewTerm(builtins.FloatToNumber(sum)))
	case ast.Set:
		// Fast path for sets of integers
		is := 0
		bail := false
		for _, term := range a.Slice() {
			if n, ok := term.Value.(ast.Number); ok {
				if i, ok := n.Int(); ok {
					if s, ok := addInt(is, i); ok {
						is = s
						continue
					}
				}
			}
			bail = true
			break
		}
		if !bail {
			return iter(ast.InternedTerm(is))
		}

		if n, ok := exactIntAccumulate(a, 0, (*big.Int).Add); ok {
			return iter(ast.NewTerm(n))
		}

		sum := new(big.Float)
		tmp := new(big.Float)

		for _, term := range a.Slice() {
			n, ok := term.Value.(ast.Number)
			if !ok {
				return builtins.NewOperandElementErr(1, a, term.Value, "number")
			}
			sum = sum.Add(sum, builtins.NumberToFloatInto(tmp, n))
		}
		return iter(ast.NewTerm(builtins.FloatToNumber(sum)))
	}
	return builtins.NewOperandTypeErr(1, operands[0].Value, "set", "array")
}

func builtinProduct(_ BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	switch a := operands[0].Value.(type) {
	case *ast.Array:
		if n, ok := exactIntAccumulate(a, 1, (*big.Int).Mul); ok {
			return iter(ast.NewTerm(n))
		}

		product := big.NewFloat(1)
		tmp := new(big.Float)
		err := a.Iter(func(x *ast.Term) error {
			n, ok := x.Value.(ast.Number)
			if !ok {
				return builtins.NewOperandElementErr(1, a, x.Value, "number")
			}
			product = product.Mul(product, builtins.NumberToFloatInto(tmp, n))
			return nil
		})
		if err != nil {
			return err
		}
		return iter(ast.NewTerm(builtins.FloatToNumber(product)))
	case ast.Set:
		if n, ok := exactIntAccumulate(a, 1, (*big.Int).Mul); ok {
			return iter(ast.NewTerm(n))
		}

		product := big.NewFloat(1)
		tmp := new(big.Float)
		err := a.Iter(func(x *ast.Term) error {
			n, ok := x.Value.(ast.Number)
			if !ok {
				return builtins.NewOperandElementErr(1, a, x.Value, "number")
			}
			product = product.Mul(product, builtins.NumberToFloatInto(tmp, n))
			return nil
		})
		if err != nil {
			return err
		}
		return iter(ast.NewTerm(builtins.FloatToNumber(product)))
	}
	return builtins.NewOperandTypeErr(1, operands[0].Value, "set", "array")
}

func builtinMax(_ BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	switch a := operands[0].Value.(type) {
	case *ast.Array:
		if a.Len() == 0 {
			return nil
		}
		max := ast.InternedNullTerm.Value
		a.Foreach(func(x *ast.Term) {
			if max.Compare(x.Value) <= 0 {
				max = x.Value
			}
		})
		return iter(ast.NewTerm(max))
	case ast.Set:
		if a.Len() == 0 {
			return nil
		}
		return iter(slices.MaxFunc(a.Slice(), ast.TermValueCompare))
	}

	return builtins.NewOperandTypeErr(1, operands[0].Value, "set", "array")
}

func builtinMin(_ BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	switch a := operands[0].Value.(type) {
	case *ast.Array:
		if a.Len() == 0 {
			return nil
		}
		min := a.Elem(0).Value
		a.Foreach(func(x *ast.Term) {
			if min.Compare(x.Value) >= 0 {
				min = x.Value
			}
		})
		return iter(ast.NewTerm(min))
	case ast.Set:
		if a.Len() == 0 {
			return nil
		}
		return iter(slices.MinFunc(a.Slice(), ast.TermValueCompare))
	}

	return builtins.NewOperandTypeErr(1, operands[0].Value, "set", "array")
}

func builtinSort(_ BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	switch a := operands[0].Value.(type) {
	case *ast.Array:
		return iter(ast.NewTerm(a.Sorted()))
	case ast.Set:
		return iter(ast.NewTerm(a.Sorted()))
	}
	return builtins.NewOperandTypeErr(1, operands[0].Value, "set", "array")
}

func builtinAll(_ BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	switch val := operands[0].Value.(type) {
	case ast.Set:
		res := true
		match := ast.InternedTerm(true)
		val.Until(func(term *ast.Term) bool {
			if !match.Equal(term) {
				res = false
				return true
			}
			return false
		})
		return iter(ast.InternedTerm(res))
	case *ast.Array:
		res := true
		match := ast.InternedTerm(true)
		val.Until(func(term *ast.Term) bool {
			if !match.Equal(term) {
				res = false
				return true
			}
			return false
		})
		return iter(ast.InternedTerm(res))
	default:
		return builtins.NewOperandTypeErr(1, operands[0].Value, "array", "set")
	}
}

func builtinAny(_ BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	switch val := operands[0].Value.(type) {
	case ast.Set:
		res := val.Len() > 0 && val.Contains(ast.InternedTerm(true))
		return iter(ast.InternedTerm(res))
	case *ast.Array:
		res := false
		match := ast.InternedTerm(true)
		val.Until(func(term *ast.Term) bool {
			if match.Equal(term) {
				res = true
				return true
			}
			return false
		})
		return iter(ast.InternedTerm(res))
	default:
		return builtins.NewOperandTypeErr(1, operands[0].Value, "array", "set")
	}
}

func builtinMember(_ BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	switch c := operands[1].Value.(type) {
	case ast.Set:
		return iter(ast.InternedTerm(c.Contains(operands[0])))
	case *ast.Array:
		return iter(ast.InternedTerm(c.Until(operands[0].Equal)))
	case ast.Object:
		return iter(ast.InternedTerm(c.Until(func(_, v *ast.Term) bool {
			return operands[0].Equal(v)
		})))
	}
	return iter(ast.InternedTerm(false))
}

func builtinMemberWithKey(_ BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	type getter interface {
		Get(*ast.Term) *ast.Term
	}
	col, key, val := operands[2], operands[0], operands[1]
	switch c := col.Value.(type) {
	case ast.Set:
		return iter(ast.InternedTerm(c.Contains(key) && key.Equal(val)))
	case getter:
		return iter(ast.InternedTerm(val.Equal(c.Get(key))))
	}
	return iter(ast.InternedTerm(false))
}

func init() {
	RegisterBuiltinFunc(ast.Count.Name, builtinCount)
	RegisterBuiltinFunc(ast.Sum.Name, builtinSum)
	RegisterBuiltinFunc(ast.Product.Name, builtinProduct)
	RegisterBuiltinFunc(ast.Max.Name, builtinMax)
	RegisterBuiltinFunc(ast.Min.Name, builtinMin)
	RegisterBuiltinFunc(ast.Sort.Name, builtinSort)
	RegisterBuiltinFunc(ast.Any.Name, builtinAny)
	RegisterBuiltinFunc(ast.All.Name, builtinAll)
	RegisterBuiltinFunc(ast.Member.Name, builtinMember)
	RegisterBuiltinFunc(ast.MemberWithKey.Name, builtinMemberWithKey)
}
