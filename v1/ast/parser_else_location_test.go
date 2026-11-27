// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import "testing"

func TestParseElseInlineIfBodyLocations(t *testing.T) {
	module := MustParseModule(`package test

p := 1 if {
	input.x
} else := 2 if input.y in {
	"a",
	"b",
}`)

	expr := module.Rules[0].Else.Body[0]
	if expr.Location.Row != 5 || expr.Location.Col != 16 {
		t.Fatalf("expected expression at 5:16, got %d:%d", expr.Location.Row, expr.Location.Col)
	}
	if exp, act := "input.y in {\n\t\"a\",\n\t\"b\",\n}", string(expr.Location.Text); exp != act {
		t.Fatalf("expected expression text %q, got %q", exp, act)
	}

	set, ok := expr.Operand(1).Value.(Set)
	if !ok {
		t.Fatalf("expected set operand, got %v", expr.Operand(1))
	}
	rows := map[string]int{}
	set.Foreach(func(term *Term) {
		rows[term.String()] = term.Location.Row
	})
	if rows[`"a"`] != 6 || rows[`"b"`] != 7 {
		t.Fatalf("expected set elements on rows 6 and 7, got %v", rows)
	}
}
