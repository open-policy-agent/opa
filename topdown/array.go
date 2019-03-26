// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"fmt"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/topdown/builtins"
)

func builtinArrayConcat(a, b ast.Value) (ast.Value, error) {
	arrA, err := builtins.ArrayOperand(a, 1)
	if err != nil {
		return nil, err
	}

	arrB, err := builtins.ArrayOperand(b, 2)
	if err != nil {
		return nil, err
	}

	arrC := make(ast.Array, 0, len(arrA)+len(arrB))

	for _, elemA := range arrA {
		arrC = append(arrC, elemA)
	}

	for _, elemB := range arrB {
		arrC = append(arrC, elemB)
	}

	return arrC, nil
}

func builtinArraySlice(a, i, j ast.Value) (ast.Value, error) {
	arrA, err := builtins.ArrayOperand(a, 1)
	if err != nil {
		return nil, err
	}

	startIndex, err := builtins.IntOperand(i, 1)
	if err != nil {
		return nil, err
	}

	stopIndex, err := builtins.IntOperand(j, 2)
	if err != nil {
		return nil, err
	}

	// Don't allow negative indices for slicing
	if startIndex < 0 || stopIndex < 0 {
		return nil, fmt.Errorf("Invalid slicing operation: negative indices not allowed")
	}

	// stopIndex can't be less than startIndex
	if stopIndex < startIndex {
		return nil, fmt.Errorf("Invalid slicing operation: stopIndex can't be less than startIndex")
	}

	arrB := make(ast.Array, 0, stopIndex-startIndex)

	for i := startIndex; i < stopIndex; i++ {
		arrB = append(arrB, arrA[i])
	}

	return arrB, nil

}

func init() {
	RegisterFunctionalBuiltin2(ast.ArrayConcat.Name, builtinArrayConcat)
	RegisterFunctionalBuiltin3(ast.ArraySlice.Name, builtinArraySlice)
}
