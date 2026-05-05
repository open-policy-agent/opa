package topdown

import "github.com/open-policy-agent/opa/v1/ast"

// EvaluatedRuleTracker records rule identifiers from annotations during
// evaluation. It extracts the ID field from each successfully evaluated
// rule's annotations.
type EvaluatedRuleTracker struct {
	IDs []string
}

func (t *EvaluatedRuleTracker) Record(rule *ast.Rule) {
	if t == nil || rule == nil {
		return
	}

	for _, a := range rule.Annotations {
		if a.ID != "" {
			t.IDs = append(t.IDs, a.ID)
			return
		}
	}
}
