package rules

import (
	"fmt"

	"github.com/vektah/gqlparser/v2/ast"

	//nolint:staticcheck // Validator rules each use dot imports for convenience.
	. "github.com/vektah/gqlparser/v2/validator/core"
)

var FragmentsOnCompositeTypesRule = Rule{
	Name: "FragmentsOnCompositeTypes",
	RuleFunc: func(observers *Events, addError AddErrFunc) {
		observers.OnInlineFragment(func(walker *Walker, inlineFragment *ast.InlineFragment) {
			fragmentType := walker.Schema.Types[inlineFragment.TypeCondition]
			if fragmentType == nil || fragmentType.IsCompositeType() {
				return
			}

			message := fmt.Sprintf(`Fragment cannot condition on non composite type "%s".`, inlineFragment.TypeCondition)

			addError(
				Message("%s", message),
				At(inlineFragment.Position),
			)
		})

		observers.OnFragment(func(walker *Walker, fragment *ast.FragmentDefinition) {
			if fragment.Definition == nil || fragment.TypeCondition == "" || fragment.Definition.IsCompositeType() {
				return
			}

			message := fmt.Sprintf(`Fragment "%s" cannot condition on non composite type "%s".`, fragment.Name, fragment.TypeCondition)

			addError(
				Message("%s", message),
				At(fragment.Position),
			)
		})
	},
}
