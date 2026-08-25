// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

func compileForInlining(t *testing.T, modules map[string]string) *Compiler {
	t.Helper()

	parsed := map[string]*Module{}
	for name, src := range modules {
		parsed[name] = MustParseModule(src)
	}

	c := NewCompiler()
	if c.Compile(parsed); c.Failed() {
		t.Fatalf("compile failed: %v", c.Errors)
	}

	return c
}

func bodyRefs(t *testing.T, c *Compiler, module, rule string) []string {
	t.Helper()

	mod, ok := c.Modules[module]
	if !ok {
		t.Fatalf("module %q not found", module)
	}

	var refs []string
	found := false
	WalkRules(mod, func(r *Rule) bool {
		if string(r.Head.Name) != rule {
			return false
		}
		found = true
		WalkRefs(r.Body, func(ref Ref) bool {
			refs = append(refs, ref.String())
			return false
		})
		return false
	})
	if !found {
		t.Fatalf("rule %q not found in %q", rule, module)
	}

	return refs
}

func hasRef(refs []string, want string) bool {
	return slices.Contains(refs, want)
}

func assertRefs(t *testing.T, c *Compiler, module, rule string, wantPresent, wantAbsent []string) {
	t.Helper()

	refs := bodyRefs(t, c, module, rule)
	for _, want := range wantPresent {
		if !hasRef(refs, want) {
			t.Errorf("%s: expected body to reference %s, got %v", rule, want, refs)
		}
	}
	for _, unwanted := range wantAbsent {
		if hasRef(refs, unwanted) {
			t.Errorf("%s: expected body NOT to reference %s, got %v", rule, unwanted, refs)
		}
	}
}

func TestInlineAliasesLeavesNonAliasRulesAlone(t *testing.T) {
	c := compileForInlining(t, map[string]string{
		"policy.rego": `package test

p if {
	input.attributes.request.http.method == "GET"
	data.config.enabled
}
`,
	})

	assertRefs(t, c, "policy.rego", "p",
		[]string{"input.attributes.request.http.method", "data.config.enabled"},
		nil)
}

func TestInlineAliasesRewritesAliasReferences(t *testing.T) {
	c := compileForInlining(t, map[string]string{
		"http.rego": `package http

request := input.attributes.request.http

method := request.method
`,
		"policy.rego": `package test

import data.http.method

p if method == "GET"
`,
	})

	assertRefs(t, c, "policy.rego", "p",
		[]string{"input.attributes.request.http.method"},
		[]string{"data.http.method", "data.http.request.method"})
}

func TestInlineAliasesRewritesReferenceBelowAlias(t *testing.T) {
	c := compileForInlining(t, map[string]string{
		"http.rego": `package http

request := input.attributes.request.http
`,
		"policy.rego": `package test

p if data.http.request.host == "example.com"
`,
	})

	assertRefs(t, c, "policy.rego", "p",
		[]string{"input.attributes.request.http.host"},
		[]string{"data.http.request.host"})
}

func TestInlineAliasesDepthLimit(t *testing.T) {
	chain := func(n int) string {
		var b strings.Builder
		b.WriteString("package chain\n\n")
		for i := n - 1; i >= 1; i-- {
			fmt.Fprintf(&b, "a%d := a%d\n\n", i, i-1)
		}
		b.WriteString("a0 := input.x\n")
		return b.String()
	}

	for _, tc := range []struct {
		hops       int
		wantInline bool
	}{
		{hops: 1, wantInline: true},
		{hops: 7, wantInline: true},
		{hops: 8, wantInline: true},
		{hops: 9, wantInline: false},
		{hops: 12, wantInline: false},
	} {
		t.Run(fmt.Sprintf("%d_hops", tc.hops), func(t *testing.T) {
			last := fmt.Sprintf("data.chain.a%d", tc.hops-1)
			c := compileForInlining(t, map[string]string{
				"zzz_chain.rego": chain(tc.hops),
				"aaa_policy.rego": fmt.Sprintf(`package test

p if %s == 1
`, last),
			})

			refs := bodyRefs(t, c, "aaa_policy.rego", "p")
			inlined := hasRef(refs, "input.x")
			if inlined != tc.wantInline {
				t.Errorf("%d hops: inlined=%v, want %v (refs: %v)", tc.hops, inlined, tc.wantInline, refs)
			}
			if !tc.wantInline && !hasRef(refs, last) {
				t.Errorf("%d hops: expected %s to be left in place, got %v", tc.hops, last, refs)
			}
		})
	}
}

func TestInlineAliasesRejectsNonAliasShapes(t *testing.T) {
	for _, tc := range []struct {
		note     string
		alias    string
		consumer string
	}{
		{
			note:  "default rule",
			alias: "default v := \"x\"\n\nv := input.x",
		},
		{
			note:  "else chain",
			alias: "v := input.x if input.a\n\nelse := input.y",
		},
		{
			note:     "function",
			alias:    "v(_) := input.x",
			consumer: "p if data.alias.v(1) == 1",
		},
		{
			note:  "two bodies",
			alias: "v := input.x if input.a\n\nv := input.y if input.b",
		},
		{
			note:     "multi-value rule",
			alias:    "v contains x if some x in input.xs",
			consumer: "p if data.alias.v == {1}",
		},
		{
			note:  "scalar value, not a ref",
			alias: "v := 42",
		},
		{
			note:  "body does more than rename",
			alias: "v := input.x if input.gate",
		},
		{
			note:  "target is not ground",
			alias: "v := input.x[_]",
		},
		{
			note:  "target is a call, not a ref",
			alias: "v := count(input.xs)",
		},
	} {
		t.Run(tc.note, func(t *testing.T) {
			consumer := tc.consumer
			if consumer == "" {
				consumer = "p if data.alias.v == 1"
			}
			c := compileForInlining(t, map[string]string{
				"alias.rego":  "package alias\n\n" + tc.alias + "\n",
				"policy.rego": "package test\n\n" + consumer + "\n",
			})

			assertRefs(t, c, "policy.rego", "p",
				[]string{"data.alias.v"},
				[]string{"input.x", "input.y"})
		})
	}
}

func TestInlineAliasesWithInScopeBlocks(t *testing.T) {
	for _, tc := range []struct {
		note        string
		target      string
		wantInlined bool
	}{
		{note: "targets the alias itself", target: "data.http.method"},
		{note: "targets a prefix of the alias", target: "data.http"},
		{note: "targets the resolved base document", target: "input.attributes.request.http.method", wantInlined: true},
		{note: "targets a prefix of the resolved base document", target: "input.attributes", wantInlined: true},
		{note: "targets the root input document", target: "input", wantInlined: true},
	} {
		t.Run(tc.note, func(t *testing.T) {
			c := compileForInlining(t, map[string]string{
				"http.rego": `package http

request := input.attributes.request.http

method := request.method
`,
				"policy.rego": fmt.Sprintf(`package test

p if data.http.method == "GET"

probe if p with %s as "GET"
`, tc.target),
			})

			if tc.wantInlined {
				assertRefs(t, c, "policy.rego", "p",
					[]string{"input.attributes.request.http.method"},
					[]string{"data.http.method"})
			} else {
				assertRefs(t, c, "policy.rego", "p",
					[]string{"data.http.method"},
					[]string{"input.attributes.request.http.method"})
			}
		})
	}
}

func TestInlineAliasesWithOutOfScopeDoesNotBlock(t *testing.T) {
	c := compileForInlining(t, map[string]string{
		"http.rego": `package http

request := input.attributes.request.http

method := request.method
`,
		"policy.rego": `package test

p if data.http.method == "GET"
`,
		"masking.rego": `package system.log

mask contains "/input/x" if {
	data.other.claims with input as input.input
}
`,
		"other.rego": `package other

claims := input.claims
`,
	})

	assertRefs(t, c, "policy.rego", "p",
		[]string{"input.attributes.request.http.method"},
		[]string{"data.http.method"})

	assertRefs(t, c, "masking.rego", "mask",
		[]string{"data.other.claims"},
		[]string{"input.claims"})
}

func TestInlineAliasesWithInsideComprehension(t *testing.T) {
	c := compileForInlining(t, map[string]string{
		"alias.rego": `package alias

v := input.x
`,
		"policy.rego": `package test

p := xs if {
	xs := {y | y := data.alias.v with input as {"x": 42}}
}
`,
	})

	assertRefs(t, c, "policy.rego", "p",
		[]string{"data.alias.v"},
		[]string{"input.x"})
}

func TestInlineAliasesProcessesTestRules(t *testing.T) {
	c := compileForInlining(t, map[string]string{
		"alias.rego": `package alias

v := input.x
`,
		"policy_test.rego": `package test

test_foo if data.alias.v == 1

helper if data.alias.v == 1
`,
	})

	assertRefs(t, c, "policy_test.rego", "test_foo",
		[]string{"input.x"},
		[]string{"data.alias.v"})

	assertRefs(t, c, "policy_test.rego", "helper",
		[]string{"input.x"},
		[]string{"data.alias.v"})
}

func TestInlineAliasesResolvesDataRootedAlias(t *testing.T) {
	c := compileForInlining(t, map[string]string{
		"alias.rego": `package alias

path := data.request.path
`,
		"policy.rego": `package test

p if data.alias.path == ["a"]
`,
	})

	assertRefs(t, c, "policy.rego", "p",
		[]string{"data.request.path"},
		[]string{"data.alias.path"})
}
