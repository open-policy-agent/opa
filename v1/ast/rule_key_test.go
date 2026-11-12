// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"slices"
	"testing"
)

// The compiler copies modules, and the rules of modules parsed without locations
// keep the keys taken while parsing; others are keyed from their source text.
func TestSourceKeysAfterCompile(t *testing.T) {
	const src = `package test

# METADATA
# title: a
p if input.x == 1

q contains 1 if input.y

q contains 2

f(x) := x if x > 0
else := 0
`
	keys := func(dropLocations bool) []RuleKey {
		m := MustParseModuleWithOpts(src, ParserOptions{ProcessAnnotation: true, DropLocations: dropLocations})
		c := NewCompiler()
		if c.Compile(map[string]*Module{"test.rego": m}); c.Failed() {
			t.Fatal(c.Errors)
		}
		var out []RuleKey
		for _, r := range c.Modules["test.rego"].Rules {
			k, ok := r.SourceKey()
			if !ok {
				t.Fatalf("locations dropped: %v: rule %v has no key after compiling", dropLocations, r)
			}
			out = append(out, k)
		}
		return out
	}
	if located, dropped := keys(false), keys(true); len(located) != 4 || !slices.Equal(located, dropped) {
		t.Errorf("expected the same 4 keys either way, got %x and %x", located, dropped)
	}

	if _, ok := MustParseModuleWithOpts(src, ParserOptions{}).Rules[0].SourceKey(); ok {
		t.Error("expected no key without annotations processed")
	}
}
