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

	return v.FindVarOccurrence(stack, name)
}

func (v *VarLocator) FindVarOccurrence(stack []ast.Node, name ast.Var) *ast.Location {
	for i := range stack {
		switch node := stack[i].(type) {
		case *ast.Rule:
			if match := v.walkToFirstOccurrence(node.Head.Args, name); match != nil {
				return match.Location
			}
		case ast.Body:
			if match := v.walkToFirstOccurrence(node, name); match != nil {
				return match.Location
			}
		}
	}

	return nil
}

func (*VarLocator) walkToFirstOccurrence(node ast.Node, needle ast.Var) (match *ast.Term) {
	ast.WalkNodes(node, func(x ast.Node) bool {
		if match == nil {
			switch x := x.(type) {
			case *ast.SomeDecl:
				// NOTE(tsandall): The visitor doesn't traverse into some decl terms
				// so special case here.
				for i := range x.Symbols {
					if x.Symbols[i].Value.Compare(needle) == 0 {
						match = x.Symbols[i]
						break
					}
				}
			case *ast.Term:
				if x.Value.Compare(needle) == 0 {
					match = x
				}
			}
		}
		return match != nil
	})
	return match
}
