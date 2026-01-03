// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/topdown/builtins"
)

// Deprecated: deprecated in v0.4.2 in favour of minus/infix "-" operation.
func builtinSetDiff(_ BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {

	s1, err := builtins.SetOperand(operands[0].Value, 1)
	if err != nil {
		return err
	}

	s2, err := builtins.SetOperand(operands[1].Value, 2)
	if err != nil {
		return err
	}

	return iter(ast.NewTerm(s1.Diff(s2)))
}

// builtinSetIntersection returns the intersection of the given input sets
func builtinSetIntersection(_ BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {

	inputSet, err := builtins.SetOperand(operands[0].Value, 1)
	if err != nil {
		return err
	}

	// empty input set
	if inputSet.Len() == 0 {
		return iter(ast.InternedEmptySet)
	}

	var result ast.Set

	err = inputSet.Iter(func(x *ast.Term) error {
		n, err := builtins.SetOperand(x.Value, 1)
		if err != nil {
			return err
		}

		if result == nil {
			result = n
		} else {
			result = result.Intersect(n)
		}
		return nil
	})
	if err != nil {
		return err
	}
	return iter(ast.NewTerm(result))
}

// builtinSetUnion returns the union of the given input sets
func builtinSetUnion(_ BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	// The set union logic here is manually inlined on purpose. By lifting
	// this logic up a level and not doing pairwise set unions, we avoid
	// many heap allocations. We also pre-allocate the result set by first
	// counting total elements across all input sets.
	inputSet, err := builtins.SetOperand(operands[0].Value, 1)
	if err != nil {
		return err
	}

	// First pass: count total elements for pre-allocation
	totalSize := 0
	err = inputSet.Iter(func(x *ast.Term) error {
		item, err := builtins.SetOperand(x.Value, 1)
		if err != nil {
			return err
		}
		totalSize += item.Len()
		return nil
	})
	if err != nil {
		return err
	}

	// Pre-allocate result set with estimated capacity
	result := ast.NewSetWithCapacity(totalSize)

	err = inputSet.Iter(func(x *ast.Term) error {
		item, _ := builtins.SetOperand(x.Value, 1) // error checked above
		item.Foreach(result.Add)
		return nil
	})
	if err != nil {
		return err
	}

	return iter(ast.NewTerm(result))
}

func init() {
	RegisterBuiltinFunc(ast.SetDiff.Name, builtinSetDiff)
	RegisterBuiltinFunc(ast.Intersection.Name, builtinSetIntersection)
	RegisterBuiltinFunc(ast.Union.Name, builtinSetUnion)
}
