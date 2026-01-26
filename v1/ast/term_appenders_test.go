package ast_test

import (
	"encoding"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
)

var value_appender_tests = []struct {
	name string
	term ast.StringLengther
	want string
}{
	{
		name: "var",
		term: ast.MustParseTerm("input.foo.bar"),
		want: "input.foo.bar",
	},
	{
		name: "string",
		term: ast.StringTerm(`foo bar baz`),
		want: `"foo bar baz"`,
	},
	{
		name: "string with escapes",
		term: ast.StringTerm(`"foo" "bar" "qux"`),
		want: `"\"foo\" \"bar\" \"qux\""`,
	},
	{
		name: "string with newlines",
		term: ast.StringTerm("line1\nline2\nline3"),
		want: `"line1\nline2\nline3"`,
	},
	{
		name: "number",
		term: ast.MustParseTerm("3.14"),
		want: "3.14",
	},
	{
		name: "boolean",
		term: ast.MustParseTerm("true"),
		want: "true",
	},
	{
		name: "null",
		term: ast.MustParseTerm("null"),
		want: "null",
	},
	{
		name: "array",
		term: ast.MustParseTerm(`[1, "two", false]`),
		want: `[1, "two", false]`,
	},
	{
		name: "set",
		term: ast.MustParseTerm(`{1, 2, 3}`),
		want: `{1, 2, 3}`,
	},
	{
		name: "object",
		term: ast.MustParseTerm(`{"a": 1, "b": "two"}`),
		want: `{"a": 1, "b": "two"}`,
	},
	{
		name: "call",
		term: ast.MustParseExpr(`foo(input.bar, "baz")`),
		want: `foo(input.bar, "baz")`,
	},
	{
		name: "template string",
		term: ast.MustParseTerm(`$"Hello, {input.name}!"`),
		want: `$"Hello, {input.name}!"`,
	},
	{
		name: "ref",
		term: ast.MustParseTerm(`data.foo["b a r"].allow`),
		want: `data.foo["b a r"].allow`,
	},
	{
		name: "object comprehension",
		term: ast.MustParseTerm(`{k: v |
				some k, v
				data.items[k] == v
				v > 10
			}`),
		want: `{k: v | some k, v; equal(data.items[k], v); gt(v, 10)}`,
	},
	{
		name: "array comprehension",
		term: ast.MustParseTerm(`[(x * 2) | x := data.numbers[_]; x > 5]`),
		want: `[mul(x, 2) | assign(x, data.numbers[_]); gt(x, 5)]`,
	},
	{
		name: "set comprehension",
		term: ast.MustParseTerm(`{x | x := data.values[_]; x < 100}`),
		want: `{x | assign(x, data.values[_]); lt(x, 100)}`,
	},
}

func TestASTValueTextAppendersAndStringLength(t *testing.T) {
	for _, tc := range value_appender_tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf []byte
			var err error

			buf, err = tc.term.(encoding.TextAppender).AppendText(buf)
			if err != nil {
				t.Fatalf("AppendText error: %v", err)
			}
			got := string(buf)
			if got != tc.want {
				t.Errorf("AppendText got %s, want %s", got, tc.want)
			}

			if expLen, gotLen := len(tc.want), tc.term.StringLength(); expLen != gotLen {
				t.Errorf("StringLength = %d; want %d", gotLen, expLen)
				t.Log(got)
			}
		})
	}
}

// Ensure no appender allocates when appending to a pre-sized buffer.
func BenchmarkNoASTTypeAllocatesOnAppendToBufferOfStringLength(b *testing.B) {
	for _, tc := range value_appender_tests {
		b.Run(tc.name, func(b *testing.B) {
			buf := make([]byte, 0, tc.term.StringLength())
			for b.Loop() {
				_, err := tc.term.(encoding.TextAppender).AppendText(buf)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
