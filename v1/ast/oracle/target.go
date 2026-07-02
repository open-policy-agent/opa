// Copyright 2025 The OPA Authors. All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.
package oracle

import (
	"github.com/open-policy-agent/opa/v1/ast"
)

// target represents the target term to find a definition for.
type target struct {
	term  *ast.Term
	isRef bool
	isVar bool
}

// findTarget searches a stack of nodes for the target term to find a definition for.
// It walks down the stack from the top to find the first Term that is a Ref or Var.
// Returns nil if no suitable target is found.
func findTarget(stack []ast.Node) *target {
	if len(stack) == 0 {
		return nil
	}

	var targetTerm *ast.Term
	var targetIsRef bool
	var targetIsVar bool

	// First, check if the very top node is a Var - this handles cases like
	// function arguments in dynamic refs: input.foo[x] where x is the target
	if top, ok := stack[len(stack)-1].(*ast.Term); ok {
		if _, ok := top.Value.(ast.Var); ok {
			targetTerm = top
			targetIsVar = true
		}
	}

	// If top wasn't a var, walk up the stack looking for a Ref.
	if targetTerm == nil {
		for i := len(stack) - 1; i >= 0; i-- {
			if term, ok := stack[i].(*ast.Term); ok {
				switch term.Value.(type) {
				case ast.Ref:
					// Found a ref - this is our target
					targetTerm = term
					targetIsRef = true
				case ast.Var:
					// Found a var - might be standalone or part of a ref
					// Keep as fallback if no ref is found
					if targetTerm == nil {
						targetTerm = term
						targetIsVar = true
					}
				}

				if targetIsRef {
					break
				}
			}
		}
	}

	if targetTerm == nil {
		return nil
	}

	return &target{
		term:  targetTerm,
		isRef: targetIsRef,
		isVar: targetIsVar,
	}
}
