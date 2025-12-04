package find

import (
	"github.com/open-policy-agent/opa/v1/ast"
)

// Locator wraps functionality for finding specific AST contructs
type Locator interface {
	// Find searches a stack of containing nodes for a definition
	// Returns nil if this locator is not applicable to the given stack
	// compiler and parsed are optional - some locators do not need them
	Find(stack []ast.Node, compiler *ast.Compiler, parsed *ast.Module) *ast.Location

	// Name returns the name of this handler, for debugging
	Name() string
}
