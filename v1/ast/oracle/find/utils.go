package find

import (
	"github.com/open-policy-agent/opa/v1/ast"
)

// findRulesDefinition looks up rules for a given ref. Rules appear in various
// other scenarios and this shares the rule look up logic.
func findRulesDefinition(compiler *ast.Compiler, ref ast.Ref) *ast.Location {
	if rules := compiler.GetRules(ref); len(rules) > 0 {
		return rules[0].Location
	}

	return nil
}
