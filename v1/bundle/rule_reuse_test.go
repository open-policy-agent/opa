// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package bundle

import (
	"slices"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
)

func TestReuseModuleParser(t *testing.T) {
	popts := generatedParserOptions(ast.ParserOptions{RegoVersion: ast.RegoV1})
	rule := func(id, body string) string {
		return "# METADATA\n# labels:\n#   id: \"" + id + "\"\nallow if { " + body + " }\n\n"
	}
	header := "package gen\n\nimport rego.v1\n\n"
	r1 := rule("r1", `input.x == "a"`)
	r2 := rule("r2", `input.x == "b"`)
	multi := "allow if {\n\t# inside\n\tinput.y == `{\n}`\n}\n"
	prevSrc := header + r1 + r2 + multi + "allow if input.w\nallow if input.v\n"

	type testCase struct {
		note   string
		prev   string // defaults to prevSrc
		src    string
		reused int
		parsed []string // bodies of the rules that were parsed
	}
	// The reader must key every statement like the compiler keyed the rule parsed from it.
	same := func(note, rules string, n int) testCase {
		src := "package gen\n\n" + rules
		return testCase{note: note, prev: src, src: src, reused: n}
	}
	tests := []testCase{
		{note: "same source", src: prevSrc, reused: 5},
		{note: "rule added", src: header + r2 + rule("r3", `input.x == "c"`) + r1, reused: 2, parsed: []string{`equal(input.x, "c")`}},
		{note: "rule changed", src: header + rule("r1", `input.x == "z"`) + r2, reused: 1, parsed: []string{`equal(input.x, "z")`}},
		{note: "comment after the rule", src: header + r1[:len(r1)-1] + "# trailing\n\n" + r2, reused: 2},
		{note: "comment on the rule's line", src: header + "allow if input.w # c\n", parsed: []string{"input.w"}},
		same("body", "allow if {\n\tinput.x\n\tinput.y\n}\n", 1),
		same("comment in body", "allow if {\n\t# c\n\tinput.x\n}\n", 1),
		same("braces in strings", "allow if {\n\tinput.x == \"}\"\n\tinput.y == `{\n}`\n}\n", 1),
		same("hash in string", "allow if input.x == \"#\"\n", 1),
		same("indented", "  allow if input.x\n", 1),
		same("metadata", "# METADATA\n# title: t\n# labels:\n#   id: \"r1\"\nallow if input.x\n", 1),
		same("crlf", "# METADATA\r\n# title: t\r\nallow if {\r\n\tinput.x\r\n}\r\n", 1),
		same("no space", "# METADATA\n#title: t\nallow if input.x\n", 1),
		same("blank comment", "# METADATA\n# title: t\n#\n# description: d\nallow if input.x\n", 1),
		same("assignment", "allow := true if input.x\n", 1),
		same("contains", "deny contains msg if {\n\tinput.x\n\tmsg := \"no\"\n}\n", 1),
		same("ref head", "a.b.c if input.x\n", 1),
		same("several", "allow if input.x\n\nallow if input.y\n# METADATA\n# title: z\nallow if input.z\n", 3),
		{
			// To the parser, the indented comment ends the METADATA block.
			note:   "split block",
			prev:   "package gen\n\n# METADATA\n# a: 1\n# b: 2\nallow if input.x\n",
			src:    "package gen\n\n# METADATA\n# a: 1\n  # b: 2\nallow if input.x\n",
			parsed: []string{"input.x"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			if tc.prev == "" {
				tc.prev = prevSrc
			}
			prevMod, err := ast.ParseModuleWithOpts("a.rego", tc.prev, popts)
			if err != nil {
				t.Fatal(err)
			}
			prev := ast.NewCompiler().WithRuleReuse(nil)
			if prev.Compile(map[string]*ast.Module{"a.rego": prevMod}); prev.Failed() {
				t.Fatal(prev.Errors)
			}

			mod, err := reuseModuleParser(func() *ast.Compiler { return prev })("a.rego", []byte(tc.src), popts)
			if err != nil {
				t.Fatal(err)
			}
			var reused int
			var parsed []string
			for _, r := range mod.Rules {
				if slices.Contains(prev.GetRulesExact(r.Ref()), r) {
					reused++
				} else {
					parsed = append(parsed, r.Body.String())
				}
			}
			if reused != tc.reused || !slices.Equal(parsed, tc.parsed) {
				t.Fatalf("reused %d, parsed %v; want %d, %v", reused, parsed, tc.reused, tc.parsed)
			}
		})
	}
}

// A module with an else chain isn't reusable, so only this sees how the scan cuts one.
func TestStatements(t *testing.T) {
	src := "package p\n\n# plain\n# METADATA\n# title: x\na if {\n\t`{`\n\t# c\n}\n# trailing\nb if \"}\"\n" +
		"c if {\n} else := 2 if {\n}\nelse := 3\n"
	var got []string
	for s := range statements([]byte(src)) {
		got = append(got, src[s.annotation:s.start]+"|"+src[s.start:s.end])
	}
	exp := []string{
		"|package p\n",
		"# METADATA\n# title: x\n|a if {\n\t`{`\n\t# c\n}\n",
		"|b if \"}\"\n",
		"|c if {\n} else := 2 if {\n}\nelse := 3\n",
	}
	if !slices.Equal(got, exp) {
		t.Fatalf("got %q, want %q", got, exp)
	}
}
