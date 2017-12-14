// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import "github.com/open-policy-agent/opa/ast"

func evalWalk(bctx BuiltinContext, args []*ast.Term, iter func(*ast.Term) error) error {
	input := args[0]
	var path ast.Array
	return walk(input, path, iter)
}

func walk(input *ast.Term, path ast.Array, iter func(*ast.Term) error) error {

	output := ast.ArrayTerm(ast.NewTerm(path), input)

	if err := iter(output); err != nil {
		return err
	}

	switch v := input.Value.(type) {
	case ast.Array:
		for i := range v {
			path = append(path, ast.IntNumberTerm(i))
			if err := walk(v[i], path, iter); err != nil {
				return err
			}
			path = path[:len(path)-1]
		}
	case ast.Object:
		return v.Iter(func(k, v *ast.Term) error {
			path = append(path, k)
			if err := walk(v, path, iter); err != nil {
				return err
			}
			path = path[:len(path)-1]
			return nil
		})
	case ast.Set:
		return v.Iter(func(elem *ast.Term) error {
			path = append(path, elem)
			if err := walk(elem, path, iter); err != nil {
				return err
			}
			path = path[:len(path)-1]
			return nil
		})
	}

	return nil
}

func init() {
	RegisterBuiltinFunc(ast.WalkBuiltin.Name, evalWalk)
}
