package find

import (
	"github.com/open-policy-agent/opa/v1/ast"
)

type SomeLocator struct {
	VarLocator
}

func NewSomeLocator() *SomeLocator {
	return &SomeLocator{}
}

func (*SomeLocator) Name() string {
	return "some_declarations"
}

func (*SomeLocator) Applicable(stack []ast.Node) bool {
	if expr, ok := stack[len(stack)-1].(*ast.Expr); ok {
		if _, ok := expr.Terms.(*ast.SomeDecl); ok {
			return true
		}
	}

	if _, ok := stack[len(stack)-1].(*ast.SomeDecl); ok {
		return true
	}

	return false
}

func (s *SomeLocator) Find(stack []ast.Node, compiler *ast.Compiler, _ *ast.Module) *ast.Location {
	var someDecl *ast.SomeDecl

	switch node := stack[len(stack)-1].(type) {
	case *ast.Expr:
		if sd, ok := node.Terms.(*ast.SomeDecl); ok {
			someDecl = sd
		}
	case *ast.SomeDecl:
		someDecl = node
	}

	if someDecl == nil {
		return nil
	}

	term := someDecl.Symbols[0]

	call, ok := term.Value.(ast.Call)
	if !ok || len(call) == 0 {
		return nil
	}

	switch v := call[len(call)-1].Value.(type) {
	case ast.Var:
		return s.FindVarDefinition(stack, v)
	case ast.Ref:
		return findRulesDefinition(compiler, v)
	}

	return nil
}
