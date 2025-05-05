package rules

import (
	"github.com/vektah/gqlparser/v2/ast"

	//nolint:staticcheck // Validator rules each use dot imports for convenience.
	. "github.com/vektah/gqlparser/v2/validator"
)

const maxListsDepth = 3

var MaxIntrospectionDepth = Rule{
	Name: "MaxIntrospectionDepth",
	RuleFunc: func(observers *Events, addError AddErrFunc) {
		// Counts the depth of list fields in "__Type" recursively and
		// returns `true` if the limit has been reached.
		observers.OnField(func(walker *Walker, field *ast.Field) {
			if field.Name == "__schema" || field.Name == "__type" {
				visitedFragments := make(map[string]bool)
				if checkDepthField(field, visitedFragments, 0) {
					addError(
						Message(`Maximum introspection depth exceeded`),
						At(field.Position),
					)
				}
				return
			}
		})
	},
}

func checkDepthSelectionSet(selectionSet ast.SelectionSet, visitedFragments map[string]bool, depth int) bool {
	for _, child := range selectionSet {
		if field, ok := child.(*ast.Field); ok {
			if checkDepthField(field, visitedFragments, depth) {
				return true
			}
		}
		if fragmentSpread, ok := child.(*ast.FragmentSpread); ok {
			if checkDepthFragmentSpread(fragmentSpread, visitedFragments, depth) {
				return true
			}
		}
		if inlineFragment, ok := child.(*ast.InlineFragment); ok {
			if checkDepthSelectionSet(inlineFragment.SelectionSet, visitedFragments, depth) {
				return true
			}
		}
	}
	return false
}

func checkDepthField(field *ast.Field, visitedFragments map[string]bool, depth int) bool {
	if field.Name == "fields" ||
		field.Name == "interfaces" ||
		field.Name == "possibleTypes" ||
		field.Name == "inputFields" {
		depth++
		if depth >= maxListsDepth {
			return true
		}
	}
	return checkDepthSelectionSet(field.SelectionSet, visitedFragments, depth)
}

func checkDepthFragmentSpread(fragmentSpread *ast.FragmentSpread, visitedFragments map[string]bool, depth int) bool {
	fragmentName := fragmentSpread.Name
	if visited, ok := visitedFragments[fragmentName]; ok && visited {
		// Fragment cycles are handled by `NoFragmentCyclesRule`.
		return false
	}
	fragment := fragmentSpread.Definition
	if fragment == nil {
		// Missing fragments checks are handled by `KnownFragmentNamesRule`.
		return false
	}

	// Rather than following an immutable programming pattern which has
	// significant memory and garbage collection overhead, we've opted to
	// take a mutable approach for efficiency's sake. Importantly visiting a
	// fragment twice is fine, so long as you don't do one visit inside the
	// other.
	visitedFragments[fragmentName] = true
	defer delete(visitedFragments, fragmentName)
	return checkDepthSelectionSet(fragment.SelectionSet, visitedFragments, depth)
}

func init() {
	AddRule(MaxIntrospectionDepth.Name, MaxIntrospectionDepth.RuleFunc)
}
