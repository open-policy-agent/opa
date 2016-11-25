// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"fmt"

	"github.com/open-policy-agent/opa/ast"
	"github.com/pkg/errors"
)

func evalSetDiff(ctx *Context, expr *ast.Expr, iter Iterator) (err error) {
	ops := expr.Terms.([]*ast.Term)
	op1, err := ResolveRefs(ops[1].Value, ctx)
	if err != nil {
		return errors.Wrapf(err, "set_diff")
	}

	s1, ok := op1.(*ast.Set)
	if !ok {
		return &Error{
			Code:    TypeErr,
			Message: fmt.Sprintf("set_diff: first input argument must be set not %T", ops[1].Value),
		}
	}

	op2, err := ResolveRefs(ops[2].Value, ctx)
	if err != nil {
		return errors.Wrapf(err, "set_diff")
	}

	s2, ok := op2.(*ast.Set)
	if !ok {
		return &Error{
			Code:    TypeErr,
			Message: fmt.Sprintf("set_diff: second input argument must be set not %T", ops[2].Value),
		}
	}

	s3 := s1.Diff(s2)
	undo, err := evalEqUnify(ctx, s3, ops[3].Value, nil, iter)
	ctx.Unbind(undo)
	return err
}
