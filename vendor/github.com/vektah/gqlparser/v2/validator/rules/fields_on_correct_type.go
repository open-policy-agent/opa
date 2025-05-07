package rules

import (
	"fmt"
	"sort"
	"strings"

	"github.com/vektah/gqlparser/v2/ast"

	//nolint:staticcheck // Validator rules each use dot imports for convenience.
	. "github.com/vektah/gqlparser/v2/validator"
)

func ruleFuncFieldsOnCorrectType(observers *Events, addError AddErrFunc, disableSuggestion bool) {
	observers.OnField(func(walker *Walker, field *ast.Field) {
		if field.ObjectDefinition == nil || field.Definition != nil {
			return
		}

		message := fmt.Sprintf(`Cannot query field "%s" on type "%s".`, field.Name, field.ObjectDefinition.Name)

		if !disableSuggestion {
			if suggestedTypeNames := getSuggestedTypeNames(walker, field.ObjectDefinition, field.Name); suggestedTypeNames != nil {
				message += " Did you mean to use an inline fragment on " + QuotedOrList(suggestedTypeNames...) + "?"
			} else if suggestedFieldNames := getSuggestedFieldNames(field.ObjectDefinition, field.Name); suggestedFieldNames != nil {
				message += " Did you mean " + QuotedOrList(suggestedFieldNames...) + "?"
			}
		}

		addError(
			Message("%s", message),
			At(field.Position),
		)
	})
}

var FieldsOnCorrectTypeRule = Rule{
	Name: "FieldsOnCorrectType",
	RuleFunc: func(observers *Events, addError AddErrFunc) {
		ruleFuncFieldsOnCorrectType(observers, addError, false)
	},
}

var FieldsOnCorrectTypeRuleWithoutSuggestions = Rule{
	Name: "FieldsOnCorrectTypeWithoutSuggestions",
	RuleFunc: func(observers *Events, addError AddErrFunc) {
		ruleFuncFieldsOnCorrectType(observers, addError, true)
	},
}

func init() {
	AddRule(FieldsOnCorrectTypeRule.Name, FieldsOnCorrectTypeRule.RuleFunc)
}

// Go through all the implementations of type, as well as the interfaces
// that they implement. If any of those types include the provided field,
// suggest them, sorted by how often the type is referenced,  starting
// with Interfaces.
func getSuggestedTypeNames(walker *Walker, parent *ast.Definition, name string) []string {
	if !parent.IsAbstractType() {
		return nil
	}

	possibleTypes := walker.Schema.GetPossibleTypes(parent)
	suggestedObjectTypes := make([]string, 0, len(possibleTypes))
	var suggestedInterfaceTypes []string
	interfaceUsageCount := map[string]int{}

	for _, possibleType := range possibleTypes {
		field := possibleType.Fields.ForName(name)
		if field == nil {
			continue
		}

		suggestedObjectTypes = append(suggestedObjectTypes, possibleType.Name)

		for _, possibleInterface := range possibleType.Interfaces {
			interfaceField := walker.Schema.Types[possibleInterface]
			if interfaceField != nil && interfaceField.Fields.ForName(name) != nil {
				if interfaceUsageCount[possibleInterface] == 0 {
					suggestedInterfaceTypes = append(suggestedInterfaceTypes, possibleInterface)
				}
				interfaceUsageCount[possibleInterface]++
			}
		}
	}

	suggestedTypes := concatSlice(suggestedInterfaceTypes, suggestedObjectTypes)

	sort.SliceStable(suggestedTypes, func(i, j int) bool {
		typeA, typeB := suggestedTypes[i], suggestedTypes[j]
		diff := interfaceUsageCount[typeB] - interfaceUsageCount[typeA]
		if diff != 0 {
			return diff < 0
		}
		return strings.Compare(typeA, typeB) < 0
	})

	return suggestedTypes
}

// By employing a full slice expression (slice[low:high:max]),
// where max is set to the slice’s length,
// we ensure that appending elements results
// in a slice backed by a distinct array.
// This method prevents the shared array issue
func concatSlice(first []string, second []string) []string {
	n := len(first)
	return append(first[:n:n], second...)
}

// For the field name provided, determine if there are any similar field names
// that may be the result of a typo.
func getSuggestedFieldNames(parent *ast.Definition, name string) []string {
	if parent.Kind != ast.Object && parent.Kind != ast.Interface {
		return nil
	}

	possibleFieldNames := make([]string, 0, len(parent.Fields))
	for _, field := range parent.Fields {
		possibleFieldNames = append(possibleFieldNames, field.Name)
	}

	return SuggestionList(name, possibleFieldNames)
}
