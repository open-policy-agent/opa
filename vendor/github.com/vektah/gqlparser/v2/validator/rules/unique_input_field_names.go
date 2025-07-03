package rules

import (
	"github.com/vektah/gqlparser/v2/ast"

	//nolint:staticcheck // Validator rules each use dot imports for convenience.
	. "github.com/vektah/gqlparser/v2/validator/core"
)

var UniqueInputFieldNamesRule = Rule{
	Name: "UniqueInputFieldNames",
	RuleFunc: func(observers *Events, addError AddErrFunc) {
		observers.OnValue(func(walker *Walker, value *ast.Value) {
			if value.Kind != ast.ObjectValue {
				return
			}

			seen := map[string]bool{}
			for _, field := range value.Children {
				if seen[field.Name] {
					addError(
						Message(`There can be only one input field named "%s".`, field.Name),
						At(field.Position),
					)
				}
				seen[field.Name] = true
			}
		})
	},
}
