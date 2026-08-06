package ast

import (
	"testing"

	astJSON "github.com/open-policy-agent/opa/v1/ast/json"
	"github.com/open-policy-agent/opa/v1/util"
)

func newLogicalAnd(lhs, rhs string) *LogicalAnd {
	return &LogicalAnd{
		Lhs: NewBody(NewExpr(VarTerm(lhs))),
		Rhs: NewBody(NewExpr(VarTerm(rhs))),
	}
}

func newLogicalOr(lhs, rhs string) *LogicalOr {
	return &LogicalOr{
		Lhs: NewBody(NewExpr(VarTerm(lhs))),
		Rhs: NewBody(NewExpr(VarTerm(rhs))),
	}
}

func TestLogicalAnd_Compare(t *testing.T) {
	tests := []struct {
		note string
		a, b *LogicalAnd
		want int
	}{
		{"equal", newLogicalAnd("x", "y"), newLogicalAnd("x", "y"), 0},
		{"different lhs", newLogicalAnd("a", "y"), newLogicalAnd("x", "y"), -1},
		{"different rhs", newLogicalAnd("x", "a"), newLogicalAnd("x", "y"), -1},
		{"swapped operands are not equal", newLogicalAnd("x", "y"), newLogicalAnd("y", "x"), -1},
	}
	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			got := tc.a.Compare(tc.b)
			if (got < 0) != (tc.want < 0) || (got > 0) != (tc.want > 0) || (got == 0) != (tc.want == 0) {
				t.Fatalf("Compare: want %d, got %d", tc.want, got)
			}
		})
	}
}

func TestLogicalOr_Compare(t *testing.T) {
	tests := []struct {
		note string
		a, b *LogicalOr
		want int
	}{
		{"equal", newLogicalOr("x", "y"), newLogicalOr("x", "y"), 0},
		{"different lhs", newLogicalOr("a", "y"), newLogicalOr("x", "y"), -1},
		{"different rhs", newLogicalOr("x", "a"), newLogicalOr("x", "y"), -1},
		{"swapped operands are not equal", newLogicalOr("x", "y"), newLogicalOr("y", "x"), -1},
	}
	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			got := tc.a.Compare(tc.b)
			if (got < 0) != (tc.want < 0) || (got > 0) != (tc.want > 0) || (got == 0) != (tc.want == 0) {
				t.Fatalf("Compare: want %d, got %d", tc.want, got)
			}
		})
	}
}

func TestLogical_ExplicitFlagsIgnoredByCompare(t *testing.T) {
	a := newLogicalAnd("x", "y")
	b := newLogicalAnd("x", "y")
	b.ExplicitLhs = true
	b.ExplicitRhs = true
	if a.Compare(b) != 0 {
		t.Fatalf("LogicalAnd.Compare should ignore Explicit flags")
	}

	c := newLogicalOr("x", "y")
	d := newLogicalOr("x", "y")
	d.ExplicitLhs = true
	d.ExplicitRhs = true
	if c.Compare(d) != 0 {
		t.Fatalf("LogicalOr.Compare should ignore Explicit flags")
	}
}

func TestLogical_Copy_IsDeep(t *testing.T) {
	orig := newLogicalAnd("x", "y")
	orig.ExplicitLhs = true
	orig.Location = NewLocation([]byte("x and y"), "test.rego", 1, 1)

	cpy := orig.Copy()
	if cpy == orig {
		t.Fatal("Copy returned same pointer")
	}
	if &cpy.Lhs[0] == &orig.Lhs[0] {
		t.Fatal("Lhs body was not deep-copied")
	}
	if !cpy.ExplicitLhs || cpy.ExplicitRhs {
		t.Fatalf("Explicit flags not preserved by Copy: %+v", cpy)
	}

	// Mutating the copy must not affect the original.
	cpy.Lhs[0].Terms = VarTerm("z")
	if Compare(orig.Lhs, cpy.Lhs) == 0 {
		t.Fatalf("mutating copy.Lhs leaked back to original")
	}
}

func TestLogical_And_Hash_StableForEqualOperands(t *testing.T) {
	a := newLogicalAnd("x", "y")
	b := newLogicalAnd("x", "y")
	if a.Hash() != b.Hash() {
		t.Fatalf("equal LogicalAnd should hash identically, got %d vs %d", a.Hash(), b.Hash())
	}
}

func TestLogical_Or_Hash_StableForEqualOperands(t *testing.T) {
	c := newLogicalOr("x", "y")
	d := newLogicalOr("x", "y")
	if c.Hash() != d.Hash() {
		t.Fatalf("equal LogicalOr should hash identically, got %d vs %d", c.Hash(), d.Hash())
	}
}

func TestLogical_String(t *testing.T) {
	tests := []struct {
		note string
		node Node
		want string
	}{
		{
			note: "and, implicit operands",
			node: newLogicalAnd("x", "y"),
			want: "x and y",
		},
		{
			note: "or, implicit operands",
			node: newLogicalOr("x", "y"),
			want: "x or y",
		},
		{
			note: "and, explicit lhs",
			node: &LogicalAnd{
				Lhs:         NewBody(NewExpr(VarTerm("x")), NewExpr(VarTerm("a"))),
				Rhs:         NewBody(NewExpr(VarTerm("y"))),
				ExplicitLhs: true,
			},
			want: "{ x; a } and y",
		},
		{
			note: "and, explicit rhs",
			node: &LogicalAnd{
				Lhs:         NewBody(NewExpr(VarTerm("x"))),
				Rhs:         NewBody(NewExpr(VarTerm("y")), NewExpr(VarTerm("a"))),
				ExplicitRhs: true,
			},
			want: "x and { y; a }",
		},
		{
			note: "and, both explicit",
			node: &LogicalAnd{
				Lhs:         NewBody(NewExpr(VarTerm("x"))),
				Rhs:         NewBody(NewExpr(VarTerm("y"))),
				ExplicitLhs: true,
				ExplicitRhs: true,
			},
			want: "{ x } and { y }",
		},
		{
			note: "and, implicit multi-expr body",
			node: &LogicalAnd{
				Lhs: NewBody(NewExpr(VarTerm("x")), NewExpr(VarTerm("y"))),
				Rhs: NewBody(NewExpr(VarTerm("z"))),
			},
			want: "{ x; y } and z",
		},
		{
			note: "or, explicit lhs",
			node: &LogicalOr{
				Lhs:         NewBody(NewExpr(VarTerm("x")), NewExpr(VarTerm("a"))),
				Rhs:         NewBody(NewExpr(VarTerm("y"))),
				ExplicitLhs: true,
			},
			want: "{ x; a } or y",
		},
		{
			note: "or, explicit rhs",
			node: &LogicalOr{
				Lhs:         NewBody(NewExpr(VarTerm("x"))),
				Rhs:         NewBody(NewExpr(VarTerm("y")), NewExpr(VarTerm("a"))),
				ExplicitRhs: true,
			},
			want: "x or { y; a }",
		},
		{
			note: "or, both explicit",
			node: &LogicalOr{
				Lhs:         NewBody(NewExpr(VarTerm("x"))),
				Rhs:         NewBody(NewExpr(VarTerm("y"))),
				ExplicitLhs: true,
				ExplicitRhs: true,
			},
			want: "{ x } or { y }",
		},
		{
			note: "or, implicit multi-expr body",
			node: &LogicalOr{
				Lhs: NewBody(NewExpr(VarTerm("x")), NewExpr(VarTerm("y"))),
				Rhs: NewBody(NewExpr(VarTerm("z"))),
			},
			want: "{ x; y } or z",
		},

		// Paren emission
		{
			note: "or under and (rhs) gets parens",
			node: &LogicalAnd{
				Lhs: NewBody(NewExpr(VarTerm("x"))),
				Rhs: NewBody(NewExpr(newLogicalOr("a", "b"))),
			},
			want: "x and (a or b)",
		},
		{
			note: "or under and (lhs) gets parens",
			node: &LogicalAnd{
				Lhs: NewBody(NewExpr(newLogicalOr("a", "b"))),
				Rhs: NewBody(NewExpr(VarTerm("x"))),
			},
			want: "(a or b) and x",
		},
		{
			note: "and under or needs no parens",
			node: &LogicalOr{
				Lhs: NewBody(NewExpr(VarTerm("x"))),
				Rhs: NewBody(NewExpr(newLogicalAnd("a", "b"))),
			},
			want: "x or a and b",
		},
		{
			note: "same-op or rhs gets parens",
			node: &LogicalOr{
				Lhs: NewBody(NewExpr(VarTerm("x"))),
				Rhs: NewBody(NewExpr(newLogicalOr("a", "b"))),
			},
			want: "x or (a or b)",
		},
		{
			note: "same-op or lhs needs no parens",
			node: &LogicalOr{
				Lhs: NewBody(NewExpr(newLogicalOr("a", "b"))),
				Rhs: NewBody(NewExpr(VarTerm("x"))),
			},
			want: "a or b or x",
		},
		{
			note: "same-op and rhs gets parens",
			node: &LogicalAnd{
				Lhs: NewBody(NewExpr(VarTerm("x"))),
				Rhs: NewBody(NewExpr(newLogicalAnd("a", "b"))),
			},
			want: "x and (a and b)",
		},
		{
			note: "value operand gets parens",
			node: &LogicalAnd{
				Lhs: NewBody(NewExpr(SetTerm(VarTerm("a")))),
				Rhs: NewBody(NewExpr(VarTerm("b"))),
			},
			want: "({a}) and b",
		},
		{
			note: "value operand rhs gets parens",
			node: &LogicalOr{
				Lhs: NewBody(NewExpr(VarTerm("a"))),
				Rhs: NewBody(NewExpr(ObjectTerm())),
			},
			want: "a or ({})",
		},
		{
			note: "operand with modifier gets parens",
			node: &LogicalOr{
				Lhs: NewBody(&Expr{
					Terms: VarTerm("a"),
					With:  []*With{{Target: NewTerm(InputRootRef), Value: VarTerm("v")}},
				}),
				Rhs: NewBody(NewExpr(VarTerm("b"))),
			},
			want: "(a with input as v) or b",
		},
	}
	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			var got string
			switch n := tc.node.(type) {
			case *LogicalAnd:
				got = n.String()
			case *LogicalOr:
				got = n.String()
			}
			if got != tc.want {
				t.Fatalf("String: want %q, got %q", tc.want, got)
			}
		})
	}
}

func TestIsAnd(t *testing.T) {
	and := NewExpr(newLogicalAnd("x", "y"))
	plain := NewExpr(VarTerm("z"))

	if !and.IsAnd() || and.IsOr() {
		t.Fatalf("IsAnd/IsOr wrong on And expr: IsAnd=%v IsOr=%v", and.IsAnd(), and.IsOr())
	}
	if plain.IsAnd() {
		t.Fatalf("IsAnd should be false for plain expr")
	}
}

func TestIsOr(t *testing.T) {
	or := NewExpr(newLogicalOr("x", "y"))
	plain := NewExpr(VarTerm("z"))

	if !or.IsOr() || or.IsAnd() {
		t.Fatalf("IsAnd/IsOr wrong on Or expr: IsAnd=%v IsOr=%v", or.IsAnd(), or.IsOr())
	}
	if plain.IsOr() {
		t.Fatalf("IsOr should be false for plain expr")
	}
}

func TestExpr_NewExprAcceptsLogicalAndOr(t *testing.T) {
	// Must not panic
	NewExpr(newLogicalAnd("x", "y"))
	NewExpr(newLogicalOr("x", "y"))
}

func TestLogicalAnd_CompareViaExpr(t *testing.T) {
	a := NewExpr(newLogicalAnd("x", "y"))
	b := NewExpr(newLogicalAnd("x", "y"))
	if a.Compare(b) != 0 {
		t.Fatalf("equal And exprs should compare equal")
	}

	c := NewExpr(newLogicalOr("x", "y"))
	if a.Compare(c) == 0 {
		t.Fatalf("And and Or exprs should not compare equal")
	}
}

func TestLogicalAnd_CopyViaExpr(t *testing.T) {
	orig := NewExpr(newLogicalAnd("x", "y"))
	cpy := orig.Copy()
	if cpy.Terms == orig.Terms {
		t.Fatal("Expr.Copy did not deep-copy *LogicalAnd")
	}
	if cpy.Compare(orig) != 0 {
		t.Fatal("copy should be Compare-equal to original")
	}
}

func TestLogicalAnd_IsGround(t *testing.T) {
	// A body of only ground terms is ground; a body containing a Var is not.
	ground := NewExpr(&LogicalAnd{
		Lhs: NewBody(NewExpr(BooleanTerm(true))),
		Rhs: NewBody(NewExpr(BooleanTerm(false))),
	})
	if !ground.IsGround() {
		t.Fatal("ground LogicalAnd expr should report IsGround=true")
	}

	notGround := NewExpr(newLogicalAnd("x", "y"))
	if notGround.IsGround() {
		t.Fatal("LogicalAnd with var operands should report IsGround=false")
	}
}

func TestLogicalAnd_Walk_DescendsIntoBodies(t *testing.T) {
	node := newLogicalAnd("a", "b")

	seen := map[string]bool{}
	WalkVars(node, func(v Var) bool {
		seen[string(v)] = true
		return false
	})

	for _, name := range []string{"a", "b"} {
		if !seen[name] {
			t.Errorf("Walk did not visit var %q in LogicalAnd body; seen=%v", name, seen)
		}
	}
}

func TestLogicalOr_Walk_DescendsIntoBodies(t *testing.T) {
	node := newLogicalOr("a", "b")

	seen := map[string]bool{}
	WalkVars(node, func(v Var) bool {
		seen[string(v)] = true
		return false
	})

	for _, name := range []string{"a", "b"} {
		if !seen[name] {
			t.Errorf("Walk did not visit var %q in LogicalOr body; seen=%v", name, seen)
		}
	}
}

func TestLogicalAnd_Transform_RewritesVarsInBothBodies(t *testing.T) {
	expr := NewExpr(newLogicalAnd("x", "y"))
	out, err := TransformVars(expr, func(v Var) (Value, error) {
		return Var(string(v) + "_renamed"), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	gotExpr, ok := out.(*Expr)
	if !ok {
		t.Fatalf("Transform returned %T, want *Expr", out)
	}
	got, ok := gotExpr.Terms.(*LogicalAnd)
	if !ok {
		t.Fatalf("Expr.Terms is %T, want *LogicalAnd", gotExpr.Terms)
	}
	if got.Lhs[0].Terms.(*Term).Value.(Var) != "x_renamed" {
		t.Errorf("Lhs var not rewritten: %v", got.Lhs)
	}
	if got.Rhs[0].Terms.(*Term).Value.(Var) != "y_renamed" {
		t.Errorf("Rhs var not rewritten: %v", got.Rhs)
	}
}

func TestLogicalOr_Transform_RewritesVarsInBothBodies(t *testing.T) {
	expr := NewExpr(newLogicalOr("x", "y"))
	out, err := TransformVars(expr, func(v Var) (Value, error) {
		return Var(string(v) + "_renamed"), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	gotExpr, ok := out.(*Expr)
	if !ok {
		t.Fatalf("Transform returned %T, want *Expr", out)
	}
	got, ok := gotExpr.Terms.(*LogicalOr)
	if !ok {
		t.Fatalf("Expr.Terms is %T, want *LogicalOr", gotExpr.Terms)
	}
	if got.Lhs[0].Terms.(*Term).Value.(Var) != "x_renamed" {
		t.Errorf("Lhs var not rewritten: %v", got.Lhs)
	}
	if got.Rhs[0].Terms.(*Term).Value.(Var) != "y_renamed" {
		t.Errorf("Rhs var not rewritten: %v", got.Rhs)
	}
}

func TestLogicalAnd_MarshalJSON(t *testing.T) {
	loc := NewLocation([]byte("x and y"), "test.rego", 4, 2)

	tests := []struct {
		note    string
		node    *LogicalAnd
		options astJSON.Options
		want    string
	}{
		{
			note: "default - no explicit flags, no location",
			node: newLogicalAnd("x", "y"),
			want: `{"lhs":[{"index":0,"terms":{"type":"var","value":"x"}}],"rhs":[{"index":0,"terms":{"type":"var","value":"y"}}],"type":"and"}`,
		},
		{
			note: "explicit_lhs only",
			node: &LogicalAnd{
				Lhs:         NewBody(NewExpr(VarTerm("x"))),
				Rhs:         NewBody(NewExpr(VarTerm("y"))),
				ExplicitLhs: true,
			},
			want: `{"explicit_lhs":true,"lhs":[{"index":0,"terms":{"type":"var","value":"x"}}],"rhs":[{"index":0,"terms":{"type":"var","value":"y"}}],"type":"and"}`,
		},
		{
			note: "both explicit",
			node: &LogicalAnd{
				Lhs:         NewBody(NewExpr(VarTerm("x"))),
				Rhs:         NewBody(NewExpr(VarTerm("y"))),
				ExplicitLhs: true,
				ExplicitRhs: true,
			},
			want: `{"explicit_lhs":true,"explicit_rhs":true,"lhs":[{"index":0,"terms":{"type":"var","value":"x"}}],"rhs":[{"index":0,"terms":{"type":"var","value":"y"}}],"type":"and"}`,
		},
		{
			note: "location set but IncludeLocation.And not toggled - location omitted",
			node: func() *LogicalAnd {
				a := newLogicalAnd("x", "y")
				a.Location = loc
				return a
			}(),
			want: `{"lhs":[{"index":0,"terms":{"type":"var","value":"x"}}],"rhs":[{"index":0,"terms":{"type":"var","value":"y"}}],"type":"and"}`,
		},
		{
			note: "location set and IncludeLocation.And toggled - location present",
			node: func() *LogicalAnd {
				a := newLogicalAnd("x", "y")
				a.Location = loc
				return a
			}(),
			options: astJSON.Options{
				MarshalOptions: astJSON.MarshalOptions{
					IncludeLocation: astJSON.NodeToggle{And: true},
				},
			},
			want: `{"lhs":[{"index":0,"terms":{"type":"var","value":"x"}}],"location":{"file":"test.rego","row":4,"col":2},"rhs":[{"index":0,"terms":{"type":"var","value":"y"}}],"type":"and"}`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			astJSON.SetOptions(tc.options)
			t.Cleanup(resetJSONOptions)

			got := util.MustMarshalJSON(tc.node)
			assertJsonEqual(t, tc.want, got)
		})
	}
}

func TestLogicalOr_MarshalJSON(t *testing.T) {
	loc := NewLocation([]byte("x or y"), "test.rego", 4, 2)

	tests := []struct {
		note    string
		node    *LogicalOr
		options astJSON.Options
		want    string
	}{
		{
			note: "default - no explicit flags, no location",
			node: newLogicalOr("x", "y"),
			want: `{"lhs":[{"index":0,"terms":{"type":"var","value":"x"}}],"rhs":[{"index":0,"terms":{"type":"var","value":"y"}}],"type":"or"}`,
		},
		{
			note: "explicit_lhs only",
			node: &LogicalOr{
				Lhs:         NewBody(NewExpr(VarTerm("x"))),
				Rhs:         NewBody(NewExpr(VarTerm("y"))),
				ExplicitLhs: true,
			},
			want: `{"explicit_lhs":true,"lhs":[{"index":0,"terms":{"type":"var","value":"x"}}],"rhs":[{"index":0,"terms":{"type":"var","value":"y"}}],"type":"or"}`,
		},
		{
			note: "both explicit",
			node: &LogicalOr{
				Lhs:         NewBody(NewExpr(VarTerm("x"))),
				Rhs:         NewBody(NewExpr(VarTerm("y"))),
				ExplicitLhs: true,
				ExplicitRhs: true,
			},
			want: `{"explicit_lhs":true,"explicit_rhs":true,"lhs":[{"index":0,"terms":{"type":"var","value":"x"}}],"rhs":[{"index":0,"terms":{"type":"var","value":"y"}}],"type":"or"}`,
		},
		{
			note: "location set but IncludeLocation.Or not toggled - location omitted",
			node: func() *LogicalOr {
				a := newLogicalOr("x", "y")
				a.Location = loc
				return a
			}(),
			want: `{"lhs":[{"index":0,"terms":{"type":"var","value":"x"}}],"rhs":[{"index":0,"terms":{"type":"var","value":"y"}}],"type":"or"}`,
		},
		{
			note: "location set and IncludeLocation.Or toggled - location present",
			node: func() *LogicalOr {
				a := newLogicalOr("x", "y")
				a.Location = loc
				return a
			}(),
			options: astJSON.Options{
				MarshalOptions: astJSON.MarshalOptions{
					IncludeLocation: astJSON.NodeToggle{Or: true},
				},
			},
			want: `{"lhs":[{"index":0,"terms":{"type":"var","value":"x"}}],"location":{"file":"test.rego","row":4,"col":2},"rhs":[{"index":0,"terms":{"type":"var","value":"y"}}],"type":"or"}`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			astJSON.SetOptions(tc.options)
			t.Cleanup(resetJSONOptions)

			got := util.MustMarshalJSON(tc.node)
			assertJsonEqual(t, tc.want, got)
		})
	}
}

func TestLogicalAnd_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		note string
		json string
		want *LogicalAnd
	}{
		{
			note: "minimal",
			json: `{"type":"and","lhs":[{"index":0,"terms":{"type":"var","value":"x"}}],"rhs":[{"index":0,"terms":{"type":"var","value":"y"}}]}`,
			want: newLogicalAnd("x", "y"),
		},
		{
			note: "explicit flags",
			json: `{"type":"and","explicit_lhs":true,"explicit_rhs":true,"lhs":[{"index":0,"terms":{"type":"var","value":"x"}}],"rhs":[{"index":0,"terms":{"type":"var","value":"y"}}]}`,
			want: &LogicalAnd{
				Lhs:         NewBody(NewExpr(VarTerm("x"))),
				Rhs:         NewBody(NewExpr(VarTerm("y"))),
				ExplicitLhs: true,
				ExplicitRhs: true,
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			got := &LogicalAnd{}
			if err := got.UnmarshalJSON([]byte(tc.json)); err != nil {
				t.Fatal(err)
			}
			if got.Compare(tc.want) != 0 {
				t.Fatalf("got %s, want %s", got, tc.want)
			}
			if got.ExplicitLhs != tc.want.ExplicitLhs || got.ExplicitRhs != tc.want.ExplicitRhs {
				t.Fatalf("explicit flags mismatch: got (%v, %v), want (%v, %v)",
					got.ExplicitLhs, got.ExplicitRhs, tc.want.ExplicitLhs, tc.want.ExplicitRhs)
			}
		})
	}
}

func TestLogicalOr_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		note string
		json string
		want *LogicalOr
	}{
		{
			note: "minimal",
			json: `{"type":"or","lhs":[{"index":0,"terms":{"type":"var","value":"x"}}],"rhs":[{"index":0,"terms":{"type":"var","value":"y"}}]}`,
			want: newLogicalOr("x", "y"),
		},
		{
			note: "explicit flags",
			json: `{"type":"or","explicit_lhs":true,"explicit_rhs":true,"lhs":[{"index":0,"terms":{"type":"var","value":"x"}}],"rhs":[{"index":0,"terms":{"type":"var","value":"y"}}]}`,
			want: &LogicalOr{
				Lhs:         NewBody(NewExpr(VarTerm("x"))),
				Rhs:         NewBody(NewExpr(VarTerm("y"))),
				ExplicitLhs: true,
				ExplicitRhs: true,
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			got := &LogicalOr{}
			if err := got.UnmarshalJSON([]byte(tc.json)); err != nil {
				t.Fatal(err)
			}
			if got.Compare(tc.want) != 0 {
				t.Fatalf("got %s, want %s", got, tc.want)
			}
			if got.ExplicitLhs != tc.want.ExplicitLhs || got.ExplicitRhs != tc.want.ExplicitRhs {
				t.Fatalf("explicit flags mismatch: got (%v, %v), want (%v, %v)",
					got.ExplicitLhs, got.ExplicitRhs, tc.want.ExplicitLhs, tc.want.ExplicitRhs)
			}
		})
	}
}

func TestLogicalAnd_UnmarshalJSON_ErrorPaths(t *testing.T) {
	tests := []struct {
		note string
		json string
	}{
		{"lhs missing", `{"type":"and","rhs":[]}`},
		{"rhs missing", `{"type":"and","lhs":[]}`},
		{"lhs wrong type", `{"type":"and","lhs":"oops","rhs":[]}`},
		{"explicit_lhs wrong type", `{"type":"and","explicit_lhs":"yes","lhs":[{"index":0,"terms":{"type":"var","value":"x"}}],"rhs":[{"index":0,"terms":{"type":"var","value":"y"}}]}`},
	}
	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			a := &LogicalAnd{}
			if err := a.UnmarshalJSON([]byte(tc.json)); err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

func TestLogicalOr_UnmarshalJSON_ErrorPaths(t *testing.T) {
	tests := []struct {
		note string
		json string
	}{
		{"lhs missing", `{"type":"and","rhs":[]}`},
		{"rhs missing", `{"type":"and","lhs":[]}`},
		{"lhs wrong type", `{"type":"and","lhs":"oops","rhs":[]}`},
		{"explicit_lhs wrong type", `{"type":"and","explicit_lhs":"yes","lhs":[{"index":0,"terms":{"type":"var","value":"x"}}],"rhs":[{"index":0,"terms":{"type":"var","value":"y"}}]}`},
	}
	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			a := &LogicalOr{}
			if err := a.UnmarshalJSON([]byte(tc.json)); err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

// TestLogical_ExprUnmarshalDispatch verifies that *Expr.UnmarshalJSON routes
// a `{"type":"and",...}` / `{"type":"or",...}` Terms payload to *LogicalAnd /
// *LogicalOr (rather than treating it as a *Term).
func TestLogical_ExprUnmarshalDispatch(t *testing.T) {
	tests := []struct {
		note string
		json string
		want any // *And or *Or
	}{
		{
			note: "and dispatch",
			json: `{"index":0,"terms":{"type":"and","lhs":[{"index":0,"terms":{"type":"var","value":"x"}}],"rhs":[{"index":0,"terms":{"type":"var","value":"y"}}]}}`,
			want: newLogicalAnd("x", "y"),
		},
		{
			note: "or dispatch",
			json: `{"index":0,"terms":{"type":"or","lhs":[{"index":0,"terms":{"type":"var","value":"x"}}],"rhs":[{"index":0,"terms":{"type":"var","value":"y"}}]}}`,
			want: newLogicalOr("x", "y"),
		},
	}
	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			got := &Expr{}
			if err := got.UnmarshalJSON([]byte(tc.json)); err != nil {
				t.Fatal(err)
			}
			switch w := tc.want.(type) {
			case *LogicalAnd:
				gotAnd, ok := got.Terms.(*LogicalAnd)
				if !ok {
					t.Fatalf("Expr.Terms is %T, want *LogicalAnd", got.Terms)
				}
				if gotAnd.Compare(w) != 0 {
					t.Fatalf("got %s, want %s", gotAnd, w)
				}
			case *LogicalOr:
				gotOr, ok := got.Terms.(*LogicalOr)
				if !ok {
					t.Fatalf("Expr.Terms is %T, want *LogicalOr", got.Terms)
				}
				if gotOr.Compare(w) != 0 {
					t.Fatalf("got %s, want %s", gotOr, w)
				}
			}
		})
	}
}

// TestLogical_MarshalUnmarshalRoundTrip verifies marshal-then-unmarshal
// produces a Compare-equal value, for both flat nodes and nodes wrapped in
// *Expr.
func TestLogical_MarshalUnmarshalRoundTrip(t *testing.T) {
	cases := []struct {
		note string
		expr *Expr
	}{
		{"and implicit", NewExpr(newLogicalAnd("x", "y"))},
		{"or implicit", NewExpr(newLogicalOr("x", "y"))},
		{"and both explicit", NewExpr(&LogicalAnd{
			Lhs:         NewBody(NewExpr(VarTerm("x"))),
			Rhs:         NewBody(NewExpr(VarTerm("y"))),
			ExplicitLhs: true,
			ExplicitRhs: true,
		})},
		{"or only explicit_rhs", NewExpr(&LogicalOr{
			Lhs:         NewBody(NewExpr(VarTerm("x"))),
			Rhs:         NewBody(NewExpr(VarTerm("y"))),
			ExplicitRhs: true,
		})},
	}
	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			bs := util.MustMarshalJSON(tc.expr)
			out := &Expr{}
			if err := out.UnmarshalJSON(bs); err != nil {
				t.Fatalf("unmarshal: %v\ninput: %s", err, bs)
			}
			if out.Compare(tc.expr) != 0 {
				t.Fatalf("round-trip mismatch:\nwant: %s\ngot:  %s", tc.expr, out)
			}
			// Explicit-flag preservation isn't covered by Compare (the typed
			// Compare ignores them on purpose), so check separately.
			switch in := tc.expr.Terms.(type) {
			case *LogicalAnd:
				outAnd := out.Terms.(*LogicalAnd)
				if in.ExplicitLhs != outAnd.ExplicitLhs || in.ExplicitRhs != outAnd.ExplicitRhs {
					t.Fatalf("explicit flags lost: in=(%v,%v) out=(%v,%v)",
						in.ExplicitLhs, in.ExplicitRhs, outAnd.ExplicitLhs, outAnd.ExplicitRhs)
				}
			case *LogicalOr:
				outOr := out.Terms.(*LogicalOr)
				if in.ExplicitLhs != outOr.ExplicitLhs || in.ExplicitRhs != outOr.ExplicitRhs {
					t.Fatalf("explicit flags lost: in=(%v,%v) out=(%v,%v)",
						in.ExplicitLhs, in.ExplicitRhs, outOr.ExplicitLhs, outOr.ExplicitRhs)
				}
			}
		})
	}
}
