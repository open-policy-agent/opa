package find

import (
	"github.com/open-policy-agent/opa/v1/ast"
)

type EveryLocator struct {
	varLocator *VarLocator
}

func NewEveryLocator() *EveryLocator {
	return &EveryLocator{
		varLocator: NewVarLocator(),
	}
}

func (*EveryLocator) Name() string {
	return "every_declarations"
}

func (*EveryLocator) Applicable(stack []ast.Node) bool {
	if expr, ok := stack[len(stack)-1].(*ast.Expr); ok {
		if _, ok := expr.Terms.(*ast.Every); ok {
			return true
		}
	}

	return false
}

func (e *EveryLocator) Find(stack []ast.Node, compiler *ast.Compiler, _ *ast.Module) *ast.Location {
	var every *ast.Every

	if expr, ok := stack[len(stack)-1].(*ast.Expr); ok {
		if e, ok := expr.Terms.(*ast.Every); ok {
			every = e
		}
	}

	if every == nil {
		return nil
	}

	switch v := every.Domain.Value.(type) {
	case ast.Var:
		return e.varLocator.FindVarDefinition(stack, v)
	case ast.Ref:
		return findRulesDefinition(compiler, v)
	}

	return nil
}
