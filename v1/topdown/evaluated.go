// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"encoding/json"
	"maps"

	"github.com/open-policy-agent/opa/v1/ast"
)

// EvaluatedRuleTracker records labels from annotations during evaluation.
// For each successfully evaluated rule, labels from the rule's annotation
// chain (subpackages, package, document, rule) are merged into a single map
// with inner-scope-wins precedence. The merged maps are deduplicated across
// rules so that identical label sets collapse to a single entry.
//
// An AnnotationSet must be set via WithAnnotationSet for the tracker to
// resolve chains; without it, Record is a no-op.
type EvaluatedRuleTracker struct {
	Labels []map[string]any
	seen   map[string]struct{}
	as     *ast.AnnotationSet
}

// WithAnnotationSet configures the AnnotationSet used to resolve each
// evaluated rule's annotation chain. Typically wired from the compiler.
func (t *EvaluatedRuleTracker) WithAnnotationSet(as *ast.AnnotationSet) *EvaluatedRuleTracker {
	if t != nil {
		t.as = as
	}
	return t
}

func (t *EvaluatedRuleTracker) Record(rule *ast.Rule) {
	if t == nil || t.as == nil {
		return
	}

	merged := mergeRuleLabels(t.as.Chain(rule))
	if len(merged) == 0 {
		return
	}

	b, _ := json.Marshal(merged)
	key := string(b)
	if t.seen == nil {
		t.seen = make(map[string]struct{})
	}
	if _, dup := t.seen[key]; dup {
		return
	}
	t.seen[key] = struct{}{}
	t.Labels = append(t.Labels, merged)
}

// mergeRuleLabels folds labels from a rule's annotation chain with inner-wins
// precedence. AnnotationSet.Chain returns entries in inner-to-outer order, so
// we iterate in reverse to fold outer-to-inner.
func mergeRuleLabels(chain ast.AnnotationsRefSet) map[string]any {
	var merged map[string]any
	for i := len(chain) - 1; i >= 0; i-- {
		a := chain[i].Annotations
		if a == nil || len(a.Labels) == 0 {
			continue
		}
		if merged == nil {
			merged = make(map[string]any, len(a.Labels))
		}
		maps.Copy(merged, a.Labels)
	}
	return merged
}
