package find

import (
	"github.com/open-policy-agent/opa/v1/ast"
)

// Manager coordinates a collection of definition locators.
// Locators are tried in registration order (first registered = highest precedence).
type Manager struct {
	locators []Locator
}

func NewManager() *Manager {
	return &Manager{
		locators: make([]Locator, 0),
	}
}

func (m *Manager) Register(locator Locator) {
	m.locators = append(m.locators, locator)
}

func (m *Manager) FindDefinition(stack []ast.Node, compiler *ast.Compiler, parsed *ast.Module) *ast.Location {
	if len(stack) == 0 {
		return nil
	}

	for _, locator := range m.locators {
		if !locator.Applicable(stack) {
			continue
		}

		location := locator.Find(stack, compiler, parsed)
		if location != nil {
			return location
		}
	}

	return nil
}
