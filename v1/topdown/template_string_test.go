package topdown

import (
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
)

func TestBuiltinTemplateString(t *testing.T) {
	tests := []struct {
		note   string
		parts  *ast.Array
		expRes *ast.Term
		expErr string
	}{
		{
			note:   "no parts",
			parts:  ast.NewArray(),
			expRes: ast.StringTerm(""),
		},
		{
			note:   "single string part",
			parts:  ast.NewArray(ast.StringTerm("foo")),
			expRes: ast.StringTerm("foo"),
		},
		{
			note:   "single undefined part",
			parts:  ast.NewArray(ast.SetTerm()),
			expRes: ast.StringTerm("<undefined>"),
		},
		{
			note:   "primitives",
			parts:  ast.NewArray(ast.StringTerm("foo"), ast.NumberTerm("42"), ast.BooleanTerm(false), ast.NullTerm()),
			expRes: ast.StringTerm("foo42falsenull"),
		},
		{
			note: "collections",
			parts: ast.NewArray(
				ast.SetTerm(ast.ArrayTerm()), ast.StringTerm(" "),
				ast.SetTerm(ast.ArrayTerm(ast.StringTerm("a"), ast.StringTerm("b"))), ast.StringTerm(" "),
				ast.SetTerm(ast.SetTerm()), ast.StringTerm(" "),
				ast.SetTerm(ast.SetTerm(ast.StringTerm("c"))), ast.StringTerm(" "),
				ast.SetTerm(ast.ObjectTerm()), ast.StringTerm(" "),
				ast.SetTerm(ast.ObjectTerm(ast.Item(ast.StringTerm("d"), ast.StringTerm("e")))),
			),
			expRes: ast.StringTerm(`[] ["a", "b"] set() {"c"} {} {"d": "e"}`),
		},
		{
			note:   "multiple outputs",
			parts:  ast.NewArray(ast.SetTerm(ast.BooleanTerm(true), ast.BooleanTerm(false))),
			expErr: "eval_conflict_error: template-strings must not produce multiple outputs",
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			var result *ast.Term

			bctx := BuiltinContext{}
			err := builtinTemplateString(bctx, []*ast.Term{ast.NewTerm(tc.parts)}, func(t *ast.Term) error {
				result = t
				return nil
			})

			if tc.expErr == "" {
				if err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}

				if ast.Compare(tc.expRes, result) != 0 {
					t.Fatalf("Expected result:\n\n%s\n\ngot:\n\n%s", tc.expRes, result)
				}
			} else {
				if err == nil {
					t.Fatalf("Expected error, got nil")
				}
				if act := err.Error(); !strings.Contains(act, tc.expErr) {
					t.Fatalf("Expected error to contain:\n\n%s\n\ngot:\n\n%s", tc.expErr, act)
				}
			}
		})
	}
}
