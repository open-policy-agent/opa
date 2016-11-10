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

	undo, err := evalEqUnify(ctx, s, ops[3].Value, nil, iter)
	ctx.Unbind(undo)
	return err
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

	undo, err := evalEqUnify(ctx, s, ops[3].Value, nil, iter)
	ctx.Unbind(undo)
	return err
}

func evalIndexOf(ctx *Context, expr *ast.Expr, iter Iterator) error {
	ops := expr.Terms.([]*ast.Term)

	base, err := ValueToString(ops[1].Value, ctx)
	if err != nil {
		return errors.Wrapf(err, "%v: base value must be a string", ast.IndexOf.Name)
	}

	search, err := ValueToString(ops[2].Value, ctx)
	if err != nil {
		return errors.Wrapf(err, "%v: search value must be a string", ast.IndexOf.Name)
	}

	index := ast.Number(strings.Index(base, search))

	undo, err := evalEqUnify(ctx, index, ops[3].Value, nil, iter)
	ctx.Unbind(undo)
	return err
}

func evalSubstring(ctx *Context, expr *ast.Expr, iter Iterator) error {
	ops := expr.Terms.([]*ast.Term)

	base, err := ValueToString(ops[1].Value, ctx)
	if err != nil {
		return errors.Wrapf(err, "%v: base value must be a string", ast.Substring.Name)
	}

	startIndex, err := ValueToInt(ops[2].Value, ctx)
	if err != nil {
		return errors.Wrapf(err, "%v: start index must be a number", ast.Substring.Name)
	}

	l, err := ValueToInt(ops[3].Value, ctx)
	if err != nil {
		return errors.Wrapf(err, "%v: length must be a number", ast.Substring.Name)
	}

	var s ast.String
	if l < 0 {
		s = ast.String(base[startIndex:])
	} else {
		s = ast.String(base[startIndex : startIndex+l])
	}

	undo, err := evalEqUnify(ctx, s, ops[4].Value, nil, iter)
	ctx.Unbind(undo)
	return err
}

func evalContains(ctx *Context, expr *ast.Expr, iter Iterator) error {
	ops := expr.Terms.([]*ast.Term)

	base, err := ValueToString(ops[1].Value, ctx)
	if err != nil {
		return errors.Wrapf(err, "%v: base value must be a string", ast.Contains.Name)
	}

	search, err := ValueToString(ops[2].Value, ctx)
	if err != nil {
		return errors.Wrapf(err, "%v: search must be a string", ast.Contains.Name)
	}

	if strings.Contains(base, search) {
		return iter(ctx)
	}
	return nil
}

func evalStartsWith(ctx *Context, expr *ast.Expr, iter Iterator) error {
	ops := expr.Terms.([]*ast.Term)

	base, err := ValueToString(ops[1].Value, ctx)
	if err != nil {
		return errors.Wrapf(err, "%v: base value must be a string", ast.StartsWith.Name)
	}

	search, err := ValueToString(ops[2].Value, ctx)
	if err != nil {
		return errors.Wrapf(err, "%v: search must be a string", ast.StartsWith.Name)
	}

	if strings.HasPrefix(base, search) {
		return iter(ctx)
	}
	return nil
}

func evalEndsWith(ctx *Context, expr *ast.Expr, iter Iterator) error {
	ops := expr.Terms.([]*ast.Term)

	base, err := ValueToString(ops[1].Value, ctx)
	if err != nil {
		return errors.Wrapf(err, "%v: base value must be a string", ast.EndsWith.Name)
	}

	search, err := ValueToString(ops[2].Value, ctx)
	if err != nil {
		return errors.Wrapf(err, "%v: search must be a string", ast.EndsWith.Name)
	}

	if strings.HasSuffix(base, search) {
		return iter(ctx)
	}
	return nil
}

func evalLower(ctx *Context, expr *ast.Expr, iter Iterator) error {
	ops := expr.Terms.([]*ast.Term)

	orig, err := ValueToString(ops[1].Value, ctx)
	if err != nil {
		return errors.Wrapf(err, "%v: original value must be a string", ast.Lower.Name)
	}

	s := ast.String(strings.ToLower(orig))

	undo, err := evalEqUnify(ctx, s, ops[2].Value, nil, iter)
	ctx.Unbind(undo)
	return err
}

func evalUpper(ctx *Context, expr *ast.Expr, iter Iterator) error {
	ops := expr.Terms.([]*ast.Term)

	orig, err := ValueToString(ops[1].Value, ctx)
	if err != nil {
		return errors.Wrapf(err, "%v: original value must be a string", ast.Upper.Name)
	}

	s := ast.String(strings.ToUpper(orig))

	undo, err := evalEqUnify(ctx, s, ops[2].Value, nil, iter)
	ctx.Unbind(undo)
	return err
}
