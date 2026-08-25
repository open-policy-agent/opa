package topdown

import (
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/storage"
	"github.com/open-policy-agent/opa/v1/storage/inmem"
)

func TestAliasIndexFollowsDataChanges(t *testing.T) {
	compiler := ast.NewCompiler()
	compiler.Compile(map[string]*ast.Module{
		"alias.rego": ast.MustParseModule("package alias\n\npath := data.request.path"),
		"test.rego": ast.MustParseModule(`package test

import data.alias.path

p := "a" if path == ["a"]

p := "b" if path == ["b"]
`),
	})
	if compiler.Failed() {
		t.Fatal(compiler.Errors)
	}

	store := inmem.New()
	ctx := t.Context()

	check := func(set string, want string) {
		t.Helper()
		txn, err := store.NewTransaction(ctx, storage.WriteParams)
		if err != nil {
			t.Fatal(err)
		}
		if err := store.Write(ctx, txn, storage.AddOp, storage.MustParsePath("/request"),
			map[string]any{"path": []any{set}}); err != nil {
			t.Fatal(err)
		}
		if err := store.Commit(ctx, txn); err != nil {
			t.Fatal(err)
		}

		rtxn, err := store.NewTransaction(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer store.Abort(ctx, rtxn)

		rs, err := NewQuery(ast.MustParseBody("data.test.p")).
			WithCompiler(compiler).WithStore(store).WithTransaction(rtxn).Run(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if want == "" {
			if len(rs) != 0 {
				t.Fatalf("data=%q: expected undefined, got %v", set, rs)
			}
			return
		}
		if len(rs) != 1 {
			t.Fatalf("data=%q: expected one result, got %v", set, rs)
		}
		var v any
		for _, val := range rs[0] {
			v = val.Value.String()
		}
		if v != `"`+want+`"` {
			t.Fatalf("data=%q: got %v, want %q", set, v, want)
		}
	}

	check("a", "a")
	check("b", "b")
	check("c", "")
	check("a", "a")
}

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
