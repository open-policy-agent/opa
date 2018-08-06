// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
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

func init() {
	RegisterFunctionalBuiltin2(ast.ArrayConcat.Name, builtinArrayConcat)
}
