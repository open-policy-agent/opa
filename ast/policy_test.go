// Copyright 2024 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import "testing"

func TestRuleString_DefaultRegoVersion(t *testing.T) {
	// ast.Rule.String() will respect the rego-version of the ast.Module it is part of.

	tests := []struct {
		note        string
		module      string
		regoVersion RegoVersion
		exp         string
	}{
		{
			note:        "v0",
			regoVersion: RegoV0,
			module: `package a.b.c

p[x] { x = "a" }`,
			exp: `p[x] { x = "a" }`,
		},
		{
			note:        "v1",
			regoVersion: RegoV1,
			module: `package a.b.c

p contains x if { x = "a" }`,
			exp: `p contains x if { x = "a" }`,
		},
		{
			note: "default rego-version",
			module: `package a.b.c

p[x] { x = "a" }`,
			exp: `p[x] { x = "a" }`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			var mod *Module

			if tc.regoVersion == RegoUndefined {
				mod = MustParseModule(tc.module)
			} else {
				mod = MustParseModuleWithOpts(tc.module, ParserOptions{RegoVersion: tc.regoVersion})
			}

			rule := mod.Rules[0]
			act := rule.String()

			if act != tc.exp {
				t.Fatalf("Expected:\n\n%s\n\nbut got:\n\n%s", tc.exp, act)
			}
		})
	}
}

func TestModuleString(t *testing.T) {

	// v0 module
	input := `package a.b.c

import data.foo.bar
import input.xyz

p = true { not bar }
q = true { xyz.abc = 2 }
wildcard = true { bar[_] = 1 }`

	mod := MustParseModule(input)

	roundtrip, err := ParseModule("", mod.String())
	if err != nil {
		t.Fatalf("Unexpected error while parsing roundtripped module: %v", err)
	}

	if !roundtrip.Equal(mod) {
		t.Fatalf("Expected roundtripped to equal original but:\n\nExpected:\n\n%v\n\nDoes not equal result:\n\n%v", mod, roundtrip)
	}
}
