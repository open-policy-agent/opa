package find

import (
	"github.com/open-policy-agent/opa/v1/ast"
)

type RefLocator struct{}

func NewRefLocator() *RefLocator {
	return &RefLocator{}
}

func (*RefLocator) Name() string {
	return "references"
}

func (*RefLocator) Applicable(stack []ast.Node) bool {
	for i := len(stack) - 1; i >= 0; i-- {
		term, ok := stack[i].(*ast.Term)
		if !ok {
			continue
		}
		if _, ok := term.Value.(ast.Ref); ok {
			return true
		}
	}
	return false
}

func (*RefLocator) Find(stack []ast.Node, compiler *ast.Compiler, parsed *ast.Module) *ast.Location {
	for i := len(stack) - 1; i >= 0; i-- {
		term, ok := stack[i].(*ast.Term)
		if !ok {
			continue
		}

		ref, ok := term.Value.(ast.Ref)
		if !ok {
			continue
		}

		if rulesResult := findRulesDefinition(compiler, ref); rulesResult != nil {
			return rulesResult
		}

		prefix := ref.ConstantPrefix()

		for _, imp := range parsed.Imports {
			path, ok := imp.Path.Value.(ast.Ref)
			if !ok {
				continue
			}
			if prefix.HasPrefix(path) {
				return imp.Path.Location
			}
		}
	}

	return nil
}
