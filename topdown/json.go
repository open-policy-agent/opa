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
	var filters []ast.String

	switch v := operands[1].Value.(type) {
	case ast.Array:
		for _, f := range v {
			filterString, ok := f.Value.(ast.String)
			if !ok {
				return builtins.NewOperandErr(2, "Expected a set or array containing only strings")
			}
			filters = append(filters, filterString)
		}
	case ast.Set:
		err := v.Iter(func(f *ast.Term) error {
			filterString, ok := f.Value.(ast.String)
			if !ok {
				return builtins.NewOperandErr(2, "Expected a set or array containing only strings")
			}
			filters = append(filters, filterString)
			return nil
		})
		if err != nil {
			return err
		}
	default:
		return builtins.NewOperandTypeErr(2, v, "set", "array")
	}

	// Actually do the filtering
	filterObj := jsonPathsToObject(filters)
	r, err := obj.Filter(filterObj)
	if err != nil {
		return err
	}

	return iter(ast.NewTerm(r))
}

func jsonPathsToObject(paths []ast.String) ast.Object {
	result := ast.NewObject()
	for _, filter := range paths {
		s := strings.Split(strings.Trim(string(filter), "/"), "/")
		o := result

		for len(s) > 0 {
			key := ast.StringTerm(s[0])
			s = s[1:]
			hasKey := o.Get(key) != nil

			if !hasKey && len(s) > 0 {
				next := ast.NewObject()
				o.Insert(key, ast.NewTerm(next))
				o = next
			} else if !hasKey && len(s) == 0 {
				o.Insert(key, ast.NullTerm())
			} else if hasKey {
				t := o.Get(key)
				if t.Value.Compare(ast.Null{}) == 0 {
					break // done with path, another shorter path has already been here
				} else if len(s) == 0 {
					o.Insert(key, ast.NullTerm())
				} else {
					o = t.Value.(ast.Object)
				}
			}
		}
	}
	return result
}

func init() {
	RegisterBuiltinFunc(ast.JSONFilter.Name, builtinJSONFilter)
}
