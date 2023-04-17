// Copyright 2022 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"encoding/json"
	"fmt"
	"testing"
)

// Test of example code in docs/content/annotations.md
func ExampleAnnotationSet_Flatten() {
	modules := [][]string{
		{
			"foo.rego", `# METADATA
# scope: subpackages
# organizations:
# - Acme Corp.
package foo`},
		{
			"mod", `# METADATA
# description: A couple of useful rules
package foo.bar

# METADATA
# title: My Rule P
p := 7`},
	}

	parsed := make([]*Module, 0, len(modules))
	for _, entry := range modules {
		pm, err := ParseModuleWithOpts(entry[0], entry[1], ParserOptions{ProcessAnnotation: true})
		if err != nil {
			panic(err)
		}
		parsed = append(parsed, pm)
	}

	as, err := BuildAnnotationSet(parsed)
	if err != nil {
		panic(err)
	}

	flattened := as.Flatten()
	for _, entry := range flattened {
		fmt.Printf("%v at %v has annotations %v\n",
			entry.Path,
			entry.Location,
			entry.Annotations)
	}

	// Output:
	// data.foo at foo.rego:5 has annotations {"organizations":["Acme Corp."],"scope":"subpackages"}
	// data.foo.bar at mod:3 has annotations {"description":"A couple of useful rules","scope":"package"}
	// data.foo.bar.p at mod:7 has annotations {"scope":"rule","title":"My Rule P"}
}

// Test of example code in docs/content/annotations.md
func ExampleAnnotationSet_Chain() {
	modules := [][]string{
		{
			"foo.rego", `# METADATA
# scope: subpackages
# organizations:
# - Acme Corp.
package foo`},
		{
			"mod", `# METADATA
# description: A couple of useful rules
package foo.bar

# METADATA
# title: My Rule P
p := 7`},
	}

	parsed := make([]*Module, 0, len(modules))
	for _, entry := range modules {
		pm, err := ParseModuleWithOpts(entry[0], entry[1], ParserOptions{ProcessAnnotation: true})
		if err != nil {
			panic(err)
		}
		parsed = append(parsed, pm)
	}

	as, err := BuildAnnotationSet(parsed)
	if err != nil {
		panic(err)
	}

	rule := parsed[1].Rules[0]

	flattened := as.Chain(rule)
	for _, entry := range flattened {
		fmt.Printf("%v at %v has annotations %v\n",
			entry.Path,
			entry.Location,
			entry.Annotations)
	}

	// Output:
	// data.foo.bar.p at mod:7 has annotations {"scope":"rule","title":"My Rule P"}
	// data.foo.bar at mod:3 has annotations {"description":"A couple of useful rules","scope":"package"}
	// data.foo at foo.rego:5 has annotations {"organizations":["Acme Corp."],"scope":"subpackages"}
}

func TestAnnotationSet_Flatten(t *testing.T) {
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
					Location: &Location{File: "module", Row: 44},
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
					Location: &Location{File: "root", Row: 4},
					Annotations: &Annotations{
						Scope: "subpackages",
						Title: "ROOT",
					},
				},
				{
					Path:     MustParseRef("data.root.bar"),
					Location: &Location{File: "root.bar", Row: 4},
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
					Location: &Location{File: "root.foo", Row: 4},
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
					Location: &Location{File: "root2", Row: 4},
					Annotations: &Annotations{
						Scope: "subpackages",
						Title: "ROOT2",
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
		{
			note: "overlapping rule paths (different modules, rule head refs)",
			modules: map[string]string{
				"mod1": `package test.a
# METADATA
# title: P1
b.c.p[v] {v = 1}`,
				"mod2": `package test
# METADATA
# title: P2
a.b.c.p[v] {v = 2}`,
			},
			expected: []AnnotationsRef{
				{
					Path:     MustParseRef("data.test.a.b.c.p"),
					Location: &Location{File: "mod1", Row: 4},
					Annotations: &Annotations{
						Scope: "rule",
						Title: "P1",
					},
				},
				{
					Path:     MustParseRef("data.test.a.b.c.p"),
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
					t.Fatalf("annotations of AnnotationRef at %d\n%v\ndoesn't match expected\n%v",
						i, a.Annotations, expected.Annotations)
				}
			}
		})
	}
}

func TestAnnotationSet_Chain(t *testing.T) {
	tests := []struct {
		note                string
		modules             map[string]string
		moduleToAnalyze     string
		ruleOnLineToAnalyze int
		expected            []AnnotationsRef
	}{
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
# - input.foo: {"type": "boolean"}
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
# - input.bar: {"type": "integer"}
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
# - input.baz: {"type": "string"}
# custom:
#  rule: rule
p = 1`,
			},
			moduleToAnalyze:     "module",
			ruleOnLineToAnalyze: 44,
			expected: []AnnotationsRef{
				{ // Rule annotation is always first
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
							schemaAnnotationFromMap("input.baz", map[string]interface{}{
								"type": "string",
							}),
						},
						Custom: map[string]interface{}{
							"rule": "rule",
						},
					},
				},
				{
					Path:     MustParseRef("data.test.p"),
					Location: &Location{File: "module", Row: 44},
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
							schemaAnnotationFromMap("input.bar", map[string]interface{}{
								"type": "integer",
							}),
						},
						Custom: map[string]interface{}{
							"doc": "doc",
						},
					},
				},
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
							schemaAnnotationFromMap("input.foo", map[string]interface{}{
								"type": "boolean",
							}),
						},
						Custom: map[string]interface{}{
							"pkg": "pkg",
						},
					},
				},
			},
		},
		{
			note: "no annotations on rule",
			modules: map[string]string{
				"module": `# METADATA
# title: pkg
# description: pkg
package test

# METADATA
# scope: document
# title: doc
# description: doc

p = 1`,
			},
			moduleToAnalyze:     "module",
			ruleOnLineToAnalyze: 11,
			expected: []AnnotationsRef{
				{ // Rule entry is always first, even if no annotations are present
					Path:        MustParseRef("data.test.p"),
					Location:    &Location{File: "module", Row: 11},
					Annotations: nil,
				},
				{
					Path:     MustParseRef("data.test.p"),
					Location: &Location{File: "module", Row: 11},
					Annotations: &Annotations{
						Scope:       "document",
						Title:       "doc",
						Description: "doc",
					},
				},

				{
					Path:     MustParseRef("data.test"),
					Location: &Location{File: "module", Row: 4},
					Annotations: &Annotations{
						Scope:       "package",
						Title:       "pkg",
						Description: "pkg",
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
				"root.foo.bar": `# METADATA
# scope: subpackages
# description: subpackages scope applied to rule in other module
# title: BAR-sub

# METADATA
# title: BAR-other
# description: This metadata is on the path of the queried rule, and should show up in the result even though it's in a different module.
package root.foo.bar

# METADATA
# scope: document
# description: document scope applied to rule in other module
# title: P-doc
p = 1`,
				"rule": `package root.foo.bar

# METADATA
# title: P
p = 1`,
			},
			moduleToAnalyze:     "rule",
			ruleOnLineToAnalyze: 5,
			expected: []AnnotationsRef{
				{
					Path:     MustParseRef("data.root.foo.bar.p"),
					Location: &Location{File: "rule", Row: 5},
					Annotations: &Annotations{
						Scope: "rule",
						Title: "P",
					},
				},
				{
					Path:     MustParseRef("data.root.foo.bar.p"),
					Location: &Location{File: "root.foo.bar", Row: 15},
					Annotations: &Annotations{
						Scope:       "document",
						Title:       "P-doc",
						Description: "document scope applied to rule in other module",
					},
				},
				{
					Path:     MustParseRef("data.root.foo.bar"),
					Location: &Location{File: "root.foo.bar", Row: 9},
					Annotations: &Annotations{
						Scope:       "package",
						Title:       "BAR-other",
						Description: "This metadata is on the path of the queried rule, and should show up in the result even though it's in a different module.",
					},
				},
				{
					Path:     MustParseRef("data.root.foo.bar"),
					Location: &Location{File: "root.foo.bar", Row: 9},
					Annotations: &Annotations{
						Scope:       "subpackages",
						Title:       "BAR-sub",
						Description: "subpackages scope applied to rule in other module",
					},
				},
				{
					Path:     MustParseRef("data.root.foo"),
					Location: &Location{File: "root.foo", Row: 4},
					Annotations: &Annotations{
						Scope: "subpackages",
						Title: "FOO",
					},
				},
				{
					Path:     MustParseRef("data.root"),
					Location: &Location{File: "root", Row: 4},
					Annotations: &Annotations{
						Scope: "subpackages",
						Title: "ROOT",
					},
				},
			},
		},
		{
			note: "multiple subpackages, refs in rule heads", // NOTE(sr): same as above, but last module's rule is `foo.bar.p` in package `root`
			modules: map[string]string{
				"root": `# METADATA
# scope: subpackages
# title: ROOT
package root`,
				"root.foo": `# METADATA
# title: FOO
# scope: subpackages
package root.foo`,
				"root.foo.bar": `# METADATA
# scope: subpackages
# description: subpackages scope applied to rule in other module
# title: BAR-sub

# METADATA
# title: BAR-other
# description: This metadata is on the path of the queried rule, but shouldn't show up in the result as it's in a different module.
package root.foo.bar

# METADATA
# scope: document
# description: document scope applied to rule in other module
# title: P-doc
p = 1`,
				"rule": `# METADATA
# title: BAR
package root

# METADATA
# title: P
foo.bar.p = 1`,
			},
			moduleToAnalyze:     "rule",
			ruleOnLineToAnalyze: 7,
			expected: []AnnotationsRef{
				{
					Path:     MustParseRef("data.root.foo.bar.p"),
					Location: &Location{File: "rule", Row: 7},
					Annotations: &Annotations{
						Scope: "rule",
						Title: "P",
					},
				},
				{
					Path:     MustParseRef("data.root.foo.bar.p"),
					Location: &Location{File: "root.foo.bar", Row: 15},
					Annotations: &Annotations{
						Scope:       "document",
						Title:       "P-doc",
						Description: "document scope applied to rule in other module",
					},
				},
				{
					Path:     MustParseRef("data.root"),
					Location: &Location{File: "rule", Row: 3},
					Annotations: &Annotations{
						Scope: "package",
						Title: "BAR",
					},
				},
				{
					Path:     MustParseRef("data.root"),
					Location: &Location{File: "root", Row: 4},
					Annotations: &Annotations{
						Scope: "subpackages",
						Title: "ROOT",
					},
				},
			},
		},
		{
			note: "multiple metadata blocks for single rule (order)",
			modules: map[string]string{
				"module": `package test

# METADATA
# title: One

# METADATA
# title: Two

# METADATA
# title: Three

# METADATA
# title: Four
p = true`,
			},
			moduleToAnalyze:     "module",
			ruleOnLineToAnalyze: 14,
			expected: []AnnotationsRef{ // Rule annotations order is expected to start closest to the rule, moving out
				{
					Path:     MustParseRef("data.test.p"),
					Location: &Location{File: "module", Row: 14},
					Annotations: &Annotations{
						Scope: "rule",
						Title: "Four",
					},
				},
				{
					Path:     MustParseRef("data.test.p"),
					Location: &Location{File: "module", Row: 14},
					Annotations: &Annotations{
						Scope: "rule",
						Title: "Three",
					},
				},
				{
					Path:     MustParseRef("data.test.p"),
					Location: &Location{File: "module", Row: 14},
					Annotations: &Annotations{
						Scope: "rule",
						Title: "Two",
					},
				},
				{
					Path:     MustParseRef("data.test.p"),
					Location: &Location{File: "module", Row: 14},
					Annotations: &Annotations{
						Scope: "rule",
						Title: "One",
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

			m := compiler.Modules[tc.moduleToAnalyze]
			if m == nil {
				t.Fatalf("no such module: %s", tc.moduleToAnalyze)
			}

			var rule *Rule
			for _, r := range m.Rules {
				if r.Location.Row == tc.ruleOnLineToAnalyze {
					rule = r
					break
				}
			}
			if rule == nil {
				t.Fatalf("no rule found on line %d in module '%s'",
					tc.ruleOnLineToAnalyze, tc.moduleToAnalyze)
			}

			chain := as.Chain(rule)

			if len(chain) != len(tc.expected) {
				t.Errorf("expected %d elements, got %d:", len(tc.expected), len(chain))
				t.Fatalf("chained AnnotationSet\n%v\n\ndoesn't match expected\n\n%v",
					toJSON(chain), toJSON(tc.expected))
			}

			for i, expected := range tc.expected {
				a := chain[i]
				if !expected.Path.Equal(a.Path) {
					t.Fatalf("path of AnnotationRef at %d '%v' doesn't match expected '%v'",
						i, a.Path, expected.Path)
				}
				if expected.Location.File != a.Location.File || expected.Location.Row != a.Location.Row {
					t.Fatalf("location of AnnotationRef at %d '%v' doesn't match expected '%v'",
						i, a.Location, expected.Location)
				}
				if expected.Annotations.Compare(a.Annotations) != 0 {
					t.Fatalf("annotations of AnnotationRef at %d\n%v\n\ndoesn't match expected\n\n%v",
						i, a.Annotations, expected.Annotations)
				}
			}
		})
	}
}

func TestAnnotations_toObject(t *testing.T) {
	annotations := Annotations{
		Scope:       annotationScopeRule,
		Title:       "A title",
		Description: "A description",
		Organizations: []string{
			"Acme Corp.",
			"Tyrell Corp.",
		},
		RelatedResources: []*RelatedResourceAnnotation{
			{
				Ref:         mustParseURL("https://example.com"),
				Description: "An example",
			},
			{
				Ref: mustParseURL("https://another.example.com"),
			},
		},
		Authors: []*AuthorAnnotation{
			{
				Name:  "John Doe",
				Email: "john@example.com",
			},
			{
				Name: "Jane Doe",
			},
			{
				Email: "jeff@example.com",
			},
		},
		Schemas: []*SchemaAnnotation{
			{
				Path:   MustParseRef("input.foo"),
				Schema: MustParseRef("schema.a"),
			},
			schemaAnnotationFromMap("input.bar", map[string]interface{}{
				"type": "boolean",
			}),
		},
		Custom: map[string]interface{}{
			"number": 42,
			"float":  2.2,
			"string": "foo bar baz",
			"bool":   true,
			"list": []interface{}{
				"a", "b",
			},
			"list_of_lists": []interface{}{
				[]interface{}{
					"a", "b",
				},
				[]interface{}{
					"b", "c",
				},
			},
			"list_of_maps": []interface{}{
				map[string]interface{}{
					"one": 1,
					"two": 2,
				},
				map[string]interface{}{
					"two":   2,
					"three": 3,
				},
			},
			"map": map[string]interface{}{
				"nested_number": 1,
				"nested_map": map[string]interface{}{
					"do": "re",
					"mi": "fa",
				},
				"nested_list": []interface{}{
					1, 2, 3,
				},
			},
		},
	}

	expected := NewObject(
		Item(StringTerm("scope"), StringTerm(annotationScopeRule)),
		Item(StringTerm("title"), StringTerm("A title")),
		Item(StringTerm("description"), StringTerm("A description")),
		Item(StringTerm("organizations"), ArrayTerm(
			StringTerm("Acme Corp."),
			StringTerm("Tyrell Corp."),
		)),
		Item(StringTerm("related_resources"), ArrayTerm(
			ObjectTerm(
				Item(StringTerm("ref"), StringTerm("https://example.com")),
				Item(StringTerm("description"), StringTerm("An example")),
			),
			ObjectTerm(
				Item(StringTerm("ref"), StringTerm("https://another.example.com")),
			),
		)),
		Item(StringTerm("authors"), ArrayTerm(
			ObjectTerm(
				Item(StringTerm("name"), StringTerm("John Doe")),
				Item(StringTerm("email"), StringTerm("john@example.com")),
			),
			ObjectTerm(
				Item(StringTerm("name"), StringTerm("Jane Doe")),
			),
			ObjectTerm(
				Item(StringTerm("email"), StringTerm("jeff@example.com")),
			),
		)),
		Item(StringTerm("schemas"), ArrayTerm(
			ObjectTerm(
				Item(StringTerm("path"), ArrayTerm(StringTerm("input"), StringTerm("foo"))),
				Item(StringTerm("schema"), ArrayTerm(StringTerm("schema"), StringTerm("a"))),
			),
			ObjectTerm(
				Item(StringTerm("path"), ArrayTerm(StringTerm("input"), StringTerm("bar"))),
				Item(StringTerm("definition"), ObjectTerm(
					Item(StringTerm("type"), StringTerm("boolean")),
				)),
			),
		)),
		Item(StringTerm("custom"), ObjectTerm(
			Item(StringTerm("number"), NumberTerm("42")),
			Item(StringTerm("float"), NumberTerm("2.2")),
			Item(StringTerm("string"), StringTerm("foo bar baz")),
			Item(StringTerm("bool"), BooleanTerm(true)),
			Item(StringTerm("list"), ArrayTerm(
				StringTerm("a"),
				StringTerm("b"),
			)),
			Item(StringTerm("list_of_lists"), ArrayTerm(
				ArrayTerm(
					StringTerm("a"),
					StringTerm("b"),
				),
				ArrayTerm(
					StringTerm("b"),
					StringTerm("c"),
				),
			)),
			Item(StringTerm("list_of_maps"), ArrayTerm(
				ObjectTerm(
					Item(StringTerm("one"), NumberTerm("1")),
					Item(StringTerm("two"), NumberTerm("2")),
				),
				ObjectTerm(
					Item(StringTerm("two"), NumberTerm("2")),
					Item(StringTerm("three"), NumberTerm("3")),
				),
			)),
			Item(StringTerm("map"), ObjectTerm(
				Item(StringTerm("nested_number"), NumberTerm("1")),
				Item(StringTerm("nested_map"), ObjectTerm(
					Item(StringTerm("do"), StringTerm("re")),
					Item(StringTerm("mi"), StringTerm("fa")),
				)),
				Item(StringTerm("nested_list"), ArrayTerm(
					NumberTerm("1"),
					NumberTerm("2"),
					NumberTerm("3"),
				)),
			)),
		)),
	)

	obj, err := annotations.toObject()
	if err != nil {
		t.Fatalf("unexpected error: %s", err.Error())
	}

	if Compare(*obj, expected) != 0 {
		t.Fatalf("object generated from annotations\n\n%v\n\ndoesn't match expected\n\n%v",
			*obj, expected)
	}
}

func toJSON(v interface{}) string {
	b, _ := json.MarshalIndent(v, "", "  ")
	return string(b)
}

func schemaAnnotationFromMap(path string, def map[string]interface{}) *SchemaAnnotation {
	var p interface{} = def
	return &SchemaAnnotation{Path: MustParseRef(path), Definition: &p}
}
