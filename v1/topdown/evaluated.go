// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"encoding/json"

	"github.com/open-policy-agent/opa/v1/ast"
)

// EvaluatedRuleTracker records labels from annotations during evaluation.
// Labels from all successfully evaluated rules are aggregated. Exact
// duplicates (same key-value pairs) are suppressed.
type EvaluatedRuleTracker struct {
	Labels []map[string]any
	seen   map[string]struct{}
}

func (t *EvaluatedRuleTracker) Record(rule *ast.Rule) {
	if t == nil || len(rule.Annotations) == 0 {
		return
	}

	for _, a := range rule.Annotations {
		if len(a.Labels) > 0 {
			b, _ := json.Marshal(a.Labels)
			key := string(b)
			if t.seen == nil {
				t.seen = make(map[string]struct{})
			}
			if _, dup := t.seen[key]; !dup {
				t.seen[key] = struct{}{}
				t.Labels = append(t.Labels, a.Labels)
			}
		}
	}
}
