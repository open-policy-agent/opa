package topdown

import (
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
)

var tests = []struct {
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

func TestBuiltinTemplateString(t *testing.T) {
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

// BenchmarkBuiltinTemplateString/no_parts-16         				13434396	        82.17 ns/op	     344 B/op	       4 allocs/op
// BenchmarkBuiltinTemplateString/single_string_part-16         	11506334	       106.0 ns/op	     376 B/op	       6 allocs/op
// BenchmarkBuiltinTemplateString/single_undefined_part-16      	11367075	       106.0 ns/op	     376 B/op	       6 allocs/op
// BenchmarkBuiltinTemplateString/primitives-16                 	 8217890	       144.9 ns/op	     440 B/op	       7 allocs/op
// BenchmarkBuiltinTemplateString/collections-16                	 2056494	       583.7 ns/op	    1144 B/op	      28 allocs/op
// BenchmarkBuiltinTemplateString/multiple_outputs-16           	 9424003	       128.8 ns/op	     480 B/op	       7 allocs/op
func BenchmarkBuiltinTemplateString(b *testing.B) {
	for _, tc := range tests {
		b.Run(tc.note, func(b *testing.B) {
			bctx := BuiltinContext{}
			oper := []*ast.Term{ast.NewTerm(tc.parts)}
			iter := eqIter(tc.expRes)

			for b.Loop() {
				_ = builtinTemplateString(bctx, oper, iter)
			}
		})
	}
}
