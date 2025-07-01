package rules

import (
	"github.com/vektah/gqlparser/v2/ast"

	//nolint:staticcheck // Validator rules each use dot imports for convenience.
	. "github.com/vektah/gqlparser/v2/validator/core"
)

var UniqueVariableNamesRule = Rule{
	Name: "UniqueVariableNames",
	RuleFunc: func(observers *Events, addError AddErrFunc) {
		observers.OnOperation(func(walker *Walker, operation *ast.OperationDefinition) {
			seen := map[string]int{}
			for _, def := range operation.VariableDefinitions {
				// add the same error only once per a variable.
				if seen[def.Variable] == 1 {
					addError(
						Message(`There can be only one variable named "$%s".`, def.Variable),
						At(def.Position),
					)
				}
				seen[def.Variable]++
			}
		})
	},
}
