// Copyright 2024 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast_test

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/format"
)

func TestParseModule_DefaultRegoVersion(t *testing.T) {
	tests := []struct {
		note     string
		mod      string
		expRules []string
		expErrs  []string
	}{
		{
			note: "v0", // NOT default rego-version
			mod: `package test
p[x] { 
	x = "a"
}`,
			expErrs: []string{
				"test.rego:2: rego_parse_error: `if` keyword is required before rule body",
				"test.rego:2: rego_parse_error: `contains` keyword is required for partial set rules",
			},
		},
		{
			note: "import rego.v1",
			mod: `package test
import rego.v1

p contains x if { 
	x = "a"
}`,
			expRules: []string{"p"},
		},
		{
			note: "v1", // default rego-version
			mod: `package test
p contains x if { 
	x = "a"
}`,
			expRules: []string{"p"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			m, err := ast.ParseModule("test.rego", tc.mod)

			if len(tc.expErrs) > 0 {
				for _, expErr := range tc.expErrs {
					if !strings.Contains(err.Error(), expErr) {
						t.Fatalf("Expected error to contain:\n\n%s\n\ngot:\n\n%s", expErr, err.Error())
					}
				}
			} else {
				if err != nil {
					t.Fatalf("Unexpected error: %s", err)
				}

				if len(m.Rules) != len(tc.expRules) {
					t.Fatalf("Expected %d rules, got %d", len(tc.expRules), len(m.Rules))
				}
				for i, r := range m.Rules {
					if r.Head.Name.String() != tc.expRules[i] {
						t.Fatalf("Expected rule %q, got %q", tc.expRules[i], r.Head.Name.String())
					}
				}
			}
		})
	}
}

func TestParseBody_DefaultRegoVersion(t *testing.T) {
	tests := []struct {
		note       string
		body       string
		expStmts   int
		assertSame bool
	}{
		{
			note: "v0", // default rego-version
			body: `x := ["a", "b", "c"][i]
`,
			expStmts:   1,
			assertSame: true,
		},
		{
			note: "v1", // NOT default rego-version
			body: `some x, i in ["a", "b", "c"]
`,
			expStmts:   1,
			assertSame: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			body, err := ast.ParseBody(tc.body)

			if err != nil {
				t.Fatalf("Unexpected error: %s", err)
			}

			if len(body) != tc.expStmts {
				t.Fatalf("Expected %d statements, got %d:%q\n\n", tc.expStmts, len(body), body)
			}

			if tc.assertSame {
				formatted, err := format.AstWithOpts(body, format.Opts{RegoVersion: ast.RegoV1}) // every body is v1-compatible
				if err != nil {
					t.Fatalf("Unexpected error: %s", err)
				}

				if strings.Compare(string(formatted), tc.body) != 0 {
					t.Fatalf("Expected body to be %q, got %q", tc.body, string(formatted))
				}
			}
		})
	}
}

func TestParseModuleDropLocations(t *testing.T) {
	src := `# METADATA
# title: pkg
# schemas:
#   - input: schema.x
# compile:
#   unknowns: [input.doc]
#   mask_rule: data.test.mask
package test

import data.foo as bar

# METADATA
# labels:
#   id: r1
p.q[k] contains v if {
	some k, v in input.xs
	every x in input.ys { x > 0 }
	not input.no
	count([y | y := input.zs[_]]) > 1 with input.zs as [1, 2]
	{a: b | a := 1; b := 2}
	{c | c := 1}
	bar.baz
}

r := 1 if { false } else := 2

f(x) := x + 1 # comment
`
	for _, processAnnotation := range []bool{true, false} {
		t.Run(fmt.Sprintf("ProcessAnnotation=%v", processAnnotation), func(t *testing.T) {
			mod, err := ast.ParseModuleWithOpts("test.rego", src, ast.ParserOptions{ProcessAnnotation: processAnnotation, DropLocations: true})
			if err != nil {
				t.Fatal(err)
			}
			if path := locationIn(reflect.ValueOf(mod), map[uintptr]bool{}, "mod"); path != "" {
				t.Errorf("location at %s", path)
			}
			if len(mod.Comments) != 11 || string(mod.Comments[10].Text) != " comment" {
				t.Errorf("comments: %v", mod.Comments)
			}
			if processAnnotation && (len(mod.Annotations) != 2 || mod.Rules[0].Annotations[0].Labels["id"] != "r1") {
				t.Errorf("annotations: %v", mod.Annotations)
			}
		})
	}
}

func locationIn(v reflect.Value, seen map[uintptr]bool, path string) string {
	switch v.Kind() {
	case reflect.Pointer:
		if v.IsNil() {
			return ""
		}
		if v.Type() == reflect.TypeFor[*ast.Location]() {
			return path
		}
		if seen[v.Pointer()] {
			return ""
		}
		seen[v.Pointer()] = true
		return locationIn(v.Elem(), seen, path)
	case reflect.Interface:
		return locationIn(v.Elem(), seen, path)
	case reflect.Struct:
		for i := range v.NumField() {
			if p := locationIn(v.Field(i), seen, path+"."+v.Type().Field(i).Name); p != "" {
				return p
			}
		}
	case reflect.Slice, reflect.Array:
		for i := range v.Len() {
			if p := locationIn(v.Index(i), seen, fmt.Sprintf("%s[%d]", path, i)); p != "" {
				return p
			}
		}
	case reflect.Map:
		for _, k := range v.MapKeys() {
			if p := locationIn(v.MapIndex(k), seen, fmt.Sprintf("%s[%v]", path, k)); p != "" {
				return p
			}
		}
	}
	return ""
}
