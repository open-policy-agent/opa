package topdown

import (
	"context"
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
	ctx := context.Background()

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
