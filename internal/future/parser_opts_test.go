// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package future

import (
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
)

func TestParserOptionsWithRegoVersionImports(t *testing.T) {
	regoV1 := &ast.Import{Path: ast.NewTerm(ast.RegoV1CompatibleRef)}
	regoV2 := &ast.Import{Path: ast.NewTerm(ast.RegoV2CompatibleRef)}
	other := &ast.Import{Path: ast.MustParseTerm("data.x")}

	tests := []struct {
		note        string
		regoVersion ast.RegoVersion
		imports     []*ast.Import
		exp         ast.RegoVersion
	}{
		{note: "undefined, no imports", regoVersion: ast.RegoUndefined, exp: ast.RegoUndefined},
		{note: "undefined, rego.v1", regoVersion: ast.RegoUndefined, imports: []*ast.Import{regoV1}, exp: ast.RegoV1},
		{note: "undefined, rego.v2", regoVersion: ast.RegoUndefined, imports: []*ast.Import{regoV2}, exp: ast.RegoV2},
		{note: "v0, no imports", regoVersion: ast.RegoV0, imports: []*ast.Import{other}, exp: ast.RegoV0},
		{note: "v0, rego.v1", regoVersion: ast.RegoV0, imports: []*ast.Import{regoV1}, exp: ast.RegoV1},
		{note: "v0, rego.v2", regoVersion: ast.RegoV0, imports: []*ast.Import{regoV2}, exp: ast.RegoV2},
		{note: "v0v1, rego.v1", regoVersion: ast.RegoV0CompatV1, imports: []*ast.Import{regoV1}, exp: ast.RegoV1},
		{note: "v1, rego.v1", regoVersion: ast.RegoV1, imports: []*ast.Import{regoV1}, exp: ast.RegoV1},
		{note: "v1, rego.v2", regoVersion: ast.RegoV1, imports: []*ast.Import{regoV2}, exp: ast.RegoV2},
		{note: "v1, rego.v2 and rego.v1", regoVersion: ast.RegoV1, imports: []*ast.Import{regoV2, regoV1}, exp: ast.RegoV2},
		{note: "v2, no imports", regoVersion: ast.RegoV2, exp: ast.RegoV2},
		{note: "v2, rego.v1", regoVersion: ast.RegoV2, imports: []*ast.Import{regoV1}, exp: ast.RegoV2},
		{note: "v2, rego.v2", regoVersion: ast.RegoV2, imports: []*ast.Import{regoV2}, exp: ast.RegoV2},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			popts := ParserOptionsWithRegoVersionImports(tc.imports, ast.ParserOptions{RegoVersion: tc.regoVersion})
			if popts.RegoVersion != tc.exp {
				t.Fatalf("expected %v, got %v", tc.exp, popts.RegoVersion)
			}
		})
	}
}
