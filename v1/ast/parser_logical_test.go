package ast

import (
	"bytes"
	"strings"
	"testing"
)

func logicalParserOpts(extraFuture ...string) ParserOptions {
	caps := CapabilitiesForThisVersion(CapabilitiesExperimentalKeywords(true))
	fk := append([]string{"and", "or"}, extraFuture...)
	return ParserOptions{
		Capabilities:   caps,
		FutureKeywords: fk,
	}
}

func TestParseLogical_Parsing(t *testing.T) {
	// `not` enabled so `not {x or y}` (the explicit-body form) works.
	opts := logicalParserOpts("not")

	tests := []struct {
		note  string
		input string
		exp   *Expr
	}{
		// Basic
		{
			note:  "and basic",
			input: "x and y",
			exp: &Expr{Terms: &LogicalAnd{
				Lhs: NewBody(NewExpr(VarTerm("x"))),
				Rhs: NewBody(NewExpr(VarTerm("y"))),
			}},
		},
		{
			note:  "or basic",
			input: "x or y",
			exp: &Expr{Terms: &LogicalOr{
				Lhs: NewBody(NewExpr(VarTerm("x"))),
				Rhs: NewBody(NewExpr(VarTerm("y"))),
			}},
		},

		// Precedence: `and` binds tighter than `or`
		{
			note:  "and tighter than or — rhs",
			input: "x or y and z",
			exp: &Expr{Terms: &LogicalOr{
				Lhs: NewBody(NewExpr(VarTerm("x"))),
				Rhs: NewBody(NewExpr(&LogicalAnd{
					Lhs: NewBody(NewExpr(VarTerm("y"))),
					Rhs: NewBody(NewExpr(VarTerm("z"))),
				})),
			}},
		},
		{
			note:  "and tighter than or — lhs",
			input: "x and y or z",
			exp: &Expr{Terms: &LogicalOr{
				Lhs: NewBody(NewExpr(&LogicalAnd{
					Lhs: NewBody(NewExpr(VarTerm("x"))),
					Rhs: NewBody(NewExpr(VarTerm("y"))),
				})),
				Rhs: NewBody(NewExpr(VarTerm("z"))),
			}},
		},

		// Associativity (both operators left-associative)
		{
			note:  "and is left-associative",
			input: "x and y and z",
			exp: &Expr{Terms: &LogicalAnd{
				Lhs: NewBody(NewExpr(&LogicalAnd{
					Lhs: NewBody(NewExpr(VarTerm("x"))),
					Rhs: NewBody(NewExpr(VarTerm("y"))),
				})),
				Rhs: NewBody(NewExpr(VarTerm("z"))),
			}},
		},
		{
			note:  "or is left-associative",
			input: "x or y or z",
			exp: &Expr{Terms: &LogicalOr{
				Lhs: NewBody(NewExpr(&LogicalOr{
					Lhs: NewBody(NewExpr(VarTerm("x"))),
					Rhs: NewBody(NewExpr(VarTerm("y"))),
				})),
				Rhs: NewBody(NewExpr(VarTerm("z"))),
			}},
		},

		// Not interaction (`not` binds tighter than both `and` and `or`)
		{
			note:  "not tighter than and",
			input: "not x and y",
			exp: &Expr{Terms: &LogicalAnd{
				Lhs: NewBody(NewExpr(&Not{Body: NewBody(NewExpr(VarTerm("x")))})),
				Rhs: NewBody(NewExpr(VarTerm("y"))),
			}},
		},
		{
			note:  "not tighter than or",
			input: "not x or y",
			exp: &Expr{Terms: &LogicalOr{
				Lhs: NewBody(NewExpr(&Not{Body: NewBody(NewExpr(VarTerm("x")))})),
				Rhs: NewBody(NewExpr(VarTerm("y"))),
			}},
		},
		{
			note:  "not with explicit body wraps or",
			input: "not {x or y}",
			exp: &Expr{Terms: &Not{
				Body: NewBody(NewExpr(&LogicalOr{
					Lhs: NewBody(NewExpr(VarTerm("x"))),
					Rhs: NewBody(NewExpr(VarTerm("y"))),
				})),
				ExplicitBody: true,
			}},
		},
		{
			note:  "not on rhs, and",
			input: "x and not y",
			exp: &Expr{Terms: &LogicalAnd{
				Lhs: NewBody(NewExpr(VarTerm("x"))),
				Rhs: NewBody(NewExpr(&Not{Body: NewBody(NewExpr(VarTerm("y")))})),
			}},
		},
		{
			note:  "not on rhs, or",
			input: "x or not y",
			exp: &Expr{Terms: &LogicalOr{
				Lhs: NewBody(NewExpr(VarTerm("x"))),
				Rhs: NewBody(NewExpr(&Not{Body: NewBody(NewExpr(VarTerm("y")))})),
			}},
		},

		// Explicit body operands
		{
			note:  "and, lhs explicit",
			input: "{x; y} and z",
			exp: &Expr{Terms: &LogicalAnd{
				Lhs:         NewBody(NewExpr(VarTerm("x")), NewExpr(VarTerm("y"))),
				Rhs:         NewBody(NewExpr(VarTerm("z"))),
				ExplicitLhs: true,
			}},
		},
		{
			note:  "and, rhs explicit",
			input: "x and {y; z}",
			exp: &Expr{Terms: &LogicalAnd{
				Lhs:         NewBody(NewExpr(VarTerm("x"))),
				Rhs:         NewBody(NewExpr(VarTerm("y")), NewExpr(VarTerm("z"))),
				ExplicitRhs: true,
			}},
		},
		{
			note:  "and, both explicit",
			input: "{a; b} and {c; d}",
			exp: &Expr{Terms: &LogicalAnd{
				Lhs:         NewBody(NewExpr(VarTerm("a")), NewExpr(VarTerm("b"))),
				Rhs:         NewBody(NewExpr(VarTerm("c")), NewExpr(VarTerm("d"))),
				ExplicitLhs: true,
				ExplicitRhs: true,
			}},
		},
		{
			note:  "or, lhs explicit",
			input: "{x; y} or z",
			exp: &Expr{Terms: &LogicalOr{
				Lhs:         NewBody(NewExpr(VarTerm("x")), NewExpr(VarTerm("y"))),
				Rhs:         NewBody(NewExpr(VarTerm("z"))),
				ExplicitLhs: true,
			}},
		},
		{
			note:  "or, rhs explicit",
			input: "x or {y; z}",
			exp: &Expr{Terms: &LogicalOr{
				Lhs:         NewBody(NewExpr(VarTerm("x"))),
				Rhs:         NewBody(NewExpr(VarTerm("y")), NewExpr(VarTerm("z"))),
				ExplicitRhs: true,
			}},
		},
		{
			note:  "or, both explicit",
			input: "{a; b} or {c; d}",
			exp: &Expr{Terms: &LogicalOr{
				Lhs:         NewBody(NewExpr(VarTerm("a")), NewExpr(VarTerm("b"))),
				Rhs:         NewBody(NewExpr(VarTerm("c")), NewExpr(VarTerm("d"))),
				ExplicitLhs: true,
				ExplicitRhs: true,
			}},
		},
		{
			note:  "nested via explicit body, rhs",
			input: "x and {y or z}",
			exp: &Expr{Terms: &LogicalAnd{
				Lhs: NewBody(NewExpr(VarTerm("x"))),
				Rhs: NewBody(NewExpr(&LogicalOr{
					Lhs: NewBody(NewExpr(VarTerm("y"))),
					Rhs: NewBody(NewExpr(VarTerm("z"))),
				})),
				ExplicitRhs: true,
			}},
		},
		{
			note:  "nested via explicit body, rhs",
			input: "{x or y} and z",
			exp: &Expr{Terms: &LogicalAnd{
				Lhs: NewBody(NewExpr(&LogicalOr{
					Lhs: NewBody(NewExpr(VarTerm("x"))),
					Rhs: NewBody(NewExpr(VarTerm("y"))),
				})),
				Rhs:         NewBody(NewExpr(VarTerm("z"))),
				ExplicitLhs: true,
			}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			assertParseOneExpr(t, tc.note, tc.input, tc.exp, opts)
		})
	}
}

func TestParseLogical_WithModifier(t *testing.T) {
	opts := logicalParserOpts()

	successTests := []struct {
		note  string
		input string
		exp   *Expr
	}{
		{
			note:  "with after binary attaches to outer",
			input: "a and b with input as v",
			exp: &Expr{
				Terms: &LogicalAnd{
					Lhs: NewBody(NewExpr(VarTerm("a"))),
					Rhs: NewBody(NewExpr(VarTerm("b"))),
				},
				With: []*With{{Target: NewTerm(InputRootRef), Value: VarTerm("v")}},
			},
		},
		{
			note:  "with inside explicit lhs body is scoped to the inner expression",
			input: "{a with input as v} and b",
			exp: &Expr{Terms: &LogicalAnd{
				Lhs: NewBody(&Expr{
					Terms: VarTerm("a"),
					With:  []*With{{Target: NewTerm(InputRootRef), Value: VarTerm("v")}},
				}),
				Rhs:         NewBody(NewExpr(VarTerm("b"))),
				ExplicitLhs: true,
			}},
		},
		{
			note:  "with on outermost in chain",
			input: "a or b and c with input as v",
			exp: &Expr{
				Terms: &LogicalOr{
					Lhs: NewBody(NewExpr(VarTerm("a"))),
					Rhs: NewBody(NewExpr(&LogicalAnd{
						Lhs: NewBody(NewExpr(VarTerm("b"))),
						Rhs: NewBody(NewExpr(VarTerm("c"))),
					})),
				},
				With: []*With{{Target: NewTerm(InputRootRef), Value: VarTerm("v")}},
			},
		},
	}
	for _, tc := range successTests {
		t.Run(tc.note, func(t *testing.T) {
			assertParseOneExpr(t, tc.note, tc.input, tc.exp, opts)
		})
	}

	errorTests := []struct {
		note      string
		input     string
		expectErr string
	}{
		{
			note:      "and, with on lhs implicit operand is an error",
			input:     "a with input as v and b",
			expectErr: "`with` modifier is not allowed on operand of `and` (hint: Wrap the operand in `(...)` or `{...}` to scope, or move `with` after the `and` expression to apply it to the whole expression)",
		},
		{
			note:      "or, with on lhs implicit operand is an error",
			input:     "a with input as v or b",
			expectErr: "`with` modifier is not allowed on operand of `or` (hint: Wrap the operand in `(...)` or `{...}` to scope, or move `with` after the `or` expression to apply it to the whole expression)",
		},
		{
			note:      "with mid-chain cannot be followed by another operator",
			input:     "a and b with input as v or c",
			expectErr: "unexpected or keyword",
		},
	}
	for _, tc := range errorTests {
		t.Run(tc.note, func(t *testing.T) {
			assertParseErrorContains(t, tc.note, tc.input, tc.expectErr, opts)
		})
	}
}

func TestParseLogical_ParseErrors(t *testing.T) {
	opts := logicalParserOpts()

	exprTests := []struct {
		note     string
		input    string
		expected string
	}{
		{"missing rhs after and", "x and", "unexpected eof"},
		{"missing rhs after or", "x or", "unexpected eof"},
		{"bare and prefix", "and y", "unexpected and keyword"},
		{"bare or prefix", "or y", "unexpected or keyword"},
		{"double or operator", "x or or y", "unexpected or keyword"},
		{"double and operator", "x and and y", "unexpected and keyword"},
		{"extra or-and operator", "x or and y", "unexpected and keyword"},
		{"extra and-or operator", "x and or y", "unexpected or keyword"},
		{"and, inside call", "f(x and y)", "unexpected and keyword"},
		{"or, inside call", "f(x or y)", "unexpected or keyword"},
		{"and, inside ref", "a[x and y]", "unexpected and keyword"},
		{"or, inside ref", "a[x or y]", "unexpected or keyword"},
		{"and, inside set comprehension head", "{x and y | x}", "unexpected and keyword"},
		{"or, inside set comprehension head", "{x or y | x}", "unexpected or keyword"},
		{"and, inside array comprehension head", "[x and y | x]", "unexpected and keyword"},
		{"or, inside array comprehension head", "[x or y | x]", "unexpected or keyword"},
		{"and, inside object comprehension head, key", "{x and y: z | x}", "unexpected and keyword"},
		{"or, inside object comprehension head, key", "{x or y: z | x}", "unexpected or keyword"},
		{"and, inside object comprehension head, value", "{x: y and z | x}", "unexpected and keyword"},
		{"or, inside object comprehension head, value", "{x: y or z | x}", "unexpected or keyword"},
		{"and, inside every value", "every x and y in z {x}", "unexpected and keyword"},
		{"or, inside every value", "every x or y in z {x}", "unexpected or keyword"},
		{"and, inside every domain", "every x in y and z {x}", "unexpected and keyword"},
		{"or, inside every domain", "every x in y or z {x}", "unexpected or keyword"},
	}
	for _, tc := range exprTests {
		t.Run(tc.note, func(t *testing.T) {
			assertParseErrorContains(t, tc.note, tc.input, tc.expected, opts)
		})
	}

	// TODO: Error messages here are not ideal, but improving them would impact other error states; revisit and improve general error reporting
	modTests := []struct {
		note     string
		input    string
		expected string
	}{
		{
			note: "and, rule value",
			input: `package test
				p := x and y
			`,
			expected: "expression cannot be used for rule head",
		},
		{
			note: "or, rule value",
			input: `package test
				p := x or y
			`,
			expected: "expression cannot be used for rule head",
		},
		{
			note: "and, free statement in module body",
			input: `package test
				x and y
			`,
			expected: "expression cannot be used for rule head",
		},
		{
			note: "or, free statement in module body",
			input: `package test
				x or y
			`,
			expected: "expression cannot be used for rule head",
		},
		{
			note: "and, in import",
			input: `package test
				import data.x and y
			`,
			expected: "var cannot be used for rule name",
		},
		{
			note: "or, in import",
			input: `package test
				import data.x or y
			`,
			expected: "var cannot be used for rule name",
		},
	}
	for _, tc := range modTests {
		t.Run(tc.note, func(t *testing.T) {
			assertParseModuleErrorMessage(t, tc.note, tc.input, tc.expected, opts)
		})
	}
}

func TestParseLogical_RefsContainingAndOr(t *testing.T) {
	opts := logicalParserOpts()
	tests := []struct {
		note  string
		input string
		exp   *Expr
	}{
		{
			note:  "x.and dot access",
			input: "x.and",
			exp:   &Expr{Terms: RefTerm(VarTerm("x"), StringTerm("and"))},
		},
		{
			note:  "and.x dot access",
			input: "and.x",
			exp:   &Expr{Terms: RefTerm(VarTerm("and"), StringTerm("x"))},
		},
		{
			note:  `x["and"] bracket access`,
			input: `x["and"]`,
			exp:   &Expr{Terms: RefTerm(VarTerm("x"), StringTerm("and"))},
		},
		{
			note:  "x.or dot access",
			input: "x.or",
			exp:   &Expr{Terms: RefTerm(VarTerm("x"), StringTerm("or"))},
		},
		{
			note:  "or.x dot access",
			input: "or.x",
			exp:   &Expr{Terms: RefTerm(VarTerm("or"), StringTerm("x"))},
		},
		{
			note:  `x["or"] bracket access`,
			input: `x["or"]`,
			exp:   &Expr{Terms: RefTerm(VarTerm("x"), StringTerm("or"))},
		},
	}
	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			assertParseOneExpr(t, tc.note, tc.input, tc.exp, opts)
		})
	}
}

func TestParseLogical_NoLeakageOnImport(t *testing.T) {
	t.Run("import error does not leak keyword names", func(t *testing.T) {
		opts := ParserOptions{Capabilities: CapabilitiesForThisVersion()}
		input := `package x
			import future.keywords.and
		`
		_, _, err := ParseStatementsWithOpts("", input, opts)
		if err == nil {
			t.Fatal("expected parse error for import of and without experimental caps")
		}
		// Error message is of the form "unexpected keyword, must be one of
		// [...]". The bracketed list must not include `and` or `or`.
		msg := err.Error()
		for _, leaked := range []string{"[and ", " and]", " and ", "[or ", " or]", " or "} {
			if strings.Contains(msg, leaked) {
				t.Errorf("error leaks internal keyword existence (%q present in %q)", leaked, msg)
			}
		}
	})

	t.Run("import accepted with experimental caps", func(t *testing.T) {
		opts := ParserOptions{
			Capabilities:   CapabilitiesForThisVersion(CapabilitiesExperimentalKeywords(true)),
			FutureKeywords: []string{"and"},
		}
		input := `package x
			allow if { x and y }
		`
		if _, _, err := ParseStatementsWithOpts("", input, opts); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("unrelated keyword import does not activate logical keywords", func(t *testing.T) {
		tests := []struct {
			note   string
			module string
		}{
			{
				note: "and, only future.keywords.not imported",
				module: `package x
					import future.keywords.not

					p if input.a and input.b
				`,
			},
			{
				note: "or, only future.keywords.not imported",
				module: `package x
					import future.keywords.not

					p if input.a or input.b
				`,
			},
			{
				note: "and, only future.keywords.in imported",
				module: `package x
					import future.keywords.in

					p if input.a and input.b
				`,
			},
			{
				note: "or, only future.keywords.in imported",
				module: `package x
					import future.keywords.in

					p if input.a or input.b
				`,
			},
		}
		opts := ParserOptions{
			RegoVersion:  RegoV1,
			Capabilities: CapabilitiesForThisVersion(CapabilitiesExperimentalKeywords(true)),
		}
		for _, tc := range tests {
			t.Run(tc.note, func(t *testing.T) {
				// `and`/`or` are plain identifiers here, so three bare terms in a
				// row is not a valid expression. Asserting on the error text would
				// be brittle; what matters is that no logical node is produced.
				mod, err := ParseModuleWithOpts("test.rego", tc.module, opts)
				if err == nil {
					t.Fatalf("expected parse error, got body: %v (%T)",
						mod.Rules[0].Body, mod.Rules[0].Body[0].Terms)
				}
			})
		}
	})

	t.Run("unrelated keyword import does not reserve logical keywords as var names", func(t *testing.T) {
		tests := []struct {
			note   string
			module string
		}{
			{
				note: "and",
				module: `package x
					import future.keywords.not

					p if {
						and := 1
						and == 1
					}
				`,
			},
			{
				note: "or",
				module: `package x
					import future.keywords.not

					p if {
						or := 1
						or == 1
					}
				`,
			},
		}
		opts := ParserOptions{
			RegoVersion:  RegoV1,
			Capabilities: CapabilitiesForThisVersion(CapabilitiesExperimentalKeywords(true)),
		}
		for _, tc := range tests {
			t.Run(tc.note, func(t *testing.T) {
				if _, err := ParseModuleWithOpts("test.rego", tc.module, opts); err != nil {
					t.Errorf("expected %q to remain usable as a variable name, got: %v", tc.note, err)
				}
			})
		}
	})
}

// TestParseLogical_PartialActivation exercises the case where one of `and` /
// `or` is enabled in the scanner but the other is not.
func TestParseLogical_PartialActivation(t *testing.T) {
	caps := CapabilitiesForThisVersion(CapabilitiesExperimentalKeywords(true))

	tests := []struct {
		note      string
		enable    []string
		input     string
		expectAnd bool
		expectOr  bool
	}{
		{
			note:      "only and enabled: `x and y` parses",
			enable:    []string{"and"},
			input:     "x and y",
			expectAnd: true,
		},
		{
			note:   "only and enabled: or stays an Ident",
			enable: []string{"and"},
			input:  "x or y",
		},
		{
			note:      "only and enabled: `{x; y} and z` parses",
			enable:    []string{"and"},
			input:     "{x; y} and z",
			expectAnd: true,
		},
		{
			note:   "only and enabled: `{x; y} or z` does NOT parse",
			enable: []string{"and"},
			input:  "{x; y} or z",
		},
		{
			note:     "only or enabled: `x or y` parses",
			enable:   []string{"or"},
			input:    "x or y",
			expectOr: true,
		},
		{
			note:   "only or enabled: and stays an Ident",
			enable: []string{"or"},
			input:  "x and y",
		},
		{
			note:     "only or enabled: `{x; y} or z` parses",
			enable:   []string{"or"},
			input:    "{x; y} or z",
			expectOr: true,
		},
		{
			note:   "only or enabled: `{x; y} and z` does NOT parse",
			enable: []string{"or"},
			input:  "{x; y} and z",
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			opts := ParserOptions{
				Capabilities:   caps,
				FutureKeywords: tc.enable,
			}
			body, err := ParseBodyWithOpts(tc.input, opts)

			if !tc.expectAnd && !tc.expectOr {
				if err == nil {
					for _, expr := range body {
						if _, ok := expr.Terms.(*LogicalAnd); ok {
							t.Errorf("unexpected *LogicalAnd in parsed body: %s", body)
						}
						if _, ok := expr.Terms.(*LogicalOr); ok {
							t.Errorf("unexpected *LogicalOr in parsed body: %s", body)
						}
					}
				}
				return
			}

			if err != nil {
				t.Fatalf("expected successful parse, got: %v", err)
			}
			if len(body) != 1 {
				t.Fatalf("expected exactly one expression, got %d: %s", len(body), body)
			}
			if tc.expectAnd {
				if _, ok := body[0].Terms.(*LogicalAnd); !ok {
					t.Fatalf("expected *LogicalAnd, got %T: %s", body[0].Terms, body)
				}
			}
			if tc.expectOr {
				if _, ok := body[0].Terms.(*LogicalOr); !ok {
					t.Fatalf("expected *LogicalOr, got %T: %s", body[0].Terms, body)
				}
			}
		})
	}
}

func TestParseLogical_RoundTripString(t *testing.T) {
	opts := logicalParserOpts()
	sources := []string{
		"x and y",
		"{ x } and { y }",
		"x or y",
		"{ x } or { y }",
		"x or y and z",
		"{ x or y } and z",
		"x and y or z",
		"x and { y or z }",
		"x and y and z",
		"x or y or z",
		"a or b and c with input as v",
	}
	for _, src := range sources {
		t.Run(src, func(t *testing.T) {
			body, err := ParseBodyWithOpts(src, opts)
			if err != nil {
				t.Fatal(err)
			}
			rendered := body.String()
			body2, err := ParseBodyWithOpts(rendered, opts)
			if err != nil {
				t.Fatalf("round-trip parse of %q failed: %v", rendered, err)
			}
			if body.Compare(body2) != 0 {
				t.Fatalf("round-trip mismatch:\noriginal: %s\nrendered: %s", body, body2)
			}
		})
	}
}

func TestParseLogical_ChainLocations(t *testing.T) {
	// `not` enabled so the `not { … }` explicit not-body RHS case parses.
	opts := logicalParserOpts("not")

	type chainLoc struct {
		col  int
		text string
	}

	tests := []struct {
		note   string
		input  string
		chains []chainLoc // expected (Col, Text) on every chain wrapper, in walk order
	}{
		{
			note:   "simple and",
			input:  "x and y",
			chains: []chainLoc{{col: 1, text: "x and y"}},
		},
		{
			note:   "simple or",
			input:  "x or y",
			chains: []chainLoc{{col: 1, text: "x or y"}},
		},
		{
			note:  "and-chain left-associative",
			input: "x and y and z",
			chains: []chainLoc{
				{col: 1, text: "x and y and z"},
				{col: 1, text: "x and y"},
			},
		},
		{
			note:  "or-chain left-associative",
			input: "x or y or z",
			chains: []chainLoc{
				{col: 1, text: "x or y or z"},
				{col: 1, text: "x or y"},
			},
		},
		{
			note:  "mixed precedence",
			input: "x or y and z",
			chains: []chainLoc{
				{col: 1, text: "x or y and z"},
				{col: 6, text: "y and z"},
			},
		},
		{
			note:  "mixed precedence — and tighter than or",
			input: "x and y or z",
			chains: []chainLoc{
				{col: 1, text: "x and y or z"},
				{col: 1, text: "x and y"},
			},
		},
		{
			note:  "mixed precedence — and-chains on both sides of or",
			input: "x and y or z and w",
			chains: []chainLoc{
				{col: 1, text: "x and y or z and w"},
				{col: 1, text: "x and y"},
				{col: 12, text: "z and w"},
			},
		},
		{
			note:   "LHS explicit body, and",
			input:  "{ x; y } and z",
			chains: []chainLoc{{col: 1, text: "{ x; y } and z"}},
		},
		{
			note:   "LHS explicit body, or",
			input:  "{ x; y } or z",
			chains: []chainLoc{{col: 1, text: "{ x; y } or z"}},
		},
		{
			note:   "RHS explicit body, and",
			input:  "x and { y; z }",
			chains: []chainLoc{{col: 1, text: "x and { y; z }"}},
		},
		{
			note:  "RHS explicit body extending into and-chain — anchors at `{`",
			input: "a or { x; y } and z",
			chains: []chainLoc{
				{col: 1, text: "a or { x; y } and z"},
				{col: 6, text: "{ x; y } and z"},
			},
		},
		{
			// Branch coverage for the `negated && notBodies && LBrace` arm
			// of parseLogicalOperand. The constructed *Not's Location
			// anchors at the `not` keyword and spans the full `not { ... }`
			// text, so the surrounding chain inherits that span.
			note:  "RHS negated explicit not-body extending into and-chain",
			input: "a or not { x; y } and z",
			chains: []chainLoc{
				{col: 1, text: "a or not { x; y } and z"},
				{col: 6, text: "not { x; y } and z"},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			body, err := ParseBodyWithOpts(tc.input, opts)
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}
			if len(body) != 1 {
				t.Fatalf("expected 1 expr in body, got %d", len(body))
			}

			var got []chainLoc
			collectChainLocs(t, body[0], func(col int, text string) {
				got = append(got, chainLoc{col: col, text: text})
			})

			if len(got) != len(tc.chains) {
				t.Fatalf("expected %d chain wrappers, got %d: %+v", len(tc.chains), len(got), got)
			}

			for i, want := range tc.chains {
				if got[i].col != want.col {
					t.Errorf("chain[%d] Col = %d, want %d", i, got[i].col, want.col)
				}
				if got[i].text != want.text {
					t.Errorf("chain[%d] Text = %q, want %q", i, got[i].text, want.text)
				}
			}
		})
	}
}

func collectChainLocs(t *testing.T, e *Expr, emit func(col int, text string)) {
	t.Helper()
	if e == nil {
		return
	}
	var nodeLoc *Location
	var lhs, rhs Body
	switch terms := e.Terms.(type) {
	case *LogicalAnd:
		nodeLoc = terms.Location
		lhs, rhs = terms.Lhs, terms.Rhs
	case *LogicalOr:
		nodeLoc = terms.Location
		lhs, rhs = terms.Lhs, terms.Rhs
	default:
		return
	}

	if e.Location == nil {
		t.Fatalf("chain wrapper Expr.Location is nil")
	}
	if nodeLoc == nil {
		t.Fatalf("chain node.Location is nil")
	}
	if e.Location.Col != nodeLoc.Col || e.Location.Row != nodeLoc.Row || !bytes.Equal(e.Location.Text, nodeLoc.Text) {
		t.Errorf("chain wrapper Expr.Location %+v != node.Location %+v", e.Location, nodeLoc)
	}

	emit(e.Location.Col, string(e.Location.Text))
	for _, x := range lhs {
		collectChainLocs(t, x, emit)
	}
	for _, x := range rhs {
		collectChainLocs(t, x, emit)
	}
}

func TestParseLogical_InnerExprHasLocation(t *testing.T) {
	module := `package test
		import future.keywords.and
		import future.keywords.or
		
		p if {
			f(1) and f(2)
			f(3) or f(4)
		}
	`

	popts := ParserOptions{
		Capabilities: CapabilitiesForThisVersion(CapabilitiesExperimentalKeywords(true)),
	}

	mod, err := ParseModuleWithOpts("test.rego", module, popts)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	outer := mod.Rules[0].Body[0]
	if outer.Location == nil {
		t.Fatalf("outer and Expr has nil Location")
	}

	and := outer.Terms.(*LogicalAnd)
	if and.Location == nil {
		t.Fatalf("LogicalAnd.Location is nil")
	}

	inner := and.Lhs[0]
	if inner.Location == nil {
		t.Fatalf("inner Expr inside LogicalAnd.Lhs has nil Location")
	}
	if inner.Location.Col != 4 {
		t.Errorf("Expected column to be 4 but got: %v", inner.Location.Col)
	}
	if inner.Location.Row != 6 {
		t.Errorf("Expected row to be 6 but got: %v", inner.Location.Row)
	}
	if inner.Location.File != "test.rego" {
		t.Errorf("Expected file to be test.rego but got: %v", inner.Location.File)
	}

	inner = and.Rhs[0]
	if inner.Location == nil {
		t.Fatalf("inner Expr inside LogicalAnd.Rhs has nil Location")
	}
	if inner.Location.Col != 13 {
		t.Errorf("Expected column to be 4 but got: %v", inner.Location.Col)
	}
	if inner.Location.Row != 6 {
		t.Errorf("Expected row to be 6 but got: %v", inner.Location.Row)
	}
	if inner.Location.File != "test.rego" {
		t.Errorf("Expected file to be test.rego but got: %v", inner.Location.File)
	}

	outer = mod.Rules[0].Body[1]
	if outer.Location == nil {
		t.Fatalf("outer or Expr has nil Location")
	}

	or := outer.Terms.(*LogicalOr)
	if and.Location == nil {
		t.Fatalf("LogicalOr.Location is nil")
	}

	inner = or.Lhs[0]
	if inner.Location == nil {
		t.Fatalf("inner Expr inside LogicalOr.Lhs has nil Location")
	}
	if inner.Location.Col != 4 {
		t.Errorf("Expected column to be 4 but got: %v", inner.Location.Col)
	}
	if inner.Location.Row != 7 {
		t.Errorf("Expected row to be 7 but got: %v", inner.Location.Row)
	}
	if inner.Location.File != "test.rego" {
		t.Errorf("Expected file to be test.rego but got: %v", inner.Location.File)
	}

	inner = or.Rhs[0]
	if inner.Location == nil {
		t.Fatalf("inner Expr inside LogicalOr.Lhs has nil Location")
	}
	if inner.Location.Col != 12 {
		t.Errorf("Expected column to be 12 but got: %v", inner.Location.Col)
	}
	if inner.Location.Row != 7 {
		t.Errorf("Expected row to be 7 but got: %v", inner.Location.Row)
	}
	if inner.Location.File != "test.rego" {
		t.Errorf("Expected file to be test.rego but got: %v", inner.Location.File)
	}
}

func TestParseLogical_ParenGrouping(t *testing.T) {
	opts := logicalParserOpts("not")

	tests := []struct {
		note  string
		input string
		exp   *Expr
	}{
		{
			note:  "rhs group",
			input: "a and (b or c)",
			exp: &Expr{Terms: &LogicalAnd{
				Lhs: NewBody(NewExpr(VarTerm("a"))),
				Rhs: NewBody(NewExpr(&LogicalOr{
					Lhs: NewBody(NewExpr(VarTerm("b"))),
					Rhs: NewBody(NewExpr(VarTerm("c"))),
				})),
			}},
		},
		{
			note:  "lhs group",
			input: "(a or b) and c",
			exp: &Expr{Terms: &LogicalAnd{
				Lhs: NewBody(NewExpr(&LogicalOr{
					Lhs: NewBody(NewExpr(VarTerm("a"))),
					Rhs: NewBody(NewExpr(VarTerm("b"))),
				})),
				Rhs: NewBody(NewExpr(VarTerm("c"))),
			}},
		},
		{
			note:  "outer group unwrapped",
			input: "(a or b)",
			exp: &Expr{Terms: &LogicalOr{
				Lhs: NewBody(NewExpr(VarTerm("a"))),
				Rhs: NewBody(NewExpr(VarTerm("b"))),
			}},
		},
		{
			note:  "nested groups",
			input: "(a and (b or c)) and d",
			exp: &Expr{Terms: &LogicalAnd{
				Lhs: NewBody(NewExpr(&LogicalAnd{
					Lhs: NewBody(NewExpr(VarTerm("a"))),
					Rhs: NewBody(NewExpr(&LogicalOr{
						Lhs: NewBody(NewExpr(VarTerm("b"))),
						Rhs: NewBody(NewExpr(VarTerm("c"))),
					})),
				})),
				Rhs: NewBody(NewExpr(VarTerm("d"))),
			}},
		},
		{
			note:  "redundant parens around group collapse",
			input: "((a or b))",
			exp: &Expr{Terms: &LogicalOr{
				Lhs: NewBody(NewExpr(VarTerm("a"))),
				Rhs: NewBody(NewExpr(VarTerm("b"))),
			}},
		},
		{
			note:  "redundant parens around operand collapse",
			input: "a or ((b))",
			exp: &Expr{Terms: &LogicalOr{
				Lhs: NewBody(NewExpr(VarTerm("a"))),
				Rhs: NewBody(NewExpr(VarTerm("b"))),
			}},
		},
		{
			note:  "group on both sides",
			input: "(a or b) and (c or d)",
			exp: &Expr{Terms: &LogicalAnd{
				Lhs: NewBody(NewExpr(&LogicalOr{
					Lhs: NewBody(NewExpr(VarTerm("a"))),
					Rhs: NewBody(NewExpr(VarTerm("b"))),
				})),
				Rhs: NewBody(NewExpr(&LogicalOr{
					Lhs: NewBody(NewExpr(VarTerm("c"))),
					Rhs: NewBody(NewExpr(VarTerm("d"))),
				})),
			}},
		},
		{
			note:  "group forces right nesting of same operator",
			input: "a or (b or c)",
			exp: &Expr{Terms: &LogicalOr{
				Lhs: NewBody(NewExpr(VarTerm("a"))),
				Rhs: NewBody(NewExpr(&LogicalOr{
					Lhs: NewBody(NewExpr(VarTerm("b"))),
					Rhs: NewBody(NewExpr(VarTerm("c"))),
				})),
			}},
		},
		{
			note:  "not inside group",
			input: "(not a)",
			exp:   NewExpr(&Not{Body: NewBody(NewExpr(VarTerm("a")))}),
		},
		{
			note:  "not with grouped operand",
			input: "not (a)",
			exp:   NewExpr(&Not{Body: NewBody(NewExpr(VarTerm("a")))}),
		},
		{
			note:  "not inside group binds under or",
			input: "(not a or b)",
			exp: &Expr{Terms: &LogicalOr{
				Lhs: NewBody(NewExpr(&Not{Body: NewBody(NewExpr(VarTerm("a")))})),
				Rhs: NewBody(NewExpr(VarTerm("b"))),
			}},
		},
		{
			note:  "not with grouped operand inside or",
			input: "not (a) or b",
			exp: &Expr{Terms: &LogicalOr{
				Lhs: NewBody(NewExpr(&Not{Body: NewBody(NewExpr(VarTerm("a")))})),
				Rhs: NewBody(NewExpr(VarTerm("b"))),
			}},
		},
		{
			note:  "not inside group binds under or",
			input: "(not a or b)",
			exp: &Expr{Terms: &LogicalOr{
				Lhs: NewBody(NewExpr(&Not{Body: NewBody(NewExpr(VarTerm("a")))})),
				Rhs: NewBody(NewExpr(VarTerm("b"))),
			}},
		},
		{
			note:  "redundant parens around single operand drop",
			input: "(a) and (b)",
			exp: &Expr{Terms: &LogicalAnd{
				Lhs: NewBody(NewExpr(VarTerm("a"))),
				Rhs: NewBody(NewExpr(VarTerm("b"))),
			}},
		},
		{
			note:  "group forces right nesting of and",
			input: "a and (b and c)",
			exp: &Expr{Terms: &LogicalAnd{
				Lhs: NewBody(NewExpr(VarTerm("a"))),
				Rhs: NewBody(NewExpr(&LogicalAnd{
					Lhs: NewBody(NewExpr(VarTerm("b"))),
					Rhs: NewBody(NewExpr(VarTerm("c"))),
				})),
			}},
		},
		{
			note:  "redundant parens preserve same-precedence tree",
			input: "(a and b) or c",
			exp: &Expr{Terms: &LogicalOr{
				Lhs: NewBody(NewExpr(&LogicalAnd{
					Lhs: NewBody(NewExpr(VarTerm("a"))),
					Rhs: NewBody(NewExpr(VarTerm("b"))),
				})),
				Rhs: NewBody(NewExpr(VarTerm("c"))),
			}},
		},
		{
			note:  "comparison operands inside groups",
			input: "(a == b) or (c == d)",
			exp: &Expr{Terms: &LogicalOr{
				Lhs: NewBody(Equal.Expr(VarTerm("a"), VarTerm("b"))),
				Rhs: NewBody(Equal.Expr(VarTerm("c"), VarTerm("d"))),
			}},
		},
		{
			note:  "deeply mixed and/or groups",
			input: "a or (b and (c or d))",
			exp: &Expr{Terms: &LogicalOr{
				Lhs: NewBody(NewExpr(VarTerm("a"))),
				Rhs: NewBody(NewExpr(&LogicalAnd{
					Lhs: NewBody(NewExpr(VarTerm("b"))),
					Rhs: NewBody(NewExpr(&LogicalOr{
						Lhs: NewBody(NewExpr(VarTerm("c"))),
						Rhs: NewBody(NewExpr(VarTerm("d"))),
					})),
				})),
			}},
		},
		{
			note:  "explicit body lhs with paren group rhs",
			input: "{x; y} and (a or b)",
			exp: &Expr{Terms: &LogicalAnd{
				Lhs: NewBody(NewExpr(VarTerm("x")), NewExpr(VarTerm("y"))),
				Rhs: NewBody(NewExpr(&LogicalOr{
					Lhs: NewBody(NewExpr(VarTerm("a"))),
					Rhs: NewBody(NewExpr(VarTerm("b"))),
				})),
				ExplicitLhs: true,
			}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			assertParseOneExpr(t, tc.note, tc.input, tc.exp, opts)
		})
	}
}

func TestParseLogical_ParenErrors(t *testing.T) {
	opts := logicalParserOpts("not")

	tests := []struct {
		note     string
		input    string
		expected string
	}{
		{
			note:     "empty group",
			input:    "a or ()",
			expected: "empty parenthesized group",
		},
		{
			note:     "parens cannot wrap a body with logical content",
			input:    "a and ({b or c})",
			expected: "`(...)` in an operand position cannot contain a body (hint: drop the parens to keep the body: `{b or c}`)",
		},
		{
			note:     "unterminated group",
			input:    "a and (b or c",
			expected: "expected ) to close parenthesized group",
		},
		{
			note:     "non-logical group operand backtracks to term path",
			input:    "a or (x = 1)",
			expected: "non-terminated expression",
		},
		{
			note:     "empty group at statement start backtracks to term",
			input:    "() and a",
			expected: "unexpected ) token",
		},
		{
			note:     "group opening with an operator",
			input:    "a and ( or b)",
			expected: "unexpected or keyword",
		},
		{
			note:     "assignment in group backtracks to term path",
			input:    "a and (x := 1)",
			expected: "non-terminated expression",
		},
		{
			note:     "some in group backtracks to term path",
			input:    "a or (some x in xs)",
			expected: "unexpected some keyword",
		},
		{
			note:     "multi-statement needs explicit body",
			input:    "a and (x; y)",
			expected: "non-terminated expression",
		},
		{
			note:     "empty not group",
			input:    "not ()",
			expected: "empty parenthesized group",
		},
		{
			note:     "logical group cannot be compared",
			input:    "(a or b) == c",
			expected: "unexpected equal token",
		},
	}

	for _, tc := range tests {
		assertParseErrorContains(t, tc.note, tc.input, tc.expected, opts)
	}
}

func TestParseLogical_ParenBraceContext(t *testing.T) {
	opts := logicalParserOpts()
	notOpts := logicalParserOpts("not")

	tests := []struct {
		note  string
		input string
		exp   *Expr
		opts  *ParserOptions
	}{
		// Operand context: bare `{...}` is a body; but parens hold a value, not a body
		{
			note:  "operand brace is a body if future not kw imported",
			opts:  &notOpts,
			input: "not {true}",
			exp:   &Expr{Terms: &Not{Body: NewBody(NewExpr(BooleanTerm(true))), ExplicitBody: true}},
		},
		{
			note:  "not operand parens hold a value if future not kw imported",
			opts:  &notOpts,
			input: "not ({true})",
			exp:   &Expr{Terms: &Not{Body: NewBody(NewExpr(SetTerm(BooleanTerm(true))))}},
		},
		{
			note:  "not operand brace is not a body if future not kw unimported",
			input: "not ({true})",
			exp: &Expr{
				Terms:   SetTerm(BooleanTerm(true)),
				Negated: true,
			},
		},
		{
			note:  "and operand braces are bodies",
			input: "{a} and {b}",
			exp: &Expr{Terms: &LogicalAnd{
				Lhs:         NewBody(NewExpr(VarTerm("a"))),
				Rhs:         NewBody(NewExpr(VarTerm("b"))),
				ExplicitLhs: true,
				ExplicitRhs: true,
			}},
		},
		{
			note:  "and operand parens hold values",
			input: "({a}) and ({b})",
			exp: &Expr{Terms: &LogicalAnd{
				Lhs: NewBody(NewExpr(SetTerm(VarTerm("a")))),
				Rhs: NewBody(NewExpr(SetTerm(VarTerm("b")))),
			}},
		},
		{
			note:  "rhs operand parens hold a value",
			input: "a and ({b})",
			exp: &Expr{Terms: &LogicalAnd{
				Lhs: NewBody(NewExpr(VarTerm("a"))),
				Rhs: NewBody(NewExpr(SetTerm(VarTerm("b")))),
			}},
		},
		{
			note:  "empty parens hold an empty object",
			input: "a or ({})",
			exp: &Expr{Terms: &LogicalOr{
				Lhs: NewBody(NewExpr(VarTerm("a"))),
				Rhs: NewBody(NewExpr(ObjectTerm())),
			}},
		},

		// Non-operand context: `({...})` is an object/set term, unchanged.
		{
			note:  "empty object",
			input: "({})",
			exp:   &Expr{Terms: ObjectTerm()},
		},
		{
			note:  "set",
			input: "({a})",
			exp:   &Expr{Terms: SetTerm(VarTerm("a"))},
		},
		{
			note:  "object on rhs of assign",
			input: "a := ({b})",
			exp:   Assign.Expr(VarTerm("a"), SetTerm(VarTerm("b"))),
		},
		{
			note:  "empty object as call arg",
			input: "g(({}))",
			exp:   &Expr{Terms: []*Term{RefTerm(VarTerm("g")), ObjectTerm()}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			var popts ParserOptions
			if tc.opts != nil {
				popts = *tc.opts
			} else {
				popts = opts
			}
			assertParseOneExpr(t, tc.note, tc.input, tc.exp, popts)
		})
	}
}

func TestParseLogical_ParenExplicit(t *testing.T) {
	opts := logicalParserOpts("not")

	tests := []struct {
		note        string
		input       string
		explicitLhs bool
		explicitRhs bool
	}{
		{
			note:        "paren group rhs stays implicit",
			input:       "a and (b or c)",
			explicitLhs: false,
			explicitRhs: false,
		},
		{
			note:        "explicit body rhs is explicit",
			input:       "a and {b}",
			explicitLhs: false,
			explicitRhs: true,
		},
		{
			note:        "explicit body with logical content is explicit",
			input:       "a and {b or c}",
			explicitLhs: false,
			explicitRhs: true,
		},
		{
			note:        "explicit body lhs before and is explicit",
			input:       "{a} and c",
			explicitLhs: true,
			explicitRhs: false,
		},
		{
			note:        "wrapped braces are a value, not an explicit body",
			input:       "a and ({b})",
			explicitLhs: false,
			explicitRhs: false,
		},
		{
			note:        "wrapped braces lhs are a value, not an explicit body",
			input:       "({a}) and c",
			explicitLhs: false,
			explicitRhs: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			body, err := ParseBodyWithOpts(tc.input, opts)
			if err != nil {
				t.Fatalf("unexpected parse error: %v", err)
			}
			and, ok := body[0].Terms.(*LogicalAnd)
			if !ok {
				t.Fatalf("expected *LogicalAnd, got %T", body[0].Terms)
			}
			if and.ExplicitLhs != tc.explicitLhs || and.ExplicitRhs != tc.explicitRhs {
				t.Errorf("explicit flags: got (lhs=%v, rhs=%v), want (lhs=%v, rhs=%v)",
					and.ExplicitLhs, and.ExplicitRhs, tc.explicitLhs, tc.explicitRhs)
			}
		})
	}
}

func TestParseLogical_ParenNot(t *testing.T) {
	opts := logicalParserOpts("not")

	tests := []struct {
		note  string
		input string
		exp   *Expr
	}{
		{
			note:  "not group or",
			input: "not (a or b)",
			exp: &Expr{
				Terms: &Not{
					Body: NewBody(NewExpr(&LogicalOr{
						Lhs: NewBody(NewExpr(VarTerm("a"))),
						Rhs: NewBody(NewExpr(VarTerm("b"))),
					})),
				},
			},
		},
		{
			note:  "not group and",
			input: "not (a and b)",
			exp: &Expr{
				Terms: &Not{
					Body: NewBody(NewExpr(&LogicalAnd{
						Lhs: NewBody(NewExpr(VarTerm("a"))),
						Rhs: NewBody(NewExpr(VarTerm("b"))),
					})),
				},
			},
		},
		{
			note:  "not group as operand of and",
			input: "x and not (a or b)",
			exp: &Expr{
				Terms: &LogicalAnd{
					Lhs: NewBody(NewExpr(VarTerm("x"))),
					Rhs: NewBody(NewExpr(&Not{
						Body: NewBody(NewExpr(&LogicalOr{
							Lhs: NewBody(NewExpr(VarTerm("a"))),
							Rhs: NewBody(NewExpr(VarTerm("b"))),
						})),
					})),
				},
			},
		},
		{
			note:  "not nested group",
			input: "not ((a or b) and c)",
			exp: &Expr{
				Terms: &Not{
					Body: NewBody(NewExpr(&LogicalAnd{
						Lhs: NewBody(NewExpr(&LogicalOr{
							Lhs: NewBody(NewExpr(VarTerm("a"))),
							Rhs: NewBody(NewExpr(VarTerm("b"))),
						})),
						Rhs: NewBody(NewExpr(VarTerm("c"))),
					})),
				},
			},
		},
		{
			note:  "not group binds tighter than trailing and",
			input: "not (a or b) and c",
			exp: &Expr{
				Terms: &LogicalAnd{
					Lhs: NewBody(NewExpr(&Not{
						Body: NewBody(NewExpr(&LogicalOr{
							Lhs: NewBody(NewExpr(VarTerm("a"))),
							Rhs: NewBody(NewExpr(VarTerm("b"))),
						})),
					})),
					Rhs: NewBody(NewExpr(VarTerm("c"))),
				},
			},
		},
		{
			note:  "not group binds tighter than trailing or",
			input: "not (a or b) or c",
			exp: &Expr{
				Terms: &LogicalOr{
					Lhs: NewBody(NewExpr(&Not{
						Body: NewBody(NewExpr(&LogicalOr{
							Lhs: NewBody(NewExpr(VarTerm("a"))),
							Rhs: NewBody(NewExpr(VarTerm("b"))),
						})),
					})),
					Rhs: NewBody(NewExpr(VarTerm("c"))),
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			assertParseOneExpr(t, tc.note, tc.input, tc.exp, opts)
		})
	}
}

func TestParseLogical_ParenRedundant(t *testing.T) {
	opts := logicalParserOpts("not")

	tests := []struct {
		note  string
		input string
		exp   *Expr
	}{
		{
			note:  "double-wrapped and-chain",
			input: "((a and b))",
			exp: &Expr{Terms: &LogicalAnd{
				Lhs: NewBody(NewExpr(VarTerm("a"))),
				Rhs: NewBody(NewExpr(VarTerm("b"))),
			}},
		},
		{
			note:  "deeply nested redundant parens",
			input: "(((((a or ((b)))) and c)))",
			exp: &Expr{Terms: &LogicalAnd{
				Lhs: NewBody(NewExpr(&LogicalOr{
					Lhs: NewBody(NewExpr(VarTerm("a"))),
					Rhs: NewBody(NewExpr(VarTerm("b"))),
				})),
				Rhs: NewBody(NewExpr(VarTerm("c"))),
			}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			assertParseOneExpr(t, tc.note, tc.input, tc.exp, opts)
		})
	}
}

func TestParseLogical_ParenWith(t *testing.T) {
	opts := logicalParserOpts("not")

	successTests := []struct {
		note  string
		input string
		exp   *Expr
	}{
		{
			note:  "with on standalone group, or",
			input: "(a or b) with input as x",
			exp: &Expr{
				Terms: &LogicalOr{
					Lhs: NewBody(NewExpr(VarTerm("a"))),
					Rhs: NewBody(NewExpr(VarTerm("b"))),
				},
				With: []*With{{Target: NewTerm(InputRootRef), Value: VarTerm("x")}},
			},
		},
		{
			note:  "with on standalone group, and",
			input: "(a and b) with input as x",
			exp: &Expr{
				Terms: &LogicalAnd{
					Lhs: NewBody(NewExpr(VarTerm("a"))),
					Rhs: NewBody(NewExpr(VarTerm("b"))),
				},
				With: []*With{{Target: NewTerm(InputRootRef), Value: VarTerm("x")}},
			},
		},
		{
			note:  "with on group chain attaches to outer",
			input: "(a or b) and c with input as x",
			exp: &Expr{
				Terms: &LogicalAnd{
					Lhs: NewBody(NewExpr(&LogicalOr{
						Lhs: NewBody(NewExpr(VarTerm("a"))),
						Rhs: NewBody(NewExpr(VarTerm("b"))),
					})),
					Rhs: NewBody(NewExpr(VarTerm("c"))),
				},
				With: []*With{{Target: NewTerm(InputRootRef), Value: VarTerm("x")}},
			},
		},
		{
			note:  "with on group chain attaches to inner",
			input: "(a or b with input as x) and c",
			exp: &Expr{
				Terms: &LogicalAnd{
					Lhs: NewBody(&Expr{
						Terms: &LogicalOr{
							Lhs: NewBody(NewExpr(VarTerm("a"))),
							Rhs: NewBody(NewExpr(VarTerm("b"))),
						},
						With: []*With{{Target: NewTerm(InputRootRef), Value: VarTerm("x")}},
					}),
					Rhs: NewBody(NewExpr(VarTerm("c"))),
				},
			},
		},
		{
			note:  "with on not group",
			input: "not (a or b) with input as x",
			exp: &Expr{
				Terms: &Not{Body: NewBody(NewExpr(&LogicalOr{
					Lhs: NewBody(NewExpr(VarTerm("a"))),
					Rhs: NewBody(NewExpr(VarTerm("b"))),
				}))},
				With: []*With{{Target: NewTerm(InputRootRef), Value: VarTerm("x")}},
			},
		},
		{
			note:  "with on lhs operand group",
			input: "(a with input as x) and b",
			exp: &Expr{Terms: &LogicalAnd{
				Lhs: NewBody(&Expr{
					Terms: VarTerm("a"),
					With:  []*With{{Target: NewTerm(InputRootRef), Value: VarTerm("x")}},
				}),
				Rhs: NewBody(NewExpr(VarTerm("b"))),
			}},
		},
		{
			note:  "with on rhs operand group",
			input: "a and (b with input as x)",
			exp: &Expr{Terms: &LogicalAnd{
				Lhs: NewBody(NewExpr(VarTerm("a"))),
				Rhs: NewBody(&Expr{
					Terms: VarTerm("b"),
					With:  []*With{{Target: NewTerm(InputRootRef), Value: VarTerm("x")}},
				}),
			}},
		},
		{
			note:  "with inside group binds to whole group",
			input: "(a and b with input as x)",
			exp: &Expr{
				Terms: &LogicalAnd{
					Lhs: NewBody(NewExpr(VarTerm("a"))),
					Rhs: NewBody(NewExpr(VarTerm("b"))),
				},
				With: []*With{{Target: NewTerm(InputRootRef), Value: VarTerm("x")}},
			},
		},
		{
			note:  "with after group binds to whole group",
			input: "(a and b) with input as x",
			exp: &Expr{
				Terms: &LogicalAnd{
					Lhs: NewBody(NewExpr(VarTerm("a"))),
					Rhs: NewBody(NewExpr(VarTerm("b"))),
				},
				With: []*With{{Target: NewTerm(InputRootRef), Value: VarTerm("x")}},
			},
		}, {
			note:  "with without group (baseline)",
			input: "a and b with input as x",
			exp: &Expr{
				Terms: &LogicalAnd{
					Lhs: NewBody(NewExpr(VarTerm("a"))),
					Rhs: NewBody(NewExpr(VarTerm("b"))),
				},
				With: []*With{{Target: NewTerm(InputRootRef), Value: VarTerm("x")}},
			},
		},
		{
			note:  "with on each operand group",
			input: "(a with input as x) and (b with input as y)",
			exp: &Expr{Terms: &LogicalAnd{
				Lhs: NewBody(&Expr{
					Terms: VarTerm("a"),
					With:  []*With{{Target: NewTerm(InputRootRef), Value: VarTerm("x")}},
				}),
				Rhs: NewBody(&Expr{
					Terms: VarTerm("b"),
					With:  []*With{{Target: NewTerm(InputRootRef), Value: VarTerm("y")}},
				}),
			}},
		},
		{
			note:  "with on single-operand not group",
			input: "not (a with input as x)",
			exp: &Expr{Terms: &Not{
				Body: NewBody(&Expr{
					Terms: VarTerm("a"),
					With:  []*With{{Target: NewTerm(InputRootRef), Value: VarTerm("x")}},
				}),
			}},
		},
		{
			note:  "with inside not group binds to group",
			input: "not (a or b with input as x)",
			exp: &Expr{Terms: &Not{Body: NewBody(&Expr{
				Terms: &LogicalOr{
					Lhs: NewBody(NewExpr(VarTerm("a"))),
					Rhs: NewBody(NewExpr(VarTerm("b"))),
				},
				With: []*With{{Target: NewTerm(InputRootRef), Value: VarTerm("x")}},
			})}},
		},
		{
			note:  "with on rhs group content",
			input: "a and (b or c with input as x)",
			exp: &Expr{Terms: &LogicalAnd{
				Lhs: NewBody(NewExpr(VarTerm("a"))),
				Rhs: NewBody(&Expr{
					Terms: &LogicalOr{
						Lhs: NewBody(NewExpr(VarTerm("b"))),
						Rhs: NewBody(NewExpr(VarTerm("c"))),
					},
					With: []*With{{Target: NewTerm(InputRootRef), Value: VarTerm("x")}},
				}),
			}},
		},
		{
			note:  "with on mid-chain group",
			input: "a and (b with input as v) and c",
			exp: &Expr{Terms: &LogicalAnd{
				Lhs: NewBody(NewExpr(&LogicalAnd{
					Lhs: NewBody(NewExpr(VarTerm("a"))),
					Rhs: NewBody(&Expr{
						Terms: VarTerm("b"),
						With:  []*With{{Target: NewTerm(InputRootRef), Value: VarTerm("v")}},
					}),
				})),
				Rhs: NewBody(NewExpr(VarTerm("c"))),
			}},
		},
	}
	for _, tc := range successTests {
		t.Run(tc.note, func(t *testing.T) {
			assertParseOneExpr(t, tc.note, tc.input, tc.exp, opts)
		})
	}

	// A `with` may appear at the end of a group (binding to the whole group), but
	// not mid-group before an `and`/`or` — that must bind to the whole group, so
	// the `with` is reported as disallowed on the operand.
	errorTests := []struct {
		note     string
		input    string
		expected string
	}{
		{
			note:     "mid-group with before and",
			input:    "(a with input as v and b)",
			expected: "`with` modifier is not allowed on operand of `and`",
		},
		{
			note:     "mid-group with before or",
			input:    "(a with input as v or b)",
			expected: "`with` modifier is not allowed on operand of `or`",
		},
		{
			note:     "mid-group with inside not group",
			input:    "not (a with input as v and b)",
			expected: "`with` modifier is not allowed on operand of `and`",
		},
	}
	for _, tc := range errorTests {
		assertParseErrorContains(t, tc.note, tc.input, tc.expected, opts)
	}
}

func TestParseLogical_ParenSerialization(t *testing.T) {
	opts := logicalParserOpts("not")

	tests := []struct {
		note   string
		input  string
		expect string
	}{
		{"or under and, rhs", "a and (b or c)", "a and (b or c)"},
		{"or under and, lhs", "(a or b) and c", "(a or b) and c"},
		{"and under or needs no parens", "a or b and c", "a or b and c"},
		{"same-op or, no parens", "a or b or c", "a or b or c"},
		{"same-op or rhs keeps parens", "a or (b or c)", "a or (b or c)"},
		{"left-assoc or drops parens", "(a or b) or c", "a or b or c"},
		{"same-op and, no parens", "a and b and c", "a and b and c"},
		{"same-op and rhs keeps parens", "a and (b and c)", "a and (b and c)"},
		{"left-assoc and drops parens", "(a and b) and c", "a and b and c"},
		{"redundant same-precedence dropped", "(a and b) or c", "a and b or c"},
		{"groups on both sides", "(a or b) and (c or d)", "(a or b) and (c or d)"},
		{"not group", "not (a or b)", "not (a or b)"},
		{"not group, drops redundant outer group", "(not a)", "not a"},
		{"not group, drops redundant operand group", "not (a)", "not a"},
		{"not group as operand", "x and not (a or b)", "x and not (a or b)"},
		{"explicit body stays braced", "a and {b or c}", "a and { b or c }"},
		{"with operand group, lhs", "(a with input as x) or b", "(a with input as x) or b"},
		{"with operand group, rhs", "a or (b with input as x)", "a or (b with input as x)"},
		{"with binds to whole group, outer", "(a and b with input as x)", "a and b with input as x"},
		{"with binds to whole group, inner", "(a and b) with input as x", "a and b with input as x"},
		{"with no parens", "a and b with input as x", "a and b with input as x"},
		{"deeply redundant parens minimized", "(((((a or ((b)))) and c)))", "(a or b) and c"},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			body, err := ParseBodyWithOpts(tc.input, opts)
			if err != nil {
				t.Fatalf("parse %q: %v", tc.input, err)
			}
			expr := body[0]

			got := expr.String()
			if got != tc.expect {
				t.Errorf("String(): want %q, got %q", tc.expect, got)
			}

			// AppendText must match String().
			at, err := expr.AppendText(nil)
			if err != nil {
				t.Fatalf("AppendText: %v", err)
			}
			if string(at) != got {
				t.Errorf("AppendText: want %q, got %q", got, string(at))
			}

			// StringLength must match the rendered length.
			if n := expr.StringLength(); n != len(got) {
				t.Errorf("StringLength: want %d, got %d (for %q)", len(got), n, got)
			}

			// String() must re-parse to an equal AST.
			reparsed, err := ParseBodyWithOpts(got, opts)
			if err != nil {
				t.Fatalf("re-parse %q: %v", got, err)
			}
			if !body.Equal(reparsed) {
				t.Errorf("re-parse not equal:\noriginal: %v\nreparsed: %v", body, reparsed)
			}
		})
	}
}
