// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"maps"

	"github.com/open-policy-agent/opa/v1/ast"
)

// EvaluatedRuleTracker records rule identifiers from annotations during
// evaluation. It extracts the ID field from each successfully evaluated
// rule's annotations. Duplicate IDs are suppressed.
type EvaluatedRuleTracker struct {
	IDs           []string
	RuleMetadata  []map[string]any
	CollectCustom bool
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
		if t.CollectCustom && len(a.Custom) > 0 {
			entry := make(map[string]any, len(a.Custom)+1)
			if a.ID != "" {
				entry["id"] = a.ID
			}
			maps.Copy(entry, a.Custom)
			t.RuleMetadata = append(t.RuleMetadata, entry)
		}
	}
}
