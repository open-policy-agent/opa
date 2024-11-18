// Copyright 2024 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

//go:build test_rego_default_v1_import
// +build test_rego_default_v1_import

package rego_default

import (
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/ast"
	_ "github.com/open-policy-agent/opa/features/rego_default_v1"
)

func TestDefaultRegoVersion(t *testing.T) {
	if ast.DefaultRegoVersion() != ast.RegoV1 {
		t.Fatal("expected default rego version to be v1")
	}
}

func TestParseModule(t *testing.T) {
	tests := []struct {
		note     string
		module   string
		expRules []string
		expErrs  []string
	}{
		{
			note: "v1 module",
			module: `package test

p contains x if {
	x in [1, 2, 3]
}`,
			expRules: []string{"p"},
		},
		{
			note: "v0 module",
			module: `package test

p[x] {
	x := [1, 2, 3][_]
}`,
			expErrs: []string{
				"test.rego:3: rego_parse_error: `if` keyword is required before rule body",
				"test.rego:3: rego_parse_error: `contains` keyword is required for partial set rules",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			mod, err := ast.ParseModule("test.rego", tc.module)

			if len(tc.expErrs) > 0 {
				if err == nil {
					t.Fatalf("expected error(s) %q, got nil", tc.expErrs)
				}

				for _, expErr := range tc.expErrs {
					if !strings.Contains(err.Error(), expErr) {
						t.Fatalf("expected error to contain:\n\n%s\n\ngot:\n\n%s", expErr, err)
					}
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %s", err)
				}

				if len(mod.Rules) != len(tc.expRules) {
					t.Fatalf("expected %d rules, got %d", len(tc.expRules), len(mod.Rules))
				}

				for i, rule := range mod.Rules {
					if rule.Head.Name.String() != tc.expRules[i] {
						t.Fatalf("expected rule %q, got %q", tc.expRules[i], rule.Head.Name.String())
					}
				}
			}

		})
	}

}
