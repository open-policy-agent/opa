// Copyright 2025 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.
package oracle

import (
	"github.com/open-policy-agent/opa/v1/ast"
)

// findDefinition searches a stack of containing nodes for a definition of the
// given target.
// It handles both variables and references and returns nil if no definition is
// found.
func findDefinition(
	t *target,
	stack []ast.Node,
	compiler *ast.Compiler,
	parsed *ast.Module,
) *ast.Location {
	// we only know how to look for vars and refs.
	if !t.isRef && !t.isVar {
		return nil
	}

	matchFn := createMatcher(t)

	// Walk down the stack from innermost to outermost, collecting matches.
	// The matchResult tracks the best match found (ref or variable).
	result := &matchResult{}
	for i := len(stack) - 1; i >= 0; i-- {
		matchFn(stack[i], compiler, parsed, result)
		// exit if we found a ref (refs always win)
		if !result.isVar && result.loc != nil {
			break
		}
	}

	return result.loc
}
