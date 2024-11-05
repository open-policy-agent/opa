// Copyright 2024 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package parser

import (
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/ast"
)

func TestParser(t *testing.T) {
	tests := []struct {
		note        string
		regoVersion ast.RegoVersion
		module      string
		expStmts    int
	}{
		// Default to v1
		{
			note: "undefined rego-version, v0 module",
			module: `package test
p {
	true
}`,
			regoVersion: ast.RegoUndefined,
			expStmts:    2, // package + rule
		},
		{
			note: "undefined rego-version, v1 module",
			module: `package test
p if {
	true
}`,
			regoVersion: ast.RegoUndefined,
			expStmts:    2, // package + rule
		},

		// v0
		{
			note: "v0 rego-version override, v0 module",
			module: `package test
p {
	true
}`,
			regoVersion: ast.RegoV0,
			expStmts:    2, // package + rule
		},
		{
			note: "v0 rego-version override, v1 module",
			module: `package test
p if {
	true
}`,
			regoVersion: ast.RegoV0,
			expStmts:    3, // package + rule*2 (if is interpreted as a rule, not a keyword)
		},

		// v1
		{
			note: "v1 rego-version override, v0 module",
			module: `package test
p {
	true
}`,
			regoVersion: ast.RegoV1,
			expStmts:    2, // package + rule
		},
		{
			note: "v1 rego-version override, v1 module",
			module: `package test
p if {
	true
}`,
			regoVersion: ast.RegoV1,
			expStmts:    2, // package + rule
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			opts := []Option{
				Reader(strings.NewReader(tc.module)),
				Capabilities(ast.CapabilitiesForThisVersion()),
			}

			if tc.regoVersion != ast.RegoUndefined {
				opts = append(opts, RegoVersion(tc.regoVersion))
			}

			p := NewParser(opts...)

			stmts, _, err := p.Parse()

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if len(stmts) != tc.expStmts {
				t.Fatalf("Expected %d statements but got %d", tc.expStmts, len(stmts))
			}
		})
	}
}
