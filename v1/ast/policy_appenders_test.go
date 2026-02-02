package ast_test

import (
	"encoding"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
)

var policyAppenderTests = []struct {
	name  string
	node  ast.StringLengther
	want  string
	nolen bool
}{
	{
		name: "module",
		node: &ast.Module{
			Package: &ast.Package{
				Path: ast.Ref{ast.DefaultRootDocument, ast.InternedTerm("a"), ast.InternedTerm("b")},
			},
			Imports: ast.MustParseImports(`
				import data.foo.bar as baz
				import input.a.b.c
			`),
		},
		want: `package a.b

import data.foo.bar as baz
import input.a.b.c
`,
	},
	{
		name: "module annotated",
		node: ast.MustParseModuleWithOpts(`# METADATA
# title: p
package p

# METADATA
# title: r
r = true`,
			ast.ParserOptions{ProcessAnnotation: true}),
		want: "# METADATA\n# {\"scope\":\"package\",\"title\":\"p\"}\npackage p\n\n# METADATA\n# {\"scope\":\"rule\",\"title\":\"r\"}\nr = true if { true }",
		// We don't count this correctly for annoations currently.
		nolen: true,
	},
	{
		name: "package",
		node: &ast.Package{
			Path: ast.MustParseRef("data.example.foo"),
		},
		want: `package example.foo`,
	},
	{
		name: "package with special chars",
		node: &ast.Package{
			Path: ast.Ref{
				ast.DefaultRootDocument,
				ast.StringTerm("pkg"),
				ast.StringTerm("with-a"),
				ast.StringTerm("dash"),
			},
		},
		want: `package pkg["with-a"].dash`,
	},
	{
		name: "import",
		node: &ast.Import{
			Path:  ast.NewTerm(ast.MustParseRef("data.example.foo")),
			Alias: ast.Var("bar"),
		},
		want: `import data.example.foo as bar`,
	},
	{
		name: "head",
		node: &ast.Head{
			Reference: ast.Ref{ast.VarTerm("allow")},
			Value:     ast.InternedTerm(true),
		},
		want: `allow = true`,
	},
	{
		name: "head assign",
		node: &ast.Head{
			Reference: ast.Ref{ast.VarTerm("allow")},
			Value:     ast.InternedTerm(false),
			Assign:    true,
		},
		want: `allow := false`,
	},
	{
		name: "head with key",
		node: &ast.Head{
			Reference: ast.Ref{ast.VarTerm("deny")},
			Key:       ast.InternedTerm("reason"),
		},
		want: `deny contains "reason"`,
	},
	{
		name: "ref head with value",
		node: &ast.Head{
			Reference: ast.Ref{ast.VarTerm("authz"), ast.StringTerm("deny"), ast.VarTerm("user")},
			Value:     ast.InternedTerm("violation"),
			Assign:    true,
		},
		want: `authz.deny[user] := "violation"`,
	},
	{
		name: "body",
		node: ast.Body{
			ast.MustParseExpr("input.foo == 1"),
			ast.MustParseExpr("input.bar != 2"),
		},
		want: `equal(input.foo, 1); neq(input.bar, 2)`,
	},
	{
		name: "expr",
		node: ast.MustParseExpr(`input.foo[_][1][baz] == "bar"`),
		want: `equal(input.foo[_][1][baz], "bar")`,
	},
	{
		name: "with",
		node: &ast.With{
			Target: ast.MustParseTerm("input.foo"),
			Value:  ast.MustParseTerm(`"bar"`),
		},
		want: `with input.foo as "bar"`,
	},
	{
		name: "every",
		node: &ast.Every{
			Key:    ast.MustParseTerm("k"),
			Value:  ast.MustParseTerm("v"),
			Domain: ast.MustParseTerm("input.map"),
			Body: ast.Body{
				ast.MustParseExpr("v > 0"),
			},
		},
		want: `every k, v in input.map { gt(v, 0) }`,
	},
	{
		name: "some decl",
		node: &ast.SomeDecl{
			Symbols: []*ast.Term{
				ast.MustParseTerm("x"),
				ast.MustParseTerm("y"),
			},
		},
		want: `some x, y`,
	},
	{
		name: "some in decl",
		node: ast.MustParseExpr("some x, y in input.map"),
		want: `some x, y in input.map`,
	},
}

func TestASTNodeTextAppendersAndLengthAllocation(t *testing.T) {
	for _, tc := range policyAppenderTests {
		t.Run(tc.name, func(t *testing.T) {
			var buf []byte
			res, err := tc.node.(encoding.TextAppender).AppendText(buf)
			if err != nil {
				t.Fatal(err)
			}
			got := string(res)
			if got != tc.want {
				t.Errorf("%s:\nexp: %q\ngot: %q", tc.name, tc.want, got)
			}

			if expLen := tc.node.StringLength(); !tc.nolen && expLen != len(got) {
				t.Errorf("%s: string length = %d, expected %d", tc.name, len(got), expLen)
				t.Logf("%q", got)
			}
		})
	}
}

func BenchmarkNoNodeTypeAllocatesOnAppend(b *testing.B) {
	for _, tc := range policyAppenderTests {
		b.Run(tc.name, func(b *testing.B) {
			buf := make([]byte, 0, tc.node.StringLength())
			for b.Loop() {
				_, err := tc.node.(encoding.TextAppender).AppendText(buf)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
