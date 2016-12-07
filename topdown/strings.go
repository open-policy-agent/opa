// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"fmt"
	"strings"

	"github.com/open-policy-agent/opa/ast"
	"github.com/pkg/errors"
)

func evalFormatInt(t *Topdown, expr *ast.Expr, iter Iterator) error {
	ops := expr.Terms.([]*ast.Term)

	input, err := ValueToJSONNumber(ops[1].Value, t)
	if err != nil {
		return errors.Wrapf(err, "%v: input must be a number", ast.FormatInt.Name)
	}

	i, _ := jsonNumberToFloat(input).Int(nil)

	base, err := ValueToInt(ops[2].Value, t)
	if err != nil {
		return errors.Wrapf(err, "%v: base must be an integer", ast.FormatInt.Name)
	}

	var format string
	switch base {
	case 2:
		format = "%b"
	case 8:
		format = "%o"
	case 10:
		format = "%d"
	case 16:
		format = "%x"
	default:
		return errors.Wrapf(err, "%v: base must be one of 2, 8, 10, 16", ast.FormatInt.Name)
	}

	s := ast.String(fmt.Sprintf(format, i))

	undo, err := evalEqUnify(t, s, ops[3].Value, nil, iter)
	t.Unbind(undo)
	return err
}

func evalConcat(t *Topdown, expr *ast.Expr, iter Iterator) error {
	ops := expr.Terms.([]*ast.Term)

	join, err := ValueToString(ops[1].Value, t)
	if err != nil {
		return errors.Wrapf(err, "%v: join value must be a string", ast.Concat.Name)
	}

	sl, err := ValueToStrings(ops[2].Value, t)
	if err != nil {
		return errors.Wrapf(err, "%v: input value must be array of strings", ast.Concat.Name)
	}

	s := ast.String(strings.Join(sl, join))

	undo, err := evalEqUnify(t, s, ops[3].Value, nil, iter)
	t.Unbind(undo)
	return err
}

func evalIndexOf(t *Topdown, expr *ast.Expr, iter Iterator) error {
	ops := expr.Terms.([]*ast.Term)

	base, err := ValueToString(ops[1].Value, t)
	if err != nil {
		return errors.Wrapf(err, "%v: base value must be a string", ast.IndexOf.Name)
	}

	search, err := ValueToString(ops[2].Value, t)
	if err != nil {
		return errors.Wrapf(err, "%v: search value must be a string", ast.IndexOf.Name)
	}

	index := ast.IntNumberTerm(strings.Index(base, search))

	undo, err := evalEqUnify(t, index.Value, ops[3].Value, nil, iter)
	t.Unbind(undo)
	return err
}

func evalSubstring(t *Topdown, expr *ast.Expr, iter Iterator) error {
	ops := expr.Terms.([]*ast.Term)

	base, err := ValueToString(ops[1].Value, t)
	if err != nil {
		return errors.Wrapf(err, "%v: base value must be a string", ast.Substring.Name)
	}

	startIndex, err := ValueToInt(ops[2].Value, t)
	if err != nil {
		return errors.Wrapf(err, "%v: start index must be a number", ast.Substring.Name)
	}

	l, err := ValueToInt(ops[3].Value, t)
	if err != nil {
		return errors.Wrapf(err, "%v: length must be a number", ast.Substring.Name)
	}

	var s ast.String
	if l < 0 {
		s = ast.String(base[startIndex:])
	} else {
		s = ast.String(base[startIndex : startIndex+l])
	}

	undo, err := evalEqUnify(t, s, ops[4].Value, nil, iter)
	t.Unbind(undo)
	return err
}

func evalContains(t *Topdown, expr *ast.Expr, iter Iterator) error {
	ops := expr.Terms.([]*ast.Term)

	base, err := ValueToString(ops[1].Value, t)
	if err != nil {
		return errors.Wrapf(err, "%v: base value must be a string", ast.Contains.Name)
	}

	search, err := ValueToString(ops[2].Value, t)
	if err != nil {
		return errors.Wrapf(err, "%v: search must be a string", ast.Contains.Name)
	}

	if strings.Contains(base, search) {
		return iter(t)
	}
	return nil
}

func evalStartsWith(t *Topdown, expr *ast.Expr, iter Iterator) error {
	ops := expr.Terms.([]*ast.Term)

	base, err := ValueToString(ops[1].Value, t)
	if err != nil {
		return errors.Wrapf(err, "%v: base value must be a string", ast.StartsWith.Name)
	}

	search, err := ValueToString(ops[2].Value, t)
	if err != nil {
		return errors.Wrapf(err, "%v: search must be a string", ast.StartsWith.Name)
	}

	if strings.HasPrefix(base, search) {
		return iter(t)
	}
	return nil
}

func evalEndsWith(t *Topdown, expr *ast.Expr, iter Iterator) error {
	ops := expr.Terms.([]*ast.Term)

	base, err := ValueToString(ops[1].Value, t)
	if err != nil {
		return errors.Wrapf(err, "%v: base value must be a string", ast.EndsWith.Name)
	}

	search, err := ValueToString(ops[2].Value, t)
	if err != nil {
		return errors.Wrapf(err, "%v: search must be a string", ast.EndsWith.Name)
	}

	if strings.HasSuffix(base, search) {
		return iter(t)
	}
	return nil
}

func evalLower(t *Topdown, expr *ast.Expr, iter Iterator) error {
	ops := expr.Terms.([]*ast.Term)

	orig, err := ValueToString(ops[1].Value, t)
	if err != nil {
		return errors.Wrapf(err, "%v: original value must be a string", ast.Lower.Name)
	}

	s := ast.String(strings.ToLower(orig))

	undo, err := evalEqUnify(t, s, ops[2].Value, nil, iter)
	t.Unbind(undo)
	return err
}

func evalUpper(t *Topdown, expr *ast.Expr, iter Iterator) error {
	ops := expr.Terms.([]*ast.Term)

	orig, err := ValueToString(ops[1].Value, t)
	if err != nil {
		return errors.Wrapf(err, "%v: original value must be a string", ast.Upper.Name)
	}

	s := ast.String(strings.ToUpper(orig))

	undo, err := evalEqUnify(t, s, ops[2].Value, nil, iter)
	t.Unbind(undo)
	return err
}
