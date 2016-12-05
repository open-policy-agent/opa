// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/util"
	"github.com/pkg/errors"
)

type reduceFunc func(x interface{}) (ast.Value, error)

type empty struct{}

func (e empty) Error() string {
	return "empty"
}

func evalReduce(f reduceFunc) BuiltinFunc {
	return func(t *Topdown, expr *ast.Expr, iter Iterator) error {
		ops := expr.Terms.([]*ast.Term)
		src, dst := ops[1].Value, ops[2].Value
		x, err := ValueToInterface(src, t)
		if err != nil {
			return errors.Wrapf(err, "aggregate")
		}

		y, err := f(x)
		if err != nil {
			switch err.(type) {
			case empty:
				return nil
			}
			return err
		}

		undo, err := evalEqUnify(t, y, dst, nil, iter)
		t.Unbind(undo)
		return err
	}
}

func reduceSum(x interface{}) (ast.Value, error) {
	if s, ok := x.([]interface{}); ok {
		sum := big.NewFloat(0)
		for _, x := range s {
			n, ok := x.(json.Number)
			if !ok {
				return nil, fmt.Errorf("sum: input elements must be numbers")
			}
			sum = new(big.Float).Add(sum, jsonNumberToFloat(n))
		}
		return floatToASTNumber(sum), nil
	}
	return nil, fmt.Errorf("sum: source must be array")
}

func reduceCount(x interface{}) (ast.Value, error) {
	switch x := x.(type) {
	case []interface{}:
		return ast.IntNumberTerm(len(x)).Value, nil
	case map[string]interface{}:
		return ast.IntNumberTerm(len(x)).Value, nil
	case string:
		return ast.IntNumberTerm(len(x)).Value, nil
	default:
		return nil, fmt.Errorf("count: source must be array, object, or string")
	}
}

func reduceMax(x interface{}) (ast.Value, error) {
	switch x := x.(type) {
	case []interface{}:
		if len(x) == 0 {
			return nil, empty{}
		}
		var max interface{}
		for i := range x {
			if util.Compare(max, x[i]) <= 0 {
				max = x[i]
			}
		}
		return ast.InterfaceToValue(max)
	}
	return nil, fmt.Errorf("max: source must be array")
}
