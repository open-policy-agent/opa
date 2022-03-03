// Copyright 2022 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"encoding/json"
	"testing"
)

func TestAnnotationSetFlatten(t *testing.T) {
	tests := []struct {
		note     string
		modules  map[string]string
		expected []AnnotationsRef
	}{
		{
			note:     "no modules",
			modules:  map[string]string{},
			expected: []AnnotationsRef{},
		},
		{
			note: "simple module (all annotation types)",
			modules: map[string]string{
				"module": `# METADATA
# title: pkg
# description: pkg
# organizations:
# - pkg
# related_resources:
# - https://pkg
# authors:
# - pkg
# schemas:
# - input: {"type": "boolean"}
# custom:
#  pkg: pkg
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
# - input: {"type": "string"}
# custom:
#  rule: rule
p = 1`,
			},
			expected: []AnnotationsRef{
				{
					Path:     MustParseRef("data.test"),
					Location: &Location{File: "module", Row: 14},
					Annotations: &Annotations{
						Scope:         "package",
						Title:         "pkg",
						Description:   "pkg",
						Organizations: []string{"pkg"},
						RelatedResources: []*RelatedResourceAnnotation{
							{
								Ref: mustParseURL("https://pkg"),
							},
						},
						Authors: []*AuthorAnnotation{
							{
								Name: "pkg",
							},
						},
						Schemas: []*SchemaAnnotation{
							schemaAnnotationFromMap("input", map[string]interface{}{
								"type": "boolean",
							}),
						},
						Custom: map[string]interface{}{
							"pkg": "pkg",
						},
					},
				},
				{
					Path:     MustParseRef("data.test.p"),
					Location: &Location{File: "module", Row: 16},
					Annotations: &Annotations{
						Scope:         "document",
						Title:         "doc",
						Description:   "doc",
						Organizations: []string{"doc"},
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
				{
					Path:     MustParseRef("data.test.p"),
					Location: &Location{File: "module", Row: 44},
					Annotations: &Annotations{
						Scope:         "rule",
						Title:         "rule",
						Description:   "rule",
						Organizations: []string{"rule"},
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
								"type": "string",
							}),
						},
						Custom: map[string]interface{}{
							"rule": "rule",
						},
					},
				},
			},
		},
		{
			note: "multiple subpackages",
			modules: map[string]string{
				"root": `# METADATA
# scope: subpackages
# title: ROOT
package root`,
				"root.foo": `# METADATA
# title: FOO
# scope: subpackages
package root.foo`,
				"root.foo.baz": `# METADATA
# title: BAZ
package root.foo.baz`,
				"root.bar": `# METADATA
# title: BAR
# scope: subpackages
package root.bar`,
				"root.bar.baz": `# METADATA
# title: BAZ
package root.bar.baz`,
				"root2": `# METADATA
# scope: subpackages
# title: ROOT2
package root2`,
			},
			expected: []AnnotationsRef{
				{
					Path:     MustParseRef("data.root"),
					Location: &Location{File: "root", Row: 1},
					Annotations: &Annotations{
						Scope: "subpackages",
						Title: "ROOT",
					},
				},
				{
					Path:     MustParseRef("data.root.bar"),
					Location: &Location{File: "root.bar", Row: 1},
					Annotations: &Annotations{
						Scope: "subpackages",
						Title: "BAR",
					},
				},
				{
					Path:     MustParseRef("data.root.bar.baz"),
					Location: &Location{File: "root.bar.baz", Row: 3},
					Annotations: &Annotations{
						Scope: "package",
						Title: "BAZ",
					},
				},
				{
					Path:     MustParseRef("data.root.foo"),
					Location: &Location{File: "root.foo", Row: 1},
					Annotations: &Annotations{
						Scope: "subpackages",
						Title: "FOO",
					},
				},
				{
					Path:     MustParseRef("data.root.foo.baz"),
					Location: &Location{File: "root.foo.baz", Row: 3},
					Annotations: &Annotations{
						Scope: "package",
						Title: "BAZ",
					},
				},
				{
					Path:     MustParseRef("data.root2"),
					Location: &Location{File: "root2", Row: 1},
					Annotations: &Annotations{
						Scope: "subpackages",
						Title: "ROOT2",
					},
				},
			},
		},
		{
			note: "overlapping package paths",
			modules: map[string]string{
				"mod1": `# METADATA
# title: TEST1
package test`,
				"mod2": `# METADATA
# title: TEST2
package test`,
			},
			expected: []AnnotationsRef{
				{
					Path:     MustParseRef("data.test"),
					Location: &Location{File: "mod1", Row: 3},
					Annotations: &Annotations{
						Scope: "package",
						Title: "TEST1",
					},
				},
				{
					Path:     MustParseRef("data.test"),
					Location: &Location{File: "mod2", Row: 3},
					Annotations: &Annotations{
						Scope: "package",
						Title: "TEST2",
					},
				},
			},
		},
		{
			note: "overlapping rule paths (same module)",
			modules: map[string]string{
				"mod": `package test

# METADATA
# title: P1
p[v] {v = 1}

# METADATA
# title: P2
p[v] {v = 2}`,
			},
			expected: []AnnotationsRef{
				{
					Path:     MustParseRef("data.test.p"),
					Location: &Location{File: "mod", Row: 5},
					Annotations: &Annotations{
						Scope: "rule",
						Title: "P1",
					},
				},
				{
					Path:     MustParseRef("data.test.p"),
					Location: &Location{File: "mod", Row: 9},
					Annotations: &Annotations{
						Scope: "rule",
						Title: "P2",
					},
				},
			},
		},
		{
			note: "overlapping rule paths (different modules)",
			modules: map[string]string{
				"mod1": `package test
# METADATA
# title: P1
p[v] {v = 1}`,
				"mod2": `package test
# METADATA
# title: P2
p[v] {v = 2}`,
			},
			expected: []AnnotationsRef{
				{
					Path:     MustParseRef("data.test.p"),
					Location: &Location{File: "mod1", Row: 4},
					Annotations: &Annotations{
						Scope: "rule",
						Title: "P1",
					},
				},
				{
					Path:     MustParseRef("data.test.p"),
					Location: &Location{File: "mod2", Row: 4},
					Annotations: &Annotations{
						Scope: "rule",
						Title: "P2",
					},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			compiler := MustCompileModulesWithOpts(tc.modules,
				CompileOpts{ParserOptions: ParserOptions{ProcessAnnotation: true}})

			as := compiler.GetAnnotationSet()
			if as == nil {
				t.Fatalf("Expected compiled AnnotationSet, got nil")
			}

			flattened := as.Flatten()

			if len(flattened) != len(tc.expected) {
				t.Fatalf("flattened AnnotationSet\n%v\ndoesn't match expected\n%v",
					toJSON(flattened), toJSON(tc.expected))
			}

			for i, expected := range tc.expected {
				a := flattened[i]
				if !expected.Path.Equal(a.Path) {
					t.Fatalf("path of AnnotationRef at %d '%v' doesn't match expected '%v'",
						i, a.Path, expected.Path)
				}
				if expected.Location.File != a.Location.File || expected.Location.Row != a.Location.Row {
					t.Fatalf("location of AnnotationRef at %d '%v' doesn't match expected '%v'",
						i, a.Location, expected.Location)
				}
				if expected.Annotations.Compare(a.Annotations) != 0 {
					t.Fatalf("annotations of AnnotationRef at %d\n%v\n doesn't match expected\n%v",
						i, a.Annotations, expected.Annotations)
				}
			}
		})
	}
}

func toJSON(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func schemaAnnotationFromMap(path string, def map[string]interface{}) *SchemaAnnotation {
	var p interface{} = def
	return &SchemaAnnotation{Path: MustParseRef(path), Definition: &p}
}
