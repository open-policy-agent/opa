// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package bundle

import (
	"context"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/metrics"
	"github.com/open-policy-agent/opa/v1/storage"
	"github.com/open-policy-agent/opa/v1/storage/inmem"
)

// TestMaterializedViewsIntegration tests that materialized views are created
// during bundle activation
func TestMaterializedViewsIntegration(t *testing.T) {
	ctx := context.Background()
	store := inmem.New()

	// Create test bundle with data and system.store policy
	b := &Bundle{
		Manifest: Manifest{
			Roots:    &[]string{""},
			Revision: "test",
		},
		Data: map[string]any{
			"employees": []any{
				map[string]any{"name": "alice", "department": "engineering", "manager": true},
				map[string]any{"name": "bob", "department": "sales", "manager": false},
				map[string]any{"name": "charlie", "department": "engineering", "manager": false},
			},
		},
		Modules: []ModuleFile{
			{
				Path: "system/store/policies.rego",
				Raw: []byte(`
					package system.store

					# Materialized view of all managers
					managers contains employee if {
						some employee in data.employees
						employee.manager == true
					}

					# Materialized view of engineering employees
					engineers contains employee if {
						some employee in data.employees
						employee.department == "engineering"
					}
				`),
			},
		},
	}

	// Parse modules and set Parsed field on ModuleFile
	for i := range b.Modules {
		module, err := ast.ParseModule(b.Modules[i].Path, string(b.Modules[i].Raw))
		if err != nil {
			t.Fatalf("failed to parse module: %v", err)
		}
		b.Modules[i].Parsed = module
	}

	// Compiler will be populated during activation
	compiler := ast.NewCompiler()

	// Activate bundle
	txn, err := store.NewTransaction(ctx, storage.WriteParams)
	if err != nil {
		t.Fatalf("failed to create transaction: %v", err)
	}

	err = Activate(&ActivateOpts{
		Ctx:      ctx,
		Store:    store,
		Txn:      txn,
		Compiler: compiler,
		Metrics:  metrics.New(),
		Bundles: map[string]*Bundle{
			"test": b,
		},
	})

	if err != nil {
		store.Abort(ctx, txn)
		t.Fatalf("failed to activate bundle: %v", err)
	}

	if err := store.Commit(ctx, txn); err != nil {
		t.Fatalf("failed to commit transaction: %v", err)
	}

	// Verify materialized views were created
	txn, err = store.NewTransaction(ctx, storage.TransactionParams{})
	if err != nil {
		t.Fatalf("failed to create read transaction: %v", err)
	}
	defer store.Abort(ctx, txn)

	// Check managers view
	managers, err := store.Read(ctx, txn, storage.MustParsePath("/system/store/managers"))
	if err != nil {
		t.Fatalf("failed to read managers view: %v", err)
	}

	managersSlice, ok := managers.([]any)
	if !ok {
		t.Fatalf("expected slice, got %T", managers)
	}

	if len(managersSlice) != 1 {
		t.Errorf("expected 1 manager, got %d", len(managersSlice))
	}

	// Verify alice is the manager
	if len(managersSlice) > 0 {
		manager := managersSlice[0].(map[string]any)
		if name := manager["name"]; name != "alice" {
			t.Errorf("expected manager to be alice, got %v", name)
		}
	}

	// Check engineers view
	engineers, err := store.Read(ctx, txn, storage.MustParsePath("/system/store/engineers"))
	if err != nil {
		t.Fatalf("failed to read engineers view: %v", err)
	}

	engineersSlice, ok := engineers.([]any)
	if !ok {
		t.Fatalf("expected slice, got %T", engineers)
	}

	if len(engineersSlice) != 2 {
		t.Errorf("expected 2 engineers, got %d", len(engineersSlice))
	}

	// Verify alice and charlie are engineers
	engineerNames := make(map[string]bool)
	for _, eng := range engineersSlice {
		engineer := eng.(map[string]any)
		engineerNames[engineer["name"].(string)] = true
	}

	if !engineerNames["alice"] {
		t.Error("alice should be in engineers")
	}
	if !engineerNames["charlie"] {
		t.Error("charlie should be in engineers")
	}
}

// TestBundleActivationWithoutSystemStore tests that bundle activation works
// normally when there are no system.store policies
func TestBundleActivationWithoutSystemStore(t *testing.T) {
	ctx := context.Background()
	store := inmem.New()

	b := &Bundle{
		Manifest: Manifest{
			Roots:    &[]string{""},
			Revision: "test",
		},
		Data: map[string]any{
			"users": []any{
				map[string]any{"name": "alice"},
			},
		},
		Modules: []ModuleFile{
			{
				Path: "authz.rego",
				Raw: []byte(`
					package authz
					allow := true
				`),
			},
		},
	}

	// Parse modules and set Parsed field on ModuleFile
	for i := range b.Modules {
		module, err := ast.ParseModule(b.Modules[i].Path, string(b.Modules[i].Raw))
		if err != nil {
			t.Fatalf("failed to parse module: %v", err)
		}
		b.Modules[i].Parsed = module
	}

	// Compiler will be populated during activation
	compiler := ast.NewCompiler()

	txn, err := store.NewTransaction(ctx, storage.WriteParams)
	if err != nil {
		t.Fatalf("failed to create transaction: %v", err)
	}

	err = Activate(&ActivateOpts{
		Ctx:      ctx,
		Store:    store,
		Txn:      txn,
		Compiler: compiler,
		Metrics:  metrics.New(),
		Bundles: map[string]*Bundle{
			"test": b,
		},
	})

	if err != nil {
		store.Abort(ctx, txn)
		t.Fatalf("failed to activate bundle: %v", err)
	}

	if err := store.Commit(ctx, txn); err != nil {
		t.Fatalf("failed to commit transaction: %v", err)
	}

	// Verify data was written normally
	txn, err = store.NewTransaction(ctx, storage.TransactionParams{})
	if err != nil {
		t.Fatalf("failed to create read transaction: %v", err)
	}
	defer store.Abort(ctx, txn)

	users, err := store.Read(ctx, txn, storage.MustParsePath("/users"))
	if err != nil {
		t.Fatalf("failed to read users: %v", err)
	}

	usersSlice, ok := users.([]any)
	if !ok {
		t.Fatalf("expected slice, got %T", users)
	}

	if len(usersSlice) != 1 {
		t.Errorf("expected 1 user, got %d", len(usersSlice))
	}
}
