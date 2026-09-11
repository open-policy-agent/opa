// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package rego

import (
	"fmt"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
	inmem "github.com/open-policy-agent/opa/v1/storage/inmem"
)

// BenchmarkIndexedRulesetEval evaluates a ruleset of the shape a "one rule per
// group" policy has: every rule reads a collection of its own and shares the
// condition that actually holds, so the index cannot narrow the set and every rule
// is evaluated.
//
// The rule index benchmarks in v1/ast time Build and Lookup on their own. This one
// times the whole query, so it shows what the trie's shape costs a caller: every
// group ref here is a level, all of them resolve, and traversal used to walk a
// private copy of the remaining levels under each one.
func BenchmarkIndexedRulesetEval(b *testing.B) {
	ctx := b.Context()

	for _, n := range []int{100, 500} {
		b.Run(fmt.Sprintf("groups=%d", n), func(b *testing.B) {
			module, data := indexedRuleset(n, 20)

			r := New(
				ParsedQuery(ast.MustParseBody("data.test.allow = x")),
				ParsedModule(ast.MustParseModule(module)),
				Store(inmem.NewFromObject(data)),
				GenerateJSON(noOpGenerateJSON),
			)

			pq, err := r.PrepareForEval(ctx)
			if err != nil {
				b.Fatal(err)
			}

			input := ast.MustParseTerm(fmt.Sprintf(`{"subject": "u%d_19", "resource": {"foo": "A"}}`, n-1))

			// The subject is a member of the last group only, so exactly one rule
			// holds however many of them the index left in the running.
			if rs, err := pq.Eval(ctx, EvalParsedInput(input.Value)); err != nil {
				b.Fatal(err)
			} else if len(rs) != 1 {
				b.Fatalf("expected one result, got %d", len(rs))
			}

			b.ResetTimer()
			for b.Loop() {
				if _, err := pq.Eval(ctx, EvalParsedInput(input.Value)); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// indexedRuleset returns n rules that each test membership in a group of their
// own, and the group data they read. Every rule also shares a condition that
// holds, so nothing in the ruleset discriminates.
func indexedRuleset(n, members int) (string, map[string]any) {
	var sb strings.Builder
	sb.WriteString("package test\n\n")

	groups := make(map[string]any, n)
	for i := range n {
		fmt.Fprintf(&sb, "allow if {\n\tinput.subject in data.groups.g%d.members\n\tinput.resource.foo == \"A\"\n}\n\n", i)

		ms := make([]any, members)
		for j := range members {
			ms[j] = fmt.Sprintf("u%d_%d", i, j)
		}
		groups[fmt.Sprintf("g%d", i)] = map[string]any{"members": ms}
	}

	return sb.String(), map[string]any{"groups": groups}
}
