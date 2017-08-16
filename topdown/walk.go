// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import "github.com/open-policy-agent/opa/ast"

func evalWalk(t *Topdown, expr *ast.Expr, iter Iterator) error {

	a, err := ResolveRefs(expr.Operand(0).Value, t)
	if err != nil {
		return err
	}

	b := expr.Operand(1)

	return walkRec(t, b.Value, a, ast.Array{}, iter)
}

func walkRec(t *Topdown, output ast.Value, v ast.Value, path ast.Array, iter Iterator) error {

	if err := unifyAndContinue(t, iter, ast.Array{ast.NewTerm(path), ast.NewTerm(v)}, output); err != nil {
		return err
	}

	if ast.IsScalar(v) {
		return nil
	}

	switch v := v.(type) {
	case ast.Array:
		for i := range v {
			path = append(path, ast.IntNumberTerm(i))
			if err := walkRec(t, output, v[i].Value, path, iter); err != nil {
				return err
			}
			path = path[:len(path)-1]
		}
	case ast.Object:
		for _, p := range v {
			path = append(path, p[0])
			if err := walkRec(t, output, p[1].Value, path, iter); err != nil {
				return err
			}
			path = path[:len(path)-1]
		}
	case *ast.Set:
		var err error
		v.Iter(func(e *ast.Term) bool {
			path = append(path, e)
			if err = walkRec(t, output, e.Value, path, iter); err != nil {
				return true
			}
			path = path[:len(path)-1]
			return false
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func init() {
	RegisterBuiltinFunc(ast.WalkBuiltin.Name, evalWalk)
}
