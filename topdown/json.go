// Copyright 2019 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"strings"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/topdown/builtins"
)

func builtinJSONFilter(_ BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {

	// Ensure we have the right parameters, expect an object and a string or array/set of strings
	obj, err := builtins.ObjectOperand(operands[0].Value, 1)
	if err != nil {
		return err
	}

	// Build a list of filter strings
	var filters [][]ast.Value

	switch v := operands[1].Value.(type) {
	case ast.Array:
		for _, f := range v {
			filter, err := parsePath(f)
			if err != nil {
				return err
			}
			filters = append(filters, filter)
		}
	case ast.Set:
		err := v.Iter(func(f *ast.Term) error {
			filter, err := parsePath(f)
			if err != nil {
				return err
			}
			filters = append(filters, filter)
			return nil
		})
		if err != nil {
			return err
		}
	default:
		return builtins.NewOperandTypeErr(2, v, "set", "array")
	}

	// Actually do the filtering
	filterObj := pathsToObject(filters)
	r, err := obj.Filter(filterObj)
	if err != nil {
		return err
	}

	return iter(ast.NewTerm(r))
}

func parsePath(path *ast.Term) ([]ast.Value, error) {
	// paths can either be a `/` separated json path or
	// an array or set of values
	var pathSegments []ast.Value
	switch p := path.Value.(type) {
	case ast.String:
		parts := strings.Split(strings.Trim(string(p), "/"), "/")
		for _, part := range parts {
			pathSegments = append(pathSegments, ast.String(part))
		}
	case ast.Array:
		for _, term := range p {
			pathSegments = append(pathSegments, term.Value)
		}
	default:
		return nil, builtins.NewOperandErr(2, "expected set or array containing string paths or list of path segments")
	}

	return pathSegments, nil
}

func pathsToObject(paths [][]ast.Value) ast.Object {

	root := ast.NewObject()

	for _, path := range paths {
		node := root
		var done bool

		for i := 0; i < len(path)-1 && !done; i++ {

			k := ast.NewTerm(path[i])
			child := node.Get(k)

			if child == nil {
				obj := ast.NewObject()
				node.Insert(k, ast.NewTerm(obj))
				node = obj
				continue
			}

			switch v := child.Value.(type) {
			case ast.Null:
				done = true
			case ast.Object:
				node = v
			default:
				panic("unreachable")
			}
		}

		if !done {
			node.Insert(ast.NewTerm(path[len(path)-1]), ast.NullTerm())
		}
	}

	return root
}

func init() {
	RegisterBuiltinFunc(ast.JSONFilter.Name, builtinJSONFilter)
}
