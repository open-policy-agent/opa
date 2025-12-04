package find

import (
	"github.com/open-policy-agent/opa/v1/ast"
)

type VarLocator struct{}

func NewVarLocator() *VarLocator {
	return &VarLocator{}
}

func (*VarLocator) Name() string {
	return "variables"
}

func (*VarLocator) Applicable(stack []ast.Node) bool {
	top, ok := stack[len(stack)-1].(*ast.Term)
	if !ok {
		return false
	}

	_, ok = top.Value.(ast.Var)
	return ok
}

func (v *VarLocator) Find(stack []ast.Node, _ *ast.Compiler, _ *ast.Module) *ast.Location {
	top, ok := stack[len(stack)-1].(*ast.Term)
	if !ok {
		return nil
	}

	name, ok := top.Value.(ast.Var)
	if !ok {
		return nil
	}

	return v.FindVarDefinition(stack, name)
}

// FindVarDefinition searches for the definition of a variable in the AST stack.
// It prioritizes explicit assignments but also falls back to finding implicit definitions
// (first occurrences in unification contexts) to handle where variables
// are introduced without explicit assignment.
func (v *VarLocator) FindVarDefinition(stack []ast.Node, name ast.Var) *ast.Location {
	for i := len(stack) - 1; i >= 0; i-- {
		if rule, ok := stack[i].(*ast.Rule); ok {
			if match := v.walkToFirstOccurrence(rule.Head.Args, name); match != nil {
				return match.Location
			}
		}
	}

	for i := len(stack) - 1; i >= 0; i-- {
		if body, ok := stack[i].(ast.Body); ok {
			if match := v.walkToFirstOccurrence(body, name); match != nil {
				return match.Location
			}
		}
	}

	return nil
}

func (*VarLocator) walkToFirstOccurrence(node ast.Node, needle ast.Var) (match *ast.Term) {
	var firstOccurrence *ast.Term

	ast.WalkNodes(node, func(x ast.Node) bool {
		if match == nil {
			switch x := x.(type) {
			case *ast.SomeDecl:
				// NOTE(tsandall): The visitor doesn't traverse into some decl terms
				// so special case here.
				for i := range x.Symbols {
					if x.Symbols[i].Value.Compare(needle) == 0 {
						match = x.Symbols[i]
						return true // found definition, stop searching
					}
				}
			case *ast.Expr:
				if x.IsAssignment() {
					if term := x.Operand(0); term != nil {
						if term.Value.Compare(needle) == 0 {
							match = term

							return true
						}
					}
				}
			case *ast.Term:
				// implicit definitions look like occurrences rather than assignments.
				// track these to return in case we find no assignments.
				if firstOccurrence == nil && x.Value.Compare(needle) == 0 {
					firstOccurrence = x
				}
			}
		}

		return match != nil
	})

	// if a definition was found, that is preferred
	if match != nil {
		return match
	}

	return firstOccurrence
}
