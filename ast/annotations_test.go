// Copyright 2022 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"fmt"
	"testing"
)

func TestGetAnnotations(t *testing.T) {
	type expectedKey struct {
		path      string
		isPackage bool
		row       int
	}

	tests := []struct {
		note     string
		modules  []string
		expected map[expectedKey]Annotations
	}{
		{
			note: "rule inherits document",
			modules: []string{
				`package test

# METADATA
# title: doc
# scope: document
# description: doc
# organizations:
# - doc
# related_resources: 
# - https://doc
# authors:
# - doc
# schemas:
# - input: {"type": "string"}
# custom:
#  doc: doc

p = 1`,
			},
			expected: map[expectedKey]Annotations{
				{path: "data.test.p"}: {
					Title:       "doc",
					Description: "doc",
					Organizations: []string{
						"doc",
					},
					RelatedResources: []*RelatedResourceAnnotation{
						{
							Ref: mustParseURL("https://doc"),
						},
					},
					Authors: []*AuthorAnnotation{
						{
							Name: "doc",
						},
					},
					Schemas: []*SchemaAnnotation{
						schemaAnnotationFromMap("input", map[string]interface{}{
							"type": "string",
						}),
					},
					Custom: map[string]interface{}{
						"doc": "doc",
					},
				},
			},
		},
		{
			note: "rule overrides document",
			modules: []string{
				`package test

# METADATA
# title: doc
# scope: document
# description: doc
# organizations:
# - doc
# related_resources: 
# - https://doc
# authors:
# - doc
# schemas:
# - input: {"type": "string"}
# custom:
#  doc: doc

# METADATA
# title: rule
# description: rule
# organizations:
# - rule
# related_resources: 
# - https://rule
# authors:
# - rule
# schemas:
# - input: {"type": "integer"}
# custom:
#  rule: rule
p = 1`,
			},
			expected: map[expectedKey]Annotations{
				{path: "data.test.p"}: {
					Title:       "rule",
					Description: "rule",
					Organizations: []string{
						"rule",
					},
					RelatedResources: []*RelatedResourceAnnotation{
						{
							Ref: mustParseURL("https://rule"),
						},
					},
					Authors: []*AuthorAnnotation{
						{
							Name: "rule",
						},
					},
					Schemas: []*SchemaAnnotation{
						schemaAnnotationFromMap("input", map[string]interface{}{
							"type": "integer",
						}),
					},
					Custom: map[string]interface{}{
						"rule": "rule",
					},
				},
			},
		},
		{
			note: "document inherits package",
			modules: []string{
				`# METADATA
# title: package
# scope: subpackages
# description: package
# organizations:
# - package
# related_resources: 
# - https://package
# authors:
# - package
# schemas:
# - input: {"type": "string"}
# custom:
#  package: package
package test

p = 1`,
			},
			expected: map[expectedKey]Annotations{
				{path: "data.test.p"}: {
					Title:       "package",
					Description: "package",
					Organizations: []string{
						"package",
					},
					RelatedResources: []*RelatedResourceAnnotation{
						{
							Ref: mustParseURL("https://package"),
						},
					},
					Authors: []*AuthorAnnotation{
						{
							Name: "package",
						},
					},
					Schemas: []*SchemaAnnotation{
						schemaAnnotationFromMap("input", map[string]interface{}{
							"type": "string",
						}),
					},
					Custom: map[string]interface{}{
						"package": "package",
					},
				},
			},
		},
		{
			note: "document overrides package",
			modules: []string{
				`# METADATA
# title: package
# scope: subpackages
# description: package
# organizations:
# - package
# related_resources: 
# - https://package
# authors:
# - package
# schemas:
# - input: {"type": "string"}
# custom:
#  package: package
package test

# METADATA
# scope: document
# title: doc
# description: doc
# organizations:
# - doc
# related_resources: 
# - https://doc
# authors:
# - doc
# schemas:
# - input: {"type": "integer"}
# custom:
#  doc: doc
p = 1`,
			},
			expected: map[expectedKey]Annotations{
				{path: "data.test.p"}: {
					Title:       "doc",
					Description: "doc",
					Organizations: []string{
						"doc",
					},
					RelatedResources: []*RelatedResourceAnnotation{
						{
							Ref: mustParseURL("https://doc"),
						},
					},
					Authors: []*AuthorAnnotation{
						{
							Name: "doc",
						},
					},
					Schemas: []*SchemaAnnotation{
						schemaAnnotationFromMap("input", map[string]interface{}{
							"type": "integer",
						}),
					},
					Custom: map[string]interface{}{
						"doc": "doc",
					},
				},
			},
		},
		{
			note: "package inherits super-package",
			modules: []string{
				`# METADATA
# title: root
# scope: subpackages
# description: root
# organizations:
# - root
# related_resources: 
# - https://root
# authors:
# - root
# schemas:
# - input: {"type": "string"}
# custom:
#  root: root
package root

p = 1`,
				`package root.leaf

p = 1`,
			},
			expected: map[expectedKey]Annotations{
				{path: "data.root.leaf", isPackage: true}: {
					Title:       "root",
					Description: "root",
					Organizations: []string{
						"root",
					},
					RelatedResources: []*RelatedResourceAnnotation{
						{
							Ref: mustParseURL("https://root"),
						},
					},
					Authors: []*AuthorAnnotation{
						{
							Name: "root",
						},
					},
					Schemas: []*SchemaAnnotation{
						schemaAnnotationFromMap("input", map[string]interface{}{
							"type": "string",
						}),
					},
					Custom: map[string]interface{}{
						"root": "root",
					},
				},
			},
		},
		{
			note: "package overrides super-package",
			modules: []string{
				`# METADATA
# title: root
# scope: subpackages
# description: root
# organizations:
# - root
# related_resources: 
# - https://root
# authors:
# - root
# schemas:
# - input: {"type": "string"}
# custom:
#  root: root
package root

p = 1`,
				`# METADATA
# title: leaf
# description: leaf
# organizations:
# - leaf
# related_resources: 
# - https://leaf
# authors:
# - leaf
# schemas:
# - input: {"type": "integer"}
# custom:
#  leaf: leaf
package root.leaf

p = 1`,
			},
			expected: map[expectedKey]Annotations{
				{path: "data.root.leaf", isPackage: true}: {
					Title:       "leaf",
					Description: "leaf",
					Organizations: []string{
						"leaf",
					},
					RelatedResources: []*RelatedResourceAnnotation{
						{
							Ref: mustParseURL("https://leaf"),
						},
					},
					Authors: []*AuthorAnnotation{
						{
							Name: "leaf",
						},
					},
					Schemas: []*SchemaAnnotation{
						schemaAnnotationFromMap("input", map[string]interface{}{
							"type": "integer",
						}),
					},
					Custom: map[string]interface{}{
						"leaf": "leaf",
					},
				},
			},
		},
		{
			note: "partial overrides",
			modules: []string{
				`# METADATA
# description: the root package
# organizations: 
# - Lex Corp.
# authors: 
# - John Doe
# scope: subpackages
package root

p = 1`, // FIXME: If this rule isn't included, annotations from this module aren't included in compilation (#4369)
				`# METADATA
# title: a package
# description: the foo package
# authors: 
# - Jane Doe
package root.foo

# METADATA
# description: a set of rules populating a set
# scope: document

# METADATA
# title: a rule
p[x] {x = 1}

# METADATA
# title: another rule
p[x] {x = 2}`,
				`package root.bar

p = 1`,
			},
			expected: map[expectedKey]Annotations{
				{path: "data.root.foo.p", row: 14}: {
					Title:       "a rule",
					Description: "a set of rules populating a set",
					Authors: []*AuthorAnnotation{
						{
							Name: "Jane Doe",
						},
					},
					Organizations: []string{
						"Lex Corp.",
					},
				},
				{path: "data.root.foo.p", row: 18}: {
					Title:       "another rule",
					Description: "a set of rules populating a set",
					Authors: []*AuthorAnnotation{
						{
							Name: "Jane Doe",
						},
					},
					Organizations: []string{
						"Lex Corp.",
					},
				},
				{path: "data.root.bar.p"}: {
					Description: "the root package",
					Authors: []*AuthorAnnotation{
						{
							Name: "John Doe",
						},
					},
					Organizations: []string{
						"Lex Corp.",
					},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			modules := make(map[string]string, len(tc.modules))
			for i, mod := range tc.modules {
				modules[fmt.Sprintf("module%d", i)] = mod
			}

			for k, expAnnotations := range tc.expected {
				compiler := MustCompileModulesWithOpts(modules,
					CompileOpts{ParserOptions: ParserOptions{ProcessAnnotation: true}})

				ref := MustParseRef(k.path)
				var annotations *Annotations

				if k.isPackage {
					pkg := getPackage(compiler, ref)
					if pkg == nil {
						t.Fatalf("no package '%s'", k.path)
					}
					annotations = compiler.GetPackageAnnotations(pkg)
				} else {
					// rules order is not stable
					rules := compiler.GetRules(ref)
					var rule *Rule
					if k.row == 0 {
						// we only expect one rule, as no row was provided
						rule = rules[0]
					} else {
						rule = getRuleOnRow(rules, k.row)
					}
					if rule == nil {
						t.Fatalf("no rule '%s' on row %d", k.path, k.row)
					}
					annotations = compiler.GetRuleAnnotations(rule)
				}

				if annotations == nil || expAnnotations.Compare(annotations) != 0 {
					t.Fatalf("expected %v for '%s':%d but got %v",
						expAnnotations, k.path, k.row, annotations)
				}
			}
		})
	}
}

func getRuleOnRow(rules []*Rule, row int) *Rule {
	for _, r := range rules {
		if r.Location.Row == row {
			return r
		}
	}
	return nil
}

func schemaAnnotationFromMap(path string, def map[string]interface{}) *SchemaAnnotation {
	var p interface{} = def
	return &SchemaAnnotation{Path: MustParseRef(path), Definition: &p}
}

func getPackage(c *Compiler, ref Ref) *Package {
	for _, mod := range c.Modules {
		if ref.Equal(mod.Package.Path) {
			return mod.Package
		}
	}
	return nil
}
