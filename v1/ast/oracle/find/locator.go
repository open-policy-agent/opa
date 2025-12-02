package find

import (
	"github.com/open-policy-agent/opa/v1/ast"
)

// Locator wraps functionality for finding specific AST contructs
type Locator interface {
	// Applicable returns true if this handler can process the given stack
	Applicable(stack []ast.Node) bool

	// Find searches a stack of containing nodes for a definition
	// compiler and parsed are optional - some locators do not need them
	Find(stack []ast.Node, compiler *ast.Compiler, parsed *ast.Module) *ast.Location

	// Name returns the name of this handler, for debugging
	Name() string
}
