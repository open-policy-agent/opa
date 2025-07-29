package rules

import (
	"github.com/vektah/gqlparser/v2/ast"

	//nolint:staticcheck // Validator rules each use dot imports for convenience.
	. "github.com/vektah/gqlparser/v2/validator/core"
)

var UniqueOperationNamesRule = Rule{
	Name: "UniqueOperationNames",
	RuleFunc: func(observers *Events, addError AddErrFunc) {
		seen := map[string]bool{}

		observers.OnOperation(func(walker *Walker, operation *ast.OperationDefinition) {
			if seen[operation.Name] {
				addError(
					Message(`There can be only one operation named "%s".`, operation.Name),
					At(operation.Position),
				)
			}
			seen[operation.Name] = true
		})
	},
}
