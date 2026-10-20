package ast

import (
	"slices"
	"strings"
	"testing"
)

func TestCheckRegoV2(t *testing.T) {
	tests := []struct {
		note    string
		module  string
		opts    *RegoCheckOptions
		expErrs []string
	}{
		{
			note: "no keywords",
			module: `package test
				p if {
					x := {1} | {2}
					y := {1} & {2}
					input.and == x
					input["or"] == y
					"and" == "or"
				}
				q.and := 1
				r.or contains 1`,
		},
		{
			note: "rule names",
			module: `package test
				and := 1
				or(x) := x`,
			expErrs: []string{
				"2:5: rego_parse_error: and keyword cannot be used for rule name",
				"3:5: rego_parse_error: or keyword cannot be used for rule name",
			},
		},
		{
			note: "rule names, NoKeywordsAsRuleNames disabled",
			module: `package test
				and := 1`,
			opts: &RegoCheckOptions{},
		},
		{
			note: "variable names, NoKeywordsAsVarNames disabled",
			module: `package test
				p if {
					and := 1
					and == 1
				}`,
			opts: &RegoCheckOptions{},
		},
		{
			note: "variable names",
			module: `package test
				p if {
					and := 1
					some or in input.xs
					every x in input.ys { x == or }
				}
				q contains and if some and in input.xs
				r := or if or := 1
				s[and] := 1 if some and in input.xs
				f(and) := 1
				g(x) := 1 if x == 1 else := 2 if {
					or := 1
				}
				h := [and | and := input.xs]
				i := $"foo {and} {or} bar"
				j := input[and].foo[or].bar
				`,
			expErrs: []string{
				"3:6: rego_parse_error: and keyword cannot be used for variable name",
				"4:11: rego_parse_error: or keyword cannot be used for variable name",
				"5:33: rego_parse_error: or keyword cannot be used for variable name",
				"7:16: rego_parse_error: and keyword cannot be used for variable name",
				"7:28: rego_parse_error: and keyword cannot be used for variable name",
				"8:10: rego_parse_error: or keyword cannot be used for variable name",
				"8:16: rego_parse_error: or keyword cannot be used for variable name",
				"9:7: rego_parse_error: and keyword cannot be used for variable name",
				"9:25: rego_parse_error: and keyword cannot be used for variable name",
				"10:7: rego_parse_error: and keyword cannot be used for variable name",
				"12:6: rego_parse_error: or keyword cannot be used for variable name",
				"14:11: rego_parse_error: and keyword cannot be used for variable name",
				"14:17: rego_parse_error: and keyword cannot be used for variable name",
				"15:17: rego_parse_error: and keyword cannot be used for variable name",
				"15:23: rego_parse_error: or keyword cannot be used for variable name",
				"16:16: rego_parse_error: and keyword cannot be used for variable name",
				"16:25: rego_parse_error: or keyword cannot be used for variable name",
			},
		},
		{
			note: "v1 checks",
			module: `package test
				import input.foo
				import data.foo
				input := 1`,
			expErrs: []string{
				"3:5: rego_compile_error: import must not shadow import input.foo",
				"4:5: rego_compile_error: rules must not shadow input (use a different rule name)",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			mod := MustParseModuleWithOpts(tc.module, ParserOptions{RegoVersion: RegoV1})

			var errs Errors
			if tc.opts != nil {
				errs = CheckRegoV2WithOptions(mod, *tc.opts)
			} else {
				errs = CheckRegoV2(mod)
			}

			act := make([]string, 0, len(errs))
			for _, err := range errs {
				act = append(act, err.Error())
			}

			exp := tc.expErrs
			if exp == nil {
				exp = []string{}
			}
			if !slices.Equal(act, exp) {
				t.Fatalf("expected errors:\n%s\n\ngot:\n%s", strings.Join(exp, ",\n"), strings.Join(act, ",\n"))
			}
		})
	}
}

func TestCheckRegoV2Rule(t *testing.T) {
	mod := MustParseModuleWithOpts("package test\nand := 1", ParserOptions{RegoVersion: RegoV1})

	errs := CheckRegoV2(mod.Rules[0])
	if exp := "2:1: rego_parse_error: and keyword cannot be used for rule name"; len(errs) != 1 || errs[0].Error() != exp {
		t.Fatalf("expected error %q, got %v", exp, errs)
	}
}

func TestCheckRegoV2PanicsOnUnsupportedType(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	CheckRegoV2(MustParseBody("x := 1"))
}
