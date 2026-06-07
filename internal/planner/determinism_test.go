// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package planner

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
)

// planToJSON is a helper function that plans an entrypoint using a slice of
// compiled modules, and returns the resulting IR policy as JSON bytes.
// compiler must be the ast.Compiler that produced the modules.
func planToJSON(t *testing.T, compiler *ast.Compiler, entrypoint string, modules []*ast.Module) []byte {
	t.Helper()

	// Build the entrypoint query. Query is: `result = data.<entrypoint>`
	resultSym := ast.VarTerm("result")
	ep := ast.MustParseRef("data." + entrypoint)
	qc := compiler.QueryCompiler()
	compiled, err := qc.Compile(ast.NewBody(ast.Equality.Expr(resultSym, ast.NewTerm(ep))))
	if err != nil {
		t.Fatal(err)
	}

	p := New().
		WithQueries([]QuerySet{
			{
				Name:          entrypoint,
				Queries:       []ast.Body{compiled},
				RewrittenVars: qc.RewrittenVars(),
			},
		}).
		WithModules(modules).
		WithBuiltinDecls(ast.BuiltinMap)

	policy, err := p.Plan()
	if err != nil {
		t.Fatal(err)
	}

	bs, err := json.Marshal(policy)
	if err != nil {
		t.Fatal(err)
	}
	return bs
}

// TestPlannerDeterministicRuleOrder is a regression test ensuring that the
// planner's outputs do not depend on ordering of the rules provided to it.
//
// This test compiles a module once, and then plans it twice, with the Rules
// slice in two different orders. This allows detecting if the planner is
// not iterating over the rule trie in a deterministic ordering. We use
// >12 rules because Go's sort.Slice uses a stable insertion sort for <=12
// elements and an unstable pdqsort above that.
//
// Warning: This test relies on implementation details of the Golang default
// sorting algorithm. If that algorithm changes, this test might no longer
// accurately exercise unstable sorting algorithm issues.
func TestPlannerDeterministicRuleOrder(t *testing.T) {
	const n = 32 // > 12 to force the unstable pdqsort path in the default sort.

	var src strings.Builder
	src.WriteString("package authz\nimport rego.v1\n")
	for i := range n {
		fmt.Fprintf(&src, "p.field%02d := %d\n", i, i)
	}
	// A parent ref-head rule (defined last) so the trie node for p accumulates
	// the 'field' children before the parent node is inserted into the rule trie.
	src.WriteString("p[k] := v if { k := input.k; v := input.v }\n")

	m, err := ast.ParseModuleWithOpts("mod.rego", src.String(), ast.ParserOptions{AllFutureKeywords: true})
	if err != nil {
		t.Fatal(err)
	}

	compiler := ast.NewCompiler()
	compiler.Compile(map[string]*ast.Module{"mod.rego": m})
	if compiler.Failed() {
		t.Fatalf("compile failed: %v", compiler.Errors)
	}
	compiled := compiler.Modules["mod.rego"]

	planFrom := func(rules []*ast.Rule) []byte {
		clone := compiled.Copy()
		clone.Rules = rules
		return planToJSON(t, compiler, "authz.p", []*ast.Module{clone})
	}

	forwardRules := slices.Clone(compiled.Rules)
	reversedRules := slices.Clone(compiled.Rules)
	slices.Reverse(reversedRules)

	forward := planFrom(forwardRules)
	backward := planFrom(reversedRules)

	if !bytes.Equal(forward, backward) {
		t.Fatalf("plan IR depends on rule order:\nforward=%s\n\nreversed=%s", forward, backward)
	}
}
