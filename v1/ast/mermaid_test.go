// Copyright 2025 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"strings"
	"testing"
)

func TestMermaidGraph(t *testing.T) {
	src := `package example

import rego.v1

default allow := false

allow if {
	input.user == "admin"
}

greet(name) := msg if {
	msg := concat(" ", ["hello", name])
}
`
	module, err := ParseModuleWithOpts("test.rego", src, ParserOptions{})
	if err != nil {
		t.Fatal(err)
	}

	out := mermaidGraph(module)

	if !strings.HasPrefix(out, "flowchart TD\n") {
		t.Errorf("expected output to start with 'flowchart TD', got:\n%s", out)
	}

	// Should contain key structural nodes.
	checks := []string{
		`"Module"`,
		`"Package: data.example"`,
		`"Import: rego.v1"`,
		"-->|package|",
		"-->|import|",
		"-->|rule|",
	}
	for _, want := range checks {
		if !strings.Contains(out, want) {
			t.Errorf("expected output to contain %q\nfull output:\n%s", want, out)
		}
	}

	t.Logf("MermaidGraph output:\n%s", out)
}
