package rego

import (
	"testing"

	"github.com/open-policy-agent/opa/v1/storage"
	"github.com/open-policy-agent/opa/v1/storage/inmem"
)

func TestAliasIndexFollowsDataChanges(t *testing.T) {
	store := inmem.New()
	r := New(
		Query("data.test.p"),
		Module("alias.rego", "package alias\n\npath := data.request.path"),
		Module("test.rego", `package test

import data.alias.path

p := "a" if path == ["a"]

p := "b" if path == ["b"]
`),
		Store(store),
	)
	pq, err := r.PrepareForEval(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	check := func(set, want string) {
		t.Helper()
		txn, err := store.NewTransaction(t.Context(), storage.WriteParams)
		if err != nil {
			t.Fatal(err)
		}
		if err := store.Write(t.Context(), txn, storage.AddOp, storage.MustParsePath("/request"),
			map[string]any{"path": []any{set}}); err != nil {
			t.Fatal(err)
		}
		if err := store.Commit(t.Context(), txn); err != nil {
			t.Fatal(err)
		}

		rs, err := pq.Eval(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		if want == "" {
			if len(rs) != 0 {
				t.Fatalf("data=%q: expected undefined, got %v", set, rs)
			}
			return
		}
		if len(rs) != 1 || len(rs[0].Expressions) != 1 || rs[0].Expressions[0].Value != want {
			t.Fatalf("data=%q: got %v, want %q", set, rs, want)
		}
	}

	check("a", "a")
	check("b", "b")
	check("c", "")
	check("a", "a")
}
