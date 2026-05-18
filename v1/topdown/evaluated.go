// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
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
	labels, key := t.as.MergedLabels(rule)
	if len(labels) == 0 {
		return
	}
	if t.seen == nil {
		t.seen = make(map[string]struct{})
	}
	if _, dup := t.seen[key]; dup {
		return
	}
	t.seen[key] = struct{}{}
	t.Labels = append(t.Labels, labels)
}
