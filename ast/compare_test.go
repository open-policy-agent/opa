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
