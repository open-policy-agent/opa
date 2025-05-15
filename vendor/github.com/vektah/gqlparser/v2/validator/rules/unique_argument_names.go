package rules

import (
	"github.com/vektah/gqlparser/v2/ast"

	//nolint:staticcheck // Validator rules each use dot imports for convenience.
	. "github.com/vektah/gqlparser/v2/validator"
)

var UniqueArgumentNamesRule = Rule{
	Name: "UniqueArgumentNames",
	RuleFunc: func(observers *Events, addError AddErrFunc) {
		observers.OnField(func(walker *Walker, field *ast.Field) {
			checkUniqueArgs(field.Arguments, addError)
		})

		observers.OnDirective(func(walker *Walker, directive *ast.Directive) {
			checkUniqueArgs(directive.Arguments, addError)
		})
	},
}

func init() {
	AddRule(UniqueArgumentNamesRule.Name, UniqueArgumentNamesRule.RuleFunc)
}

func checkUniqueArgs(args ast.ArgumentList, addError AddErrFunc) {
	knownArgNames := map[string]int{}

	for _, arg := range args {
		if knownArgNames[arg.Name] == 1 {
			addError(
				Message(`There can be only one argument named "%s".`, arg.Name),
				At(arg.Position),
			)
		}

		knownArgNames[arg.Name]++
	}
}
