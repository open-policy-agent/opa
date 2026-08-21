// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package plugins

import (
	"context"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/storage"
	realinmem "github.com/open-policy-agent/opa/v1/storage/inmem"
)

const ruleIndexDataPolicy = `package test

allow if input.subject in data.groups.admins.members
`

var membersRef = ast.MustParseRef("data.groups.admins.members")

// prunedTo returns the subjects the rule index keeps the rule a candidate for.
// The resolver's own data is empty, so only members read into the index when it
// was built survive -- and all three subjects coming back means the index holds
// nothing it can use.
func prunedTo(t *testing.T, m *Manager) []string {
	t.Helper()

	compiler := m.GetCompiler()
	if compiler == nil {
		t.Fatal("no compiler on the manager")
	}

	if refs := compiler.IndexDataRefs(); len(refs) != 1 || !refs[0].Equal(membersRef) {
		t.Fatalf("expected %v to be reported, got %v", membersRef, refs)
	}

	index := compiler.RuleIndex(ast.MustParseRef("data.test.allow"))
	if index == nil {
		t.Fatal("no rule index for data.test.allow")
	}

	var found []string
	for _, candidate := range []string{"alice", "bob", "eve"} {
		result, err := index.Lookup(indexTestResolver{
			compiler: compiler,
			input:    ast.MustParseTerm(`{"subject": "` + candidate + `"}`),
			data:     ast.MustParseTerm(`{"groups": {"admins": {"members": []}}}`),
		})
		if err != nil {
			t.Fatalf("index lookup: %v", err)
		}
		if len(result.Rules) > 0 {
			found = append(found, candidate)
		}
	}
	return found
}

// indexTestResolver stands in for topdown's resolver, which reports the data the
// manager has marked as moved (see ast.Compiler.MarkIndexDataStale).
type indexTestResolver struct {
	compiler *ast.Compiler
	input    *ast.Term
	data     *ast.Term
}

func (r indexTestResolver) Resolve(ref ast.Ref) (ast.Value, error) {
	root := r.input
	if ref.HasPrefix(ast.DefaultRootRef) {
		root = r.data
	}
	v, err := root.Value.Find(ref[1:])
	if err != nil {
		return nil, nil
	}
	return v, nil
}

func (r indexTestResolver) IndexDataStale(ref ast.Ref) bool {
	return r.compiler.IndexDataStale(ref)
}

func newRuleIndexDataManager(t *testing.T, ctx context.Context, store storage.Store) *Manager {
	t.Helper()

	m, err := New([]byte{}, "test", store, RuleIndexData(100))
	if err != nil {
		t.Fatal(err)
	}

	if err := storage.Txn(ctx, store, storage.WriteParams, func(txn storage.Transaction) error {
		return store.UpsertPolicy(ctx, txn, "test.rego", []byte(ruleIndexDataPolicy))
	}); err != nil {
		t.Fatal(err)
	}

	if err := m.Init(ctx); err != nil {
		t.Fatal(err)
	}
	return m
}

func storeWithAdmins(members ...any) storage.Store {
	return realinmem.NewFromObject(map[string]any{
		"groups": map[string]any{"admins": map[string]any{"members": members}},
	})
}

func upsertPolicy(t *testing.T, ctx context.Context, store storage.Store, name, module string) {
	t.Helper()

	if err := storage.Txn(ctx, store, storage.WriteParams, func(txn storage.Transaction) error {
		return store.UpsertPolicy(ctx, txn, name, []byte(module))
	}); err != nil {
		t.Fatal(err)
	}
}

// A data change marks the refs whose data moved rather than recompiling: putting
// a policy compilation on the path of a data write would cost more than the
// indexing saves for any deployment whose data is dynamic.
func TestRuleIndexDataMarkedOnDataChange(t *testing.T) {
	ctx := t.Context()
	store := storeWithAdmins("alice")

	m := newRuleIndexDataManager(t, ctx, store)

	if act := prunedTo(t, m); len(act) != 1 || act[0] != "alice" {
		t.Fatalf("expected the index to prune to [alice], got %v", act)
	}

	before := m.GetCompiler()
	if err := storage.WriteOne(ctx, store, storage.AddOp,
		storage.MustParsePath("/groups/admins/members/-"), "bob"); err != nil {
		t.Fatal(err)
	}

	if m.GetCompiler() != before {
		t.Error("expected no recompilation on a data change")
	}
	if !m.GetCompiler().IndexDataStale(membersRef) {
		t.Errorf("expected %v to be marked stale", membersRef)
	}
	if act := prunedTo(t, m); len(act) != 3 {
		t.Errorf("expected the rule to be a candidate for every subject, got %v", act)
	}

	// The next compilation reads the members as they are now, and stops marking.
	upsertPolicy(t, ctx, store, "other.rego", "package other\n\nq := 1\n")

	if m.GetCompiler().IndexDataStale(membersRef) {
		t.Errorf("expected %v not to be marked after recompiling", membersRef)
	}
	if act := prunedTo(t, m); len(act) != 2 || act[0] != "alice" || act[1] != "bob" {
		t.Errorf("expected the index to prune to [alice bob], got %v", act)
	}
}

// Policies compiled first, data pushed afterwards: nothing was read into the
// index, so nothing prunes until a compilation reads the data that arrived.
func TestRuleIndexDataArrivingAfterThePolicies(t *testing.T) {
	ctx := t.Context()
	store := realinmem.NewFromObject(map[string]any{})

	m := newRuleIndexDataManager(t, ctx, store)

	if act := prunedTo(t, m); len(act) != 3 {
		t.Fatalf("expected no pruning without the data, got %v", act)
	}

	if err := storage.WriteOne(ctx, store, storage.AddOp,
		storage.MustParsePath("/groups"), map[string]any{
			"admins": map[string]any{"members": []any{"alice"}},
		}); err != nil {
		t.Fatal(err)
	}

	if act := prunedTo(t, m); len(act) != 3 {
		t.Errorf("expected the write not to reach the index by itself, got %v", act)
	}

	upsertPolicy(t, ctx, store, "other.rego", "package other\n\nq := 1\n")

	if act := prunedTo(t, m); len(act) != 1 || act[0] != "alice" {
		t.Errorf("expected the index to prune to [alice] after recompiling, got %v", act)
	}
}

func TestRuleIndexDataIgnoresUnrelatedDataChange(t *testing.T) {
	ctx := t.Context()
	store := storeWithAdmins("alice")

	m := newRuleIndexDataManager(t, ctx, store)

	if err := storage.WriteOne(ctx, store, storage.AddOp,
		storage.MustParsePath("/unrelated"), "x"); err != nil {
		t.Fatal(err)
	}

	if m.GetCompiler().IndexDataStale(membersRef) {
		t.Error("expected a write outside the index's refs to leave it alone")
	}
	if act := prunedTo(t, m); len(act) != 1 || act[0] != "alice" {
		t.Errorf("expected the index to still prune to [alice], got %v", act)
	}
}

// A policy change recompiles without any data changing, so nothing about the
// commit says the indices need data read into them again -- but the compiler
// replacing the one in use has none.
func TestRuleIndexDataSurvivesAPolicyChange(t *testing.T) {
	ctx := t.Context()
	store := storeWithAdmins("alice")

	m := newRuleIndexDataManager(t, ctx, store)

	if act := prunedTo(t, m); len(act) != 1 || act[0] != "alice" {
		t.Fatalf("expected the index to prune to [alice], got %v", act)
	}

	upsertPolicy(t, ctx, store, "other.rego", "package other\n\nq := 1\n")

	if act := prunedTo(t, m); len(act) != 1 || act[0] != "alice" {
		t.Errorf("expected the index to still prune to [alice], got %v", act)
	}
}

func TestRuleIndexDataOnByDefault(t *testing.T) {
	ctx := t.Context()
	store := storeWithAdmins("alice")

	m, err := New([]byte{}, "test", store)
	if err != nil {
		t.Fatal(err)
	}
	upsertPolicy(t, ctx, store, "test.rego", ruleIndexDataPolicy)
	if err := m.Init(ctx); err != nil {
		t.Fatal(err)
	}

	if act := prunedTo(t, m); len(act) != 1 || act[0] != "alice" {
		t.Errorf("expected the index to prune to [alice] without passing the option, got %v", act)
	}
}

func TestRuleIndexDataOff(t *testing.T) {
	ctx := t.Context()
	store := storeWithAdmins("alice")

	m, err := New([]byte{}, "test", store, RuleIndexData(0))
	if err != nil {
		t.Fatal(err)
	}
	upsertPolicy(t, ctx, store, "test.rego", ruleIndexDataPolicy)
	if err := m.Init(ctx); err != nil {
		t.Fatal(err)
	}

	// The data ref is reported either way -- it is a property of the policy --
	// but nothing was read into the index, so nothing is pruned.
	if act := prunedTo(t, m); len(act) != 3 {
		t.Errorf("expected no pruning with the option off, got %v", act)
	}

	if err := storage.WriteOne(ctx, store, storage.AddOp,
		storage.MustParsePath("/groups/admins/members/-"), "bob"); err != nil {
		t.Fatal(err)
	}
	if m.GetCompiler().IndexDataStale(membersRef) {
		t.Error("expected no marking with the option off")
	}
}
