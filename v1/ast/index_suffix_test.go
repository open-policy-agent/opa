// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import "testing"

// TestSuffixIndexing covers what recording a base reversed makes of it. What a
// suffix statement has to be for the indexer to take it at all is shared with
// the prefix statements, and tested with them.
func TestSuffixIndexing(t *testing.T) {
	c := MustCompileModules(map[string]string{"test.rego": `package test

p if endswith(input.path, ".gz")

p if endswith(input.path, ".tar.gz")

p if strings.any_suffix_match(input.path, [".go", ".rego"])
`})

	index := newBaseDocEqIndex(func(Ref) bool { return false })
	if !index.Build(c.Modules["test.rego"].Rules) {
		t.Fatal("expected index build to succeed")
	}

	for _, tc := range []struct {
		input string
		exp   int
	}{
		// Every base the value ends with, not just one: ".tar.gz" and ".gz"
		// both hold here, and share a stem reversed.
		{`{"path": "a.tar.gz"}`, 2},
		{`{"path": "a.gz"}`, 1},
		{`{"path": "main.go"}`, 1},
		{`{"path": "policy.rego"}`, 1},
		{`{"path": "main.rs"}`, 0},
		// Anchored to the end, and to the end of the value rather than the end
		// of the base: "gz" is what ".gz" ends with, not the other way round.
		{`{"path": "a.tar.gzip"}`, 0},
		{`{"path": "gz"}`, 0},
		{`{"path": ""}`, 0},
		// Read backwards it would match, which is the mistake to make here.
		{`{"path": "zg.rat.a"}`, 0},
		// As for prefixes, a value that is not a string satisfies no suffix.
		{`{"path": 42}`, 0},
	} {
		t.Run(tc.input, func(t *testing.T) {
			result, err := index.Lookup(testResolver{input: MustParseTerm(tc.input)})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(result.Rules) != tc.exp {
				t.Errorf("expected %d candidate(s), got %d", tc.exp, len(result.Rules))
			}
		})
	}
}
