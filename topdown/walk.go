// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"github.com/open-policy-agent/opa/ast"
)

func evalWalk(bctx BuiltinContext, args []*ast.Term, iter func(*ast.Term) error) error {
	input := args[0]
	filter := getOutputPath(args)
	var path ast.Array
	return walk(filter, path, input, iter)
}

func walk(filter, path ast.Array, input *ast.Term, iter func(*ast.Term) error) error {

	if filter.Len() == 0 {
		if err := iter(ast.ArrayTerm(ast.NewTerm(path), input)); err != nil {
			return err
		}
	}

	if filter.Len() > 0 {
		key := filter.Elem(0)
		filter = filter.Slice(1, -1)
		if key.IsGround() {
			if term := input.Get(key); term != nil {
				path = path.Append(key)
				return walk(filter, path, term, iter)
			}
			return nil
		}
	}

	switch v := input.Value.(type) {
	case ast.Array:
		for i := 0; i < v.Len(); i++ {
			path = path.Append(ast.IntNumberTerm(i))
			if err := walk(filter, path, v.Elem(i), iter); err != nil {
				return err
			}
			path = path.Slice(0, path.Len()-1)
		}
	case ast.Object:
		return v.Iter(func(k, v *ast.Term) error {
			path = path.Append(k)
			if err := walk(filter, path, v, iter); err != nil {
				return err
			}
			path = path.Slice(0, path.Len()-1)
			return nil
		})
	case ast.Set:
		return v.Iter(func(elem *ast.Term) error {
			path = path.Append(elem)
			if err := walk(filter, path, elem, iter); err != nil {
				return err
			}
			path = path.Slice(0, path.Len()-1)
			return nil
		})
	}

	return nil
}

func getOutputPath(args []*ast.Term) ast.Array {
	if len(args) == 2 {
		if arr, ok := args[1].Value.(ast.Array); ok {
			if arr.Len() == 2 {
				if path, ok := arr.Elem(0).Value.(ast.Array); ok {
					return path
				}
			}
		}
	}
	return ast.NewArray()
}

func init() {
	RegisterBuiltinFunc(ast.WalkBuiltin.Name, evalWalk)
}
