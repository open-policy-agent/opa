// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"math/big"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/topdown/builtins"
)

func builtinCount(a ast.Value) (ast.Value, error) {
	switch a := a.(type) {
	case ast.Array:
		return ast.IntNumberTerm(len(a)).Value, nil
	case ast.Object:
		return ast.IntNumberTerm(len(a)).Value, nil
	case *ast.Set:
		return ast.IntNumberTerm(len(*a)).Value, nil
	case ast.String:
		return ast.IntNumberTerm(len(a)).Value, nil
	}
	return nil, builtins.NewOperandTypeErr(1, a, ast.ArrayTypeName, ast.ObjectTypeName, ast.SetTypeName)
}

func builtinSum(a ast.Value) (ast.Value, error) {
	switch a := a.(type) {
	case ast.Array:
		sum := big.NewFloat(0)
		for _, x := range a {
			n, ok := x.Value.(ast.Number)
			if !ok {
				return nil, builtins.NewOperandElementErr(1, a, x.Value, ast.NumberTypeName)
			}
			sum = new(big.Float).Add(sum, builtins.NumberToFloat(n))
		}
		return builtins.FloatToNumber(sum), nil
	case *ast.Set:
		sum := big.NewFloat(0)
		for _, x := range *a {
			n, ok := x.Value.(ast.Number)
			if !ok {
				return nil, builtins.NewOperandElementErr(1, a, x.Value, ast.NumberTypeName)
			}
			sum = new(big.Float).Add(sum, builtins.NumberToFloat(n))
		}
		return builtins.FloatToNumber(sum), nil
	}
	return nil, builtins.NewOperandTypeErr(1, a, ast.SetTypeName, ast.ArrayTypeName)
}

func builtinMax(a ast.Value) (ast.Value, error) {
	switch a := a.(type) {
	case ast.Array:
		if len(a) == 0 {
			return nil, BuiltinEmpty{}
		}
		var max ast.Value = ast.Null{}
		for i := range a {
			if ast.Compare(max, a[i].Value) <= 0 {
				max = a[i].Value
			}
		}
		return max, nil
	case *ast.Set:
		if len(*a) == 0 {
			return nil, BuiltinEmpty{}
		}
		max, err := a.Reduce(ast.NullTerm(), func(max *ast.Term, elem *ast.Term) (*ast.Term, error) {
			if ast.Compare(max, elem) <= 0 {
				return elem, nil
			}
			return max, nil
		})
		return max.Value, err
	}

	return nil, builtins.NewOperandTypeErr(1, a, ast.SetTypeName, ast.ArrayTypeName)
}

func init() {
	RegisterFunctionalBuiltin1(ast.Count.Name, builtinCount)
	RegisterFunctionalBuiltin1(ast.Sum.Name, builtinSum)
	RegisterFunctionalBuiltin1(ast.Max.Name, builtinMax)
}
