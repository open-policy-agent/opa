package rules

import (
	"github.com/vektah/gqlparser/v2/ast"

	//nolint:staticcheck // Validator rules each use dot imports for convenience.
	. "github.com/vektah/gqlparser/v2/validator"
)

var NoUndefinedVariablesRule = Rule{
	Name: "NoUndefinedVariables",
	RuleFunc: func(observers *Events, addError AddErrFunc) {
		observers.OnValue(func(walker *Walker, value *ast.Value) {
			if walker.CurrentOperation == nil || value.Kind != ast.Variable || value.VariableDefinition != nil {
				return
			}

			if walker.CurrentOperation.Name != "" {
				addError(
					Message(`Variable "%s" is not defined by operation "%s".`, value, walker.CurrentOperation.Name),
					At(value.Position),
				)
			} else {
				addError(
					Message(`Variable "%s" is not defined.`, value),
					At(value.Position),
				)
			}
		})
	},
}

func init() {
	AddRule(NoUndefinedVariablesRule.Name, NoUndefinedVariablesRule.RuleFunc)
}
