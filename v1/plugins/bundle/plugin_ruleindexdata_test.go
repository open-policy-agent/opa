// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package bundle

import (
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/bundle"
	"github.com/open-policy-agent/opa/v1/download"
	"github.com/open-policy-agent/opa/v1/metrics"
	"github.com/open-policy-agent/opa/v1/plugins"
	inmemtst "github.com/open-policy-agent/opa/v1/storage/inmem/test"
	"github.com/open-policy-agent/opa/v1/util"
)

const ruleIndexDataModule = `package test

allow if input.subject in data.groups.admins.members
`

// materializedAdmins returns the members the rule index prunes against.
func materializedAdmins(t *testing.T, manager *plugins.Manager) []string {
	t.Helper()

	compiler := manager.GetCompiler()
	if compiler == nil {
		t.Fatal("no compiler on the manager")
	}

	if refs := compiler.IndexDataRefs(); len(refs) != 1 ||
		!refs[0].Equal(ast.MustParseRef("data.groups.admins.members")) {
		t.Fatalf("expected data.groups.admins.members to be materialized, got %v", refs)
	}

	index := compiler.RuleIndex(ast.MustParseRef("data.test.allow"))
	if index == nil {
		t.Fatal("no rule index for data.test.allow")
	}

	var found []string
	for _, candidate := range []string{"alice", "bob"} {
		// The resolver's data is empty on purpose: only values materializedto the
		// index at compile time can keep the rule a candidate here.
		result, err := index.Lookup(indexDataTestResolver{
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

type indexDataTestResolver struct {
	compiler *ast.Compiler
	input    *ast.Term
	data     *ast.Term
}

func (r indexDataTestResolver) IndexDataStale(ref ast.Ref) bool {
	return r.compiler.IndexDataStale(ref)
}

func (r indexDataTestResolver) Resolve(ref ast.Ref) (ast.Value, error) {
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

// A snapshot bundle carries the policy and the data it indexes in one
// transaction, and the modules are compiled before that data reaches the store.
func TestRuleIndexDataSnapshotBundle(t *testing.T) {
	ctx := t.Context()

	store := inmemtst.New()
	manager, err := plugins.New(nil, "test-instance-id", store, plugins.RuleIndexData(100))
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Init(ctx); err != nil {
		t.Fatal(err)
	}
	defer manager.Stop(ctx)

	plugin := New(&Config{}, manager)
	bundleName := "test-bundle"
	plugin.status[bundleName] = &Status{Name: bundleName, Metrics: metrics.New()}
	plugin.downloaders[bundleName] = download.New(download.Config{}, plugin.manager.Client(""), bundleName)

	b := bundle.Bundle{
		Manifest: bundle.Manifest{Revision: "one"},
		Data:     util.MustUnmarshalJSON([]byte(`{"groups": {"admins": {"members": ["alice"]}}}`)).(map[string]any),
		Modules: []bundle.ModuleFile{{
			Path:   "/test.rego",
			Parsed: ast.MustParseModule(ruleIndexDataModule),
			Raw:    []byte(ruleIndexDataModule),
		}},
	}
	b.Manifest.Init()

	if err := plugin.oneShot(ctx, bundleName, download.Update{Bundle: &b, Metrics: metrics.New(), Size: snapshotBundleSize}); err != nil {
		t.Fatal(err)
	}

	if act := materializedAdmins(t, manager); len(act) != 1 || act[0] != "alice" {
		t.Fatalf("expected [alice] materialized, got %v", act)
	}

	// A second snapshot with different data: recompiled anyway, but the values
	// have to come from the new bundle rather than the store's previous state.
	b2 := b
	b2.Manifest = bundle.Manifest{Revision: "two"}
	b2.Data = util.MustUnmarshalJSON([]byte(`{"groups": {"admins": {"members": ["bob"]}}}`)).(map[string]any)
	b2.Manifest.Init()

	if err := plugin.oneShot(ctx, bundleName, download.Update{Bundle: &b2, Metrics: metrics.New(), Size: snapshotBundleSize}); err != nil {
		t.Fatal(err)
	}

	if act := materializedAdmins(t, manager); len(act) != 1 || act[0] != "bob" {
		t.Errorf("expected [bob] materialized after the second snapshot, got %v", act)
	}
}

// A delta bundle patches data without compiling anything, so the values read into
// the indices are the ones the last activation saw. The manager marks the refs it
// moved rather than recompiling -- see plugins.Manager.onCommit.
func TestRuleIndexDataDeltaBundle(t *testing.T) {
	ctx := t.Context()

	store := inmemtst.New()
	manager, err := plugins.New(nil, "test-instance-id", store, plugins.RuleIndexData(100))
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Init(ctx); err != nil {
		t.Fatal(err)
	}
	defer manager.Stop(ctx)

	plugin := New(&Config{}, manager)
	bundleName := "test-bundle"
	plugin.status[bundleName] = &Status{Name: bundleName, Metrics: metrics.New()}
	plugin.downloaders[bundleName] = download.New(download.Config{}, plugin.manager.Client(""), bundleName)

	b := bundle.Bundle{
		Manifest: bundle.Manifest{Revision: "one", Roots: &[]string{"groups", "test"}},
		Data:     util.MustUnmarshalJSON([]byte(`{"groups": {"admins": {"members": ["alice"]}}}`)).(map[string]any),
		Modules: []bundle.ModuleFile{{
			Path:   "/test.rego",
			Parsed: ast.MustParseModule(ruleIndexDataModule),
			Raw:    []byte(ruleIndexDataModule),
		}},
	}

	if err := plugin.oneShot(ctx, bundleName, download.Update{Bundle: &b, Metrics: metrics.New(), Size: snapshotBundleSize}); err != nil {
		t.Fatal(err)
	}
	if act := materializedAdmins(t, manager); len(act) != 1 || act[0] != "alice" {
		t.Fatalf("expected [alice] materialized, got %v", act)
	}

	delta := bundle.Bundle{
		Manifest: bundle.Manifest{Revision: "two", Roots: &[]string{"groups", "test"}},
		Patch: bundle.Patch{Data: []bundle.PatchOperation{{
			Op:    "upsert",
			Path:  "/groups/admins/members",
			Value: []any{"bob"},
		}}},
	}

	if err := plugin.oneShot(ctx, bundleName, download.Update{Bundle: &delta, Metrics: metrics.New(), Size: deltaBundleSize}); err != nil {
		t.Fatal(err)
	}

	membersRef := ast.MustParseRef("data.groups.admins.members")
	if !manager.GetCompiler().IndexDataStale(membersRef) {
		t.Errorf("expected %v to be marked stale by the delta bundle", membersRef)
	}
	if act := materializedAdmins(t, manager); len(act) != 2 {
		t.Errorf("expected the rule to be a candidate for both subjects, got %v", act)
	}

	// The next snapshot compiles, so it reads the patched members.
	snapshot := bundle.Bundle{
		Manifest: bundle.Manifest{Revision: "three", Roots: &[]string{"groups", "test"}},
		Data:     util.MustUnmarshalJSON([]byte(`{"groups": {"admins": {"members": ["bob"]}}}`)).(map[string]any),
		Modules: []bundle.ModuleFile{{
			Path:   "/test.rego",
			Parsed: ast.MustParseModule(ruleIndexDataModule),
			Raw:    []byte(ruleIndexDataModule),
		}},
	}

	if err := plugin.oneShot(ctx, bundleName, download.Update{Bundle: &snapshot, Metrics: metrics.New(), Size: snapshotBundleSize}); err != nil {
		t.Fatal(err)
	}

	if act := materializedAdmins(t, manager); len(act) != 1 || act[0] != "bob" {
		t.Errorf("expected [bob] read into the index after the snapshot, got %v", act)
	}
}
