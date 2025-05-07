package rules

import (
	"github.com/vektah/gqlparser/v2/ast"

	//nolint:staticcheck // Validator rules each use dot imports for convenience.
	. "github.com/vektah/gqlparser/v2/validator"
)

var VariablesInAllowedPositionRule = Rule{
	Name: "VariablesInAllowedPosition",
	RuleFunc: func(observers *Events, addError AddErrFunc) {
		observers.OnValue(func(walker *Walker, value *ast.Value) {
			if value.Kind != ast.Variable || value.ExpectedType == nil || value.VariableDefinition == nil || walker.CurrentOperation == nil {
				return
			}

			tmp := *value.ExpectedType

			// todo: move me into walk
			// If there is a default non nullable types can be null
			if value.VariableDefinition.DefaultValue != nil && value.VariableDefinition.DefaultValue.Kind != ast.NullValue {
				if value.ExpectedType.NonNull {
					tmp.NonNull = false
				}
			}

			if !value.VariableDefinition.Type.IsCompatible(&tmp) {
				addError(
					Message(
						`Variable "%s" of type "%s" used in position expecting type "%s".`,
						value,
						value.VariableDefinition.Type.String(),
						value.ExpectedType.String(),
					),
					At(value.Position),
				)
			}
		})
	},
}

func init() {
	AddRule(VariablesInAllowedPositionRule.Name, VariablesInAllowedPositionRule.RuleFunc)
}
