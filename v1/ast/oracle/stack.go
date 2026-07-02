// Copyright 2025 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.
package oracle

import (
	"github.com/open-policy-agent/opa/v1/ast"
)

func findContainingNodeStack(module *ast.Module, pos int) []ast.Node {
	var matches []ast.Node

	ast.WalkNodes(module, func(x ast.Node) bool {
		minLoc, maxLoc := getLocMinMax(x)

		// ast.Every nodes have no location but should still be traversed
		// to reach children
		if minLoc == -1 && maxLoc == -1 {
			if _, ok := x.(*ast.Every); ok {
				return false
			}

			// otherwise if missing, then skip
			return true
		}

		// skip if not contained
		if pos < minLoc || pos >= maxLoc {
			return true
		}

		matches = append(matches, x)

		// WalkNodes walks to SomeDecl.Symbols (which are Terms),
		// but doesn't descend into the Call values.
		if expr, ok := x.(*ast.Expr); ok {
			if someDecl, ok := expr.Terms.(*ast.SomeDecl); ok {
				for _, symbol := range someDecl.Symbols {
					ast.WalkTerms(symbol, func(t *ast.Term) bool {
						if t.Location != nil {
							minLoc := t.Location.Offset
							maxLoc := minLoc + len(t.Location.Text)
							if pos >= minLoc && pos < maxLoc {
								matches = append(matches, t)
							}
						}

						return false
					})
				}
			}
		}

		return false
	})

	return matches
}

func getLocMinMax(x ast.Node) (int, int) {
	if x.Loc() == nil {
		return -1, -1
	}

	loc := x.Loc()
	minOff := loc.Offset

	// Special case bodies because location text is only for the first expr.
	if body, ok := x.(ast.Body); ok {
		last := findLastExpr(body)
		if last == nil {
			// No non-generated expressions in body
			return -1, -1
		}
		extraLoc := last.Loc()
		if extraLoc == nil {
			return -1, -1
		}
		return minOff, extraLoc.Offset + len(extraLoc.Text)
	}

	// Special case Expr with Every terms because the Expr's location text
	// only covers "every k, v in domain" but we need to include the body
	if expr, ok := x.(*ast.Expr); ok {
		if every, ok := expr.Terms.(*ast.Every); ok {
			if len(every.Body) > 0 {
				last := findLastExpr(every.Body)
				if last != nil && last.Loc() != nil {
					extraLoc := last.Loc()

					return minOff, extraLoc.Offset + len(extraLoc.Text)
				}
			}
		}
	}

	return minOff, minOff + len(loc.Text)
}

// findLastExpr returns the last expression in an ast.Body that has not been generated
// by the compiler. It's used to cope with the fact that a compiler stage before SetRuleTree
// has rewritten the rule bodies slightly. By ignoring appended generated body expressions,
// we can still use the "circling in on the variable" logic based on node locations.
func findLastExpr(body ast.Body) *ast.Expr {
	for i := len(body) - 1; i >= 0; i-- {
		if !body[i].Generated {
			return body[i]
		}
	}
	// NOTE(sr): I believe this shouldn't happen -- we only ever start circling in on a node
	// inside a body if there's something in that body. A body that only consists of generated
	// expressions should not appear here. Either way, the caller deals with `nil` returned by
	// this helper.
	return nil
}
