// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import "github.com/open-policy-agent/opa/v1/ast"

// EvaluatedRuleTracker records rule identifiers from annotations during
// evaluation. It extracts the ID field from each successfully evaluated
// rule's annotations. Duplicate IDs are suppressed.
type EvaluatedRuleTracker struct {
	IDs           []string
	Labels        []map[string]any
	CollectLabels bool
	seen          map[string]struct{}
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
		}
		if t.CollectLabels && len(a.Labels) > 0 {
			t.Labels = append(t.Labels, a.Labels)
		}
	}
}
