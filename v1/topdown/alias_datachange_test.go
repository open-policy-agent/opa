package topdown

import (
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/storage/inmem"
)

func TestAliasInliningRespectsWithInElseBranch(t *testing.T) {
	compiler := ast.NewCompiler()
	compiler.Compile(map[string]*ast.Module{
		"alias.rego": ast.MustParseModule("package alias\n\nv := input.x\n"),
		"lib.rego": ast.MustParseModule(`package lib

g := 1 if input.a

else := 2 if data.alias.v == 1
`),
		"top.rego": ast.MustParseModule(`package top

p if data.lib.g == 2 with data.alias.v as 1
`),
	})
	if compiler.Failed() {
		t.Fatal(compiler.Errors)
	}

	rs, err := NewQuery(ast.MustParseBody("data.top.p")).
		WithCompiler(compiler).
		WithStore(inmem.New()).
		WithInput(ast.MustParseTerm(`{}`)).
		Run(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(rs) != 1 {
		t.Fatalf("expected data.top.p to be defined, got %v", rs)
	}
}
