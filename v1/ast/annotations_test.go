// Copyright 2022 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"encoding/json"
	"fmt"
	"maps"
	"runtime"
	"strings"
	"testing"
	"weak"
)

func TestEntrypointAnnotationScopeRequirements(t *testing.T) {
	tests := []struct {
		note        string
		module      string
		expectError bool
		expectScope string
	}{
		{
			note: "package scope explicit",
			module: `# METADATA
# entrypoint: true
# scope: package
package foo`,
			expectError: false,
			expectScope: "package",
		},
		{
			note: "package scope implied",
			module: `# METADATA
# entrypoint: true
package foo`,
			expectError: false,
			expectScope: "package",
		},
		{
			note: "subpackages scope explicit",
			module: `# METADATA
# entrypoint: true
# scope: subpackages
package foo`,
			expectError: true,
		},
		{
			note: "document scope explicit",
			module: `package foo
# METADATA
# entrypoint: true
# scope: document
foo := true`,
			expectError: false,
			expectScope: "document",
		},
		{
			note: "document scope implied",
			module: `package foo
# METADATA
# entrypoint: true
foo := true`,
			expectError: false,
			expectScope: "document",
		},
		{
			note: "rule scope explicit",
			module: `package foo
# METADATA
# entrypoint: true
# scope: rule
foo := true`,
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			module, err := ParseModuleWithOpts("test.rego", tc.module, ParserOptions{ProcessAnnotation: true})
			if err != nil {
				if !tc.expectError {
					t.Errorf("unexpected error: %v", err)
				}
				return
			}
			if tc.expectError {
				t.Fatalf("expected error")
			}
			if tc.expectScope != module.Annotations[0].Scope {
				t.Fatalf("expected scope %q, got %q", tc.expectScope, module.Annotations[0].Scope)
			}
		})
	}

}

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
							schemaAnnotationFromMap("input", map[string]any{
								"type": "boolean",
							}),
						},
						Custom: map[string]any{
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
							schemaAnnotationFromMap("input", map[string]any{
								"type": "integer",
							}),
						},
						Custom: map[string]any{
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
							schemaAnnotationFromMap("input", map[string]any{
								"type": "string",
							}),
						},
						Custom: map[string]any{
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
import rego.v1

# METADATA
# title: P1
p contains v if {v = 1}

# METADATA
# title: P2
p contains v if {v = 2}`,
			},
			expected: []AnnotationsRef{
				{
					Path:     MustParseRef("data.test.p"),
					Location: &Location{File: "mod", Row: 6},
					Annotations: &Annotations{
						Scope: "rule",
						Title: "P1",
					},
				},
				{
					Path:     MustParseRef("data.test.p"),
					Location: &Location{File: "mod", Row: 10},
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
import rego.v1

# METADATA
# title: P1
p contains v if {v = 1}`,
				"mod2": `package test
import rego.v1

# METADATA
# title: P2
p contains v if {v = 2}`,
			},
			expected: []AnnotationsRef{
				{
					Path:     MustParseRef("data.test.p"),
					Location: &Location{File: "mod1", Row: 6},
					Annotations: &Annotations{
						Scope: "rule",
						Title: "P1",
					},
				},
				{
					Path:     MustParseRef("data.test.p"),
					Location: &Location{File: "mod2", Row: 6},
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
import rego.v1

# METADATA
# title: P1
b.c.p[v] if {v = 1}`,
				"mod2": `package test
import rego.v1

# METADATA
# title: P2
a.b.c.p[v] if {v = 2}`,
			},
			expected: []AnnotationsRef{
				{
					Path:     MustParseRef("data.test.a.b.c.p"),
					Location: &Location{File: "mod1", Row: 6},
					Annotations: &Annotations{
						Scope: "rule",
						Title: "P1",
					},
				},
				{
					Path:     MustParseRef("data.test.a.b.c.p"),
					Location: &Location{File: "mod2", Row: 6},
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
							schemaAnnotationFromMap("input.baz", map[string]any{
								"type": "string",
							}),
						},
						Custom: map[string]any{
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
							schemaAnnotationFromMap("input.bar", map[string]any{
								"type": "integer",
							}),
						},
						Custom: map[string]any{
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
							schemaAnnotationFromMap("input.foo", map[string]any{
								"type": "boolean",
							}),
						},
						Custom: map[string]any{
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

// Rules supplied by an ExternalRuleSource are not tracked in the outer
// compiler's AnnotationSet. Chain must fall back to rule.Annotations so that
// labels (and other annotation-driven features) remain reachable for them.
func TestAnnotationSet_Chain_FallbackToRuleAnnotations(t *testing.T) {
	mod := MustParseModuleWithOpts(`package test

# METADATA
# scope: document
# title: doc
# labels:
#   component: authz

# METADATA
# labels:
#   id: allow-admin
allow if input.role == "admin"
`, ParserOptions{ProcessAnnotation: true})

	rule := mod.Rules[0]
	if len(rule.Annotations) != 2 {
		t.Fatalf("expected rule.Annotations to carry rule+document scope (2 entries), got %d", len(rule.Annotations))
	}

	as, errs := BuildAnnotationSet(nil) // empty: rule is not tracked
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}

	chain := as.Chain(rule)
	if len(chain) != 2 {
		t.Fatalf("expected 2 entries from rule.Annotations fallback, got %d: %v", len(chain), toJSON(chain))
	}

	var sawRule, sawDoc bool
	for _, ref := range chain {
		if ref.Annotations == nil {
			t.Fatalf("expected non-nil Annotations on fallback ref, got placeholder: %v", toJSON(ref))
		}
		switch ref.Annotations.Scope {
		case annotationScopeRule:
			sawRule = true
			if got := ref.Annotations.Labels["id"]; got != "allow-admin" {
				t.Errorf("rule-scope id label: want %q, got %v", "allow-admin", got)
			}
		case annotationScopeDocument:
			sawDoc = true
			if got := ref.Annotations.Labels["component"]; got != "authz" {
				t.Errorf("document-scope component label: want %q, got %v", "authz", got)
			}
		}
	}
	if !sawRule || !sawDoc {
		t.Errorf("expected rule- and document-scope entries; sawRule=%v sawDoc=%v", sawRule, sawDoc)
	}
}

// MergedLabels must work for rules whose source module isn't tracked by the
// AnnotationSet, mirroring the Chain fallback behavior so that decision-log
// label aggregation keeps working for ExternalRuleSource rules.
func TestAnnotationSet_MergedLabels_ExternalRule(t *testing.T) {
	mod := MustParseModuleWithOpts(`package test

# METADATA
# scope: document
# labels:
#   component: authz
#   severity: low

# METADATA
# labels:
#   id: allow-admin
#   severity: high
allow if input.role == "admin"
`, ParserOptions{ProcessAnnotation: true})

	rule := mod.Rules[0]

	as, errs := BuildAnnotationSet(nil) // empty: rule is not tracked
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}

	want := map[string]any{
		"component": "authz",
		"id":        "allow-admin",
		"severity":  "high", // rule scope overrides document scope
	}

	labels, key := as.MergedLabels(rule)
	if !maps.Equal(labels, want) {
		t.Errorf("first call: labels = %v, want %v", labels, want)
	}
	if key == "" {
		t.Error("first call: key should be non-empty for a rule with labels")
	}

	// Second call must return the cached entry.
	labels2, key2 := as.MergedLabels(rule)
	if !maps.Equal(labels2, want) {
		t.Errorf("second call: labels = %v, want %v", labels2, want)
	}
	if key2 != key {
		t.Errorf("second call: key changed: %q -> %q", key, key2)
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
			schemaAnnotationFromMap("input.bar", map[string]any{
				"type": "boolean",
			}),
		},
		Custom: map[string]any{
			"number": 42,
			"float":  2.2,
			"string": "foo bar baz",
			"bool":   true,
			"list": []any{
				"a", "b",
			},
			"list_of_lists": []any{
				[]any{
					"a", "b",
				},
				[]any{
					"b", "c",
				},
			},
			"list_of_maps": []any{
				map[string]any{
					"one": 1,
					"two": 2,
				},
				map[string]any{
					"two":   2,
					"three": 3,
				},
			},
			"map": map[string]any{
				"nested_number": 1,
				"nested_map": map[string]any{
					"do": "re",
					"mi": "fa",
				},
				"nested_list": []any{
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

	term, err := annotations.toTerm()
	if err != nil {
		t.Fatalf("unexpected error: %s", err.Error())
	}

	if term.Value.Compare(expected) != 0 {
		t.Fatalf("object generated from annotations\n\n%v\n\ndoesn't match expected\n\n%v", term, expected)
	}
}

func toJSON(v any) string {
	b, _ := json.MarshalIndent(v, "", "  ")
	return string(b)
}

func schemaAnnotationFromMap(path string, def map[string]any) *SchemaAnnotation {
	var p any = def
	return &SchemaAnnotation{Path: MustParseRef(path), Definition: &p}
}

// TestAnnotationSet_MergedLabels_Collectable verifies that an AnnotationSet
// can be garbage-collected after it goes out of scope, even when MergedLabels
// has been called (which populates the internal mergedLabels cache).
//
// Regression test for a memory leak where the cleanup closure registered by
// runtime.AddCleanup captured the AnnotationSet itself. The AnnotationSet
// holds strong references to all modules (and their rules) via as.modules, so
// the cleanup could never fire: rule → cleanup closure → AnnotationSet →
// modules → rule. Each bundle reload left the old AnnotationSet permanently
// alive, causing unbounded heap growth.
func TestAnnotationSet_MergedLabels_Collectable(t *testing.T) {
	const src = `package test

# METADATA
# labels:
#   tier: fast
allow if true
`
	var watch weak.Pointer[AnnotationSet]

	func() {
		mod := MustParseModuleWithOpts(src, ParserOptions{ProcessAnnotation: true})
		as, errs := BuildAnnotationSet([]*Module{mod})
		if len(errs) > 0 {
			t.Fatalf("BuildAnnotationSet: %v", errs)
		}
		// Populate the mergedLabels cache for all rules.
		for _, r := range mod.Rules {
			as.MergedLabels(r)
		}
		watch = weak.Make(as)
		// mod and as go out of scope here.
	}()

	// Two GC cycles: one to discover unreachable objects, one to collect them.
	runtime.GC()
	runtime.GC()

	if watch.Value() != nil {
		t.Fatal("AnnotationSet was not garbage-collected: mergedLabels cache likely holds a retaining cycle")
	}
}

func TestAnnotations_StringDeterministic(t *testing.T) {
	a := &Annotations{
		Scope:       "rule",
		Description: "<b>&</b>",
		Custom: map[string]any{
			"zeta": 1, "alpha": 2, "mu": 3, "beta": 4, "omega": 5,
		},
	}

	exp := a.String()
	for i := range 10 {
		if got := a.String(); got != exp {
			t.Fatalf("String() is not deterministic across calls:\ncall 0: %s\ncall %d: %s", exp, i+1, got)
		}
	}

	if raw := "<b>&</b>"; strings.Contains(exp, raw) {
		t.Fatalf("expected HTML characters to be escaped, but found raw %s in %s", raw, exp)
	}
	if escaped := `\u003cb\u003e\u0026\u003c/b\u003e`; !strings.Contains(exp, escaped) {
		t.Fatalf("expected HTML characters to be escaped as %s, got %s", escaped, exp)
	}
}

func BenchmarkAnnotationToTerm(b *testing.B) {
	annotations := &Annotations{
		Entrypoint:  true,
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
				Email: "foo@example.com",
			},
		},
		Custom: map[string]any{
			"number": 42,
			"float":  2.2,
			"string": "a",
			"bool":   true,
			"list": []any{
				"a", "b",
			},
		},
		Labels: map[string]any{
			"tier": "fast",
			"foo":  "bar",
		},
		Location: &Location{
			File: "module",
			Row:  42,
		},
	}

	for b.Loop() {
		if _, err := annotations.toTerm(); err != nil {
			b.Fatalf("unexpected error: %s", err.Error())
		}
	}
}
