package oracle

import (
	"errors"

	"github.com/open-policy-agent/opa/v1/ast"
)

func findContainingNodeStack(module *ast.Module, pos int) []ast.Node {
	var matches []ast.Node

	ast.WalkNodes(module, func(x ast.Node) bool {
		minLoc, maxLoc := getLocMinMax(x)

		if pos < minLoc || pos >= maxLoc {
			return true
		}

		matches = append(matches, x)
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
		extraLoc := last.Loc()
		if extraLoc == nil {
			return -1, -1
		}
		return minOff, extraLoc.Offset + len(extraLoc.Text)
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

func halted(c *ast.Compiler) error {
	if c.Failed() && len(c.Errors) == 1 && c.Errors[0].Code == "halt" {
		return nil
	} else if len(c.Errors) > 0 {
		return c.Errors
	}
	// NOTE(tsandall): this indicate an internal error in the compiler and should
	// not be reachable.
	return errors.New("unreachable: did not halt")
}
