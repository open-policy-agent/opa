// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"strconv"
	"strings"

	"github.com/open-policy-agent/opa/ast"
	"github.com/pkg/errors"
)

func evalFormatInt(ctx *Context, expr *ast.Expr, iter Iterator) error {
	ops := expr.Terms.([]*ast.Term)

	input, err := ValueToFloat64(ops[1].Value, ctx)
	if err != nil {
		return errors.Wrapf(err, "%v: input must be a number", ast.FormatInt.Name)
	}

	i := int64(input)

	base, err := ValueToFloat64(ops[2].Value, ctx)
	if err != nil {
		return errors.Wrapf(err, "%v: base must be a number", ast.FormatInt.Name)
	}

	b := int(base)
	s := ast.String(strconv.FormatInt(i, b))

	// TODO(tsandall): revisit handling of output arguments. If a reference is given in an output
	// position, the built-in should evaluate the referenced value. Several of the built-ins call
	// the ast.Equal function on the output argument (which is not correct for references.)
	switch r := ops[3].Value.(type) {
	case ast.Var:
		return Continue(ctx, r, s, iter)
	case ast.String:
		if r.Equal(s) {
			return iter(ctx)
		}
	}

	return nil
}

func evalConcat(ctx *Context, expr *ast.Expr, iter Iterator) error {
	ops := expr.Terms.([]*ast.Term)

	join, err := ValueToString(ops[1].Value, ctx)
	if err != nil {
		return errors.Wrapf(err, "%v: join value must be a string", ast.Concat.Name)
	}

	sl, err := ValueToStrings(ops[2].Value, ctx)
	if err != nil {
		return errors.Wrapf(err, "%v: input value must be array of strings", ast.Concat.Name)
	}

	s := ast.String(strings.Join(sl, join))

	switch r := ops[3].Value.(type) {
	case ast.Var:
		return Continue(ctx, r, s, iter)
	case ast.String:
		if r.Equal(s) {
			return iter(ctx)
		}
	}
	return nil
}
