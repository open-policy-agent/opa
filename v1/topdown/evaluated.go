package topdown

import "github.com/open-policy-agent/opa/v1/ast"

// EvaluatedRuleTracker records rule identifiers from annotations during
// evaluation. It extracts the ID field from each successfully evaluated
// rule's annotations. Duplicate IDs are suppressed.
type EvaluatedRuleTracker struct {
	IDs  []string
	seen map[string]struct{}
}

func (t *EvaluatedRuleTracker) Record(rule *ast.Rule) {
	if t == nil || len(rule.Annotations) == 0 {
		return
	}

	for _, a := range rule.Annotations {
		if a.ID != "" {
			if t.seen == nil {
				t.seen = make(map[string]struct{})
			}
			if _, dup := t.seen[a.ID]; !dup {
				t.seen[a.ID] = struct{}{}
				t.IDs = append(t.IDs, a.ID)
			}
			return
		}
	}
}
