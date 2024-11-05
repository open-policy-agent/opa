// Copyright 2024 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package parser

import (
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/util/test"
)

func TestParseModule(t *testing.T) {
	tests := []struct {
		note        string
		regoVersion ast.RegoVersion
		module      string
		expRules    []string
		expErr      string
	}{
		// Default to v1
		{
			note: "undefined rego-version, v0 module",
			module: `package test
p := x {
	x := 42
}`,
			regoVersion: ast.RegoUndefined,
			expErr:      "rego_parse_error: `if` keyword is required before rule body",
		},
		{
			note: "undefined rego-version, rego.v1 import",
			module: `package test
import rego.v1

p := x if {
	x := 42
}`,
			regoVersion: ast.RegoUndefined,
			expRules:    []string{"p"},
		},
		{
			note: "undefined rego-version, v1 module",
			module: `package test
p := x if {
	x := 42
}`,
			regoVersion: ast.RegoUndefined,
			expRules:    []string{"p"},
		},

		// Override to v0
		{
			note: "v0 rego-version override, v0 module",
			module: `package test
p := x {
	x := 42
}`,
			regoVersion: ast.RegoV0,
			expRules:    []string{"p"},
		},
		{
			note: "v0 rego-version override, rego.v1 import",
			module: `package test
import rego.v1

p := x if {
	x := 42
}`,
			regoVersion: ast.RegoV0,
			expRules:    []string{"p"},
		},
		{
			note: "v0 rego-version override, v1 module",
			module: `package test
p := x if {
	x := 42
}`,
			regoVersion: ast.RegoV0,
			// 'if' is interpreted as a rule name, and not a keyword, in v0
			expRules: []string{"p", "if"},
		},

		// Override to v1 (same as default)
		{
			note: "v1 rego-version override, v0 module",
			module: `package test
p := x {
	x := 42
}`,
			regoVersion: ast.RegoV1,
			expErr:      "rego_parse_error: `if` keyword is required before rule body",
		},
		{
			note: "v1 rego-version override, rego.v1 import",
			module: `package test
import rego.v1

p := x if {
	x := 42
}`,
			regoVersion: ast.RegoV1,
			expRules:    []string{"p"},
		},
		{
			note: "v1 rego-version override, v1 module",
			module: `package test
p := x if {
	x := 42
}`,
			regoVersion: ast.RegoV1,
			expRules:    []string{"p"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			opts := ParserOptions{}
			if tc.regoVersion != ast.RegoUndefined {
				opts.RegoVersion = tc.regoVersion
			}

			m, err := ParseModuleWithOpts("test.rego", tc.module, opts)

			if tc.expErr != "" {
				if err == nil {
					t.Fatalf("expected error but got nil")
				}
				if !strings.Contains(err.Error(), tc.expErr) {
					test.FatalMismatch(t, err, tc.expErr)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %s", err)
				}
				if m == nil {
					t.Fatalf("expected non-nil module")
				}

				if len(m.Rules) != len(tc.expRules) {
					t.Fatalf("expected %d rules but got %d", len(tc.expRules), len(m.Rules))
				}
				for _, r := range m.Rules {
					found := false
					for _, exp := range tc.expRules {
						if r.Head.Name.String() == exp {
							found = true
							break
						}
					}
					if !found {
						t.Fatalf("expected rules %v but got %s", tc.expRules, r.Head.Name)
					}
				}
			}
		})
	}
}
