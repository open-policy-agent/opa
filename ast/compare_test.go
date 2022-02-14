// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"testing"
)

func TestCompare(t *testing.T) {

	// Many of the comparison cases are covered by existing equality tests. Here
	// we cover edge cases.
	tests := []struct {
		a        string
		b        string
		expected int
	}{
		// Comparisons to Go nil. Everything is greater than nil and nil is equal to nil
		{"null", "", 1},
		{"", "null", -1},
		{"", "", 0},

		// Booleans
		{"false", "true", -1},
		{"true", "false", 1},

		// Numbers
		{"0", "1", -1},
		{"1", "0", 1},
		{"0", "0", 0},
		{"0", "1.5", -1},
		{"1.5", "0", 1},
		{"123456789123456789123", "123456789123456789123", 0},
		{"123456789123456789123", "123456789123456789122", 1},
		{"123456789123456789122", "123456789123456789123", -1},
		{"123456789123456789123.5", "123456789123456789123.5", 0},
		{"123456789123456789123.5", "123456789123456789122.5", 1},
		{"123456789123456789122.5", "123456789123456789123.5", -1},
		{"630E-840354372", "0", 0},

		// Object comparisons are consistent
		{`{1: 2, 3: 4}`, `{4: 3, 1: 2}`, -1},
		{`{1: 2, 3: 4}`, `{1: 2, 4: 3}`, -1},
		{`{1: 2, 3: 4}`, `{1: 2, 3: 5}`, -1},
		{`{1: 2, 3: 4}`, `{1: 2, 3: 4, 5: 6}`, -1},
		{`{1: 2, 3: 4, 5: 6}`, `{1: 2, 3: 4}`, 1},

		// Comprehensions
		{`[null | true]`, `[false | null]`, -1},
		{`{null | true}`, `{false | null}`, -1},
		{`{"abc": null | true}`, `{"cba": false | null}`, -1},

		// Expressions
		{`a = b`, `b = a`, -1},
		{`b = a`, `not a = b`, -1},
		{`a = b`, `x`, 1},
		{`a = b`, `a = b with input.foo as bar`, -1},
		{`a = b with input.foo as bar`, `a = b`, 1},
		{`a = b with input.foo as bar`, `a = b with input.foo.bar.baz as qux`, -1},
		{`a = b with input.foo as bar`, `a = b with input.foo as bar with input.baz as qux`, -1},

		// Body
		{`a = b`, `a = b; b = a`, -1},
		{`a = b; b = a`, `a = b`, 1},
	}
	for _, tc := range tests {
		var a, b interface{}
		if len(tc.a) > 0 {
			a = MustParseStatement(tc.a)
		}
		if len(tc.b) > 0 {
			b = MustParseStatement(tc.b)
		}
		result := Compare(a, b)
		if tc.expected != result {
			t.Errorf("Expected %v.Compare(%v) == %v but got %v", a, b, tc.expected, result)
		}
	}
}

func TestCompareModule(t *testing.T) {
	a := MustParseModule(`package a.b.c`)
	b := MustParseModule(`package a.b.d`)
	result := Compare(a, b)

	if result != -1 {
		t.Errorf("Expected %v to be less than %v but got: %v", a, b, result)
	}

	a = MustParseModule(`package a.b.c

import input.x.y`)
	b = MustParseModule(`package a.b.c

import input.x.z`)
	result = Compare(a, b)

	if result != -1 {
		t.Errorf("Expected %v to be less than %v but got: %v", a, b, result)
	}

	var err error

	a, err = ParseModuleWithOpts("test.rego", `package a

# METADATA
# scope: rule
# schemas:
# - input: schema.a
p := 7`, ParserOptions{ProcessAnnotation: true})
	if err != nil {
		t.Fatal(err)
	}

	b, err = ParseModuleWithOpts("test.rego", `package a

# METADATA
# scope: rule
# schemas:
# - input: schema.b
p := 7`, ParserOptions{ProcessAnnotation: true})
	if err != nil {
		t.Fatal(err)
	}

	result = Compare(a, b)

	if result != -1 {
		t.Errorf("Expected %v to be less than %v but got: %v", a, b, result)
	}
}

func TestCompareAnnotations(t *testing.T) {

	tests := []struct {
		note string
		a    string
		b    string
		exp  int
	}{
		{
			note: "same",
			a: `
# METADATA
# scope: a`,
			b: `
# METADATA
# scope: a`,
			exp: 0,
		},
		{
			note: "unknown scope",
			a: `
# METADATA
# scope: rule`,
			b: `
# METADATA
# scope: a`,
			exp: 1,
		},
		{
			note: "unknown scope - less than",
			a: `
# METADATA
# scope: a`,
			b: `
# METADATA
# scope: rule`,
			exp: -1,
		},
		{
			note: "unknown scope - greater than - lexigraphical",
			a: `
# METADATA
# scope: b`,
			b: `
# METADATA
# scope: a`,
			exp: 1,
		},
		{
			note: "unknown scope - less than - lexigraphical",
			a: `
# METADATA
# scope: b`,
			b: `
# METADATA
# scope: c`,
			exp: -1,
		},
		{
			note: "schema",
			a: `
# METADATA
# scope: rule
# schemas:
# - input: schema`,
			b: `
# METADATA
# scope: rule
# schemas:
# - input: schema`,
			exp: 0,
		},
		{
			note: "schema - less than",
			a: `
# METADATA
# scope: rule
# schemas:
# - input.a: schema`,
			b: `
# METADATA
# scope: rule
# schemas:
# - input.b: schema`,
			exp: -1,
		},
		{
			note: "schema - greater than",
			a: `
# METADATA
# scope: rule
# schemas:
# - input.b: schema`,
			b: `
# METADATA
# scope: rule
# schemas:
# - input.a: schema`,
			exp: 1,
		},
		{
			note: "schema - less than (fewer)",
			a: `
# METADATA
# scope: rule
# schemas:
# - input.a: schema`,
			b: `
# METADATA
# scope: rule
# schemas:
# - input.a: schema
# - input.b: schema`,
			exp: -1,
		},
		{
			note: "schema - greater than (more)",
			a: `
# METADATA
# scope: rule
# schemas:
# - input.a: schema
# - input.b: schema`,
			b: `
# METADATA
# scope: rule
# schemas:
# - input.a: schema`,
			exp: 1,
		},
		{
			note: "schema - less than - lexigraphical",
			a: `
# METADATA
# scope: rule
# schemas:
# - input: schema.a`,
			b: `
# METADATA
# scope: rule
# schemas:
# - input: schema.b`,
			exp: -1,
		},
		{
			note: "schema - greater than - lexigraphical",
			a: `
# METADATA
# scope: rule
# schemas:
# - input: schema.c`,
			b: `
# METADATA
# scope: rule
# schemas:
# - input: schema.b`,
			exp: 1,
		},
		{
			note: "definition",
			a: `
# METADATA
# schemas:
# - input: {"type": "string"}`,
			b: `
# METADATA
# schemas:
# - input: {"type": "string"}`,
		},
		{
			note: "definition - less than schema",
			a: `
# METADATA
# schemas:
# - input: {"type": "string"}`,
			b: `
# METADATA
# schemas:
# - input: schema.a`,
			exp: -1,
		},
		{
			note: "schema - greater than definition",
			a: `
# METADATA
# schemas:
# - input: schema.a`,
			b: `
# METADATA
# schemas:
# - input: {"type": "string"}`,
			exp: 1,
		},
		{
			note: "title",
			a: `
# METADATA
# title: a`,
			b: `
# METADATA
# title: a`,
			exp: 0,
		},
		{
			note: "title - less than",
			a: `
# METADATA
# title: a`,
			b: `
# METADATA
# title: b`,
			exp: -1,
		},
		{
			note: "title - greater than",
			a: `
# METADATA
# title: b`,
			b: `
# METADATA
# title: a`,
			exp: 1,
		},
		{
			note: "description",
			a: `
# METADATA
# description: a`,
			b: `
# METADATA
# description: a`,
			exp: 0,
		},
		{
			note: "description - less than",
			a: `
# METADATA
# description: a`,
			b: `
# METADATA
# description: b`,
			exp: -1,
		},
		{
			note: "description - greater than",
			a: `
# METADATA
# description: b`,
			b: `
# METADATA
# description: a`,
			exp: 1,
		},
		{
			note: "authors",
			a: `
# METADATA
# authors: 
# - John Doe
# - Jane Doe`,
			b: `
# METADATA
# authors: 
# - John Doe
# - Jane Doe`,
			exp: 0,
		},
		{
			note: "authors - less than",
			a: `
# METADATA
# authors: 
# - Jane Doe
# - John Doe
`,
			b: `
# METADATA
# authors: 
# - John Doe
# - Jane Doe`,
			exp: -1,
		},
		{
			note: "authors - greater than",
			a: `
# METADATA
# authors: 
# - John Doe
# - Jane Doe`,
			b: `
# METADATA
# authors: 
# - Jane Doe
# - John Doe`,
			exp: 1,
		},
		{
			note: "authors - less than (fewer)",
			a: `
# METADATA
# scope: rule
# authors:
# - John Doe`,
			b: `
# METADATA
# scope: rule
# authors:
# - John Doe
# - Jane Doe`,
			exp: -1,
		},
		{
			note: "authors - greater than (more)",
			a: `
# METADATA
# scope: rule
# authors:
# - John Doe
# - Jane Doe`,
			b: `
# METADATA
# scope: rule
# authors:
# - John Doe`,
			exp: 1,
		},
		{
			note: "authors - less than (email)",
			a: `
# METADATA
# authors: 
# - John Doe <a@example.com>`,
			b: `
# METADATA
# authors: 
# - John Doe <b@example.com>`,
			exp: -1,
		},
		{
			note: "authors - greater than (email)",
			a: `
# METADATA
# authors: 
# - John Doe <b@example.com>`,
			b: `
# METADATA
# authors: 
# - John Doe <a@example.com>`,
			exp: 1,
		},
		{
			note: "organizations",
			a: `
# METADATA
# organizations: 
# - a
# - b`,
			b: `
# METADATA
# organizations: 
# - a
# - b`,
			exp: 0,
		},
		{
			note: "organizations - less than",
			a: `
# METADATA
# organizations: 
# - a
# - b`,
			b: `
# METADATA
# organizations: 
# - c
# - d`,
			exp: -1,
		},
		{
			note: "organizations - greater than",
			a: `
# METADATA
# organizations: 
# - c
# - d`,
			b: `
# METADATA
# organizations: 
# - a
# - b`,
			exp: 1,
		},
		{
			note: "organizations - less than (fewer)",
			a: `
# METADATA
# scope: rule
# organizations:
# - a`,
			b: `
# METADATA
# scope: rule
# organizations:
# - a
# - b`,
			exp: -1,
		},
		{
			note: "organizations - greater than (more)",
			a: `
# METADATA
# scope: rule
# organizations:
# - a
# - b`,
			b: `
# METADATA
# scope: rule
# organizations:
# - a`,
			exp: 1,
		},
		{
			note: "related_resources",
			a: `
# METADATA
# related_resources: 
# - https://a.example.com
# - 
#  ref: https://b.example.com
#  description: foo bar`,
			b: `
# METADATA
# related_resources: 
# - https://a.example.com
# - 
#  ref: https://b.example.com
#  description: foo bar`,
			exp: 0,
		},
		{
			note: "related_resources - less than",
			a: `
# METADATA
# related_resources: 
# - https://a.example.com
# - https://b.example.com`,
			b: `
# METADATA
# related_resources: 
# - https://b.example.com
# - https://c.example.com`,
			exp: -1,
		},
		{
			note: "related_resources - greater than",
			a: `
# METADATA
# related_resources: 
# - https://b.example.com
# - https://c.example.com`,
			b: `
# METADATA
# related_resources: 
# - https://a.example.com
# - https://b.example.com`,
			exp: 1,
		},
		{
			note: "related_resources - less than (fewer)",
			a: `
# METADATA
# scope: rule
# organizations:
# - https://a.example.com`,
			b: `
# METADATA
# scope: rule
# organizations:
# - https://a.example.com
# - https://b.example.com`,
			exp: -1,
		},
		{
			note: "related_resources - greater than (more)",
			a: `
# METADATA
# scope: rule
# organizations:
# - https://a.example.com
# - https://b.example.com`,
			b: `
# METADATA
# scope: rule
# organizations:
# - https://a.example.com`,
			exp: 1,
		},
		{
			note: "related_resources - less than (description)",
			a: `
# METADATA
# related_resources:
# -
#  ref: https://example.com
#  description: a`,
			b: `
# METADATA
# related_resources:
# -
#  ref: https://example.com
#  description: b`,
			exp: -1,
		},
		{
			note: "related_resources - greater than (description)",
			a: `
# METADATA
# related_resources:
# -
#  ref: https://example.com
#  description: b`,
			b: `
# METADATA
# related_resources:
# -
#  ref: https://example.com
#  description: a`,
			exp: 1,
		},
		{
			note: "custom",
			a: `
# METADATA
# custom: 
#  a: 1
#  b: true
#  c:
#  d:
#  - 1
#  - 2
#  e:
#   i: 1
#   j: 2`,
			b: `
# METADATA
# custom: 
#  a: 1
#  b: true
#  c:
#  d:
#  - 1
#  - 2
#  e:
#   i: 1
#   j: 2`,
			exp: 0,
		},
		{
			note: "custom - less than",
			a: `
# METADATA
# custom: 
#  a: 1`,
			b: `
# METADATA
# custom: 
#  b: 1`,
			exp: -1,
		},
		{
			note: "custom - greater than",
			a: `
# METADATA
# custom: 
#  b: 1`,
			b: `
# METADATA
# custom: 
#  a: 1`,
			exp: 1,
		},
		{
			note: "custom - less than (value)",
			a: `
# METADATA
# custom: 
#  a: 1`,
			b: `
# METADATA
# custom: 
#  a: 2`,
			exp: -1,
		},
		{
			note: "custom - greater than (value)",
			a: `
# METADATA
# custom: 
#  a: 2`,
			b: `
# METADATA
# custom: 
#  a: 1`,
			exp: 1,
		},
		{
			note: "custom - less than (fewer)",
			a: `
# METADATA
# custom: 
#  a: 1`,
			b: `
# METADATA
# custom: 
#  a: 1
#  b: 2`,
			exp: -1,
		},
		{
			note: "custom - greater than (more)",
			a: `
# METADATA
# custom: 
#  a: 1
#  b: 2`,
			b: `
# METADATA
# custom: 
#  a: 1`,
			exp: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			stmts, _, err := ParseStatementsWithOpts("test.rego", tc.a, ParserOptions{ProcessAnnotation: true})
			if err != nil {
				t.Fatal(err)
			}
			a := stmts[0].(*Annotations)
			stmts, _, err = ParseStatementsWithOpts("test.rego", tc.b, ParserOptions{ProcessAnnotation: true})
			if err != nil {
				t.Fatal(err)
			}
			b := stmts[0].(*Annotations)
			result := a.Compare(b)
			if result != tc.exp {
				t.Fatalf("Expected %d but got %v for %v and %v", tc.exp, result, a, b)
			}
		})
	}
}
