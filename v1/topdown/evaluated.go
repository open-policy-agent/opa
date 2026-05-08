// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import "github.com/open-policy-agent/opa/v1/ast"

// EvaluatedRuleTracker records labels from annotations during evaluation.
// Labels from all successfully evaluated rules are aggregated.
type EvaluatedRuleTracker struct {
	Labels []map[string]any
}

func (t *EvaluatedRuleTracker) Record(rule *ast.Rule) {
	if t == nil || len(rule.Annotations) == 0 {
		return
	}

	for _, a := range rule.Annotations {
		if len(a.Labels) > 0 {
			t.Labels = append(t.Labels, a.Labels)
		}
	}
}
