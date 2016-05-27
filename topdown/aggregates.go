// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"fmt"

	"github.com/open-policy-agent/opa/ast"
	"github.com/pkg/errors"
)

func evalCount(ctx *Context, expr *ast.Expr, iter Iterator) error {
	ops := expr.Terms.([]*ast.Term)
	src, dst := ops[1].Value, ops[2].Value
	s, err := ValueToInterface(src, ctx)
	if err != nil {
		return errors.Wrapf(err, "count")
	}

	var count ast.Number

	switch s := s.(type) {
	case []interface{}:
		count = ast.Number(len(s))
	case map[string]interface{}:
		count = ast.Number(len(s))
	default:
		return fmt.Errorf("count: source must be a collection: %v", src)
	}

	switch dst := dst.(type) {
	case ast.Var:
		ctx = ctx.BindVar(dst, count)
		return iter(ctx)
	default:
		if dst.Equal(count) {
			return iter(ctx)
		}
		return nil
	}
}

func evalSum(ctx *Context, expr *ast.Expr, iter Iterator) error {
	ops := expr.Terms.([]*ast.Term)
	src, dst := ops[1].Value, ops[2].Value
	s, err := ValueToSlice(src, ctx)
	if err != nil {
		return errors.Wrapf(err, "sum")
	}

	sum := ast.Number(0)
	for _, x := range s {
		sum += ast.Number(x.(float64))
	}

	switch dst := dst.(type) {
	case ast.Var:
		ctx = ctx.BindVar(dst, sum)
		return iter(ctx)
	default:
		if dst.Equal(sum) {
			return iter(ctx)
		}
		return nil
	}
}
