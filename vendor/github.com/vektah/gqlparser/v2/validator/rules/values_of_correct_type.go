package rules

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/vektah/gqlparser/v2/ast"

	//nolint:staticcheck // Validator rules each use dot imports for convenience.
	. "github.com/vektah/gqlparser/v2/validator"
)

func ruleFuncValuesOfCorrectType(observers *Events, addError AddErrFunc, disableSuggestion bool) {
	observers.OnValue(func(walker *Walker, value *ast.Value) {
		if value.Definition == nil || value.ExpectedType == nil {
			return
		}

		if value.Kind == ast.NullValue && value.ExpectedType.NonNull {
			addError(
				Message(`Expected value of type "%s", found %s.`, value.ExpectedType.String(), value.String()),
				At(value.Position),
			)
		}

		if value.Definition.Kind == ast.Scalar {
			// Skip custom validating scalars
			if !value.Definition.OneOf("Int", "Float", "String", "Boolean", "ID") {
				return
			}
		}

		var possibleEnums []string
		if value.Definition.Kind == ast.Enum {
			for _, val := range value.Definition.EnumValues {
				possibleEnums = append(possibleEnums, val.Name)
			}
		}

		rawVal, err := value.Value(nil)
		if err != nil {
			unexpectedTypeMessage(addError, value)
		}

		switch value.Kind {
		case ast.NullValue:
			return
		case ast.ListValue:
			if value.ExpectedType.Elem == nil {
				unexpectedTypeMessage(addError, value)
				return
			}

		case ast.IntValue:
			if !value.Definition.OneOf("Int", "Float", "ID") {
				unexpectedTypeMessage(addError, value)
			}

		case ast.FloatValue:
			if !value.Definition.OneOf("Float") {
				unexpectedTypeMessage(addError, value)
			}

		case ast.StringValue, ast.BlockValue:
			if value.Definition.Kind == ast.Enum {
				if disableSuggestion {
					addError(
						Message(`Enum "%s" cannot represent non-enum value: %s.`, value.ExpectedType.String(), value.String()),
						At(value.Position),
					)
				} else {
					rawValStr := fmt.Sprint(rawVal)
					addError(
						Message(`Enum "%s" cannot represent non-enum value: %s.`, value.ExpectedType.String(), value.String()),
						SuggestListQuoted("Did you mean the enum value", rawValStr, possibleEnums),
						At(value.Position),
					)
				}
			} else if !value.Definition.OneOf("String", "ID") {
				unexpectedTypeMessage(addError, value)
			}

		case ast.EnumValue:
			if value.Definition.Kind != ast.Enum {
				if disableSuggestion {
					addError(
						unexpectedTypeMessageOnly(value),
						At(value.Position),
					)
				} else {
					rawValStr := fmt.Sprint(rawVal)
					addError(
						unexpectedTypeMessageOnly(value),
						SuggestListUnquoted("Did you mean the enum value", rawValStr, possibleEnums),
						At(value.Position),
					)
				}
			} else if value.Definition.EnumValues.ForName(value.Raw) == nil {
				if disableSuggestion {
					addError(
						Message(`Value "%s" does not exist in "%s" enum.`, value.String(), value.ExpectedType.String()),
						At(value.Position),
					)
				} else {
					rawValStr := fmt.Sprint(rawVal)
					addError(
						Message(`Value "%s" does not exist in "%s" enum.`, value.String(), value.ExpectedType.String()),
						SuggestListQuoted("Did you mean the enum value", rawValStr, possibleEnums),
						At(value.Position),
					)
				}
			}

		case ast.BooleanValue:
			if !value.Definition.OneOf("Boolean") {
				unexpectedTypeMessage(addError, value)
			}

		case ast.ObjectValue:

			for _, field := range value.Definition.Fields {
				if field.Type.NonNull {
					fieldValue := value.Children.ForName(field.Name)
					if fieldValue == nil && field.DefaultValue == nil {
						addError(
							Message(`Field "%s.%s" of required type "%s" was not provided.`, value.Definition.Name, field.Name, field.Type.String()),
							At(value.Position),
						)
						continue
					}
				}
			}

			for _, directive := range value.Definition.Directives {
				if directive.Name == "oneOf" {
					func() {
						if len(value.Children) != 1 {
							addError(
								Message(`OneOf Input Object "%s" must specify exactly one key.`, value.Definition.Name),
								At(value.Position),
							)
							return
						}

						fieldValue := value.Children[0].Value
						isNullLiteral := fieldValue == nil || fieldValue.Kind == ast.NullValue
						if isNullLiteral {
							addError(
								Message(`Field "%s.%s" must be non-null.`, value.Definition.Name, value.Definition.Fields[0].Name),
								At(fieldValue.Position),
							)
							return
						}

						isVariable := fieldValue.Kind == ast.Variable
						if isVariable {
							variableName := fieldValue.VariableDefinition.Variable
							isNullableVariable := !fieldValue.VariableDefinition.Type.NonNull
							if isNullableVariable {
								addError(
									Message(`Variable "%s" must be non-nullable to be used for OneOf Input Object "%s".`, variableName, value.Definition.Name),
									At(fieldValue.Position),
								)
							}
						}
					}()
				}
			}

			for _, fieldValue := range value.Children {
				if value.Definition.Fields.ForName(fieldValue.Name) == nil {
					if disableSuggestion {
						addError(
							Message(`Field "%s" is not defined by type "%s".`, fieldValue.Name, value.Definition.Name),
							At(fieldValue.Position),
						)
					} else {
						var suggestions []string
						for _, fieldValue := range value.Definition.Fields {
							suggestions = append(suggestions, fieldValue.Name)
						}

						addError(
							Message(`Field "%s" is not defined by type "%s".`, fieldValue.Name, value.Definition.Name),
							SuggestListQuoted("Did you mean", fieldValue.Name, suggestions),
							At(fieldValue.Position),
						)
					}
				}
			}

		case ast.Variable:
			return

		default:
			panic(fmt.Errorf("unhandled %T", value))
		}
	})
}

var ValuesOfCorrectTypeRule = Rule{
	Name: "ValuesOfCorrectType",
	RuleFunc: func(observers *Events, addError AddErrFunc) {
		ruleFuncValuesOfCorrectType(observers, addError, false)
	},
}

var ValuesOfCorrectTypeRuleWithoutSuggestions = Rule{
	Name: "ValuesOfCorrectTypeWithoutSuggestions",
	RuleFunc: func(observers *Events, addError AddErrFunc) {
		ruleFuncValuesOfCorrectType(observers, addError, true)
	},
}

func init() {
	AddRule(ValuesOfCorrectTypeRule.Name, ValuesOfCorrectTypeRule.RuleFunc)
}

func unexpectedTypeMessage(addError AddErrFunc, v *ast.Value) {
	addError(
		unexpectedTypeMessageOnly(v),
		At(v.Position),
	)
}

func unexpectedTypeMessageOnly(v *ast.Value) ErrorOption {
	switch v.ExpectedType.String() {
	case "Int", "Int!":
		if _, err := strconv.ParseInt(v.Raw, 10, 32); err != nil && errors.Is(err, strconv.ErrRange) {
			return Message(`Int cannot represent non 32-bit signed integer value: %s`, v.String())
		}
		return Message(`Int cannot represent non-integer value: %s`, v.String())
	case "String", "String!", "[String]":
		return Message(`String cannot represent a non string value: %s`, v.String())
	case "Boolean", "Boolean!":
		return Message(`Boolean cannot represent a non boolean value: %s`, v.String())
	case "Float", "Float!":
		return Message(`Float cannot represent non numeric value: %s`, v.String())
	case "ID", "ID!":
		return Message(`ID cannot represent a non-string and non-integer value: %s`, v.String())
	// case "Enum":
	//		return Message(`Enum "%s" cannot represent non-enum value: %s`, v.ExpectedType.String(), v.String())
	default:
		if v.Definition.Kind == ast.Enum {
			return Message(`Enum "%s" cannot represent non-enum value: %s.`, v.ExpectedType.String(), v.String())
		}
		return Message(`Expected value of type "%s", found %s.`, v.ExpectedType.String(), v.String())
	}
}
